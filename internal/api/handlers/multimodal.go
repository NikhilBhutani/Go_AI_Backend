package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/nikhilbhutani/backendwithai/internal/llm"
	"github.com/nikhilbhutani/backendwithai/internal/multimodal"
)

type MultimodalHandler struct {
	vision    *multimodal.VisionService
	imageGen  *multimodal.ImageGenerator
	tts       *multimodal.TextToSpeech
}

func NewMultimodalHandler(gw llm.Gateway, openaiKey string) *MultimodalHandler {
	return &MultimodalHandler{
		vision:   multimodal.NewVisionService(gw, ""),
		imageGen: multimodal.NewImageGenerator(openaiKey),
		tts:      multimodal.NewTextToSpeech(openaiKey),
	}
}

// Analyze sends images to a vision model with a prompt.
func (h *MultimodalHandler) Analyze(w http.ResponseWriter, r *http.Request) {
	var req multimodal.VisionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if len(req.Images) == 0 || req.Prompt == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "images and prompt required"})
		return
	}

	result, err := h.vision.Analyze(r.Context(), req)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// GenerateImage creates images from text prompts.
func (h *MultimodalHandler) GenerateImage(w http.ResponseWriter, r *http.Request) {
	var req multimodal.ImageGenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.Prompt == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "prompt required"})
		return
	}

	result, err := h.imageGen.Generate(r.Context(), req)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// Speak converts text to audio.
func (h *MultimodalHandler) Speak(w http.ResponseWriter, r *http.Request) {
	var req multimodal.TTSRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.Input == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "input text required"})
		return
	}

	audio, err := h.tts.Synthesize(r.Context(), req)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "audio/mpeg")
	w.WriteHeader(http.StatusOK)
	w.Write(audio)
}
