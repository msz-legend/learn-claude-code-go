package harness

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestJSON(t *testing.T) {
	value := Obj("a", 1, "b", Arr(true, nil, "x"), "c", Obj("d", -2.5))
	text := Stringify(value)
	parsed, err := ParseJSON(text)
	if err != nil {
		t.Fatal(err)
	}
	item := AsObject(parsed)
	if Int(item, "a", 0) != 1 {
		t.Fatalf("a = %v", item["a"])
	}
	if len(AsArray(item["b"])) != 3 {
		t.Fatalf("b = %v", item["b"])
	}
	if !strings.Contains(Stringify(Obj("t", "a\"b\nc")), "\\\"") {
		t.Fatal("string escaping failed")
	}
}

func TestPermission(t *testing.T) {
	if !IsDangerous("rm -rf /") {
		t.Fatal("rm -rf / should be dangerous")
	}
	if IsDangerous("ls -la") {
		t.Fatal("ls should be safe")
	}
	policy := RulesPolicy(DefaultRules())
	safe := ToolUseBlock("1", "bash", Obj("command", "ls"))
	if policy.Check(safe, nil).Mode != PermAllow {
		t.Fatal("safe command should be allowed")
	}
	ask := ToolUseBlock("2", "bash", Obj("command", "rm notes.txt"))
	if policy.Check(ask, nil).Mode != PermAsk {
		t.Fatal("delete should require approval")
	}
}

func TestTodo(t *testing.T) {
	todo := &Todo{}
	todo.Write(Arr(
		Obj("content", "read code", "status", "completed"),
		Obj("content", "write test", "status", "in_progress")))
	if !strings.Contains(todo.Render(), "[>] write test") {
		t.Fatal(todo.Render())
	}
	result := todo.Write(Arr(
		Obj("content", "a", "status", "in_progress"),
		Obj("content", "b", "status", "in_progress")))
	if !strings.HasPrefix(result, "Error:") {
		t.Fatal("two in-progress todos must fail")
	}
}

func TestCron(t *testing.T) {
	if err := ValidateCron("* * * * *"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateCron("* * *"); err == nil {
		t.Fatal("three fields must be rejected")
	}
	now := parseTestTime(t, "2026-10-21T10:10:00Z")
	if !CronMatches("*/5 * * * *", now) {
		t.Fatal("*/5 should match minute 10")
	}
	if CronMatches("0 0 1 1 *", now) {
		t.Fatal("January 1 should not match October 21")
	}
}

func TestTasks(t *testing.T) {
	dir := t.TempDir()
	store := NewTaskStore(dir)
	first := store.Create("first", "", nil)
	if !strings.HasPrefix(first, "Created ") {
		t.Fatal(first)
	}
	id := strings.TrimSuffix(strings.TrimPrefix(first, "Created "), ": first")
	if _, ok := store.Get(id); !ok {
		t.Fatal("task not found")
	}
	second := store.Create("second", "", []string{id})
	secondID := strings.SplitN(strings.TrimPrefix(second, "Created "), ":", 2)[0]
	if !strings.HasPrefix(store.Claim(secondID, "me"), "Error:") {
		t.Fatal("blocked task must not be claimable")
	}
	if !strings.HasPrefix(store.Claim(id, "me"), "Claimed") {
		t.Fatal("ready task should be claimable")
	}
	if !strings.HasPrefix(store.Complete(id, "me"), "Completed") {
		t.Fatal("claimed task should complete")
	}
	if len(store.Ready()) != 1 {
		t.Fatalf("one task should be ready, got %d", len(store.Ready()))
	}
	if !strings.HasPrefix(store.Update(id, []string{secondID}), "Error:") {
		t.Fatal("dependency cycle must be rejected")
	}
}

func TestMemory(t *testing.T) {
	dir := t.TempDir()
	memory := NewMemory(dir)
	memory.Add("The build command is javac -d out", []string{"build"})
	if len(memory.Entries()) != 1 {
		t.Fatal("memory not saved")
	}
	if len(memory.Search("javac", 5)) != 1 {
		t.Fatal("search failed")
	}
	if !strings.Contains(memory.RenderForPrompt("build", 5), "javac") {
		t.Fatal("prompt rendering failed")
	}
	if !strings.HasPrefix(memory.Add("The build command is javac -d out", nil), "Memory already exists") {
		t.Fatal("duplicate must be rejected")
	}
}

func TestShell(t *testing.T) {
	command := "echo hello"
	if IsWindows() {
		command = "echo hello"
	}
	result, err := RunShell(command, ".", 20)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.ToLower(result.Output), "hello") {
		t.Fatalf("output = %q", result.Output)
	}
}

