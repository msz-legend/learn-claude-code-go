package main

import (
	"fmt"
	"os"
	"sync/atomic"

	"learnclaude-go/internal/harness"
)

// s05: TodoWrite
//
// The plan lives in the harness, not only in the prompt. A small hook counts
// tool rounds since the last todo_write and appends a reminder after three.
const remindAfter = 3

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "--help" || os.Args[1] == "-h") {
		fmt.Println(`s05: TodoWrite

Usage:
  go run ./s05_todo_write

The todo_write tool stores a plan and the loop nudges the model
when the plan has not been updated for three tool rounds.`)
		return
	}

	harness.Header("s05", "TodoWrite")

	workspace := harness.NewWorkspace(harness.Workdir())
	client := harness.FromEnv()
	todo := &harness.Todo{}
	tools := harness.Base(workspace, 120).Register(todo.Tool())

	var sinceTodo atomic.Int64

	reminder := &harness.Hooks{
		PreToolUse: func(call harness.Block) (string, error) {
			if call.Name == "todo_write" {
				sinceTodo.Store(0)
			}
			return "", nil
		},
		PostToolUse: func(call harness.Block, output string) (string, error) {
			if call.Name == "todo_write" {
				return output, nil
			}
			if sinceTodo.Add(1) >= remindAfter {
				sinceTodo.Store(0)
				return output + "\n\n<reminder>Update todo_write before continuing. " +
					"Current plan:\n" + todo.Render() + "</reminder>", nil
			}
			return output, nil
		},
	}

	system := harness.SystemPrompt("a coding agent", workspace,
		"Before a multi-step task, call todo_write with the full plan. "+
			"Keep exactly one item in_progress and update it as you finish steps.")

	loop := harness.NewAgentLoop(client, tools, system).
		Hooks(reminder).
		Listener(harness.PrintingListener())

	history := []harness.Msg{}
	harness.REPL("s05", loop, &history, func(query string) bool {
		if query != "/todo" {
			return false
		}
		fmt.Println(todo.Render())
		return true
	})
}
