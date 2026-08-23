package appapi

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bg-dao/axon-codex-dramaops/internal/approval"
	"github.com/bg-dao/axon-codex-dramaops/internal/domain"
	"github.com/bg-dao/axon-codex-dramaops/internal/media"
	"github.com/bg-dao/axon-codex-dramaops/internal/project"
	"github.com/bg-dao/axon-codex-dramaops/internal/provider"
	renderengine "github.com/bg-dao/axon-codex-dramaops/internal/render"
	"github.com/bg-dao/axon-codex-dramaops/internal/secret"
	"github.com/google/uuid"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type AssetAPI struct{ backend *Backend }

type VoiceBindingStatus struct {
	ProfileID         string `json:"profileId"`
	Configured        bool   `json:"configured"`
	ConsentConfigured bool   `json:"consentConfigured"`
}

func NewAssetAPI(backend *Backend) *AssetAPI { return &AssetAPI{backend: backend} }

func (a *AssetAPI) GenerateImage(shotID string, request provider.ImageRequest) (media.Result, error) {
	service, err := a.backend.Media()
	if err != nil {
		return media.Result{}, err
	}
	return service.GenerateImage(a.backend.projectContext(), shotID, request)
}

func (a *AssetAPI) GenerateVideo(shotID string, request provider.VideoRequest) (media.Result, error) {
	service, err := a.backend.Media()
	if err != nil {
		return media.Result{}, err
	}
	result, err := service.GenerateVideo(a.backend.projectContext(), shotID, request)
	if err == nil && result.Asset != nil {
		_ = a.ensureContinuityFrames(*result.Asset)
	}
	return result, err
}

func (a *AssetAPI) GenerateSpeech(episodeID, scriptBlockID string, request provider.SpeechRequest) (media.Result, error) {
	service, err := a.backend.Media()
	if err != nil {
		return media.Result{}, err
	}
	return service.GenerateSpeech(a.backend.projectContext(), episodeID, scriptBlockID, request)
}

func (a *AssetAPI) Capabilities() (provider.Capabilities, error) {
	service, err := a.backend.Media()
	if err != nil {
		return provider.Capabilities{}, err
	}
	ctx, cancel := context.WithTimeout(a.backend.projectContext(), 20*time.Second)
	defer cancel()
	return service.Capabilities(ctx)
}

func (a *AssetAPI) GetRun(runID string) (media.Result, error) {
	service, err := a.backend.Media()
	if err != nil {
		return media.Result{}, err
	}
	ctx, cancel := context.WithTimeout(a.backend.projectContext(), 30*time.Second)
	defer cancel()
	result, err := service.GetRun(ctx, runID)
	if err == nil && result.Asset != nil && result.Asset.Kind == domain.AssetKindVideo {
		_ = a.ensureContinuityFrames(*result.Asset)
	}
	return result, err
}

