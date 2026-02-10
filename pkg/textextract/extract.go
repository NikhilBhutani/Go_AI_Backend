package textextract

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/ledongthuc/pdf"
)

type ExtractedText struct {
	Content  string
	Pages    int
	Metadata map[string]string
}

func Extract(data io.ReaderAt, size int64, fileType string) (*ExtractedText, error) {
	switch strings.ToLower(fileType) {
	case ".pdf", "pdf", "application/pdf":
		return extractPDF(data, size)
	case ".docx", "docx", "application/vnd.openxmlformats-officedocument.wordprocessingml.document":
		return extractDOCX(data, size)
	case ".txt", "txt", "text/plain":
		return extractTXT(data, size)
	default:
		return nil, fmt.Errorf("unsupported file type: %s", fileType)
	}
}

func SupportedTypes() []string {
	return []string{".pdf", ".docx", ".txt"}
}

func extractPDF(data io.ReaderAt, size int64) (*ExtractedText, error) {
	reader, err := pdf.NewReader(data, size)
	if err != nil {
		return nil, fmt.Errorf("open PDF: %w", err)
	}

	var buf strings.Builder
	numPages := reader.NumPage()

	for i := 1; i <= numPages; i++ {
		page := reader.Page(i)
		if page.V.IsNull() {
			continue
		}
		text, err := page.GetPlainText(nil)
		if err != nil {
			continue
		}
		buf.WriteString(text)
		buf.WriteString("\n")
	}

	return &ExtractedText{
		Content: buf.String(),
		Pages:   numPages,
		Metadata: map[string]string{
			"type": "pdf",
		},
	}, nil
}

func extractDOCX(data io.ReaderAt, size int64) (*ExtractedText, error) {
	reader, err := zip.NewReader(data, size)
	if err != nil {
		return nil, fmt.Errorf("open DOCX: %w", err)
	}

	var buf strings.Builder
	for _, f := range reader.File {
		if filepath.Base(f.Name) == "document.xml" {
			rc, err := f.Open()
			if err != nil {
				return nil, fmt.Errorf("open document.xml: %w", err)
			}
			defer rc.Close()

			content, err := io.ReadAll(rc)
			if err != nil {
				return nil, fmt.Errorf("read document.xml: %w", err)
			}

			// Simple XML text extraction — strip tags
			text := stripXMLTags(string(content))
			buf.WriteString(text)
			break
		}
	}

	return &ExtractedText{
		Content: buf.String(),
		Pages:   1,
		Metadata: map[string]string{
			"type": "docx",
		},
	}, nil
}

func extractTXT(data io.ReaderAt, size int64) (*ExtractedText, error) {
	buf := make([]byte, size)
	_, err := data.ReadAt(buf, 0)
	if err != nil && err != io.EOF {
		return nil, fmt.Errorf("read TXT: %w", err)
	}

	return &ExtractedText{
		Content: string(bytes.TrimSpace(buf)),
		Pages:   1,
		Metadata: map[string]string{
			"type": "txt",
		},
	}, nil
}

func stripXMLTags(s string) string {
	var result strings.Builder
	inTag := false
	for _, r := range s {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
			result.WriteRune(' ')
		case !inTag:
			result.WriteRune(r)
		}
	}
	// Collapse whitespace
	text := result.String()
	parts := strings.Fields(text)
	return strings.Join(parts, " ")
}
