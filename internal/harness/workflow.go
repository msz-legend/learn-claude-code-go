package harness

import (
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

type WorkflowMeta struct {
	Name        string
	Description string
	Phases      []string
}

type AgentCall func(prompt string) (string, error)

type WorkflowStep struct {
	Name string
	Body func(*WorkflowContext) (string, error)
}

type WorkflowStepResult struct {
	Step   string `json:"step"`
	Output string `json:"output"`
	Cached bool   `json:"cached"`
	Millis int64  `json:"ms"`
}

type WorkflowResult struct {
	RunID    string               `json:"runId"`
	Workflow string               `json:"workflow"`
	Status   string               `json:"status"`
	Output   map[string]string    `json:"output"`
	Steps    []WorkflowStepResult `json:"steps"`
	Events   []string             `json:"events"`
}

func (r WorkflowResult) Render() string {
	var out strings.Builder
	out.WriteString(fmt.Sprintf("workflow %s run=%s status=%s\n", r.Workflow, r.RunID, r.Status))
	for _, step := range r.Steps {
		if step.Cached {
			out.WriteString("  - " + step.Step + " (cached)\n")
		} else {
			out.WriteString(fmt.Sprintf("  - %s (%d ms)\n", step.Step, step.Millis))
		}
	}
	if len(r.Events) > 0 {
		out.WriteString("events:\n")
		for _, event := range r.Events {
			out.WriteString("  · " + event + "\n")
		}
	}
	return strings.TrimSpace(out.String())
}

type workflowState struct {
	mu      sync.Mutex
	agents  map[string]string
	outputs map[string]string // completed step outputs, seeded from the journal
	journal string
	events  []string
}

func (s *workflowState) event(text string) {
	s.mu.Lock()
	s.events = append(s.events, text)
	s.mu.Unlock()
}

func (s *workflowState) appendJournal(event map[string]any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	file, err := os.OpenFile(s.journal, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = file.WriteString(Stringify(event) + "\n")
	return err
}

type WorkflowContext struct {
	agentCall AgentCall
	stepName  string
	state     *workflowState
}

func (c *WorkflowContext) Agent(prompt string) (string, error) {
	key := fmt.Sprintf("%s:%x:%d", c.stepName, hashText(prompt), len(prompt))
	c.state.mu.Lock()
	cached, ok := c.state.agents[key]
	c.state.mu.Unlock()
	if ok {
		c.state.event("agent.cached " + c.stepName)
		return cached, nil
	}
	c.state.event("agent.start " + c.stepName)
	output, err := c.agentCall(prompt)
	if err != nil {
		return "", err
	}
	c.state.mu.Lock()
	c.state.agents[key] = output
	c.state.mu.Unlock()
	if err := c.state.appendJournal(Obj(
		"type", "agent",
		"key", key,
		"step", c.stepName,
		"output", output,
		"at", time.Now().UTC().Format(time.RFC3339))); err != nil {
		return "", err
	}
	c.state.event("agent.done " + c.stepName)
	return output, nil
}

func (c *WorkflowContext) Parallel(prompts []string) ([]string, error) {
	if len(prompts) == 0 {
		return []string{}, nil
	}
	out := make([]string, len(prompts))
	errs := make([]error, len(prompts))
	var wait sync.WaitGroup
	for i, prompt := range prompts {
		wait.Add(1)
		go func(index int, text string) {
			defer wait.Done()
			out[index], errs[index] = c.Agent(text)
		}(i, prompt)
	}
	wait.Wait()
	for _, err := range errs {
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (c *WorkflowContext) Phase(title string) {
	c.state.event("phase " + title)
	fmt.Println("[workflow] phase: " + title)
}

// Previous returns a completed step's journaled output ("", if absent).
// On a resumed run this carries outputs of steps that were not re-executed.
func (c *WorkflowContext) Previous(step string) string {
	if c.state == nil {
		return ""
	}
	c.state.mu.Lock()
	defer c.state.mu.Unlock()
	return c.state.outputs[step]
}

func (c *WorkflowContext) Log(message string) {
	c.state.event("log " + message)
	fmt.Println("[workflow] " + message)
}

type WorkflowRunner struct {
	root      string
	meta      WorkflowMeta
	steps     []WorkflowStep
	agentCall AgentCall
}

func NewWorkflowRunner(workspace string, meta WorkflowMeta, steps []WorkflowStep, agentCall AgentCall) *WorkflowRunner {
	return &WorkflowRunner{
		root:      filepath.Join(workspace, ".workflows"),
		meta:      meta,
		steps:     steps,
		agentCall: agentCall,
	}
}

func (r *WorkflowRunner) Meta() WorkflowMeta { return r.meta }

func (r *WorkflowRunner) Run(runID string) (WorkflowResult, error) {
	id := strings.TrimSpace(runID)
	if id == "" {
		id = fmt.Sprintf("wf-%d", time.Now().UnixMilli())
	}
	if !regexp.MustCompile(`^[a-zA-Z0-9_.-]{1,80}$`).MatchString(id) {
		return WorkflowResult{}, fmt.Errorf("run id must be a safe 1-80 character slug")
	}
	dir := filepath.Join(r.root, id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return WorkflowResult{}, err
	}
	journal := filepath.Join(dir, "journal.jsonl")
	completed, agents := loadWorkflowJournal(journal)
	state := &workflowState{agents: agents, outputs: completed, journal: journal, events: []string{}}

	results := []WorkflowStepResult{}
	output := map[string]string{}
	status := "completed"

	for _, step := range r.steps {
		if cached, ok := completed[step.Name]; ok {
			results = append(results, WorkflowStepResult{Step: step.Name, Output: cached, Cached: true})
			output[step.Name] = cached
			state.event("step.cached " + step.Name)
			continue
		}
		start := time.Now()
		state.event("step.start " + step.Name)
		fmt.Println("[workflow] step: " + step.Name)
		ctx := &WorkflowContext{agentCall: r.agentCall, stepName: step.Name, state: state}
		value, err := step.Body(ctx)
		millis := time.Since(start).Milliseconds()
		if err != nil {
			results = append(results, WorkflowStepResult{Step: step.Name, Output: "Error: " + err.Error(), Millis: millis})
			output[step.Name] = "Error: " + err.Error()
			_ = state.appendJournal(Obj(
				"type", "step", "step", step.Name, "status", "error",
				"error", err.Error(), "at", time.Now().UTC().Format(time.RFC3339)))
			state.event("step.error " + step.Name)
			status = "failed"
			break
		}
		results = append(results, WorkflowStepResult{Step: step.Name, Output: value, Millis: millis})
		output[step.Name] = value
		if err := state.appendJournal(Obj(
			"type", "step", "step", step.Name, "status", "complete",
			"output", value, "ms", millis, "at", time.Now().UTC().Format(time.RFC3339))); err != nil {
			return WorkflowResult{}, err
		}
		state.event("step.complete " + step.Name)
	}

	if status == "completed" {
		state.event("workflow.complete " + r.meta.Name)
	}
	result := WorkflowResult{
		RunID: id, Workflow: r.meta.Name, Status: status,
		Output: output, Steps: results, Events: state.events,
	}
	data := Stringify(result)
	_ = os.WriteFile(filepath.Join(dir, "result.json"), []byte(data), 0o644)
	return result, nil
}

func loadWorkflowJournal(journal string) (map[string]string, map[string]string) {
	completed := map[string]string{}
	agents := map[string]string{}
	data, err := os.ReadFile(journal)
	if err != nil {
		return completed, agents
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		event, err := ParseJSON(line)
		if err != nil {
			continue
		}
		item := AsObject(event)
		switch Str(item, "type") {
		case "step":
			if Str(item, "status") == "complete" {
				completed[Str(item, "step")] = Str(item, "output")
			}
		case "agent":
			agents[Str(item, "key")] = Str(item, "output")
		}
	}
	return completed, agents
}

func hashText(text string) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(text))
	return h.Sum64()
}

type WorkflowRegistry struct {
	workspace string
	agentCall AgentCall
	runners   map[string]*WorkflowRunner
	order     []string
}

func NewWorkflowRegistry(workspace string, agentCall AgentCall) *WorkflowRegistry {
	return &WorkflowRegistry{workspace: workspace, agentCall: agentCall, runners: map[string]*WorkflowRunner{}}
}

func (r *WorkflowRegistry) Register(meta WorkflowMeta, steps []WorkflowStep) *WorkflowRegistry {
	if !regexp.MustCompile(`^[a-zA-Z0-9_.-]{1,64}$`).MatchString(meta.Name) {
		panic("workflow meta.name must be a safe 1-64 character slug")
	}
	if strings.TrimSpace(meta.Description) == "" {
		panic("workflow meta.description is required")
	}
	if _, exists := r.runners[meta.Name]; !exists {
		r.order = append(r.order, meta.Name)
	}
	r.runners[meta.Name] = NewWorkflowRunner(r.workspace, meta, steps, r.agentCall)
	return r
}

func (r *WorkflowRegistry) Run(name, resumeFromRunID string) (WorkflowResult, error) {
	runner, ok := r.runners[name]
	if !ok {
		names := append([]string{}, r.order...)
		sort.Strings(names)
		return WorkflowResult{}, fmt.Errorf("unknown workflow: %s (available: %s)",
			name, strings.Join(names, ", "))
	}
	return runner.Run(resumeFromRunID)
}

func (r *WorkflowRegistry) RenderCatalog() string {
	if len(r.runners) == 0 {
		return "(no workflows)"
	}
	var out strings.Builder
	for _, name := range r.order {
		runner := r.runners[name]
		out.WriteString("- " + runner.meta.Name + ": " + runner.meta.Description + "\n")
	}
	return strings.TrimSpace(out.String())
}

func (r *WorkflowRegistry) Tool() Tool {
	return Tool{
		Name:        "workflow",
		Description: "Run a saved workflow. The host owns the orchestration; pass a workflow name.",
		InputSchema: Obj(
			"type", "object",
			"properties", Obj(
				"name", Obj("type", "string"),
				"resume_from_run_id", Obj("type", "string")),
			"required", Arr("name")),
		Handler: func(input map[string]any) (string, error) {
			result, err := r.Run(Str(input, "name"), Str(input, "resume_from_run_id"))
			if err != nil {
				return "", err
			}
			fmt.Println(result.Render())
			return Stringify(result), nil
		},
	}
}
