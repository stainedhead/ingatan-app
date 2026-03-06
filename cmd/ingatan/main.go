// Command ingatan starts the ingatan memory server.
package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/server"
	"github.com/stainedhead/ingatan/internal/adapter/mcp"
	"github.com/stainedhead/ingatan/internal/adapter/rest"
	apimw "github.com/stainedhead/ingatan/internal/adapter/rest/middleware"
	"github.com/stainedhead/ingatan/internal/domain"
	"github.com/stainedhead/ingatan/internal/infrastructure/backup"
	"github.com/stainedhead/ingatan/internal/infrastructure/config"
	"github.com/stainedhead/ingatan/internal/infrastructure/embed"
	"github.com/stainedhead/ingatan/internal/infrastructure/index"
	"github.com/stainedhead/ingatan/internal/infrastructure/ingest"
	"github.com/stainedhead/ingatan/internal/infrastructure/llm"
	"github.com/stainedhead/ingatan/internal/infrastructure/storage"
	conversationuc "github.com/stainedhead/ingatan/internal/usecase/conversation"
	memoryuc "github.com/stainedhead/ingatan/internal/usecase/memory"
	principaluc "github.com/stainedhead/ingatan/internal/usecase/principal"
	storeuc "github.com/stainedhead/ingatan/internal/usecase/store"
)

const version = "1.0.0-dev"

func main() {
	var configPath string
	flag.StringVar(&configPath, "config", "", "path to config.yaml (default: ~/.ingatan/config.yaml)")
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	if err := run(configPath); err != nil {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}
}

