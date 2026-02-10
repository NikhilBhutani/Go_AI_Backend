package reasoning

import (
	"context"
	"fmt"
	"strings"

	"github.com/nikhilbhutani/backendwithai/internal/llm"
)

// Strategy defines how reasoning is applied.
type Strategy string

const (
	StrategyZeroShot Strategy = "zero_shot" // "Let's think step by step"
	StrategyFewShot  Strategy = "few_shot"  // Provide worked examples
	StrategyTreeOfThought Strategy = "tree_of_thought" // Explore multiple paths
	StrategySelfConsistency Strategy = "self_consistency" // Sample multiple, take majority
)

// ChainOfThought adds step-by-step reasoning to LLM calls.
type ChainOfThought struct {
	gateway  llm.Gateway
	strategy Strategy
	model    string
}

func NewChainOfThought(gw llm.Gateway, model string, strategy Strategy) *ChainOfThought {
	if strategy == "" {
		strategy = StrategyZeroShot
	}
	return &ChainOfThought{gateway: gw, model: model, strategy: strategy}
}

// ReasoningResult holds the LLM output split into reasoning steps and final answer.
type ReasoningResult struct {
	Steps       []string `json:"steps"`
	FinalAnswer string   `json:"final_answer"`
	RawOutput   string   `json:"raw_output"`
	Model       string   `json:"model"`
}

// Reason processes a query with chain-of-thought prompting.
func (c *ChainOfThought) Reason(ctx context.Context, query string, examples []Example) (*ReasoningResult, error) {
	var messages []llm.Message

	switch c.strategy {
	case StrategyFewShot:
		messages = c.buildFewShotMessages(query, examples)
	default:
		messages = c.buildZeroShotMessages(query)
	}

	resp, err := c.gateway.Chat(ctx, llm.ChatRequest{
		Model:       c.model,
		Messages:    messages,
		Temperature: 0.1,
	})
	if err != nil {
		return nil, fmt.Errorf("cot reasoning: %w", err)
	}

	return c.parseReasoning(resp.Content, resp.Model), nil
}

// Example is a worked example for few-shot CoT.
type Example struct {
	Question  string `json:"question"`
	Reasoning string `json:"reasoning"`
	Answer    string `json:"answer"`
}

func (c *ChainOfThought) buildZeroShotMessages(query string) []llm.Message {
	return []llm.Message{
		{
			Role: "system",
			Content: `You are a careful, logical thinker. When given a question:
1. Break it down into steps
2. Reason through each step explicitly
3. State your final answer clearly

Format your response as:
Step 1: ...
Step 2: ...
...
Final Answer: ...`,
		},
		{
			Role:    "user",
			Content: query + "\n\nLet's think step by step.",
		},
	}
}

func (c *ChainOfThought) buildFewShotMessages(query string, examples []Example) []llm.Message {
	var sb strings.Builder
	sb.WriteString("Here are examples of step-by-step reasoning:\n\n")
	for i, ex := range examples {
		fmt.Fprintf(&sb, "Example %d:\nQ: %s\n%s\nAnswer: %s\n\n", i+1, ex.Question, ex.Reasoning, ex.Answer)
	}

	return []llm.Message{
		{
			Role: "system",
			Content: `You are a careful, logical thinker. Follow the reasoning pattern shown in the examples.
Break down your thinking into explicit steps, then give a final answer.`,
		},
		{Role: "user", Content: sb.String()},
		{Role: "assistant", Content: "I understand the reasoning pattern. I'll apply it to the next question."},
		{Role: "user", Content: fmt.Sprintf("Q: %s\n\nLet's think step by step.", query)},
	}
}

func (c *ChainOfThought) parseReasoning(content, model string) *ReasoningResult {
	result := &ReasoningResult{
		RawOutput: content,
		Model:     model,
	}

	lines := strings.Split(content, "\n")
	var currentStep strings.Builder

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		if strings.HasPrefix(trimmed, "Final Answer:") {
			// Save any pending step
			if currentStep.Len() > 0 {
				result.Steps = append(result.Steps, strings.TrimSpace(currentStep.String()))
				currentStep.Reset()
			}
			result.FinalAnswer = strings.TrimPrefix(trimmed, "Final Answer:")
			result.FinalAnswer = strings.TrimSpace(result.FinalAnswer)
		} else if strings.HasPrefix(trimmed, "Step ") {
			// Save previous step
			if currentStep.Len() > 0 {
				result.Steps = append(result.Steps, strings.TrimSpace(currentStep.String()))
				currentStep.Reset()
			}
			currentStep.WriteString(trimmed)
		} else {
			if currentStep.Len() > 0 {
				currentStep.WriteString(" ")
			}
			currentStep.WriteString(trimmed)
		}
	}

	if currentStep.Len() > 0 {
		result.Steps = append(result.Steps, strings.TrimSpace(currentStep.String()))
	}

	// If no explicit final answer was found, use the last step or full output
	if result.FinalAnswer == "" && len(result.Steps) > 0 {
		result.FinalAnswer = result.Steps[len(result.Steps)-1]
	} else if result.FinalAnswer == "" {
		result.FinalAnswer = content
	}

	return result
}
