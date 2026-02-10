package guardrails

import (
	"context"
	"fmt"
	"strings"

	"github.com/nikhilbhutani/backendwithai/internal/llm"
)

// GuardrailResult holds the outcome of a safety check.
type GuardrailResult struct {
	Allowed bool              `json:"allowed"`
	Flags   []string          `json:"flags,omitempty"`
	Scores  map[string]float64 `json:"scores,omitempty"`
	Reason  string            `json:"reason,omitempty"`
}

// Guardrail is a check that can be applied to input or output.
type Guardrail interface {
	Check(ctx context.Context, text string) (*GuardrailResult, error)
	Name() string
}

// Pipeline chains multiple guardrails together.
type Pipeline struct {
	inputGuardrails  []Guardrail
	outputGuardrails []Guardrail
}

func NewPipeline() *Pipeline {
	return &Pipeline{}
}

func (p *Pipeline) AddInputGuardrail(g Guardrail) {
	p.inputGuardrails = append(p.inputGuardrails, g)
}

func (p *Pipeline) AddOutputGuardrail(g Guardrail) {
	p.outputGuardrails = append(p.outputGuardrails, g)
}

// CheckInput runs all input guardrails against the text.
func (p *Pipeline) CheckInput(ctx context.Context, text string) (*GuardrailResult, error) {
	return p.runChecks(ctx, text, p.inputGuardrails)
}

// CheckOutput runs all output guardrails against the text.
func (p *Pipeline) CheckOutput(ctx context.Context, text string) (*GuardrailResult, error) {
	return p.runChecks(ctx, text, p.outputGuardrails)
}

func (p *Pipeline) runChecks(ctx context.Context, text string, guards []Guardrail) (*GuardrailResult, error) {
	combined := &GuardrailResult{
		Allowed: true,
		Scores:  make(map[string]float64),
	}

	for _, g := range guards {
		result, err := g.Check(ctx, text)
		if err != nil {
			return nil, fmt.Errorf("guardrail %s: %w", g.Name(), err)
		}
		if !result.Allowed {
			combined.Allowed = false
			combined.Reason = fmt.Sprintf("blocked by %s: %s", g.Name(), result.Reason)
		}
		combined.Flags = append(combined.Flags, result.Flags...)
		for k, v := range result.Scores {
			combined.Scores[k] = v
		}
	}

	return combined, nil
}

// DefaultPipeline creates a guardrail pipeline with all built-in checks.
func DefaultPipeline(gw llm.Gateway) *Pipeline {
	p := NewPipeline()

	// Input guardrails
	p.AddInputGuardrail(NewPromptInjectionDetector(gw))
	p.AddInputGuardrail(NewContentFilter())
	p.AddInputGuardrail(NewInputLengthGuard(50000)) // 50K char max

	// Output guardrails
	p.AddOutputGuardrail(NewContentFilter())
	p.AddOutputGuardrail(NewPIIDetector())

	return p
}

// InputLengthGuard rejects inputs that are too long.
type InputLengthGuard struct {
	maxLength int
}

func NewInputLengthGuard(maxLen int) *InputLengthGuard {
	return &InputLengthGuard{maxLength: maxLen}
}

func (g *InputLengthGuard) Name() string { return "input_length" }

func (g *InputLengthGuard) Check(_ context.Context, text string) (*GuardrailResult, error) {
	if len(text) > g.maxLength {
		return &GuardrailResult{
			Allowed: false,
			Reason:  fmt.Sprintf("input exceeds %d characters", g.maxLength),
			Flags:   []string{"input_too_long"},
		}, nil
	}
	return &GuardrailResult{Allowed: true}, nil
}

// PIIDetector checks for common PII patterns in output.
type PIIDetector struct{}

func NewPIIDetector() *PIIDetector { return &PIIDetector{} }

func (d *PIIDetector) Name() string { return "pii_detector" }

func (d *PIIDetector) Check(_ context.Context, text string) (*GuardrailResult, error) {
	var flags []string
	lower := strings.ToLower(text)

	piiPatterns := map[string][]string{
		"ssn_pattern":         {"social security", "ssn"},
		"credit_card_pattern": {"credit card", "card number"},
		"password_pattern":    {"password is", "my password"},
	}

	for flag, patterns := range piiPatterns {
		for _, p := range patterns {
			if strings.Contains(lower, p) {
				flags = append(flags, flag)
				break
			}
		}
	}

	return &GuardrailResult{
		Allowed: true, // flag but don't block
		Flags:   flags,
	}, nil
}
