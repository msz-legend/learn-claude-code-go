package harness

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"time"
)

type RemoteTool struct {
	Name        string
	Description string
	InputSchema map[string]any
}

type McpClient struct {
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	lines   chan string
	ids     int
	timeout time.Duration
	mu      sync.Mutex
}

func StartMcp(command []string, cwd string) (*McpClient, error) {
	if len(command) == 0 {
		return nil, fmt.Errorf("MCP command is empty")
	}
	cmd := exec.Command(command[0], command[1:]...)
	if cwd != "" {
		cmd.Dir = cwd
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	client := &McpClient{
		cmd:     cmd,
		stdin:   stdin,
		lines:   make(chan string, 64),
		timeout: time.Duration(GetInt("MCP_TIMEOUT_SECONDS", 60)) * time.Second,
	}
	go func() {
		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
		for scanner.Scan() {
			client.lines <- scanner.Text()
		}
		close(client.lines)
	}()
	return client, nil
}

func (c *McpClient) Initialize() (map[string]any, error) {
	result, err := c.request("initialize", Obj(
		"protocolVersion", "2024-11-05",
		"capabilities", Obj(),
		"clientInfo", Obj("name", "learn-claude-code-go", "version", "1.0")))
	if err != nil {
		return nil, err
	}
	if err := c.notify("notifications/initialized", Obj()); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *McpClient) ListTools() ([]RemoteTool, error) {
	result, err := c.request("tools/list", Obj())
	if err != nil {
		return nil, err
	}
	tools := []RemoteTool{}
	for _, value := range AsArray(result["tools"]) {
		item := AsObject(value)
		schema := AsObject(item["inputSchema"])
		if len(schema) == 0 {
			schema = AsObject(item["input_schema"])
		}
		if len(schema) == 0 {
			schema = Obj("type", "object")
		}
		tools = append(tools, RemoteTool{
			Name:        Str(item, "name"),
			Description: Str(item, "description"),
			InputSchema: schema,
		})
	}
	return tools, nil
}

func (c *McpClient) CallTool(name string, arguments map[string]any) (string, error) {
	result, err := c.request("tools/call", Obj("name", name, "arguments", arguments))
	if err != nil {
		return "", err
	}
	out := []string{}
	for _, value := range AsArray(result["content"]) {
		block := AsObject(value)
		if Str(block, "type") == "text" {
			out = append(out, Str(block, "text"))
		}
	}
	text := strings.Join(out, "\n")
	if text == "" {
		text = Stringify(result)
	}
	if Bool(result, "isError", false) {
		return "Error: " + text, nil
	}
	return text, nil
}

// Register exposes every remote tool as mcp__<server>__<tool>.
func (c *McpClient) Register(registry *Registry, serverName string) (int, error) {
	tools, err := c.ListTools()
	if err != nil {
		return 0, err
	}
	for _, remote := range tools {
		name := "mcp__" + safeName(serverName) + "__" + safeName(remote.Name)
		remoteName := remote.Name
		description := remote.Description
		if description == "" {
			description = "MCP tool " + remote.Name
		}
		registry.Register(Tool{
			Name:        name,
			Description: description,
			InputSchema: remote.InputSchema,
			Handler: func(input map[string]any) (string, error) {
				return c.CallTool(remoteName, input)
			},
		})
	}
	return len(tools), nil
}

func (c *McpClient) Alive() bool { return c.cmd.ProcessState == nil }

// Close stops the server process (and its children) and reaps it so that
// ProcessState reflects the exit.
func (c *McpClient) Close() error {
	_ = c.stdin.Close()
	if c.cmd.Process != nil {
		killTree(c.cmd)
	}
	_ = c.cmd.Wait()
	return nil
}

// request is serialized so two concurrent callers cannot consume each other's
// responses through the shared reader channel.
func (c *McpClient) request(method string, params map[string]any) (map[string]any, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ids++
	id := c.ids
	request := Obj("jsonrpc", "2.0", "id", id, "method", method, "params", params)
	if _, err := c.stdin.Write([]byte(Stringify(request) + "\n")); err != nil {
		return nil, err
	}
	deadline := time.After(c.timeout)
	for {
		select {
		case line, ok := <-c.lines:
			if !ok {
				return nil, fmt.Errorf("MCP server closed stdout")
			}
			var response map[string]any
			if err := json.Unmarshal([]byte(line), &response); err != nil {
				continue
			}
			if Int(response, "id", -1) != id {
				// Notification or a stale response; keep waiting for ours.
				continue
			}
			if errValue, ok := response["error"]; ok {
				message := JsonError(errValue)
				return nil, fmt.Errorf("MCP error: %s", message)
			}
			return AsObject(response["result"]), nil
		case <-deadline:
			return nil, fmt.Errorf("MCP request timed out: %s", method)
		}
	}
}

func (c *McpClient) notify(method string, params map[string]any) error {
	request := Obj("jsonrpc", "2.0", "method", method, "params", params)
	_, err := c.stdin.Write([]byte(Stringify(request) + "\n"))
	return err
}

func JsonError(value any) string {
	item := AsObject(value)
	if message := Str(item, "message"); message != "" {
		return message
	}
	return Stringify(value)
}

func safeName(text string) string {
	return strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || r == '_' || r == '-' {
			return r
		}
		return '_'
	}, text)
}
