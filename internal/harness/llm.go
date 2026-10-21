package harness

// Client is the provider-neutral model interface used by every chapter.
type Client interface {
	Complete(system string, messages []Msg, tools []ToolSpec) (LlmResponse, error)
	Model() string
}

func FromEnv() Client {
	switch Protocol() {
	case "anthropic":
		return NewAnthropicClient()
	case "openai":
		return NewOpenAIClient()
	default:
		panic("Unknown LLM_PROTOCOL: " + Protocol() + " (use anthropic or openai)")
	}
}
