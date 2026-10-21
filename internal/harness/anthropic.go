package harness

import (
	"encoding/json"
	"errors"
	"fmt"
)

// AnthropicClient speaks the Messages API, also used by Anthropic-compatible
// gateways.
type AnthropicClient struct {
	apiKey  string
	baseURL string
	model   string
	timeout int
}

func NewAnthropicClient() *AnthropicClient {
	return &AnthropicClient{
		apiKey:  Require("ANTHROPIC_API_KEY"),
		baseURL: BaseURL(),
		model:   Model(),
		timeout: TimeoutSeconds(),
	}
}

func (c *AnthropicClient) Model() string { return c.model }

func (c *AnthropicClient) Complete(system string, messages []Msg, tools []ToolSpec) (LlmResponse, error) {
	body := Obj(
		"model", c.model,
		"max_tokens", MaxTokens(),
		"messages", encodeAnthropicMessages(messages),
	)
	if system != "" {
		body["system"] = system
	}
	if len(tools) > 0 {
		specs := []any{}
		for _, tool := range tools {
			specs = append(specs, Obj(
				"name", tool.Name,
				"description", tool.Description,
				"input_schema", tool.InputSchema,
			))
		}
		body["tools"] = specs
	}

	raw, err := PostJSON(c.baseURL+"/v1/messages", map[string]string{
		"x-api-key":         c.apiKey,
		"anthropic-version": "2023-06-01",
	}, Stringify(body), c.timeout)
	if err != nil {
		return LlmResponse{}, err
	}

	var response struct {
		Content    []map[string]any `json:"content"`
		StopReason string           `json:"stop_reason"`
	}
	if err := json.Unmarshal([]byte(raw), &response); err != nil {
		return LlmResponse{}, fmt.Errorf("decode anthropic response: %w", err)
	}
	blocks := []Block{}
	for _, item := range response.Content {
		switch Str(item, "type") {
		case "text":
			blocks = append(blocks, TextBlock(Str(item, "text")))
		case "tool_use":
			blocks = append(blocks, ToolUseBlock(
				Str(item, "id"), Str(item, "name"), AsObject(item["input"])))
		}
	}
	if len(blocks) == 0 {
		return LlmResponse{}, errors.New("anthropic returned no content")
	}
	return LlmResponse{Blocks: blocks, StopReason: response.StopReason}, nil
}

func encodeAnthropicMessages(messages []Msg) []any {
	out := []any{}
	for _, message := range messages {
		content := []any{}
		for _, block := range message.Blocks {
			switch block.Type {
			case BlockText:
				content = append(content, Obj("type", "text", "text", block.Text))
			case BlockToolUse:
				content = append(content, Obj(
					"type", "tool_use", "id", block.ID, "name", block.Name, "input", block.Input))
			case BlockToolResult:
				content = append(content, Obj(
					"type", "tool_result", "tool_use_id", block.ToolUseID,
					"content", block.Content, "is_error", block.IsError))
			}
		}
		out = append(out, Obj("role", message.Role, "content", content))
	}
	return out
}
