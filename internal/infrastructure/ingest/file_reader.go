package ingest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/net/html"

	"github.com/stainedhead/ingatan/internal/domain"
)

// supportedExtensions lists file extensions that MultiFileReader can process.
var supportedExtensions = map[string]bool{
	".md": true, ".txt": true, ".html": true, ".htm": true,
	".json": true, ".yaml": true, ".yml": true,
	".go": true, ".py": true, ".ts": true, ".js": true,
	".rs": true, ".sh": true, ".toml": true, ".xml": true,
	".csv": true, ".pdf": true,
}

// MultiFileReader reads text content from files of various supported types.
type MultiFileReader struct {
	maxBytes     int
	pdfExtractor *PDFExtractor
}

// NewMultiFileReader creates a new MultiFileReader with the given maximum file size
// and PDF extractor. If maxBytes is <= 0, it defaults to 10MB.
func NewMultiFileReader(maxBytes int, pdfExtractor *PDFExtractor) *MultiFileReader {
	if maxBytes <= 0 {
		maxBytes = defaultMaxBytes
	}
	return &MultiFileReader{
		maxBytes:     maxBytes,
		pdfExtractor: pdfExtractor,
	}
}

// Supports returns true if the file extension is in the supported set.
func (r *MultiFileReader) Supports(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return supportedExtensions[ext]
}

// Read reads a file and returns its text content. HTML files have tags stripped.
// PDF files are delegated to the PDF extractor.
func (r *MultiFileReader) Read(path string) (string, error) {
	ext := strings.ToLower(filepath.Ext(path))
	if !supportedExtensions[ext] {
		return "", domain.NewAppError(domain.ErrCodeInvalidRequest, "unsupported file type: "+ext)
	}

	if ext == ".pdf" {
		return r.pdfExtractor.Extract(path)
	}

	info, err := os.Stat(path)
	if err != nil {
		return "", domain.NewAppError(domain.ErrCodeInternalError, fmt.Sprintf("cannot stat file: %s", err))
	}
	if info.Size() > int64(r.maxBytes) {
		return "", domain.NewAppError(domain.ErrCodeContentTooLarge, "file exceeds maximum size")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return "", domain.NewAppError(domain.ErrCodeInternalError, fmt.Sprintf("cannot read file: %s", err))
	}

	if ext == ".html" || ext == ".htm" {
		return extractTextFromHTML(string(data)), nil
	}

	return string(data), nil
}

// extractTextFromHTML parses HTML and returns only the text content.
func extractTextFromHTML(raw string) string {
	doc, err := html.Parse(strings.NewReader(raw))
	if err != nil {
		return raw
	}

	var buf strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			text := strings.TrimSpace(n.Data)
			if text != "" {
				buf.WriteString(text)
				buf.WriteString(" ")
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)

	return strings.TrimSpace(buf.String())
}
