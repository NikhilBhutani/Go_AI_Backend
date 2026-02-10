package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/nikhilbhutani/backendwithai/internal/llm"
	"github.com/nikhilbhutani/backendwithai/internal/reasoning"
)

type ReasoningHandler struct {
	gateway llm.Gateway
}

func NewReasoningHandler(gw llm.Gateway) *ReasoningHandler {
	return &ReasoningHandler{gateway: gw}
}

// ChainOfThought runs step-by-step reasoning.
func (h *ReasoningHandler) ChainOfThought(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Query    string              `json:"query"`
		Model    string              `json:"model,omitempty"`
		Strategy string              `json:"strategy,omitempty"` // zero_shot, few_shot
		Examples []reasoning.Example `json:"examples,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.Query == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "query required"})
		return
	}

	model := req.Model
	if model == "" {
		model = "gpt-4o"
	}

	strategy := reasoning.Strategy(req.Strategy)
	if strategy == "" {
		strategy = reasoning.StrategyZeroShot
	}

	cot := reasoning.NewChainOfThought(h.gateway, model, strategy)
	result, err := cot.Reason(r.Context(), req.Query, req.Examples)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// TreeOfThought explores multiple reasoning paths.
func (h *ReasoningHandler) TreeOfThought(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Query    string `json:"query"`
		Model    string `json:"model,omitempty"`
		Branches int    `json:"branches,omitempty"`
		MaxDepth int    `json:"max_depth,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.Query == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "query required"})
		return
	}

	model := req.Model
	if model == "" {
		model = "gpt-4o"
	}

	tot := reasoning.NewTreeOfThought(h.gateway, model, req.Branches, req.MaxDepth)
	result, err := tot.Reason(r.Context(), req.Query)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// SelfConsistency samples multiple reasoning chains and picks majority answer.
func (h *ReasoningHandler) SelfConsistency(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Query   string `json:"query"`
		Model   string `json:"model,omitempty"`
		Samples int    `json:"samples,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.Query == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "query required"})
		return
	}

	model := req.Model
	if model == "" {
		model = "gpt-4o"
	}

	sc := reasoning.NewSelfConsistency(h.gateway, model, req.Samples)
	result, err := sc.Reason(r.Context(), req.Query)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// Reflect runs the reflection pattern (generate-critique-revise).
func (h *ReasoningHandler) Reflect(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Task       string `json:"task"`
		Model      string `json:"model,omitempty"`
		Iterations int    `json:"iterations,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.Task == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "task required"})
		return
	}

	model := req.Model
	if model == "" {
		model = "gpt-4o"
	}

	rc := reasoning.NewReflectionChain(h.gateway, model, req.Iterations)
	result, err := rc.Run(r.Context(), req.Task)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, result)
}
