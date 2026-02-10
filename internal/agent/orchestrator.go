package agent

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/nikhilbhutani/backendwithai/internal/llm"
	"github.com/nikhilbhutani/backendwithai/internal/memory"
)

// Orchestrator manages multiple agents and routes tasks to the right one.
type Orchestrator struct {
	gateway llm.Gateway
	model   string
	agents  map[string]*Agent
	router  Router
}

func NewOrchestrator(gw llm.Gateway, model string) *Orchestrator {
	return &Orchestrator{
		gateway: gw,
		model:   model,
		agents:  make(map[string]*Agent),
	}
}

// RegisterAgent adds a named agent to the orchestrator.
func (o *Orchestrator) RegisterAgent(name string, agent *Agent) {
	o.agents[name] = agent
}

// SetRouter sets the routing strategy.
func (o *Orchestrator) SetRouter(r Router) {
	o.router = r
}

// Run routes the message to the appropriate agent and executes it.
func (o *Orchestrator) Run(ctx context.Context, message string) (*AgentResponse, error) {
	if len(o.agents) == 0 {
		return nil, fmt.Errorf("no agents registered")
	}

	// If only one agent, use it directly
	if len(o.agents) == 1 {
		for _, agent := range o.agents {
			return agent.Run(ctx, message)
		}
	}

	// Route to the right agent
	agentName, err := o.route(ctx, message)
	if err != nil {
		return nil, fmt.Errorf("routing: %w", err)
	}

	agent, ok := o.agents[agentName]
	if !ok {
		return nil, fmt.Errorf("agent %q not found", agentName)
	}

	slog.Info("routing to agent", "agent", agentName, "message_preview", truncateStr(message, 50))
	return agent.Run(ctx, message)
}

// Router determines which agent should handle a message.
type Router interface {
	Route(ctx context.Context, message string, agentNames []string) (string, error)
}

// LLMRouter uses an LLM to decide which agent should handle the message.
type LLMRouter struct {
	gateway     llm.Gateway
	model       string
	descriptions map[string]string // agent name -> description
}

func NewLLMRouter(gw llm.Gateway, model string) *LLMRouter {
	return &LLMRouter{
		gateway:      gw,
		model:        model,
		descriptions: make(map[string]string),
	}
}

func (r *LLMRouter) AddAgentDescription(name, description string) {
	r.descriptions[name] = description
}

func (r *LLMRouter) Route(ctx context.Context, message string, agentNames []string) (string, error) {
	desc := ""
	for _, name := range agentNames {
		if d, ok := r.descriptions[name]; ok {
			desc += fmt.Sprintf("- %s: %s\n", name, d)
		} else {
			desc += fmt.Sprintf("- %s\n", name)
		}
	}

	resp, err := r.gateway.Chat(ctx, llm.ChatRequest{
		Model: r.model,
		Messages: []llm.Message{
			{
				Role: "system",
				Content: fmt.Sprintf(`You are a routing agent. Given a user message, determine which agent is best suited to handle it.

Available agents:
%s
Reply with ONLY the agent name, nothing else.`, desc),
			},
			{Role: "user", Content: message},
		},
		Temperature: 0,
		MaxTokens:   20,
	})
	if err != nil {
		return agentNames[0], nil
	}

	return resp.Content, nil
}

func (o *Orchestrator) route(ctx context.Context, message string) (string, error) {
	names := make([]string, 0, len(o.agents))
	for name := range o.agents {
		names = append(names, name)
	}

	if o.router != nil {
		return o.router.Route(ctx, message, names)
	}

	// Default: use LLM-based routing
	router := NewLLMRouter(o.gateway, o.model)
	return router.Route(ctx, message, names)
}

// PromptChain executes a sequence of LLM calls where each step's output
// feeds into the next step's input.
type PromptChain struct {
	gateway llm.Gateway
	steps   []ChainStep
}

type ChainStep struct {
	Name         string
	SystemPrompt string
	Model        string
	Transform    func(input, output string) string // optional transform between steps
}

func NewPromptChain(gw llm.Gateway) *PromptChain {
	return &PromptChain{gateway: gw}
}

func (c *PromptChain) AddStep(step ChainStep) {
	c.steps = append(c.steps, step)
}

// Execute runs the chain, feeding output from each step to the next.
func (c *PromptChain) Execute(ctx context.Context, initialInput string) (string, error) {
	current := initialInput

	for i, step := range c.steps {
		resp, err := c.gateway.Chat(ctx, llm.ChatRequest{
			Model: step.Model,
			Messages: []llm.Message{
				{Role: "system", Content: step.SystemPrompt},
				{Role: "user", Content: current},
			},
		})
		if err != nil {
			return "", fmt.Errorf("chain step %d (%s): %w", i, step.Name, err)
		}

		output := resp.Content
		if step.Transform != nil {
			current = step.Transform(current, output)
		} else {
			current = output
		}
	}

	return current, nil
}

// MultiAgentConversation enables multiple agents to discuss a topic.
type MultiAgentConversation struct {
	gateway llm.Gateway
	agents  []*Agent
	rounds  int
}

func NewMultiAgentConversation(gw llm.Gateway, rounds int) *MultiAgentConversation {
	return &MultiAgentConversation{gateway: gw, rounds: rounds}
}

func (m *MultiAgentConversation) AddAgent(agent *Agent) {
	m.agents = append(m.agents, agent)
}

// Discuss runs a multi-agent conversation on a topic.
func (m *MultiAgentConversation) Discuss(ctx context.Context, topic string) ([]memory.Entry, error) {
	var transcript []memory.Entry

	currentMessage := topic
	for round := 0; round < m.rounds; round++ {
		for _, agent := range m.agents {
			resp, err := agent.Run(ctx, currentMessage)
			if err != nil {
				continue
			}

			entry := memory.Entry{
				Role:    agent.name,
				Content: resp.Answer,
			}
			transcript = append(transcript, entry)
			currentMessage = resp.Answer
		}
	}

	return transcript, nil
}

func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
