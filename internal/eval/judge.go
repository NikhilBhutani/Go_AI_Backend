package eval

import (
	"context"
	"fmt"
	"time"

	"github.com/nikhilbhutani/backendwithai/internal/llm"
)

// LLMJudge uses an LLM to evaluate the quality of another LLM's response.
// This implements the "LLM as a Judge" pattern from evaluation research.
type LLMJudge struct {
	gateway llm.Gateway
	model   string
}

func NewLLMJudge(gw llm.Gateway, model string) *LLMJudge {
	if model == "" {
		model = "gpt-4o"
	}
	return &LLMJudge{gateway: gw, model: model}
}

func (j *LLMJudge) Name() string { return "llm_judge" }

// Evaluate performs a comprehensive quality assessment of the response.
func (j *LLMJudge) Evaluate(ctx context.Context, input EvalInput) (*EvalResult, error) {
	start := time.Now()

	prompt := fmt.Sprintf(`Question: %s

Response being evaluated: %s`, input.Query, input.Response)

	if input.ExpectedAnswer != "" {
		prompt += fmt.Sprintf("\n\nExpected/Reference Answer: %s", input.ExpectedAnswer)
	}

	resp, err := j.gateway.Chat(ctx, llm.ChatRequest{
		Model: j.model,
		Messages: []llm.Message{
			{
				Role: "system",
				Content: `You are an expert AI response evaluator. Score the response on these dimensions:

1. **Accuracy** (0-1): Is the information correct?
2. **Completeness** (0-1): Does it fully answer the question?
3. **Clarity** (0-1): Is it well-written and easy to understand?
4. **Helpfulness** (0-1): Is it useful to the person asking?

Reply with ONLY a JSON object:
{
  "score": 0.0,
  "details": {"accuracy": 0.0, "completeness": 0.0, "clarity": 0.0, "helpfulness": 0.0},
  "reasoning": "brief explanation"
}

The overall "score" should be the weighted average: accuracy(0.4) + completeness(0.3) + clarity(0.15) + helpfulness(0.15)`,
			},
			{Role: "user", Content: prompt},
		},
		Temperature: 0,
	})
	if err != nil {
		return nil, fmt.Errorf("llm judge: %w", err)
	}

	result, err := parseEvalJSON(j.Name(), resp.Content, time.Since(start))
	if err != nil {
		return nil, err
	}

	return result, nil
}

// PairwiseJudge compares two responses and determines which is better.
// Useful for A/B testing prompts or models.
type PairwiseJudge struct {
	gateway llm.Gateway
	model   string
}

func NewPairwiseJudge(gw llm.Gateway, model string) *PairwiseJudge {
	if model == "" {
		model = "gpt-4o"
	}
	return &PairwiseJudge{gateway: gw, model: model}
}

// ComparisonResult holds the outcome of a pairwise comparison.
type ComparisonResult struct {
	Winner    string `json:"winner"`    // "A", "B", or "tie"
	Reasoning string `json:"reasoning"`
	ScoreA    float64 `json:"score_a"`
	ScoreB    float64 `json:"score_b"`
}

// Compare evaluates two responses to the same query and picks the better one.
func (j *PairwiseJudge) Compare(ctx context.Context, query, responseA, responseB string) (*ComparisonResult, error) {
	resp, err := j.gateway.Chat(ctx, llm.ChatRequest{
		Model: j.model,
		Messages: []llm.Message{
			{
				Role: "system",
				Content: `Compare two AI responses to the same question. Evaluate accuracy, completeness,
clarity, and helpfulness. Reply with ONLY a JSON object:
{"winner": "A" or "B" or "tie", "score_a": 0.0, "score_b": 0.0, "reasoning": "explanation"}`,
			},
			{
				Role: "user",
				Content: fmt.Sprintf("Question: %s\n\nResponse A:\n%s\n\nResponse B:\n%s", query, responseA, responseB),
			},
		},
		Temperature: 0,
	})
	if err != nil {
		return nil, fmt.Errorf("pairwise judge: %w", err)
	}

	result, _ := parseEvalJSON("pairwise", resp.Content, 0)
	return &ComparisonResult{
		ScoreA:    result.Score,
		Reasoning: result.Reasoning,
	}, nil
}
