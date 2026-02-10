package document

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/nikhilbhutani/backendwithai/pkg/textextract"
)

type TextExtractor interface {
	Extract(ctx context.Context, data io.ReaderAt, size int64, fileType string) (*textextract.ExtractedText, error)
	SupportedTypes() []string
}

type extractor struct {
	ocr *OCRService
}

func NewTextExtractor() TextExtractor {
	return &extractor{
		ocr: NewOCRService(),
	}
}

func (e *extractor) Extract(ctx context.Context, data io.ReaderAt, size int64, fileType string) (*textextract.ExtractedText, error) {
	result, err := textextract.Extract(data, size, fileType)
	if err != nil {
		return nil, fmt.Errorf("extract text: %w", err)
	}

	// If extracted text is empty/minimal, try OCR for PDFs
	if len(result.Content) < 50 && (fileType == ".pdf" || fileType == "pdf") && e.ocr.IsAvailable() {
		// For scanned PDFs, we'd need to convert pages to images first.
		// This is a placeholder for the full OCR pipeline.
		_ = ctx
	}

	return result, nil
}

func (e *extractor) SupportedTypes() []string {
	return textextract.SupportedTypes()
}

// ReaderAtFromBytes creates an io.ReaderAt from a byte slice.
func ReaderAtFromBytes(data []byte) *bytes.Reader {
	return bytes.NewReader(data)
}
