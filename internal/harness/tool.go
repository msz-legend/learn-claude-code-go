package harness

import "strings"

type Tool struct {
	Name        string
	Description string
	InputSchema map[string]any
	Handler     func(map[string]any) (string, error)
}

type ToolResult struct {
	Output string
	Error  bool
}

func (t Tool) Spec() ToolSpec {
	schema := t.InputSchema
	if schema == nil {
		schema = Obj("type", "object")
	}
	return ToolSpec{Name: t.Name, Description: t.Description, InputSchema: schema}
}

func (t Tool) Run(input map[string]any) ToolResult {
	if input == nil {
		input = map[string]any{}
	}
	output, err := t.Handler(input)
	if err != nil {
		return ToolResult{Output: "Error: " + err.Error(), Error: true}
	}
	if output == "" {
		output = "(no output)"
	}
	return ToolResult{Output: output, Error: strings.HasPrefix(output, "Error:")}
}

type Registry struct {
	order []string
	tools map[string]Tool
}

func NewRegistry() *Registry {
	return &Registry{tools: map[string]Tool{}}
}

func (r *Registry) Register(tools ...Tool) *Registry {
	for _, tool := range tools {
		if _, exists := r.tools[tool.Name]; !exists {
			r.order = append(r.order, tool.Name)
		}
		r.tools[tool.Name] = tool
	}
	return r
}

func (r *Registry) Get(name string) (Tool, bool) {
	tool, ok := r.tools[name]
	return tool, ok
}

func (r *Registry) All() map[string]Tool {
	out := map[string]Tool{}
	for name, tool := range r.tools {
		out[name] = tool
	}
	return out
}

func (r *Registry) Names() []string {
	return append([]string{}, r.order...)
}

func (r *Registry) Specs() []ToolSpec {
	specs := []ToolSpec{}
	for _, name := range r.order {
		specs = append(specs, r.tools[name].Spec())
	}
	return specs
}

func (r *Registry) Execute(call Block) ToolResult {
	tool, ok := r.tools[call.Name]
	if !ok {
		return ToolResult{Output: "Error: unknown tool: " + call.Name, Error: true}
	}
	return tool.Run(call.Input)
}
