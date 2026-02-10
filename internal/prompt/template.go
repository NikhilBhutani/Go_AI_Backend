package prompt

import (
	"fmt"
	"regexp"
	"strings"
)

var variablePattern = regexp.MustCompile(`\{\{(\w+)\}\}`)

// Render replaces {{variable}} placeholders in the template with values from vars.
func Render(template string, vars map[string]string) (string, error) {
	missing := findMissingVars(template, vars)
	if len(missing) > 0 {
		return "", fmt.Errorf("missing template variables: %s", strings.Join(missing, ", "))
	}

	result := variablePattern.ReplaceAllStringFunc(template, func(match string) string {
		key := match[2 : len(match)-2] // strip {{ and }}
		if val, ok := vars[key]; ok {
			return val
		}
		return match
	})

	return result, nil
}

// ExtractVariables returns a list of variable names found in the template.
func ExtractVariables(template string) []string {
	matches := variablePattern.FindAllStringSubmatch(template, -1)
	seen := make(map[string]bool)
	var vars []string
	for _, m := range matches {
		if len(m) > 1 && !seen[m[1]] {
			vars = append(vars, m[1])
			seen[m[1]] = true
		}
	}
	return vars
}

func findMissingVars(template string, vars map[string]string) []string {
	required := ExtractVariables(template)
	var missing []string
	for _, v := range required {
		if _, ok := vars[v]; !ok {
			missing = append(missing, v)
		}
	}
	return missing
}
