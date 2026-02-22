package multimodal

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ImageGenerator creates images from text prompts using AI models (DALL-E, etc.).
type ImageGenerator struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
}

func NewImageGenerator(apiKey string) *ImageGenerator {
	return &ImageGenerator{
		apiKey:  apiKey,
		baseURL: "https://api.openai.com/v1",
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

// ImageGenRequest holds the parameters for image generation.
type ImageGenRequest struct {
	Prompt  string `json:"prompt"`
	Model   string `json:"model,omitempty"`   // dall-e-3, dall-e-2
	Size    string `json:"size,omitempty"`    // 1024x1024, 1792x1024, 1024x1792
	Quality string `json:"quality,omitempty"` // standard, hd
	N       int    `json:"n,omitempty"`       // number of images
	Style   string `json:"style,omitempty"`   // vivid, natural
}

// GeneratedImage holds a generated image result.
type GeneratedImage struct {
	URL           string `json:"url,omitempty"`
	Base64        string `json:"b64_json,omitempty"`
	RevisedPrompt string `json:"revised_prompt,omitempty"`
}

// ImageGenResponse holds the complete generation response.
type ImageGenResponse struct {
	Images []GeneratedImage `json:"images"`
	Model  string           `json:"model"`
}

// Generate creates images from a text prompt.
func (g *ImageGenerator) Generate(ctx context.Context, req ImageGenRequest) (*ImageGenResponse, error) {
	if req.Model == "" {
		req.Model = "dall-e-3"
	}
	if req.Size == "" {
		req.Size = "1024x1024"
	}
	if req.N <= 0 {
		req.N = 1
	}

	body := map[string]any{
		"model":  req.Model,
		"prompt": req.Prompt,
		"size":   req.Size,
		"n":      req.N,
	}
	if req.Quality != "" {
		body["quality"] = req.Quality
	}
	if req.Style != "" {
		body["style"] = req.Style
	}

	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", g.baseURL+"/images/generations", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+g.apiKey)

	resp, err := g.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("image generation request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("image generation failed (status %d): %s", resp.StatusCode, string(respBody))
	}

	var apiResp struct {
		Data []GeneratedImage `json:"data"`
	}
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	return &ImageGenResponse{
		Images: apiResp.Data,
		Model:  req.Model,
	}, nil
}
