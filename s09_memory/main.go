package main

import (
	"fmt"
	"os"
	"strings"

	"learnclaude-go/internal/harness"
)

// s09: Memory System
//
// Three jobs around a small durable store:
//
//	selection     - show relevant entries before a request
//	extraction    - save durable facts from a finished turn
//	consolidation - merge or prune entries when the store grows
//
// This chapter exposes them as tools so every step is visible.
func main() {
	if len(os.Args) > 1 && (os.Args[1] == "--help" || os.Args[1] == "-h") {
		fmt.Println(`s09: Memory System

Usage:
  go run ./s09_memory

Tools: remember, recall, consolidate_memory.
Memory is stored in .memory/memory.json.`)
		return
	}

	harness.Header("s09", "Memory System")

	workspace := harness.NewWorkspace(harness.Workdir())
	client := harness.FromEnv()
	memory := harness.NewMemory(workspace.Root())

	consolidate := harness.Tool{
		Name:        "consolidate_memory",
		Description: "Merge duplicates and prune stale long-term memories.",
		InputSchema: harness.Obj("type", "object", "properties", harness.Obj()),
		Handler: func(map[string]any) (string, error) {
			return memory.Consolidate(client)
		},
	}

	tools := harness.Base(workspace, 120).
		Register(memory.WriteTool()).
		Register(memory.SearchTool()).
		Register(consolidate)

	system := harness.SystemPrompt("a coding agent", workspace,
		"Save durable project facts with remember, look them up with recall, "+
			"and consolidate when the store has duplicates.\n\n"+memory.Summary())

	loop := harness.NewAgentLoop(client, tools, system).
		Listener(harness.PrintingListener())

	history := []harness.Msg{}
	harness.REPL("s09", loop, &history, func(query string) bool {
		if strings.HasPrefix(query, "/recall ") {
			term := strings.TrimSpace(strings.TrimPrefix(query, "/recall "))
			rendered := memory.RenderForPrompt(term, 5)
			if rendered == "" {
				fmt.Println("(no matching memories)")
			} else {
				fmt.Println(rendered)
			}
			return true
		}
		if query != "/memory" {
			return false
		}
		fmt.Println(memory.Summary())
		return true
	})
}
