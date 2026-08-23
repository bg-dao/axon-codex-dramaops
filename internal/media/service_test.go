package media

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bg-dao/axon-codex-dramaops/internal/approval"
	"github.com/bg-dao/axon-codex-dramaops/internal/domain"
	"github.com/bg-dao/axon-codex-dramaops/internal/project"
	"github.com/bg-dao/axon-codex-dramaops/internal/provider"
)

type fakeProvider struct {
	mu            sync.Mutex
	jobs          map[string]provider.Job
	data          map[string]string
	videoEnabled  bool
	lastImage     provider.ImageRequest
	lastVideo     provider.VideoRequest
	lastSpeech    provider.SpeechRequest
	providerCalls int
}

func newFakeProvider() *fakeProvider {
	return &fakeProvider{jobs: make(map[string]provider.Job), data: make(map[string]string), videoEnabled: true}
}

func (f *fakeProvider) Name() string { return "fake" }
func (f *fakeProvider) ImageCapabilities(context.Context) (provider.Capabilities, error) {
	return provider.Capabilities{ImageGeneration: true, ImageReferences: true, MaxImageReferences: 8}, nil
}
func (f *fakeProvider) VideoCapabilities(context.Context) (provider.Capabilities, error) {
	return provider.Capabilities{VideoGeneration: f.videoEnabled, VideoExperimental: true, VideoReferenceRoles: []string{"keyframe", "previous_tail"}, MaxVideoReferences: 2, Reason: "unavailable"}, nil
}
func (f *fakeProvider) SpeechCapabilities(context.Context) (provider.Capabilities, error) {
	return provider.Capabilities{SpeechGeneration: true, CustomVoices: true, BuiltInVoices: []string{"alloy"}}, nil
}
func (f *fakeProvider) GenerateImage(_ context.Context, request provider.ImageRequest) (provider.Job, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lastImage, f.providerCalls = request, f.providerCalls+1
	job := provider.Job{ID: "image-job", Kind: "image", Status: provider.JobSucceeded, ProviderRequestID: "req-image", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	f.jobs[job.ID], f.data[job.ID] = job, "image-bytes"
	return job, nil
}
func (f *fakeProvider) DownloadImage(_ context.Context, id string, destination io.Writer) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	_, err := io.WriteString(destination, f.data[id])
	return err
}
func (f *fakeProvider) GenerateVideo(_ context.Context, request provider.VideoRequest) (provider.Job, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lastVideo, f.providerCalls = request, f.providerCalls+1
	job := provider.Job{ID: "video-job", Kind: "video", Status: provider.JobQueued, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	f.jobs[job.ID], f.data[job.ID] = job, "video-bytes"
	return job, nil
}
func (f *fakeProvider) GetVideoJob(_ context.Context, id string) (provider.Job, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	job, ok := f.jobs[id]
	if !ok {
		return provider.Job{}, errors.New("job not found")
	}
	job.Status, job.Progress = provider.JobSucceeded, 100
	f.jobs[id] = job
	return job, nil
}
func (f *fakeProvider) CancelVideoJob(_ context.Context, id string) (provider.Job, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	job := f.jobs[id]
	job.Status = provider.JobCancelled
	f.jobs[id] = job
	return job, nil
}
func (f *fakeProvider) DownloadVideo(_ context.Context, id string, destination io.Writer) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	_, err := io.WriteString(destination, f.data[id])
	return err
}
func (f *fakeProvider) GenerateSpeech(_ context.Context, request provider.SpeechRequest, destination io.Writer) (provider.SpeechResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lastSpeech, f.providerCalls = request, f.providerCalls+1
	_, _ = io.WriteString(destination, "speech-bytes")
	return provider.SpeechResult{ProviderRequestID: "req-speech", Model: request.Model, Voice: request.Voice, Format: "wav"}, nil
}
func (f *fakeProvider) CreateCustomVoice(context.Context, provider.CustomVoiceRequest) (provider.CustomVoiceResult, error) {
	return provider.CustomVoiceResult{ProviderVoiceID: "voice-device-only", ConsentID: "consent-device-only"}, nil
}

