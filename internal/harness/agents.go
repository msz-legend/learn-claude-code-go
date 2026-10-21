package harness

import "fmt"

// SubagentTool runs a focused subtask in a fresh message list.
func SubagentTool(client Client, tools *Registry, system string, maxTurns int) Tool {
	return Tool{
		Name:        "task",
		Description: "Run a focused subtask in a fresh context. Returns only the final text.",
		InputSchema: Obj(
			"type", "object",
			"properties", Obj(
				"description", Obj("type", "string"),
				"prompt", Obj("type", "string")),
			"required", Arr("description")),
		Handler: func(input map[string]any) (string, error) {
			prompt := Str(input, "prompt", Str(input, "description"))
			if prompt == "" {
				return "Error: description is required", nil
			}
			fmt.Println("[subagent] " + Shorten(prompt, 160))
			history := []Msg{User(prompt)}
			loop := NewAgentLoop(client, tools, system).
				MaxTurns(maxTurns).
				Listener(&Listener{
					OnToolStart: func(call Block) {
						fmt.Println("  [subagent] > " + call.Name)
					},
				})
			text, err := loop.Run(&history)
			if err != nil {
				return "", err
			}
			if text == "" {
				return "(subagent returned no text)", nil
			}
			return text, nil
		},
	}
}
