package appapi

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/bg-dao/axon-codex-dramaops/internal/domain"
	"github.com/bg-dao/axon-codex-dramaops/internal/project"
	"github.com/bg-dao/axon-codex-dramaops/internal/redact"
	renderengine "github.com/bg-dao/axon-codex-dramaops/internal/render"
	"github.com/google/uuid"
)

type RenderAPI struct{ backend *Backend }

type TimelineValidation struct {
	Valid    bool                     `json:"valid"`
	Issues   []domain.ContinuityIssue `json:"issues"`
	Duration float64                  `json:"durationSeconds"`
}

func NewRenderAPI(backend *Backend) *RenderAPI { return &RenderAPI{backend: backend} }

func (a *RenderAPI) Runtime() renderengine.RuntimeStatus {
	a.backend.mu.Lock()
	defer a.backend.mu.Unlock()
	if a.backend.renderRuntime.Compatible {
		return a.backend.renderRuntime
	}
	config, _ := os.UserConfigDir()
	private := ""
	if config != "" {
		private = filepath.Join(config, "DramaOps", "runtime", "ffmpeg", "current", "bin")
	}
	ctx, cancel := context.WithTimeout(a.backend.context(), 15*time.Second)
	defer cancel()
	a.backend.renderRuntime = renderengine.DetectRuntime(ctx, private)
	return a.backend.renderRuntime
}

func (a *RenderAPI) ProbeAsset(assetID string) (domain.MediaInfo, error) {
	root, err := a.backend.Root()
	if err != nil {
		return domain.MediaInfo{}, err
	}
	snapshot, err := a.backend.store.Open(root)
	if err != nil {
		return domain.MediaInfo{}, err
	}
	runtime := a.Runtime()
	if err := runtime.Require(); err != nil {
		return domain.MediaInfo{}, err
	}
	for _, asset := range snapshot.Assets {
		if asset.ID != assetID {
			continue
		}
		path, err := project.ResolveRelative(root, asset.RelativePath)
		if err != nil {
			return domain.MediaInfo{}, err
		}
		info, err := renderengine.Probe(a.backend.context(), runtime, path)
		if err != nil {
			return domain.MediaInfo{}, err
		}
		if err := renderengine.ValidateMedia(info, asset.Kind); err != nil {
			return domain.MediaInfo{}, err
		}
		asset.MediaInfo = info
		if err := a.backend.store.SaveAsset(root, asset); err != nil {
			return domain.MediaInfo{}, err
		}
		_ = project.RebuildIndex(root)
		return info, nil
	}
	return domain.MediaInfo{}, fmt.Errorf("asset %s not found", assetID)
}

