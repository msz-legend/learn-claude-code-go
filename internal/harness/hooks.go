package harness

// Hooks are optional function fields around the loop. A nil field is a no-op.
type Hooks struct {
	UserPromptSubmit func(prompt string) error
	PreToolUse       func(call Block) (blockReason string, err error)
	PostToolUse      func(call Block, output string) (string, error)
	Stop             func(history []Msg) error
}

func NoHooks() *Hooks { return &Hooks{} }

func (h *Hooks) OnUserPrompt(prompt string) error {
	if h != nil && h.UserPromptSubmit != nil {
		return h.UserPromptSubmit(prompt)
	}
	return nil
}

func (h *Hooks) BeforeTool(call Block) (string, error) {
	if h != nil && h.PreToolUse != nil {
		return h.PreToolUse(call)
	}
	return "", nil
}

func (h *Hooks) AfterTool(call Block, output string) (string, error) {
	if h != nil && h.PostToolUse != nil {
		return h.PostToolUse(call, output)
	}
	return output, nil
}

func (h *Hooks) OnStop(history []Msg) error {
	if h != nil && h.Stop != nil {
		return h.Stop(history)
	}
	return nil
}

// ComposeHooks runs hooks in order. A block from any hook wins; post-tool
// output is threaded through every hook.
func ComposeHooks(hooks ...*Hooks) *Hooks {
	return &Hooks{
		UserPromptSubmit: func(prompt string) error {
			for _, hook := range hooks {
				if err := hook.OnUserPrompt(prompt); err != nil {
					return err
				}
			}
			return nil
		},
		PreToolUse: func(call Block) (string, error) {
			for _, hook := range hooks {
				reason, err := hook.BeforeTool(call)
				if err != nil || reason != "" {
					return reason, err
				}
			}
			return "", nil
		},
		PostToolUse: func(call Block, output string) (string, error) {
			current := output
			for _, hook := range hooks {
				next, err := hook.AfterTool(call, current)
				if err != nil {
					return current, err
				}
				current = next
			}
			return current, nil
		},
		Stop: func(history []Msg) error {
			for _, hook := range hooks {
				if err := hook.OnStop(history); err != nil {
					return err
				}
			}
			return nil
		},
	}
}
