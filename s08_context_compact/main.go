package main

import (
	"fmt"
	"os"

	"learnclaude-go/internal/harness"
)

// s08: Context Compact
//
// Four layers, cheapest first:
//  1. archive oversized tool results to .compact/
//  2. truncate old tool results
//  3. summarize older turns and keep a recent window
//  4. repeat truncation if the summary still does not fit
//
// Thresholds are environment variables so a demo can trigger compaction
// without filling a real context window.
func main() {
	if len(os.Args) > 1 && (os.Args[1] == "--help" || os.Args[1] == "-h") {
		fmt.Println(`s08: Context Compact

Usage:
  go run ./s08_context_compact

Tuning:
  COMPACT_TOOL_RESULT_BUDGET   total chars of tool results (default 200000)
  COMPACT_SUMMARIZE_AT_TOKENS  summary threshold (default 24000)
  COMPACT_KEEP_MESSAGES        recent messages to keep (default 8)

Try: COMPACT_SUMMARIZE_AT_TOKENS=1500 COMPACT_KEEP_MESSAGES=2`)
		return
	}

	harness.Header("s08", "Context Compact")

	workspace := harness.NewWorkspace(harness.Workdir())
	client := harness.FromEnv()
	compactor := harness.NewContextCompactor(workspace.Root())
	tools := harness.Base(workspace, 120)

	system := harness.SystemPrompt("a coding agent", workspace,
		"Keep working across compactions. After a summary, re-read files if you need exact text.")

	loop := harness.NewAgentLoop(client, tools, system).
		Compactor(compactor).
		Listener(harness.PrintingListener())

	history := []harness.Msg{}
	harness.REPL("s08", loop, &history, func(query string) bool {
		if query != "/context" {
			return false
		}
		fmt.Println("messages:", len(history))
		fmt.Println("estimated tokens:", harness.EstimateTokens(history))
		fmt.Println("compactions:", compactor.Compactions())
		return true
	})
}
