package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/nikhilbhutani/backendwithai/internal/guardrails"
	"github.com/nikhilbhutani/backendwithai/internal/llm"
)

type GuardrailHandler struct {
	pipeline *guardrails.Pipeline
}

func NewGuardrailHandler(gw llm.Gateway) *GuardrailHandler {
	return &GuardrailHandler{
		pipeline: guardrails.DefaultPipeline(gw),
	}
}

// Check validates input text against all guardrails.
func (h *GuardrailHandler) Check(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Text string `json:"text"`
		Type string `json:"type,omitempty"` // "input" or "output", defaults to "input"
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.Text == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "text required"})
		return
	}

	var result *guardrails.GuardrailResult
	var err error

	if req.Type == "output" {
		result, err = h.pipeline.CheckOutput(r.Context(), req.Text)
	} else {
		result, err = h.pipeline.CheckInput(r.Context(), req.Text)
	}

	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// Classify detects the intent of input text.
func (h *GuardrailHandler) Classify(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	// Run input checks which include intent classification
	result, err := h.pipeline.CheckInput(r.Context(), req.Text)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, result)
}
