package guardrails

import (
	"context"
	"strings"
)

// ContentFilter checks for harmful, toxic, or inappropriate content
// using keyword-based heuristics.
type ContentFilter struct {
	blockedCategories map[string][]string
}

func NewContentFilter() *ContentFilter {
	return &ContentFilter{
		blockedCategories: map[string][]string{
			"violence": {
				"how to make a bomb", "how to make explosives",
				"how to harm", "how to kill",
			},
			"illegal": {
				"how to hack into", "how to steal",
				"how to counterfeit", "how to forge",
			},
			"malware": {
				"write malware", "create a virus",
				"write ransomware", "create a trojan",
			},
		},
	}
}

func (f *ContentFilter) Name() string { return "content_filter" }

func (f *ContentFilter) Check(_ context.Context, text string) (*GuardrailResult, error) {
	lower := strings.ToLower(text)
	var flags []string

	for category, patterns := range f.blockedCategories {
		for _, p := range patterns {
			if strings.Contains(lower, p) {
				flags = append(flags, "blocked_"+category)
				return &GuardrailResult{
					Allowed: false,
					Reason:  "content policy violation: " + category,
					Flags:   flags,
					Scores:  map[string]float64{category: 1.0},
				}, nil
			}
		}
	}

	return &GuardrailResult{Allowed: true}, nil
}
