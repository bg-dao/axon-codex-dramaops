package appserver

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bg-dao/axon-codex-dramaops/internal/redact"
)

type Event struct {
	Method    string          `json:"method"`
	Params    json.RawMessage `json:"params,omitempty"`
	RequestID string          `json:"requestId,omitempty"`
	Timestamp time.Time       `json:"timestamp"`
}

type RPCError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func (e *RPCError) Error() string {
	return fmt.Sprintf("app-server error %d: %s", e.Code, redact.String(e.Message))
}

type rpcResponse struct {
	ID     json.RawMessage `json:"id"`
	Result json.RawMessage `json:"result"`
	Error  *RPCError       `json:"error"`
}

type pendingResult struct {
	result json.RawMessage
	err    error
}

type Client struct {
	writeMu       sync.Mutex
	stateMu       sync.Mutex
	stdin         io.WriteCloser
	cmd           *exec.Cmd
	pending       map[string]chan pendingResult
	serverRequest map[string]json.RawMessage
	nextID        atomic.Uint64
	done          chan struct{}
	doneOnce      sync.Once
	err           error
	onEvent       func(Event)
}

func StartClient(ctx context.Context, executable string, args []string, onEvent func(Event)) (*Client, error) {
	if executable == "" {
		return nil, errors.New("Codex executable is required")
	}
	command := exec.CommandContext(ctx, executable, args...)
	return startClientCommand(ctx, command, onEvent)
}

func startClientCommand(ctx context.Context, command *exec.Cmd, onEvent func(Event)) (*Client, error) {
	stdin, err := command.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := command.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr, err := command.StderrPipe()
	if err != nil {
		return nil, err
	}
	client := &Client{
		stdin:         stdin,
		cmd:           command,
		pending:       make(map[string]chan pendingResult),
		serverRequest: make(map[string]json.RawMessage),
		done:          make(chan struct{}),
		onEvent:       onEvent,
	}
	if err := command.Start(); err != nil {
		return nil, fmt.Errorf("start Codex app-server: %w", err)
	}
	go client.readLoop(stdout)
	go client.stderrLoop(stderr)
	go func() {
		waitErr := command.Wait()
		client.finish(waitErr)
	}()
	initCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	var initialized map[string]any
	if err := client.Request(initCtx, "initialize", map[string]any{
		"clientInfo": map[string]any{"name": "dramaops", "title": "DramaOps by Axon", "version": "0.2.0"},
	}, &initialized); err != nil {
		_ = client.Close()
		return nil, err
	}
	if err := client.Notify("initialized", map[string]any{}); err != nil {
		_ = client.Close()
		return nil, err
	}
	return client, nil
}

func (c *Client) Request(ctx context.Context, method string, params any, output any) error {
	id := c.nextID.Add(1)
	idBytes := json.RawMessage(fmt.Sprintf("%d", id))
	key := string(idBytes)
	resultChannel := make(chan pendingResult, 1)
	c.stateMu.Lock()
	select {
	case <-c.done:
		err := c.err
		c.stateMu.Unlock()
		if err == nil {
			err = errors.New("app-server is closed")
		}
		return err
	default:
		c.pending[key] = resultChannel
	}
	c.stateMu.Unlock()
	message := map[string]any{"id": id, "method": method, "params": params}
	if err := c.write(message); err != nil {
		c.removePending(key)
		return err
	}
	select {
	case <-ctx.Done():
		c.removePending(key)
		return ctx.Err()
	case result := <-resultChannel:
		if result.err != nil {
			return result.err
		}
		if output == nil || len(result.result) == 0 || string(result.result) == "null" {
			return nil
		}
		if err := json.Unmarshal(result.result, output); err != nil {
			return fmt.Errorf("decode %s response: %w", method, err)
		}
		return nil
	}
}

func (c *Client) Notify(method string, params any) error {
	return c.write(map[string]any{"method": method, "params": params})
}

