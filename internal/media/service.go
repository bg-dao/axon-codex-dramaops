package media

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/bg-dao/axon-codex-sceneops/internal/approval"
	"github.com/bg-dao/axon-codex-sceneops/internal/domain"
	"github.com/bg-dao/axon-codex-sceneops/internal/project"
	"github.com/bg-dao/axon-codex-sceneops/internal/provider"
	"github.com/bg-dao/axon-codex-sceneops/internal/redact"
	"github.com/google/uuid"
)

type Service struct {
	Root     string
	Store    *project.Store
	Provider provider.MediaProvider
	Approval approval.Gate
}

type Result struct {
	Run   domain.Run    `json:"run"`
	Job   provider.Job  `json:"job"`
	Asset *domain.Asset `json:"asset,omitempty"`
}

func (s *Service) GenerateImage(ctx context.Context, shotID string, request provider.ImageRequest) (Result, error) {
	if err := project.ValidateID(shotID); err != nil {
		return Result{}, err
	}
	request = normalizeImageRequest(request)
	run, err := s.newRun("image_generate", shotID, map[string]any{
		"prompt": request.Prompt, "model": request.Model, "parameters": request.Parameters,
	})
	if err != nil {
		return Result{}, err
	}
	if _, err := s.Approval.Request(ctx, approval.ImageGenerate, "Generate a paid image asset", map[string]any{"runId": run.ID, "shotId": shotID, "prompt": request.Prompt}); err != nil {
		return Result{Run: s.failOrCancel(run, domain.RunCancelled, err)}, err
	}
	run, err = s.Store.TransitionRun(s.Root, run.ID, domain.RunRunning, "")
	if err != nil {
		return Result{}, err
	}
	job, err := s.Provider.GenerateImage(ctx, request)
	if err != nil {
		return Result{Run: s.failOrCancel(run, domain.RunFailed, err)}, err
	}
	run.ProviderJobID = job.ID
	if err := s.Store.SaveRun(s.Root, run); err != nil {
		return Result{}, err
	}
	asset, err := s.persistResult(ctx, run, job, domain.AssetKindImage, ".png", request.Prompt, request.Model, request.Parameters)
	if err != nil {
		return Result{Run: s.failOrCancel(run, domain.RunFailed, err), Job: job}, err
	}
	return Result{Run: mustTransition(s.Store, s.Root, run.ID, domain.RunSucceeded), Job: job, Asset: &asset}, nil
}

func (s *Service) GenerateVideo(ctx context.Context, shotID string, request provider.VideoRequest) (Result, error) {
	if err := project.ValidateID(shotID); err != nil {
		return Result{}, err
	}
	capabilities, err := s.Provider.Capabilities(ctx)
	if err != nil {
		return Result{}, err
	}
	if !capabilities.VideoGeneration {
		return Result{}, fmt.Errorf("video generation is unavailable: %s", capabilities.Reason)
	}
	request = normalizeVideoRequest(request)
	run, err := s.newRun("video_generate", shotID, map[string]any{
		"prompt": request.Prompt, "model": request.Model, "parameters": request.Parameters,
	})
	if err != nil {
		return Result{}, err
	}
	if _, err := s.Approval.Request(ctx, approval.VideoGenerate, "Generate a paid video asset", map[string]any{"runId": run.ID, "shotId": shotID, "prompt": request.Prompt, "seconds": request.Seconds, "size": request.Size}); err != nil {
		return Result{Run: s.failOrCancel(run, domain.RunCancelled, err)}, err
	}
	run, err = s.Store.TransitionRun(s.Root, run.ID, domain.RunRunning, "")
	if err != nil {
		return Result{}, err
	}
	job, err := s.Provider.GenerateVideo(ctx, request)
	if err != nil {
		return Result{Run: s.failOrCancel(run, domain.RunFailed, err)}, err
	}
	run.ProviderJobID = job.ID
	if err := s.Store.SaveRun(s.Root, run); err != nil {
		return Result{}, err
	}
	if job.Status == provider.JobSucceeded {
		asset, persistErr := s.persistResult(ctx, run, job, domain.AssetKindVideo, ".mp4", request.Prompt, request.Model, request.Parameters)
		if persistErr != nil {
			return Result{Run: s.failOrCancel(run, domain.RunFailed, persistErr), Job: job}, persistErr
		}
		return Result{Run: mustTransition(s.Store, s.Root, run.ID, domain.RunSucceeded), Job: job, Asset: &asset}, nil
	}
	_ = project.RebuildIndex(s.Root)
	return Result{Run: run, Job: job}, nil
}

