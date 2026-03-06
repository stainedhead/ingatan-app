package ingest

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/stainedhead/ingatan/internal/domain"
)

func TestPDFExtractor_FileNotFound(t *testing.T) {
	extractor := NewPDFExtractor(0)
	_, err := extractor.Extract("/nonexistent/path/file.pdf")

	require.Error(t, err)
	var appErr *domain.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, domain.ErrCodePDFExtractionError, appErr.Code)
}

func TestPDFExtractor_InvalidFile(t *testing.T) {
	tmpDir := t.TempDir()
	fakePDF := filepath.Join(tmpDir, "garbage.pdf")
	err := os.WriteFile(fakePDF, []byte("this is not a real pdf file at all"), 0o644)
	require.NoError(t, err)

	extractor := NewPDFExtractor(0)
	_, extractErr := extractor.Extract(fakePDF)

	// Should return an error without panicking.
	require.Error(t, extractErr)
	var appErr *domain.AppError
	require.ErrorAs(t, extractErr, &appErr)
	assert.Equal(t, domain.ErrCodePDFExtractionError, appErr.Code)
}
