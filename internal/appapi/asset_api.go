package appapi

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bg-dao/axon-codex-sceneops/internal/approval"
	"github.com/bg-dao/axon-codex-sceneops/internal/domain"
	"github.com/bg-dao/axon-codex-sceneops/internal/media"
	"github.com/bg-dao/axon-codex-sceneops/internal/project"
	"github.com/bg-dao/axon-codex-sceneops/internal/provider"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type AssetAPI struct{ backend *Backend }

func NewAssetAPI(backend *Backend) *AssetAPI { return &AssetAPI{backend: backend} }

func (a *AssetAPI) GenerateImage(shotID string, request provider.ImageRequest) (media.Result, error) {
	service, err := a.backend.Media()
	if err != nil {
		return media.Result{}, err
	}
	return service.GenerateImage(a.backend.context(), shotID, request)
}

func (a *AssetAPI) GenerateVideo(shotID string, request provider.VideoRequest) (media.Result, error) {
	service, err := a.backend.Media()
	if err != nil {
		return media.Result{}, err
	}
	return service.GenerateVideo(a.backend.context(), shotID, request)
}

func (a *AssetAPI) Capabilities() (provider.Capabilities, error) {
	service, err := a.backend.Media()
	if err != nil {
		return provider.Capabilities{}, err
	}
	ctx, cancel := context.WithTimeout(a.backend.context(), 20*time.Second)
	defer cancel()
	return service.Provider.Capabilities(ctx)
}

func (a *AssetAPI) GetRun(runID string) (media.Result, error) {
	service, err := a.backend.Media()
	if err != nil {
		return media.Result{}, err
	}
	ctx, cancel := context.WithTimeout(a.backend.context(), 30*time.Second)
	defer cancel()
	return service.GetRun(ctx, runID)
}

func (a *AssetAPI) CancelRun(runID string) (media.Result, error) {
	service, err := a.backend.Media()
	if err != nil {
		return media.Result{}, err
	}
	return service.CancelRun(a.backend.context(), runID)
}

func (a *AssetAPI) PendingApprovals() ([]approval.Request, error) {
	gate, err := a.backend.Gate()
	if err != nil {
		return nil, err
	}
	return gate.Pending()
}

func (a *AssetAPI) ResolveApproval(id string, approved bool) (approval.Decision, error) {
	gate, err := a.backend.Gate()
	if err != nil {
		return approval.Decision{}, err
	}
	decision, err := gate.Resolve(id, approved)
	if err == nil {
		a.backend.emit(EventApprovalResolved, decision)
	}
	return decision, err
}

func (a *AssetAPI) Import(source, shotID string, kind domain.AssetKind) (domain.Asset, error) {
	root, err := a.backend.Root()
	if err != nil {
		return domain.Asset{}, err
	}
	return a.backend.store.ImportAsset(root, source, shotID, kind)
}

func (a *AssetAPI) ImportReference(shotID string) (domain.Asset, error) {
	return a.importFromDialog(shotID, domain.AssetKindReference, "Reference images", "*.png;*.jpg;*.jpeg;*.webp;*.gif")
}

func (a *AssetAPI) ImportExternalVideo(shotID string) (domain.Asset, error) {
	return a.importFromDialog(shotID, domain.AssetKindVideo, "Video files", "*.mp4;*.mov;*.m4v;*.webm")
}

func (a *AssetAPI) DataURL(assetID string) (string, error) {
	root, err := a.backend.Root()
	if err != nil {
		return "", err
	}
	snapshot, err := a.backend.store.Open(root)
	if err != nil {
		return "", err
	}
	for _, asset := range snapshot.Assets {
		if asset.ID != assetID {
			continue
		}
		path, err := project.ResolveRelative(root, asset.RelativePath)
		if err != nil {
			return "", err
		}
		info, err := os.Stat(path)
		if err != nil {
			return "", err
		}
		if !info.Mode().IsRegular() || info.Size() > 256<<20 {
			return "", fmt.Errorf("asset %s is not a previewable regular file", asset.ID)
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		digest := sha256.Sum256(content)
		if !equalHash(asset.SHA256, hex.EncodeToString(digest[:])) {
			return "", fmt.Errorf("asset %s failed SHA-256 verification", asset.ID)
		}
		mediaType := mime.TypeByExtension(strings.ToLower(filepath.Ext(path)))
		if mediaType == "" {
			if asset.Kind == domain.AssetKindVideo {
				mediaType = "video/mp4"
			} else {
				mediaType = "image/png"
			}
		}
		return "data:" + mediaType + ";base64," + base64.StdEncoding.EncodeToString(content), nil
	}
	return "", fmt.Errorf("asset %s not found", assetID)
}

func (a *AssetAPI) importFromDialog(shotID string, kind domain.AssetKind, displayName, pattern string) (domain.Asset, error) {
	root, err := a.backend.Root()
	if err != nil {
		return domain.Asset{}, err
	}
	source, err := wailsruntime.OpenFileDialog(a.backend.context(), wailsruntime.OpenDialogOptions{
		Title:   "Import " + displayName,
		Filters: []wailsruntime.FileFilter{{DisplayName: displayName, Pattern: pattern}},
	})
	if err != nil || source == "" {
		return domain.Asset{}, err
	}
	parentAssetID := ""
	if kind == domain.AssetKindVideo {
		snapshot, openErr := a.backend.store.Open(root)
		if openErr != nil {
			return domain.Asset{}, openErr
		}
		for _, shot := range snapshot.Shots {
			if shot.ID == shotID {
				parentAssetID = shot.SelectedAssetID
				break
			}
		}
	}
	asset, err := a.backend.store.ImportAssetWithParent(root, source, shotID, kind, parentAssetID)
	if err == nil {
		a.backend.emit(EventProjectChanged, map[string]any{"root": root})
	}
	return asset, err
}

func equalHash(left, right string) bool {
	return strings.EqualFold(strings.TrimSpace(left), strings.TrimSpace(right))
}

func (a *AssetAPI) SelectVersion(shotID, assetID string) (domain.Shot, error) {
	root, err := a.backend.Root()
	if err != nil {
		return domain.Shot{}, err
	}
	shot, err := a.backend.store.SelectImageVersion(root, shotID, assetID)
	if err == nil {
		a.backend.emit(EventProjectChanged, map[string]any{"root": root})
	}
	return shot, err
}
