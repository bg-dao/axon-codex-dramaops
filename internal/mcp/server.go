package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"
)

const ProtocolVersion = "2024-11-05"

type Tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type Server struct {
	Handler *Handler
	mu      sync.Mutex
}

func (s *Server) Serve(ctx context.Context, input io.Reader, output io.Writer) error {
	if s.Handler == nil {
		return fmt.Errorf("MCP handler is required")
	}
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 64*1024), 16*1024*1024)
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		line := append([]byte(nil), scanner.Bytes()...)
		var message request
		if err := json.Unmarshal(line, &message); err != nil {
			_ = s.write(output, response{JSONRPC: "2.0", Error: &rpcError{Code: -32700, Message: "parse error"}})
			continue
		}
		if len(message.ID) == 0 {
			continue
		}
		result, rpcErr := s.handle(ctx, message)
		if err := s.write(output, response{JSONRPC: "2.0", ID: message.ID, Result: result, Error: rpcErr}); err != nil {
			return err
		}
	}
	return scanner.Err()
}

func (s *Server) handle(ctx context.Context, message request) (any, *rpcError) {
	switch message.Method {
	case "initialize":
		return map[string]any{
			"protocolVersion": ProtocolVersion,
			"capabilities":    map[string]any{"tools": map[string]any{"listChanged": false}},
			"serverInfo":      map[string]any{"name": "dramaops", "version": "0.2.0"},
		}, nil
	case "ping":
		return map[string]any{}, nil
	case "tools/list":
		return map[string]any{"tools": s.Handler.Tools()}, nil
	case "tools/call":
		var input struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if err := json.Unmarshal(message.Params, &input); err != nil {
			return nil, &rpcError{Code: -32602, Message: "invalid tool call parameters"}
		}
		value, err := s.Handler.Call(ctx, input.Name, input.Arguments)
		if err != nil {
			return map[string]any{"content": []map[string]any{{"type": "text", "text": err.Error()}}, "isError": true}, nil
		}
		encoded, err := json.Marshal(value)
		if err != nil {
			return nil, &rpcError{Code: -32603, Message: "encode tool result"}
		}
		return map[string]any{"content": []map[string]any{{"type": "text", "text": string(encoded)}}, "structuredContent": value, "isError": false}, nil
	default:
		return nil, &rpcError{Code: -32601, Message: "method not found"}
	}
}

func (s *Server) write(output io.Writer, value response) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	_, err = output.Write(data)
	return err
}
