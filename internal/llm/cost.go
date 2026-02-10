package llm

// costPerToken stores per-1K-token pricing for known models.
// Prices in USD per 1K tokens: [input, output].
var costPerToken = map[string][2]float64{
	// OpenAI
	"gpt-4":             {0.03, 0.06},
	"gpt-4-turbo":       {0.01, 0.03},
	"gpt-4o":            {0.005, 0.015},
	"gpt-4o-mini":       {0.00015, 0.0006},
	"gpt-3.5-turbo":     {0.0005, 0.0015},
	"text-embedding-ada-002":    {0.0001, 0},
	"text-embedding-3-small":    {0.00002, 0},
	"text-embedding-3-large":    {0.00013, 0},

	// Anthropic
	"claude-3-opus-20240229":   {0.015, 0.075},
	"claude-3-sonnet-20240229": {0.003, 0.015},
	"claude-3-haiku-20240307":  {0.00025, 0.00125},
	"claude-sonnet-4-20250514": {0.003, 0.015},
	"claude-opus-4-20250514":   {0.015, 0.075},
}

func CalculateCost(model string, inputTokens, outputTokens int) float64 {
	prices, ok := costPerToken[model]
	if !ok {
		return 0
	}
	inputCost := float64(inputTokens) / 1000.0 * prices[0]
	outputCost := float64(outputTokens) / 1000.0 * prices[1]
	return inputCost + outputCost
}
