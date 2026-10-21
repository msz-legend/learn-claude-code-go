package harness

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type TeamMember struct {
	Name   string `json:"name"`
	Role   string `json:"role"`
	Status string `json:"status"`
}

type TeamMail struct {
	ID        string `json:"id"`
	From      string `json:"from"`
	To        string `json:"to"`
	Type      string `json:"type"`
	Content   string `json:"content"`
	CreatedAt string `json:"createdAt"`
}

type TeamToolFactory func(team *Team, member TeamMember, workspace *Workspace) *Registry

var teamNamePattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,32}$`)

// Team coordinates persistent teammates through disk-backed mailboxes and a
// shared task board.
type Team struct {
	mu          sync.Mutex
	root        string
	mailboxes   string
	cursors     string
	lockFile    string
	workspace   *Workspace
	tasks       *TaskStore
	client      Client
	members     map[string]TeamMember
	threads     map[string]bool
	toolFactory TeamToolFactory
	sequence    atomic.Int64
}

func NewTeam(workspace *Workspace, tasks *TaskStore, client Client) *Team {
	root := filepath.Join(workspace.Root(), ".team")
	t := &Team{
		root:      root,
		mailboxes: filepath.Join(root, "mailboxes"),
		cursors:   filepath.Join(root, "cursors"),
		lockFile:  filepath.Join(root, ".lock"),
		workspace: workspace,
		tasks:     tasks,
		client:    client,
		members:   map[string]TeamMember{},
		threads:   map[string]bool{},
	}
	_ = os.MkdirAll(t.mailboxes, 0o755)
	_ = os.MkdirAll(t.cursors, 0o755)
	t.loadMembers()
	return t
}

func (t *Team) ToolFactory(factory TeamToolFactory) *Team {
	t.toolFactory = factory
	return t
}

func (t *Team) Tasks() *TaskStore { return t.tasks }

func (t *Team) Spawn(name, role, prompt string, worktree bool, taskID string) string {
	memberName := strings.TrimSpace(name)
	if !teamNamePattern.MatchString(memberName) {
		return "Error: name must be 1-32 letters, digits, _ or -"
	}
	if memberName == "lead" {
		return "Error: lead is reserved"
	}
	t.mu.Lock()
	if _, exists := t.members[memberName]; exists {
		t.mu.Unlock()
		return "Error: teammate already exists: " + memberName
	}
	t.mu.Unlock()
	if t.client == nil || t.toolFactory == nil {
		return "Error: team is not configured with a model client and tool factory"
	}

	cwd := t.workspace.Root()
	if worktree {
		created := t.CreateWorktree(memberName)
		if strings.HasPrefix(created, "Error:") {
			return created
		}
		cwd = strings.TrimSpace(strings.TrimPrefix(created, "Worktree:"))
	}
	if taskID != "" {
		t.tasks.SetWorkdir(taskID, cwd)
	}

	member := TeamMember{Name: memberName, Role: strings.TrimSpace(role), Status: "working"}
	if member.Role == "" {
		member.Role = "teammate"
	}
	t.mu.Lock()
	t.members[memberName] = member
	t.saveMembersLocked()
	t.mu.Unlock()

	memberWorkspace := NewWorkspace(cwd)
	tools := t.toolFactory(t, member, memberWorkspace)
	system := fmt.Sprintf(`You are teammate %s (%s) in a small coding team at %s.
You share a task board with the lead and other teammates.
Use task_list and task_claim before working, and task_complete when done.
Send progress or findings with team_send to "lead".
Check team_inbox for messages. Stay inside your workspace.
Act, don't explain.`, memberName, member.Role, cwd)

	history := []Msg{}
	if strings.TrimSpace(prompt) == "" {
		prompt = "Start by listing the task board and claiming a ready task."
	}
	history = append(history, User(prompt))

	loop := NewAgentLoop(t.client, tools, system).
		MaxTurns(GetInt("TEAMMATE_MAX_TURNS", 30)).
		Notifications(func() []string {
			notes := []string{}
			for _, mail := range t.InboxMessages(memberName) {
				notes = append(notes, "[team "+mail.From+"] "+mail.Content)
			}
			return notes
		}).
		Listener(&Listener{
			OnToolStart: func(call Block) {
				fmt.Println("[" + memberName + "] > " + call.Name)
			},
			OnToolResult: func(call Block, output string, isError bool) {
				if isError {
					fmt.Println("[" + memberName + "] ! " + Shorten(output, 240))
				}
			},
		})

	t.mu.Lock()
	t.threads[memberName] = true
	t.mu.Unlock()

	go func() {
		text, err := loop.Run(&history)
		if err != nil {
			t.Send(memberName, "lead", "error", "Teammate failed: "+err.Error())
			t.setStatus(memberName, "offline")
		} else {
			if strings.TrimSpace(text) == "" {
				text = "(teammate finished without text)"
			}
			t.Send(memberName, "lead", "result", text)
			t.setStatus(memberName, "idle")
		}
		t.mu.Lock()
		delete(t.threads, memberName)
		t.mu.Unlock()
	}()

	return "Spawned " + memberName + " (" + member.Role + ") in " + cwd
}

