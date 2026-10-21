package harness

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func withFileLock(lockFile string, fn func() error) error {
	_ = os.MkdirAll(filepath.Dir(lockFile), 0o755)
	var lastErr error
	for attempt := 0; attempt < 200; attempt++ {
		file, err := os.OpenFile(lockFile, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err == nil {
			_, _ = file.WriteString(fmt.Sprintf("%d", os.Getpid()))
			_ = file.Close()
			defer os.Remove(lockFile)
			return fn()
		}
		lastErr = err
		time.Sleep(10 * time.Millisecond)
	}
	return fmt.Errorf("could not acquire lock %s: %w", lockFile, lastErr)
}

type Task struct {
	ID          string   `json:"id"`
	Subject     string   `json:"subject"`
	Description string   `json:"description"`
	Status      string   `json:"status"`
	BlockedBy   []string `json:"blockedBy"`
	Owner       string   `json:"owner"`
	Workdir     string   `json:"workdir"`
	CreatedAt   string   `json:"createdAt"`
	UpdatedAt   string   `json:"updatedAt"`
}

type TaskStore struct {
	dir      string
	lockFile string
}

func NewTaskStore(workspace string) *TaskStore {
	dir := filepath.Join(workspace, ".tasks")
	_ = os.MkdirAll(dir, 0o755)
	return &TaskStore{dir: dir, lockFile: filepath.Join(dir, ".lock")}
}

func (s *TaskStore) Directory() string { return s.dir }

func (s *TaskStore) Create(subject, description string, blockedBy []string) string {
	var result string
	err := withFileLock(s.lockFile, func() error {
		title := strings.TrimSpace(subject)
		if title == "" {
			result = "Error: subject is required"
			return nil
		}
		now := time.Now().UTC()
		task := Task{
			ID:          "task-" + now.Format("20060102-150405") + "-" + fmt.Sprintf("%04d", now.Nanosecond()/1_000_000),
			Subject:     title,
			Description: strings.TrimSpace(description),
			Status:      "pending",
			BlockedBy:   append([]string{}, blockedBy...),
			CreatedAt:   now.Format(time.RFC3339Nano),
			UpdatedAt:   now.Format(time.RFC3339Nano),
		}
		if err := s.save(task); err != nil {
			return err
		}
		result = "Created " + task.ID + ": " + title
		return nil
	})
	if err != nil {
		return "Error: " + err.Error()
	}
	return result
}

func (s *TaskStore) Update(id string, addBlockedBy []string) string {
	var result string
	err := withFileLock(s.lockFile, func() error {
		task, ok := s.getLocked(id)
		if !ok {
			result = "Error: task not found: " + id
			return nil
		}
		deps := append([]string{}, task.BlockedBy...)
		for _, dependency := range addBlockedBy {
			if strings.TrimSpace(dependency) == "" || dependency == id {
				result = "Error: invalid dependency: " + dependency
				return nil
			}
			if s.dependsOnLocked(dependency, id) {
				result = "Error: dependency cycle: " + dependency + " already depends on " + id
				return nil
			}
			if !contains(deps, dependency) {
				deps = append(deps, dependency)
			}
		}
		task.BlockedBy = deps
		task.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
		if err := s.save(task); err != nil {
			return err
		}
		result = "Updated " + id + " blockedBy=" + strings.Join(deps, ",")
		return nil
	})
	if err != nil {
		return "Error: " + err.Error()
	}
	return result
}

func (s *TaskStore) Get(id string) (Task, bool) { return s.getLocked(id) }

func (s *TaskStore) List() []Task {
	tasks := []Task{}
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return tasks
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.dir, entry.Name()))
		if err != nil {
			continue
		}
		var task Task
		if err := json.Unmarshal(data, &task); err == nil {
			tasks = append(tasks, task)
		}
	}
	sort.Slice(tasks, func(i, j int) bool { return tasks[i].CreatedAt < tasks[j].CreatedAt })
	return tasks
}

