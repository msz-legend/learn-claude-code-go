package harness

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type MemoryEntry struct {
	ID        string   `json:"id"`
	Text      string   `json:"text"`
	Tags      []string `json:"tags"`
	CreatedAt string   `json:"createdAt"`
}

// Memory is a small durable store with selection, extraction and consolidation.
type Memory struct {
	mu      sync.Mutex
	file    string
	entries []MemoryEntry
}

func NewMemory(workspace string) *Memory {
	m := &Memory{file: filepath.Join(workspace, ".memory", "memory.json")}
	m.load()
	return m
}

func (m *Memory) Entries() []MemoryEntry {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]MemoryEntry{}, m.entries...)
}

func (m *Memory) Add(text string, tags []string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	value := strings.TrimSpace(text)
	if len(value) < 3 {
		return "Error: memory text is too short"
	}
	for _, entry := range m.entries {
		if strings.EqualFold(entry.Text, value) {
			return "Memory already exists: " + entry.ID
		}
	}
	entry := MemoryEntry{
		ID:        fmt.Sprintf("mem-%d", time.Now().UnixNano()),
		Text:      value,
		Tags:      append([]string{}, tags...),
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	m.entries = append(m.entries, entry)
	if len(m.entries) > 200 {
		m.entries = m.entries[len(m.entries)-200:]
	}
	m.save()
	return "Remembered: " + value
}

func (m *Memory) Search(query string, limit int) []MemoryEntry {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.searchLocked(query, limit)
}

func (m *Memory) searchLocked(query string, limit int) []MemoryEntry {
	if limit < 1 {
		limit = 5
	}
	if limit > 20 {
		limit = 20
	}
	text := strings.TrimSpace(query)
	if text == "" {
		out := append([]MemoryEntry{}, m.entries...)
		sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt > out[j].CreatedAt })
		if len(out) > limit {
			out = out[:limit]
		}
		return out
	}
	terms := memoryTerms(text)
	type scored struct {
		entry MemoryEntry
		score int
	}
	results := []scored{}
	for _, entry := range m.entries {
		haystack := strings.ToLower(entry.Text + " " + strings.Join(entry.Tags, " "))
		score := 0
		for _, term := range terms {
			if strings.Contains(haystack, term) {
				score += 2
			}
		}
		if score > 0 {
			results = append(results, scored{entry: entry, score: score})
		}
	}
	sort.Slice(results, func(i, j int) bool {
		if results[i].score != results[j].score {
			return results[i].score > results[j].score
		}
		return results[i].entry.CreatedAt > results[j].entry.CreatedAt
	})
	out := []MemoryEntry{}
	for i := 0; i < len(results) && i < limit; i++ {
		out = append(out, results[i].entry)
	}
	return out
}

func (m *Memory) RenderForPrompt(query string, limit int) string {
	found := m.Search(query, limit)
	if len(found) == 0 {
		return ""
	}
	var out strings.Builder
	out.WriteString("Relevant long-term memory:\n")
	for _, entry := range found {
		out.WriteString("- " + entry.Text)
		if len(entry.Tags) > 0 {
			out.WriteString(" [" + strings.Join(entry.Tags, ", ") + "]")
		}
		out.WriteString("\n")
	}
	return out.String()
}

func (m *Memory) Summary() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.entries) == 0 {
		return "(no long-term memory)"
	}
	var out strings.Builder
	out.WriteString("Long-term memory:\n")
	for _, entry := range m.entries {
		out.WriteString("- " + entry.Text + "\n")
	}
	return out.String()
}

func (m *Memory) Consolidate(client Client) (string, error) {
	m.mu.Lock()
	entries := append([]MemoryEntry{}, m.entries...)
	m.mu.Unlock()
	if len(entries) < 2 {
		return "Memory is already small; nothing to consolidate.", nil
	}
	raw, _ := json.Marshal(entries)
	prompt := `Merge duplicate or near-duplicate memories. Keep only durable,
useful project facts. Return JSON only: an array of objects with "text" and
"tags". At most 20 items.

Memories:
` + string(raw)
	response, err := client.Complete(
		"You consolidate a coding agent's long-term memory. Be conservative and concise.",
		[]Msg{User(prompt)}, nil)
	if err != nil {
		return "", err
	}
	items := extractJSONArray(response.Text())
	if items == nil {
		return "Error: could not parse consolidated memory", nil
	}
	next := []MemoryEntry{}
	for _, item := range items {
		text := strings.TrimSpace(Str(item, "text"))
		if len(text) < 3 {
			continue
		}
		next = append(next, MemoryEntry{
			ID:        fmt.Sprintf("mem-%d-%d", time.Now().UnixNano(), len(next)),
			Text:      text,
			Tags:      StringList(item["tags"]),
			CreatedAt: time.Now().UTC().Format(time.RFC3339),
		})
	}
	if len(next) == 0 {
		return "Error: consolidation produced no usable memories", nil
	}
	m.mu.Lock()
	m.entries = next
	m.save()
	m.mu.Unlock()
	return fmt.Sprintf("Consolidated %d memories into %d.", len(entries), len(next)), nil
}

