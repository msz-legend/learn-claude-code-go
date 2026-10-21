package harness

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type BackgroundJob struct {
	ID         string `json:"id"`
	Command    string `json:"command"`
	Status     string `json:"status"`
	Output     string `json:"output"`
	StartedAt  int64  `json:"startedAt"`
	FinishedAt int64  `json:"finishedAt"`
}

// Background runs slow commands off the conversation thread.
type Background struct {
	cwd      string
	mu       sync.Mutex
	jobs     map[string]BackgroundJob
	reported map[string]bool
	counter  atomic.Int64
}

func NewBackground(cwd string) *Background {
	return &Background{cwd: cwd, jobs: map[string]BackgroundJob{}, reported: map[string]bool{}}
}

func (b *Background) Start(command string) string {
	text := strings.TrimSpace(command)
	if text == "" {
		return "Error: command is required"
	}
	if IsDangerous(text) {
		return "Error: Dangerous command blocked"
	}
	id := fmt.Sprintf("job-%d-%d", time.Now().UnixMilli(), b.counter.Add(1))
	b.mu.Lock()
	b.jobs[id] = BackgroundJob{ID: id, Command: text, Status: "running", StartedAt: time.Now().UnixMilli()}
	b.mu.Unlock()

	go func() {
		result, err := RunShell(text, b.cwd, 600)
		status := "completed"
		output := result.Output
		if err != nil {
			status = "failed"
			output = "Error: " + err.Error()
		} else if result.TimedOut {
			status = "timed_out"
		} else if result.ExitCode != 0 {
			status = "failed"
		}
		if len(output) > 30_000 {
			output = output[:30_000] + "\n... (truncated)"
		}
		b.mu.Lock()
		job := b.jobs[id]
		job.Status = status
		job.Output = output
		job.FinishedAt = time.Now().UnixMilli()
		b.jobs[id] = job
		b.mu.Unlock()
	}()
	return "Started " + id + " in the background: " + text
}

func (b *Background) Check(id string) string {
	b.mu.Lock()
	defer b.mu.Unlock()
	job, ok := b.jobs[id]
	if !ok {
		return "Error: unknown job: " + id
	}
	if job.Status == "running" {
		return fmt.Sprintf("%s is still running (%d ms)", job.ID, time.Now().UnixMilli()-job.StartedAt)
	}
	return job.ID + " " + job.Status + "\n" + job.Output
}

func (b *Background) List() []BackgroundJob {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := []BackgroundJob{}
	for _, job := range b.jobs {
		out = append(out, job)
	}
	return out
}

// DrainNotifications returns each completed job once.
func (b *Background) DrainNotifications() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := []string{}
	for id, job := range b.jobs {
		if job.Status != "running" && !b.reported[id] {
			b.reported[id] = true
			out = append(out, "[background "+job.ID+" "+job.Status+"]\n"+job.Output)
		}
	}
	return out
}

func (b *Background) Shutdown() {}

func (b *Background) RunTool() Tool {
	return Tool{
		Name:        "background_run",
		Description: "Run a slow shell command in the background. The result returns later.",
		InputSchema: Obj(
			"type", "object",
			"properties", Obj("command", Obj("type", "string")),
			"required", Arr("command")),
		Handler: func(input map[string]any) (string, error) {
			return b.Start(Str(input, "command")), nil
		},
	}
}

func (b *Background) CheckTool() Tool {
	return Tool{
		Name:        "background_check",
		Description: "Check one background job by id.",
		InputSchema: Obj(
			"type", "object",
			"properties", Obj("id", Obj("type", "string")),
			"required", Arr("id")),
		Handler: func(input map[string]any) (string, error) {
			return b.Check(Str(input, "id")), nil
		},
	}
}
