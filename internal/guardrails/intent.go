package guardrails

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/nikhilbhutani/backendwithai/internal/llm"
)

// Intent represents a classified user intent.
type Intent struct {
	Name       string  `json:"name"`
	Confidence float64 `json:"confidence"`
	Entities   map[string]string `json:"entities,omitempty"`
}

// IntentClassifier classifies user messages into predefined intents.
type IntentClassifier struct {
	gateway llm.Gateway
	model   string
	intents []IntentDefinition
}

// IntentDefinition describes a possible intent with examples.
type IntentDefinition struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Examples    []string `json:"examples"`
}

func NewIntentClassifier(gw llm.Gateway, model string, intents []IntentDefinition) *IntentClassifier {
	if model == "" {
		model = "gpt-4o-mini"
	}
	return &IntentClassifier{
		gateway: gw,
		model:   model,
		intents: intents,
	}
}

// DefaultIntents returns a set of common intents for AI applications.
func DefaultIntents() []IntentDefinition {
	return []IntentDefinition{
		{
			Name:        "question",
			Description: "User is asking a factual question",
			Examples:    []string{"What is...", "How does...", "Explain..."},
		},
		{
			Name:        "command",
			Description: "User wants the AI to perform an action",
			Examples:    []string{"Generate a...", "Create a...", "Summarize..."},
		},
		{
			Name:        "conversation",
			Description: "User is having casual conversation",
			Examples:    []string{"Hello", "Thanks", "How are you"},
		},
		{
			Name:        "complaint",
			Description: "User is expressing dissatisfaction",
			Examples:    []string{"This doesn't work", "I'm unhappy with...", "This is wrong"},
		},
		{
			Name:        "code",
			Description: "User wants help with code or programming",
			Examples:    []string{"Write a function...", "Debug this...", "How do I implement..."},
		},
		{
			Name:        "unknown",
			Description: "Intent cannot be determined",
			Examples:    []string{},
		},
	}
}

// Classify determines the intent of the user's message.
func (c *IntentClassifier) Classify(ctx context.Context, text string) (*Intent, error) {
	// Build intent descriptions for the prompt
	var intentDesc strings.Builder
	for _, intent := range c.intents {
		fmt.Fprintf(&intentDesc, "- %s: %s\n", intent.Name, intent.Description)
	}

	resp, err := c.gateway.Chat(ctx, llm.ChatRequest{
		Model: c.model,
		Messages: []llm.Message{
			{
				Role: "system",
				Content: fmt.Sprintf(`Classify the user's message into one of these intents:
%s
Reply with ONLY a JSON object: {"name": "intent_name", "confidence": 0.0-1.0}`, intentDesc.String()),
			},
			{
				Role:    "user",
				Content: text,
			},
		},
		Temperature: 0,
		MaxTokens:   50,
	})
	if err != nil {
		return &Intent{Name: "unknown", Confidence: 0}, nil
	}

	content := strings.TrimSpace(resp.Content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	var intent Intent
	if err := json.Unmarshal([]byte(content), &intent); err != nil {
		return &Intent{Name: "unknown", Confidence: 0}, nil
	}

	return &intent, nil
}
