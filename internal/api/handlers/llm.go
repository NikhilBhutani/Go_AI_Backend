package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/nikhilbhutani/backendwithai/internal/llm"
)

type LLMHandler struct {
	gateway llm.Gateway
}

func NewLLMHandler(gw llm.Gateway) *LLMHandler {
	return &LLMHandler{gateway: gw}
}

func (h *LLMHandler) Chat(w http.ResponseWriter, r *http.Request) {
	var req llm.ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if len(req.Messages) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "messages required"})
		return
	}

	resp, err := h.gateway.Chat(r.Context(), req)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

func (h *LLMHandler) ChatStream(w http.ResponseWriter, r *http.Request) {
	var req llm.ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "streaming not supported"})
		return
	}

	ch, err := h.gateway.ChatStream(r.Context(), req)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	for chunk := range ch {
		if chunk.Error != nil {
			fmt.Fprintf(w, "data: {\"error\":%q}\n\n", chunk.Error.Error())
			flusher.Flush()
			return
		}

		data, _ := json.Marshal(chunk)
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()

		if chunk.Done {
			return
		}
	}
}

func (h *LLMHandler) Embed(w http.ResponseWriter, r *http.Request) {
	var req llm.EmbeddingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if len(req.Input) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "input required"})
		return
	}

	resp, err := h.gateway.Embed(r.Context(), req)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

func (h *LLMHandler) Models(w http.ResponseWriter, r *http.Request) {
	models := h.gateway.ListModels()
	writeJSON(w, http.StatusOK, map[string]interface{}{"models": models})
}
