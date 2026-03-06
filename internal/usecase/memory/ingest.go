package memory

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/stainedhead/ingatan/internal/domain"
)

// SaveURL fetches the content of a URL, saves it as a memory, and returns the memory.
func (s *serviceImpl) SaveURL(ctx context.Context, req SaveURLRequest) (*domain.Memory, error) {
	if req.Store == "" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "store must not be empty")
	}
	if req.URL == "" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "url must not be empty")
	}
	if s.urlFetcher == nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "URL ingest is not configured")
	}

	title, content, err := s.urlFetcher.Fetch(ctx, req.URL)
	if err != nil {
		return nil, fmt.Errorf("fetch URL: %w", err)
	}
	if content == "" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "URL returned empty content")
	}

	return s.Save(ctx, SaveRequest{
		Store:     req.Store,
		Title:     title,
		Content:   content,
		Tags:      req.Tags,
		Source:    domain.MemorySourceURL,
		SourceURL: req.URL,
		Metadata:  req.Metadata,
		Principal: req.Principal,
	})
}

// SaveFile reads a local file, saves its content as a memory, and returns the memory.
func (s *serviceImpl) SaveFile(ctx context.Context, req SaveFileRequest) (*domain.Memory, error) {
	if req.Store == "" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "store must not be empty")
	}
	if req.FilePath == "" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "file_path must not be empty")
	}
	if s.fileReader == nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "file ingest is not configured")
	}

	// Path traversal guard — check before Clean() resolves the segments.
	if strings.Contains(req.FilePath, "..") {
		return nil, domain.NewAppError(domain.ErrCodePathNotAllowed, "path traversal not allowed")
	}
	cleanPath := filepath.Clean(req.FilePath)

	// Allowed paths enforcement.
	if len(s.ingestOpts.AllowedPaths) > 0 {
		allowed := false
		for _, prefix := range s.ingestOpts.AllowedPaths {
			if strings.HasPrefix(cleanPath, filepath.Clean(prefix)) {
				allowed = true
				break
			}
		}
		if !allowed {
			return nil, domain.NewAppError(domain.ErrCodePathNotAllowed, "file path is not in an allowed directory")
		}
	}

	if !s.fileReader.Supports(cleanPath) {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "unsupported file type")
	}

	content, err := s.fileReader.Read(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}
	if content == "" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "file returned empty content")
	}

	title := filepath.Base(cleanPath)

	return s.Save(ctx, SaveRequest{
		Store:      req.Store,
		Title:      title,
		Content:    content,
		Tags:       req.Tags,
		Source:     domain.MemorySourceFile,
		SourcePath: cleanPath,
		Metadata:   req.Metadata,
		Principal:  req.Principal,
	})
}
