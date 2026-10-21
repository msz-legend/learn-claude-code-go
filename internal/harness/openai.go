package harness

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// OpenAIClient speaks the OpenAI chat-completions shape used by OpenAI,
// DeepSeek, GLM, Qwen, Ollama and most compatible gateways.
type OpenAIClient struct {
	apiKey  string
	baseURL string
	model   string
	timeout int
}

func NewOpenAIClient() *OpenAIClient {
	return &OpenAIClient{
		apiKey:  Require("OPENAI_API_KEY"),
		baseURL: BaseURL(),
		model:   Model(),
		timeout: TimeoutSeconds(),
	}
}

func (c *OpenAIClient) Model() string { return c.model }

func (c *OpenAIClient) Complete(system string, messages []Msg, tools []ToolSpec) (LlmResponse, error) {
	encoded := []any{}
	if system != "" {
		encoded = append(encoded, Obj("role", "system", "content", system))
	}
	encoded = append(encoded, encodeOpenAIMessages(messages)...)

	body := Obj("model", c.model, "messages", encoded)
	if len(tools) > 0 {
		specs := []any{}
		for _, tool := range tools {
			specs = append(specs, Obj(
				"type", "function",
				"function", Obj(
					"name", tool.Name,
					"description", tool.Description,
					"parameters", tool.InputSchema,
				)))
		}
		body["tools"] = specs
	}

	raw, err := PostJSON(c.baseURL+"/chat/completions", map[string]string{
		"authorization": "Bearer " + c.apiKey,
	}, Stringify(body), c.timeout)
	if err != nil {
		return LlmResponse{}, err
	}

	var response struct {
		Choices []struct {
			Message struct {
				Content   any `json:"content"`
				ToolCalls []struct {
					ID       string `json:"id"`
					Function struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
	}
	if err := json.Unmarshal([]byte(raw), &response); err != nil {
		return LlmResponse{}, fmt.Errorf("decode openai response: %w", err)
	}
	if len(response.Choices) == 0 {
		return LlmResponse{}, errors.New("openai returned no choices")
	}

	choice := response.Choices[0]
	blocks := []Block{}
	if text := contentText(choice.Message.Content); text != "" {
		blocks = append(blocks, TextBlock(text))
	}
	for _, call := range choice.Message.ToolCalls {
		input := map[string]any{}
		if call.Function.Arguments != "" {
			if err := json.Unmarshal([]byte(call.Function.Arguments), &input); err != nil {
				input = Obj("_raw", call.Function.Arguments)
			}
		}
		blocks = append(blocks, ToolUseBlock(call.ID, call.Function.Name, input))
	}
	if len(blocks) == 0 {
		return LlmResponse{}, errors.New("openai returned no content")
	}
	return LlmResponse{Blocks: blocks, StopReason: choice.FinishReason}, nil
}

func contentText(value any) string {
	switch content := value.(type) {
	case string:
		return content
	case []any:
		text := ""
		for _, item := range content {
			part := AsObject(item)
			if Str(part, "type") == "text" {
				if text != "" {
					text += "\n"
				}
				text += Str(part, "text")
			}
		}
		return text
	default:
		return ""
	}
}

func encodeOpenAIMessages(messages []Msg) []any {
	out := []any{}
	for _, message := range messages {
		if message.Role == "user" {
			results := []Block{}
			for _, block := range message.Blocks {
				if block.Type == BlockToolResult {
					results = append(results, block)
				}
			}
			if len(results) > 0 {
				if text := message.Text(); strings.TrimSpace(text) != "" {
					out = append(out, Obj("role", "user", "content", text))
				}
				for _, block := range results {
					out = append(out, Obj(
						"role", "tool", "tool_call_id", block.ToolUseID, "content", block.Content))
				}
				continue
			}
			out = append(out, Obj("role", "user", "content", message.Text()))
			continue
		}

		text := message.Text()
		toolCalls := []any{}
		for _, block := range message.Blocks {
			if block.Type != BlockToolUse {
				continue
			}
			toolCalls = append(toolCalls, Obj(
				"id", block.ID,
				"type", "function",
				"function", Obj(
					"name", block.Name,
					"arguments", Stringify(block.Input),
				)))
		}
		entry := Obj("role", "assistant")
		if text != "" {
			entry["content"] = text
		} else {
			entry["content"] = nil
		}
		if len(toolCalls) > 0 {
			entry["tool_calls"] = toolCalls
		}
		out = append(out, entry)
	}
	return out
}
