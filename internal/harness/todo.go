package harness

import (
	"fmt"
	"strconv"
	"strings"
)

func itoa(value int) string { return strconv.Itoa(value) }

type TodoItem struct {
	ID      string `json:"id"`
	Content string `json:"content"`
	Status  string `json:"status"`
}

// Todo is a small plan the model can replace as work progresses.
type Todo struct {
	items []TodoItem
}

func (t *Todo) Items() []TodoItem { return append([]TodoItem{}, t.items...) }

func (t *Todo) Write(raw any) string {
	values, ok := raw.([]any)
	if !ok {
		if text, isString := raw.(string); isString {
			parsed, err := ParseJSON(text)
			if err != nil {
				return "Error: todos must be an array of {content, status} objects"
			}
			values, ok = parsed.([]any)
			if !ok {
				return "Error: todos must be an array of {content, status} objects"
			}
		} else {
			return "Error: todos must be an array of {content, status} objects"
		}
	}
	next := []TodoItem{}
	inProgress := 0
	for index, value := range values {
		item := AsObject(value)
		content := strings.TrimSpace(Str(item, "content", Str(item, "text")))
		if content == "" {
			return fmt.Sprintf("Error: todo #%d has no content", index+1)
		}
		status := normalizeTodoStatus(Str(item, "status", "pending"))
		if status == "" {
			return fmt.Sprintf("Error: todo #%d has an invalid status", index+1)
		}
		if status == "in_progress" {
			inProgress++
		}
		id := Str(item, "id", fmt.Sprintf("todo-%d", index+1))
		next = append(next, TodoItem{ID: id, Content: content, Status: status})
	}
	if inProgress > 1 {
		return "Error: only one todo may be in_progress"
	}
	t.items = next
	return t.Render()
}

func (t *Todo) Render() string {
	if len(t.items) == 0 {
		return "(no todos)"
	}
	var out strings.Builder
	done := 0
	for _, item := range t.items {
		mark := "[ ]"
		switch item.Status {
		case "completed":
			mark = "[x]"
			done++
		case "in_progress":
			mark = "[>]"
		}
		out.WriteString(mark + " " + item.Content + "\n")
	}
	out.WriteString(fmt.Sprintf("(%d/%d completed)", done, len(t.items)))
	return out.String()
}

func (t *Todo) Tool() Tool {
	return Tool{
		Name: "todo_write",
		Description: "Create or replace the task plan. Use statuses pending, in_progress, completed. " +
			"Keep at most one item in_progress.",
		InputSchema: Obj(
			"type", "object",
			"properties", Obj(
				"todos", Obj(
					"type", "array",
					"items", Obj(
						"type", "object",
						"properties", Obj(
							"content", Obj("type", "string"),
							"status", Obj("type", "string",
								"enum", Arr("pending", "in_progress", "completed"))),
						"required", Arr("content", "status")))),
			"required", Arr("todos")),
		Handler: func(input map[string]any) (string, error) {
			return t.Write(input["todos"]), nil
		},
	}
}

func normalizeTodoStatus(raw string) string {
	switch strings.ReplaceAll(strings.ToLower(strings.TrimSpace(raw)), "-", "_") {
	case "pending", "todo", "not_started":
		return "pending"
	case "in_progress", "active", "doing":
		return "in_progress"
	case "completed", "complete", "done":
		return "completed"
	default:
		return ""
	}
}
