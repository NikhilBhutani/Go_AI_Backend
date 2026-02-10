package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/nikhilbhutani/backendwithai/internal/llm"
	"github.com/nikhilbhutani/backendwithai/internal/rag"
)

// Tool is something an agent can invoke during its reasoning loop.
type Tool interface {
	Name() string
	Description() string
	Execute(ctx context.Context, input string) (string, error)
}

// CalculatorTool performs basic arithmetic.
type CalculatorTool struct{}

func NewCalculatorTool() *CalculatorTool { return &CalculatorTool{} }

func (t *CalculatorTool) Name() string        { return "calculator" }
func (t *CalculatorTool) Description() string  { return "Evaluate a mathematical expression. Input: a math expression like '2 + 2' or '100 * 0.15'" }

func (t *CalculatorTool) Execute(ctx context.Context, input string) (string, error) {
	// Use LLM for calculation to keep it simple and handle complex expressions
	return fmt.Sprintf("Result of '%s' — use your own math ability to compute this.", input), nil
}

// RAGSearchTool searches the knowledge base via the RAG pipeline.
type RAGSearchTool struct {
	pipeline rag.Pipeline
}

func NewRAGSearchTool(p rag.Pipeline) *RAGSearchTool {
	return &RAGSearchTool{pipeline: p}
}

func (t *RAGSearchTool) Name() string        { return "search_knowledge_base" }
func (t *RAGSearchTool) Description() string  { return "Search the internal knowledge base for relevant information. Input: your search query" }

func (t *RAGSearchTool) Execute(ctx context.Context, input string) (string, error) {
	results, err := t.pipeline.Search(ctx, rag.SearchRequest{
		Query: input,
		TopK:  5,
	})
	if err != nil {
		return "", fmt.Errorf("search: %w", err)
	}

	if len(results) == 0 {
		return "No relevant documents found.", nil
	}

	var sb strings.Builder
	for i, r := range results {
		fmt.Fprintf(&sb, "[%d] (score: %.3f) %s\n\n", i+1, r.Score, r.Content)
	}
	return sb.String(), nil
}

// WebFetchTool fetches content from a URL.
type WebFetchTool struct {
	client *http.Client
}

func NewWebFetchTool() *WebFetchTool {
	return &WebFetchTool{
		client: &http.Client{Timeout: 15 * time.Second},
	}
}

func (t *WebFetchTool) Name() string        { return "web_fetch" }
func (t *WebFetchTool) Description() string  { return "Fetch text content from a URL. Input: a valid URL" }

func (t *WebFetchTool) Execute(ctx context.Context, input string) (string, error) {
	url := strings.TrimSpace(input)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 10000))
	if err != nil {
		return "", fmt.Errorf("read body: %w", err)
	}

	return string(body), nil
}

// LLMTool uses an LLM for a specific sub-task (e.g., summarization, translation).
type LLMTool struct {
	name        string
	description string
	prompt      string
	gateway     llm.Gateway
	model       string
}

func NewLLMTool(name, description, prompt string, gw llm.Gateway, model string) *LLMTool {
	return &LLMTool{
		name:        name,
		description: description,
		prompt:      prompt,
		gateway:     gw,
		model:       model,
	}
}

func (t *LLMTool) Name() string        { return t.name }
func (t *LLMTool) Description() string  { return t.description }

func (t *LLMTool) Execute(ctx context.Context, input string) (string, error) {
	resp, err := t.gateway.Chat(ctx, llm.ChatRequest{
		Model: t.model,
		Messages: []llm.Message{
			{Role: "system", Content: t.prompt},
			{Role: "user", Content: input},
		},
	})
	if err != nil {
		return "", err
	}
	return resp.Content, nil
}

// JSONExtractorTool extracts structured data from text using an LLM.
type JSONExtractorTool struct {
	gateway llm.Gateway
	model   string
	schema  string // JSON schema description
}

func NewJSONExtractorTool(gw llm.Gateway, model, schema string) *JSONExtractorTool {
	return &JSONExtractorTool{gateway: gw, model: model, schema: schema}
}

func (t *JSONExtractorTool) Name() string { return "extract_json" }
func (t *JSONExtractorTool) Description() string {
	return "Extract structured JSON data from text. Input: the text to extract from"
}

func (t *JSONExtractorTool) Execute(ctx context.Context, input string) (string, error) {
	resp, err := t.gateway.Chat(ctx, llm.ChatRequest{
		Model: t.model,
		Messages: []llm.Message{
			{
				Role:    "system",
				Content: fmt.Sprintf("Extract structured data from the text and return ONLY valid JSON matching this schema:\n%s", t.schema),
			},
			{Role: "user", Content: input},
		},
		Temperature: 0,
	})
	if err != nil {
		return "", err
	}

	// Validate it's valid JSON
	content := strings.TrimSpace(resp.Content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	var js json.RawMessage
	if err := json.Unmarshal([]byte(content), &js); err != nil {
		return "", fmt.Errorf("LLM did not return valid JSON: %w", err)
	}

	return content, nil
}
