package media

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/bg-dao/axon-codex-dramaops/internal/approval"
	"github.com/bg-dao/axon-codex-dramaops/internal/domain"
	"github.com/bg-dao/axon-codex-dramaops/internal/project"
	"github.com/bg-dao/axon-codex-dramaops/internal/provider"
	"github.com/bg-dao/axon-codex-dramaops/internal/redact"
	"github.com/google/uuid"
)

type VoiceResolver func(profileID string) (string, error)

type referenceCandidate struct {
	id, role string
	priority int
}

type Service struct {
	Root         string
	Store        *project.Store
	Image        provider.ImageProvider
	Video        provider.VideoProvider
	Speech       provider.SpeechProvider
	Approval     approval.Gate
	ResolveVoice VoiceResolver
	runsMu       sync.Mutex
}

type Result struct {
	Run   domain.Run    `json:"run"`
	Job   provider.Job  `json:"job"`
	Asset *domain.Asset `json:"asset,omitempty"`
}

func (s *Service) Capabilities(ctx context.Context) (provider.Capabilities, error) {
	var result provider.Capabilities
	if s.Image != nil {
		value, err := s.Image.ImageCapabilities(ctx)
		if err != nil {
			return result, err
		}
		mergeCapabilities(&result, value)
	}
	if s.Video != nil {
		value, err := s.Video.VideoCapabilities(ctx)
		if err != nil {
			result.Reason = err.Error()
		} else {
			mergeCapabilities(&result, value)
		}
	}
	if s.Speech != nil {
		value, err := s.Speech.SpeechCapabilities(ctx)
		if err != nil {
			return result, err
		}
		mergeCapabilities(&result, value)
	}
	return result, nil
}

// RecoverInterruptedRuns leaves resumable provider video jobs alone and marks
// synchronous or pre-submission work as failed so the creator can retry it.
func (s *Service) RecoverInterruptedRuns() error {
	s.runsMu.Lock()
	defer s.runsMu.Unlock()
	snapshot, err := s.Store.Open(s.Root)
	if err != nil {
		return err
	}
	changed := false
	for _, run := range snapshot.Runs {
		if isTerminal(run.Status) || run.Operation == "episode_render" {
			continue
		}
		if run.Operation == "video_generate" && run.Status == domain.RunRunning && strings.TrimSpace(run.ProviderJobID) != "" {
			continue
		}
		if _, err := s.Store.TransitionRun(s.Root, run.ID, domain.RunFailed, "operation was interrupted before it could be resumed"); err != nil {
			return err
		}
		changed = true
	}
	if changed {
		return project.RebuildIndex(s.Root)
	}
	return nil
}

func mergeCapabilities(target *provider.Capabilities, source provider.Capabilities) {
	if source.ImageGeneration {
		target.ImageGeneration = true
		target.ImageReferences = source.ImageReferences
		target.MaxImageReferences = source.MaxImageReferences
		target.ImageModels = source.ImageModels
	}
	if source.VideoGeneration {
		target.VideoGeneration = true
		target.VideoReferenceRoles = source.VideoReferenceRoles
		target.MaxVideoReferences = source.MaxVideoReferences
		target.VideoModels = source.VideoModels
	}
	if source.VideoExperimental {
		target.VideoExperimental = true
	}
	if source.SpeechGeneration {
		target.SpeechGeneration = true
		target.CustomVoices = source.CustomVoices
		target.SpeechModels = source.SpeechModels
		target.BuiltInVoices = source.BuiltInVoices
	}
	if source.SoundGeneration {
		target.SoundGeneration = true
	}
	if source.Reason != "" {
		target.Reason = source.Reason
	}
	if source.VideoNotice != "" {
		target.VideoNotice = source.VideoNotice
	}
}

