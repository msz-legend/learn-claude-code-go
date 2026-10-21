package harness

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// Workspace confines every file tool to one root directory.
type Workspace struct {
	root string
}

func NewWorkspace(root string) *Workspace {
	abs, err := filepath.Abs(root)
	if err != nil {
		abs = root
	}
	return &Workspace{root: abs}
}

func (w *Workspace) Root() string { return w.root }

func (w *Workspace) Resolve(name string) (string, error) {
	if name == "" {
		return w.root, nil
	}
	target := name
	if !filepath.IsAbs(target) {
		target = filepath.Join(w.root, target)
	}
	target = filepath.Clean(target)
	rel, err := filepath.Rel(w.root, target)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("path escapes workspace: %s", name)
	}
	return target, nil
}

func (w *Workspace) Read(name string, limit int) (string, error) {
	target, err := w.Resolve(name)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(target)
	if err != nil {
		return "", err
	}
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	if limit > 0 && limit < len(lines) {
		omitted := len(lines) - limit
		lines = append(lines[:limit], fmt.Sprintf("... (%d more lines)", omitted))
	}
	return strings.Join(lines, "\n"), nil
}

func (w *Workspace) Write(name, content string) (string, error) {
	target, err := w.Resolve(name)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(target, []byte(content), 0o644); err != nil {
		return "", err
	}
	return fmt.Sprintf("Wrote %d bytes to %s", len(content), name), nil
}

func (w *Workspace) Edit(name, oldText, newText string) (string, error) {
	target, err := w.Resolve(name)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(target)
	if err != nil {
		return "", err
	}
	text := string(data)
	if !strings.Contains(text, oldText) {
		return "", fmt.Errorf("text not found in %s", name)
	}
	text = strings.Replace(text, oldText, newText, 1)
	if err := os.WriteFile(target, []byte(text), 0o644); err != nil {
		return "", err
	}
	return "Edited " + name, nil
}

func (w *Workspace) Glob(pattern string) (string, error) {
	pattern = filepath.ToSlash(strings.TrimSpace(pattern))
	if pattern == "" {
		return "", fmt.Errorf("pattern is required")
	}
	matches := []string{}
	err := filepath.WalkDir(w.root, func(current string, entry os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if entry.IsDir() {
			if entry.Name() == ".git" || entry.Name() == "out" || entry.Name() == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(w.root, current)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if globMatch(pattern, rel) {
			matches = append(matches, rel)
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Strings(matches)
	if len(matches) == 0 {
		return "(no matches)", nil
	}
	if len(matches) > 200 {
		matches = append(matches[:200], fmt.Sprintf("... (%d more files)", len(matches)-200))
	}
	return strings.Join(matches, "\n"), nil
}

func globMatch(pattern, name string) bool {
	return matchParts(strings.Split(pattern, "/"), strings.Split(name, "/"))
}

func matchParts(pattern, name []string) bool {
	if len(pattern) == 0 {
		return len(name) == 0
	}
	if pattern[0] == "**" {
		if matchParts(pattern[1:], name) {
			return true
		}
		return len(name) > 0 && matchParts(pattern, name[1:])
	}
	if len(name) == 0 {
		return false
	}
	ok, err := path.Match(pattern[0], name[0])
	if err != nil || !ok {
		return false
	}
	return matchParts(pattern[1:], name[1:])
}
