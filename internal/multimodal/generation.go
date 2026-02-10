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
	Size    string `json:"size,omitempty"`     // 1024x1024, 1792x1024, 1024x1792
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

// AudioTranscriber converts audio to text using Whisper or similar models.
type AudioTranscriber struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
}

func NewAudioTranscriber(apiKey string) *AudioTranscriber {
	return &AudioTranscriber{
		apiKey:  apiKey,
		baseURL: "https://api.openai.com/v1",
		httpClient: &http.Client{
			Timeout: 300 * time.Second,
		},
	}
}

// TranscriptionRequest holds the parameters for audio transcription.
type TranscriptionRequest struct {
	FilePath string `json:"file_path"`
	Model    string `json:"model,omitempty"`    // whisper-1
	Language string `json:"language,omitempty"` // ISO-639-1 code
	Prompt   string `json:"prompt,omitempty"`   // optional context hint
}

// TranscriptionResponse holds the transcription result.
type TranscriptionResponse struct {
	Text     string  `json:"text"`
	Language string  `json:"language"`
	Duration float64 `json:"duration"`
}

// TextToSpeech converts text to audio using TTS models.
type TextToSpeech struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
}

func NewTextToSpeech(apiKey string) *TextToSpeech {
	return &TextToSpeech{
		apiKey:  apiKey,
		baseURL: "https://api.openai.com/v1",
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

// TTSRequest holds the parameters for text-to-speech generation.
type TTSRequest struct {
	Input string `json:"input"`
	Model string `json:"model,omitempty"` // tts-1, tts-1-hd
	Voice string `json:"voice,omitempty"` // alloy, echo, fable, onyx, nova, shimmer
	Speed float64 `json:"speed,omitempty"` // 0.25 to 4.0
}

// Synthesize converts text to audio and returns the audio bytes.
func (t *TextToSpeech) Synthesize(ctx context.Context, req TTSRequest) ([]byte, error) {
	if req.Model == "" {
		req.Model = "tts-1"
	}
	if req.Voice == "" {
		req.Voice = "alloy"
	}

	body := map[string]any{
		"model": req.Model,
		"input": req.Input,
		"voice": req.Voice,
	}
	if req.Speed > 0 {
		body["speed"] = req.Speed
	}

	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", t.baseURL+"/audio/speech", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+t.apiKey)

	resp, err := t.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("tts request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("tts failed (status %d): %s", resp.StatusCode, string(respBody))
	}

	return io.ReadAll(resp.Body)
}
