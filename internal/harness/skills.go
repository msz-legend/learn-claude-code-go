package harness

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Skill struct {
	Name        string
	Description string
	Path        string
}

// Skills exposes a catalog; the body is read only when the model asks.
type Skills struct {
	root   string
	byName map[string]Skill
}

func NewSkills(root string) *Skills {
	abs, _ := filepath.Abs(root)
	return &Skills{root: abs, byName: map[string]Skill{}}
}

func (s *Skills) Catalog() []Skill {
	s.byName = map[string]Skill{}
	skills := []Skill{}
	_ = filepath.WalkDir(s.root, func(current string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() || !strings.EqualFold(entry.Name(), "SKILL.md") {
			return nil
		}
		data, err := os.ReadFile(current)
		if err != nil {
			return nil
		}
		text := string(data)
		front := frontmatter(text)
		name := strings.TrimSpace(front["name"])
		if name == "" {
			name = filepath.Base(filepath.Dir(current))
		}
		description := strings.TrimSpace(front["description"])
		if description == "" {
			description = firstBodyLine(text)
		}
		if len(description) > 300 {
			description = description[:300] + "..."
		}
		skill := Skill{Name: name, Description: description, Path: current}
		skills = append(skills, skill)
		s.byName[name] = skill
		return nil
	})
	sort.Slice(skills, func(i, j int) bool { return skills[i].Name < skills[j].Name })
	return skills
}

func (s *Skills) CatalogText() string {
	skills := s.Catalog()
	if len(skills) == 0 {
		return "(no skills found)"
	}
	var out strings.Builder
	out.WriteString("Available skills:\n")
	for _, skill := range skills {
		out.WriteString("- " + skill.Name + ": " + skill.Description + "\n")
	}
	out.WriteString("\nCall load_skill with a skill name to read its instructions.")
	return out.String()
}

func (s *Skills) Load(name string) string {
	if len(s.byName) == 0 {
		s.Catalog()
	}
	skill, ok := s.byName[name]
	if !ok {
		return "Error: unknown skill: " + name
	}
	data, err := os.ReadFile(skill.Path)
	if err != nil {
		return "Error: " + err.Error()
	}
	text := string(data)
	if len(text) > 60_000 {
		text = text[:60_000] + "\n... (truncated)"
	}
	return text
}

func (s *Skills) ListTool() Tool {
	return Tool{
		Name:        "list_skills",
		Description: "List the skills available on this machine.",
		InputSchema: Obj("type", "object", "properties", Obj()),
		Handler: func(map[string]any) (string, error) {
			return s.CatalogText(), nil
		},
	}
}

func (s *Skills) LoadTool() Tool {
	return Tool{
		Name:        "load_skill",
		Description: "Load the full instructions for one skill.",
		InputSchema: Obj(
			"type", "object",
			"properties", Obj("name", Obj("type", "string")),
			"required", Arr("name")),
		Handler: func(input map[string]any) (string, error) {
			return s.Load(Str(input, "name")), nil
		},
	}
}

func frontmatter(text string) map[string]string {
	out := map[string]string{}
	if !strings.HasPrefix(text, "---") {
		return out
	}
	end := strings.Index(text[3:], "\n---")
	if end < 0 {
		return out
	}
	block := text[3 : 3+end]
	for _, line := range strings.Split(block, "\n") {
		split := strings.Index(line, ":")
		if split <= 0 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(line[:split]))
		value := strings.TrimSpace(line[split+1:])
		if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
			value = value[1 : len(value)-1]
		}
		out[key] = value
	}
	return out
}

func firstBodyLine(text string) string {
	inFront := false
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "---" {
			inFront = !inFront
			continue
		}
		if !inFront && trimmed != "" && !strings.HasPrefix(trimmed, "#") {
			return trimmed
		}
	}
	return "(no description)"
}

func (s Skill) String() string {
	return fmt.Sprintf("%s: %s", s.Name, s.Description)
}
