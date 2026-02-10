package agent

import (
	"strings"
)

// parsedResponse holds the components extracted from a ReAct-formatted LLM response.
type parsedResponse struct {
	Thought     string
	Action      string
	ActionInput string
	FinalAnswer string
}

// parseReActResponse extracts Thought, Action, Action Input, and Final Answer
// from a ReAct-formatted LLM response.
func parseReActResponse(content string) parsedResponse {
	var result parsedResponse

	lines := strings.Split(content, "\n")
	var currentSection string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		switch {
		case strings.HasPrefix(trimmed, "Thought:"):
			currentSection = "thought"
			result.Thought = strings.TrimSpace(strings.TrimPrefix(trimmed, "Thought:"))

		case strings.HasPrefix(trimmed, "Action:"):
			currentSection = "action"
			result.Action = strings.TrimSpace(strings.TrimPrefix(trimmed, "Action:"))

		case strings.HasPrefix(trimmed, "Action Input:"):
			currentSection = "action_input"
			result.ActionInput = strings.TrimSpace(strings.TrimPrefix(trimmed, "Action Input:"))

		case strings.HasPrefix(trimmed, "Final Answer:"):
			currentSection = "final_answer"
			result.FinalAnswer = strings.TrimSpace(strings.TrimPrefix(trimmed, "Final Answer:"))

		default:
			// Continuation of the current section
			switch currentSection {
			case "thought":
				result.Thought += "\n" + trimmed
			case "action_input":
				result.ActionInput += "\n" + trimmed
			case "final_answer":
				result.FinalAnswer += "\n" + trimmed
			}
		}
	}

	// Trim all results
	result.Thought = strings.TrimSpace(result.Thought)
	result.Action = strings.TrimSpace(result.Action)
	result.ActionInput = strings.TrimSpace(result.ActionInput)
	result.FinalAnswer = strings.TrimSpace(result.FinalAnswer)

	return result
}
