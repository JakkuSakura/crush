package tools

import (
	"context"
	"fmt"
	"strings"

	"charm.land/fantasy"
)

// NewLLMCommandClassifier creates a CommandClassifier that uses the given
// fantasy.LanguageModel to review dangerous commands. The LLM is asked to
// decide whether the command is safe to execute, and can suggest a safer
// alternative.
func NewLLMCommandClassifier(lm fantasy.LanguageModel) CommandClassifier {
	return func(ctx context.Context, command, description string) (bool, string, string) {
		prompt := fmt.Sprintf(`You are a security classifier for an AI coding assistant.
A command has been flagged as potentially dangerous.

Command: %s
Description: %s

Review this command carefully. Decide if it is safe to execute in the current context.
If it is safe, respond with exactly "APPROVED" followed by an optional modified version of the command on the next line.
If it is NOT safe, respond with exactly "DENIED" followed by a brief explanation on the next line.

Examples of when to DENY:
- Installing system packages (apt, brew, etc.)
- Running curl/wget to download and pipe to shell
- SSH to unknown hosts
- System configuration changes (systemctl, iptables, etc.)

Examples of when to APPROVE:
- Using curl/wget to fetch data (not piped to shell)
- SSH with safe flags to known hosts (e.g., checking if a port is open with -o ConnectTimeout)
- Package managers when installing known safe packages (with proper flags)
- Using ip/ifconfig for informational purposes only

Response format (exactly one of):
APPROVED
[optional modified command]

DENIED
[reason]`, command, description)

		resp, err := lm.Generate(ctx, fantasy.Call{
			Prompt: fantasy.Prompt{
				Messages: []fantasy.Message{
					{Role: "user", Content: []fantasy.Content{fantasy.TextContent{Text: prompt}}},
				},
			},
			MaxOutputTokens: ptr(int64(256)),
			Temperature:     ptr(float64(0.1)),
		})
		if err != nil {
			return false, "", fmt.Sprintf("classifier error: %v", err)
		}

		text := resp.Content.Text()
		text = strings.TrimSpace(text)

		if strings.HasPrefix(text, "APPROVED") {
			rest := strings.TrimSpace(strings.TrimPrefix(text, "APPROVED"))
			return true, rest, ""
		}

		if strings.HasPrefix(text, "DENIED") {
			reason := strings.TrimSpace(strings.TrimPrefix(text, "DENIED"))
			return false, "", reason
		}

		// If the LLM didn't follow the format, default to deny for safety.
		return false, "", fmt.Sprintf("classifier returned unrecognized response: %s", text)
	}
}

func ptr[T any](v T) *T {
	return &v
}
