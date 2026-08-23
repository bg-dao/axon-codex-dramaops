package appapi

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bg-dao/axon-codex-sceneops/internal/domain"
	"github.com/bg-dao/axon-codex-sceneops/internal/project"
)

func TestAssetDataURLVerifiesHash(t *testing.T) {
	root := filepath.Join(t.TempDir(), "project")
	store := project.NewStore()
	if _, err := store.Create(root, "Preview"); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(t.TempDir(), "frame.png")
	if err := os.WriteFile(source, []byte("png-bytes"), 0o600); err != nil {
		t.Fatal(err)
	}
	asset, err := store.ImportAsset(root, source, "", domain.AssetKindImage)
	if err != nil {
		t.Fatal(err)
	}
	api := NewAssetAPI(&Backend{store: store, root: root})
	dataURL, err := api.DataURL(asset.ID)
	if err != nil || !strings.HasPrefix(dataURL, "data:image/png;base64,") {
		t.Fatalf("unexpected preview: %q %v", dataURL, err)
	}
	path, err := project.ResolveRelative(root, asset.RelativePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("tampered"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := api.DataURL(asset.ID); err == nil || !strings.Contains(err.Error(), "SHA-256") {
		t.Fatalf("expected checksum failure, got %v", err)
	}
}
