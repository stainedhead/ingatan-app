package ingest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/stainedhead/ingatan/internal/domain"
)

func TestHTTPURLFetcher_HTMLContent(t *testing.T) {
	html := `<!DOCTYPE html>
<html><head><title>Test Page</title></head>
<body>
<h1>Hello World</h1>
<p>This is a test paragraph with meaningful content for readability extraction.</p>
<p>Another paragraph to ensure enough content for the readability algorithm.</p>
</body></html>`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(html))
	}))
	defer srv.Close()

	fetcher := NewHTTPURLFetcher(0)
	title, content, err := fetcher.Fetch(context.Background(), srv.URL)

	require.NoError(t, err)
	// Readability may or may not extract a title depending on content length,
	// but content should contain the text from the page.
	assert.NotEmpty(t, content)
	_ = title // title may be extracted or may fall back to URL
}

func TestHTTPURLFetcher_PlainText(t *testing.T) {
	text := "Hello, this is plain text content."

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte(text))
	}))
	defer srv.Close()

	fetcher := NewHTTPURLFetcher(0)
	title, content, err := fetcher.Fetch(context.Background(), srv.URL)

	require.NoError(t, err)
	assert.Equal(t, srv.URL, title)
	assert.Equal(t, text, content)
}

func TestHTTPURLFetcher_ContentTooLarge(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte(strings.Repeat("x", 200)))
	}))
	defer srv.Close()

	fetcher := NewHTTPURLFetcher(100)
	_, _, err := fetcher.Fetch(context.Background(), srv.URL)

	require.Error(t, err)
	var appErr *domain.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, domain.ErrCodeContentTooLarge, appErr.Code)
}

func TestHTTPURLFetcher_InvalidScheme(t *testing.T) {
	fetcher := NewHTTPURLFetcher(0)
	_, _, err := fetcher.Fetch(context.Background(), "ftp://example.com/file.txt")

	require.Error(t, err)
	var appErr *domain.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, domain.ErrCodeInvalidRequest, appErr.Code)
	assert.Contains(t, appErr.Message, "unsupported URL scheme")
}