func (s *Service) GetRun(ctx context.Context, runID string) (Result, error) {
	run, err := s.findRun(runID)
	if err != nil {
		return Result{}, err
	}
	if asset, found := s.assetForRun(runID); found {
		job := provider.Job{ID: run.ProviderJobID, Kind: string(asset.Kind), Status: provider.JobSucceeded, Progress: 100, UpdatedAt: run.UpdatedAt}
		return Result{Run: run, Job: job, Asset: &asset}, nil
	}
	if run.ProviderJobID == "" {
		return Result{Run: run}, nil
	}
	job, err := s.Provider.GetJob(ctx, run.ProviderJobID)
	if err != nil {
		return Result{Run: run}, err
	}
	switch job.Status {
	case provider.JobSucceeded:
		kind, ext := domain.AssetKindVideo, ".mp4"
		if run.Operation == "image_generate" {
			kind, ext = domain.AssetKindImage, ".png"
		}
		prompt, _ := run.Metadata["prompt"].(string)
		model, _ := run.Metadata["model"].(string)
		parameters, _ := run.Metadata["parameters"].(map[string]any)
		asset, persistErr := s.persistResult(ctx, run, job, kind, ext, prompt, model, parameters)
		if persistErr != nil {
			return Result{Run: s.failOrCancel(run, domain.RunFailed, persistErr), Job: job}, persistErr
		}
		run = mustTransition(s.Store, s.Root, run.ID, domain.RunSucceeded)
		return Result{Run: run, Job: job, Asset: &asset}, nil
	case provider.JobFailed:
		run = s.failOrCancel(run, domain.RunFailed, errors.New(job.Error))
	case provider.JobCancelled:
		run = s.failOrCancel(run, domain.RunCancelled, errors.New("provider job cancelled"))
	}
	return Result{Run: run, Job: job}, nil
}

func normalizeImageRequest(request provider.ImageRequest) provider.ImageRequest {
	if request.Model == "" {
		request.Model = provider.DefaultImageModel
	}
	if request.Size == "" {
		request.Size = "1536x1024"
	}
	if request.Quality == "" {
		request.Quality = "medium"
	}
	request.Parameters = cloneParameters(request.Parameters)
	request.Parameters["size"] = request.Size
	request.Parameters["quality"] = request.Quality
	return request
}

func normalizeVideoRequest(request provider.VideoRequest) provider.VideoRequest {
	if request.Model == "" {
		request.Model = provider.DefaultVideoModel
	}
	if request.Seconds == 0 {
		request.Seconds = 4
	}
	if request.Size == "" {
		request.Size = "1280x720"
	}
	request.Parameters = cloneParameters(request.Parameters)
	request.Parameters["seconds"] = request.Seconds
	request.Parameters["size"] = request.Size
	return request
}

func cloneParameters(input map[string]any) map[string]any {
	output := make(map[string]any, len(input)+2)
	for key, value := range input {
		output[key] = value
	}
	return output
}

func (s *Service) CancelRun(ctx context.Context, runID string) (Result, error) {
	run, err := s.findRun(runID)
	if err != nil {
		return Result{}, err
	}
	if run.Status == domain.RunSucceeded || run.Status == domain.RunFailed || run.Status == domain.RunCancelled {
		return Result{}, fmt.Errorf("run %s is already terminal", run.ID)
	}
	if _, err := s.Approval.Request(ctx, approval.JobCancel, "Cancel a media generation job", map[string]any{"runId": run.ID, "providerJobId": run.ProviderJobID}); err != nil {
		return Result{Run: run}, err
	}
	job, err := s.Provider.CancelJob(ctx, run.ProviderJobID)
	if err != nil {
		return Result{Run: run}, err
	}
	run, err = s.Store.TransitionRun(s.Root, run.ID, domain.RunCancelled, "")
	if err != nil {
		return Result{}, err
	}
	_ = project.RebuildIndex(s.Root)
	return Result{Run: run, Job: job}, nil
}

