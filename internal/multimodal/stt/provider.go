package stt

import "context"

// TranscriptionRequest holds the parameters for audio transcription.
type TranscriptionRequest struct {
	FilePath string `json:"file_path"`
	Language string `json:"language,omitempty"`
	Prompt   string `json:"prompt,omitempty"`
}

// TranscriptionResponse holds the transcription result.
type TranscriptionResponse struct {
	Text     string  `json:"text"`
	Language string  `json:"language"`
	Duration float64 `json:"duration"`
}

// STTProvider is the interface for speech-to-text backends.
type STTProvider interface {
	Transcribe(ctx context.Context, req TranscriptionRequest) (*TranscriptionResponse, error)
	Name() string
}
