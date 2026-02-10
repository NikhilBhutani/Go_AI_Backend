package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/nikhilbhutani/backendwithai/internal/eval"
	"github.com/nikhilbhutani/backendwithai/internal/llm"
)

type EvalHandler struct {
	gateway llm.Gateway
}

func NewEvalHandler(gw llm.Gateway) *EvalHandler {
	return &EvalHandler{gateway: gw}
}

// RunSuite runs the default eval suite against a response.
func (h *EvalHandler) RunSuite(w http.ResponseWriter, r *http.Request) {
	var input eval.EvalInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if input.Query == "" || input.Response == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "query and response required"})
		return
	}

	suite := eval.DefaultSuite(h.gateway)
	results, err := suite.RunAll(r.Context(), input)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"results": results})
}

// Judge runs the LLM judge evaluator.
func (h *EvalHandler) Judge(w http.ResponseWriter, r *http.Request) {
	var input eval.EvalInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	judge := eval.NewLLMJudge(h.gateway, "")
	result, err := judge.Evaluate(r.Context(), input)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// Compare performs pairwise comparison of two responses.
func (h *EvalHandler) Compare(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Query     string `json:"query"`
		ResponseA string `json:"response_a"`
		ResponseB string `json:"response_b"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	judge := eval.NewPairwiseJudge(h.gateway, "")
	result, err := judge.Compare(r.Context(), req.Query, req.ResponseA, req.ResponseB)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, result)
}