func (a *RenderAPI) BuildTimeline(episodeID string) (domain.EpisodeEdit, error) {
	root, err := a.backend.Root()
	if err != nil {
		return domain.EpisodeEdit{}, err
	}
	snapshot, err := a.backend.store.Open(root)
	if err != nil {
		return domain.EpisodeEdit{}, err
	}
	runtime := a.Runtime()
	if err := runtime.Require(); err != nil {
		return domain.EpisodeEdit{}, err
	}
	shots := orderedEpisodeShots(snapshot, episodeID)
	if len(shots) == 0 {
		return domain.EpisodeEdit{}, errors.New("episode has no shots")
	}
	assetByID := map[string]domain.Asset{}
	for _, asset := range snapshot.Assets {
		assetByID[asset.ID] = asset
	}
	track := make([]domain.VideoClip, 0, len(shots))
	starts := map[string]float64{}
	cursor := 0.0
	for i, shot := range shots {
		asset, ok := assetByID[shot.SelectedVideoAssetID]
		if !ok || asset.Kind != domain.AssetKindVideo {
			return domain.EpisodeEdit{}, fmt.Errorf("shot %s has no selected video", shot.Title)
		}
		if asset.MediaInfo.DurationSeconds <= 0 {
			path, err := project.ResolveRelative(root, asset.RelativePath)
			if err != nil {
				return domain.EpisodeEdit{}, err
			}
			asset.MediaInfo, err = renderengine.Probe(a.backend.context(), runtime, path)
			if err != nil {
				return domain.EpisodeEdit{}, err
			}
			if err := renderengine.ValidateMedia(asset.MediaInfo, domain.AssetKindVideo); err != nil {
				return domain.EpisodeEdit{}, err
			}
			if err := a.backend.store.SaveAsset(root, asset); err != nil {
				return domain.EpisodeEdit{}, err
			}
			assetByID[asset.ID] = asset
		}
		duration := shot.DurationSeconds
		if duration <= 0 || duration > asset.MediaInfo.DurationSeconds {
			duration = asset.MediaInfo.DurationSeconds
		}
		transition, transitionSeconds := shot.Transition, 0.0
		if i == 0 {
			transition = domain.TransitionCut
		}
		if transition == domain.TransitionDissolve || transition == domain.TransitionFade {
			transitionSeconds = 0.3
		}
		starts[shot.ID] = cursor
		track = append(track, domain.VideoClip{ID: fmt.Sprintf("clip-%03d", i+1), ShotID: shot.ID, AssetID: asset.ID, Order: i, InSeconds: 0, OutSeconds: duration, Transition: transition, TransitionSeconds: transitionSeconds, Fit: domain.FitCover})
		cursor += duration - transitionSeconds
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
		return domain.EpisodeEdit{}, fmt.Errorf("episode %s not found", episodeID)
	}
	blockShot := map[string]string{}
	for _, shot := range shots {
		for _, blockID := range shot.ScriptBlockIDs {
			if blockShot[blockID] == "" {
				blockShot[blockID] = shot.ID
			}
		}
	}
	var audio []domain.AudioCue
	var subtitles []domain.SubtitleCue
	dialogueCursor := 0.0
	for _, block := range episode.ScriptBlocks {
		if block.Kind != domain.ScriptDialogue && block.Kind != domain.ScriptVoiceOver {
			continue
		}
		start := starts[blockShot[block.ID]]
		if dialogueCursor > start {
			start = dialogueCursor
		}
		duration := 2.5
		if asset, ok := assetByID[block.SelectedVoiceAssetID]; ok && asset.Kind == domain.AssetKindAudio {
			if asset.MediaInfo.DurationSeconds <= 0 {
				if path, resolveErr := project.ResolveRelative(root, asset.RelativePath); resolveErr == nil {
					if info, probeErr := renderengine.Probe(a.backend.context(), runtime, path); probeErr == nil {
						asset.MediaInfo = info
						_ = a.backend.store.SaveAsset(root, asset)
						assetByID[asset.ID] = asset
					}
				}
			}
			if asset.MediaInfo.DurationSeconds > 0 {
				duration = asset.MediaInfo.DurationSeconds
			}
			audio = append(audio, domain.AudioCue{ID: "audio-" + block.ID, Lane: domain.LaneDialogue, AssetID: asset.ID, ScriptBlockID: block.ID, StartSeconds: start, DurationSeconds: duration, DuckBGM: true})
		}
		subtitles = append(subtitles, domain.SubtitleCue{ID: "subtitle-" + block.ID, ScriptBlockID: block.ID, StartSeconds: start, DurationSeconds: duration, Text: block.Text})
		dialogueCursor = start + duration
	}
	edit := domain.EpisodeEdit{SchemaVersion: domain.SchemaVersion, EpisodeID: episodeID, VideoTrack: track, AudioCues: audio, SubtitleCues: subtitles, Output: snapshot.Project.Output, UpdatedAt: time.Now().UTC()}
	if err := a.backend.store.SaveEdit(root, edit); err != nil {
		return domain.EpisodeEdit{}, err
	}
	_ = project.RebuildIndex(root)
	a.backend.emit(EventProjectChanged, map[string]any{"root": root})
	return edit, nil
}

func (a *RenderAPI) Validate(episodeID string) (TimelineValidation, error) {
	root, err := a.backend.Root()
	if err != nil {
		return TimelineValidation{}, err
	}
	snapshot, err := a.backend.store.Open(root)
	if err != nil {
		return TimelineValidation{}, err
	}
	edit, err := findEdit(snapshot, episodeID)
	if err != nil {
		return TimelineValidation{}, err
	}
	issues := make([]domain.ContinuityIssue, 0)
	for _, value := range snapshot.ContinuityIssues {
		if value.EpisodeID == "" || value.EpisodeID == episodeID {
			issues = append(issues, value)
		}
	}
	duration := 0.0
	for _, clip := range edit.VideoTrack {
		duration += clip.OutSeconds - clip.InSeconds - clip.TransitionSeconds
	}
	valid := len(edit.VideoTrack) > 0
	for _, issue := range issues {
		if issue.Severity == domain.ContinuityError {
			valid = false
		}
	}
	return TimelineValidation{Valid: valid, Issues: issues, Duration: duration}, nil
}

