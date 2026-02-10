package reasoning

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/nikhilbhutani/backendwithai/internal/llm"
)

// TreeOfThought implements the Tree-of-Thought reasoning pattern.
// It explores multiple reasoning paths in parallel and selects the best one.
type TreeOfThought struct {
	gateway    llm.Gateway
	model      string
	branches   int // number of parallel reasoning paths
	maxDepth   int // maximum depth of exploration
}

func NewTreeOfThought(gw llm.Gateway, model string, branches, maxDepth int) *TreeOfThought {
	if branches <= 0 {
		branches = 3
	}
	if maxDepth <= 0 {
		maxDepth = 3
	}
	return &TreeOfThought{
		gateway:  gw,
		model:    model,
		branches: branches,
		maxDepth: maxDepth,
	}
}

// ThoughtNode represents a single node in the reasoning tree.
type ThoughtNode struct {
	Thought    string         `json:"thought"`
	Score      float64        `json:"score"`
	Children   []*ThoughtNode `json:"children,omitempty"`
	IsTerminal bool           `json:"is_terminal"`
	Answer     string         `json:"answer,omitempty"`
}

// TreeResult holds the outcome of tree-of-thought reasoning.
type TreeResult struct {
	BestPath    []*ThoughtNode `json:"best_path"`
	FinalAnswer string         `json:"final_answer"`
	AllPaths    [][]*ThoughtNode `json:"all_paths"`
	Model       string         `json:"model"`
}

// Reason explores multiple reasoning paths and returns the best result.
func (t *TreeOfThought) Reason(ctx context.Context, query string) (*TreeResult, error) {
	// Generate initial thoughts
	initialThoughts, err := t.generateThoughts(ctx, query, nil)
	if err != nil {
		return nil, fmt.Errorf("tot generate initial: %w", err)
	}

	// Score initial thoughts
	for i := range initialThoughts {
		score, err := t.evaluateThought(ctx, query, initialThoughts[i].Thought)
		if err != nil {
			continue
		}
		initialThoughts[i].Score = score
	}

	// Sort by score and keep top branches
	sort.Slice(initialThoughts, func(i, j int) bool {
		return initialThoughts[i].Score > initialThoughts[j].Score
	})
	if len(initialThoughts) > t.branches {
		initialThoughts = initialThoughts[:t.branches]
	}

	// Expand the tree depth-first for each branch
	var allPaths [][]*ThoughtNode
	for _, root := range initialThoughts {
		path := []*ThoughtNode{root}
		completedPath := t.expandPath(ctx, query, path, 1)
		allPaths = append(allPaths, completedPath)
	}

	// Find the best path
	bestPath := allPaths[0]
	bestScore := pathScore(bestPath)
	for _, path := range allPaths[1:] {
		if s := pathScore(path); s > bestScore {
			bestScore = s
			bestPath = path
		}
	}

	// Extract final answer from best path's last node
	finalAnswer := ""
	if len(bestPath) > 0 {
		last := bestPath[len(bestPath)-1]
		if last.Answer != "" {
			finalAnswer = last.Answer
		} else {
			finalAnswer = last.Thought
		}
	}

	return &TreeResult{
		BestPath:    bestPath,
		FinalAnswer: finalAnswer,
		AllPaths:    allPaths,
		Model:       t.model,
	}, nil
}

func (t *TreeOfThought) expandPath(ctx context.Context, query string, path []*ThoughtNode, depth int) []*ThoughtNode {
	if depth >= t.maxDepth {
		return path
	}

	currentThought := path[len(path)-1].Thought
	nextThoughts, err := t.generateThoughts(ctx, query, &currentThought)
	if err != nil || len(nextThoughts) == 0 {
		return path
	}

	// Score and pick best continuation
	best := nextThoughts[0]
	bestScore := 0.0
	for i := range nextThoughts {
		score, err := t.evaluateThought(ctx, query, nextThoughts[i].Thought)
		if err != nil {
			continue
		}
		nextThoughts[i].Score = score
		if score > bestScore {
			bestScore = score
			best = nextThoughts[i]
		}
	}

	return t.expandPath(ctx, query, append(path, best), depth+1)
}

