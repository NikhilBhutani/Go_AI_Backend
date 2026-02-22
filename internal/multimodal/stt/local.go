package stt

import "context"

// LocalSTTConfig holds configuration for the local whisper.cpp STT backend.
type LocalSTTConfig struct {
	BaseURL string // default: "http://localhost:8178"
}

// LocalSTT wraps OpenAISTT pointing at a local whisper.cpp server.
// Start the server with: ./server -m models/ggml-base.en.bin --port 8178
type LocalSTT struct {
	*OpenAISTT
}

// NewLocalSTT creates a LocalSTT backed by a local whisper.cpp HTTP server.
func NewLocalSTT(cfg LocalSTTConfig) *LocalSTT {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "http://localhost:8178"
	}
	return &LocalSTT{
		OpenAISTT: NewOpenAISTT(OpenAISTTConfig{
			BaseURL: baseURL,
			// No API key needed for local server
		}),
	}
}

func (l *LocalSTT) Name() string { return "local-whisper" }

func (l *LocalSTT) Transcribe(ctx context.Context, req TranscriptionRequest) (*TranscriptionResponse, error) {
	return l.OpenAISTT.Transcribe(ctx, req)
}