func (a *RenderAPI) Start(episodeID string) (domain.Run, error) { return a.start(episodeID, "") }

func (a *RenderAPI) start(episodeID, recoveredFrom string) (domain.Run, error) {
	root, err := a.backend.Root()
	if err != nil {
		return domain.Run{}, err
	}
	snapshot, err := a.backend.store.Open(root)
	if err != nil {
		return domain.Run{}, err
	}
	edit, err := findEdit(snapshot, episodeID)
	if err != nil {
		return domain.Run{}, err
	}
	for _, run := range snapshot.Runs {
		if run.Operation == "episode_render" && run.EpisodeID == episodeID && (run.Status == domain.RunQueued || run.Status == domain.RunRunning) {
			return domain.Run{}, errors.New("episode render is already running")
		}
	}
	runtime := a.Runtime()
	if err := runtime.Require(); err != nil {
		return domain.Run{}, err
	}
	now := time.Now().UTC()
	metadata := map[string]any{"episodeId": episodeID}
	if recoveredFrom != "" {
		metadata["recoveredFrom"] = recoveredFrom
	}
	run := domain.Run{SchemaVersion: domain.SchemaVersion, ID: uuid.NewString(), Operation: "episode_render", Status: domain.RunQueued, EpisodeID: episodeID, Metadata: metadata, CreatedAt: now, UpdatedAt: now}
	if err := a.backend.store.SaveRun(root, run); err != nil {
		return domain.Run{}, err
	}
	run, err = a.backend.store.TransitionRun(root, run.ID, domain.RunRunning, "")
	if err != nil {
		return domain.Run{}, err
	}
	a.launch(root, snapshot, edit, run, runtime)
	return run, nil
}

func (a *RenderAPI) launch(root string, snapshot domain.Snapshot, edit domain.EpisodeEdit, run domain.Run, runtime renderengine.RuntimeStatus) {
	ctx, cancel := context.WithCancel(a.backend.context())
	a.backend.mu.Lock()
	a.backend.renderCancels[run.ID] = cancel
	a.backend.mu.Unlock()
	go func() {
		defer func() { a.backend.mu.Lock(); delete(a.backend.renderCancels, run.ID); a.backend.mu.Unlock() }()
		outputRel := filepath.ToSlash(filepath.Join("renders", fmt.Sprintf("%s-%s.mp4", edit.EpisodeID, run.ID)))
		srtRel := filepath.ToSlash(filepath.Join("renders", fmt.Sprintf("%s-%s.srt", edit.EpisodeID, run.ID)))
		outputPath, _ := project.ResolveRelative(root, outputRel)
		srtPath, _ := project.ResolveRelative(root, srtRel)
		engine := &renderengine.Engine{Runtime: runtime}
		lastProgress := -1
		result, err := engine.Render(ctx, renderengine.Request{Root: root, Snapshot: snapshot, Edit: edit, OutputPath: outputPath, SRTPath: srtPath}, func(progress renderengine.Progress) {
			if progress.Percent == lastProgress {
				return
			}
			lastProgress = progress.Percent
			run.Progress = progress.Percent
			_ = a.backend.store.SaveRun(root, run)
			a.backend.emit(EventRenderProgress, map[string]any{"runId": run.ID, "episodeId": run.EpisodeID, "progress": progress})
		})
		if err != nil {
			status := domain.RunFailed
			if errors.Is(err, context.Canceled) {
				status = domain.RunCancelled
			}
			_, _ = a.backend.store.TransitionRun(root, run.ID, status, redact.String(err.Error()))
			_ = project.RebuildIndex(root)
			a.backend.emit(EventRunUpdated, map[string]any{"runId": run.ID, "status": status})
			return
		}
		hash, hashErr := project.HashFile(result.Path)
		if hashErr != nil {
			_, _ = a.backend.store.TransitionRun(root, run.ID, domain.RunFailed, hashErr.Error())
			return
		}
		inputs := renderInputs(edit)
		asset := domain.Asset{SchemaVersion: domain.SchemaVersion, ID: uuid.NewString(), EpisodeID: edit.EpisodeID, Kind: domain.AssetKindRender, RelativePath: outputRel, SHA256: hash, Inputs: inputs, RunID: run.ID, MediaInfo: result.MediaInfo, Provenance: domain.Provenance{Provider: "local-ffmpeg", Parameters: map[string]any{"arguments": result.Arguments, "output": edit.Output}, ToolVersion: result.Version, GeneratedAt: time.Now().UTC()}, CreatedAt: time.Now().UTC()}
		if err := a.backend.store.SaveAsset(root, asset); err != nil {
			_, _ = a.backend.store.TransitionRun(root, run.ID, domain.RunFailed, err.Error())
			return
		}
		if _, err := a.backend.store.TransitionRun(root, run.ID, domain.RunSucceeded, ""); err == nil {
			for _, episode := range snapshot.Episodes {
				if episode.ID == edit.EpisodeID {
					episode.Status = domain.EpisodeComplete
					_ = a.backend.store.SaveEpisode(root, episode)
					break
				}
			}
		}
		_ = project.RebuildIndex(root)
		a.backend.emit(EventProjectChanged, map[string]any{"root": root})
		a.backend.emit(EventRunUpdated, map[string]any{"runId": run.ID, "status": domain.RunSucceeded})
	}()
}

