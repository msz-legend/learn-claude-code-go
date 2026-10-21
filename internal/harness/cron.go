package harness

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

type CronJob struct {
	ID         string `json:"id"`
	Expression string `json:"expression"`
	Prompt     string `json:"prompt"`
	Recurring  bool   `json:"recurring"`
	Enabled    bool   `json:"enabled"`
	LastRunKey string `json:"lastRunKey"`
	CreatedAt  string `json:"createdAt"`
}

// Cron is a durable five-field scheduler. Due jobs enter the next request as
// harness notifications.
type Cron struct {
	mu      sync.Mutex
	file    string
	jobs    []CronJob
	due     []CronJob
	stop    chan struct{}
	running bool
}

func NewCron(workspace string) *Cron {
	c := &Cron{file: filepath.Join(workspace, ".cron", "jobs.json")}
	c.load()
	return c
}

func (c *Cron) Schedule(expression, prompt string, recurring bool) string {
	if err := ValidateCron(expression); err != nil {
		return "Error: " + err.Error()
	}
	text := strings.TrimSpace(prompt)
	if text == "" {
		return "Error: prompt is required"
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	job := CronJob{
		ID:         fmt.Sprintf("cron-%d-%d", time.Now().UnixMilli(), len(c.jobs)+1),
		Expression: strings.TrimSpace(expression),
		Prompt:     text,
		Recurring:  recurring,
		Enabled:    true,
		CreatedAt:  time.Now().UTC().Format(time.RFC3339),
	}
	c.jobs = append(c.jobs, job)
	c.saveLocked()
	return "Scheduled " + job.ID + " [" + job.Expression + "] " + text
}

func (c *Cron) Cancel(id string) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i, job := range c.jobs {
		if job.ID == id {
			c.jobs = append(c.jobs[:i], c.jobs[i+1:]...)
			c.saveLocked()
			return "Cancelled " + id
		}
	}
	return "Error: unknown job: " + id
}

func (c *Cron) Render() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.jobs) == 0 {
		return "(no scheduled jobs)"
	}
	var out strings.Builder
	for _, job := range c.jobs {
		state := "[on] "
		if !job.Enabled {
			state = "[off] "
		}
		out.WriteString(state + job.ID + "  " + job.Expression + "  " + job.Prompt)
		if job.LastRunKey != "" {
			out.WriteString("  last=" + job.LastRunKey)
		}
		out.WriteString("\n")
	}
	return strings.TrimSpace(out.String())
}

func (c *Cron) Jobs() []CronJob {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]CronJob{}, c.jobs...)
}

func (c *Cron) DrainDue() []CronJob {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := append([]CronJob{}, c.due...)
	c.due = nil
	return out
}

func (c *Cron) Poll(now time.Time) {
	key := now.Format("2006-01-02T15:04")
	c.mu.Lock()
	defer c.mu.Unlock()
	changed := false
	for i := range c.jobs {
		job := &c.jobs[i]
		if !job.Enabled || job.LastRunKey == key {
			continue
		}
		if !CronMatches(job.Expression, now) {
			continue
		}
		c.due = append(c.due, *job)
		job.LastRunKey = key
		job.Enabled = job.Recurring
		changed = true
	}
	if changed {
		c.saveLocked()
	}
}

func (c *Cron) Start() {
	c.mu.Lock()
	if c.running {
		c.mu.Unlock()
		return
	}
	c.running = true
	c.stop = make(chan struct{})
	stop := c.stop
	c.mu.Unlock()
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				c.Poll(time.Now())
			case <-stop:
				return
			}
		}
	}()
}

func (c *Cron) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.running {
		return
	}
	c.running = false
	close(c.stop)
}

func (c *Cron) CreateTool() Tool {
	return Tool{
		Name:        "cron_create",
		Description: "Schedule a prompt with a five-field cron expression.",
		InputSchema: Obj(
			"type", "object",
			"properties", Obj(
				"cron", Obj("type", "string"),
				"prompt", Obj("type", "string"),
				"recurring", Obj("type", "boolean")),
			"required", Arr("cron", "prompt")),
		Handler: func(input map[string]any) (string, error) {
			return c.Schedule(Str(input, "cron"), Str(input, "prompt"),
				Bool(input, "recurring", true)), nil
		},
	}
}

