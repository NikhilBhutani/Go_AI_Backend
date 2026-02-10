package reasoning

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/nikhilbhutani/backendwithai/internal/llm"
)

// StructuredOutput forces LLM responses into a defined JSON schema.
// Implements structured output / function calling patterns.
type StructuredOutput struct {
	gateway llm.Gateway
	model   string
}

func NewStructuredOutput(gw llm.Gateway, model string) *StructuredOutput {
	return &StructuredOutput{gateway: gw, model: model}
}

// SchemaField defines a field in the expected output schema.
type SchemaField struct {
	Name        string `json:"name"`
	Type        string `json:"type"` // string, number, boolean, array, object
	Description string `json:"description"`
	Required    bool   `json:"required"`
}

// Generate produces a response conforming to the given schema.
func (s *StructuredOutput) Generate(ctx context.Context, prompt string, schema []SchemaField) (map[string]any, error) {
	schemaDesc := buildSchemaDescription(schema)

	resp, err := s.gateway.Chat(ctx, llm.ChatRequest{
		Model: s.model,
		Messages: []llm.Message{
			{
				Role: "system",
				Content: fmt.Sprintf(`You must respond with ONLY a valid JSON object matching this schema:

%s

Do not include any text outside the JSON object. No markdown, no explanation.`, schemaDesc),
			},
			{Role: "user", Content: prompt},
		},
		Temperature: 0,
	})
	if err != nil {
		return nil, fmt.Errorf("structured output: %w", err)
	}

	content := strings.TrimSpace(resp.Content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	var result map[string]any
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return nil, fmt.Errorf("failed to parse structured output: %w", err)
	}

	return result, nil
}

// GenerateTyped produces a response and unmarshals it into the given type.
func (s *StructuredOutput) GenerateTyped(ctx context.Context, prompt string, schema []SchemaField, target any) error {
	result, err := s.Generate(ctx, prompt, schema)
	if err != nil {
		return err
	}

	data, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("re-marshal: %w", err)
	}

	return json.Unmarshal(data, target)
}

func buildSchemaDescription(fields []SchemaField) string {
	var sb strings.Builder
	sb.WriteString("{\n")
	for i, f := range fields {
		required := ""
		if f.Required {
			required = " (REQUIRED)"
		}
		fmt.Fprintf(&sb, `  "%s": <%s>%s // %s`, f.Name, f.Type, required, f.Description)
		if i < len(fields)-1 {
			sb.WriteString(",")
		}
		sb.WriteString("\n")
	}
	sb.WriteString("}")
	return sb.String()
}

// ReflectionChain implements the Reflection pattern where the LLM
// critiques its own output and iteratively improves it.
type ReflectionChain struct {
	gateway    llm.Gateway
	model      string
	iterations int
}

func NewReflectionChain(gw llm.Gateway, model string, iterations int) *ReflectionChain {
	if iterations <= 0 {
		iterations = 2
	}
	return &ReflectionChain{gateway: gw, model: model, iterations: iterations}
}

// ReflectionResult contains the final and intermediate outputs.
type ReflectionResult struct {
	FinalOutput  string             `json:"final_output"`
	Iterations   []ReflectionRound  `json:"iterations"`
}

// ReflectionRound is a single generate-then-critique cycle.
type ReflectionRound struct {
	Draft    string `json:"draft"`
	Critique string `json:"critique"`
	Revised  string `json:"revised"`
}

// Run executes the reflection loop: generate → critique → revise.
func (r *ReflectionChain) Run(ctx context.Context, task string) (*ReflectionResult, error) {
	// Initial generation
	resp, err := r.gateway.Chat(ctx, llm.ChatRequest{
		Model:    r.model,
		Messages: []llm.Message{{Role: "user", Content: task}},
	})
	if err != nil {
		return nil, fmt.Errorf("reflection initial: %w", err)
	}

	currentDraft := resp.Content
	var iterations []ReflectionRound

	for i := 0; i < r.iterations; i++ {
		// Critique
		critiqueResp, err := r.gateway.Chat(ctx, llm.ChatRequest{
			Model: r.model,
			Messages: []llm.Message{
				{
					Role: "system",
					Content: `You are a critical reviewer. Identify specific weaknesses, errors,
or areas for improvement in the following draft. Be constructive and specific.`,
				},
				{
					Role:    "user",
					Content: fmt.Sprintf("Task: %s\n\nDraft:\n%s", task, currentDraft),
				},
			},
		})
		if err != nil {
			break
		}

		// Revise based on critique
		reviseResp, err := r.gateway.Chat(ctx, llm.ChatRequest{
			Model: r.model,
			Messages: []llm.Message{
				{
					Role: "system",
					Content: `Revise the draft to address the critique. Keep what's good, fix what's identified as wrong or weak.`,
				},
				{
					Role:    "user",
					Content: fmt.Sprintf("Original task: %s\n\nCurrent draft:\n%s\n\nCritique:\n%s\n\nPlease provide a revised version.", task, currentDraft, critiqueResp.Content),
				},
			},
		})
		if err != nil {
			break
		}

		iterations = append(iterations, ReflectionRound{
			Draft:    currentDraft,
			Critique: critiqueResp.Content,
			Revised:  reviseResp.Content,
		})

		currentDraft = reviseResp.Content
	}

	return &ReflectionResult{
		FinalOutput: currentDraft,
		Iterations:  iterations,
	}, nil
}
