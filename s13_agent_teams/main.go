package main

import (
	"fmt"
	"os"

	"learnclaude-go/internal/harness"
)

// s13: Agent Teams
//
// The lead and its teammates share a disk-backed task board and append-only
// mailboxes. A spawn can bind a teammate to a task and an optional worktree.
func main() {
	if len(os.Args) > 1 && (os.Args[1] == "--help" || os.Args[1] == "-h") {
		fmt.Println(`s13: Agent Teams

Usage:
  go run ./s13_agent_teams

Lead tools: team_spawn, team_list, team_send, team_inbox,
team_shutdown plus task_* tools. Teammates run in background
goroutines and report to the lead mailbox.`)
		return
	}

	harness.Header("s13", "Agent Teams")

	workspace := harness.NewWorkspace(harness.Workdir())
	client := harness.FromEnv()
	tasks := harness.NewTaskStore(workspace.Root())
	team := harness.NewTeam(workspace, tasks, client)

	team.ToolFactory(func(shared *harness.Team, member harness.TeamMember,
		memberWorkspace *harness.Workspace) *harness.Registry {
		return harness.Base(memberWorkspace, 120).
			Register(shared.Tasks().CreateTool()).
			Register(shared.Tasks().ListTool()).
			Register(shared.Tasks().GetTool()).
			Register(shared.Tasks().ClaimTool()).
			Register(shared.Tasks().CompleteTool()).
			Register(shared.SendTool(member.Name)).
			Register(shared.InboxTool(member.Name))
	})

	tools := harness.Base(workspace, 120).
		Register(tasks.CreateTool()).
		Register(tasks.ListTool()).
		Register(tasks.GetTool()).
		Register(tasks.UpdateTool()).
		Register(tasks.ClaimTool()).
		Register(tasks.CompleteTool()).
		Register(team.SpawnTool()).
		Register(team.ListTool()).
		Register(team.SendTool("lead")).
		Register(team.InboxTool("lead")).
		Register(team.ShutdownTool())

	system := harness.SystemPrompt("the lead agent of a small coding team", workspace,
		"Plan on the shared task board. Spawn one teammate per independent task. "+
			"Give each teammate a concrete prompt, then check team_inbox for results. "+
			"Only one owner may hold a task at a time.")

	loop := harness.NewAgentLoop(client, tools, system).
		Notifications(func() []string {
			notes := []string{}
			for _, mail := range team.InboxMessages("lead") {
				notes = append(notes,
					"[team "+mail.From+" / "+mail.Type+"] "+mail.Content)
			}
			return notes
		}).
		Listener(harness.PrintingListener())

	history := []harness.Msg{}
	harness.REPL("s13", loop, &history, func(query string) bool {
		if query != "/team" {
			return false
		}
		fmt.Println(team.List())
		fmt.Println()
		fmt.Println(tasks.Render())
		return true
	})
}