func setupService(t *testing.T, gate approval.Gate) (*Service, string, *fakeProvider) {
	t.Helper()
	root := filepath.Join(t.TempDir(), "project")
	store := project.NewStore()
	if _, err := store.Create(root, "Media Test"); err != nil {
		t.Fatal(err)
	}
	plan := project.ScriptPlan{
		Episode: domain.Episode{ID: "episode-001", Title: "Episode", ScriptBlocks: []domain.ScriptBlock{
			{ID: "block-1", SceneID: "scene-1", Kind: domain.ScriptDialogue, CharacterID: "character-1", Text: "Stay with me", Emotion: "urgent"},
		}},
		Scenes:     []domain.Scene{{ID: "scene-1", Title: "Platform", LocationID: "location-1"}},
		Characters: []domain.Character{{ID: "character-1", Name: "Aria", Appearance: "short black hair", Wardrobe: "green coat", VoiceProfile: domain.VoiceProfile{ID: "voice-1", Kind: domain.VoiceBuiltIn, Name: "Aria voice", BuiltInVoice: "alloy"}}},
		Locations:  []domain.Location{{ID: "location-1", Name: "Platform", Description: "rainy night"}},
	}
	if _, err := store.ApplyScript(root, plan); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ApplyShotPlan(root, "episode-001", []domain.Shot{
		{ID: "shot-1", SceneID: "scene-1", Title: "Close", Prompt: "Aria waits", CharacterIDs: []string{"character-1"}},
		{ID: "shot-2", SceneID: "scene-1", Title: "Answer", Prompt: "Aria answers", CharacterIDs: []string{"character-1"}},
	}); err != nil {
		t.Fatal(err)
	}
	fake := newFakeProvider()
	return &Service{Root: root, Store: store, Image: fake, Video: fake, Speech: fake, Approval: gate}, root, fake
}

func importReference(t *testing.T, store *project.Store, root, shotID, role string) domain.Asset {
	t.Helper()
	source := filepath.Join(t.TempDir(), role+".png")
	if err := os.WriteFile(source, []byte(role), 0o600); err != nil {
		t.Fatal(err)
	}
	asset, err := store.ImportAsset(root, project.ImportOptions{Source: source, EpisodeID: "episode-001", ShotID: shotID, Kind: domain.AssetKindReference})
	if err != nil {
		t.Fatal(err)
	}
	return asset
}

func TestGenerateImageAssemblesReferencesAndSelectsFirstVersion(t *testing.T) {
	service, root, fake := setupService(t, approval.AutoGate{Approved: true})
	characterRef := importReference(t, service.Store, root, "shot-1", "character")
	snapshot, _ := service.Store.Open(root)
	character := snapshot.Characters[0]
	character.ReferenceAssets = []string{characterRef.ID}
	if err := service.Store.SaveCharacter(root, character); err != nil {
		t.Fatal(err)
	}
	result, err := service.GenerateImage(context.Background(), "shot-1", provider.ImageRequest{Prompt: "warm horizon", Model: "image-test", Parameters: map[string]any{"apiKey": "sk-must-not-persist", "nested": map[string]any{"access_token": "must-not-persist", "seed": 42}}})
	if err != nil || result.Run.Status != domain.RunSucceeded || result.Asset == nil {
		t.Fatalf("result = %+v, err = %v", result, err)
	}
	if !strings.Contains(fake.lastImage.Prompt, "green coat") || len(fake.lastImage.References) != 1 || fake.lastImage.References[0].Role != "character:character-1" {
		t.Fatalf("consistency request = %+v", fake.lastImage)
	}
	if result.Asset.Provenance.ProviderRequestID != "req-image" || len(result.Asset.Inputs) != 1 {
		t.Fatalf("provenance = %+v", result.Asset)
	}
	encoded, _ := os.ReadFile(filepath.Join(root, "runs", result.Run.ID+".json"))
	manifest, _ := os.ReadFile(filepath.Join(root, "assets", result.Asset.ID, "asset.json"))
	if strings.Contains(string(encoded), "sk-must-not-persist") || strings.Contains(string(encoded), "must-not-persist") || strings.Contains(string(manifest), "must-not-persist") {
		t.Fatal("sensitive provider parameters leaked into durable project data")
	}
	path, _ := project.ResolveRelative(root, result.Asset.RelativePath)
	if hash, _ := project.HashFile(path); hash != result.Asset.SHA256 {
		t.Fatal("asset checksum mismatch")
	}
	snapshot, _ = service.Store.Open(root)
	if snapshot.Shots[0].SelectedKeyframeAssetID != result.Asset.ID {
		t.Fatalf("first keyframe was not selected: %+v", snapshot.Shots[0])
	}
	refreshed, err := service.GetRun(context.Background(), result.Run.ID)
	if err != nil || refreshed.Asset == nil || refreshed.Asset.ID != result.Asset.ID {
		t.Fatalf("idempotent refresh = %+v, err = %v", refreshed, err)
	}
}