// run is the real entry point. Separating it from main lets defers execute
// before os.Exit is called (in particular the OTel provider shutdown).
func run(configPath string) error {
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	dataDir := expandHome(cfg.DataDir)
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return fmt.Errorf("create data directory %s: %w", dataDir, err)
	}

	// Infrastructure: storage
	fs := storage.NewFileStore(dataDir)
	memRepo := storage.NewMemoryRepo(fs)
	chunkRepo := storage.NewChunkRepo(fs)
	storeRepo := storage.NewStoreRepo(fs)
	principalRepo := storage.NewPrincipalRepo(fs)
	conversationRepo := storage.NewConversationRepo(fs)
	msgRepo := storage.NewMessageRepo(fs)

	// Infrastructure: chunker
	chunker := ingest.NewRecursiveChunker(cfg.Chunking)

	// Infrastructure: embedder (optional — nil if not configured)
	var embedder memoryuc.Embedder
	if cfg.Embedding.Provider != "" && cfg.Embedding.Model != "" {
		embedder = embed.NewOpenAIEmbedder(cfg.Embedding)
		slog.Info("embedding enabled", "provider", cfg.Embedding.Provider, "model", cfg.Embedding.Model)
	} else {
		slog.Info("embedding disabled — no provider configured; memories will be saved without vectors")
	}

	// Infrastructure: search indexes (per-store HNSW + BM25 registries).
	// Dimensions default to 1536 (text-embedding-3-small) when not specified.
	dims := cfg.Embedding.Dimensions
	if dims <= 0 {
		dims = 1536
	}
	hnswStore := index.NewHNSWStore(dataDir, dims)
	bm25Store := index.NewBM25Store(dataDir)

	// Infrastructure: ingest — URL fetcher + file reader (optional; used by memory_save_url / memory_save_file).
	maxIngestBytes := cfg.Ingest.MaxContentBytes
	if maxIngestBytes <= 0 {
		maxIngestBytes = 10 * 1024 * 1024
	}
	urlFetcher := ingest.NewHTTPURLFetcher(maxIngestBytes)
	pdfExtractor := ingest.NewPDFExtractor(maxIngestBytes)
	fileReader := ingest.NewMultiFileReader(maxIngestBytes, pdfExtractor)
	ingestOpts := memoryuc.IngestOptions{
		AllowedPaths:    cfg.Ingest.AllowedPaths,
		MaxContentBytes: maxIngestBytes,
	}
	slog.Info("ingest configured", "max_bytes", maxIngestBytes, "allowed_paths", len(cfg.Ingest.AllowedPaths))

	// Use case: store + principal services
	storeSvc := storeuc.NewService(storeRepo)
	principalSvc := principaluc.NewService(principalRepo, storeRepo)

	// Store access adapter: wraps storeRepo for memory service access checks.
	storeAccess := &storeAccessImpl{repo: storeRepo}

	// Use case: memory service
	memorySvc := memoryuc.NewService(memRepo, chunkRepo, chunker, embedder, hnswStore, bm25Store, urlFetcher, fileReader, ingestOpts, storeAccess)

	// Infrastructure: LLM provider (optional — nil if not configured).
	var llmProvider conversationuc.LLMProvider
	switch cfg.LLM.Provider {
	case "anthropic":
		if cfg.LLM.APIKey != "" && cfg.LLM.Model != "" {
			llmProvider = llm.NewAnthropicProvider(cfg.LLM.APIKey, cfg.LLM.Model, cfg.LLM.BaseURL)
			slog.Info("LLM enabled", "provider", "anthropic", "model", cfg.LLM.Model)
		}
	case "openai", "ollama":
		if cfg.LLM.Model != "" {
			llmProvider = llm.NewOpenAIProvider(cfg.LLM.APIKey, cfg.LLM.Model, cfg.LLM.BaseURL)
			slog.Info("LLM enabled", "provider", cfg.LLM.Provider, "model", cfg.LLM.Model)
		}
	default:
		if cfg.LLM.Provider != "" {
			slog.Warn("unknown LLM provider — summarization disabled", "provider", cfg.LLM.Provider)
		} else {
			slog.Info("LLM disabled — no provider configured; summarization unavailable")
		}
	}

	// Memory saver adapter: wraps memorySvc for conversation promotion.
	memorySaverAdapt := &memorySaverAdapter{svc: memorySvc}

	// Conversation auto-summarize config.
	autoSummCfg := conversationuc.AutoSummarizeConfig{
		MessageThreshold:       cfg.Conversation.AutoSummarizeMessageThreshold,
		TokenEstimateThreshold: cfg.Conversation.AutoSummarizeTokenEstimateThreshold,
	}

	// Use case: conversation service.
	conversationSvc := conversationuc.NewService(conversationRepo, msgRepo, llmProvider, memorySaverAdapt, autoSummCfg)

	// Backup providers from config.
	backupProviders := buildBackupProviders(cfg.Backup)

	// Adapter: REST handlers
	memoryHandler := rest.NewMemoryHandler(memorySvc)
	searchHandler := rest.NewSearchHandler(memorySvc)
	ingestHandler := rest.NewIngestHandler(memorySvc)
	storeHandler := rest.NewStoreHandler(storeSvc)
	principalHandler := rest.NewPrincipalHandler(principalSvc)
	conversationHandler := rest.NewConversationHandler(conversationSvc)
	backupHandler := rest.NewBackupHandler(backupProviders, dataDir)

	// Adapter: MCP server + all tools
	mcpSrv := server.NewMCPServer("ingatan", version)
	mcp.NewMemoryTools(memorySvc).Register(mcpSrv)
	mcp.NewSearchTools(memorySvc).Register(mcpSrv)
	mcp.NewIngestTools(memorySvc).Register(mcpSrv)
	mcp.NewStoreTools(storeSvc).Register(mcpSrv)
	mcp.NewPrincipalTools(principalSvc).Register(mcpSrv)
	mcp.NewConversationTools(conversationSvc).Register(mcpSrv)
	mcpHTTP := server.NewStreamableHTTPServer(mcpSrv, server.WithStateLess(true))

	// System service
	sysSvc := &systemService{cfg: cfg, startedAt: time.Now()}

	var jwtSecret []byte
	if cfg.Auth.Secret != "" {
		jwtSecret = []byte(cfg.Auth.Secret)
	}

	// Principal lookup: GetOrCreate persists the principal and auto-creates their personal store.
	lookup := func(ctx context.Context, claims apimw.JWTClaims) (*domain.Principal, error) {
		return principalSvc.GetOrCreate(ctx, claims)
	}

	// Build router options: wire OTel tracing and rate limiting from config.
	// otelShutdown is always non-nil; defers correctly because run() returns normally.
	routerOpts, otelShutdown := buildRouterOpts(cfg)
	defer func() {
		if shutdownErr := otelShutdown(context.Background()); shutdownErr != nil {
			slog.Warn("OTel provider shutdown error", "error", shutdownErr)
		}
	}()

	router := rest.NewRouter(jwtSecret, lookup, sysSvc, routerOpts,
		memoryHandler, searchHandler, ingestHandler,
		storeHandler, principalHandler, conversationHandler,
		backupHandler,
	)
	router.Mount("/mcp", mcpHTTP)

	// Slog request logger wraps the full router.
	loggedRouter := apimw.SlogLogger(slog.Default())(router)

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      loggedRouter,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	if cfg.Server.TLS.CertFile != "" && cfg.Server.TLS.KeyFile != "" {
		tlsCfg := &tls.Config{
			MinVersion: tls.VersionTLS12,
		}
		if cfg.Server.TLS.MinVersion == "1.3" {
			tlsCfg.MinVersion = tls.VersionTLS13
		}
		// mTLS: load client CA if configured.
		if cfg.Server.TLS.ClientCA != "" {
			caPool, caErr := apimw.LoadClientCA(cfg.Server.TLS.ClientCA)
			if caErr != nil {
				return fmt.Errorf("load client CA %s: %w", cfg.Server.TLS.ClientCA, caErr)
			}
			apimw.ApplyClientAuth(tlsCfg, apimw.ClientAuthRequired, caPool)
			slog.Info("mTLS enabled — client certificates required", "ca_file", cfg.Server.TLS.ClientCA)
		}

		srv.TLSConfig = tlsCfg

		slog.Info("starting ingatan server (TLS)", "addr", addr, "version", version)
		return srv.ListenAndServeTLS(cfg.Server.TLS.CertFile, cfg.Server.TLS.KeyFile)
	}

	slog.Info("starting ingatan server (plain HTTP — no TLS configured)", "addr", addr, "version", version)
	return srv.ListenAndServe()
}

