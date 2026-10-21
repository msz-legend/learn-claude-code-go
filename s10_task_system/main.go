package main

import (
	"fmt"
	"os"

	"learnclaude-go/internal/harness"
)

// s10: Task System
//
// Tasks live on disk, one JSON file each. blockedBy forms a dependency graph.
// claim() is atomic, so two agents cannot take the same ready task.
func main() {
	if len(os.Args) > 1 && (os.Args[1] == "--help" || os.Args[1] == "-h") {
		fmt.Println(`s10: Task System

Usage:
  go run ./s10_task_system

Tools: task_create, task_list, task_get, task_update,
       task_claim, task_complete. State lives in .tasks/.`)
		return
	}

	harness.Header("s10", "Task System")

	workspace := harness.NewWorkspace(harness.Workdir())
	client := harness.FromEnv()
	tasks := harness.NewTaskStore(workspace.Root())

	tools := harness.Base(workspace, 120).
		Register(tasks.CreateTool()).
		Register(tasks.ListTool()).
		Register(tasks.GetTool()).
		Register(tasks.UpdateTool()).
		Register(tasks.ClaimTool()).
		Register(tasks.CompleteTool())

	system := harness.SystemPrompt("a coding agent", workspace,
		"Persist multi-step work as tasks. Create tasks with dependencies, "+
			"claim one ready task at a time, and complete it with evidence. "+
			"Use owner \"lead\" unless told otherwise.")

	loop := harness.NewAgentLoop(client, tools, system).
		Listener(harness.PrintingListener())

	history := []harness.Msg{}
	harness.REPL("s10", loop, &history, func(query string) bool {
		if query != "/tasks" {
			return false
		}
		fmt.Println(tasks.Render())
		return true
	})
}
