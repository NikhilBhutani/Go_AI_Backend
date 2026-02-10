package agent

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/nikhilbhutani/backendwithai/internal/llm"
	"github.com/nikhilbhutani/backendwithai/internal/memory"
)

// Agent is an LLM-powered entity that can use tools and maintain state.
type Agent struct {
	name        string
	systemPrompt string
	gateway     llm.Gateway
	model       string
	tools       map[string]Tool
	memory      memory.Memory
	maxSteps    int
}

// AgentConfig holds configuration for creating an agent.
type AgentConfig struct {
	Name         string
	SystemPrompt string
	Model        string
	MaxSteps     int // max ReAct iterations
}

func NewAgent(gw llm.Gateway, mem memory.Memory, cfg AgentConfig) *Agent {
	if cfg.MaxSteps <= 0 {
		cfg.MaxSteps = 10
	}
	return &Agent{
		name:         cfg.Name,
		systemPrompt: cfg.SystemPrompt,
		gateway:      gw,
		model:        cfg.Model,
		tools:        make(map[string]Tool),
		memory:       mem,
		maxSteps:     cfg.MaxSteps,
	}
}

// RegisterTool adds a tool the agent can use.
func (a *Agent) RegisterTool(tool Tool) {
	a.tools[tool.Name()] = tool
}

// AgentResponse is the final output from an agent run.
type AgentResponse struct {
	Answer     string       `json:"answer"`
	Steps      []AgentStep  `json:"steps"`
	TotalSteps int          `json:"total_steps"`
	TokensUsed int          `json:"tokens_used"`
}

// AgentStep records one iteration of the agent's reasoning loop.
type AgentStep struct {
	StepNumber int    `json:"step"`
	Thought    string `json:"thought"`
	Action     string `json:"action,omitempty"`
	ActionInput string `json:"action_input,omitempty"`
	Observation string `json:"observation,omitempty"`
}

// Run executes the agent with a user message using the ReAct pattern.
func (a *Agent) Run(ctx context.Context, userMessage string) (*AgentResponse, error) {
	// Add user message to memory
	if a.memory != nil {
		a.memory.Add(ctx, memory.Entry{Role: "user", Content: userMessage})
	}

	// Build tool descriptions for the system prompt
	toolDesc := a.buildToolDescriptions()
	systemPrompt := fmt.Sprintf(`%s

You have access to the following tools:
%s

To use a tool, respond with this EXACT format:
Thought: [your reasoning about what to do]
Action: [tool name]
Action Input: [input to the tool]

When you have enough information to answer, respond with:
Thought: [your final reasoning]
Final Answer: [your response to the user]

Always start with a Thought. You MUST end with a Final Answer.`, a.systemPrompt, toolDesc)

	var steps []AgentStep
	totalTokens := 0

	// Build message history
	messages := []llm.Message{{Role: "system", Content: systemPrompt}}

	// Include conversation history from memory
	if a.memory != nil {
		history := a.memory.Get(ctx, 20)
		for _, h := range history {
			messages = append(messages, llm.Message{Role: h.Role, Content: h.Content})
		}
	}

	messages = append(messages, llm.Message{Role: "user", Content: userMessage})

	// ReAct loop
	for step := 0; step < a.maxSteps; step++ {
		resp, err := a.gateway.Chat(ctx, llm.ChatRequest{
			Model:    a.model,
			Messages: messages,
		})
		if err != nil {
			return nil, fmt.Errorf("agent step %d: %w", step, err)
		}

		totalTokens += resp.TotalTokens

		parsed := parseReActResponse(resp.Content)
		agentStep := AgentStep{
			StepNumber:  step + 1,
			Thought:     parsed.Thought,
			Action:      parsed.Action,
			ActionInput: parsed.ActionInput,
		}

		// If we got a Final Answer, we're done
		if parsed.FinalAnswer != "" {
			steps = append(steps, agentStep)

			if a.memory != nil {
				a.memory.Add(ctx, memory.Entry{Role: "assistant", Content: parsed.FinalAnswer})
			}

			return &AgentResponse{
				Answer:     parsed.FinalAnswer,
				Steps:      steps,
				TotalSteps: step + 1,
				TokensUsed: totalTokens,
			}, nil
		}

		// Execute the tool
		if parsed.Action != "" {
			tool, exists := a.tools[parsed.Action]
			if !exists {
				agentStep.Observation = fmt.Sprintf("Error: tool %q not found. Available tools: %s", parsed.Action, a.toolNames())
			} else {
				slog.Debug("agent executing tool", "tool", parsed.Action, "input", parsed.ActionInput)
				result, err := tool.Execute(ctx, parsed.ActionInput)
				if err != nil {
					agentStep.Observation = fmt.Sprintf("Error: %s", err.Error())
				} else {
					agentStep.Observation = result
				}
			}
		}

		steps = append(steps, agentStep)

		// Feed observation back
		messages = append(messages,
			llm.Message{Role: "assistant", Content: resp.Content},
			llm.Message{Role: "user", Content: fmt.Sprintf("Observation: %s", agentStep.Observation)},
		)
	}

	return &AgentResponse{
		Answer:     "I was unable to complete the task within the maximum number of steps.",
		Steps:      steps,
		TotalSteps: a.maxSteps,
		TokensUsed: totalTokens,
	}, nil
}

func (a *Agent) buildToolDescriptions() string {
	desc := ""
	for _, tool := range a.tools {
		desc += fmt.Sprintf("- %s: %s\n", tool.Name(), tool.Description())
	}
	return desc
}

func (a *Agent) toolNames() string {
	names := ""
	for name := range a.tools {
		if names != "" {
			names += ", "
		}
		names += name
	}
	return names
}