func (a *RenderAPI) Cancel(runID string) error {
	a.backend.mu.RLock()
	cancel := a.backend.renderCancels[runID]
	a.backend.mu.RUnlock()
	if cancel == nil {
		return fmt.Errorf("render run %s is not active", runID)
	}
	cancel()
	return nil
}

func (a *RenderAPI) Locate(assetID string) (string, error) {
	root, err := a.backend.Root()
	if err != nil {
		return "", err
	}
	snapshot, err := a.backend.store.Open(root)
	if err != nil {
		return "", err
	}
	for _, asset := range snapshot.Assets {
		if asset.ID == assetID && asset.Kind == domain.AssetKindRender {
			return project.ResolveRelative(root, asset.RelativePath)
		}
	}
	return "", fmt.Errorf("render asset %s not found", assetID)
}

func (a *RenderAPI) recover(root string) {
	snapshot, err := a.backend.store.Open(root)
	if err != nil {
		return
	}
	for _, run := range snapshot.Runs {
		if run.Operation != "episode_render" || run.Status != domain.RunRunning {
			continue
		}
		if _, err := a.backend.store.TransitionRun(root, run.ID, domain.RunFailed, "render interrupted by application restart; recovering with a new run"); err != nil {
			continue
		}
		_, _ = a.start(run.EpisodeID, run.ID)
	}
}

func orderedEpisodeShots(snapshot domain.Snapshot, episodeID string) []domain.Shot {
	sceneOrder := map[string]int{}
	for _, scene := range snapshot.Scenes {
		if scene.EpisodeID == episodeID {
			sceneOrder[scene.ID] = scene.Order
		}
	}
	shots := make([]domain.Shot, 0)
	for _, shot := range snapshot.Shots {
		if shot.EpisodeID == episodeID {
			shots = append(shots, shot)
		}
	}
	sort.Slice(shots, func(i, j int) bool {
		if sceneOrder[shots[i].SceneID] == sceneOrder[shots[j].SceneID] {
			return shots[i].Order < shots[j].Order
		}
		return sceneOrder[shots[i].SceneID] < sceneOrder[shots[j].SceneID]
	})
	return shots
}

func findEdit(snapshot domain.Snapshot, episodeID string) (domain.EpisodeEdit, error) {
	for _, edit := range snapshot.Edits {
		if edit.EpisodeID == episodeID {
			return edit, nil
		}
	}
	return domain.EpisodeEdit{}, fmt.Errorf("episode edit %s not found", episodeID)
}

func renderInputs(edit domain.EpisodeEdit) []domain.AssetInput {
	result := []domain.AssetInput{}
	seen := map[string]bool{}
	for _, clip := range edit.VideoTrack {
		if !seen[clip.AssetID] {
			seen[clip.AssetID] = true
			result = append(result, domain.AssetInput{AssetID: clip.AssetID, Role: "video"})
		}
	}
	for _, cue := range edit.AudioCues {
		if !seen[cue.AssetID] {
			seen[cue.AssetID] = true
			result = append(result, domain.AssetInput{AssetID: cue.AssetID, Role: string(cue.Lane)})
		}
	}
	return result
}
