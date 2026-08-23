package media

import (
	"context"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bg-dao/axon-codex-sceneops/internal/approval"
	"github.com/bg-dao/axon-codex-sceneops/internal/domain"
	"github.com/bg-dao/axon-codex-sceneops/internal/project"
	"github.com/bg-dao/axon-codex-sceneops/internal/provider"
)

type fakeProvider struct {
	mu           sync.Mutex
	jobs         map[string]provider.Job
	data         map[string]string
	videoEnabled bool
	lastVideo    provider.VideoRequest
}

func newFakeProvider() *fakeProvider {
	return &fakeProvider{jobs: make(map[string]provider.Job), data: make(map[string]string), videoEnabled: true}
}

func (f *fakeProvider) Name() string { return "fake" }
func (f *fakeProvider) Capabilities(context.Context) (provider.Capabilities, error) {
	return provider.Capabilities{ImageGeneration: true, VideoGeneration: f.videoEnabled, VideoReferences: f.videoEnabled}, nil
}
func (f *fakeProvider) GenerateImage(context.Context, provider.ImageRequest) (provider.Job, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	job := provider.Job{ID: "image-job", Kind: "image", Status: provider.JobSucceeded, ProviderRequestID: "req-1", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	f.jobs[job.ID] = job
	f.data[job.ID] = "image-bytes"
	return job, nil
}
func (f *fakeProvider) GenerateVideo(_ context.Context, request provider.VideoRequest) (provider.Job, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lastVideo = request
	job := provider.Job{ID: "video-job", Kind: "video", Status: provider.JobQueued, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	f.jobs[job.ID] = job
	f.data[job.ID] = "video-bytes"
	return job, nil
}
func (f *fakeProvider) GetJob(_ context.Context, id string) (provider.Job, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	job, ok := f.jobs[id]
	if !ok {
		return provider.Job{}, errors.New("job not found")
	}
	job.Status = provider.JobSucceeded
	job.Progress = 100
	f.jobs[id] = job
	return job, nil
}
func (f *fakeProvider) CancelJob(_ context.Context, id string) (provider.Job, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	job := f.jobs[id]
	job.Status = provider.JobCancelled
	f.jobs[id] = job
	return job, nil
}
func (f *fakeProvider) DownloadResult(_ context.Context, id string, destination io.Writer) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	_, err := io.WriteString(destination, f.data[id])
	return err
}

func setupService(t *testing.T, gate approval.Gate) (*Service, string) {
	t.Helper()
	root := filepath.Join(t.TempDir(), "project")
	store := project.NewStore()
	if _, err := store.Create(root, "Media Test"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ApplyStoryboard(root, []domain.Scene{{ID: "scene-1", Title: "Scene"}}, []domain.Shot{{ID: "shot-1", SceneID: "scene-1", Title: "Shot"}}); err != nil {
		t.Fatal(err)
	}
	return &Service{Root: root, Store: store, Provider: newFakeProvider(), Approval: gate}, root
}

func TestGenerateImagePersistsAssetAndProvenance(t *testing.T) {
	service, root := setupService(t, approval.AutoGate{Approved: true})
	result, err := service.GenerateImage(context.Background(), "shot-1", provider.ImageRequest{Prompt: "warm horizon", Model: "image-test", Parameters: map[string]any{"quality": "high"}})
	if err != nil {
		t.Fatal(err)
	}
	if result.Run.Status != domain.RunSucceeded || result.Asset == nil {
		t.Fatalf("unexpected result: %+v", result)
	}
	if result.Asset.Provenance.Provider != "fake" || result.Asset.Provenance.ProviderRequestID != "req-1" {
		t.Fatalf("unexpected provenance: %+v", result.Asset.Provenance)
	}
	if result.Asset.Provenance.Parameters["size"] != "1536x1024" || result.Asset.Provenance.Parameters["quality"] != "medium" {
		t.Fatalf("effective generation parameters were not preserved: %#v", result.Asset.Provenance.Parameters)
	}
	path, err := project.ResolveRelative(root, result.Asset.RelativePath)
	if err != nil {
		t.Fatal(err)
	}
	if hash, _ := project.HashFile(path); hash != result.Asset.SHA256 {
		t.Fatalf("asset checksum mismatch")
	}
	snapshot, err := service.Store.Open(root)
	if err != nil || snapshot.Shots[0].SelectedAssetID != result.Asset.ID {
		t.Fatalf("first image was not selected: %+v %v", snapshot.Shots, err)
	}
	refreshed, err := service.GetRun(context.Background(), result.Run.ID)
	if err != nil || refreshed.Asset == nil || refreshed.Asset.ID != result.Asset.ID {
		t.Fatalf("idempotent status refresh failed: %+v %v", refreshed, err)
	}
}

func TestVideoUsesSelectedKeyframeAndPersistsLineage(t *testing.T) {
	service, _ := setupService(t, approval.AutoGate{Approved: true})
	image, err := service.GenerateImage(context.Background(), "shot-1", provider.ImageRequest{Prompt: "keyframe"})
	if err != nil || image.Asset == nil {
		t.Fatalf("image = %+v, err = %v", image, err)
	}
	video, err := service.GenerateVideo(context.Background(), "shot-1", provider.VideoRequest{Prompt: "slow push in"})
	if err != nil {
		t.Fatal(err)
	}
	fake := service.Provider.(*fakeProvider)
	if fake.lastVideo.ReferenceAssetID != image.Asset.ID || fake.lastVideo.ReferencePath == "" {
		t.Fatalf("video reference was not resolved: %+v", fake.lastVideo)
	}
	completed, err := service.GetRun(context.Background(), video.Run.ID)
	if err != nil || completed.Asset == nil || completed.Asset.ParentAssetID != image.Asset.ID {
		t.Fatalf("video lineage = %+v, err = %v", completed, err)
	}
	again, err := service.GetRun(context.Background(), video.Run.ID)
	if err != nil || again.Asset == nil || again.Asset.ID != completed.Asset.ID {
		t.Fatalf("video refresh was not idempotent: %+v, err = %v", again, err)
	}
}

func TestCancelRunningVideo(t *testing.T) {
	service, _ := setupService(t, approval.AutoGate{Approved: true})
	video, err := service.GenerateVideo(context.Background(), "shot-1", provider.VideoRequest{Prompt: "cancel me"})
	if err != nil {
		t.Fatal(err)
	}
	cancelled, err := service.CancelRun(context.Background(), video.Run.ID)
	if err != nil || cancelled.Run.Status != domain.RunCancelled || cancelled.Job.Status != provider.JobCancelled {
		t.Fatalf("cancelled = %+v, err = %v", cancelled, err)
	}
}

func TestGenerateImageDeclineCancelsWithoutProviderCall(t *testing.T) {
	service, _ := setupService(t, approval.AutoGate{Approved: false})
	_, err := service.GenerateImage(context.Background(), "shot-1", provider.ImageRequest{Prompt: "declined"})
	if err == nil || !strings.Contains(err.Error(), "declined") {
		t.Fatalf("expected decline, got %v", err)
	}
	snapshot, _ := service.Store.Open(service.Root)
	if len(snapshot.Runs) != 1 || snapshot.Runs[0].Status != domain.RunCancelled || len(snapshot.Assets) != 0 {
		t.Fatalf("declined run persisted incorrectly: %+v", snapshot)
	}
}

func TestVideoCapabilityFailsClosed(t *testing.T) {
	service, _ := setupService(t, approval.AutoGate{Approved: true})
	service.Provider.(*fakeProvider).videoEnabled = false
	_, err := service.GenerateVideo(context.Background(), "shot-1", provider.VideoRequest{Prompt: "video"})
	if err == nil || !strings.Contains(err.Error(), "unavailable") {
		t.Fatalf("expected capability failure, got %v", err)
	}
}
