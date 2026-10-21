package main

import (
	"fmt"
	"os"
	"strings"

	"learnclaude-go/internal/harness"
)

// s03: Permission System
//
// Three gates before a tool runs:
//  1. hard deny rules (rm -rf /, sudo, ...)
//  2. rule matching (deletes, force pushes, secrets)
//  3. an interactive approval for ASK rules
//
// A blocked call still produces a tool_result, so the model can recover.
func main() {
	if len(os.Args) > 1 && (os.Args[1] == "--help" || os.Args[1] == "-h") {
		fmt.Println(`s03: Permission System

Usage:
  go run ./s03_permission

Destructive commands are denied or require interactive approval.
Set PERMISSION_MODE=allow to disable the gate for a quick demo.`)
		return
	}

	harness.Header("s03", "Permission System")

	workspace := harness.NewWorkspace(harness.Workdir())
	client := harness.FromEnv()
	tools := harness.Base(workspace, 120)

	var policy harness.Policy = harness.RulesPolicy(harness.DefaultRules())
	if strings.EqualFold(harness.GetOr("PERMISSION_MODE", "ask"), "allow") {
		policy = harness.AllowAllPolicy()
	}

	system := harness.SystemPrompt("a coding agent", workspace,
		"Destructive operations require user approval. "+
			"If a tool is denied, explain what you need and choose a safer action.")

	loop := harness.NewAgentLoop(client, tools, system).
		Permission(policy).
		Approver(harness.ConsoleApprover).
		Listener(harness.PrintingListener())

	history := []harness.Msg{}
	harness.REPL("s03", loop, &history, func(query string) bool {
		if query != "/rules" {
			return false
		}
		for _, rule := range harness.DefaultRules() {
			extra := ""
			if rule.Contains != "" {
				extra = " contains=" + rule.Contains
			}
			fmt.Printf("- %s %s%s  (%s)\n", rule.Mode, rule.Tool, extra, rule.Reason)
		}
		return true
	})
}
