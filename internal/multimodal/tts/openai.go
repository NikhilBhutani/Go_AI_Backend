package tts

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// OpenAITTSConfig holds configuration for the OpenAI TTS backend.
type OpenAITTSConfig struct {
	APIKey  string
	BaseURL string // default: "https://api.openai.com/v1"
	Model   string // default: "tts-1"
}

// OpenAITTS synthesizes speech using OpenAI's TTS API.
type OpenAITTS struct {
	cfg        OpenAITTSConfig
	httpClient *http.Client
}

// NewOpenAITTS creates an OpenAITTS with sensible defaults applied.
func NewOpenAITTS(cfg OpenAITTSConfig) *OpenAITTS {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.openai.com/v1"
	}
	if cfg.Model == "" {
		cfg.Model = "tts-1"
	}
	return &OpenAITTS{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

func (o *OpenAITTS) Name() string { return "openai-tts" }

// Synthesize converts text to audio and returns the audio bytes as MP3.
func (o *OpenAITTS) Synthesize(ctx context.Context, req SynthesisRequest) (*SynthesisResult, error) {
	voice := req.Voice
	if voice == "" {
		voice = "alloy"
	}

	body := map[string]any{
		"model": o.cfg.Model,
		"input": req.Input,
		"voice": voice,
	}
	if req.Speed > 0 {
		body["speed"] = req.Speed
	}

	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", o.cfg.BaseURL+"/audio/speech", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+o.cfg.APIKey)

	resp, err := o.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("tts request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("tts failed (status %d): %s", resp.StatusCode, string(respBody))
	}

	audio, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read audio: %w", err)
	}

	return &SynthesisResult{
		Audio:       audio,
		ContentType: "audio/mpeg",
	}, nil
}