func (t *Team) List() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	if len(t.members) == 0 {
		return "(no teammates)"
	}
	var out strings.Builder
	for _, member := range t.members {
		out.WriteString("- " + member.Name + " [" + member.Status + "] " + member.Role + "\n")
	}
	return strings.TrimSpace(out.String())
}

func (t *Team) Members() []TeamMember {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := []TeamMember{}
	for _, member := range t.members {
		out = append(out, member)
	}
	return out
}

func (t *Team) Send(from, to, messageType, content string) string {
	target := strings.TrimSpace(to)
	if target == "" {
		target = "lead"
	}
	t.mu.Lock()
	_, exists := t.members[target]
	t.mu.Unlock()
	if target != "lead" && !exists {
		return "Error: unknown recipient: " + target
	}
	text := strings.TrimSpace(content)
	if text == "" {
		return "Error: message content is required"
	}
	if strings.TrimSpace(from) == "" {
		from = "lead"
	}
	if strings.TrimSpace(messageType) == "" {
		messageType = "message"
	}
	mail := TeamMail{
		ID:        fmt.Sprintf("msg-%d-%d", time.Now().UnixMilli(), t.sequence.Add(1)),
		From:      from,
		To:        target,
		Type:      messageType,
		Content:   text,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	err := withFileLock(t.lockFile, func() error {
		file, err := os.OpenFile(filepath.Join(t.mailboxes, target+".jsonl"),
			os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			return err
		}
		defer file.Close()
		_, err = file.WriteString(Stringify(mail) + "\n")
		return err
	})
	if err != nil {
		return "Error: could not append mailbox: " + err.Error()
	}
	return "Sent to " + target
}

func (t *Team) InboxMessages(name string) []TeamMail {
	who := strings.TrimSpace(name)
	if who == "" {
		who = "lead"
	}
	file := filepath.Join(t.mailboxes, who+".jsonl")
	data, err := os.ReadFile(file)
	if err != nil {
		return []TeamMail{}
	}
	out := []TeamMail{}
	err = withFileLock(t.lockFile, func() error {
		offset := t.readCursor(who)
		if offset > len(data) {
			offset = 0
		}
		chunk := data[offset:]
		lastNewline := strings.LastIndex(string(chunk), "\n")
		if lastNewline < 0 {
			return nil
		}
		complete := chunk[:lastNewline+1]
		for _, line := range strings.Split(string(complete), "\n") {
			if strings.TrimSpace(line) == "" {
				continue
			}
			var mail TeamMail
			if json.Unmarshal([]byte(line), &mail) == nil {
				out = append(out, mail)
			}
		}
		t.writeCursor(who, offset+len(complete))
		return nil
	})
	if err != nil {
		return []TeamMail{}
	}
	return out
}

func (t *Team) Inbox(name string) string {
	messages := t.InboxMessages(name)
	if len(messages) == 0 {
		return "(no new messages)"
	}
	var out strings.Builder
	for _, mail := range messages {
		out.WriteString("[" + mail.From + " / " + mail.Type + "] " + mail.Content + "\n")
	}
	return strings.TrimSpace(out.String())
}

func (t *Team) Shutdown(name string) string {
	target := strings.TrimSpace(name)
	t.mu.Lock()
	_, exists := t.members[target]
	t.mu.Unlock()
	if !exists {
		return "Error: unknown teammate: " + target
	}
	t.setStatus(target, "offline")
	t.Send("lead", target, "shutdown", "Please stop after your current step.")
	return "Requested shutdown of " + target
}

func (t *Team) CreateWorktree(name string) string {
	repo := t.workspace.Root()
	inside, err := RunShell("git rev-parse --is-inside-work-tree", repo, 20)
	if err != nil {
		return "Error: could not run git: " + err.Error()
	}
	if inside.ExitCode != 0 {
		return "Error: workspace is not a git repository; cannot create a worktree"
	}
	dir := filepath.Join(repo, ".worktrees", name)
	quoted := `"` + strings.ReplaceAll(dir, `"`, `\"`) + `"`
	_ = os.MkdirAll(filepath.Dir(dir), 0o755)
	result, _ := RunShell("git worktree add -b task/"+name+" "+quoted, repo, 120)
	if result.ExitCode != 0 {
		result, _ = RunShell("git worktree add "+quoted+" task/"+name, repo, 120)
	}
	if result.ExitCode != 0 {
		return "Error: git worktree failed: " + result.Output
	}
	return "Worktree: " + dir
}

func (t *Team) RemoveWorktree(name string) string {
	dir := filepath.Join(t.workspace.Root(), ".worktrees", name)
	result, err := RunShell(`git worktree remove --force "`+
		strings.ReplaceAll(dir, `"`, `\"`)+`"`, t.workspace.Root(), 120)
	if err != nil {
		return "Error: " + err.Error()
	}
	if result.ExitCode != 0 {
		return "Error: " + result.Output
	}
	return "Removed worktree " + dir
}

func (t *Team) SpawnTool() Tool {
	return Tool{
		Name:        "team_spawn",
		Description: "Start a persistent teammate with its own agent loop and mailbox.",
		InputSchema: Obj(
			"type", "object",
			"properties", Obj(
				"name", Obj("type", "string"),
				"role", Obj("type", "string"),
				"prompt", Obj("type", "string"),
				"worktree", Obj("type", "boolean"),
				"task_id", Obj("type", "string")),
			"required", Arr("name", "role", "prompt")),
		Handler: func(input map[string]any) (string, error) {
			return t.Spawn(Str(input, "name"), Str(input, "role", "teammate"),
				Str(input, "prompt"), Bool(input, "worktree", false), Str(input, "task_id")), nil
		},
	}
}

func (t *Team) ListTool() Tool {
	return Tool{
		Name:        "team_list",
		Description: "List teammates and their status.",
		InputSchema: Obj("type", "object", "properties", Obj()),
		Handler: func(map[string]any) (string, error) {
			return t.List(), nil
		},
	}
}

func (t *Team) SendTool(from string) Tool {
	return Tool{
		Name:        "team_send",
		Description: "Send a message to the lead or another teammate.",
		InputSchema: Obj(
			"type", "object",
			"properties", Obj(
				"to", Obj("type", "string"),
				"content", Obj("type", "string"),
				"type", Obj("type", "string")),
			"required", Arr("to", "content")),
		Handler: func(input map[string]any) (string, error) {
			return t.Send(from, Str(input, "to", "lead"), Str(input, "type", "message"),
				Str(input, "content")), nil
		},
	}
}

func (t *Team) InboxTool(name string) Tool {
	return Tool{
		Name:        "team_inbox",
		Description: "Read new messages from your mailbox.",
		InputSchema: Obj("type", "object", "properties", Obj()),
		Handler: func(map[string]any) (string, error) {
			return t.Inbox(name), nil
		},
	}
}

func (t *Team) ShutdownTool() Tool {
	return Tool{
		Name:        "team_shutdown",
		Description: "Ask a teammate to stop.",
		InputSchema: Obj(
			"type", "object",
			"properties", Obj("name", Obj("type", "string")),
			"required", Arr("name")),
		Handler: func(input map[string]any) (string, error) {
			return t.Shutdown(Str(input, "name")), nil
		},
	}
}

func (t *Team) setStatus(name, status string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	member, exists := t.members[name]
	if !exists {
		return
	}
	member.Status = status
	t.members[name] = member
	t.saveMembersLocked()
}

func (t *Team) readCursor(name string) int {
	data, err := os.ReadFile(filepath.Join(t.cursors, name+".json"))
	if err != nil {
		return 0
	}
	var cursor struct {
		Offset int `json:"offset"`
	}
	if json.Unmarshal(data, &cursor) != nil {
		return 0
	}
	return cursor.Offset
}

func (t *Team) writeCursor(name string, offset int) {
	data, _ := json.Marshal(Obj("offset", offset))
	_ = os.MkdirAll(t.cursors, 0o755)
	_ = os.WriteFile(filepath.Join(t.cursors, name+".json"), data, 0o644)
}

func (t *Team) loadMembers() {
	data, err := os.ReadFile(filepath.Join(t.root, "members.json"))
	if err != nil {
		return
	}
	var members []TeamMember
	if json.Unmarshal(data, &members) != nil {
		return
	}
	for _, member := range members {
		t.members[member.Name] = member
	}
}

func (t *Team) saveMembersLocked() {
	members := make([]TeamMember, 0, len(t.members))
	for _, member := range t.members {
		members = append(members, member)
	}
	data, err := json.MarshalIndent(members, "", "  ")
	if err != nil {
		return
	}
	_ = os.MkdirAll(t.root, 0o755)
	_ = os.WriteFile(filepath.Join(t.root, "members.json"), data, 0o644)
}
