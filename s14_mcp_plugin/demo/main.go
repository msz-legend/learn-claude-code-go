package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"learnclaude-go/internal/harness"
)

// s14 demo: a tiny stdio MCP server with no external dependencies.
//
// It speaks newline-delimited JSON-RPC 2.0 and exposes one tool, echo.
func main() {
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		parsed, err := harness.ParseJSON(line)
		if err != nil {
			continue
		}
		request := harness.AsObject(parsed)
		method := harness.Str(request, "method")
		id, hasID := request["id"]
		switch method {
		case "initialize":
			respond(id, hasID, harness.Obj(
				"protocolVersion", "2024-11-05",
				"capabilities", harness.Obj("tools", harness.Obj()),
				"serverInfo", harness.Obj("name", "demo", "version", "1.0")))
		case "notifications/initialized":
			// notification: no response
		case "tools/list":
			respond(id, hasID, harness.Obj("tools", harness.Arr(
				harness.Obj(
					"name", "echo",
					"description", "Echo the given text back.",
					"inputSchema", harness.Obj(
						"type", "object",
						"properties", harness.Obj("text", harness.Obj("type", "string")),
						"required", harness.Arr("text"))),
				harness.Obj(
					"name", "add",
					"description", "Add two integers.",
					"inputSchema", harness.Obj(
						"type", "object",
						"properties", harness.Obj(
							"a", harness.Obj("type", "integer"),
							"b", harness.Obj("type", "integer")),
						"required", harness.Arr("a", "b"))))))
		case "tools/call":
			params := harness.AsObject(request["params"])
			name := harness.Str(params, "name")
			arguments := harness.AsObject(params["arguments"])
			text := ""
			switch name {
			case "echo":
				text = harness.Str(arguments, "text")
			case "add":
				text = fmt.Sprintf("%d",
					harness.Int(arguments, "a", 0)+harness.Int(arguments, "b", 0))
			default:
				respond(id, hasID, harness.Obj("content", harness.Arr(
					harness.Obj("type", "text", "text", "unknown tool: "+name)),
					"isError", true))
				continue
			}
			respond(id, hasID, harness.Obj(
				"content", harness.Arr(harness.Obj("type", "text", "text", text))))
		default:
			if hasID {
				respond(id, true, harness.Obj(
					"error", harness.Obj("code", -32601, "message", "method not found")))
			}
		}
	}
}

func respond(id any, hasID bool, result map[string]any) {
	if !hasID {
		return
	}
	response := harness.Obj("jsonrpc", "2.0", "id", id)
	if _, isError := result["error"]; isError {
		response["error"] = result["error"]
	} else {
		response["result"] = result
	}
	fmt.Println(harness.Stringify(response))
}
