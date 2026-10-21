package main

import (
	"fmt"
	"os"

	"learnclaude-go/internal/harness"
)

// s11: Background Tasks
//
// Slow commands run off the conversation thread. When a job finishes, its
// output is queued and injected as a user message before the next request.
func main() {
	if len(os.Args) > 1 && (os.Args[1] == "--help" || os.Args[1] == "-h") {
		fmt.Println(`s11: Background Tasks

Usage:
  go run ./s11_background_tasks

Tools: background_run, background_check. Completed jobs are
announced automatically before the next model call.`)
		return
	}

	harness.Header("s11", "Background Tasks")

	workspace := harness.NewWorkspace(harness.Workdir())
	client := harness.FromEnv()
	background := harness.NewBackground(workspace.Root())

	tools := harness.Base(workspace, 120).
		Register(background.RunTool()).
		Register(background.CheckTool())

	system := harness.SystemPrompt("a coding agent", workspace,
		"Use background_run for slow commands (builds, test suites, downloads). "+
			"Keep working on something else, then check the job or wait for its notification.")

	loop := harness.NewAgentLoop(client, tools, system).
		Notifications(background.DrainNotifications).
		Listener(harness.PrintingListener())

	history := []harness.Msg{}
	defer background.Shutdown()
	harness.REPL("s11", loop, &history, func(query string) bool {
		if query != "/jobs" {
			return false
		}
		jobs := background.List()
		if len(jobs) == 0 {
			fmt.Println("(no background jobs)")
		}
		for _, job := range jobs {
			fmt.Printf("- %s [%s] %s\n", job.ID, job.Status, job.Command)
		}
		return true
	})
}