func (t *TreeOfThought) generateThoughts(ctx context.Context, query string, previousThought *string) ([]*ThoughtNode, error) {
	prompt := fmt.Sprintf("Problem: %s\n\n", query)
	if previousThought != nil {
		prompt += fmt.Sprintf("Previous reasoning step: %s\n\n", *previousThought)
		prompt += fmt.Sprintf("Generate %d possible next reasoning steps.", t.branches)
	} else {
		prompt += fmt.Sprintf("Generate %d different initial approaches to solve this.", t.branches)
	}

	resp, err := t.gateway.Chat(ctx, llm.ChatRequest{
		Model: t.model,
		Messages: []llm.Message{
			{
				Role: "system",
				Content: fmt.Sprintf(`Generate exactly %d distinct reasoning steps as a JSON array.
Each step should be a different approach or continuation.
Reply with ONLY: [{"thought": "...", "answer": "..." or null}]
Set "answer" only if this step reaches a final conclusion.`, t.branches),
			},
			{Role: "user", Content: prompt},
		},
		Temperature: 0.7,
	})
	if err != nil {
		return nil, err
	}

	content := strings.TrimSpace(resp.Content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	var thoughts []*ThoughtNode
	if err := json.Unmarshal([]byte(content), &thoughts); err != nil {
		// Fallback: treat entire response as a single thought
		return []*ThoughtNode{{Thought: resp.Content}}, nil
	}

	return thoughts, nil
}

func (t *TreeOfThought) evaluateThought(ctx context.Context, query, thought string) (float64, error) {
	resp, err := t.gateway.Chat(ctx, llm.ChatRequest{
		Model: t.model,
		Messages: []llm.Message{
			{
				Role:    "system",
				Content: `Rate this reasoning step from 0.0 to 1.0 for logical soundness, relevance, and progress toward the answer. Reply with ONLY a JSON object: {"score": 0.0}`,
			},
			{
				Role:    "user",
				Content: fmt.Sprintf("Problem: %s\nReasoning step: %s", query, thought),
			},
		},
		Temperature: 0,
	})
	if err != nil {
		return 0.5, nil // default score on error
	}

	var result struct {
		Score float64 `json:"score"`
	}
	content := strings.TrimSpace(resp.Content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	if err := json.Unmarshal([]byte(strings.TrimSpace(content)), &result); err != nil {
		return 0.5, nil
	}

	return result.Score, nil
}

func pathScore(path []*ThoughtNode) float64 {
	if len(path) == 0 {
		return 0
	}
	total := 0.0
	for _, n := range path {
		total += n.Score
	}
	return total / float64(len(path))
}

// SelfConsistency implements the self-consistency decoding pattern.
// Samples multiple reasoning chains and takes the majority answer.
type SelfConsistency struct {
	gateway llm.Gateway
	model   string
	samples int
}

func NewSelfConsistency(gw llm.Gateway, model string, samples int) *SelfConsistency {
	if samples <= 0 {
		samples = 5
	}
	return &SelfConsistency{gateway: gw, model: model, samples: samples}
}

// ConsistencyResult holds the outcome of self-consistency reasoning.
type ConsistencyResult struct {
	MajorityAnswer string            `json:"majority_answer"`
	Confidence     float64           `json:"confidence"` // fraction that agreed
	AnswerCounts   map[string]int    `json:"answer_counts"`
	Samples        []ReasoningResult `json:"samples"`
}

// Reason samples multiple reasoning chains and returns the most common answer.
func (sc *SelfConsistency) Reason(ctx context.Context, query string) (*ConsistencyResult, error) {
	cot := NewChainOfThought(sc.gateway, sc.model, StrategyZeroShot)

	var mu sync.Mutex
	samples := make([]ReasoningResult, 0, sc.samples)
	var wg sync.WaitGroup

	for i := 0; i < sc.samples; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := cot.Reason(ctx, query, nil)
			if err != nil {
				return
			}
			mu.Lock()
			samples = append(samples, *result)
			mu.Unlock()
		}()
	}
	wg.Wait()

	if len(samples) == 0 {
		return nil, fmt.Errorf("self-consistency: all samples failed")
	}

	// Count answers (normalize for comparison)
	counts := make(map[string]int)
	for _, s := range samples {
		key := strings.TrimSpace(strings.ToLower(s.FinalAnswer))
		counts[key]++
	}

	// Find majority
	majorityAnswer := ""
	maxCount := 0
	for answer, count := range counts {
		if count > maxCount {
			maxCount = count
			majorityAnswer = answer
		}
	}

	// Use the original-cased version from samples
	for _, s := range samples {
		if strings.TrimSpace(strings.ToLower(s.FinalAnswer)) == majorityAnswer {
			majorityAnswer = s.FinalAnswer
			break
		}
	}

	return &ConsistencyResult{
		MajorityAnswer: majorityAnswer,
		Confidence:     float64(maxCount) / float64(len(samples)),
		AnswerCounts:   counts,
		Samples:        samples,
	}, nil
}
