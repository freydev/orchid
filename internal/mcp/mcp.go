// Package mcp implements a minimal stdio MCP (Model Context Protocol) server
// exposing a single tool, set_status, that forwards the agent's reported state
// to a running orchid server. JSON-RPC 2.0, one message per line.
package mcp

import (
	"bufio"
	"encoding/json"
	"io"
	"os"

	"orchid/internal/statusclient"
)

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  interface{}     `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

const setStatusTool = `{
  "name": "set_status",
  "description": "Report this agent session's status to the orchid dashboard. Call with state=\"requires_action\" when you end your turn asking the user a question or otherwise need their input; state=\"running\" when actively working; state=\"idle\" when finished and nothing is needed.",
  "inputSchema": {
    "type": "object",
    "properties": {
      "state": {"type": "string", "enum": ["running", "idle", "requires_action"]},
      "message": {"type": "string", "description": "Optional short note about what you need."}
    },
    "required": ["state"]
  }
}`

// Serve reads JSON-RPC requests from in and writes responses to out until EOF.
func Serve(in io.Reader, out io.Writer) error {
	enc := json.NewEncoder(out)
	sc := bufio.NewScanner(in)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var req rpcRequest
		if err := json.Unmarshal(line, &req); err != nil {
			continue
		}
		resp := handle(req)
		if resp != nil {
			if err := enc.Encode(resp); err != nil {
				return err
			}
		}
	}
	return sc.Err()
}

func handle(req rpcRequest) *rpcResponse {
	switch req.Method {
	case "initialize":
		return &rpcResponse{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]any{"name": "orchid", "version": "1.0.0"},
		}}
	case "notifications/initialized":
		return nil
	case "tools/list":
		var tool any
		_ = json.Unmarshal([]byte(setStatusTool), &tool)
		return &rpcResponse{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{
			"tools": []any{tool},
		}}
	case "tools/call":
		return callTool(req)
	default:
		if len(req.ID) == 0 {
			return nil
		}
		return &rpcResponse{JSONRPC: "2.0", ID: req.ID, Error: &rpcError{Code: -32601, Message: "method not found"}}
	}
}

func callTool(req rpcRequest) *rpcResponse {
	var p struct {
		Name      string `json:"name"`
		Arguments struct {
			State   string `json:"state"`
			Message string `json:"message"`
		} `json:"arguments"`
	}
	_ = json.Unmarshal(req.Params, &p)
	text := "status not updated"
	if p.Name == "set_status" {
		switch p.Arguments.State {
		case "running", "idle", "requires_action":
			_ = statusclient.Post(os.Getenv("ORCHID_URL"), os.Getenv("ORCHID_SESSION"), p.Arguments.State)
			text = "status set to " + p.Arguments.State
		}
	}
	return &rpcResponse{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{
		"content": []any{map[string]any{"type": "text", "text": text}},
	}}
}