func (s *TaskStore) Claim(id, owner string) string {
	var result string
	err := withFileLock(s.lockFile, func() error {
		task, ok := s.getLocked(id)
		if !ok {
			result = "Error: task not found: " + id
			return nil
		}
		if task.Status != "pending" {
			result = "Error: task " + id + " is " + task.Status
			return nil
		}
		if incomplete := s.incompleteDependenciesLocked(task); len(incomplete) > 0 {
			result = "Error: blocked by " + strings.Join(incomplete, ", ")
			return nil
		}
		who := strings.TrimSpace(owner)
		if who == "" {
			who = "agent"
		}
		for _, other := range s.List() {
			if other.Owner == who && other.Status == "in_progress" {
				result = "Error: " + who + " already has " + other.ID + " in progress"
				return nil
			}
		}
		task.Status = "in_progress"
		task.Owner = who
		task.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
		if err := s.save(task); err != nil {
			return err
		}
		result = "Claimed " + id + " for " + who
		return nil
	})
	if err != nil {
		return "Error: " + err.Error()
	}
	return result
}

func (s *TaskStore) Complete(id, owner string) string {
	var result string
	err := withFileLock(s.lockFile, func() error {
		task, ok := s.getLocked(id)
		if !ok {
			result = "Error: task not found: " + id
			return nil
		}
		if task.Status != "in_progress" {
			result = "Error: task " + id + " is not in progress"
			return nil
		}
		if strings.TrimSpace(owner) != "" && task.Owner != owner {
			result = "Error: task " + id + " is owned by " + task.Owner
			return nil
		}
		task.Status = "completed"
		task.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
		if err := s.save(task); err != nil {
			return err
		}
		result = "Completed " + id
		return nil
	})
	if err != nil {
		return "Error: " + err.Error()
	}
	return result
}

func (s *TaskStore) SetWorkdir(id, workdir string) string {
	var result string
	err := withFileLock(s.lockFile, func() error {
		task, ok := s.getLocked(id)
		if !ok {
			result = "Error: task not found: " + id
			return nil
		}
		task.Workdir = workdir
		task.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
		if err := s.save(task); err != nil {
			return err
		}
		result = "Set " + id + " workdir=" + workdir
		return nil
	})
	if err != nil {
		return "Error: " + err.Error()
	}
	return result
}

func (s *TaskStore) Ready() []Task {
	ready := []Task{}
	for _, task := range s.List() {
		if task.Status == "pending" && len(s.incompleteDependenciesLocked(task)) == 0 {
			ready = append(ready, task)
		}
	}
	return ready
}

func (s *TaskStore) Render() string {
	tasks := s.List()
	if len(tasks) == 0 {
		return "(no tasks)"
	}
	var out strings.Builder
	for _, task := range tasks {
		out.WriteString("[" + task.Status + "] " + task.ID + "  " + task.Subject)
		if task.Owner != "" {
			out.WriteString("  @" + task.Owner)
		}
		if len(task.BlockedBy) > 0 {
			out.WriteString("  blockedBy=" + strings.Join(task.BlockedBy, ","))
		}
		out.WriteString("\n")
	}
	return strings.TrimSpace(out.String())
}

func (s *TaskStore) CreateTool() Tool {
	return Tool{
		Name:        "task_create",
		Description: "Create a task in the shared task graph.",
		InputSchema: Obj(
			"type", "object",
			"properties", Obj(
				"subject", Obj("type", "string"),
				"description", Obj("type", "string"),
				"blocked_by", Obj("type", "array", "items", Obj("type", "string"))),
			"required", Arr("subject")),
		Handler: func(input map[string]any) (string, error) {
			return s.Create(Str(input, "subject"), Str(input, "description"),
				StringList(input["blocked_by"])), nil
		},
	}
}

