package main

import (
	"fmt"
	"os"

	"learnclaude-go/internal/harness"
)

// s12: Cron Scheduler
//
// Jobs are persisted to .cron/jobs.json. A scheduler thread checks once a
// minute; due prompts enter the next model request as harness notifications.
func main() {
	if len(os.Args) > 1 && (os.Args[1] == "--help" || os.Args[1] == "-h") {
		fmt.Println(`s12: Cron Scheduler

Usage:
  go run ./s12_cron_scheduler

Tools: cron_create, cron_list, cron_cancel.
Expressions use five fields: minute hour day month weekday.
Jobs run while this process is open.`)
		return
	}

	harness.Header("s12", "Cron Scheduler")

	workspace := harness.NewWorkspace(harness.Workdir())
	client := harness.FromEnv()
	cron := harness.NewCron(workspace.Root())
	cron.Start()

	tools := harness.Base(workspace, 120).
		Register(cron.CreateTool()).
		Register(cron.ListTool()).
		Register(cron.CancelTool())

	system := harness.SystemPrompt("a coding agent", workspace,
		"You can schedule future prompts with cron_create. "+
			"When a scheduled prompt arrives, treat it as a new user turn.")

	loop := harness.NewAgentLoop(client, tools, system).
		Notifications(func() []string {
			notes := []string{}
			for _, job := range cron.DrainDue() {
				notes = append(notes,
					"[scheduled "+job.ID+" "+job.Expression+"] "+job.Prompt)
			}
			return notes
		}).
		Listener(harness.PrintingListener())

	history := []harness.Msg{}
	defer cron.Stop()
	harness.REPL("s12", loop, &history, func(query string) bool {
		if query != "/cron" {
			return false
		}
		fmt.Println(cron.Render())
		return true
	})
}
