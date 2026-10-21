package main

import (
	"fmt"
	"os"
	"strings"

	"learnclaude-go/internal/harness"
)

// s16: Workflow Runtime
//
// The host owns the orchestration shape; the model only chooses a workflow
// name. Agent calls and steps are journaled under .workflows/<run>/ so a
// restart can resume without repeating completed work.
func main() {
	if len(os.Args) > 1 && (os.Args[1] == "--help" || os.Args[1] == "-h") {
		fmt.Println(`s16: Workflow Runtime

Usage:
  go run ./s16_workflow_runtime [run-id]

The default "review" workflow inspects the workspace, runs two
reviews in parallel, and writes a report. Pass a run id to resume
a journaled run. The model may also call the workflow tool.`)
		return
	}

	harness.Header("s16", "Workflow Runtime")

	workspace := harness.NewWorkspace(harness.Workdir())
	client := harness.FromEnv()

	reviewTools := harness.Base(workspace, 120)
	reviewSystem := harness.SystemPrompt("a focused reviewer", workspace,
		"Read the actual files. Report concrete findings, not generic advice.")

	agentCall := func(prompt string) (string, error) {
		history := []harness.Msg{harness.User(prompt)}
		return harness.NewAgentLoop(client, reviewTools, reviewSystem).
			MaxTurns(harness.GetInt("WORKFLOW_AGENT_TURNS", 10)).
			Listener(harness.PrintingListener()).
			Run(&history)
	}

	state := map[string]string{}

	steps := []harness.WorkflowStep{
		{Name: "inspect", Body: func(ctx *harness.WorkflowContext) (string, error) {
			ctx.Phase("Inspect")
			output, err := ctx.Agent(`Inspect this repository. Summarize its purpose, layout, build
command, and the three most important files. Keep it under 400 words.`)
			if err != nil {
				return "", err
			}
			state["inspect"] = output
			return output, nil
		}},
		{Name: "review", Body: func(ctx *harness.WorkflowContext) (string, error) {
			ctx.Phase("Review")
			outputs, err := ctx.Parallel([]string{
				"Review the code for correctness bugs. Read the actual files. " +
					"List findings with file names and severity.",
				"Review the code for missing tests and unclear documentation. " +
					"Read the actual files. List concrete gaps.",
			})
			if err != nil {
				return "", err
			}
			var joined strings.Builder
			for i, output := range outputs {
				fmt.Fprintf(&joined, "Review %d:\n%s\n\n", i+1, output)
			}
			state["review"] = strings.TrimSpace(joined.String())
			return state["review"], nil
		}},
		{Name: "report", Body: func(ctx *harness.WorkflowContext) (string, error) {
			ctx.Phase("Report")
			// On a resumed run, earlier steps may be journal-cached transactions
			// rather than fresh executions: prefer the fresh value, fall back to
			// the journal so the report never invents "empty" input.
			inspect := state["inspect"]
			if inspect == "" {
				inspect = ctx.Previous("inspect")
			}
			review := state["review"]
			if review == "" {
				review = ctx.Previous("review")
			}
			report, err := ctx.Agent(fmt.Sprintf(`Write a short review report for the repository.
Use only these findings; do not invent new ones.

Inspection:
%s

Reviews:
%s`, inspect, review))
			if err != nil {
				return "", err
			}
			state["report"] = report
			return report, nil
		}},
	}

	registry := harness.NewWorkflowRegistry(workspace.Root(), agentCall).
		Register(harness.WorkflowMeta{
			Name:        "review",
			Description: "Inspect, review in parallel, then write a report.",
			Phases:      []string{"inspect", "review", "report"},
		}, steps).
		Register(harness.WorkflowMeta{
			Name:        "summarize",
			Description: "One agent call that summarizes the repository.",
			Phases:      []string{"summarize"},
		}, []harness.WorkflowStep{
			{Name: "summarize", Body: func(ctx *harness.WorkflowContext) (string, error) {
				return ctx.Agent("Summarize this repository for a new contributor in under 300 words.")
			}},
		})

	tools := harness.Base(workspace, 120).Register(registry.Tool())
	system := harness.SystemPrompt("a coding agent that can run saved workflows", workspace,
		"For a multi-phase review, call workflow with name=review. "+
			"The workflow runtime owns the steps and journal.\n\n"+
			registry.RenderCatalog())

	if len(os.Args) > 1 {
		result, err := registry.Run("review", os.Args[1])
		if err != nil {
			fmt.Println("Error:", err)
			return
		}
		fmt.Println(result.Render())
		fmt.Println()
		if report, ok := result.Output["report"]; ok {
			fmt.Println(report)
		} else {
			fmt.Println("(no report)")
		}
		return
	}

	loop := harness.NewAgentLoop(client, tools, system).
		Listener(harness.PrintingListener())

	history := []harness.Msg{}
	harness.REPL("s16", loop, &history, func(query string) bool {
		if !strings.HasPrefix(query, "/run ") {
			return false
		}
		parts := strings.Fields(strings.TrimSpace(strings.TrimPrefix(query, "/run ")))
		name := "review"
		runID := ""
		if len(parts) > 0 {
			name = parts[0]
		}
		if len(parts) > 1 {
			runID = parts[1]
		}
		result, err := registry.Run(name, runID)
		if err != nil {
			fmt.Println("Error:", err)
			return true
		}
		fmt.Println(result.Render())
		return true
	})
}
