package main

import (
	"fmt"
	"os"

	"learnclaude-go/internal/harness"
)

// s15: Integrated Harness
//
// Every mechanism from the course around the same loop:
// file/shell tools, todo plans, skills, memory, tasks, background jobs,
// scheduling, subagents and context compaction.
func main() {
	if len(os.Args) > 1 && (os.Args[1] == "--help" || os.Args[1] == "-h") {
		fmt.Println(`s15: Integrated Harness

Usage:
  go run ./s15_integrated_harness

One loop, many harness ports. State is written under
.tasks/, .memory/, .cron/, .compact/ and skills/.`)
		return
	}

	harness.Header("s15", "Integrated Harness")

	workspace := harness.NewWorkspace(harness.Workdir())
	client := harness.FromEnv()

	todo := &harness.Todo{}
	skills := harness.NewSkills(harness.GetOr("SKILLS_DIR", "skills"))
	skills.Catalog()
	memory := harness.NewMemory(workspace.Root())
	tasks := harness.NewTaskStore(workspace.Root())
	background := harness.NewBackground(workspace.Root())
	cron := harness.NewCron(workspace.Root())
	compactor := harness.NewContextCompactor(workspace.Root())

	childTools := harness.Base(workspace, 120).
		Register(todo.Tool()).
		Register(skills.ListTool()).
		Register(skills.LoadTool()).
		Register(memory.WriteTool()).
		Register(memory.SearchTool())

	tools := harness.Base(workspace, 120).
		Register(todo.Tool()).
		Register(skills.ListTool()).
		Register(skills.LoadTool()).
		Register(memory.WriteTool()).
		Register(memory.SearchTool()).
		Register(tasks.CreateTool()).
		Register(tasks.ListTool()).
		Register(tasks.GetTool()).
		Register(tasks.UpdateTool()).
		Register(tasks.ClaimTool()).
		Register(tasks.CompleteTool()).
		Register(background.RunTool()).
		Register(background.CheckTool()).
		Register(cron.CreateTool()).
		Register(cron.ListTool()).
		Register(cron.CancelTool()).
		Register(harness.SubagentTool(client, childTools,
			harness.SystemPrompt("a focused subagent", workspace,
				"Finish the assigned subtask and report evidence."),
			harness.GetInt("SUBAGENT_MAX_TURNS", 12)))

	cron.Start()

	system := harness.SystemPrompt("a coding agent with a full harness", workspace,
		"Plan with todo_write. Reuse durable facts with recall. Track multi-step "+
			"work on the task board. Run slow commands in the background. "+
			"Schedule future prompts with cron_create. Delegate focused work to task.\n\n"+
			skills.CatalogText()+"\n\n"+memory.Summary())

	loop := harness.NewAgentLoop(client, tools, system).
		Permission(harness.RulesPolicy(harness.DefaultRules())).
		Approver(harness.ConsoleApprover).
		Compactor(compactor).
		Notifications(func() []string {
			notes := append([]string{}, background.DrainNotifications()...)
			for _, job := range cron.DrainDue() {
				notes = append(notes, "[scheduled "+job.ID+"] "+job.Prompt)
			}
			return notes
		}).
		Listener(harness.PrintingListener())

	history := []harness.Msg{}
	defer func() {
		cron.Stop()
		background.Shutdown()
	}()
	harness.REPL("s15", loop, &history, func(query string) bool {
		if query == "/status" {
			fmt.Printf("messages: %d  tokens~%d  compactions: %d\n",
				len(history), harness.EstimateTokens(history), compactor.Compactions())
			fmt.Printf("jobs: %d  tasks: %d  memories: %d  skills: %d\n",
				len(background.List()), len(tasks.List()),
				len(memory.Entries()), len(skills.Catalog()))
			return true
		}
		if query == "/tasks" {
			fmt.Println(tasks.Render())
			return true
		}
		return false
	})
}
