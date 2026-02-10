package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/nikhilbhutani/backendwithai/internal/agent"
	"github.com/nikhilbhutani/backendwithai/internal/llm"
	"github.com/nikhilbhutani/backendwithai/internal/memory"
)

type AgentHandler struct {
	gateway llm.Gateway
}

func NewAgentHandler(gw llm.Gateway) *AgentHandler {
	return &AgentHandler{gateway: gw}
}

// Run executes an agent with the ReAct pattern.
func (h *AgentHandler) Run(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Query    string `json:"query"`
		Model    string `json:"model,omitempty"`
		MaxSteps int    `json:"max_steps,omitempty"`
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
	maxSteps := req.MaxSteps
	if maxSteps <= 0 {
		maxSteps = 10
	}

	mem := memory.NewBufferMemory(20)
	a := agent.NewAgent(h.gateway, mem, agent.AgentConfig{
		Name:         "api_agent",
		SystemPrompt: "You are a helpful AI assistant.",
		Model:        model,
		MaxSteps:     maxSteps,
	})
	a.RegisterTool(agent.NewCalculatorTool())

	result, err := a.Run(r.Context(), req.Query)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// Chain executes a prompt chain.
func (h *AgentHandler) Chain(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Steps []struct {
			Name         string `json:"name"`
			SystemPrompt string `json:"system_prompt"`
			Model        string `json:"model,omitempty"`
		} `json:"steps"`
		Input string `json:"input"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if len(req.Steps) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "steps required"})
		return
	}

	chain := agent.NewPromptChain(h.gateway)
	for _, s := range req.Steps {
		model := s.Model
		if model == "" {
			model = "gpt-4o"
		}
		chain.AddStep(agent.ChainStep{
			Name:         s.Name,
			SystemPrompt: s.SystemPrompt,
			Model:        model,
		})
	}

	result, err := chain.Execute(r.Context(), req.Input)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"output": result})
}
