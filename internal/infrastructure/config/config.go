// Package config provides configuration loading for the ingatan server.
// Configuration is loaded from an optional YAML file and overridden by
// environment variables prefixed with INGATAN_ (e.g. INGATAN_SERVER_PORT=9000).
package config

import (
	"strings"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/confmap"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

const envPrefix = "INGATAN_"

// Config is the top-level server configuration.
type Config struct {
	Server       ServerConfig
	DataDir      string `koanf:"data_dir"`
	Embedding    EmbeddingConfig
	LLM          LLMConfig
	Chunking     ChunkingConfig
	Auth         AuthConfig
	Ingest       IngestConfig
	Conversation ConversationConfig
	RateLimit    RateLimitConfig `koanf:"rate_limit"`
	Backup       BackupConfig
	Telemetry    TelemetryConfig
}

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	Host string
	Port int
	TLS  TLSConfig
}

// TLSConfig holds TLS certificate and optional mTLS settings.
type TLSConfig struct {
	CertFile   string `koanf:"cert_file"`
	KeyFile    string `koanf:"key_file"`
	ClientCA   string `koanf:"client_ca"`
	MinVersion string `koanf:"min_version"`
}

// AuthConfig holds JWT validation settings.
type AuthConfig struct {
	Algorithm string
	Secret    string // HS256 — load from env only, never from config file.
	PublicKey string `koanf:"public_key"` // RS256 — path to PEM file.
	Issuer    string
	TokenTTL  string `koanf:"token_ttl"` // Duration string for issued JWT lifetime (e.g. "24h"). Default: "24h".
}

// EmbeddingConfig holds embedding provider settings.
type EmbeddingConfig struct {
	Provider   string
	Model      string
	Dimensions int
	APIKey     string `koanf:"api_key"`
	BaseURL    string `koanf:"base_url"`
}

// LLMConfig holds LLM provider settings for summarization.
type LLMConfig struct {
	Provider string
	Model    string
	APIKey   string `koanf:"api_key"`
	BaseURL  string `koanf:"base_url"`
}

// ChunkingConfig holds text chunking parameters.
type ChunkingConfig struct {
	Strategy        string
	ChunkSize       int `koanf:"chunk_size"`
	ChunkOverlap    int `koanf:"chunk_overlap"`
	MaxContentBytes int `koanf:"max_content_bytes"`
}

// IngestConfig holds file and URL ingestion settings.
type IngestConfig struct {
	AllowedPaths        []string `koanf:"allowed_paths"`
	MaxContentBytes     int      `koanf:"max_content_bytes"`
	RobotsTxtCompliance bool     `koanf:"robots_txt_compliance"`
}

// ConversationConfig holds auto-summarization thresholds.
type ConversationConfig struct {
	AutoSummarizeMessageThreshold       int `koanf:"auto_summarize_message_threshold"`
	AutoSummarizeTokenEstimateThreshold int `koanf:"auto_summarize_token_estimate_threshold"`
}

// RateLimitConfig holds rate limiting settings.
type RateLimitConfig struct {
	RequestsPerMinute int `koanf:"requests_per_minute"`
	BurstSize         int `koanf:"burst_size"`
}

// BackupConfig holds S3 and git backup settings.
type BackupConfig struct {
	S3  S3BackupConfig
	Git GitBackupConfig
}

// S3BackupConfig holds S3-compatible backup settings.
type S3BackupConfig struct {
	Enabled  bool
	Bucket   string
	Region   string
	Prefix   string
	Schedule string
}

// GitBackupConfig holds git-based backup settings.
type GitBackupConfig struct {
	Enabled   bool
	RemoteURL string `koanf:"remote_url"`
	Branch    string
	Schedule  string
}

// TelemetryConfig holds OpenTelemetry settings.
type TelemetryConfig struct {
	OTelEndpoint string `koanf:"otel_endpoint"`
	ServiceName  string `koanf:"service_name"`
}

// configDefaults returns default configuration values keyed by koanf dot-path.
func configDefaults() map[string]any {
	return map[string]any{
		"auth.token_ttl":                                "24h",
		"server.host":                                   "0.0.0.0",
		"server.port":                                   8443,
		"server.tls.min_version":                        "1.2",
		"data_dir":                                      "~/.ingatan/data",
		"chunking.strategy":                             "recursive",
		"chunking.chunk_size":                           512,
		"chunking.chunk_overlap":                        64,
		"chunking.max_content_bytes":                    10485760,
		"ingest.max_content_bytes":                      10485760,
		"ingest.robots_txt_compliance":                  true,
		"conversation.auto_summarize_message_threshold": 50,
		"conversation.auto_summarize_token_estimate_threshold": 8000,
		"rate_limit.requests_per_minute":                       300,
		"rate_limit.burst_size":                                50,
		"telemetry.service_name":                               "ingatan",
	}
}

// envTransform converts an INGATAN_ prefixed env var name to a koanf dot-path.
// INGATAN_SERVER_PORT → server.port
func envTransform(s string) string {
	s = strings.TrimPrefix(s, envPrefix)
	s = strings.ToLower(s)
	return strings.ReplaceAll(s, "_", ".")
}

// LoadConfig loads configuration from an optional YAML file at path and
// overrides with INGATAN_ prefixed environment variables.
// If path is empty, only defaults and environment variables are used.
func LoadConfig(path string) (*Config, error) {
	k := koanf.New(".")

	// Load defaults first.
	if err := k.Load(confmap.Provider(configDefaults(), "."), nil); err != nil {
		return nil, err
	}

	// Load YAML file if provided, overriding defaults.
	if path != "" {
		if err := k.Load(file.Provider(path), yaml.Parser()); err != nil {
			return nil, err
		}
	}

	// Override from environment variables (highest precedence).
	if err := k.Load(env.Provider(envPrefix, ".", envTransform), nil); err != nil {
		return nil, err
	}

	cfg := &Config{}
	if err := k.UnmarshalWithConf("", cfg, koanf.UnmarshalConf{Tag: "koanf"}); err != nil {
		return nil, err
	}
	return cfg, nil
}
