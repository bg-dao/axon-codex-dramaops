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

func TestSelectVersionAcceptsOnlyGeneratedImages(t *testing.T) {
	root := filepath.Join(t.TempDir(), "project")
	store := project.NewStore()
	if _, err := store.Create(root, "Versions"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ApplyStoryboard(root, []domain.Scene{{ID: "scene-1", Title: "Scene"}}, []domain.Shot{{ID: "shot-1", SceneID: "scene-1", Title: "Shot"}}); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(t.TempDir(), "frame.png")
	if err := os.WriteFile(source, []byte("png-bytes"), 0o600); err != nil {
		t.Fatal(err)
	}
	image, err := store.ImportAsset(root, source, "shot-1", domain.AssetKindImage)
	if err != nil {
		t.Fatal(err)
	}
	reference, err := store.ImportAsset(root, source, "shot-1", domain.AssetKindReference)
	if err != nil {
		t.Fatal(err)
	}
	api := NewAssetAPI(&Backend{store: store, root: root})
	if _, err := api.SelectVersion("shot-1", reference.ID); err == nil || !strings.Contains(err.Error(), "not an image version") {
		t.Fatalf("reference should not be selectable: %v", err)
	}
	shot, err := api.SelectVersion("shot-1", image.ID)
	if err != nil || shot.SelectedAssetID != image.ID {
		t.Fatalf("image selection = %+v, err = %v", shot, err)
	}
	snapshot, err := store.Open(root)
	if err != nil || len(snapshot.Shots[0].ReferenceAssets) != 1 || snapshot.Shots[0].ReferenceAssets[0] != reference.ID {
		t.Fatalf("reference attachment = %+v, err = %v", snapshot.Shots, err)
	}
	videoSource := filepath.Join(t.TempDir(), "shot.mp4")
	if err := os.WriteFile(videoSource, []byte("video-bytes"), 0o600); err != nil {
		t.Fatal(err)
	}
	video, err := store.ImportAssetWithParent(root, videoSource, "shot-1", domain.AssetKindVideo, image.ID)
	if err != nil || video.ParentAssetID != image.ID {
		t.Fatalf("imported video lineage = %+v, err = %v", video, err)
	}
}
