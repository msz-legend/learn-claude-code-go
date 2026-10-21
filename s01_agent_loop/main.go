package main

import (
	"fmt"
	"os"

	"learnclaude-go/internal/harness"
)

// s01: one loop + one tool.
//
//	for {
//	  response = model(messages, tools)
//	  if no tool_use: return
//	  run tools, append tool_result, repeat
//	}
func main() {
	if len(os.Args) > 1 && (os.Args[1] == "--help" || os.Args[1] == "-h") {
		fmt.Println(`s01: Agent Loop

Usage:
  go run ./s01_agent_loop

Requires ANTHROPIC_API_KEY or OPENAI_API_KEY (see .env.example).`)
		return
	}

	harness.Header("s01", "Agent Loop")

	workspace := harness.NewWorkspace(harness.Workdir())
	client := harness.FromEnv()

	// The whole tool surface of s01: one shell.
	tools := harness.NewRegistry().Register(harness.BashTool(workspace, 120))

	system := harness.SystemPrompt("a coding agent", workspace, "Use bash to solve tasks.")

	loop := harness.NewAgentLoop(client, tools, system).
		Listener(harness.PrintingListener())

	history := []harness.Msg{}
	harness.REPL("s01", loop, &history, nil)
}
