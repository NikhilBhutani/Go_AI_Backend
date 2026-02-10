package finetune

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// TrainingExample represents a single training example in chat format.
type TrainingExample struct {
	Messages []TrainingMessage `json:"messages"`
}

type TrainingMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ValidateJSONL validates a JSONL training file and returns the record count.
func ValidateJSONL(r io.Reader) (int, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024) // 1MB buffer per line

	count := 0
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var example TrainingExample
		if err := json.Unmarshal([]byte(line), &example); err != nil {
			return count, fmt.Errorf("line %d: invalid JSON: %w", count+1, err)
		}

		if len(example.Messages) < 2 {
			return count, fmt.Errorf("line %d: need at least 2 messages (user + assistant)", count+1)
		}

		hasUser := false
		hasAssistant := false
		for _, m := range example.Messages {
			switch m.Role {
			case "system", "user", "assistant":
				if m.Role == "user" {
					hasUser = true
				}
				if m.Role == "assistant" {
					hasAssistant = true
				}
			default:
				return count, fmt.Errorf("line %d: invalid role %q", count+1, m.Role)
			}
			if m.Content == "" {
				return count, fmt.Errorf("line %d: empty content for role %s", count+1, m.Role)
			}
		}

		if !hasUser || !hasAssistant {
			return count, fmt.Errorf("line %d: need at least one user and one assistant message", count+1)
		}

		count++
	}

	if err := scanner.Err(); err != nil {
		return count, fmt.Errorf("scan error: %w", err)
	}

	if count == 0 {
		return 0, fmt.Errorf("empty dataset")
	}

	return count, nil
}

// FormatToJSONL converts training examples to JSONL format.
func FormatToJSONL(examples []TrainingExample, w io.Writer) error {
	for i, ex := range examples {
		data, err := json.Marshal(ex)
		if err != nil {
			return fmt.Errorf("marshal example %d: %w", i, err)
		}
		if _, err := fmt.Fprintf(w, "%s\n", data); err != nil {
			return fmt.Errorf("write example %d: %w", i, err)
		}
	}
	return nil
}
