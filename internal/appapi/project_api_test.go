package appapi

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/bg-dao/axon-codex-dramaops/internal/approval"
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

func TestSwitchingProjectsCancelsOldRendersButReopeningDoesNot(t *testing.T) {
	backend := NewBackend()
	backend.Startup(context.Background())
	defer backend.Shutdown(context.Background())
	firstRoot := filepath.Join(t.TempDir(), "first")
	secondRoot := filepath.Join(t.TempDir(), "second")
	if _, err := backend.store.Create(firstRoot, "First"); err != nil {
		t.Fatal(err)
	}
	if _, err := backend.store.Create(secondRoot, "Second"); err != nil {
		t.Fatal(err)
	}
	staleGate := approval.NewFileGate(firstRoot)
	interrupted, cancelApproval := context.WithCancel(context.Background())
	cancelApproval()
	if _, err := staleGate.Request(interrupted, approval.ImageGenerate, "Interrupted", map[string]any{"runId": "interrupted-run"}); err == nil {
		t.Fatal("cancelled approval request unexpectedly succeeded")
	}
	if pending, err := staleGate.Pending(); err != nil || len(pending) != 1 {
		t.Fatalf("stale approval setup = %v, err = %v", pending, err)
	}
	if err := backend.SetProject(firstRoot); err != nil {
		t.Fatal(err)
	}
	firstProjectContext := backend.projectContext()
	if pending, err := staleGate.Pending(); err != nil || len(pending) != 0 {
		t.Fatalf("stale approvals survived project recovery: %v, err = %v", pending, err)
	}
	renderContext, cancelRender := context.WithCancel(context.Background())
	backend.mu.Lock()
	backend.renderCancels["render-1"] = cancelRender
	backend.mu.Unlock()
	if err := backend.SetProject(firstRoot); err != nil {
		t.Fatal(err)
	}
	select {
	case <-renderContext.Done():
		t.Fatal("reopening the active project cancelled its render")
	default:
	}
	select {
	case <-firstProjectContext.Done():
		t.Fatal("reopening the active project cancelled its operations")
	default:
	}
	if err := backend.SetProject(secondRoot); err != nil {
		t.Fatal(err)
	}
	select {
	case <-renderContext.Done():
	default:
		t.Fatal("switching projects did not cancel the old render")
	}
	select {
	case <-firstProjectContext.Done():
	default:
		t.Fatal("switching projects did not cancel old provider operations")
	}
	backend.mu.RLock()
	defer backend.mu.RUnlock()
	if backend.root != secondRoot || len(backend.renderCancels) != 0 {
		t.Fatalf("backend retained old project render state: root=%s renders=%d", backend.root, len(backend.renderCancels))
	}
}
