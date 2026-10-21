package main

import (
	"fmt"
	"os"
	"strings"

	"learnclaude-go/internal/harness"
)

// s04: Hook System
//
// Hooks wrap the loop without editing it:
//
//	userPromptSubmit -> observe the incoming turn
//	preToolUse       -> block a call before it runs
//	postToolUse      -> rewrite what the model sees
//	stop             -> observe the final answer
func main() {
	if len(os.Args) > 1 && (os.Args[1] == "--help" || os.Args[1] == "-h") {
		fmt.Println(`s04: Hook System

Usage:
  go run ./s04_hooks

Built-in hooks block writes to .env and tag shortened tool output.`)
		return
	}

	harness.Header("s04", "Hook System")

	workspace := harness.NewWorkspace(harness.Workdir())
	client := harness.FromEnv()
	tools := harness.Base(workspace, 120)

	audit := &harness.Hooks{
		UserPromptSubmit: func(prompt string) error {
			fmt.Println(harness.Color("[hook] prompt: "+harness.Shorten(prompt, 100), harness.Dim))
			return nil
		},
		PreToolUse: func(call harness.Block) (string, error) {
			if call.Name == "write_file" || call.Name == "edit_file" {
				path := strings.ToLower(harness.Str(call.Input, "path"))
				if strings.HasSuffix(path, ".env") {
					return "Writing .env is blocked by the secrets hook", nil
				}
			}
			return "", nil
		},
		PostToolUse: func(call harness.Block, output string) (string, error) {
			if len(output) > 2_000 {
				return output[:2_000] +
					fmt.Sprintf("\n... [hook] output shortened; %d chars total", len(output)), nil
			}
			return output, nil
		},
		Stop: func(history []harness.Msg) error {
			fmt.Println(harness.Color(
				fmt.Sprintf("[hook] turn ended with %d messages", len(history)), harness.Dim))
			return nil
		},
	}

	system := harness.SystemPrompt("a coding agent", workspace,
		"Hooks may block or rewrite tool calls. Treat their messages as harness feedback.")

	loop := harness.NewAgentLoop(client, tools, system).
		Hooks(audit).
		Listener(harness.PrintingListener())

	history := []harness.Msg{}
	harness.REPL("s04", loop, &history, nil)
}
