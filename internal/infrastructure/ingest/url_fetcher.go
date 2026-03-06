package ingest

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	readability "github.com/go-shiori/go-readability"

	"github.com/stainedhead/ingatan/internal/domain"
)

const defaultMaxBytes = 10 * 1024 * 1024 // 10MB

// HTTPURLFetcher fetches and extracts readable text content from HTTP/HTTPS URLs.
type HTTPURLFetcher struct {
	client   *http.Client
	maxBytes int
}

// NewHTTPURLFetcher creates a new HTTPURLFetcher with the given maximum response size.
// If maxBytes is <= 0, it defaults to 10MB.
func NewHTTPURLFetcher(maxBytes int) *HTTPURLFetcher {
	if maxBytes <= 0 {
		maxBytes = defaultMaxBytes
	}
	return &HTTPURLFetcher{
		client:   &http.Client{Timeout: 30 * time.Second},
		maxBytes: maxBytes,
	}
}

// Fetch retrieves content from the given URL. For HTML pages, it uses readability
// to extract the main text content and title. For other text types, the raw content
// is returned with the URL as the title.
func (f *HTTPURLFetcher) Fetch(ctx context.Context, rawURL string) (title, content string, err error) {
	if rawURL == "" {
		return "", "", domain.NewAppError(domain.ErrCodeInvalidRequest, "URL must not be empty")
	}

	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return "", "", domain.NewAppError(domain.ErrCodeInvalidRequest, fmt.Sprintf("invalid URL: %s", err))
	}

	scheme := strings.ToLower(parsedURL.Scheme)
	if scheme != "http" && scheme != "https" {
		return "", "", domain.NewAppError(domain.ErrCodeInvalidRequest, fmt.Sprintf("unsupported URL scheme: %s", scheme))
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", "", domain.NewAppError(domain.ErrCodeInternalError, fmt.Sprintf("failed to create request: %s", err))
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return "", "", domain.NewAppError(domain.ErrCodeInternalError, fmt.Sprintf("fetch failed: %s", err))
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", "", domain.NewAppError(domain.ErrCodeInternalError, fmt.Sprintf("fetch failed: HTTP %d", resp.StatusCode))
	}

	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, int64(f.maxBytes+1)))
	if err != nil {
		return "", "", domain.NewAppError(domain.ErrCodeInternalError, fmt.Sprintf("fetch failed: %s", err))
	}

	if len(bodyBytes) > f.maxBytes {
		return "", "", domain.NewAppError(domain.ErrCodeContentTooLarge, "URL content exceeds maximum size")
	}

	contentType := resp.Header.Get("Content-Type")
	if strings.HasPrefix(contentType, "text/html") {
		article, readErr := readability.FromReader(bytes.NewReader(bodyBytes), parsedURL)
		if readErr != nil {
			return rawURL, string(bodyBytes), nil
		}
		return article.Title, article.TextContent, nil
	}

	return rawURL, string(bodyBytes), nil
}
