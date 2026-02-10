package document

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

type OCRService struct {
	tesseractPath string
}

func NewOCRService() *OCRService {
	path, _ := exec.LookPath("tesseract")
	if path == "" {
		path = "tesseract"
	}
	return &OCRService{tesseractPath: path}
}

func (o *OCRService) IsAvailable() bool {
	cmd := exec.Command(o.tesseractPath, "--version")
	return cmd.Run() == nil
}

func (o *OCRService) ExtractText(ctx context.Context, imagePath string) (string, error) {
	cmd := exec.CommandContext(ctx, o.tesseractPath, imagePath, "stdout", "-l", "eng")

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("tesseract OCR: %w", err)
	}

	return strings.TrimSpace(string(output)), nil
}
