package harness

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type ShellResult struct {
	Output   string
	ExitCode int
	TimedOut bool
}

func RunShell(command, cwd string, timeoutSeconds int) (ShellResult, error) {
	return RunShellEnv(command, cwd, timeoutSeconds, nil)
}

func RunShellEnv(command, cwd string, timeoutSeconds int, extraEnv map[string]string) (ShellResult, error) {
	if strings.TrimSpace(command) == "" {
		return ShellResult{Output: "Error: command is required", ExitCode: 1}, nil
	}
	if timeoutSeconds <= 0 {
		timeoutSeconds = 120
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSeconds)*time.Second)
	defer cancel()

	var cmd *exec.Cmd
	if IsWindows() {
		cmd = exec.CommandContext(ctx, "cmd.exe", "/C", command)
	} else {
		cmd = exec.CommandContext(ctx, "sh", "-c", command)
	}
	cmd.Dir = cwd
	if len(extraEnv) > 0 {
		env := []string{}
		for key, value := range extraEnv {
			env = append(env, key+"="+value)
		}
		cmd.Env = append(cmd.Environ(), env...)
	}

	var buffer bytes.Buffer
	cmd.Stdout = &buffer
	cmd.Stderr = &buffer
	err := cmd.Run()
	result := ShellResult{Output: strings.TrimSpace(buffer.String()), ExitCode: 0}
	if ctx.Err() == context.DeadlineExceeded {
		result.TimedOut = true
		result.ExitCode = 124
		result.Output = strings.TrimSpace(result.Output + fmt.Sprintf("\nError: Timeout (%ds)", timeoutSeconds))
		return result, nil
	}
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitError.ExitCode()
			return result, nil
		}
		return result, err
	}
	if result.Output == "" {
		result.Output = "(no output)"
	}
	if len(result.Output) > 50000 {
		result.Output = result.Output[:50000] + "\n... (truncated)"
	}
	return result, nil
}