func (m *Memory) Extract(client Client, userText, assistantText string) (string, error) {
	prompt := `Extract at most three durable facts from this exchange that would
help a future session. Skip transient details, secrets, and anything already
obvious from the repository. Return JSON only: an array of objects with "text"
and "tags". Return [] when nothing is worth keeping.

User:
` + userText + `

Assistant:
` + assistantText
	response, err := client.Complete(
		"You extract durable project memory for a coding agent. Return JSON only.",
		[]Msg{User(prompt)}, nil)
	if err != nil {
		return "", err
	}
	items := extractJSONArray(response.Text())
	if items == nil || len(items) == 0 {
		return "No durable facts found.", nil
	}
	saved := 0
	for _, item := range items {
		text := strings.TrimSpace(Str(item, "text"))
		if len(text) < 3 {
			continue
		}
		result := m.Add(text, StringList(item["tags"]))
		if !strings.HasPrefix(result, "Error:") {
			saved++
		}
	}
	return fmt.Sprintf("Extracted %d memories.", saved), nil
}

func (m *Memory) WriteTool() Tool {
	return Tool{
		Name:        "remember",
		Description: "Save a durable project fact to long-term memory.",
		InputSchema: Obj(
			"type", "object",
			"properties", Obj(
				"text", Obj("type", "string"),
				"tags", Obj("type", "array", "items", Obj("type", "string"))),
			"required", Arr("text")),
		Handler: func(input map[string]any) (string, error) {
			return m.Add(Str(input, "text"), StringList(input["tags"])), nil
		},
	}
}

func (m *Memory) SearchTool() Tool {
	return Tool{
		Name:        "recall",
		Description: "Search long-term memory for relevant facts.",
		InputSchema: Obj(
			"type", "object",
			"properties", Obj(
				"query", Obj("type", "string"),
				"limit", Obj("type", "integer")),
			"required", Arr("query")),
		Handler: func(input map[string]any) (string, error) {
			limit := Int(input, "limit", 5)
			rendered := m.RenderForPrompt(Str(input, "query"), limit)
			if rendered == "" {
				return "(no matching memories)", nil
			}
			return rendered, nil
		},
	}
}

func (m *Memory) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries = nil
	m.save()
}

func memoryTerms(text string) []string {
	fields := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !(r >= 'a' && r <= 'z') && !(r >= '0' && r <= '9') &&
			r < 0x80 && r != '_' && r != '-'
	})
	out := []string{}
	seen := map[string]bool{}
	for _, field := range fields {
		if len(field) < 2 || seen[field] {
			continue
		}
		seen[field] = true
		out = append(out, field)
	}
	return out
}

func (m *Memory) load() {
	data, err := os.ReadFile(m.file)
	if err != nil {
		return
	}
	var root struct {
		Entries []MemoryEntry `json:"entries"`
	}
	if err := json.Unmarshal(data, &root); err != nil {
		return
	}
	m.entries = root.Entries
}

func (m *Memory) save() {
	if err := os.MkdirAll(filepath.Dir(m.file), 0o755); err != nil {
		return
	}
	data, err := json.MarshalIndent(Obj("entries", m.entries), "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(m.file, data, 0o644)
}

func extractJSONArray(text string) []map[string]any {
	start := strings.Index(text, "[")
	end := strings.LastIndex(text, "]")
	if start < 0 || end <= start {
		return nil
	}
	parsed, err := ParseJSON(text[start : end+1])
	if err != nil {
		return nil
	}
	values, ok := parsed.([]any)
	if !ok {
		return nil
	}
	out := []map[string]any{}
	for _, value := range values {
		out = append(out, AsObject(value))
	}
	return out
}
