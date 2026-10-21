package harness

// The provider-neutral message model shared by both API clients.

const (
	BlockText       = "text"
	BlockToolUse    = "tool_use"
	BlockToolResult = "tool_result"
)

type Block struct {
	Type      string         `json:"type"`
	Text      string         `json:"text,omitempty"`
	ID        string         `json:"id,omitempty"`
	Name      string         `json:"name,omitempty"`
	Input     map[string]any `json:"input,omitempty"`
	ToolUseID string         `json:"tool_use_id,omitempty"`
	Content   string         `json:"content,omitempty"`
	IsError   bool           `json:"is_error,omitempty"`
}

type Msg struct {
	Role   string  `json:"role"`
	Blocks []Block `json:"blocks"`
}

func TextBlock(text string) Block {
	return Block{Type: BlockText, Text: text}
}

func ToolUseBlock(id, name string, input map[string]any) Block {
	return Block{Type: BlockToolUse, ID: id, Name: name, Input: input}
}

func ToolResultBlock(toolUseID, content string, isError bool) Block {
	return Block{Type: BlockToolResult, ToolUseID: toolUseID, Content: content, IsError: isError}
}

func User(text string) Msg {
	return Msg{Role: "user", Blocks: []Block{TextBlock(text)}}
}

func Assistant(blocks []Block) Msg {
	return Msg{Role: "assistant", Blocks: blocks}
}

func ToolResults(results []Block) Msg {
	return Msg{Role: "user", Blocks: results}
}

func (m Msg) Text() string {
	text := ""
	for _, block := range m.Blocks {
		if block.Type == BlockText {
			if text != "" {
				text += "\n"
			}
			text += block.Text
		}
	}
	return text
}

func (m Msg) ToolUses() []Block {
	out := []Block{}
	for _, block := range m.Blocks {
		if block.Type == BlockToolUse {
			out = append(out, block)
		}
	}
	return out
}

type ToolSpec struct {
	Name        string
	Description string
	InputSchema map[string]any
}

type LlmResponse struct {
	Blocks     []Block
	StopReason string
}

func (r LlmResponse) Text() string {
	text := ""
	for _, block := range r.Blocks {
		if block.Type == BlockText {
			if text != "" {
				text += "\n"
			}
			text += block.Text
		}
	}
	return text
}

func (r LlmResponse) ToolUses() []Block {
	out := []Block{}
	for _, block := range r.Blocks {
		if block.Type == BlockToolUse {
			out = append(out, block)
		}
	}
	return out
}
