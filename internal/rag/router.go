package rag

import (
	"context"
	"fmt"
	"strings"

	"github.com/nikhilbhutani/backendwithai/internal/llm"
)

// QueryRoute describes how to handle a query.
type QueryRoute struct {
	Strategy    string // "simple", "complex", "comparison"
	UseDecompose bool
	UseHyDE     bool
	UseRewrite  bool
	Reasoning   string
}

// QueryRouter classifies queries and selects the retrieval strategy.
type QueryRouter interface {
	Route(ctx context.Context, query string) (*QueryRoute, error)
}

// LLMQueryRouter uses an LLM to classify queries.
type LLMQueryRouter struct {
	gateway llm.Gateway
	model   string
}

func NewLLMQueryRouter(gw llm.Gateway, model string) *LLMQueryRouter {
	if model == "" {
		model = "gpt-4o-mini"
	}
	return &LLMQueryRouter{gateway: gw, model: model}
}

// Route classifies the query and returns the appropriate retrieval strategy.
// Falls back to "simple" on error.
func (r *LLMQueryRouter) Route(ctx context.Context, query string) (*QueryRoute, error) {
	resp, err := r.gateway.Chat(ctx, llm.ChatRequest{
		Model: r.model,
		Messages: []llm.Message{
			{
				Role: "system",
				Content: `You are a query classifier for a RAG system. Classify the user query into one of three strategies:

- simple: A factual, single-hop question that can be answered by looking up one piece of information.
- complex: A multi-part or analytical question requiring synthesis of multiple pieces of information.
- comparison: A question comparing two or more entities, concepts, or options.

Respond in exactly this format (no extra text):
STRATEGY: <simple|complex|comparison>
REASONING: <one sentence explanation>`,
			},
			{Role: "user", Content: query},
		},
		Temperature: 0,
	})
	if err != nil {
		return defaultRoute(), nil
	}

	return parseRoute(resp.Content), nil
}

func parseRoute(content string) *QueryRoute {
	route := defaultRoute()
	for _, line := range strings.Split(strings.TrimSpace(content), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "STRATEGY:") {
			strategy := strings.TrimSpace(strings.TrimPrefix(line, "STRATEGY:"))
			switch strategy {
			case "simple", "complex", "comparison":
				route.Strategy = strategy
			}
		} else if strings.HasPrefix(line, "REASONING:") {
			route.Reasoning = strings.TrimSpace(strings.TrimPrefix(line, "REASONING:"))
		}
	}

	switch route.Strategy {
	case "complex":
		route.UseDecompose = true
	case "comparison":
		route.UseRewrite = true
	}

	return route
}

func defaultRoute() *QueryRoute {
	return &QueryRoute{
		Strategy:  "simple",
		Reasoning: "default fallback",
	}
}

// RouteInfo is included in QueryResponse to expose routing decisions.
type RouteInfo struct {
	Strategy  string `json:"strategy"`
	Reasoning string `json:"reasoning"`
}

func routeInfoFromRoute(r *QueryRoute) RouteInfo {
	if r == nil {
		return RouteInfo{Strategy: "simple"}
	}
	return RouteInfo{Strategy: r.Strategy, Reasoning: r.Reasoning}
}

// Ensure LLMQueryRouter implements QueryRouter.
var _ QueryRouter = (*LLMQueryRouter)(nil)

// noopRouter always returns "simple" — used when no router is configured.
type noopRouter struct{}

func (noopRouter) Route(_ context.Context, _ string) (*QueryRoute, error) {
	return defaultRoute(), nil
}

// validateStrategy returns an error for unknown strategy strings.
func validateStrategy(s string) error {
	switch s {
	case "", "simple", "complex", "comparison":
		return nil
	}
	return fmt.Errorf("unknown routing strategy %q", s)
}
