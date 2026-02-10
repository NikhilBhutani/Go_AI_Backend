package eval

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/nikhilbhutani/backendwithai/internal/llm"
)

// HallucinationDetector checks if the LLM's response contains information
// not grounded in the provided context (for RAG) or factually incorrect claims.
type HallucinationDetector struct {
	gateway llm.Gateway
	model   string
}

func NewHallucinationDetector(gw llm.Gateway, model string) *HallucinationDetector {
	if model == "" {
		model = "gpt-4o"
	}
	return &HallucinationDetector{gateway: gw, model: model}
}

func (d *HallucinationDetector) Name() string { return "hallucination" }

// Evaluate checks the response for hallucinated content.
// If context documents are provided, it checks groundedness.
// Otherwise, it checks for obviously false or fabricated claims.
func (d *HallucinationDetector) Evaluate(ctx context.Context, input EvalInput) (*EvalResult, error) {
	start := time.Now()

	var systemPrompt string
	var userPrompt string

	if len(input.Context) > 0 {
		// Grounded hallucination check — RAG scenario
		contextStr := strings.Join(input.Context, "\n---\n")
		systemPrompt = `You are an expert hallucination detector for RAG systems.
Given the source context and a response, identify any claims in the response
that are NOT supported by the provided context.

Score from 0.0 to 1.0 where:
- 1.0 = fully grounded, every claim is supported by the context
- 0.5 = some claims are unsupported but key facts are correct
- 0.0 = mostly hallucinated, contains fabricated information

Reply with ONLY a JSON object:
{"score": 0.0, "reasoning": "brief explanation of any hallucinated claims found"}`

		userPrompt = fmt.Sprintf("Source Context:\n%s\n\nResponse to check:\n%s", contextStr, input.Response)
	} else {
		// Open-ended hallucination check — no context available
		systemPrompt = `You are an expert fact-checker. Evaluate whether the response
contains obviously false, fabricated, or nonsensical claims.

Score from 0.0 to 1.0 where:
- 1.0 = all claims appear factually reasonable
- 0.5 = some questionable claims but mostly reasonable
- 0.0 = contains clearly false or fabricated information

Reply with ONLY a JSON object:
{"score": 0.0, "reasoning": "brief explanation"}`

		userPrompt = fmt.Sprintf("Question: %s\n\nResponse to check:\n%s", input.Query, input.Response)
	}

	if input.GroundTruth != "" {
		userPrompt += fmt.Sprintf("\n\nGround Truth (known correct answer): %s", input.GroundTruth)
	}

	resp, err := d.gateway.Chat(ctx, llm.ChatRequest{
		Model: d.model,
		Messages: []llm.Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		Temperature: 0,
	})
	if err != nil {
		return nil, fmt.Errorf("hallucination detector: %w", err)
	}

	return parseEvalJSON(d.Name(), resp.Content, time.Since(start))
}

// ClaimExtractor breaks a response into individual claims and verifies each one.
// More granular than the overall hallucination score.
type ClaimExtractor struct {
	gateway llm.Gateway
	model   string
}

func NewClaimExtractor(gw llm.Gateway, model string) *ClaimExtractor {
	if model == "" {
		model = "gpt-4o"
	}
	return &ClaimExtractor{gateway: gw, model: model}
}

// ClaimVerification holds the result of verifying a single claim.
type ClaimVerification struct {
	Claim     string  `json:"claim"`
	Supported bool    `json:"supported"`
	Evidence  string  `json:"evidence"`
	Score     float64 `json:"score"`
}

// ExtractAndVerify decomposes a response into claims and checks each against context.
func (ce *ClaimExtractor) ExtractAndVerify(ctx context.Context, response string, context []string) ([]ClaimVerification, error) {
	contextStr := strings.Join(context, "\n---\n")

	resp, err := ce.gateway.Chat(ctx, llm.ChatRequest{
		Model: ce.model,
		Messages: []llm.Message{
			{
				Role: "system",
				Content: `Extract each factual claim from the response and verify it against the context.
Reply with ONLY a JSON array:
[{"claim": "...", "supported": true/false, "evidence": "quote from context or 'not found'", "score": 0.0-1.0}]`,
			},
			{
				Role:    "user",
				Content: fmt.Sprintf("Context:\n%s\n\nResponse:\n%s", contextStr, response),
			},
		},
		Temperature: 0,
	})
	if err != nil {
		return nil, fmt.Errorf("claim extraction: %w", err)
	}

	content := strings.TrimSpace(resp.Content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	var claims []ClaimVerification
	if err := json.Unmarshal([]byte(content), &claims); err != nil {
		return nil, fmt.Errorf("failed to parse claims: %w", err)
	}

	return claims, nil
}