func (c *Client) RespondServerRequest(requestID, decision string) error {
	if decision != "accept" && decision != "decline" && decision != "cancel" {
		return fmt.Errorf("unsupported approval decision %q", decision)
	}
	c.stateMu.Lock()
	rawID, ok := c.serverRequest[requestID]
	if ok {
		delete(c.serverRequest, requestID)
	}
	c.stateMu.Unlock()
	if !ok {
		return fmt.Errorf("app-server request %s is not pending", requestID)
	}
	return c.write(map[string]any{"id": rawID, "result": map[string]any{"decision": decision}})
}

func (c *Client) Done() <-chan struct{} { return c.done }

func (c *Client) Err() error {
	c.stateMu.Lock()
	defer c.stateMu.Unlock()
	return c.err
}

func (c *Client) Close() error {
	c.stateMu.Lock()
	stdin := c.stdin
	process := c.cmd.Process
	c.stateMu.Unlock()
	if stdin != nil {
		_ = stdin.Close()
	}
	select {
	case <-c.done:
		return nil
	case <-time.After(750 * time.Millisecond):
		if process != nil {
			_ = process.Kill()
		}
	}
	select {
	case <-c.done:
		return nil
	case <-time.After(2 * time.Second):
		return errors.New("timed out stopping Codex app-server")
	}
}

func (c *Client) readLoop(reader io.Reader) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64*1024), 32*1024*1024)
	for scanner.Scan() {
		line := append([]byte(nil), scanner.Bytes()...)
		var envelope struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
			Result json.RawMessage `json:"result"`
			Error  *RPCError       `json:"error"`
		}
		if err := json.Unmarshal(line, &envelope); err != nil {
			c.emit(Event{Method: "dramaops/protocol/error", Params: mustJSON(map[string]any{"message": "invalid JSON from app-server"}), Timestamp: time.Now().UTC()})
			continue
		}
		if len(envelope.ID) > 0 && envelope.Method == "" {
			key := string(envelope.ID)
			c.stateMu.Lock()
			channel := c.pending[key]
			delete(c.pending, key)
			c.stateMu.Unlock()
			if channel != nil {
				if envelope.Error != nil {
					channel <- pendingResult{err: envelope.Error}
				} else {
					channel <- pendingResult{result: envelope.Result}
				}
			}
			continue
		}
		if len(envelope.ID) > 0 && envelope.Method != "" {
			key := string(envelope.ID)
			c.stateMu.Lock()
			c.serverRequest[key] = append(json.RawMessage(nil), envelope.ID...)
			c.stateMu.Unlock()
			c.emit(Event{Method: envelope.Method, Params: envelope.Params, RequestID: key, Timestamp: time.Now().UTC()})
			continue
		}
		if envelope.Method != "" {
			c.emit(Event{Method: envelope.Method, Params: envelope.Params, Timestamp: time.Now().UTC()})
		}
	}
	if err := scanner.Err(); err != nil {
		c.finish(err)
	}
}

func (c *Client) stderrLoop(reader io.Reader) {
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		message := redact.String(scanner.Text())
		c.emit(Event{Method: "dramaops/runtime/stderr", Params: mustJSON(map[string]any{"message": message}), Timestamp: time.Now().UTC()})
	}
}

func (c *Client) write(value any) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	_, err = c.stdin.Write(data)
	if err != nil {
		return fmt.Errorf("write to app-server: %w", err)
	}
	return nil
}

func (c *Client) finish(err error) {
	c.doneOnce.Do(func() {
		c.stateMu.Lock()
		c.err = err
		for key, channel := range c.pending {
			delete(c.pending, key)
			channel <- pendingResult{err: errors.New("app-server exited")}
		}
		c.stateMu.Unlock()
		close(c.done)
	})
}

func (c *Client) removePending(key string) {
	c.stateMu.Lock()
	delete(c.pending, key)
	c.stateMu.Unlock()
}

func (c *Client) emit(event Event) {
	if c.onEvent != nil {
		c.onEvent(event)
	}
}

func mustJSON(value any) json.RawMessage {
	data, _ := json.Marshal(value)
	return data
}