// buildBackupProviders constructs backup.Backuper instances from config.
// Returns an empty slice if no backup is configured. Logs a warning on error.
func buildBackupProviders(cfg config.BackupConfig) []backup.Backuper {
	var providers []backup.Backuper

	if cfg.S3.Enabled {
		s3b, err := backup.NewS3Backup(backup.S3Config{
			Bucket: cfg.S3.Bucket,
			Region: cfg.S3.Region,
			Prefix: cfg.S3.Prefix,
		})
		if err != nil {
			slog.Warn("S3 backup disabled — configuration error", "error", err)
		} else {
			providers = append(providers, s3b)
			slog.Info("S3 backup enabled", "bucket", cfg.S3.Bucket, "prefix", cfg.S3.Prefix)
		}
	}

	if cfg.Git.Enabled {
		gitb := backup.NewGitBackup(backup.GitConfig{
			RemoteURL: cfg.Git.RemoteURL,
			Branch:    cfg.Git.Branch,
		})
		providers = append(providers, gitb)
		slog.Info("git backup enabled", "remote", cfg.Git.RemoteURL)
	}

	return providers
}

// memorySaverAdapter adapts memoryuc.Service to the conversationuc.MemorySaver interface,
// allowing the conversation service to create memories without a direct use case dependency.
type memorySaverAdapter struct {
	svc memoryuc.Service
}

// CreateFromConversation saves conversation content as a new memory record.
func (a *memorySaverAdapter) CreateFromConversation(ctx context.Context, req conversationuc.CreateMemoryRequest) (*domain.Memory, error) {
	return a.svc.Save(ctx, memoryuc.SaveRequest{
		Store:     req.Store,
		Title:     req.Title,
		Content:   req.Content,
		Tags:      req.Tags,
		Source:    domain.MemorySourceConversation,
		SourceRef: req.ConversationID,
		Principal: req.Principal,
	})
}

// storeAccessImpl adapts storeuc.Repository to the memoryuc.StoreAccess interface.
// It lets the memory service check store membership without depending on the store service.
type storeAccessImpl struct {
	repo storeuc.Repository
}

// GetMemberRole returns the principal's role in the named store.
// Returns an empty StoreRole if the principal is not a member.
func (a *storeAccessImpl) GetMemberRole(ctx context.Context, storeName, principalID string) (domain.StoreRole, error) {
	s, err := a.repo.Get(ctx, storeName)
	if err != nil {
		return "", err
	}
	return s.MemberRole(principalID), nil
}

// systemService is a minimal implementation of rest.SystemService for M0/M1.
type systemService struct {
	cfg       *config.Config
	startedAt time.Time
}

// Health returns the current server health status.
func (s *systemService) Health() *rest.HealthStatus {
	return &rest.HealthStatus{
		Status:  "ok",
		Version: version,
	}
}

// buildRouterOpts constructs ServerOptions from config and returns a shutdown
// function for any OTel provider that was initialised. The shutdown function is
// always non-nil and safe to call even when OTel is disabled.
func buildRouterOpts(cfg *config.Config) (rest.ServerOptions, func(context.Context) error) {
	opts := rest.ServerOptions{}
	shutdown := func(_ context.Context) error { return nil }

	if cfg.Telemetry.OTelEndpoint != "" || cfg.Telemetry.ServiceName != "" {
		otelCfg := apimw.OTelConfig{
			Endpoint:    cfg.Telemetry.OTelEndpoint,
			ServiceName: cfg.Telemetry.ServiceName,
		}
		provider, err := apimw.NewOTelProvider(otelCfg)
		if err != nil {
			slog.Warn("failed to initialise OTel provider — tracing disabled", "error", err)
		} else {
			opts.OTelTracer = provider.Tracer("ingatan")
			shutdown = provider.Shutdown
			slog.Info("OTel tracing enabled",
				"endpoint", cfg.Telemetry.OTelEndpoint,
				"service", cfg.Telemetry.ServiceName)
		}
	}

	if cfg.RateLimit.RequestsPerMinute > 0 {
		opts.RateLimitRPS = float64(cfg.RateLimit.RequestsPerMinute) / 60.0
		opts.RateLimitBurst = cfg.RateLimit.BurstSize
		slog.Info("rate limiting enabled",
			"rps", opts.RateLimitRPS,
			"burst", opts.RateLimitBurst)
	}

	return opts, shutdown
}

// expandHome replaces a leading ~ with the user's home directory.
func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[2:])
	}
	return path
}
