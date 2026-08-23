package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/bg-dao/axon-codex-sceneops/internal/approval"
	"github.com/bg-dao/axon-codex-sceneops/internal/domain"
	"github.com/bg-dao/axon-codex-sceneops/internal/project"
)

func TestToolsAreExactlyThePublicContract(t *testing.T) {
	handler := &Handler{}
	tools := handler.Tools()
	want := []string{ToolProjectRead, ToolStoryboardApply, ToolImageGenerate, ToolVideoGenerate, ToolJobStatus, ToolJobCancel}
	if len(tools) != len(want) {
		t.Fatalf("tool count = %d", len(tools))
	}
	for index, name := range want {
		if tools[index].Name != name {
			t.Fatalf("tool %d = %s, want %s", index, tools[index].Name, name)
		}
	}
}

func TestServerInitializeListAndProjectRead(t *testing.T) {
	root := filepath.Join(t.TempDir(), "project")
	store := project.NewStore()
	if _, err := store.Create(root, "MCP Test"); err != nil {
		t.Fatal(err)
	}
	handler := &Handler{Root: root, Store: store, Approval: approval.AutoGate{Approved: true}}
	input := bytes.NewBufferString(
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}` + "\n" +
			`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}` + "\n" +
			`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"sceneops_project_read","arguments":{}}}` + "\n",
	)
	var output bytes.Buffer
	if err := (&Server{Handler: handler}).Serve(context.Background(), input, &output); err != nil {
		t.Fatal(err)
	}
	lines := bytes.Split(bytes.TrimSpace(output.Bytes()), []byte("\n"))
	if len(lines) != 3 {
		t.Fatalf("response lines = %d: %s", len(lines), output.String())
	}
	var callResult struct {
		Result struct {
			IsError           bool `json:"isError"`
			StructuredContent struct {
				Project domain.Project `json:"project"`
			} `json:"structuredContent"`
		} `json:"result"`
	}
	if err := json.Unmarshal(lines[2], &callResult); err != nil {
		t.Fatal(err)
	}
	if callResult.Result.IsError || callResult.Result.StructuredContent.Project.Name != "MCP Test" {
		t.Fatalf("unexpected project read: %s", lines[2])
	}
}

func TestStoryboardToolIsIdempotentForStableIDs(t *testing.T) {
	root := filepath.Join(t.TempDir(), "project")
	store := project.NewStore()
	if _, err := store.Create(root, "MCP Storyboard"); err != nil {
		t.Fatal(err)
	}
	handler := &Handler{Root: root, Store: store, Approval: approval.AutoGate{Approved: true}}
	args := json.RawMessage(`{"scenes":[{"id":"scene-1","title":"Opening"}],"shots":[{"id":"shot-1","sceneId":"scene-1","title":"Wide"}]}`)
	if _, err := handler.Call(context.Background(), ToolStoryboardApply, args); err != nil {
		t.Fatal(err)
	}
	if _, err := handler.Call(context.Background(), ToolStoryboardApply, args); err != nil {
		t.Fatal(err)
	}
	snapshot, _ := store.Open(root)
	if len(snapshot.Scenes) != 1 || len(snapshot.Shots) != 1 {
		t.Fatalf("repeated apply duplicated manifests: %+v", snapshot)
	}
}
