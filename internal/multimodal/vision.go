package multimodal

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/nikhilbhutani/backendwithai/internal/llm"
)

// VisionService handles image understanding tasks using vision-capable LLMs.
type VisionService struct {
	gateway llm.Gateway
	model   string // must be a vision-capable model (gpt-4o, claude-3, etc.)
}

func NewVisionService(gw llm.Gateway, model string) *VisionService {
	if model == "" {
		model = "gpt-4o"
	}
	return &VisionService{gateway: gw, model: model}
}

// ImageInput represents an image for vision analysis.
type ImageInput struct {
	// Exactly one of these should be set
	URL      string `json:"url,omitempty"`       // public URL
	Base64   string `json:"base64,omitempty"`     // base64-encoded image data
	FilePath string `json:"file_path,omitempty"`  // local file path
	MimeType string `json:"mime_type,omitempty"`  // image/png, image/jpeg, etc.
}

// VisionRequest holds the input for a vision task.
type VisionRequest struct {
	Images []ImageInput `json:"images"`
	Prompt string       `json:"prompt"`
}

// VisionResponse holds the output from a vision task.
type VisionResponse struct {
	Content     string  `json:"content"`
	Model       string  `json:"model"`
	InputTokens int     `json:"input_tokens"`
	CostUSD     float64 `json:"cost_usd"`
}

// Analyze sends images to a vision model with a prompt.
func (v *VisionService) Analyze(ctx context.Context, req VisionRequest) (*VisionResponse, error) {
	// Build the content parts
	contentParts := []string{}

	for _, img := range req.Images {
		dataURL, err := v.resolveImage(img)
		if err != nil {
			return nil, fmt.Errorf("resolve image: %w", err)
		}
		contentParts = append(contentParts, dataURL)
	}

	// For the gateway, we encode images as markdown-style references in the content.
	// The provider implementations should handle parsing these for their specific API.
	var prompt strings.Builder
	for i, part := range contentParts {
		fmt.Fprintf(&prompt, "[Image %d: %s]\n", i+1, part)
	}
	prompt.WriteString("\n")
	prompt.WriteString(req.Prompt)

	resp, err := v.gateway.Chat(ctx, llm.ChatRequest{
		Model: v.model,
		Messages: []llm.Message{
			{
				Role:    "system",
				Content: "You are a helpful assistant that can analyze images. Describe what you see accurately and thoroughly.",
			},
			{
				Role:    "user",
				Content: prompt.String(),
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("vision analyze: %w", err)
	}

	return &VisionResponse{
		Content:     resp.Content,
		Model:       resp.Model,
		InputTokens: resp.InputTokens,
		CostUSD:     resp.CostUSD,
	}, nil
}

// Describe generates a detailed description of an image.
func (v *VisionService) Describe(ctx context.Context, image ImageInput) (string, error) {
	resp, err := v.Analyze(ctx, VisionRequest{
		Images: []ImageInput{image},
		Prompt: "Describe this image in detail. Include all visible elements, text, colors, and spatial relationships.",
	})
	if err != nil {
		return "", err
	}
	return resp.Content, nil
}

// ExtractText performs OCR-like text extraction from an image using a vision model.
func (v *VisionService) ExtractText(ctx context.Context, image ImageInput) (string, error) {
	resp, err := v.Analyze(ctx, VisionRequest{
		Images: []ImageInput{image},
		Prompt: "Extract ALL text visible in this image. Return only the text content, preserving the original formatting as closely as possible.",
	})
	if err != nil {
		return "", err
	}
	return resp.Content, nil
}

// Compare analyzes multiple images and describes their differences/similarities.
func (v *VisionService) Compare(ctx context.Context, images []ImageInput, aspect string) (string, error) {
	prompt := "Compare these images."
	if aspect != "" {
		prompt = fmt.Sprintf("Compare these images, focusing on: %s", aspect)
	}

	resp, err := v.Analyze(ctx, VisionRequest{
		Images: images,
		Prompt: prompt,
	})
	if err != nil {
		return "", err
	}
	return resp.Content, nil
}

func (v *VisionService) resolveImage(img ImageInput) (string, error) {
	if img.URL != "" {
		return img.URL, nil
	}

	if img.Base64 != "" {
		mimeType := img.MimeType
		if mimeType == "" {
			mimeType = "image/png"
		}
		return fmt.Sprintf("data:%s;base64,%s", mimeType, img.Base64), nil
	}

	if img.FilePath != "" {
		data, err := os.ReadFile(img.FilePath)
		if err != nil {
			return "", fmt.Errorf("read image file: %w", err)
		}

		mimeType := img.MimeType
		if mimeType == "" {
			mimeType = mimeFromExtension(filepath.Ext(img.FilePath))
		}

		encoded := base64.StdEncoding.EncodeToString(data)
		return fmt.Sprintf("data:%s;base64,%s", mimeType, encoded), nil
	}

	return "", fmt.Errorf("image input must have url, base64, or file_path")
}

func mimeFromExtension(ext string) string {
	switch strings.ToLower(ext) {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".svg":
		return "image/svg+xml"
	default:
		return "image/png"
	}
}
