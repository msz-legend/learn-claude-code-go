package main

import (
	"fmt"
	"os"

	"learnclaude-go/internal/harness"
)

// s07: Skill Loading
//
// The system prompt advertises only a skill catalog. The full body is read
// through load_skill when the model decides the skill is relevant.
func main() {
	if len(os.Args) > 1 && (os.Args[1] == "--help" || os.Args[1] == "-h") {
		fmt.Println(`s07: Skill Loading

Usage:
  go run ./s07_skill_loading

Skills are folders containing SKILL.md. Set SKILLS_DIR to
override the default ./skills root.`)
		return
	}

	harness.Header("s07", "Skill Loading")

	workspace := harness.NewWorkspace(harness.Workdir())
	client := harness.FromEnv()
	skills := harness.NewSkills(harness.GetOr("SKILLS_DIR", "skills"))
	skills.Catalog()

	tools := harness.Base(workspace, 120).
		Register(skills.ListTool()).
		Register(skills.LoadTool())

	system := harness.SystemPrompt("a coding agent", workspace,
		"Skills are loaded on demand. Read the catalog below, then call "+
			"load_skill for the one that matches the task.\n\n"+skills.CatalogText())

	loop := harness.NewAgentLoop(client, tools, system).
		Listener(harness.PrintingListener())

	history := []harness.Msg{}
	harness.REPL("s07", loop, &history, func(query string) bool {
		if query != "/skills" {
			return false
		}
		fmt.Println(skills.CatalogText())
		return true
	})
}