func (s *Service) GenerateImage(ctx context.Context, shotID string, request provider.ImageRequest) (Result, error) {
	if s.Image == nil {
		return Result{}, errors.New("image generation is not configured")
	}
	snapshot, shot, err := s.findShot(shotID)
	if err != nil {
		return Result{}, err
	}
	request = normalizeImageRequest(request, shot)
	request.Prompt = consistencyPrompt(snapshot, shot, request.Prompt)
	capabilities, err := s.Image.ImageCapabilities(ctx)
	if err != nil {
		return Result{}, err
	}
	if capabilities.ImageReferences {
		request.References, err = s.imageReferences(snapshot, shot, capabilities.MaxImageReferences)
		if err != nil {
			return Result{}, err
		}
	} else {
		request.References = nil
	}
	inputs := referencesToInputs(request.References)
	run, err := s.newUniqueRun("image_generate", shot.EpisodeID, shot.ID, "", map[string]any{
		"prompt": request.Prompt, "model": request.Model, "parameters": request.Parameters, "referenceAssetIds": inputIDs(inputs),
	})
	if err != nil {
		return Result{}, err
	}
	if _, err := s.Approval.Request(ctx, approval.ImageGenerate, "Generate a paid keyframe image", map[string]any{"runId": run.ID, "shotId": shot.ID, "references": len(inputs)}); err != nil {
		return Result{Run: s.failOrCancel(run, domain.RunCancelled, err)}, err
	}
	run, err = s.Store.TransitionRun(s.Root, run.ID, domain.RunRunning, "")
	if err != nil {
		return Result{}, err
	}
	job, err := s.Image.GenerateImage(ctx, request)
	if err != nil {
		return Result{Run: s.failOrCancel(run, domain.RunFailed, err)}, err
	}
	s.runsMu.Lock()
	defer s.runsMu.Unlock()
	run.ProviderJobID = job.ID
	if err := s.Store.SaveRun(s.Root, run); err != nil {
		return Result{}, err
	}
	asset, err := s.persistDownloaded(ctx, run, job, domain.AssetKindImage, ".png", s.Image.Name(), request.Prompt, request.Model, request.Parameters, inputs, s.Image.DownloadImage)
	if err != nil {
		return Result{Run: s.failOrCancel(run, domain.RunFailed, err), Job: job}, err
	}
	if shot.SelectedKeyframeAssetID == "" {
		if _, err := s.Store.SelectKeyframeVersion(s.Root, shot.ID, asset.ID); err != nil {
			return Result{}, err
		}
	}
	completed, err := s.completeRun(run.ID)
	return Result{Run: completed, Job: job, Asset: &asset}, err
}

func (s *Service) GenerateVideo(ctx context.Context, shotID string, request provider.VideoRequest) (Result, error) {
	if s.Video == nil {
		return Result{}, errors.New("video generation is not configured")
	}
	snapshot, shot, err := s.findShot(shotID)
	if err != nil {
		return Result{}, err
	}
	capabilities, err := s.Video.VideoCapabilities(ctx)
	if err != nil {
		return Result{}, err
	}
	if !capabilities.VideoGeneration {
		return Result{}, fmt.Errorf("video generation is unavailable: %s", capabilities.Reason)
	}
	request = normalizeVideoRequest(request, shot)
	request.References, err = s.videoReferences(snapshot, shot, request.References, capabilities)
	if err != nil {
		return Result{}, err
	}
	inputs := referencesToInputs(request.References)
	run, err := s.newUniqueRun("video_generate", shot.EpisodeID, shot.ID, "", map[string]any{
		"prompt": request.Prompt, "model": request.Model, "parameters": request.Parameters, "inputs": inputs,
	})
	if err != nil {
		return Result{}, err
	}
	if _, err := s.Approval.Request(ctx, approval.VideoGenerate, "Generate a paid video clip", map[string]any{"runId": run.ID, "shotId": shot.ID, "seconds": request.Seconds, "references": len(inputs)}); err != nil {
		return Result{Run: s.failOrCancel(run, domain.RunCancelled, err)}, err
	}
	run, err = s.Store.TransitionRun(s.Root, run.ID, domain.RunRunning, "")
	if err != nil {
		return Result{}, err
	}
	job, err := s.Video.GenerateVideo(ctx, request)
	if err != nil {
		return Result{Run: s.failOrCancel(run, domain.RunFailed, err)}, err
	}
	s.runsMu.Lock()
	defer s.runsMu.Unlock()
	run.ProviderJobID, run.Progress = job.ID, job.Progress
	if err := s.Store.SaveRun(s.Root, run); err != nil {
		return Result{}, err
	}
	if job.Status != provider.JobSucceeded {
		_ = project.RebuildIndex(s.Root)
		return Result{Run: run, Job: job}, nil
	}
	asset, err := s.persistDownloaded(ctx, run, job, domain.AssetKindVideo, ".mp4", s.Video.Name(), request.Prompt, request.Model, request.Parameters, inputs, s.Video.DownloadVideo)
	if err != nil {
		return Result{Run: s.failOrCancel(run, domain.RunFailed, err), Job: job}, err
	}
	if _, err := s.Store.SelectVideoVersion(s.Root, shot.ID, asset.ID); err != nil {
		return Result{}, err
	}
	completed, err := s.completeRun(run.ID)
	return Result{Run: completed, Job: job, Asset: &asset}, err
}

