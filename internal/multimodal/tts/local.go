package tts

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// LocalTTSConfig holds configuration for the local Piper TTS backend.
type LocalTTSConfig struct {
	PiperBinPath string // default: "piper"
	ModelPath    string // required: path to the .onnx voice model
}

// LocalTTS synthesizes speech using the Piper binary via subprocess.
// Voice selection and speed are controlled via the model file, not runtime flags.
type LocalTTS struct {
	cfg LocalTTSConfig
}

// NewLocalTTS creates a LocalTTS backed by a local Piper binary.
func NewLocalTTS(cfg LocalTTSConfig) *LocalTTS {
	if cfg.PiperBinPath == "" {
		cfg.PiperBinPath = "piper"
	}
	return &LocalTTS{cfg: cfg}
}

func (l *LocalTTS) Name() string { return "local-piper" }

// Synthesize pipes text into Piper via stdin and returns the WAV output from stdout.
func (l *LocalTTS) Synthesize(ctx context.Context, req SynthesisRequest) (*SynthesisResult, error) {
	if l.cfg.ModelPath == "" {
		return nil, fmt.Errorf("piper model path is required (set TTS_LOCAL_PIPER_MODEL)")
	}

	cmd := exec.CommandContext(ctx, l.cfg.PiperBinPath, "--model", l.cfg.ModelPath, "--output-raw")

	cmd.Stdin = strings.NewReader(req.Input)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("piper failed: %w (stderr: %s)", err, stderr.String())
	}

	return &SynthesisResult{
		Audio:       stdout.Bytes(),
		ContentType: "audio/wav",
	}, nil
}