func TestVideoUsesSelectedKeyframeAndPersistsRoleLineage(t *testing.T) {
	service, _, fake := setupService(t, approval.AutoGate{Approved: true})
	image, err := service.GenerateImage(context.Background(), "shot-1", provider.ImageRequest{Prompt: "keyframe"})
	if err != nil || image.Asset == nil {
		t.Fatalf("image = %+v, err = %v", image, err)
	}
	video, err := service.GenerateVideo(context.Background(), "shot-1", provider.VideoRequest{Prompt: "slow push in"})
	if err != nil || video.Job.Status != provider.JobQueued {
		t.Fatalf("video = %+v, err = %v", video, err)
	}
	if len(fake.lastVideo.References) != 1 || fake.lastVideo.References[0].AssetID != image.Asset.ID || fake.lastVideo.References[0].Path == "" {
		t.Fatalf("video reference = %+v", fake.lastVideo.References)
	}
	completed, err := service.GetRun(context.Background(), video.Run.ID)
	if err != nil || completed.Asset == nil || len(completed.Asset.Inputs) != 1 || completed.Asset.Inputs[0].AssetID != image.Asset.ID || completed.Asset.Inputs[0].Role != "keyframe" {
		t.Fatalf("video lineage = %+v, err = %v", completed, err)
	}
	again, err := service.GetRun(context.Background(), video.Run.ID)
	if err != nil || again.Asset == nil || again.Asset.ID != completed.Asset.ID {
		t.Fatalf("repeat refresh = %+v, err = %v", again, err)
	}
}

func TestSpeechUsesLockedVoiceProfile(t *testing.T) {
	service, root, fake := setupService(t, approval.AutoGate{Approved: true})
	result, err := service.GenerateSpeech(context.Background(), "episode-001", "block-1", provider.SpeechRequest{Instructions: "natural"})
	if err != nil || result.Asset == nil || result.Run.Status != domain.RunSucceeded {
		t.Fatalf("speech = %+v, err = %v", result, err)
	}
	if fake.lastSpeech.Voice != "alloy" || fake.lastSpeech.Text != "Stay with me" || fake.lastSpeech.VoiceProfileID != "voice-1" {
		t.Fatalf("speech request = %+v", fake.lastSpeech)
	}
	snapshot, _ := service.Store.Open(root)
	if snapshot.Episodes[0].ScriptBlocks[0].SelectedVoiceAssetID != result.Asset.ID {
		t.Fatalf("voice selection missing: %+v", snapshot.Episodes[0])
	}
	if result.Asset.Provenance.Parameters["voiceProfileId"] != "voice-1" {
		t.Fatalf("voice provenance = %#v", result.Asset.Provenance.Parameters)
	}
}