func (s *Service) GenerateSpeech(ctx context.Context, episodeID, blockID string, request provider.SpeechRequest) (Result, error) {
	if s.Speech == nil {
		return Result{}, errors.New("speech generation is not configured")
	}
	_, episode, block, character, err := s.findDialogue(episodeID, blockID)
	if err != nil {
		return Result{}, err
	}
	request.Text, request.VoiceProfileID = block.Text, character.VoiceProfile.ID
	request.Model = defaultString(request.Model, provider.DefaultSpeechModel)
	request.ResponseFormat = defaultString(request.ResponseFormat, "wav")
	request.Parameters = cloneParameters(request.Parameters)
	request.Parameters["voiceProfileId"] = character.VoiceProfile.ID
	request.Parameters["emotion"] = block.Emotion
	switch character.VoiceProfile.Kind {
	case domain.VoiceBuiltIn:
		request.Voice = character.VoiceProfile.BuiltInVoice
	case domain.VoiceCustom:
		if !character.VoiceProfile.ConsentConfirmed {
			return Result{}, errors.New("custom voice consent has not been confirmed")
		}
		if s.ResolveVoice == nil {
			return Result{}, errors.New("custom voice is not bound on this device")
		}
		request.Voice, err = s.ResolveVoice(character.VoiceProfile.ID)
		if err != nil {
			return Result{}, errors.New("custom voice is not bound on this device")
		}
	case domain.VoiceExternal:
		return Result{}, errors.New("external voice profiles require imported audio")
	default:
		return Result{}, errors.New("character voice profile is invalid")
	}
	run, err := s.newUniqueRun("speech_generate", episode.ID, "", block.ID, map[string]any{
		"model": request.Model, "parameters": request.Parameters, "voiceProfileId": character.VoiceProfile.ID,
	})
	if err != nil {
		return Result{}, err
	}
	if _, err := s.Approval.Request(ctx, approval.SpeechGenerate, "Generate paid character speech", map[string]any{"runId": run.ID, "episodeId": episode.ID, "scriptBlockId": block.ID, "character": character.Name}); err != nil {
		return Result{Run: s.failOrCancel(run, domain.RunCancelled, err)}, err
	}
	run, err = s.Store.TransitionRun(s.Root, run.ID, domain.RunRunning, "")
	if err != nil {
		return Result{}, err
	}
	var output bytes.Buffer
	result, err := s.Speech.GenerateSpeech(ctx, request, &output)
	if err != nil {
		return Result{Run: s.failOrCancel(run, domain.RunFailed, err)}, err
	}
	if output.Len() == 0 {
		return Result{Run: s.failOrCancel(run, domain.RunFailed, errors.New("provider returned empty speech"))}, errors.New("provider returned empty speech")
	}
	s.runsMu.Lock()
	defer s.runsMu.Unlock()
	asset, err := s.persistBytes(run, domain.AssetKindAudio, "."+safeExtension(result.Format, "wav"), output.Bytes(), s.Speech.Name(), "", result.Model, request.Parameters, nil, result.ProviderRequestID)
	if err != nil {
		return Result{Run: s.failOrCancel(run, domain.RunFailed, err)}, err
	}
	if _, err := s.Store.SelectVoiceAsset(s.Root, episode.ID, block.ID, asset.ID); err != nil {
		return Result{}, err
	}
	now := time.Now().UTC()
	job := provider.Job{ID: run.ID, Kind: "speech", Status: provider.JobSucceeded, Progress: 100, ProviderRequestID: result.ProviderRequestID, CreatedAt: now, UpdatedAt: now}
	completed, err := s.completeRun(run.ID)
	return Result{Run: completed, Job: job, Asset: &asset}, err
}

