package main

import (
	"fmt"
	"os"
	"path/filepath"

	"learnclaude-go/internal/harness"
)

// s14: MCP Plugin
//
// MCP tools are discovered at startup and exposed in the same registry as
// local tools, namespaced as mcp__<server>__<tool>.
//
// mcp.json:
//
//	{
//	  "servers": {
//	    "demo": {"command": ["go", "run", "./s14_mcp_plugin/demo"]}
//	  }
//	}
func main() {
	if len(os.Args) > 1 && (os.Args[1] == "--help" || os.Args[1] == "-h") {
		fmt.Println(`s14: MCP Plugin

Usage:
  go run ./s14_mcp_plugin

Reads server commands from mcp.json. If the file is absent, the
bundled demo server starts so the chapter works offline.`)
		return
	}

	harness.Header("s14", "MCP Plugin")

	workspace := harness.NewWorkspace(harness.Workdir())
	client := harness.FromEnv()
	tools := harness.Base(workspace, 120)

	servers := loadServers(workspace.Root())
	clients := []*harness.McpClient{}
	defer func() {
		for _, mcp := range clients {
			_ = mcp.Close()
		}
	}()

	for name, command := range servers {
		mcp, err := harness.StartMcp(command, workspace.Root())
		if err != nil {
			fmt.Println(harness.Color("[mcp] "+name+" failed: "+err.Error(), harness.Red))
			continue
		}
		if _, err := mcp.Initialize(); err != nil {
			fmt.Println(harness.Color("[mcp] "+name+" failed: "+err.Error(), harness.Red))
			_ = mcp.Close()
			continue
		}
		count, err := mcp.Register(tools, name)
		if err != nil {
			fmt.Println(harness.Color("[mcp] "+name+" failed: "+err.Error(), harness.Red))
			_ = mcp.Close()
			continue
		}
		clients = append(clients, mcp)
		fmt.Println(harness.Color(
			fmt.Sprintf("[mcp] %s: %d tools", name, count), harness.Dim))
	}

	system := harness.SystemPrompt("a coding agent", workspace,
		"MCP tools are named mcp__<server>__<tool>. "+
			"Use them like local tools; their schemas are authoritative.")

	loop := harness.NewAgentLoop(client, tools, system).
		Listener(harness.PrintingListener())

	history := []harness.Msg{}
	harness.REPL("s14", loop, &history, func(query string) bool {
		if query != "/mcp" {
			return false
		}
		for _, name := range tools.Names() {
			if len(name) >= 5 && name[:5] == "mcp__" {
				fmt.Println("- " + name)
			}
		}
		return true
	})
}

func loadServers(workspace string) map[string][]string {
	servers := map[string][]string{}
	file := filepath.Join(workspace, "mcp.json")
	if data, err := os.ReadFile(file); err == nil {
		if parsed, err := harness.ParseJSON(string(data)); err == nil {
			root := harness.AsObject(parsed)
			for name, value := range harness.AsObject(root["servers"]) {
				command := []string{}
				for _, part := range harness.AsArray(harness.AsObject(value)["command"]) {
					command = append(command, fmt.Sprint(part))
				}
				if len(command) > 0 {
					servers[name] = command
				}
			}
		}
	}
	if len(servers) == 0 {
		servers["demo"] = []string{"go", "run", "./s14_mcp_plugin/demo"}
	}
	return servers
}
