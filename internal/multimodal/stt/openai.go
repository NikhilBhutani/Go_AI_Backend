package stt

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// OpenAISTTConfig holds configuration for the OpenAI STT backend.
type OpenAISTTConfig struct {
	APIKey  string
	BaseURL string // default: "https://api.openai.com/v1"
	Model   string // default: "whisper-1"
}

// OpenAISTT transcribes audio using OpenAI's Whisper API (or a compatible endpoint).
type OpenAISTT struct {
	cfg        OpenAISTTConfig
	httpClient *http.Client
}

// NewOpenAISTT creates an OpenAISTT with sensible defaults applied.
func NewOpenAISTT(cfg OpenAISTTConfig) *OpenAISTT {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.openai.com/v1"
	}
	if cfg.Model == "" {
		cfg.Model = "whisper-1"
	}
	return &OpenAISTT{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: 300 * time.Second,
		},
	}
}

func (o *OpenAISTT) Name() string { return "openai-whisper" }

// Transcribe sends the audio file to the Whisper API using a proper multipart upload.
func (o *OpenAISTT) Transcribe(ctx context.Context, req TranscriptionRequest) (*TranscriptionResponse, error) {
	f, err := os.Open(req.FilePath)
	if err != nil {
		return nil, fmt.Errorf("open audio file: %w", err)
	}
	defer f.Close()

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)

	// Audio file part
	fw, err := mw.CreateFormFile("file", filepath.Base(req.FilePath))
	if err != nil {
		return nil, fmt.Errorf("create form file: %w", err)
	}
	if _, err = io.Copy(fw, f); err != nil {
		return nil, fmt.Errorf("copy audio data: %w", err)
	}

	// Required fields
	_ = mw.WriteField("model", o.cfg.Model)
	_ = mw.WriteField("response_format", "verbose_json")

	// Optional fields
	if req.Language != "" {
		_ = mw.WriteField("language", req.Language)
	}
	if req.Prompt != "" {
		_ = mw.WriteField("prompt", req.Prompt)
	}

	if err = mw.Close(); err != nil {
		return nil, fmt.Errorf("close multipart writer: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", o.cfg.BaseURL+"/audio/transcriptions", &body)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", mw.FormDataContentType())
	if o.cfg.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+o.cfg.APIKey)
	}

	resp, err := o.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("transcription request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("transcription failed (status %d): %s", resp.StatusCode, string(respBody))
	}

	var apiResp struct {
		Text     string  `json:"text"`
		Language string  `json:"language"`
		Duration float64 `json:"duration"`
	}
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	return &TranscriptionResponse{
		Text:     apiResp.Text,
		Language: apiResp.Language,
		Duration: apiResp.Duration,
	}, nil
}