func (s *Service) GetRun(ctx context.Context, runID string) (Result, error) {
	s.runsMu.Lock()
	defer s.runsMu.Unlock()
	run, err := s.findRun(runID)
	if err != nil {
		return Result{}, err
	}
	if asset, found := s.assetForRun(runID); found {
		return Result{Run: run, Job: provider.Job{ID: run.ProviderJobID, Kind: string(asset.Kind), Status: provider.JobSucceeded, Progress: 100, UpdatedAt: run.UpdatedAt}, Asset: &asset}, nil
	}
	if isTerminal(run.Status) {
		return Result{Run: run}, nil
	}
	if run.ProviderJobID == "" || run.Operation != "video_generate" {
		return Result{Run: run}, nil
	}
	if s.Video == nil {
		return Result{Run: run}, errors.New("video provider is unavailable")
	}
	job, err := s.Video.GetVideoJob(ctx, run.ProviderJobID)
	if err != nil {
		return Result{Run: run}, err
	}
	run.Progress = job.Progress
	if err := s.Store.SaveRun(s.Root, run); err != nil {
		return Result{Run: run, Job: job}, err
	}
	switch job.Status {
	case provider.JobSucceeded:
		prompt, _ := run.Metadata["prompt"].(string)
		model, _ := run.Metadata["model"].(string)
		parameters, _ := run.Metadata["parameters"].(map[string]any)
		inputs, resolveErr := s.inputsFromMetadata(run.Metadata)
		if resolveErr != nil {
			return Result{Run: run, Job: job}, resolveErr
		}
		asset, persistErr := s.persistDownloaded(ctx, run, job, domain.AssetKindVideo, ".mp4", s.Video.Name(), prompt, model, parameters, inputs, s.Video.DownloadVideo)
		if persistErr != nil {
			return Result{Run: s.failOrCancel(run, domain.RunFailed, persistErr), Job: job}, persistErr
		}
		if _, err := s.Store.SelectVideoVersion(s.Root, run.ShotID, asset.ID); err != nil {
			return Result{}, err
		}
		run, err = s.completeRun(run.ID)
		if err != nil {
			return Result{Run: run, Job: job, Asset: &asset}, err
		}
		return Result{Run: run, Job: job, Asset: &asset}, nil
	case provider.JobFailed:
		run = s.failOrCancel(run, domain.RunFailed, errors.New(job.Error))
	case provider.JobCancelled:
		run = s.failOrCancel(run, domain.RunCancelled, errors.New("provider job cancelled"))
	}
	return Result{Run: run, Job: job}, nil
}

