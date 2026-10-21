package harness

import (
	"encoding/json"
	"fmt"
)

// Small helpers over encoding/json so tool code stays readable.

func Obj(pairs ...any) map[string]any {
	out := make(map[string]any, len(pairs)/2)
	for i := 0; i+1 < len(pairs); i += 2 {
		out[fmt.Sprint(pairs[i])] = pairs[i+1]
	}
	return out
}

func Arr(values ...any) []any {
	return values
}

func Str(m map[string]any, key string, fallback ...string) string {
	if value, ok := m[key]; ok {
		if text, ok := value.(string); ok {
			return text
		}
	}
	if len(fallback) > 0 {
		return fallback[0]
	}
	return ""
}

func Int(m map[string]any, key string, fallback int) int {
	switch value := m[key].(type) {
	case float64:
		return int(value)
	case int:
		return value
	case int64:
		return int(value)
	case json.Number:
		n, err := value.Int64()
		if err == nil {
			return int(n)
		}
	}
	return fallback
}

func Bool(m map[string]any, key string, fallback bool) bool {
	if value, ok := m[key].(bool); ok {
		return value
	}
	return fallback
}

func AsObject(value any) map[string]any {
	if m, ok := value.(map[string]any); ok {
		return m
	}
	return map[string]any{}
}

func AsArray(value any) []any {
	if a, ok := value.([]any); ok {
		return a
	}
	return []any{}
}

func ParseJSON(text string) (any, error) {
	var value any
	if err := json.Unmarshal([]byte(text), &value); err != nil {
		return nil, err
	}
	return value, nil
}

func Stringify(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}
	return string(data)
}

func StringList(value any) []string {
	out := []string{}
	for _, item := range AsArray(value) {
		out = append(out, fmt.Sprint(item))
	}
	return out
}
