package harness

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

type ShellResult struct {
	Output   string
	ExitCode int
	TimedOut bool
}

// killTree stops cmd and, on Windows, every child it spawned, so an orphaned
// grandchild cannot hold the stdout pipe open and stall cmd.Run().
func killTree(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	if IsWindows() {
		_ = exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(cmd.Process.Pid)).Run()
		return
	}
	_ = cmd.Process.Kill()
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
	// Replace the default single-process Kill with a tree kill, and give the
	// pipes a bounded drain window even if some child still holds them.
	cmd.Cancel = func() error { killTree(cmd); return nil }
	cmd.WaitDelay = 5 * time.Second
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
		result.Output = Truncate(result.Output, 50000) + "\n... (truncated)"
	}
	return result, nil
}
