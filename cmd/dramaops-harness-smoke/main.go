package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/bg-dao/axon-codex-dramaops/internal/appserver"
	codexruntime "github.com/bg-dao/axon-codex-dramaops/internal/runtime"
)

func main() {
	projectRoot := flag.String("project", "", "absolute DramaOps project root")
	mcpCommand := flag.String("mcp-command", "", "DramaOps desktop or dramaops-mcp executable")
	prompt := flag.String("prompt", "", "optional no-side-effect turn prompt")
	useSystemCodex := flag.Bool("use-system-codex", false, "use the detected system Codex instead of installing the pinned runtime")
	legacySandboxWire := flag.Bool("legacy-sandbox-wire", false, "use prerelease kebab-case sandbox wire values")
	flag.Parse()
	if *projectRoot == "" || *mcpCommand == "" {
		fmt.Fprintln(os.Stderr, "--project and --mcp-command are required")
		os.Exit(2)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	codexPath := ""
	var err error
	if *useSystemCodex {
		codexPath, err = exec.LookPath("codex")
	} else {
		runtimeStatus, ensureErr := (&codexruntime.Manager{}).Ensure(ctx)
		if ensureErr != nil {
			fatal(ensureErr)
		}
		codexPath = runtimeStatus.Path
	}
	threadSandboxWire := "workspaceWrite"
	approvalWire := "onRequest"
	if *legacySandboxWire {
		threadSandboxWire = "workspace-write"
		approvalWire = "on-request"
	}
	events := make(chan appserver.Event, 256)
	var client *appserver.Client
	client, err = appserver.StartClient(ctx, codexPath, appserver.AppServerArgs(*mcpCommand, *projectRoot), func(event appserver.Event) {
		events <- event
		if event.RequestID != "" && client != nil {
			_ = client.RespondServerRequest(event.RequestID, "decline")
		}
	})
	if err != nil {
		fatal(err)
	}
	defer client.Close()
	var account map[string]any
	if err := client.Request(ctx, "account/read", map[string]any{"refreshToken": false}, &account); err != nil {
		fatal(err)
	}
	fmt.Printf("account: %v\n", account["account"] != nil || account["authMode"] != nil)
	var thread struct {
		Thread struct {
			ID string `json:"id"`
		} `json:"thread"`
	}
	if err := client.Request(ctx, "thread/start", map[string]any{"cwd": *projectRoot, "approvalPolicy": approvalWire, "sandbox": threadSandboxWire, "serviceName": "dramaops_smoke"}, &thread); err != nil {
		fatal(err)
	}
	if thread.Thread.ID == "" {
		fatal(fmt.Errorf("thread/start returned no id"))
	}
	fmt.Println("thread: started")
	if strings.TrimSpace(*prompt) == "" {
		return
	}
	var turn struct {
		Turn struct {
			ID string `json:"id"`
		} `json:"turn"`
	}
	if err := client.Request(ctx, "turn/start", map[string]any{
		"threadId":       thread.Thread.ID,
		"input":          []map[string]any{{"type": "text", "text": *prompt}},
		"cwd":            *projectRoot,
		"approvalPolicy": approvalWire,
		"sandboxPolicy":  map[string]any{"type": threadSandboxWire, "writableRoots": []string{*projectRoot}, "networkAccess": false},
	}, &turn); err != nil {
		fatal(err)
	}
	var text strings.Builder
	for {
		select {
		case <-ctx.Done():
			fatal(ctx.Err())
		case event := <-events:
			switch event.Method {
			case "item/agentMessage/delta":
				var delta struct {
					Delta string `json:"delta"`
				}
				_ = json.Unmarshal(event.Params, &delta)
				text.WriteString(delta.Delta)
			case "turn/completed":
				fmt.Printf("turn: completed (%d streamed characters)\n", text.Len())
				return
			case "error":
				var retry struct {
					WillRetry bool `json:"willRetry"`
				}
				_ = json.Unmarshal(event.Params, &retry)
				if !retry.WillRetry {
					fatal(fmt.Errorf("app-server event %s: %s", event.Method, event.Params))
				}
			case "dramaops/runtime/failed":
				fatal(fmt.Errorf("app-server event %s: %s", event.Method, event.Params))
			}
		}
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
