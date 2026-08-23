package appserver

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestClientCorrelatesResponsesStreamsEventsAndApproves(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	command := exec.CommandContext(ctx, os.Args[0], "-test.run=TestAppServerHelperProcess")
	command.Env = append(os.Environ(), "DRAMAOPS_APP_SERVER_HELPER=1")
	events := make(chan Event, 20)
	client, err := startClientCommand(ctx, command, func(event Event) { events <- event })
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	var account map[string]any
	if err := client.Request(ctx, "account/read", map[string]any{}, &account); err != nil {
		t.Fatal(err)
	}
	if account["authMode"] != "chatgpt" {
		t.Fatalf("account = %+v", account)
	}
	var turn map[string]any
	if err := client.Request(ctx, "turn/start", map[string]any{}, &turn); err != nil {
		t.Fatal(err)
	}
	deadline := time.After(2 * time.Second)
	approved := false
	redacted := false
	for !approved || !redacted {
		select {
		case event := <-events:
			if event.Method == "item/commandExecution/requestApproval" {
				if event.RequestID == "" {
					t.Fatal("server request did not expose a correlation id")
				}
				if err := client.RespondServerRequest(event.RequestID, "accept"); err != nil {
					t.Fatal(err)
				}
				approved = true
			}
			if event.Method == "dramaops/runtime/stderr" {
				if strings.Contains(string(event.Params), "sk-helpersecret") {
					t.Fatalf("stderr leaked a secret: %s", event.Params)
				}
				redacted = true
			}
		case <-deadline:
			t.Fatalf("timed out waiting for approval/redaction events: approved=%v redacted=%v", approved, redacted)
		}
	}
	if err := client.RespondServerRequest("missing", "acceptForSession"); err == nil {
		t.Fatal("session-wide approval must not be accepted")
	}
}

func TestClientContextCancellationRemovesPendingRequest(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	command := exec.CommandContext(ctx, os.Args[0], "-test.run=TestAppServerHelperProcess")
	command.Env = append(os.Environ(), "DRAMAOPS_APP_SERVER_HELPER=1")
	client, err := startClientCommand(ctx, command, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	requestCtx, requestCancel := context.WithTimeout(ctx, 20*time.Millisecond)
	defer requestCancel()
	if err := client.Request(requestCtx, "test/no-response", map[string]any{}, nil); err == nil {
		t.Fatal("expected request cancellation")
	}
	client.stateMu.Lock()
	pending := len(client.pending)
	client.stateMu.Unlock()
	if pending != 0 {
		t.Fatalf("pending request leaked after cancellation: %d", pending)
	}
}

func TestAppServerHelperProcess(t *testing.T) {
	if os.Getenv("DRAMAOPS_APP_SERVER_HELPER") != "1" {
		return
	}
	fmt.Fprintln(os.Stderr, "api_key=sk-helpersecret")
	initialized := false
	scanner := bufio.NewScanner(os.Stdin)
	encoder := json.NewEncoder(os.Stdout)
	for scanner.Scan() {
		var message struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
		}
		_ = json.Unmarshal(scanner.Bytes(), &message)
		if message.Method == "initialize" {
			initialized = true
			_ = encoder.Encode(map[string]any{"id": message.ID, "result": map[string]any{"userAgent": "helper"}})
			continue
		}
		if message.Method == "initialized" {
			continue
		}
		if !initialized {
			_ = encoder.Encode(map[string]any{"id": message.ID, "error": map[string]any{"code": -32000, "message": "Not initialized"}})
			continue
		}
		switch message.Method {
		case "account/read":
			_ = encoder.Encode(map[string]any{"method": "account/updated", "params": map[string]any{"authMode": "chatgpt"}})
			_ = encoder.Encode(map[string]any{"id": message.ID, "result": map[string]any{"authMode": "chatgpt"}})
		case "turn/start":
			_ = encoder.Encode(map[string]any{"id": 9001, "method": "item/commandExecution/requestApproval", "params": map[string]any{"threadId": "thr_1", "turnId": "turn_1", "itemId": "item_1"}})
			_ = encoder.Encode(map[string]any{"id": message.ID, "result": map[string]any{"turn": map[string]any{"id": "turn_1", "status": "inProgress"}}})
		case "test/no-response":
		default:
			if len(message.ID) > 0 {
				_ = encoder.Encode(map[string]any{"id": message.ID, "result": map[string]any{}})
			}
		}
	}
	os.Exit(0)
}