func TestNextVideoAddsPreviousTailWhenProviderSupportsIt(t *testing.T) {
	service, root, fake := setupService(t, approval.AutoGate{Approved: true})
	firstImage, err := service.GenerateImage(context.Background(), "shot-1", provider.ImageRequest{Prompt: "first keyframe"})
	if err != nil || firstImage.Asset == nil {
		t.Fatal(err)
	}
	firstVideo, err := service.GenerateVideo(context.Background(), "shot-1", provider.VideoRequest{Prompt: "first clip"})
	if err != nil {
		t.Fatal(err)
	}
	completed, err := service.GetRun(context.Background(), firstVideo.Run.ID)
	if err != nil || completed.Asset == nil {
		t.Fatalf("first video = %+v, err = %v", completed, err)
	}
	tailRelative := filepath.ToSlash(filepath.Join("assets", "tail-asset", "video-tail.png"))
	tailPath, _ := project.ResolveRelative(root, tailRelative)
	if err := project.AtomicWrite(tailPath, []byte("tail-frame"), 0o644); err != nil {
		t.Fatal(err)
	}
	tailHash, _ := project.HashFile(tailPath)
	if err := service.Store.SaveAsset(root, domain.Asset{
		SchemaVersion: domain.SchemaVersion, ID: "tail-asset", EpisodeID: "episode-001", ShotID: "shot-1", Kind: domain.AssetKindReference,
		RelativePath: tailRelative, SHA256: tailHash, Inputs: []domain.AssetInput{{AssetID: completed.Asset.ID, Role: "source_video"}},
		Provenance: domain.Provenance{Provider: "local-ffmpeg", Parameters: map[string]any{"frameRole": "tail"}}, CreatedAt: time.Now(),
	}); err != nil {
		t.Fatal(err)
	}
	secondImage, err := service.GenerateImage(context.Background(), "shot-2", provider.ImageRequest{Prompt: "second keyframe"})
	if err != nil || secondImage.Asset == nil {
		t.Fatal(err)
	}
	if _, err := service.GenerateVideo(context.Background(), "shot-2", provider.VideoRequest{Prompt: "second clip"}); err != nil {
		t.Fatal(err)
	}
	if len(fake.lastVideo.References) != 2 || fake.lastVideo.References[0].AssetID != secondImage.Asset.ID || fake.lastVideo.References[0].Role != "keyframe" || fake.lastVideo.References[1].AssetID != "tail-asset" || fake.lastVideo.References[1].Role != "previous_tail" {
		t.Fatalf("continuity references = %+v", fake.lastVideo.References)
	}
}

func TestApprovalDeclineCapabilityAndCancelFailClosed(t *testing.T) {
	service, _, fake := setupService(t, approval.AutoGate{Approved: false})
	_, err := service.GenerateImage(context.Background(), "shot-1", provider.ImageRequest{Prompt: "declined"})
	if err == nil || !strings.Contains(err.Error(), "declined") || fake.providerCalls != 0 {
		t.Fatalf("decline = %v, provider calls = %d", err, fake.providerCalls)
	}
	service.Approval = approval.AutoGate{Approved: true}
	fake.videoEnabled = false
	if _, err := service.GenerateVideo(context.Background(), "shot-1", provider.VideoRequest{Prompt: "video"}); err == nil || !strings.Contains(err.Error(), "unavailable") {
		t.Fatalf("expected capability failure, got %v", err)
	}
	fake.videoEnabled = true
	if image, err := service.GenerateImage(context.Background(), "shot-1", provider.ImageRequest{Prompt: "approved keyframe"}); err != nil || image.Asset == nil {
		t.Fatalf("keyframe = %+v, err = %v", image, err)
	}
	video, err := service.GenerateVideo(context.Background(), "shot-1", provider.VideoRequest{Prompt: "cancel"})
	if err != nil {
		t.Fatal(err)
	}
	cancelled, err := service.CancelRun(context.Background(), video.Run.ID)
	if err != nil || cancelled.Run.Status != domain.RunCancelled || cancelled.Job.Status != provider.JobCancelled {
		t.Fatalf("cancelled = %+v, err = %v", cancelled, err)
	}
}
