package approval

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/bg-dao/axon-codex-dramaops/internal/project"
)

func TestFileGateRequiresFreshDecision(t *testing.T) {
	root := filepath.Join(t.TempDir(), "project")
	if _, err := project.NewStore().Create(root, "Approval Test"); err != nil {
		t.Fatal(err)
	}
	gate := NewFileGate(root)
	gate.PollInterval = 5 * time.Millisecond
	result := make(chan error, 1)
	go func() {
		_, err := gate.Request(context.Background(), ImageGenerate, "Generate image", map[string]any{"shotId": "shot-1"})
		result <- err
	}()
	var pending []Request
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		pending, _ = gate.Pending()
		if len(pending) == 1 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if len(pending) != 1 {
		t.Fatal("approval request did not become pending")
	}
	if _, err := gate.Resolve(pending[0].ID, true); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-result:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("approved operation did not resume")
	}
	if remaining, _ := gate.Pending(); len(remaining) != 0 {
		t.Fatalf("resolved approval remained pending: %v", remaining)
	}
}
