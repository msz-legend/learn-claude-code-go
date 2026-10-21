package main

import (
	"fmt"
	"os"
	"strings"

	"learnclaude-go/internal/harness"
)

// s17: Goal Loop
//
// The worker may propose a stop by ending its turn. An independent evaluator
// reads the transcript and either accepts the goal, rejects it with a reason,
// or declares it impossible. The loop continues until one of those happens.
func main() {
	if len(os.Args) > 1 && (os.Args[1] == "--help" || os.Args[1] == "-h") {
		fmt.Println(`s17: Goal Loop

Usage:
  go run ./s17_goal_loop --goal "make the tests pass"
  go run ./s17_goal_loop          # prompt for a goal

Environment:
  GOAL_MAX_TURNS   evaluator budget (default 12)
  PERMISSION_MODE  ask (default) or allow`)
		return
	}

	goalText := parseGoal(os.Args[1:])
	if strings.TrimSpace(goalText) == "" {
		line, ok := harness.ReadLine("Goal: ")
		if ok {
			goalText = line
		}
	}
	if strings.TrimSpace(goalText) == "" {
		fmt.Println("No goal given. Use --goal \"...\".")
		return
	}

	harness.Header("s17", "Goal Loop")
	fmt.Println("goal: " + goalText)
	fmt.Println()

	workspace := harness.NewWorkspace(harness.Workdir())
	client := harness.FromEnv()
	goal := harness.NewGoal(goalText, client)

	tools := harness.Base(workspace, 120)

	var policy harness.Policy = harness.RulesPolicy(harness.DefaultRules())
	if strings.EqualFold(harness.GetOr("PERMISSION_MODE", "ask"), "allow") {
		policy = harness.AllowAllPolicy()
	}

	system := harness.SystemPrompt("a coding agent working against a checkable goal", workspace,
		"The goal is:\n"+goalText+"\n\n"+
			"Work until the goal is met. Verify with a concrete command and quote its output. "+
			"If you stop, an independent evaluator will inspect the transcript and may send you back.")

	loop := harness.NewAgentLoop(client, tools, system).
		Permission(policy).
		Approver(harness.ConsoleApprover).
		StopGate(goal).
		Listener(harness.PrintingListener())

	history := []harness.Msg{
		harness.User("Goal: " + goalText + "\nStart by inspecting the workspace."),
	}

	result, err := loop.Run(&history)
	fmt.Println()
	if err != nil {
		fmt.Println(harness.Color("error: "+err.Error(), harness.Red))
		fmt.Println(harness.Color("status: "+loop.LastStatus(), harness.Cyan))
		fmt.Println(harness.Color(goal.StatusLine(), harness.Dim))
		return
	}
	fmt.Println(harness.Color("status: "+loop.LastStatus(), harness.Cyan))
	fmt.Println(harness.Color(goal.StatusLine(), harness.Dim))
	if strings.TrimSpace(result) != "" {
		fmt.Println()
		fmt.Println(result)
	}
}

func parseGoal(args []string) string {
	for i := 0; i < len(args); i++ {
		if args[i] == "--goal" && i+1 < len(args) {
			return args[i+1]
		}
	}
	if env := harness.Get("GOAL"); strings.TrimSpace(env) != "" {
		return env
	}
	for _, arg := range args {
		if !strings.HasPrefix(arg, "-") {
			return arg
		}
	}
	return ""
}
