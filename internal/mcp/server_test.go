package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bg-dao/axon-codex-dramaops/internal/approval"
	"github.com/bg-dao/axon-codex-dramaops/internal/domain"
	"github.com/bg-dao/axon-codex-dramaops/internal/project"
)

func TestToolsAreExactlyThePublicContract(t *testing.T) {
	tools := (&Handler{}).Tools()
	want := []string{ToolProjectRead, ToolScriptApply, ToolShotPlanApply, ToolImageGenerate, ToolVideoGenerate, ToolSpeechGenerate, ToolJobStatus, ToolJobCancel}
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
	input := bytes.NewBufferString(
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}` + "\n" +
			`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}` + "\n" +
			`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"dramaops_project_read","arguments":{}}}` + "\n",
	)
	var output bytes.Buffer
	if err := (&Server{Handler: &Handler{Root: root, Store: store, Approval: approval.AutoGate{Approved: true}}}).Serve(context.Background(), input, &output); err != nil {
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
				Project  domain.Project   `json:"project"`
				Episodes []domain.Episode `json:"episodes"`
			} `json:"structuredContent"`
		} `json:"result"`
	}
	if err := json.Unmarshal(lines[2], &callResult); err != nil {
		t.Fatal(err)
	}
	if callResult.Result.IsError || callResult.Result.StructuredContent.Project.Name != "MCP Test" || len(callResult.Result.StructuredContent.Episodes) != 1 {
		t.Fatalf("unexpected project read: %s", lines[2])
	}
}

func TestScriptAndShotPlanApplyOnceWithStrictArguments(t *testing.T) {
	root := filepath.Join(t.TempDir(), "project")
	store := project.NewStore()
	if _, err := store.Create(root, "MCP Drama"); err != nil {
		t.Fatal(err)
	}
	handler := &Handler{Root: root, Store: store, Approval: approval.AutoGate{Approved: true}}
	script := json.RawMessage(`{
		"episode":{"id":"episode-001","title":"The Call","scriptBlocks":[{"id":"block-1","sceneId":"scene-1","kind":"dialogue","characterId":"character-1","text":"Answer me."}]},
		"scenes":[{"id":"scene-1","title":"INT. ROOM - NIGHT"}],
		"characters":[{"id":"character-1","name":"Lin","voiceProfile":{"id":"voice-1","kind":"built_in","name":"Lin","builtInVoice":"alloy"}}]
	}`)
	if _, err := handler.Call(context.Background(), ToolScriptApply, script); err != nil {
		t.Fatal(err)
	}
	if _, err := handler.Call(context.Background(), ToolScriptApply, script); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("expected repeated script apply to fail, got %v", err)
	}
	shots := json.RawMessage(`{"episodeId":"episode-001","shots":[{"id":"shot-1","sceneId":"scene-1","title":"Reveal","prompt":"Lin answers the phone","shotSize":"CU","cameraAngle":"eye_level","cameraMovement":"dolly"}]}`)
	if _, err := handler.Call(context.Background(), ToolShotPlanApply, shots); err != nil {
		t.Fatal(err)
	}
	if _, err := handler.Call(context.Background(), ToolShotPlanApply, shots); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("expected repeated shot plan apply to fail, got %v", err)
	}
	if _, err := handler.Call(context.Background(), ToolJobStatus, json.RawMessage(`{"runId":"x","unexpected":true}`)); err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("expected unknown field rejection, got %v", err)
	}
	snapshot, _ := store.Open(root)
	if len(snapshot.Scenes) != 1 || len(snapshot.Shots) != 1 || len(snapshot.Characters) != 1 {
		t.Fatalf("unexpected snapshot: %+v", snapshot)
	}
}

func TestWriteApprovalDeclineLeavesEpisodeEmpty(t *testing.T) {
	root := filepath.Join(t.TempDir(), "project")
	store := project.NewStore()
	if _, err := store.Create(root, "Decline"); err != nil {
		t.Fatal(err)
	}
	handler := &Handler{Root: root, Store: store, Approval: approval.AutoGate{Approved: false}}
	input := json.RawMessage(`{"episode":{"id":"episode-001","title":"Declined","scriptBlocks":[{"sceneId":"scene-1","kind":"action","text":"No write."}]},"scenes":[{"id":"scene-1","title":"ROOM"}]}`)
	if _, err := handler.Call(context.Background(), ToolScriptApply, input); err == nil {
		t.Fatal("declined script write must fail")
	}
	snapshot, _ := store.Open(root)
	if len(snapshot.Scenes) != 0 || len(snapshot.Episodes[0].ScriptBlocks) != 0 {
		t.Fatalf("declined write mutated project: %+v", snapshot)
	}
}
