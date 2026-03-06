package ingest

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/stainedhead/ingatan/internal/domain"
)

func TestMultiFileReader_TextFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.txt")
	err := os.WriteFile(path, []byte("hello world"), 0o644)
	require.NoError(t, err)

	reader := NewMultiFileReader(0, NewPDFExtractor(0))
	content, readErr := reader.Read(path)

	require.NoError(t, readErr)
	assert.Equal(t, "hello world", content)
}

func TestMultiFileReader_MarkdownFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "readme.md")
	md := "# Title\n\nSome markdown content."
	err := os.WriteFile(path, []byte(md), 0o644)
	require.NoError(t, err)

	reader := NewMultiFileReader(0, NewPDFExtractor(0))
	content, readErr := reader.Read(path)

	require.NoError(t, readErr)
	assert.Equal(t, md, content)
}

func TestMultiFileReader_JSONFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "data.json")
	jsonData := `{"key": "value"}`
	err := os.WriteFile(path, []byte(jsonData), 0o644)
	require.NoError(t, err)

	reader := NewMultiFileReader(0, NewPDFExtractor(0))
	content, readErr := reader.Read(path)

	require.NoError(t, readErr)
	assert.Equal(t, jsonData, content)
}

func TestMultiFileReader_HTMLFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "page.html")
	htmlContent := `<html><body><h1>Title</h1><p>Paragraph text</p></body></html>`
	err := os.WriteFile(path, []byte(htmlContent), 0o644)
	require.NoError(t, err)

	reader := NewMultiFileReader(0, NewPDFExtractor(0))
	content, readErr := reader.Read(path)

	require.NoError(t, readErr)
	assert.NotContains(t, content, "<h1>")
	assert.NotContains(t, content, "<p>")
	assert.Contains(t, content, "Title")
	assert.Contains(t, content, "Paragraph text")
}

func TestMultiFileReader_UnsupportedExtension(t *testing.T) {
	reader := NewMultiFileReader(0, NewPDFExtractor(0))
	_, err := reader.Read("/tmp/archive.zip")

	require.Error(t, err)
	var appErr *domain.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, domain.ErrCodeInvalidRequest, appErr.Code)
	assert.Contains(t, appErr.Message, "unsupported file type")
}

func TestMultiFileReader_FileTooLarge(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "big.txt")
	err := os.WriteFile(path, make([]byte, 20), 0o644)
	require.NoError(t, err)

	reader := NewMultiFileReader(10, NewPDFExtractor(0))
	_, readErr := reader.Read(path)

	require.Error(t, readErr)
	var appErr *domain.AppError
	require.ErrorAs(t, readErr, &appErr)
	assert.Equal(t, domain.ErrCodeContentTooLarge, appErr.Code)
}

func TestMultiFileReader_FileNotFound(t *testing.T) {
	reader := NewMultiFileReader(0, NewPDFExtractor(0))
	_, err := reader.Read("/nonexistent/path/file.txt")

	require.Error(t, err)
	var appErr *domain.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, domain.ErrCodeInternalError, appErr.Code)
}
