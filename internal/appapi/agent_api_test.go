package appapi

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/bg-dao/axon-codex-dramaops/internal/domain"
	"github.com/bg-dao/axon-codex-dramaops/internal/project"
)

func TestAgentGenerationGuardsProjectStateBeforeRuntime(t *testing.T) {
	root := filepath.Join(t.TempDir(), "project")
	store := project.NewStore()
	if _, err := store.Create(root, "Agent Guard"); err != nil {
		t.Fatal(err)
	}
	api := NewAgentAPI(&Backend{store: store, root: root})
	if _, err := api.GenerateScript("episode-001"); err == nil || !strings.Contains(err.Error(), "logline or synopsis") {
		t.Fatalf("expected missing story premise guard, got %v", err)
	}
	snapshot, _ := store.Open(root)
	episode := snapshot.Episodes[0]
	episode.Logline = "A courier answers a phone that predicts the next minute."
	if err := store.SaveEpisode(root, episode); err != nil {
		t.Fatal(err)
	}
	plan := project.ScriptPlan{
		Episode: domain.Episode{ID: "episode-001", Title: "The Call", ScriptBlocks: []domain.ScriptBlock{{ID: "block-1", SceneID: "scene-1", Kind: domain.ScriptAction, Text: "The phone rings."}}},
		Scenes:  []domain.Scene{{ID: "scene-1", Title: "ROOM"}},
	}
	if _, err := store.ApplyScript(root, plan); err != nil {
		t.Fatal(err)
	}
	if _, err := api.GenerateScript("episode-001"); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("expected existing script guard, got %v", err)
	}
	if _, err := store.ApplyShotPlan(root, "episode-001", []domain.Shot{{ID: "shot-1", SceneID: "scene-1", Title: "Phone", Prompt: "phone rings"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := api.GenerateShotPlan("episode-001"); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("expected existing shot plan guard, got %v", err)
	}
}