func TestWorkspace(t *testing.T) {
	dir := t.TempDir()
	workspace := NewWorkspace(dir)
	if _, err := workspace.Write("notes/a.txt", "hello"); err != nil {
		t.Fatal(err)
	}
	text, err := workspace.Read("notes/a.txt", 0)
	if err != nil || text != "hello" {
		t.Fatalf("read = %q, err = %v", text, err)
	}
	if _, err := workspace.Resolve("../escape.txt"); err == nil {
		t.Fatal("path escape must be rejected")
	}
	found, err := workspace.Glob("**/*.txt")
	if err != nil || !strings.Contains(found, "notes/a.txt") {
		t.Fatalf("glob = %q, err = %v", found, err)
	}
}

func TestWorkflowResume(t *testing.T) {
	dir := t.TempDir()
	calls := 0
	registry := NewWorkflowRegistry(dir, func(prompt string) (string, error) {
		calls++
		return "review of " + prompt, nil
	})
	registry.Register(WorkflowMeta{Name: "review", Description: "test"}, []WorkflowStep{
		{Name: "one", Body: func(ctx *WorkflowContext) (string, error) {
			return ctx.Agent("first")
		}},
		{Name: "two", Body: func(ctx *WorkflowContext) (string, error) {
			return ctx.Agent("second")
		}},
	})
	first, err := registry.Run("review", "run-1")
	if err != nil {
		t.Fatal(err)
	}
	if first.Status != "completed" || calls != 2 {
		t.Fatalf("status=%s calls=%d", first.Status, calls)
	}
	second, err := registry.Run("review", "run-1")
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatalf("resume repeated agent calls: %d", calls)
	}
	for _, step := range second.Steps {
		if !step.Cached {
			t.Fatalf("step %s was not cached", step.Step)
		}
	}
	_ = os.Remove(filepath.Join(dir, ".workflows"))
}

// stubClient answers with one recorded tool call, then a final text.
type stubClient struct {
	calls int
	name  string
}

func (c *stubClient) Complete(_ string, _ []Msg, _ []ToolSpec) (LlmResponse, error) {
	c.calls++
	if c.calls == 1 {
		return LlmResponse{Blocks: []Block{
			ToolUseBlock("stub_call_1", c.name, map[string]any{}),
		}, StopReason: "tool_use"}, nil
	}
	return LlmResponse{Blocks: []Block{TextBlock("stub final")}, StopReason: "end_turn"}, nil
}

func (c *stubClient) Model() string { return "stub" }

func TestHookErrorKeepsHistoryValid(t *testing.T) {
	broken := &Hooks{PreToolUse: func(Block) (string, error) {
		return "", errors.New("hook exploded")
	}}
	tools := NewRegistry().Register(Tool{Name: "stub", Handler: func(map[string]any) (string, error) {
		return "ran", nil
	}})
	loop := NewAgentLoop(&stubClient{name: "stub"}, tools, "system").Hooks(broken)
	history := []Msg{User("go")}
	if _, err := loop.Run(&history); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	last := history[len(history)-1]
	if last.Role != "assistant" || strings.TrimSpace(last.Text()) != "stub final" {
		t.Fatalf("unexpected final message: %+v", last)
	}
	errorResults := 0
	for _, message := range history {
		if message.Role != "user" {
			continue
		}
		for _, block := range message.Blocks {
			if block.Type == BlockToolResult && block.IsError &&
				strings.Contains(block.Content, "hook exploded") {
				errorResults++
			}
		}
	}
	if errorResults != 1 {
		t.Fatalf("expected exactly one hook-failure tool_result, got %d", errorResults)
	}
}

func TestWorkflowPreviousFeedsResumedStep(t *testing.T) {
	dir := filepath.Join(t.TempDir(), ".workflows")
	if err := os.MkdirAll(filepath.Join(dir, "partial"), 0o755); err != nil {
		t.Fatal(err)
	}
	journal := `{"step":"one","type":"step","status":"complete","output":"first output","ms":1,"at":"2025-01-01T00:00:00Z"}` + "\n"
	if err := os.WriteFile(filepath.Join(dir, "partial", "journal.jsonl"), []byte(journal), 0o644); err != nil {
		t.Fatal(err)
	}
	registry := NewWorkflowRegistry(filepath.Dir(dir), func(string) (string, error) {
		return "fresh", nil
	})
	registry.Register(WorkflowMeta{Name: "review", Description: "test"}, []WorkflowStep{
		{Name: "one", Body: func(*WorkflowContext) (string, error) { return "never", nil }},
		{Name: "two", Body: func(ctx *WorkflowContext) (string, error) {
			return "report over " + ctx.Previous("one"), nil
		}},
	})
	result, err := registry.Run("review", "partial")
	if err != nil {
		t.Fatal(err)
	}
	cached, fresh := 0, 0
	for _, step := range result.Steps {
		if step.Cached {
			cached++
		} else {
			fresh++
		}
	}
	if cached != 1 || fresh != 1 {
		t.Fatalf("cached=%d fresh=%d", cached, fresh)
	}
	if result.Output["two"] != "report over first output" {
		t.Fatalf("two = %q", result.Output["two"])
	}
}

func parseTestTime(t *testing.T, value string) time.Time {
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}