func (c *Cron) ListTool() Tool {
	return Tool{
		Name:        "cron_list",
		Description: "List scheduled jobs.",
		InputSchema: Obj("type", "object", "properties", Obj()),
		Handler: func(map[string]any) (string, error) {
			return c.Render(), nil
		},
	}
}

func (c *Cron) CancelTool() Tool {
	return Tool{
		Name:        "cron_cancel",
		Description: "Cancel a scheduled job by id.",
		InputSchema: Obj(
			"type", "object",
			"properties", Obj("id", Obj("type", "string")),
			"required", Arr("id")),
		Handler: func(input map[string]any) (string, error) {
			return c.Cancel(Str(input, "id")), nil
		},
	}
}

func (c *Cron) load() {
	data, err := os.ReadFile(c.file)
	if err != nil {
		return
	}
	var root struct {
		Jobs []CronJob `json:"jobs"`
	}
	if err := json.Unmarshal(data, &root); err != nil {
		return
	}
	c.jobs = root.Jobs
}

func (c *Cron) saveLocked() {
	_ = os.MkdirAll(filepath.Dir(c.file), 0o755)
	data, err := json.MarshalIndent(Obj("jobs", c.jobs), "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(c.file, data, 0o644)
}

func ValidateCron(expression string) error {
	fields := strings.Fields(strings.TrimSpace(expression))
	if len(fields) != 5 {
		return fmt.Errorf("cron expression must have five fields")
	}
	bounds := [][2]int{{0, 59}, {0, 23}, {1, 31}, {1, 12}, {0, 7}}
	for i, field := range fields {
		if _, err := parseCronField(field, bounds[i][0], bounds[i][1]); err != nil {
			return err
		}
	}
	return nil
}

func CronMatches(expression string, now time.Time) bool {
	fields := strings.Fields(strings.TrimSpace(expression))
	if len(fields) != 5 {
		return false
	}
	values := []int{now.Minute(), now.Hour(), now.Day(), int(now.Month()), int(now.Weekday())}
	for i, field := range fields {
		match, err := parseCronField(field, map[int][2]int{
			0: {0, 59}, 1: {0, 23}, 2: {1, 31}, 3: {1, 12}, 4: {0, 7},
		}[i][0], map[int][2]int{
			0: {0, 59}, 1: {0, 23}, 2: {1, 31}, 3: {1, 12}, 4: {0, 7},
		}[i][1])
		if err != nil {
			return false
		}
		value := values[i]
		ok := match(value)
		if i == 4 && value == 0 && match(7) {
			ok = true
		}
		if !ok {
			return false
		}
	}
	return true
}

func parseCronField(field string, min, max int) (func(int) bool, error) {
	allowed := map[int]bool{}
	for _, part := range strings.Split(field, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			return nil, fmt.Errorf("empty cron field")
		}
		step := 1
		rangePart := part
		if slash := strings.Index(part, "/"); slash >= 0 {
			rangePart = part[:slash]
			parsed, err := strconv.Atoi(part[slash+1:])
			if err != nil || parsed <= 0 {
				return nil, fmt.Errorf("invalid cron step: %s", part)
			}
			step = parsed
		}
		start, end := min, max
		switch {
		case rangePart == "*" || rangePart == "?":
			// full range
		case strings.Contains(rangePart, "-"):
			pieces := strings.SplitN(rangePart, "-", 2)
			a, errA := strconv.Atoi(pieces[0])
			b, errB := strconv.Atoi(pieces[1])
			if errA != nil || errB != nil {
				return nil, fmt.Errorf("invalid cron range: %s", part)
			}
			start, end = a, b
		default:
			value, err := strconv.Atoi(rangePart)
			if err != nil {
				return nil, fmt.Errorf("invalid cron field: %s", part)
			}
			start, end = value, value
			if step > 1 {
				end = max
			}
		}
		if start < min || end > max || start > end {
			return nil, fmt.Errorf("cron field out of range: %s", part)
		}
		for value := start; value <= end; value += step {
			allowed[value] = true
		}
	}
	return func(value int) bool { return allowed[value] }, nil
}
