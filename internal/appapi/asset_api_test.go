package appapi

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bg-dao/axon-codex-dramaops/internal/approval"
	"github.com/bg-dao/axon-codex-dramaops/internal/domain"
	"github.com/bg-dao/axon-codex-dramaops/internal/project"
	"github.com/bg-dao/axon-codex-dramaops/internal/provider"
	"github.com/bg-dao/axon-codex-dramaops/internal/secret"
)

func setupAssetProject(t *testing.T) (string, *project.Store) {
	t.Helper()
	root := filepath.Join(t.TempDir(), "project")
	store := project.NewStore()
	snapshot, err := store.Create(root, "Assets")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.ApplyScript(root, project.ScriptPlan{
		Episode:    domain.Episode{ID: snapshot.Episodes[0].ID, Title: "Episode", ScriptBlocks: []domain.ScriptBlock{{ID: "block-1", SceneID: "scene-1", Kind: domain.ScriptDialogue, CharacterID: "character-1", Text: "Hello"}}},
		Scenes:     []domain.Scene{{ID: "scene-1", Title: "ROOM"}},
		Characters: []domain.Character{{ID: "character-1", Name: "Lin", VoiceProfile: domain.VoiceProfile{ID: "voice-1", Kind: domain.VoiceBuiltIn, Name: "Lin", BuiltInVoice: "alloy"}}},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ApplyShotPlan(root, "episode-001", []domain.Shot{{ID: "shot-1", SceneID: "scene-1", Title: "Shot", Prompt: "Lin"}}); err != nil {
		t.Fatal(err)
	}
	return root, store
}

func importTestAsset(t *testing.T, store *project.Store, root, name string, kind domain.AssetKind, inputs []domain.AssetInput) domain.Asset {
	t.Helper()
	source := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(source, []byte(name), 0o600); err != nil {
		t.Fatal(err)
	}
	asset, err := store.ImportAsset(root, project.ImportOptions{Source: source, EpisodeID: "episode-001", ShotID: "shot-1", Kind: kind, Inputs: inputs})
	if err != nil {
		t.Fatal(err)
	}
	return asset
}

func TestAssetDataURLVerifiesHash(t *testing.T) {
	root, store := setupAssetProject(t)
	asset := importTestAsset(t, store, root, "frame.png", domain.AssetKindImage, nil)
	api := NewAssetAPI(&Backend{store: store, root: root})
	dataURL, err := api.DataURL(asset.ID)
	if err != nil || !strings.HasPrefix(dataURL, "data:image/png;base64,") {
		t.Fatalf("unexpected preview: %q %v", dataURL, err)
	}
	path, _ := project.ResolveRelative(root, asset.RelativePath)
	if err := os.WriteFile(path, []byte("tampered"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := api.DataURL(asset.ID); err == nil || !strings.Contains(err.Error(), "SHA-256") {
		t.Fatalf("expected checksum failure, got %v", err)
	}
}

func TestSelectKeyframeAndVideoEnforcesTypesAndLineage(t *testing.T) {
	root, store := setupAssetProject(t)
	image := importTestAsset(t, store, root, "frame.png", domain.AssetKindImage, nil)
	reference := importTestAsset(t, store, root, "reference.png", domain.AssetKindReference, nil)
	api := NewAssetAPI(&Backend{store: store, root: root})
	if _, err := api.SelectKeyframe("shot-1", reference.ID); err == nil || !strings.Contains(err.Error(), "not a image version") {
		t.Fatalf("reference should not be selectable: %v", err)
	}
	shot, err := api.SelectKeyframe("shot-1", image.ID)
	if err != nil || shot.SelectedKeyframeAssetID != image.ID {
		t.Fatalf("keyframe selection = %+v, err = %v", shot, err)
	}
	video := importTestAsset(t, store, root, "clip.mp4", domain.AssetKindVideo, []domain.AssetInput{{AssetID: image.ID, Role: "keyframe"}, {AssetID: reference.ID, Role: "character"}})
	shot, err = api.SelectVideo("shot-1", video.ID)
	if err != nil || shot.SelectedVideoAssetID != video.ID || len(video.Inputs) != 2 {
		t.Fatalf("video selection = %+v, asset = %+v, err = %v", shot, video, err)
	}
}

type voiceTestProvider struct{}

func (voiceTestProvider) Name() string { return "fake" }
func (voiceTestProvider) SpeechCapabilities(context.Context) (provider.Capabilities, error) {
	return provider.Capabilities{SpeechGeneration: true, CustomVoices: true}, nil
}
func (voiceTestProvider) GenerateSpeech(context.Context, provider.SpeechRequest, io.Writer) (provider.SpeechResult, error) {
	return provider.SpeechResult{}, nil
}
func (voiceTestProvider) CreateCustomVoice(_ context.Context, request provider.CustomVoiceRequest) (provider.CustomVoiceResult, error) {
	return provider.CustomVoiceResult{ProviderVoiceID: "provider-voice-secret", ConsentID: "provider-consent-secret"}, nil
}

func TestCustomVoiceRequiresConsentAndStoresProviderIDOnlyInKeychain(t *testing.T) {
	root, store := setupAssetProject(t)
	secrets := secret.NewMemoryStore()
	backend := &Backend{store: store, root: root, secrets: secrets, speechProvider: voiceTestProvider{}, approvalOverride: approval.AutoGate{Approved: true}}
	api := NewAssetAPI(backend)
	if _, err := api.CreateCustomVoice("character-1", "consent.wav", "sample.wav", false); err == nil {
		t.Fatal("missing authorization must fail closed")
	}
	status, err := api.CreateCustomVoice("character-1", "consent.wav", "sample.wav", true)
	if err != nil || !status.Configured || !status.ConsentConfigured || status.ProfileID != "voice-1" {
		t.Fatalf("status = %+v, err = %v", status, err)
	}
	if value, err := secrets.Get(secret.VoiceBindingEntry("voice-1")); err != nil || value != "provider-voice-secret" {
		t.Fatalf("device binding = %q, err = %v", value, err)
	}
	if value, err := secrets.Get(secret.VoiceConsentEntry("voice-1")); err != nil || value != "provider-consent-secret" {
		t.Fatalf("consent binding = %q, err = %v", value, err)
	}
	snapshot, err := store.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	character := snapshot.Characters[0]
	if character.VoiceProfile.Kind != domain.VoiceCustom || !character.VoiceProfile.ConsentConfirmed || character.VoiceProfile.BuiltInVoice != "" {
		t.Fatalf("character voice profile = %+v", character.VoiceProfile)
	}
	manifest, _ := os.ReadFile(filepath.Join(root, "characters", "character-1.json"))
	if strings.Contains(string(manifest), "provider-voice-secret") || strings.Contains(string(manifest), "provider-consent-secret") {
		t.Fatalf("provider voice secrets leaked to manifest: %s", manifest)
	}
}
