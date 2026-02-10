package eval

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/nikhilbhutani/backendwithai/internal/llm"
)

// EvalResult holds the outcome of an evaluation.
type EvalResult struct {
	Name      string             `json:"name"`
	Score     float64            `json:"score"`     // 0.0 to 1.0
	Pass      bool               `json:"pass"`
	Details   map[string]float64 `json:"details,omitempty"`
	Reasoning string             `json:"reasoning,omitempty"`
	Duration  time.Duration      `json:"duration_ms"`
}

// Evaluator scores an LLM response against criteria.
type Evaluator interface {
	Evaluate(ctx context.Context, input EvalInput) (*EvalResult, error)
	Name() string
}

// EvalInput is the data fed into an evaluator.
type EvalInput struct {
	Query           string   `json:"query"`
	Response        string   `json:"response"`
	ExpectedAnswer  string   `json:"expected_answer,omitempty"`
	Context         []string `json:"context,omitempty"` // retrieved documents
	GroundTruth     string   `json:"ground_truth,omitempty"`
}

// EvalSuite runs multiple evaluators and aggregates results.
type EvalSuite struct {
	evaluators []Evaluator
}

func NewEvalSuite() *EvalSuite {
	return &EvalSuite{}
}

func (s *EvalSuite) Add(e Evaluator) {
	s.evaluators = append(s.evaluators, e)
}

// RunAll executes all evaluators and returns combined results.
func (s *EvalSuite) RunAll(ctx context.Context, input EvalInput) ([]EvalResult, error) {
	var results []EvalResult
	for _, e := range s.evaluators {
		result, err := e.Evaluate(ctx, input)
		if err != nil {
			results = append(results, EvalResult{
				Name:      e.Name(),
				Score:     0,
				Pass:      false,
				Reasoning: fmt.Sprintf("evaluation error: %s", err.Error()),
			})
			continue
		}
		results = append(results, *result)
	}
	return results, nil
}

// DefaultSuite creates an eval suite with all standard evaluators.
func DefaultSuite(gw llm.Gateway) *EvalSuite {
	suite := NewEvalSuite()
	suite.Add(NewLLMJudge(gw, ""))
	suite.Add(NewHallucinationDetector(gw, ""))
	suite.Add(NewRelevanceEvaluator(gw, ""))
	suite.Add(NewFaithfulnessEvaluator(gw, ""))
	return suite
}

// RelevanceEvaluator checks if the response is relevant to the query.
type RelevanceEvaluator struct {
	gateway llm.Gateway
	model   string
}

func NewRelevanceEvaluator(gw llm.Gateway, model string) *RelevanceEvaluator {
	if model == "" {
		model = "gpt-4o-mini"
	}
	return &RelevanceEvaluator{gateway: gw, model: model}
}

func (e *RelevanceEvaluator) Name() string { return "relevance" }

func (e *RelevanceEvaluator) Evaluate(ctx context.Context, input EvalInput) (*EvalResult, error) {
	start := time.Now()

	resp, err := e.gateway.Chat(ctx, llm.ChatRequest{
		Model: e.model,
		Messages: []llm.Message{
			{
				Role: "system",
				Content: `Rate how relevant the response is to the question on a scale of 0.0 to 1.0.
Consider: Does the response address the question? Is it on-topic?
Reply with ONLY a JSON object: {"score": 0.0, "reasoning": "brief explanation"}`,
			},
			{
				Role:    "user",
				Content: fmt.Sprintf("Question: %s\n\nResponse: %s", input.Query, input.Response),
			},
		},
		Temperature: 0,
	})
	if err != nil {
		return nil, err
	}

	return parseEvalJSON(e.Name(), resp.Content, time.Since(start))
}

// FaithfulnessEvaluator checks if the response is faithful to the provided context.
// This is critical for RAG — does the answer stick to what the documents say?
type FaithfulnessEvaluator struct {
	gateway llm.Gateway
	model   string
}

func NewFaithfulnessEvaluator(gw llm.Gateway, model string) *FaithfulnessEvaluator {
	if model == "" {
		model = "gpt-4o-mini"
	}
	return &FaithfulnessEvaluator{gateway: gw, model: model}
}

func (e *FaithfulnessEvaluator) Name() string { return "faithfulness" }

func (e *FaithfulnessEvaluator) Evaluate(ctx context.Context, input EvalInput) (*EvalResult, error) {
	if len(input.Context) == 0 {
		return &EvalResult{Name: e.Name(), Score: 1.0, Pass: true, Reasoning: "no context to check against"}, nil
	}

	start := time.Now()
	contextStr := strings.Join(input.Context, "\n---\n")

	resp, err := e.gateway.Chat(ctx, llm.ChatRequest{
		Model: e.model,
		Messages: []llm.Message{
			{
				Role: "system",
				Content: `You are evaluating faithfulness: does the response ONLY contain information
supported by the provided context? Score from 0.0 (completely unfaithful) to 1.0 (completely faithful).
Penalize any claims not supported by the context.
Reply with ONLY a JSON object: {"score": 0.0, "reasoning": "brief explanation"}`,
			},
			{
				Role: "user",
				Content: fmt.Sprintf("Context:\n%s\n\nResponse: %s", contextStr, input.Response),
			},
		},
		Temperature: 0,
	})
	if err != nil {
		return nil, err
	}

	return parseEvalJSON(e.Name(), resp.Content, time.Since(start))
}

func parseEvalJSON(name, content string, duration time.Duration) (*EvalResult, error) {
	content = strings.TrimSpace(content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	var parsed struct {
		Score     float64 `json:"score"`
		Reasoning string  `json:"reasoning"`
	}
	if err := json.Unmarshal([]byte(content), &parsed); err != nil {
		return &EvalResult{Name: name, Score: 0, Pass: false, Reasoning: "failed to parse eval response"}, nil
	}

	return &EvalResult{
		Name:      name,
		Score:     parsed.Score,
		Pass:      parsed.Score >= 0.5,
		Reasoning: parsed.Reasoning,
		Duration:  duration,
	}, nil
}
