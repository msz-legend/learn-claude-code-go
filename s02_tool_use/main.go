package main

import (
	"fmt"
	"os"

	"learnclaude-go/internal/harness"
)

// s02: Tool Use
//
// The loop from s01 does not change. The tool surface grows from one bash tool
// to a dispatch table of five tools, and every file path is confined to the
// workspace.
func main() {
	if len(os.Args) > 1 && (os.Args[1] == "--help" || os.Args[1] == "-h") {
		fmt.Println(`s02: Tool Use

Usage:
  go run ./s02_tool_use

Tools: bash, read_file, write_file, edit_file, glob.
Requires ANTHROPIC_API_KEY or OPENAI_API_KEY (see .env.example).`)
		return
	}

	harness.Header("s02", "Tool Use")

	workspace := harness.NewWorkspace(harness.Workdir())
	client := harness.FromEnv()
	tools := harness.Base(workspace, 120)

	system := harness.SystemPrompt("a coding agent", workspace,
		"Use the dedicated file tools instead of shell redirection. "+
			"All paths are relative to the workspace.")

	loop := harness.NewAgentLoop(client, tools, system).
		Listener(harness.PrintingListener())

	history := []harness.Msg{}
	harness.REPL("s02", loop, &history, func(query string) bool {
		if query != "/tools" {
			return false
		}
		for _, name := range tools.Names() {
			fmt.Println("- " + name)
		}
		return true
	})
}
