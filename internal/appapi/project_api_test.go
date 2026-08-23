package appapi

import (
	"path/filepath"
	"testing"

	"github.com/bg-dao/axon-codex-dramaops/internal/domain"
	"github.com/bg-dao/axon-codex-dramaops/internal/project"
)

func TestSaveSettingsDoesNotOverwriteRuntimeOrSeriesBibleState(t *testing.T) {
	root := filepath.Join(t.TempDir(), "project")
	store := project.NewStore()
	snapshot, err := store.Create(root, "Settings")
	if err != nil {
		t.Fatal(err)
	}
	snapshot.Project.ActiveThreadID = "thread-current"
	snapshot.Project.StyleBible.VisualStyle = "locked style"
	if err := store.SaveProject(root, snapshot.Project); err != nil {
		t.Fatal(err)
	}

	stale := snapshot.Project
	stale.ActiveThreadID = ""
	stale.StyleBible.VisualStyle = "stale style"
	stale.ContentLanguage = "en"
	stale.Output = domain.DefaultOutputSettings(domain.OrientationLandscape)
	api := NewProjectAPI(&Backend{store: store, root: root})
	updated, err := api.SaveSettings(stale)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Project.ActiveThreadID != "thread-current" || updated.Project.StyleBible.VisualStyle != "locked style" {
		t.Fatalf("settings overwrote unrelated state: %+v", updated.Project)
	}
	if updated.Project.ContentLanguage != "en" || updated.Project.Output.Orientation != domain.OrientationLandscape || updated.Edits[0].Output.Orientation != domain.OrientationLandscape {
		t.Fatalf("settings were not propagated: %+v", updated.Project)
	}
}
