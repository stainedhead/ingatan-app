package ingest

import (
	"strings"

	"github.com/ledongthuc/pdf"

	"github.com/stainedhead/ingatan/internal/domain"
)

// PDFExtractor extracts plain text content from PDF files.
type PDFExtractor struct {
	maxBytes int
}

// NewPDFExtractor creates a new PDFExtractor with the given maximum output size.
// If maxBytes is <= 0, it defaults to 10MB.
func NewPDFExtractor(maxBytes int) *PDFExtractor {
	if maxBytes <= 0 {
		maxBytes = defaultMaxBytes
	}
	return &PDFExtractor{maxBytes: maxBytes}
}

// Extract reads a PDF file and returns its plain text content.
// It wraps the extraction in a panic recovery since PDF parsing libraries
// may panic on malformed input.
func (e *PDFExtractor) Extract(path string) (content string, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = domain.NewAppError(domain.ErrCodePDFExtractionError, "PDF extraction panicked")
		}
	}()

	f, r, pdfErr := pdf.Open(path)
	if pdfErr != nil {
		return "", domain.NewAppError(domain.ErrCodePDFExtractionError, "cannot open PDF: "+pdfErr.Error())
	}
	defer func() { _ = f.Close() }()

	var buf strings.Builder
	totalPages := r.NumPage()
	for pageIndex := 1; pageIndex <= totalPages; pageIndex++ {
		p := r.Page(pageIndex)
		if p.V.IsNull() {
			continue
		}
		text, textErr := p.GetPlainText(nil)
		if textErr != nil {
			continue
		}
		buf.WriteString(text)
		buf.WriteString("\n")
		if buf.Len() > e.maxBytes {
			break
		}
	}

	return buf.String(), nil
}
