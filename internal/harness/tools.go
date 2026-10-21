package harness

// Base returns bash plus the four file tools used from s02 onward.
func Base(workspace *Workspace, timeoutSeconds int) *Registry {
	return NewRegistry().
		Register(BashTool(workspace, timeoutSeconds)).
		Register(ReadFileTool(workspace)).
		Register(WriteFileTool(workspace)).
		Register(EditFileTool(workspace)).
		Register(GlobTool(workspace))
}

func SystemPrompt(role string, workspace *Workspace, extra string) string {
	prompt := "You are " + role + " at " + workspace.Root() + ". " +
		"Environment: " + EnvironmentPrompt() + ". "
	if extra != "" {
		prompt += extra + " "
	}
	return prompt + "Act, don't explain."
}

func BashTool(workspace *Workspace, timeoutSeconds int) Tool {
	return Tool{
		Name:        "bash",
		Description: "Run a shell command in the workspace.",
		InputSchema: Obj(
			"type", "object",
			"properties", Obj("command", Obj("type", "string")),
			"required", Arr("command")),
		Handler: func(input map[string]any) (string, error) {
			command := Str(input, "command")
			if command == "" {
				return "Error: command is required", nil
			}
			if IsDangerous(command) {
				return "Error: Dangerous command blocked", nil
			}
			result, err := RunShell(command, workspace.Root(), timeoutSeconds)
			if err != nil {
				return "", err
			}
			if result.TimedOut {
				return "Error: Timeout\n" + result.Output, nil
			}
			output := result.Output
			if output == "" {
				output = "(no output)"
			}
			if len(output) > 50_000 {
				output = Truncate(output, 50_000) + "\n... (output truncated)"
			}
			if result.ExitCode != 0 {
				output += "\n[exit code: " + itoa(result.ExitCode) + "]"
			}
			return output, nil
		},
	}
}

func ReadFileTool(workspace *Workspace) Tool {
	return Tool{
		Name:        "read_file",
		Description: "Read a UTF-8 text file. Optionally return only the first N lines.",
		InputSchema: Obj(
			"type", "object",
			"properties", Obj(
				"path", Obj("type", "string"),
				"limit", Obj("type", "integer")),
			"required", Arr("path")),
		Handler: func(input map[string]any) (string, error) {
			return workspace.Read(Str(input, "path"), Int(input, "limit", 0))
		},
	}
}

func WriteFileTool(workspace *Workspace) Tool {
	return Tool{
		Name:        "write_file",
		Description: "Write content to a file, creating parent directories when needed.",
		InputSchema: Obj(
			"type", "object",
			"properties", Obj(
				"path", Obj("type", "string"),
				"content", Obj("type", "string")),
			"required", Arr("path", "content")),
		Handler: func(input map[string]any) (string, error) {
			return workspace.Write(Str(input, "path"), Str(input, "content"))
		},
	}
}

func EditFileTool(workspace *Workspace) Tool {
	return Tool{
		Name:        "edit_file",
		Description: "Replace the first exact occurrence of old_text with new_text.",
		InputSchema: Obj(
			"type", "object",
			"properties", Obj(
				"path", Obj("type", "string"),
				"old_text", Obj("type", "string"),
				"new_text", Obj("type", "string")),
			"required", Arr("path", "old_text", "new_text")),
		Handler: func(input map[string]any) (string, error) {
			return workspace.Edit(Str(input, "path"), Str(input, "old_text"), Str(input, "new_text"))
		},
	}
}

func GlobTool(workspace *Workspace) Tool {
	return Tool{
		Name:        "glob",
		Description: "Find files matching a glob pattern; ** matches recursively.",
		InputSchema: Obj(
			"type", "object",
			"properties", Obj("pattern", Obj("type", "string")),
			"required", Arr("pattern")),
		Handler: func(input map[string]any) (string, error) {
			return workspace.Glob(Str(input, "pattern"))
		},
	}
}