func (s *Service) CancelRun(ctx context.Context, runID string) (Result, error) {
	s.runsMu.Lock()
	run, err := s.findRun(runID)
	if err != nil {
		s.runsMu.Unlock()
		return Result{}, err
	}
	if run.Status == domain.RunSucceeded || run.Status == domain.RunFailed || run.Status == domain.RunCancelled {
		s.runsMu.Unlock()
		return Result{}, fmt.Errorf("run %s is already terminal", run.ID)
	}
	if run.Operation != "video_generate" || s.Video == nil {
		s.runsMu.Unlock()
		return Result{}, errors.New("only active provider video jobs can be cancelled here")
	}
	if strings.TrimSpace(run.ProviderJobID) == "" {
		s.runsMu.Unlock()
		return Result{Run: run}, errors.New("video provider job has not started yet")
	}
	s.runsMu.Unlock()
	if _, err := s.Approval.Request(ctx, approval.JobCancel, "Cancel a paid generation job", map[string]any{"runId": run.ID}); err != nil {
		return Result{Run: run}, err
	}
	s.runsMu.Lock()
	defer s.runsMu.Unlock()
	run, err = s.findRun(runID)
	if err != nil {
		return Result{}, err
	}
	if isTerminal(run.Status) {
		return Result{Run: run}, fmt.Errorf("run %s completed while cancellation was awaiting approval", run.ID)
	}
	job, err := s.Video.CancelVideoJob(ctx, run.ProviderJobID)
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

func (s *Service) findShot(shotID string) (domain.Snapshot, domain.Shot, error) {
	if err := project.ValidateID(shotID); err != nil {
		return domain.Snapshot{}, domain.Shot{}, err
	}
	snapshot, err := s.Store.Open(s.Root)
	if err != nil {
		return domain.Snapshot{}, domain.Shot{}, err
	}
	for _, shot := range snapshot.Shots {
		if shot.ID == shotID {
			return snapshot, shot, nil
		}
	}
	return domain.Snapshot{}, domain.Shot{}, fmt.Errorf("shot %s not found", shotID)
}

func (s *Service) findDialogue(episodeID, blockID string) (domain.Snapshot, domain.Episode, domain.ScriptBlock, domain.Character, error) {
	snapshot, err := s.Store.Open(s.Root)
	if err != nil {
		return domain.Snapshot{}, domain.Episode{}, domain.ScriptBlock{}, domain.Character{}, err
	}
	var episode domain.Episode
	found := false
	for _, value := range snapshot.Episodes {
		if value.ID == episodeID {
			episode, found = value, true
			break
		}
	}
	if !found {
		return snapshot, episode, domain.ScriptBlock{}, domain.Character{}, fmt.Errorf("episode %s not found", episodeID)
	}
	var block domain.ScriptBlock
	found = false
	for _, value := range episode.ScriptBlocks {
		if value.ID == blockID {
			block, found = value, true
			break
		}
	}
	if !found || (block.Kind != domain.ScriptDialogue && block.Kind != domain.ScriptVoiceOver) {
		return snapshot, episode, block, domain.Character{}, fmt.Errorf("dialogue block %s not found", blockID)
	}
	for _, character := range snapshot.Characters {
		if character.ID == block.CharacterID {
			return snapshot, episode, block, character, nil
		}
	}
	return snapshot, episode, block, domain.Character{}, fmt.Errorf("character %s not found", block.CharacterID)
}

func (s *Service) imageReferences(snapshot domain.Snapshot, shot domain.Shot, maximum int) ([]provider.Reference, error) {
	var candidates []referenceCandidate
	add := func(ids []string, role string, priority int) {
		for _, id := range ids {
			candidates = append(candidates, referenceCandidate{id, role, priority})
		}
	}
	add(snapshot.Project.StyleBible.ReferenceAssets, "style", 10)
	for _, id := range shot.CharacterIDs {
		for _, value := range snapshot.Characters {
			if value.ID == id {
				add(value.ReferenceAssets, "character:"+id, 100)
			}
		}
	}
	for _, scene := range snapshot.Scenes {
		if scene.ID == shot.SceneID {
			for _, value := range snapshot.Locations {
				if value.ID == scene.LocationID {
					add(value.ReferenceAssets, "location:"+value.ID, 80)
				}
			}
		}
	}
	for _, id := range shot.PropIDs {
		for _, value := range snapshot.Props {
			if value.ID == id {
				add(value.ReferenceAssets, "prop:"+id, 70)
			}
		}
	}
	add(shot.ReferenceAssets, "shot_reference", 90)
	sort.SliceStable(candidates, func(i, j int) bool { return candidates[i].priority > candidates[j].priority })
	return s.resolveReferences(snapshot, candidates, maximum)
}

func (s *Service) videoReferences(snapshot domain.Snapshot, shot domain.Shot, requested []provider.Reference, capabilities provider.Capabilities) ([]provider.Reference, error) {
	if capabilities.MaxVideoReferences <= 0 || len(capabilities.VideoReferenceRoles) == 0 {
		return nil, nil
	}
	allowed := make(map[string]bool)
	for _, role := range capabilities.VideoReferenceRoles {
		allowed[role] = true
	}
	if allowed["keyframe"] && shot.SelectedKeyframeAssetID == "" {
		return nil, errors.New("select a keyframe before generating video")
	}
	candidates := make([]provider.Reference, 0, len(requested)+2)
	if shot.SelectedKeyframeAssetID != "" {
		candidates = append(candidates, provider.Reference{AssetID: shot.SelectedKeyframeAssetID, Role: "keyframe"})
	}
	candidates = append(candidates, requested...)
	if previous, ok := previousEpisodeShot(snapshot, shot); ok && previous.SelectedVideoAssetID != "" {
		if tailID := extractedFrameAssetID(snapshot, previous.SelectedVideoAssetID, "tail"); tailID != "" {
			candidates = append(candidates, provider.Reference{AssetID: tailID, Role: "previous_tail"})
		}
	}
	result := make([]provider.Reference, 0, capabilities.MaxVideoReferences)
	seen := map[string]bool{}
	for _, reference := range candidates {
		if !allowed[reference.Role] {
			continue
		}
		key := reference.AssetID + "\x00" + reference.Role
		if seen[key] {
			continue
		}
		seen[key] = true
		asset, ok := findAsset(snapshot, reference.AssetID)
		if !ok {
			return nil, fmt.Errorf("video reference asset %s not found", reference.AssetID)
		}
		path, err := s.verifiedAssetPath(asset)
		if err != nil {
			return nil, err
		}
		reference.Path = path
		result = append(result, reference)
		if len(result) == capabilities.MaxVideoReferences {
			break
		}
	}
	return result, nil
}

func previousEpisodeShot(snapshot domain.Snapshot, current domain.Shot) (domain.Shot, bool) {
	sceneOrder := map[string]int{}
	for _, scene := range snapshot.Scenes {
		if scene.EpisodeID == current.EpisodeID {
			sceneOrder[scene.ID] = scene.Order
		}
	}
	shots := make([]domain.Shot, 0)
	for _, shot := range snapshot.Shots {
		if shot.EpisodeID == current.EpisodeID {
			shots = append(shots, shot)
		}
	}
	sort.SliceStable(shots, func(i, j int) bool {
		if sceneOrder[shots[i].SceneID] == sceneOrder[shots[j].SceneID] {
			return shots[i].Order < shots[j].Order
		}
		return sceneOrder[shots[i].SceneID] < sceneOrder[shots[j].SceneID]
	})
	for index, shot := range shots {
		if shot.ID == current.ID && index > 0 {
			return shots[index-1], true
		}
	}
	return domain.Shot{}, false
}

func extractedFrameAssetID(snapshot domain.Snapshot, sourceVideoID, role string) string {
	for _, asset := range snapshot.Assets {
		if asset.Kind != domain.AssetKindReference || asset.Provenance.Provider != "local-ffmpeg" || asset.Provenance.Parameters["frameRole"] != role {
			continue
		}
		for _, input := range asset.Inputs {
			if input.AssetID == sourceVideoID && input.Role == "source_video" {
				return asset.ID
			}
		}
	}
	return ""
}

func (s *Service) resolveReferences(snapshot domain.Snapshot, candidates []referenceCandidate, maximum int) ([]provider.Reference, error) {
	if maximum <= 0 {
		return nil, nil
	}
	seen := map[string]bool{}
	result := make([]provider.Reference, 0, maximum)
	for _, candidate := range candidates {
		if seen[candidate.id] {
			continue
		}
		asset, ok := findAsset(snapshot, candidate.id)
		if !ok {
			continue
		}
		path, err := s.verifiedAssetPath(asset)
		if err != nil {
			return nil, err
		}
		seen[candidate.id] = true
		result = append(result, provider.Reference{AssetID: candidate.id, Role: candidate.role, Path: path})
		if len(result) == maximum {
			break
		}
	}
	return result, nil
}

func (s *Service) verifiedAssetPath(asset domain.Asset) (string, error) {
	path, err := project.ResolveRelative(s.Root, asset.RelativePath)
	if err != nil {
		return "", err
	}
	hash, err := project.HashFile(path)
	if err != nil {
		return "", err
	}
	if !strings.EqualFold(hash, asset.SHA256) {
		return "", fmt.Errorf("asset %s failed SHA-256 verification", asset.ID)
	}
	return path, nil
}

func consistencyPrompt(snapshot domain.Snapshot, shot domain.Shot, prompt string) string {
	parts := []string{strings.TrimSpace(prompt)}
	if value := strings.TrimSpace(snapshot.Project.StyleBible.VisualStyle); value != "" {
		parts = append(parts, "Series visual style: "+value)
	}
	if value := strings.TrimSpace(snapshot.Project.StyleBible.LightingRules); value != "" {
		parts = append(parts, "Lighting rules: "+value)
	}
	for _, id := range shot.CharacterIDs {
		for _, value := range snapshot.Characters {
			if value.ID == id {
				parts = append(parts, fmt.Sprintf("Keep %s consistent: %s; wardrobe: %s", value.Name, value.Appearance, value.Wardrobe))
			}
		}
	}
	for _, scene := range snapshot.Scenes {
		if scene.ID == shot.SceneID {
			for _, value := range snapshot.Locations {
				if value.ID == scene.LocationID {
					parts = append(parts, "Location continuity: "+value.Description)
				}
			}
		}
	}
	if value := strings.TrimSpace(shot.WardrobeContinuity); value != "" {
		parts = append(parts, "Wardrobe state: "+value)
	}
	if value := strings.TrimSpace(shot.PropContinuity); value != "" {
		parts = append(parts, "Prop state: "+value)
	}
	if value := strings.TrimSpace(snapshot.Project.StyleBible.NegativePrompt); value != "" {
		parts = append(parts, "Avoid: "+value)
	}
	return strings.Join(parts, "\n")
}

func normalizeImageRequest(request provider.ImageRequest, shot domain.Shot) provider.ImageRequest {
	request.Model = defaultString(request.Model, provider.DefaultImageModel)
	if request.Size == "" {
		if shot.AspectRatio == "9:16" {
			request.Size = "1024x1536"
		} else {
			request.Size = "1536x1024"
		}
	}
	request.Quality = defaultString(request.Quality, "medium")
	request.Parameters = cloneParameters(request.Parameters)
	request.Parameters["size"], request.Parameters["quality"] = request.Size, request.Quality
	return request
}

func normalizeVideoRequest(request provider.VideoRequest, shot domain.Shot) provider.VideoRequest {
	request.Model = defaultString(request.Model, provider.DefaultVideoModel)
	if request.Seconds == 0 {
		request.Seconds = int(shot.DurationSeconds)
		if request.Seconds == 0 {
			request.Seconds = 4
		}
	}
	if request.Size == "" {
		if shot.AspectRatio == "9:16" {
			request.Size = "720x1280"
		} else {
			request.Size = "1280x720"
		}
	}
	request.Parameters = cloneParameters(request.Parameters)
	request.Parameters["seconds"], request.Parameters["size"] = request.Seconds, request.Size
	return request
}

func (s *Service) newRun(operation, episodeID, shotID, blockID string, metadata map[string]any) (domain.Run, error) {
	now := time.Now().UTC()
	run := domain.Run{SchemaVersion: domain.SchemaVersion, ID: uuid.NewString(), Operation: operation, Status: domain.RunAwaitingApproval, EpisodeID: episodeID, ShotID: shotID, ScriptBlockID: blockID, Metadata: metadata, CreatedAt: now, UpdatedAt: now}
	if err := s.Store.SaveRun(s.Root, run); err != nil {
		return domain.Run{}, err
	}
	_ = project.RebuildIndex(s.Root)
	return run, nil
}

func (s *Service) newUniqueRun(operation, episodeID, shotID, blockID string, metadata map[string]any) (domain.Run, error) {
	s.runsMu.Lock()
	defer s.runsMu.Unlock()
	current, err := s.Store.Open(s.Root)
	if err != nil {
		return domain.Run{}, err
	}
	for _, run := range current.Runs {
		if run.Operation == operation && run.EpisodeID == episodeID && run.ShotID == shotID && run.ScriptBlockID == blockID && !isTerminal(run.Status) {
			return domain.Run{}, fmt.Errorf("%s is already active for this target", operation)
		}
	}
	return s.newRun(operation, episodeID, shotID, blockID, metadata)
}

func (s *Service) persistDownloaded(ctx context.Context, run domain.Run, job provider.Job, kind domain.AssetKind, ext, providerName, prompt, model string, parameters map[string]any, inputs []domain.AssetInput, download func(context.Context, string, io.Writer) error) (domain.Asset, error) {
	if asset, found := s.assetForRun(run.ID); found {
		return asset, nil
	}
	var output bytes.Buffer
	if err := download(ctx, job.ID, &output); err != nil {
		return domain.Asset{}, err
	}
	return s.persistBytes(run, kind, ext, output.Bytes(), providerName, prompt, model, parameters, inputs, job.ProviderRequestID)
}

func (s *Service) persistBytes(run domain.Run, kind domain.AssetKind, ext string, content []byte, providerName, prompt, model string, parameters map[string]any, inputs []domain.AssetInput, requestID string) (domain.Asset, error) {
	if len(content) == 0 {
		return domain.Asset{}, errors.New("provider returned an empty asset")
	}
	assetID := uuid.NewString()
	relativePath := filepath.ToSlash(filepath.Join("assets", assetID, "result"+ext))
	destination, err := project.ResolveRelative(s.Root, relativePath)
	if err != nil {
		return domain.Asset{}, err
	}
	if err := project.AtomicWrite(destination, content, 0o644); err != nil {
		return domain.Asset{}, err
	}
	hash := sha256.Sum256(content)
	asset := domain.Asset{
		SchemaVersion: domain.SchemaVersion, ID: assetID, EpisodeID: run.EpisodeID, ShotID: run.ShotID, ScriptBlockID: run.ScriptBlockID,
		Kind: kind, RelativePath: relativePath, SHA256: hex.EncodeToString(hash[:]), Inputs: inputs, RunID: run.ID,
		Provenance: domain.Provenance{Provider: providerName, Model: model, Prompt: prompt, Parameters: parameters, ProviderRequestID: requestID, GeneratedAt: time.Now().UTC()},
		CreatedAt:  time.Now().UTC(),
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

func (s *Service) inputsFromMetadata(metadata map[string]any) ([]domain.AssetInput, error) {
	raw, ok := metadata["inputs"]
	if !ok {
		return nil, nil
	}
	encoded, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("encode run inputs: %w", err)
	}
	var result []domain.AssetInput
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&result); err != nil {
		return nil, fmt.Errorf("decode run inputs: %w", err)
	}
	for _, input := range result {
		if err := project.ValidateID(input.AssetID); err != nil || strings.TrimSpace(input.Role) == "" {
			return nil, errors.New("run contains an invalid asset input")
		}
	}
	return result, nil
}

func findAsset(snapshot domain.Snapshot, id string) (domain.Asset, bool) {
	for _, asset := range snapshot.Assets {
		if asset.ID == id {
			return asset, true
		}
	}
	return domain.Asset{}, false
}
func referencesToInputs(values []provider.Reference) []domain.AssetInput {
	result := make([]domain.AssetInput, 0, len(values))
	for _, value := range values {
		result = append(result, domain.AssetInput{AssetID: value.AssetID, Role: value.Role})
	}
	return result
}
func inputIDs(values []domain.AssetInput) []string {
	result := make([]string, len(values))
	for i, value := range values {
		result[i] = value.AssetID
	}
	return result
}
func cloneParameters(input map[string]any) map[string]any {
	output := make(map[string]any, len(input)+2)
	for key, value := range input {
		if sensitiveParameterKey(key) {
			continue
		}
		output[key] = sanitizeParameterValue(value)
	}
	return output
}

func sensitiveParameterKey(value string) bool {
	normalized := strings.NewReplacer("_", "", "-", "", ".", "").Replace(strings.ToLower(value))
	for _, forbidden := range []string{"apikey", "authorization", "accesstoken", "refreshtoken", "providervoiceid", "consentid", "consentrecording", "voicesample"} {
		if strings.Contains(normalized, forbidden) {
			return true
		}
	}
	return false
}

func sanitizeParameterValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return cloneParameters(typed)
	case []any:
		result := make([]any, len(typed))
		for index, item := range typed {
			result[index] = sanitizeParameterValue(item)
		}
		return result
	default:
		return typed
	}
}
func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
func safeExtension(value, fallback string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	for _, r := range value {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') {
			return fallback
		}
	}
	if value == "" {
		return fallback
	}
	return value
}

func (s *Service) completeRun(runID string) (domain.Run, error) {
	run, err := s.Store.TransitionRun(s.Root, runID, domain.RunSucceeded, "")
	if err != nil {
		return domain.Run{}, err
	}
	_ = project.RebuildIndex(s.Root)
	return run, nil
}

func isTerminal(status domain.RunStatus) bool {
	return status == domain.RunSucceeded || status == domain.RunFailed || status == domain.RunCancelled
}
