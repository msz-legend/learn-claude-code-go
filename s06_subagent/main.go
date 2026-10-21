package main

import (
	"fmt"
	"os"

	"learnclaude-go/internal/harness"
)

// s06: Subagent
//
// A subtask runs in a fresh message list and returns only its final text.
// The parent context stays small; the child can still use every base tool.
func main() {
	if len(os.Args) > 1 && (os.Args[1] == "--help" || os.Args[1] == "-h") {
		fmt.Println(`s06: Subagent

Usage:
  go run ./s06_subagent

The task tool starts a child loop with a fresh history.
SUBAGENT_MAX_TURNS controls the child's turn limit.`)
		return
	}

	harness.Header("s06", "Subagent")

	workspace := harness.NewWorkspace(harness.Workdir())
	client := harness.FromEnv()
	childTools := harness.Base(workspace, 120)
	tools := harness.Base(workspace, 120)

	childSystem := harness.SystemPrompt("a focused subagent", workspace,
		"Solve only the assigned subtask. Inspect files before changing them. "+
			"Return a concise result with evidence.")

	tools.Register(harness.SubagentTool(
		client,
		childTools,
		childSystem,
		harness.GetInt("SUBAGENT_MAX_TURNS", 12)))

	system := harness.SystemPrompt("a coding agent", workspace,
		"Delegate self-contained investigation or implementation work to task. "+
			"Give the subagent enough context to work without your history.")

	loop := harness.NewAgentLoop(client, tools, system).
		Listener(harness.PrintingListener())

	history := []harness.Msg{}
	harness.REPL("s06", loop, &history, nil)
}