func (s *Service) newRun(operation, shotID string, metadata map[string]any) (domain.Run, error) {
	now := time.Now().UTC()
	run := domain.Run{SchemaVersion: domain.SchemaVersion, ID: uuid.NewString(), Operation: operation, Status: domain.RunAwaitingApproval, ShotID: shotID, Metadata: metadata, CreatedAt: now, UpdatedAt: now}
	if err := s.Store.SaveRun(s.Root, run); err != nil {
		return domain.Run{}, err
	}
	_ = project.RebuildIndex(s.Root)
	return run, nil
}

func (s *Service) persistResult(ctx context.Context, run domain.Run, job provider.Job, kind domain.AssetKind, ext, prompt, model string, parameters map[string]any) (domain.Asset, error) {
	if asset, found := s.assetForRun(run.ID); found {
		return asset, nil
	}
	assetID := uuid.NewString()
	relativePath := filepath.ToSlash(filepath.Join("assets", assetID, "result"+ext))
	destination, err := project.ResolveRelative(s.Root, relativePath)
	if err != nil {
		return domain.Asset{}, err
	}
	var output bytes.Buffer
	if err := s.Provider.DownloadResult(ctx, job.ID, &output); err != nil {
		return domain.Asset{}, err
	}
	if output.Len() == 0 {
		return domain.Asset{}, errors.New("provider returned an empty asset")
	}
	if err := project.AtomicWrite(destination, output.Bytes(), 0o644); err != nil {
		return domain.Asset{}, err
	}
	hash := sha256.Sum256(output.Bytes())
	if model == "" {
		if kind == domain.AssetKindImage {
			model = provider.DefaultImageModel
		} else {
			model = provider.DefaultVideoModel
		}
	}
	asset := domain.Asset{
		SchemaVersion: domain.SchemaVersion,
		ID:            assetID,
		ShotID:        run.ShotID,
		Kind:          kind,
		RelativePath:  relativePath,
		SHA256:        hex.EncodeToString(hash[:]),
		RunID:         run.ID,
		Provenance: domain.Provenance{
			Provider:          s.Provider.Name(),
			Model:             model,
			Prompt:            prompt,
			Parameters:        parameters,
			ProviderRequestID: job.ProviderRequestID,
			GeneratedAt:       time.Now().UTC(),
		},
		CreatedAt: time.Now().UTC(),
	}
	if err := s.Store.SaveAsset(s.Root, asset); err != nil {
		return domain.Asset{}, err
	}
	if err := project.RebuildIndex(s.Root); err != nil {
		return domain.Asset{}, err
	}
	return asset, nil
}

func (s *Service) findRun(runID string) (domain.Run, error) {
	if err := project.ValidateID(runID); err != nil {
		return domain.Run{}, err
	}
	snapshot, err := s.Store.Open(s.Root)
	if err != nil {
		return domain.Run{}, err
	}
	for _, run := range snapshot.Runs {
		if run.ID == runID {
			return run, nil
		}
	}
	return domain.Run{}, fmt.Errorf("run %s not found", runID)
}

func (s *Service) assetForRun(runID string) (domain.Asset, bool) {
	snapshot, err := s.Store.Open(s.Root)
	if err != nil {
		return domain.Asset{}, false
	}
	for _, asset := range snapshot.Assets {
		if asset.RunID == runID {
			return asset, true
		}
	}
	return domain.Asset{}, false
}

func (s *Service) failOrCancel(run domain.Run, status domain.RunStatus, err error) domain.Run {
	message := ""
	if err != nil {
		message = redact.String(err.Error())
	}
	updated, transitionErr := s.Store.TransitionRun(s.Root, run.ID, status, message)
	if transitionErr == nil {
		_ = project.RebuildIndex(s.Root)
		return updated
	}
	return run
}

func mustTransition(store *project.Store, root, runID string, status domain.RunStatus) domain.Run {
	run, err := store.TransitionRun(root, runID, status, "")
	if err == nil {
		_ = project.RebuildIndex(root)
	}
	return run
}
