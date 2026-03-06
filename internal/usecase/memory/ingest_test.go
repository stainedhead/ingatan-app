package memory

import (
	"context"
	"fmt"
	"testing"

	"github.com/stainedhead/ingatan/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Mock implementations for ingest ---

type mockURLFetcher struct {
	title   string
	content string
	err     error
}

func (m *mockURLFetcher) Fetch(_ context.Context, _ string) (string, string, error) {
	return m.title, m.content, m.err
}

type mockFileReader struct {
	content   string
	err       error
	supported bool
}

func (m *mockFileReader) Read(_ string) (string, error) { return m.content, m.err }
func (m *mockFileReader) Supports(_ string) bool        { return m.supported }

// --- Helpers ---

func buildIngestService(fetcher URLFetcher, reader FileReader, opts IngestOptions) Service {
	return NewService(newMockRepo(), newMockChunkRepo(), &mockChunker{}, &mockEmbedder{}, nil, nil, fetcher, reader, opts, nil)
}

// --- SaveURL Tests ---

func TestServiceSaveURL_Success(t *testing.T) {
	fetcher := &mockURLFetcher{title: "Example Page", content: "Hello from the web."}
	svc := buildIngestService(fetcher, nil, IngestOptions{})

	mem, err := svc.SaveURL(context.Background(), SaveURLRequest{
		Store:     "default",
		URL:       "https://example.com/page",
		Tags:      []string{"web"},
		Principal: principal(),
	})

	require.NoError(t, err)
	require.NotNil(t, mem)
	assert.Equal(t, "Example Page", mem.Title)
	assert.Equal(t, "Hello from the web.", mem.Content)
	assert.Equal(t, domain.MemorySourceURL, mem.Source)
	assert.Equal(t, "https://example.com/page", mem.SourceURL)
	assert.Equal(t, []string{"web"}, mem.Tags)
}

func TestServiceSaveURL_NoFetcher(t *testing.T) {
	svc := buildIngestService(nil, nil, IngestOptions{})

	_, err := svc.SaveURL(context.Background(), SaveURLRequest{
		Store: "default",
		URL:   "https://example.com",
	})

	require.Error(t, err)
	var appErr *domain.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, domain.ErrCodeInternalError, appErr.Code)
}

func TestServiceSaveURL_EmptyStore(t *testing.T) {
	svc := buildIngestService(&mockURLFetcher{}, nil, IngestOptions{})

	_, err := svc.SaveURL(context.Background(), SaveURLRequest{
		Store: "",
		URL:   "https://example.com",
	})

	require.Error(t, err)
	var appErr *domain.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, domain.ErrCodeInvalidRequest, appErr.Code)
}

func TestServiceSaveURL_EmptyURL(t *testing.T) {
	svc := buildIngestService(&mockURLFetcher{}, nil, IngestOptions{})

	_, err := svc.SaveURL(context.Background(), SaveURLRequest{
		Store: "default",
		URL:   "",
	})

	require.Error(t, err)
	var appErr *domain.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, domain.ErrCodeInvalidRequest, appErr.Code)
}

func TestServiceSaveURL_FetchError(t *testing.T) {
	fetcher := &mockURLFetcher{err: fmt.Errorf("network error")}
	svc := buildIngestService(fetcher, nil, IngestOptions{})

	_, err := svc.SaveURL(context.Background(), SaveURLRequest{
		Store: "default",
		URL:   "https://example.com",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "fetch URL")
}

func TestServiceSaveURL_EmptyContent(t *testing.T) {
	fetcher := &mockURLFetcher{title: "Empty Page", content: ""}
	svc := buildIngestService(fetcher, nil, IngestOptions{})

	_, err := svc.SaveURL(context.Background(), SaveURLRequest{
		Store: "default",
		URL:   "https://example.com",
	})

	require.Error(t, err)
	var appErr *domain.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, domain.ErrCodeInvalidRequest, appErr.Code)
}

// --- SaveFile Tests ---

func TestServiceSaveFile_Success(t *testing.T) {
	reader := &mockFileReader{content: "File content here.", supported: true}
	svc := buildIngestService(nil, reader, IngestOptions{})

	mem, err := svc.SaveFile(context.Background(), SaveFileRequest{
		Store:     "default",
		FilePath:  "/safe/docs/readme.txt",
		Tags:      []string{"doc"},
		Principal: principal(),
	})

	require.NoError(t, err)
	require.NotNil(t, mem)
	assert.Equal(t, "readme.txt", mem.Title)
	assert.Equal(t, "File content here.", mem.Content)
	assert.Equal(t, domain.MemorySourceFile, mem.Source)
	assert.Equal(t, "/safe/docs/readme.txt", mem.SourcePath)
	assert.Equal(t, []string{"doc"}, mem.Tags)
}

func TestServiceSaveFile_NoReader(t *testing.T) {
	svc := buildIngestService(nil, nil, IngestOptions{})

	_, err := svc.SaveFile(context.Background(), SaveFileRequest{
		Store:    "default",
		FilePath: "/tmp/test.txt",
	})

	require.Error(t, err)
	var appErr *domain.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, domain.ErrCodeInternalError, appErr.Code)
}

func TestServiceSaveFile_EmptyStore(t *testing.T) {
	svc := buildIngestService(nil, &mockFileReader{supported: true}, IngestOptions{})

	_, err := svc.SaveFile(context.Background(), SaveFileRequest{
		Store:    "",
		FilePath: "/tmp/test.txt",
	})

	require.Error(t, err)
	var appErr *domain.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, domain.ErrCodeInvalidRequest, appErr.Code)
}

func TestServiceSaveFile_PathTraversal(t *testing.T) {
	reader := &mockFileReader{content: "content", supported: true}
	svc := buildIngestService(nil, reader, IngestOptions{})

	_, err := svc.SaveFile(context.Background(), SaveFileRequest{
		Store:    "default",
		FilePath: "/safe/docs/../../etc/passwd",
	})

	require.Error(t, err)
	var appErr *domain.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, domain.ErrCodePathNotAllowed, appErr.Code)
}

func TestServiceSaveFile_NotAllowedPath(t *testing.T) {
	reader := &mockFileReader{content: "content", supported: true}
	svc := buildIngestService(nil, reader, IngestOptions{
		AllowedPaths: []string{"/safe"},
	})

	_, err := svc.SaveFile(context.Background(), SaveFileRequest{
		Store:    "default",
		FilePath: "/tmp/x.txt",
	})

	require.Error(t, err)
	var appErr *domain.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, domain.ErrCodePathNotAllowed, appErr.Code)
}

func TestServiceSaveFile_UnsupportedType(t *testing.T) {
	reader := &mockFileReader{content: "content", supported: false}
	svc := buildIngestService(nil, reader, IngestOptions{})

	_, err := svc.SaveFile(context.Background(), SaveFileRequest{
		Store:    "default",
		FilePath: "/tmp/test.xyz",
	})

	require.Error(t, err)
	var appErr *domain.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, domain.ErrCodeInvalidRequest, appErr.Code)
}

func TestServiceSaveFile_EmptyContent(t *testing.T) {
	reader := &mockFileReader{content: "", supported: true}
	svc := buildIngestService(nil, reader, IngestOptions{})

	_, err := svc.SaveFile(context.Background(), SaveFileRequest{
		Store:    "default",
		FilePath: "/tmp/empty.txt",
	})

	require.Error(t, err)
	var appErr *domain.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, domain.ErrCodeInvalidRequest, appErr.Code)
}
