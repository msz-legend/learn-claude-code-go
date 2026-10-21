package harness

import (
	"strings"
)

type Listener struct {
	OnTurnStart     func(turn int)
	OnAssistantText func(text string)
	OnToolStart     func(call Block)
	OnToolResult    func(call Block, output string, isError bool)
	OnToolBlocked   func(call Block, reason string)
	OnStop          func(text, status string)
}

func (l *Listener) turn(turn int) {
	if l != nil && l.OnTurnStart != nil {
		l.OnTurnStart(turn)
	}
}
func (l *Listener) text(text string) {
	if l != nil && l.OnAssistantText != nil {
		l.OnAssistantText(text)
	}
}
func (l *Listener) toolStart(call Block) {
	if l != nil && l.OnToolStart != nil {
		l.OnToolStart(call)
	}
}
func (l *Listener) toolResult(call Block, output string, isError bool) {
	if l != nil && l.OnToolResult != nil {
		l.OnToolResult(call, output, isError)
	}
}
func (l *Listener) toolBlocked(call Block, reason string) {
	if l != nil && l.OnToolBlocked != nil {
		l.OnToolBlocked(call, reason)
	}
}
func (l *Listener) stop(text, status string) {
	if l != nil && l.OnStop != nil {
		l.OnStop(text, status)
	}
}

// AgentLoop is the one loop: model -> tools -> tool results -> repeat.
type AgentLoop struct {
	client        Client
	tools         *Registry
	system        string
	permission    Policy
	approver      Approver
	hooks         *Hooks
	compactor     Compactor
	notifications func() []string
	stopGate      StopGate
	listener      *Listener
	maxTurns      int
	lastStatus    string
}

func NewAgentLoop(client Client, tools *Registry, system string) *AgentLoop {
	return &AgentLoop{
		client:     client,
		tools:      tools,
		system:     system,
		permission: AllowAllPolicy(),
		hooks:      NoHooks(),
		compactor:  NoCompactor{},
		listener:   &Listener{},
	}
}

func (l *AgentLoop) Permission(policy Policy) *AgentLoop {
	if policy != nil {
		l.permission = policy
	}
	return l
}
func (l *AgentLoop) Approver(approver Approver) *AgentLoop { l.approver = approver; return l }
func (l *AgentLoop) Hooks(hooks *Hooks) *AgentLoop {
	if hooks != nil {
		l.hooks = hooks
	}
	return l
}
func (l *AgentLoop) Compactor(compactor Compactor) *AgentLoop {
	if compactor != nil {
		l.compactor = compactor
	}
	return l
}
func (l *AgentLoop) Notifications(source func() []string) *AgentLoop {
	l.notifications = source
	return l
}
func (l *AgentLoop) StopGate(gate StopGate) *AgentLoop { l.stopGate = gate; return l }
func (l *AgentLoop) Listener(listener *Listener) *AgentLoop {
	if listener != nil {
		l.listener = listener
	}
	return l
}
func (l *AgentLoop) MaxTurns(turns int) *AgentLoop {
	if turns > 0 {
		l.maxTurns = turns
	}
	return l
}
func (l *AgentLoop) System(system string) *AgentLoop { l.system = system; return l }
func (l *AgentLoop) Client() Client                  { return l.client }
func (l *AgentLoop) Tools() *Registry                { return l.tools }
func (l *AgentLoop) LastStatus() string              { return l.lastStatus }

func (l *AgentLoop) Run(history *[]Msg) (string, error) {
	l.lastStatus = "working"
	for i := len(*history) - 1; i >= 0; i-- {
		if (*history)[i].Role == "user" {
			if text := (*history)[i].Text(); strings.TrimSpace(text) != "" {
				if err := l.hooks.OnUserPrompt(text); err != nil {
					return "", err
				}
			}
			break
		}
	}

	for turn := 1; ; turn++ {
		if l.maxTurns > 0 && turn > l.maxTurns {
			l.lastStatus = "max_turns"
			message := "Stopped: the maximum turn count was reached."
			l.listener.stop(message, l.lastStatus)
			return message, nil
		}
		l.listener.turn(turn)

		if err := l.compactor.BeforeRequest(history, l.client); err != nil {
			return "", err
		}
		if l.notifications != nil {
			if notes := l.notifications(); len(notes) > 0 {
				*history = append(*history, User(strings.Join(notes, "\n\n")))
			}
		}

		response, err := l.client.Complete(l.system, *history, l.tools.Specs())
		if err != nil {
			l.lastStatus = "error"
			return "", err
		}
		*history = append(*history, Assistant(response.Blocks))

		text := response.Text()
		if strings.TrimSpace(text) != "" {
			l.listener.text(text)
		}

		calls := response.ToolUses()
		if len(calls) == 0 {
			status := "completed"
			if l.stopGate != nil {
				decision, err := l.stopGate.BeforeStop(*history, text)
				if err != nil {
					return "", err
				}
				if !decision.Stop {
					reason := decision.Reason
					if reason == "" {
						reason = "Continue working."
					}
					*history = append(*history, User(reason))
					continue
				}
				if decision.Status != "" {
					status = decision.Status
				}
			}
			l.lastStatus = status
			if err := l.hooks.OnStop(*history); err != nil {
				return "", err
			}
			l.listener.stop(text, status)
			return text, nil
		}

		results := []Block{}
		for _, call := range calls {
			l.listener.toolStart(call)
			tool, exists := l.tools.Get(call.Name)
			blocked := ""
			if !exists {
				blocked = "Unknown tool: " + call.Name
			} else {
				decision := l.permission.Check(call, &tool)
				switch decision.Mode {
				case PermDeny:
					blocked = decision.Reason
					if blocked == "" {
						blocked = "Denied by policy"
					}
				case PermAsk:
					approved := l.approver != nil && l.approver(call, decision.Reason)
					if !approved {
						blocked = "Permission denied: " + decision.Reason
					}
				}
			}
			if blocked == "" {
				reason, hookErr := l.hooks.BeforeTool(call)
				if hookErr != nil {
					// Record a tool_result anyway so the assistant's tool_use
					// block is always answered and history stays valid.
					results = append(results, ToolResultBlock(call.ID, "Error: pre-tool hook failed: "+hookErr.Error(), true))
					l.listener.toolBlocked(call, "pre-tool hook failed")
					continue
				}
				blocked = reason
			}
			if blocked != "" {
				results = append(results, ToolResultBlock(call.ID, "Error: "+blocked, true))
				l.listener.toolBlocked(call, blocked)
				continue
			}

			result := l.tools.Execute(call)
			output, hookErr := l.hooks.AfterTool(call, result.Output)
			if hookErr != nil {
				// The tool already ran; report both its output and the hook failure.
				output += "\nError: post-tool hook failed: " + hookErr.Error()
				results = append(results, ToolResultBlock(call.ID, output, true))
				l.listener.toolResult(call, output, true)
				continue
			}
			results = append(results, ToolResultBlock(call.ID, output, result.Error))
			l.listener.toolResult(call, output, result.Error)
		}
		*history = append(*history, ToolResults(results))
	}
}