func (s *TaskStore) ListTool() Tool {
	return Tool{
		Name:        "task_list",
		Description: "List tasks and their status.",
		InputSchema: Obj("type", "object", "properties", Obj()),
		Handler: func(map[string]any) (string, error) {
			return s.Render(), nil
		},
	}
}

func (s *TaskStore) GetTool() Tool {
	return Tool{
		Name:        "task_get",
		Description: "Read one task as JSON.",
		InputSchema: Obj(
			"type", "object",
			"properties", Obj("id", Obj("type", "string")),
			"required", Arr("id")),
		Handler: func(input map[string]any) (string, error) {
			task, ok := s.Get(Str(input, "id"))
			if !ok {
				return "Error: task not found", nil
			}
			return Stringify(task), nil
		},
	}
}

func (s *TaskStore) UpdateTool() Tool {
	return Tool{
		Name:        "task_update",
		Description: "Add dependencies to a task.",
		InputSchema: Obj(
			"type", "object",
			"properties", Obj(
				"id", Obj("type", "string"),
				"add_blocked_by", Obj("type", "array", "items", Obj("type", "string"))),
			"required", Arr("id")),
		Handler: func(input map[string]any) (string, error) {
			return s.Update(Str(input, "id"), StringList(input["add_blocked_by"])), nil
		},
	}
}

func (s *TaskStore) ClaimTool() Tool {
	return Tool{
		Name:        "task_claim",
		Description: "Claim a ready task. Only one in-progress task per owner.",
		InputSchema: Obj(
			"type", "object",
			"properties", Obj(
				"id", Obj("type", "string"),
				"owner", Obj("type", "string")),
			"required", Arr("id")),
		Handler: func(input map[string]any) (string, error) {
			return s.Claim(Str(input, "id"), Str(input, "owner", "agent")), nil
		},
	}
}

func (s *TaskStore) CompleteTool() Tool {
	return Tool{
		Name:        "task_complete",
		Description: "Mark a claimed task complete.",
		InputSchema: Obj(
			"type", "object",
			"properties", Obj(
				"id", Obj("type", "string"),
				"owner", Obj("type", "string")),
			"required", Arr("id")),
		Handler: func(input map[string]any) (string, error) {
			return s.Complete(Str(input, "id"), Str(input, "owner", "agent")), nil
		},
	}
}

func (s *TaskStore) getLocked(id string) (Task, bool) {
	if !strings.HasPrefix(id, "task-") || strings.ContainsAny(id, "/\\") {
		return Task{}, false
	}
	data, err := os.ReadFile(filepath.Join(s.dir, id+".json"))
	if err != nil {
		return Task{}, false
	}
	var task Task
	if err := json.Unmarshal(data, &task); err != nil {
		return Task{}, false
	}
	return task, true
}

func (s *TaskStore) save(task Task) error {
	data, err := json.MarshalIndent(task, "", "  ")
	if err != nil {
		return err
	}
	temp := filepath.Join(s.dir, task.ID+".tmp")
	if err := os.WriteFile(temp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(temp, filepath.Join(s.dir, task.ID+".json"))
}

func (s *TaskStore) incompleteDependenciesLocked(task Task) []string {
	incomplete := []string{}
	for _, dependency := range task.BlockedBy {
		other, ok := s.getLocked(dependency)
		if !ok || other.Status != "completed" {
			incomplete = append(incomplete, dependency)
		}
	}
	return incomplete
}

func (s *TaskStore) dependsOnLocked(id, target string) bool {
	seen := map[string]bool{}
	var visit func(string) bool
	visit = func(current string) bool {
		if current == target {
			return true
		}
		if seen[current] {
			return false
		}
		seen[current] = true
		task, ok := s.getLocked(current)
		if !ok {
			return false
		}
		for _, dependency := range task.BlockedBy {
			if visit(dependency) {
				return true
			}
		}
		return false
	}
	return visit(id)
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
