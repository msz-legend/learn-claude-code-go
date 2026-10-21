package harness

import (
	"fmt"
	"strings"
)

type PermissionMode string

const (
	PermAllow PermissionMode = "allow"
	PermDeny  PermissionMode = "deny"
	PermAsk   PermissionMode = "ask"
)

type Decision struct {
	Mode   PermissionMode
	Reason string
}

func Allow() Decision            { return Decision{Mode: PermAllow} }
func Deny(reason string) Decision { return Decision{Mode: PermDeny, Reason: reason} }
func Ask(reason string) Decision  { return Decision{Mode: PermAsk, Reason: reason} }

type Policy interface {
	Check(call Block, tool *Tool) Decision
}

type PolicyFunc func(Block, *Tool) Decision

func (f PolicyFunc) Check(call Block, tool *Tool) Decision { return f(call, tool) }

type Approver func(Block, string) bool

type Rule struct {
	Tool     string
	Contains string
	Mode     PermissionMode
	Reason   string
}

func (r Rule) Matches(call Block) bool {
	if r.Tool != "*" && r.Tool != call.Name {
		return false
	}
	if strings.TrimSpace(r.Contains) == "" {
		return true
	}
	return strings.Contains(searchable(call.Input), strings.ToLower(r.Contains))
}

func AllowAllPolicy() Policy {
	return PolicyFunc(func(Block, *Tool) Decision { return Allow() })
}

func RulesPolicy(rules []Rule) Policy {
	copied := append([]Rule{}, rules...)
	return PolicyFunc(func(call Block, tool *Tool) Decision {
		for _, rule := range copied {
			if rule.Matches(call) {
				return Decision{Mode: rule.Mode, Reason: rule.Reason}
			}
		}
		return Allow()
	})
}

func DefaultRules() []Rule {
	rules := []Rule{}
	for _, dangerous := range []string{"rm -rf /", "sudo ", "shutdown", "reboot", "mkfs", "dd if=", "> /dev/", ":(){:|:&};:"} {
		rules = append(rules, Rule{Tool: "bash", Contains: dangerous, Mode: PermDeny,
			Reason: "Dangerous command blocked: " + dangerous})
	}
	rules = append(rules,
		Rule{Tool: "bash", Contains: "rm ", Mode: PermAsk, Reason: "This command deletes files"},
		Rule{Tool: "bash", Contains: "git reset --hard", Mode: PermAsk, Reason: "This command discards local changes"},
		Rule{Tool: "bash", Contains: "git push --force", Mode: PermAsk, Reason: "This command rewrites remote history"},
		Rule{Tool: "bash", Contains: "git clean -fd", Mode: PermAsk, Reason: "This command deletes untracked files"},
		Rule{Tool: "bash", Contains: "chmod 777", Mode: PermAsk, Reason: "This command widens file permissions"},
		Rule{Tool: "write_file", Contains: ".env", Mode: PermAsk, Reason: "This writes a secrets file"},
	)
	return rules
}

func IsDangerous(command string) bool {
	text := strings.ToLower(command)
	for _, marker := range []string{"rm -rf /", "sudo ", "shutdown", "reboot", "mkfs", "dd if=", "> /dev/", ":(){:|:&};:"} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

func searchable(input map[string]any) string {
	var out strings.Builder
	for _, value := range input {
		out.WriteString(fmt.Sprint(value))
		out.WriteString("\n")
	}
	return strings.ToLower(out.String())
}
