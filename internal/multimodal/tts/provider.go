package tts

import "context"

// SynthesisRequest holds the parameters for text-to-speech generation.
type SynthesisRequest struct {
	Input string  `json:"input"`
	Voice string  `json:"voice,omitempty"`
	Speed float64 `json:"speed,omitempty"`
}

// SynthesisResult holds the generated audio and its content type.
type SynthesisResult struct {
	Audio       []byte
	ContentType string // "audio/mpeg" (OpenAI) or "audio/wav" (Piper)
}

// TTSProvider is the interface for text-to-speech backends.
type TTSProvider interface {
	Synthesize(ctx context.Context, req SynthesisRequest) (*SynthesisResult, error)
	Name() string
}
