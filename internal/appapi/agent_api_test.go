package appapi

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/bg-dao/axon-codex-sceneops/internal/domain"
	"github.com/bg-dao/axon-codex-sceneops/internal/project"
)

func TestGenerateStoryboardGuardsBriefAndExistingStoryboardBeforeRuntime(t *testing.T) {
	root := filepath.Join(t.TempDir(), "project")
	store := project.NewStore()
	if _, err := store.Create(root, "Agent Guard"); err != nil {
		t.Fatal(err)
	}
	api := NewAgentAPI(&Backend{store: store, root: root})
	if _, err := api.GenerateStoryboard(); err == nil || !strings.Contains(err.Error(), "save a creative brief") {
		t.Fatalf("expected missing brief guard, got %v", err)
	}
	if err := store.SaveBrief(root, "# Creative brief\n\nA launch film.\n"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ApplyStoryboard(root, []domain.Scene{{ID: "scene-1", Title: "Scene"}}, []domain.Shot{{ID: "shot-1", SceneID: "scene-1", Title: "Shot"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := api.GenerateStoryboard(); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("expected existing storyboard guard, got %v", err)
	}
}