func (a *AssetAPI) CancelRun(runID string) (media.Result, error) {
	service, err := a.backend.Media()
	if err != nil {
		return media.Result{}, err
	}
	return service.CancelRun(a.backend.projectContext(), runID)
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

func (a *AssetAPI) ImportReference(shotID string) (domain.Asset, error) {
	snapshot, err := a.current()
	if err != nil {
		return domain.Asset{}, err
	}
	shot, err := findShot(snapshot, shotID)
	if err != nil {
		return domain.Asset{}, err
	}
	return a.importFromDialog(project.ImportOptions{EpisodeID: shot.EpisodeID, ShotID: shot.ID, Kind: domain.AssetKindReference}, "Reference images", "*.png;*.jpg;*.jpeg;*.webp")
}

func (a *AssetAPI) ImportExternalVideo(shotID string) (domain.Asset, error) {
	snapshot, err := a.current()
	if err != nil {
		return domain.Asset{}, err
	}
	shot, err := findShot(snapshot, shotID)
	if err != nil {
		return domain.Asset{}, err
	}
	inputs := []domain.AssetInput{}
	if shot.SelectedKeyframeAssetID != "" {
		inputs = append(inputs, domain.AssetInput{AssetID: shot.SelectedKeyframeAssetID, Role: "keyframe"})
	}
	asset, err := a.importFromDialog(project.ImportOptions{EpisodeID: shot.EpisodeID, ShotID: shot.ID, Kind: domain.AssetKindVideo, Inputs: inputs}, "Video clips", "*.mp4;*.mov;*.m4v;*.webm")
	if err == nil {
		_ = a.ensureContinuityFrames(asset)
	}
	return asset, err
}

func (a *AssetAPI) ImportDialogue(episodeID, scriptBlockID string) (domain.Asset, error) {
	return a.importFromDialog(project.ImportOptions{EpisodeID: episodeID, ScriptBlockID: scriptBlockID, Kind: domain.AssetKindAudio}, "Dialogue audio", "*.wav;*.mp3;*.m4a;*.aac;*.flac")
}

func (a *AssetAPI) ImportSound(episodeID string, kind string) (domain.Asset, error) {
	if kind != "sfx" && kind != "bgm" && kind != "ambience" {
		return domain.Asset{}, fmt.Errorf("unsupported sound role %q", kind)
	}
	asset, err := a.importFromDialog(project.ImportOptions{EpisodeID: episodeID, Kind: domain.AssetKindAudio}, "Audio", "*.wav;*.mp3;*.m4a;*.aac;*.flac")
	if err != nil || asset.ID == "" {
		return asset, err
	}
	root, err := a.backend.Root()
	if err != nil {
		return domain.Asset{}, err
	}
	snapshot, err := a.backend.store.Open(root)
	if err != nil {
		return domain.Asset{}, err
	}
	switch kind {
	case "bgm":
		snapshot.Project.SoundPalette.BGMAssetIDs = appendUnique(snapshot.Project.SoundPalette.BGMAssetIDs, asset.ID)
	case "ambience":
		snapshot.Project.SoundPalette.AmbienceAssetIDs = appendUnique(snapshot.Project.SoundPalette.AmbienceAssetIDs, asset.ID)
	case "sfx":
		if snapshot.Project.SoundPalette.Motifs == nil {
			snapshot.Project.SoundPalette.Motifs = map[string]string{}
		}
		snapshot.Project.SoundPalette.Motifs["sfx:"+asset.ID] = asset.ID
	}
	if err := a.backend.store.SaveProject(root, snapshot.Project); err != nil {
		return domain.Asset{}, err
	}
	_ = project.RebuildIndex(root)
	a.backend.emit(EventProjectChanged, map[string]any{"root": root})
	return asset, nil
}

func (a *AssetAPI) SelectKeyframe(shotID, assetID string) (domain.Shot, error) {
	root, err := a.backend.Root()
	if err != nil {
		return domain.Shot{}, err
	}
	shot, err := a.backend.store.SelectKeyframeVersion(root, shotID, assetID)
	if err == nil {
		a.backend.emit(EventProjectChanged, map[string]any{"root": root})
	}
	return shot, err
}

func (a *AssetAPI) SelectVideo(shotID, assetID string) (domain.Shot, error) {
	root, err := a.backend.Root()
	if err != nil {
		return domain.Shot{}, err
	}
	shot, err := a.backend.store.SelectVideoVersion(root, shotID, assetID)
	if err == nil {
		if snapshot, openErr := a.backend.store.Open(root); openErr == nil {
			for _, asset := range snapshot.Assets {
				if asset.ID == assetID {
					_ = a.ensureContinuityFrames(asset)
					break
				}
			}
		}
		a.backend.emit(EventProjectChanged, map[string]any{"root": root})
	}
	return shot, err
}

func (a *AssetAPI) VoiceBinding(profileID string) VoiceBindingStatus {
	return VoiceBindingStatus{
		ProfileID:         profileID,
		Configured:        a.backend.secrets.Exists(secret.VoiceBindingEntry(profileID)),
		ConsentConfigured: a.backend.secrets.Exists(secret.VoiceConsentEntry(profileID)),
	}
}

func (a *AssetAPI) CreateCustomVoice(characterID, consentPath, samplePath string, confirmed bool) (VoiceBindingStatus, error) {
	if !confirmed {
		return VoiceBindingStatus{}, errors.New("explicit voice authorization confirmation is required")
	}
	root, err := a.backend.Root()
	if err != nil {
		return VoiceBindingStatus{}, err
	}
	snapshot, err := a.backend.store.Open(root)
	if err != nil {
		return VoiceBindingStatus{}, err
	}
	var character domain.Character
	found := false
	for _, value := range snapshot.Characters {
		if value.ID == characterID {
			character, found = value, true
			break
		}
	}
	if !found {
		return VoiceBindingStatus{}, fmt.Errorf("character %s not found", characterID)
	}
	if a.backend.speechProvider == nil {
		return VoiceBindingStatus{}, errors.New("custom voice provider is unavailable")
	}
	gate, err := a.backend.ApprovalGate()
	if err != nil {
		return VoiceBindingStatus{}, err
	}
	if _, err := gate.Request(a.backend.projectContext(), approval.VoiceCreate, "Create a paid custom voice with confirmed authorization", map[string]any{"characterId": character.ID, "voiceProfileId": character.VoiceProfile.ID}); err != nil {
		return VoiceBindingStatus{}, err
	}
	result, err := a.backend.speechProvider.CreateCustomVoice(a.backend.projectContext(), provider.CustomVoiceRequest{Name: character.Name, Language: snapshot.Project.ContentLanguage, ConsentPath: consentPath, SamplePath: samplePath, Confirmed: true})
	if err != nil {
		return VoiceBindingStatus{}, err
	}
	if strings.TrimSpace(result.ProviderVoiceID) == "" || strings.TrimSpace(result.ConsentID) == "" {
		return VoiceBindingStatus{}, errors.New("custom voice provider returned an incomplete device binding")
	}
	// Provider voice and consent identifiers are device-only secrets. The consent
	// recording and sample are never copied into the project.
	if err := a.backend.secrets.Set(secret.VoiceConsentEntry(character.VoiceProfile.ID), result.ConsentID); err != nil {
		return VoiceBindingStatus{}, err
	}
	if err := a.backend.secrets.Set(secret.VoiceBindingEntry(character.VoiceProfile.ID), result.ProviderVoiceID); err != nil {
		_ = a.backend.secrets.Delete(secret.VoiceConsentEntry(character.VoiceProfile.ID))
		return VoiceBindingStatus{}, err
	}
	character.VoiceProfile.Kind, character.VoiceProfile.ConsentConfirmed = domain.VoiceCustom, true
	character.VoiceProfile.BuiltInVoice, character.VoiceProfile.ExternalAssetID = "", ""
	if err := a.backend.store.SaveCharacter(root, character); err != nil {
		_ = a.backend.secrets.Delete(secret.VoiceBindingEntry(character.VoiceProfile.ID))
		_ = a.backend.secrets.Delete(secret.VoiceConsentEntry(character.VoiceProfile.ID))
		return VoiceBindingStatus{}, err
	}
	_ = project.RebuildIndex(root)
	a.backend.emit(EventProjectChanged, map[string]any{"root": root})
	return VoiceBindingStatus{ProfileID: character.VoiceProfile.ID, Configured: true, ConsentConfigured: true}, nil
}

func (a *AssetAPI) ChooseAudioFile(title string) (string, error) {
	return wailsruntime.OpenFileDialog(a.backend.context(), wailsruntime.OpenDialogOptions{Title: title, Filters: []wailsruntime.FileFilter{{DisplayName: "Audio", Pattern: "*.wav;*.mp3;*.m4a;*.aac;*.flac"}}})
}

func (a *AssetAPI) DataURL(assetID string) (string, error) {
	snapshot, err := a.current()
	if err != nil {
		return "", err
	}
	for _, asset := range snapshot.Assets {
		if asset.ID != assetID {
			continue
		}
		path, err := project.ResolveRelative(snapshot.Root, asset.RelativePath)
		if err != nil {
			return "", err
		}
		info, err := os.Stat(path)
		if err != nil {
			return "", err
		}
		if !info.Mode().IsRegular() || info.Size() > 512<<20 {
			return "", fmt.Errorf("asset %s is not previewable", asset.ID)
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		digest := sha256.Sum256(content)
		if !strings.EqualFold(asset.SHA256, hex.EncodeToString(digest[:])) {
			return "", fmt.Errorf("asset %s failed SHA-256 verification", asset.ID)
		}
		mediaType := mime.TypeByExtension(strings.ToLower(filepath.Ext(path)))
		if mediaType == "" {
			switch asset.Kind {
			case domain.AssetKindVideo, domain.AssetKindRender:
				mediaType = "video/mp4"
			case domain.AssetKindAudio:
				mediaType = "audio/wav"
			case domain.AssetKindSubtitle:
				mediaType = "text/plain"
			default:
				mediaType = "image/png"
			}
		}
		return "data:" + mediaType + ";base64," + base64.StdEncoding.EncodeToString(content), nil
	}
	return "", fmt.Errorf("asset %s not found", assetID)
}

func (a *AssetAPI) importFromDialog(options project.ImportOptions, displayName, pattern string) (domain.Asset, error) {
	root, err := a.backend.Root()
	if err != nil {
		return domain.Asset{}, err
	}
	source, err := wailsruntime.OpenFileDialog(a.backend.context(), wailsruntime.OpenDialogOptions{Title: "Import " + displayName, Filters: []wailsruntime.FileFilter{{DisplayName: displayName, Pattern: pattern}}})
	if err != nil || source == "" {
		return domain.Asset{}, err
	}
	options.Source = source
	asset, err := a.backend.store.ImportAsset(root, options)
	if err == nil {
		a.backend.emit(EventProjectChanged, map[string]any{"root": root})
	}
	return asset, err
}

func (a *AssetAPI) current() (domain.Snapshot, error) {
	root, err := a.backend.Root()
	if err != nil {
		return domain.Snapshot{}, err
	}
	return a.backend.store.Open(root)
}

func findShot(snapshot domain.Snapshot, shotID string) (domain.Shot, error) {
	for _, shot := range snapshot.Shots {
		if shot.ID == shotID {
			return shot, nil
		}
	}
	return domain.Shot{}, fmt.Errorf("shot %s not found", shotID)
}

func appendUnique(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func (a *AssetAPI) ensureContinuityFrames(video domain.Asset) error {
	if video.Kind != domain.AssetKindVideo || video.ShotID == "" {
		return nil
	}
	a.backend.continuityMu.Lock()
	defer a.backend.continuityMu.Unlock()
	root, err := a.backend.Root()
	if err != nil {
		return err
	}
	snapshot, err := a.backend.store.Open(root)
	if err != nil {
		return err
	}
	existing := map[string]bool{}
	for _, asset := range snapshot.Assets {
		if asset.Kind != domain.AssetKindReference || asset.ShotID != video.ShotID {
			continue
		}
		usesVideo := false
		for _, input := range asset.Inputs {
			if input.AssetID == video.ID && input.Role == "source_video" {
				usesVideo = true
				break
			}
		}
		if usesVideo {
			if role, ok := asset.Provenance.Parameters["frameRole"].(string); ok {
				existing[role] = true
			}
		}
	}
	if existing["first"] && existing["tail"] {
		return nil
	}
	runtime := NewRenderAPI(a.backend).Runtime()
	if err := runtime.Require(); err != nil {
		return err
	}
	source, err := project.ResolveRelative(root, video.RelativePath)
	if err != nil {
		return err
	}
	actualHash, err := project.HashFile(source)
	if err != nil {
		return err
	}
	if !strings.EqualFold(actualHash, video.SHA256) {
		return fmt.Errorf("video asset %s failed SHA-256 verification", video.ID)
	}
	info, err := renderengine.Probe(a.backend.projectContext(), runtime, source)
	if err != nil {
		return err
	}
	if err := renderengine.ValidateMedia(info, domain.AssetKindVideo); err != nil {
		return err
	}
	if video.MediaInfo.DurationSeconds <= 0 {
		video.MediaInfo = info
		if err := a.backend.store.SaveAsset(root, video); err != nil {
			return err
		}
	}
	frameStep := 1.0 / float64(max(info.FPS, 25))
	positions := map[string]float64{"first": 0, "tail": max(0, info.DurationSeconds-frameStep)}
	for _, role := range []string{"first", "tail"} {
		if existing[role] {
			continue
		}
		temporaryDir, err := os.MkdirTemp("", "dramaops-frame-*")
		if err != nil {
			return err
		}
		temporary := filepath.Join(temporaryDir, role+".png")
		extractErr := renderengine.ExtractFrame(a.backend.projectContext(), runtime, source, positions[role], temporary)
		if extractErr != nil {
			_ = os.RemoveAll(temporaryDir)
			return extractErr
		}
		content, readErr := os.ReadFile(temporary)
		_ = os.RemoveAll(temporaryDir)
		if readErr != nil {
			return readErr
		}
		assetID := uuid.NewString()
		relative := filepath.ToSlash(filepath.Join("assets", assetID, "video-"+role+".png"))
		destination, err := project.ResolveRelative(root, relative)
		if err != nil {
			return err
		}
		if err := project.AtomicWrite(destination, content, 0o644); err != nil {
			return err
		}
		digest := sha256.Sum256(content)
		asset := domain.Asset{
			SchemaVersion: domain.SchemaVersion, ID: assetID, EpisodeID: video.EpisodeID, ShotID: video.ShotID,
			Kind: domain.AssetKindReference, RelativePath: relative, SHA256: hex.EncodeToString(digest[:]),
			Inputs:     []domain.AssetInput{{AssetID: video.ID, Role: "source_video"}},
			Provenance: domain.Provenance{Provider: "local-ffmpeg", Parameters: map[string]any{"frameRole": role, "atSeconds": positions[role]}, ToolVersion: runtime.Version, GeneratedAt: time.Now().UTC()},
			CreatedAt:  time.Now().UTC(),
		}
		if err := a.backend.store.SaveAsset(root, asset); err != nil {
			return err
		}
	}
	if err := project.RebuildIndex(root); err != nil {
		return err
	}
	a.backend.emit(EventProjectChanged, map[string]any{"root": root})
	return nil
}
