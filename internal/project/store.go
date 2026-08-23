package project

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/bg-dao/axon-codex-dramaops/internal/continuity"
	"github.com/bg-dao/axon-codex-dramaops/internal/domain"
	"github.com/google/uuid"
)

const ProjectManifest = "dramaops.project.json"

type CreateOptions struct {
	Name            string             `json:"name"`
	ContentLanguage string             `json:"contentLanguage"`
	Orientation     domain.Orientation `json:"orientation"`
}

type ImportOptions struct {
	Source        string              `json:"source"`
	EpisodeID     string              `json:"episodeId,omitempty"`
	ShotID        string              `json:"shotId,omitempty"`
	ScriptBlockID string              `json:"scriptBlockId,omitempty"`
	Kind          domain.AssetKind    `json:"kind"`
	Inputs        []domain.AssetInput `json:"inputs,omitempty"`
}

type ScriptPlan struct {
	Episode    domain.Episode     `json:"episode"`
	Scenes     []domain.Scene     `json:"scenes"`
	Characters []domain.Character `json:"characters,omitempty"`
	Locations  []domain.Location  `json:"locations,omitempty"`
	Props      []domain.Prop      `json:"props,omitempty"`
}

type Store struct {
	now func() time.Time
}

func NewStore() *Store { return &Store{now: func() time.Time { return time.Now().UTC() }} }

func (s *Store) Create(root, name string) (domain.Snapshot, error) {
	return s.CreateWithOptions(root, CreateOptions{Name: name, ContentLanguage: "zh-CN", Orientation: domain.OrientationPortrait})
}

func (s *Store) CreateWithOptions(root string, options CreateOptions) (domain.Snapshot, error) {
	if strings.TrimSpace(options.Name) == "" {
		return domain.Snapshot{}, errors.New("series name is required")
	}
	if options.Orientation == "" {
		options.Orientation = domain.OrientationPortrait
	}
	if options.Orientation != domain.OrientationPortrait && options.Orientation != domain.OrientationLandscape {
		return domain.Snapshot{}, fmt.Errorf("unsupported orientation %q", options.Orientation)
	}
	if strings.TrimSpace(options.ContentLanguage) == "" {
		options.ContentLanguage = "zh-CN"
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return domain.Snapshot{}, fmt.Errorf("resolve root: %w", err)
	}
	if err := os.MkdirAll(absRoot, 0o755); err != nil {
		return domain.Snapshot{}, fmt.Errorf("create project root: %w", err)
	}
	manifestPath, err := ResolveRelative(absRoot, ProjectManifest)
	if err != nil {
		return domain.Snapshot{}, err
	}
	if _, err := os.Lstat(manifestPath); err == nil {
		return domain.Snapshot{}, fmt.Errorf("a DramaOps series already exists at %s", absRoot)
	} else if !errors.Is(err, os.ErrNotExist) {
		return domain.Snapshot{}, fmt.Errorf("inspect project manifest: %w", err)
	}
	for _, rel := range []string{
		"episodes", "characters", "locations", "props", "scenes", "shots", "assets", "runs",
		"renders", "exports", ".dramaops", ".dramaops/approvals",
	} {
		path, pathErr := ResolveRelative(absRoot, rel)
		if pathErr != nil {
			return domain.Snapshot{}, pathErr
		}
		if err := os.MkdirAll(path, 0o755); err != nil {
			return domain.Snapshot{}, fmt.Errorf("create %s: %w", rel, err)
		}
	}
	now := s.now()
	episodeID := "episode-001"
	manifest := domain.Project{
		SchemaVersion: domain.SchemaVersion,
		ID:            uuid.NewString(), Name: strings.TrimSpace(options.Name),
		ContentLanguage: strings.TrimSpace(options.ContentLanguage), ActiveEpisodeID: episodeID,
		SoundPalette: domain.SoundPalette{Motifs: map[string]string{}},
		Output:       domain.DefaultOutputSettings(options.Orientation),
		CreatedAt:    now, UpdatedAt: now,
	}
	if err := s.SaveProject(absRoot, manifest); err != nil {
		return domain.Snapshot{}, err
	}
	episode := domain.Episode{
		SchemaVersion: domain.SchemaVersion, ID: episodeID, Number: 1,
		Title: localize(options.ContentLanguage, "第一集", "Episode 1"), Status: domain.EpisodeDraft,
		SceneIDs: []string{}, ScriptBlocks: []domain.ScriptBlock{}, CreatedAt: now, UpdatedAt: now,
	}
	if err := s.SaveEpisode(absRoot, episode); err != nil {
		return domain.Snapshot{}, err
	}
	if err := s.SaveEdit(absRoot, domain.EpisodeEdit{
		SchemaVersion: domain.SchemaVersion, EpisodeID: episodeID,
		VideoTrack: []domain.VideoClip{}, AudioCues: []domain.AudioCue{}, SubtitleCues: []domain.SubtitleCue{},
		Output: manifest.Output, UpdatedAt: now,
	}); err != nil {
		return domain.Snapshot{}, err
	}
	if err := writeIfMissing(filepath.Join(absRoot, "AGENTS.md"), projectAgentInstructions()); err != nil {
		return domain.Snapshot{}, err
	}
	if err := RebuildIndex(absRoot); err != nil {
		return domain.Snapshot{}, err
	}
	return s.Open(absRoot)
}

func localize(language, zh, en string) string {
	if strings.HasPrefix(strings.ToLower(language), "zh") {
		return zh
	}
	return en
}

func (s *Store) Open(root string) (domain.Snapshot, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return domain.Snapshot{}, fmt.Errorf("resolve root: %w", err)
	}
	projectPath, err := ResolveRelative(absRoot, ProjectManifest)
	if err != nil {
		return domain.Snapshot{}, err
	}
	var manifest domain.Project
	if err := readJSON(projectPath, &manifest); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			legacy := filepath.Join(absRoot, "scene"+"ops.project.json")
			if _, legacyErr := os.Lstat(legacy); legacyErr == nil {
				return domain.Snapshot{}, errors.New("unsupported legacy project format; create a new DramaOps series")
			}
		}
		return domain.Snapshot{}, fmt.Errorf("read DramaOps project manifest: %w", err)
	}
	if err := requireSchema(manifest.SchemaVersion); err != nil {
		return domain.Snapshot{}, err
	}
	snapshot := domain.Snapshot{
		Root: absRoot, Project: manifest,
		Episodes: []domain.Episode{}, Characters: []domain.Character{}, Locations: []domain.Location{}, Props: []domain.Prop{},
		Scenes: []domain.Scene{}, Shots: []domain.Shot{}, Edits: []domain.EpisodeEdit{}, Assets: []domain.Asset{}, Runs: []domain.Run{},
	}
	loads := []func() error{
		func() error {
			return loadJSONGlob(filepath.Join(absRoot, "episodes", "*", "episode.json"), &snapshot.Episodes)
		},
		func() error {
			return loadJSONGlob(filepath.Join(absRoot, "episodes", "*", "edit.json"), &snapshot.Edits)
		},
		func() error {
			return loadJSONGlob(filepath.Join(absRoot, "characters", "*.json"), &snapshot.Characters)
		},
		func() error { return loadJSONGlob(filepath.Join(absRoot, "locations", "*.json"), &snapshot.Locations) },
		func() error { return loadJSONGlob(filepath.Join(absRoot, "props", "*.json"), &snapshot.Props) },
		func() error { return loadJSONGlob(filepath.Join(absRoot, "scenes", "*.json"), &snapshot.Scenes) },
		func() error { return loadJSONGlob(filepath.Join(absRoot, "shots", "*.json"), &snapshot.Shots) },
		func() error {
			return loadJSONGlob(filepath.Join(absRoot, "assets", "*", "asset.json"), &snapshot.Assets)
		},
		func() error { return loadJSONGlob(filepath.Join(absRoot, "runs", "*.json"), &snapshot.Runs) },
	}
	for _, load := range loads {
		if err := load(); err != nil {
			return domain.Snapshot{}, err
		}
	}
	if err := validateSnapshotSchemas(snapshot); err != nil {
		return domain.Snapshot{}, err
	}
	if err := validateSnapshotSemantics(snapshot); err != nil {
		return domain.Snapshot{}, err
	}
	sortSnapshot(&snapshot)
	snapshot.ContinuityIssues = continuity.Check(snapshot)
	return snapshot, nil
}

func validateSnapshotSchemas(snapshot domain.Snapshot) error {
	check := func(kind, id string, version int) error {
		if version != domain.SchemaVersion {
			return fmt.Errorf("%s %s: unsupported schemaVersion %d", kind, id, version)
		}
		return nil
	}
	for _, value := range snapshot.Episodes {
		if err := check("episode", value.ID, value.SchemaVersion); err != nil {
			return err
		}
	}
	for _, value := range snapshot.Characters {
		if err := check("character", value.ID, value.SchemaVersion); err != nil {
			return err
		}
	}
	for _, value := range snapshot.Locations {
		if err := check("location", value.ID, value.SchemaVersion); err != nil {
			return err
		}
	}
	for _, value := range snapshot.Props {
		if err := check("prop", value.ID, value.SchemaVersion); err != nil {
			return err
		}
	}
	for _, value := range snapshot.Scenes {
		if err := check("scene", value.ID, value.SchemaVersion); err != nil {
			return err
		}
	}
	for _, value := range snapshot.Shots {
		if err := check("shot", value.ID, value.SchemaVersion); err != nil {
			return err
		}
	}
	for _, value := range snapshot.Edits {
		if err := check("edit", value.EpisodeID, value.SchemaVersion); err != nil {
			return err
		}
	}
	for _, value := range snapshot.Assets {
		if err := check("asset", value.ID, value.SchemaVersion); err != nil {
			return err
		}
	}
	for _, value := range snapshot.Runs {
		if err := check("run", value.ID, value.SchemaVersion); err != nil {
			return err
		}
	}
	return nil
}

func sortSnapshot(snapshot *domain.Snapshot) {
	sort.Slice(snapshot.Episodes, func(i, j int) bool { return snapshot.Episodes[i].Number < snapshot.Episodes[j].Number })
	sort.Slice(snapshot.Scenes, func(i, j int) bool {
		if snapshot.Scenes[i].EpisodeID == snapshot.Scenes[j].EpisodeID {
			return snapshot.Scenes[i].Order < snapshot.Scenes[j].Order
		}
		return snapshot.Scenes[i].EpisodeID < snapshot.Scenes[j].EpisodeID
	})
	sort.Slice(snapshot.Shots, func(i, j int) bool {
		if snapshot.Shots[i].SceneID == snapshot.Shots[j].SceneID {
			return snapshot.Shots[i].Order < snapshot.Shots[j].Order
		}
		return snapshot.Shots[i].SceneID < snapshot.Shots[j].SceneID
	})
	sort.Slice(snapshot.Assets, func(i, j int) bool { return snapshot.Assets[i].CreatedAt.Before(snapshot.Assets[j].CreatedAt) })
	sort.Slice(snapshot.Runs, func(i, j int) bool { return snapshot.Runs[i].CreatedAt.After(snapshot.Runs[j].CreatedAt) })
}

func (s *Store) SaveProject(root string, value domain.Project) error {
	if value.SchemaVersion == 0 {
		value.SchemaVersion = domain.SchemaVersion
	}
	if err := requireSchema(value.SchemaVersion); err != nil {
		return err
	}
	if value.Output.Width == 0 {
		value.Output = domain.DefaultOutputSettings(domain.OrientationPortrait)
	}
	if err := validateProject(value); err != nil {
		return err
	}
	value.UpdatedAt = s.now()
	path, err := ResolveRelative(root, ProjectManifest)
	if err != nil {
		return err
	}
	return AtomicWriteJSON(path, value)
}

func (s *Store) SaveEpisode(root string, value domain.Episode) error {
	if err := ValidateID(value.ID); err != nil {
		return err
	}
	if value.SchemaVersion == 0 {
		value.SchemaVersion = domain.SchemaVersion
	}
	if err := requireSchema(value.SchemaVersion); err != nil {
		return err
	}
	if strings.TrimSpace(value.Title) == "" {
		return errors.New("episode title is required")
	}
	if value.Number < 1 {
		return errors.New("episode number must be positive")
	}
	if value.Status == "" {
		value.Status = domain.EpisodeDraft
	}
	seen := make(map[string]struct{}, len(value.ScriptBlocks))
	for i := range value.ScriptBlocks {
		block := &value.ScriptBlocks[i]
		if block.ID == "" {
			block.ID = stableScriptBlockID(value.ID, i)
		}
		if err := ValidateID(block.ID); err != nil {
			return fmt.Errorf("script block: %w", err)
		}
		if _, exists := seen[block.ID]; exists {
			return fmt.Errorf("duplicate script block id %q", block.ID)
		}
		seen[block.ID] = struct{}{}
		if err := validateScriptBlock(*block); err != nil {
			return err
		}
		block.Order = i
	}
	if err := validateEpisode(value); err != nil {
		return err
	}
	if value.CreatedAt.IsZero() {
		value.CreatedAt = s.now()
	}
	value.UpdatedAt = s.now()
	path, err := ResolveRelative(root, filepath.Join("episodes", value.ID, "episode.json"))
	if err != nil {
		return err
	}
	return AtomicWriteJSON(path, value)
}

func stableScriptBlockID(episodeID string, index int) string {
	return fmt.Sprintf("%s-block-%03d", episodeID, index+1)
}

func validateScriptBlock(block domain.ScriptBlock) error {
	switch block.Kind {
	case domain.ScriptAction, domain.ScriptDialogue, domain.ScriptVoiceOver, domain.ScriptSFX, domain.ScriptMusic:
	default:
		return fmt.Errorf("script block %s has unsupported kind %q", block.ID, block.Kind)
	}
	if strings.TrimSpace(block.Text) == "" {
		return fmt.Errorf("script block %s text is required", block.ID)
	}
	if (block.Kind == domain.ScriptDialogue || block.Kind == domain.ScriptVoiceOver) && block.CharacterID == "" {
		return fmt.Errorf("script block %s requires a character", block.ID)
	}
	return nil
}

func (s *Store) SaveEdit(root string, value domain.EpisodeEdit) error {
	if err := ValidateID(value.EpisodeID); err != nil {
		return err
	}
	if value.SchemaVersion == 0 {
		value.SchemaVersion = domain.SchemaVersion
	}
	if err := requireSchema(value.SchemaVersion); err != nil {
		return err
	}
	if value.Output.Width == 0 {
		value.Output = domain.DefaultOutputSettings(domain.OrientationPortrait)
	}
	if err := validateEdit(value); err != nil {
		return err
	}
	if err := validateOutput(value.Output); err != nil {
		return err
	}
	value.UpdatedAt = s.now()
	path, err := ResolveRelative(root, filepath.Join("episodes", value.EpisodeID, "edit.json"))
	if err != nil {
		return err
	}
	return AtomicWriteJSON(path, value)
}

func validateEdit(edit domain.EpisodeEdit) error {
	seen := map[string]bool{}
	for i, clip := range edit.VideoTrack {
		if err := ValidateID(clip.ID); err != nil {
			return fmt.Errorf("video clip: %w", err)
		}
		if err := ValidateID(clip.ShotID); err != nil {
			return fmt.Errorf("video clip shot: %w", err)
		}
		if err := ValidateID(clip.AssetID); err != nil {
			return fmt.Errorf("video clip asset: %w", err)
		}
		if clip.Order != i {
			return fmt.Errorf("video track order must be contiguous at clip %s", clip.ID)
		}
		if seen[clip.ID] {
			return fmt.Errorf("duplicate edit cue id %s", clip.ID)
		}
		seen[clip.ID] = true
		if clip.InSeconds < 0 || clip.OutSeconds <= clip.InSeconds {
			return fmt.Errorf("invalid trim range for clip %s", clip.ID)
		}
		if clip.Fit != domain.FitCover && clip.Fit != domain.FitContain {
			return fmt.Errorf("invalid fit for clip %s", clip.ID)
		}
		switch clip.Transition {
		case domain.TransitionCut, domain.TransitionDissolve, domain.TransitionFade:
		default:
			return fmt.Errorf("invalid transition for clip %s", clip.ID)
		}
		if clip.TransitionSeconds < 0 || clip.TransitionSeconds >= clip.OutSeconds-clip.InSeconds {
			return fmt.Errorf("invalid transition duration for clip %s", clip.ID)
		}
	}
	for _, cue := range edit.AudioCues {
		if err := ValidateID(cue.ID); err != nil {
			return fmt.Errorf("audio cue: %w", err)
		}
		if err := ValidateID(cue.AssetID); err != nil {
			return fmt.Errorf("audio cue asset: %w", err)
		}
		if cue.ScriptBlockID != "" {
			if err := ValidateID(cue.ScriptBlockID); err != nil {
				return fmt.Errorf("audio cue script block: %w", err)
			}
		}
		if seen[cue.ID] {
			return fmt.Errorf("duplicate edit cue id %s", cue.ID)
		}
		seen[cue.ID] = true
		if cue.StartSeconds < 0 || cue.DurationSeconds <= 0 {
			return fmt.Errorf("invalid audio cue timing %s", cue.ID)
		}
		switch cue.Lane {
		case domain.LaneDialogue, domain.LaneSFX, domain.LaneBGM:
		default:
			return fmt.Errorf("invalid audio lane for cue %s", cue.ID)
		}
	}
	for _, cue := range edit.SubtitleCues {
		if err := ValidateID(cue.ID); err != nil {
			return fmt.Errorf("subtitle cue: %w", err)
		}
		if cue.ScriptBlockID != "" {
			if err := ValidateID(cue.ScriptBlockID); err != nil {
				return fmt.Errorf("subtitle script block: %w", err)
			}
		}
		if seen[cue.ID] {
			return fmt.Errorf("duplicate edit cue id %s", cue.ID)
		}
		seen[cue.ID] = true
		if cue.StartSeconds < 0 || cue.DurationSeconds <= 0 || strings.TrimSpace(cue.Text) == "" {
			return fmt.Errorf("invalid subtitle cue %s", cue.ID)
		}
	}
	return nil
}

func (s *Store) SaveCharacter(root string, value domain.Character) error {
	if err := prepareBibleEntity(&value.SchemaVersion, value.ID, &value.CreatedAt, &value.UpdatedAt, s.now); err != nil {
		return err
	}
	if strings.TrimSpace(value.Name) == "" {
		return errors.New("character name is required")
	}
	if value.VoiceProfile.ID == "" {
		value.VoiceProfile.ID = "voice-" + value.ID
	}
	if value.VoiceProfile.Kind == "" {
		value.VoiceProfile.Kind = domain.VoiceBuiltIn
	}
	if value.VoiceProfile.Kind == domain.VoiceBuiltIn && value.VoiceProfile.BuiltInVoice == "" {
		value.VoiceProfile.BuiltInVoice = "alloy"
	}
	if err := validateCharacter(value); err != nil {
		return err
	}
	path, err := ResolveRelative(root, filepath.Join("characters", value.ID+".json"))
	if err != nil {
		return err
	}
	return AtomicWriteJSON(path, value)
}

func (s *Store) SaveLocation(root string, value domain.Location) error {
	if err := prepareBibleEntity(&value.SchemaVersion, value.ID, &value.CreatedAt, &value.UpdatedAt, s.now); err != nil {
		return err
	}
	if strings.TrimSpace(value.Name) == "" {
		return errors.New("location name is required")
	}
	path, err := ResolveRelative(root, filepath.Join("locations", value.ID+".json"))
	if err != nil {
		return err
	}
	return AtomicWriteJSON(path, value)
}

func (s *Store) SaveProp(root string, value domain.Prop) error {
	if err := prepareBibleEntity(&value.SchemaVersion, value.ID, &value.CreatedAt, &value.UpdatedAt, s.now); err != nil {
		return err
	}
	if strings.TrimSpace(value.Name) == "" {
		return errors.New("prop name is required")
	}
	path, err := ResolveRelative(root, filepath.Join("props", value.ID+".json"))
	if err != nil {
		return err
	}
	return AtomicWriteJSON(path, value)
}

func prepareBibleEntity(version *int, id string, created, updated *time.Time, now func() time.Time) error {
	if err := ValidateID(id); err != nil {
		return err
	}
	if *version == 0 {
		*version = domain.SchemaVersion
	}
	if err := requireSchema(*version); err != nil {
		return err
	}
	if created.IsZero() {
		*created = now()
	}
	*updated = now()
	return nil
}

func (s *Store) SaveScene(root string, value domain.Scene) error {
	if err := ValidateID(value.ID); err != nil {
		return err
	}
	if err := ValidateID(value.EpisodeID); err != nil {
		return fmt.Errorf("episode id: %w", err)
	}
	if value.SchemaVersion == 0 {
		value.SchemaVersion = domain.SchemaVersion
	}
	if err := requireSchema(value.SchemaVersion); err != nil {
		return err
	}
	if value.CreatedAt.IsZero() {
		value.CreatedAt = s.now()
	}
	if err := validateScene(value); err != nil {
		return err
	}
	value.UpdatedAt = s.now()
	path, err := ResolveRelative(root, filepath.Join("scenes", value.ID+".json"))
	if err != nil {
		return err
	}
	return AtomicWriteJSON(path, value)
}

func (s *Store) SaveShot(root string, value domain.Shot) error {
	if err := ValidateID(value.ID); err != nil {
		return err
	}
	if err := ValidateID(value.EpisodeID); err != nil {
		return fmt.Errorf("episode id: %w", err)
	}
	if err := ValidateID(value.SceneID); err != nil {
		return fmt.Errorf("scene id: %w", err)
	}
	if value.SchemaVersion == 0 {
		value.SchemaVersion = domain.SchemaVersion
	}
	if err := requireSchema(value.SchemaVersion); err != nil {
		return err
	}
	applyShotDefaults(&value)
	if err := validateShot(value); err != nil {
		return err
	}
	if value.CreatedAt.IsZero() {
		value.CreatedAt = s.now()
	}
	value.UpdatedAt = s.now()
	path, err := ResolveRelative(root, filepath.Join("shots", value.ID+".json"))
	if err != nil {
		return err
	}
	return AtomicWriteJSON(path, value)
}

func applyShotDefaults(value *domain.Shot) {
	if value.DurationSeconds <= 0 {
		value.DurationSeconds = 4
	}
	if value.AspectRatio == "" {
		value.AspectRatio = "9:16"
	}
	if value.ShotSize == "" {
		value.ShotSize = domain.ShotMS
	}
	if value.CameraAngle == "" {
		value.CameraAngle = domain.AngleEyeLevel
	}
	if value.CameraMovement == "" {
		value.CameraMovement = domain.MovementStatic
	}
	if value.Transition == "" {
		value.Transition = domain.TransitionCut
	}
}

func (s *Store) SaveAsset(root string, value domain.Asset) error {
	if value.SchemaVersion == 0 {
		value.SchemaVersion = domain.SchemaVersion
	}
	if err := requireSchema(value.SchemaVersion); err != nil {
		return err
	}
	if value.CreatedAt.IsZero() {
		value.CreatedAt = s.now()
	}
	if err := validateAsset(root, value); err != nil {
		return err
	}
	path, err := ResolveRelative(root, filepath.Join("assets", value.ID, "asset.json"))
	if err != nil {
		return err
	}
	return AtomicWriteJSON(path, value)
}

func (s *Store) SaveRun(root string, value domain.Run) error {
	if value.SchemaVersion == 0 {
		value.SchemaVersion = domain.SchemaVersion
	}
	if err := requireSchema(value.SchemaVersion); err != nil {
		return err
	}
	if value.CreatedAt.IsZero() {
		value.CreatedAt = s.now()
	}
	if err := validateRun(value); err != nil {
		return err
	}
	value.UpdatedAt = s.now()
	path, err := ResolveRelative(root, filepath.Join("runs", value.ID+".json"))
	if err != nil {
		return err
	}
	return AtomicWriteJSON(path, value)
}

func (s *Store) TransitionRun(root, runID string, next domain.RunStatus, message string) (domain.Run, error) {
	if err := ValidateID(runID); err != nil {
		return domain.Run{}, err
	}
	path, err := ResolveRelative(root, filepath.Join("runs", runID+".json"))
	if err != nil {
		return domain.Run{}, err
	}
	var run domain.Run
	if err := readJSON(path, &run); err != nil {
		return domain.Run{}, err
	}
	if !domain.CanTransitionRun(run.Status, next) {
		return domain.Run{}, fmt.Errorf("invalid run transition %s -> %s", run.Status, next)
	}
	run.Status, run.Error = next, message
	if err := s.SaveRun(root, run); err != nil {
		return domain.Run{}, err
	}
	return run, nil
}

func (s *Store) ApplyScript(root string, plan ScriptPlan) (domain.Snapshot, error) {
	existing, err := s.Open(root)
	if err != nil {
		return domain.Snapshot{}, err
	}
	var current *domain.Episode
	for i := range existing.Episodes {
		if existing.Episodes[i].ID == plan.Episode.ID {
			current = &existing.Episodes[i]
			break
		}
	}
	if current == nil {
		return domain.Snapshot{}, fmt.Errorf("episode %s not found", plan.Episode.ID)
	}
	if len(current.ScriptBlocks) > 0 || len(current.SceneIDs) > 0 {
		return domain.Snapshot{}, errors.New("episode script already exists; edit it directly instead")
	}
	if len(plan.Scenes) == 0 || len(plan.Episode.ScriptBlocks) == 0 {
		return domain.Snapshot{}, errors.New("script requires at least one scene and one block")
	}
	plan.Episode.SchemaVersion = domain.SchemaVersion
	plan.Episode.Number, plan.Episode.CreatedAt = current.Number, current.CreatedAt
	if plan.Episode.Status == "" {
		plan.Episode.Status = domain.EpisodePlanning
	}
	plan.Episode.SceneIDs = make([]string, 0, len(plan.Scenes))
	sceneIDs := make(map[string]struct{}, len(plan.Scenes))
	for i := range plan.Scenes {
		scene := &plan.Scenes[i]
		if scene.ID == "" {
			scene.ID = fmt.Sprintf("%s-scene-%02d", plan.Episode.ID, i+1)
		}
		if err := ValidateID(scene.ID); err != nil {
			return domain.Snapshot{}, err
		}
		if _, duplicate := sceneIDs[scene.ID]; duplicate {
			return domain.Snapshot{}, fmt.Errorf("duplicate scene id %q", scene.ID)
		}
		sceneIDs[scene.ID] = struct{}{}
		scene.SchemaVersion, scene.EpisodeID, scene.Order = domain.SchemaVersion, plan.Episode.ID, i
		scene.ShotIDs = []string{}
		plan.Episode.SceneIDs = append(plan.Episode.SceneIDs, scene.ID)
	}
	for _, block := range plan.Episode.ScriptBlocks {
		if _, ok := sceneIDs[block.SceneID]; !ok {
			return domain.Snapshot{}, fmt.Errorf("script block %s references unknown scene %s", block.ID, block.SceneID)
		}
	}
	for i := range plan.Characters {
		if err := s.SaveCharacter(root, plan.Characters[i]); err != nil {
			return domain.Snapshot{}, err
		}
	}
	for i := range plan.Locations {
		if err := s.SaveLocation(root, plan.Locations[i]); err != nil {
			return domain.Snapshot{}, err
		}
	}
	for i := range plan.Props {
		if err := s.SaveProp(root, plan.Props[i]); err != nil {
			return domain.Snapshot{}, err
		}
	}
	for i := range plan.Scenes {
		if err := s.SaveScene(root, plan.Scenes[i]); err != nil {
			return domain.Snapshot{}, err
		}
	}
	if err := s.SaveEpisode(root, plan.Episode); err != nil {
		return domain.Snapshot{}, err
	}
	if err := RebuildIndex(root); err != nil {
		return domain.Snapshot{}, err
	}
	return s.Open(root)
}

func (s *Store) ApplyShotPlan(root, episodeID string, shots []domain.Shot) (domain.Snapshot, error) {
	existing, err := s.Open(root)
	if err != nil {
		return domain.Snapshot{}, err
	}
	for _, shot := range existing.Shots {
		if shot.EpisodeID == episodeID {
			return domain.Snapshot{}, errors.New("episode shot plan already exists; edit shots directly instead")
		}
	}
	if len(shots) == 0 {
		return domain.Snapshot{}, errors.New("shot plan requires at least one shot")
	}
	scenes := make(map[string]domain.Scene)
	for _, scene := range existing.Scenes {
		if scene.EpisodeID == episodeID {
			scenes[scene.ID] = scene
		}
	}
	if len(scenes) == 0 {
		return domain.Snapshot{}, fmt.Errorf("episode %s has no script scenes", episodeID)
	}
	order := make(map[string]int)
	ids := make(map[string]struct{})
	for i := range shots {
		shot := &shots[i]
		if shot.ID == "" {
			shot.ID = fmt.Sprintf("%s-shot-%03d", episodeID, i+1)
		}
		if err := ValidateID(shot.ID); err != nil {
			return domain.Snapshot{}, err
		}
		if _, duplicate := ids[shot.ID]; duplicate {
			return domain.Snapshot{}, fmt.Errorf("duplicate shot id %q", shot.ID)
		}
		ids[shot.ID] = struct{}{}
		scene, ok := scenes[shot.SceneID]
		if !ok {
			return domain.Snapshot{}, fmt.Errorf("shot %s references unknown scene %s", shot.ID, shot.SceneID)
		}
		shot.SchemaVersion, shot.EpisodeID, shot.Order = domain.SchemaVersion, episodeID, order[shot.SceneID]
		order[shot.SceneID]++
		applyShotDefaults(shot)
		scene.ShotIDs = append(scene.ShotIDs, shot.ID)
		scenes[scene.ID] = scene
		if err := validateShot(*shot); err != nil {
			return domain.Snapshot{}, err
		}
	}
	// Validate the complete plan before the first durable write so a malformed
	// later shot cannot leave a partially applied shot list behind.
	for _, shot := range shots {
		if err := s.SaveShot(root, shot); err != nil {
			return domain.Snapshot{}, err
		}
	}
	for _, scene := range scenes {
		if err := s.SaveScene(root, scene); err != nil {
			return domain.Snapshot{}, err
		}
	}
	if err := RebuildIndex(root); err != nil {
		return domain.Snapshot{}, err
	}
	return s.Open(root)
}

func (s *Store) SelectKeyframeVersion(root, shotID, assetID string) (domain.Shot, error) {
	return s.selectShotAsset(root, shotID, assetID, domain.AssetKindImage)
}

func (s *Store) SelectVideoVersion(root, shotID, assetID string) (domain.Shot, error) {
	return s.selectShotAsset(root, shotID, assetID, domain.AssetKindVideo)
}

func (s *Store) selectShotAsset(root, shotID, assetID string, kind domain.AssetKind) (domain.Shot, error) {
	if err := ValidateID(shotID); err != nil {
		return domain.Shot{}, err
	}
	if err := ValidateID(assetID); err != nil {
		return domain.Shot{}, err
	}
	snapshot, err := s.Open(root)
	if err != nil {
		return domain.Shot{}, err
	}
	valid := false
	for _, asset := range snapshot.Assets {
		if asset.ID == assetID && asset.ShotID == shotID && asset.Kind == kind {
			valid = true
			break
		}
	}
	if !valid {
		return domain.Shot{}, fmt.Errorf("asset %s is not a %s version of shot %s", assetID, kind, shotID)
	}
	for _, shot := range snapshot.Shots {
		if shot.ID != shotID {
			continue
		}
		if kind == domain.AssetKindImage {
			shot.SelectedKeyframeAssetID = assetID
		} else {
			shot.SelectedVideoAssetID = assetID
		}
		if err := s.SaveShot(root, shot); err != nil {
			return domain.Shot{}, err
		}
		return shot, nil
	}
	return domain.Shot{}, fmt.Errorf("shot %s not found", shotID)
}

func (s *Store) SelectVoiceAsset(root, episodeID, blockID, assetID string) (domain.Episode, error) {
	snapshot, err := s.Open(root)
	if err != nil {
		return domain.Episode{}, err
	}
	valid := false
	for _, asset := range snapshot.Assets {
		if asset.ID == assetID && asset.EpisodeID == episodeID && asset.ScriptBlockID == blockID && asset.Kind == domain.AssetKindAudio {
			valid = true
			break
		}
	}
	if !valid {
		return domain.Episode{}, fmt.Errorf("asset %s is not voice audio for block %s", assetID, blockID)
	}
	for _, episode := range snapshot.Episodes {
		if episode.ID != episodeID {
			continue
		}
		for i := range episode.ScriptBlocks {
			if episode.ScriptBlocks[i].ID == blockID {
				episode.ScriptBlocks[i].SelectedVoiceAssetID = assetID
				if err := s.SaveEpisode(root, episode); err != nil {
					return domain.Episode{}, err
				}
				return episode, nil
			}
		}
		return domain.Episode{}, fmt.Errorf("script block %s not found", blockID)
	}
	return domain.Episode{}, fmt.Errorf("episode %s not found", episodeID)
}

func (s *Store) AddReferenceAsset(root, shotID, assetID string) (domain.Shot, error) {
	snapshot, err := s.Open(root)
	if err != nil {
		return domain.Shot{}, err
	}
	valid := false
	for _, asset := range snapshot.Assets {
		if asset.ID == assetID && asset.ShotID == shotID && asset.Kind == domain.AssetKindReference {
			valid = true
			break
		}
	}
	if !valid {
		return domain.Shot{}, fmt.Errorf("asset %s is not a reference for shot %s", assetID, shotID)
	}
	for _, shot := range snapshot.Shots {
		if shot.ID != shotID {
			continue
		}
		for _, existing := range shot.ReferenceAssets {
			if existing == assetID {
				return shot, nil
			}
		}
		shot.ReferenceAssets = append(shot.ReferenceAssets, assetID)
		if err := s.SaveShot(root, shot); err != nil {
			return domain.Shot{}, err
		}
		return shot, nil
	}
	return domain.Shot{}, fmt.Errorf("shot %s not found", shotID)
}

func (s *Store) ImportAsset(root string, options ImportOptions) (domain.Asset, error) {
	if !supportedAssetKind(options.Kind) {
		return domain.Asset{}, fmt.Errorf("unsupported asset kind %q", options.Kind)
	}
	snapshot, err := s.Open(root)
	if err != nil {
		return domain.Asset{}, err
	}
	if options.EpisodeID != "" && !episodeExists(snapshot, options.EpisodeID) {
		return domain.Asset{}, fmt.Errorf("episode %s not found", options.EpisodeID)
	}
	if options.ShotID != "" && !shotExists(snapshot, options.ShotID) {
		return domain.Asset{}, fmt.Errorf("shot %s not found", options.ShotID)
	}
	if options.ScriptBlockID != "" && !scriptBlockExists(snapshot, options.ScriptBlockID) {
		return domain.Asset{}, fmt.Errorf("script block %s not found", options.ScriptBlockID)
	}
	for _, input := range options.Inputs {
		if !assetExists(snapshot, input.AssetID) {
			return domain.Asset{}, fmt.Errorf("input asset %s not found", input.AssetID)
		}
	}
	input, err := os.Open(options.Source)
	if err != nil {
		return domain.Asset{}, fmt.Errorf("open imported asset: %w", err)
	}
	defer input.Close()
	assetID := uuid.NewString()
	ext := strings.ToLower(filepath.Ext(options.Source))
	if ext == "" || len(ext) > 12 {
		ext = ".bin"
	}
	rel := filepath.Join("assets", assetID, "source"+ext)
	destination, err := ResolveRelative(root, rel)
	if err != nil {
		return domain.Asset{}, err
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return domain.Asset{}, err
	}
	tmp, err := os.CreateTemp(filepath.Dir(destination), ".dramaops-import-*")
	if err != nil {
		return domain.Asset{}, err
	}
	tmpName := tmp.Name()
	hash := sha256.New()
	if _, err := io.Copy(io.MultiWriter(tmp, hash), input); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return domain.Asset{}, err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return domain.Asset{}, err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return domain.Asset{}, err
	}
	if err := os.Rename(tmpName, destination); err != nil {
		_ = os.Remove(tmpName)
		return domain.Asset{}, err
	}
	asset := domain.Asset{
		SchemaVersion: domain.SchemaVersion, ID: assetID, EpisodeID: options.EpisodeID, ShotID: options.ShotID,
		ScriptBlockID: options.ScriptBlockID, Kind: options.Kind, RelativePath: filepath.ToSlash(rel),
		SHA256: hex.EncodeToString(hash.Sum(nil)), Inputs: options.Inputs,
		Provenance: domain.Provenance{Provider: "external-import", GeneratedAt: s.now()}, CreatedAt: s.now(),
	}
	if err := s.SaveAsset(root, asset); err != nil {
		return domain.Asset{}, err
	}
	if options.Kind == domain.AssetKindReference && options.ShotID != "" {
		if _, err := s.AddReferenceAsset(root, options.ShotID, asset.ID); err != nil {
			return domain.Asset{}, err
		}
	}
	if options.Kind == domain.AssetKindVideo && options.ShotID != "" {
		if _, err := s.SelectVideoVersion(root, options.ShotID, asset.ID); err != nil {
			return domain.Asset{}, err
		}
	}
	if options.Kind == domain.AssetKindAudio && options.ScriptBlockID != "" {
		if _, err := s.SelectVoiceAsset(root, options.EpisodeID, options.ScriptBlockID, asset.ID); err != nil {
			return domain.Asset{}, err
		}
	}
	if err := RebuildIndex(root); err != nil {
		return domain.Asset{}, err
	}
	return asset, nil
}

func supportedAssetKind(kind domain.AssetKind) bool {
	switch kind {
	case domain.AssetKindImage, domain.AssetKindVideo, domain.AssetKindReference, domain.AssetKindAudio, domain.AssetKindSubtitle, domain.AssetKindRender:
		return true
	default:
		return false
	}
}

func episodeExists(snapshot domain.Snapshot, id string) bool {
	for _, value := range snapshot.Episodes {
		if value.ID == id {
			return true
		}
	}
	return false
}
func shotExists(snapshot domain.Snapshot, id string) bool {
	for _, value := range snapshot.Shots {
		if value.ID == id {
			return true
		}
	}
	return false
}
func scriptBlockExists(snapshot domain.Snapshot, id string) bool {
	for _, episode := range snapshot.Episodes {
		for _, block := range episode.ScriptBlocks {
			if block.ID == id {
				return true
			}
		}
	}
	return false
}
func assetExists(snapshot domain.Snapshot, id string) bool {
	for _, value := range snapshot.Assets {
		if value.ID == id {
			return true
		}
	}
	return false
}

func HashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func requireSchema(version int) error {
	if version != domain.SchemaVersion {
		return fmt.Errorf("unsupported schemaVersion %d", version)
	}
	return nil
}

func readJSON(path string, output any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(output); err != nil {
		return fmt.Errorf("decode %s: %w", path, err)
	}
	return nil
}

func loadJSONGlob[T any](pattern string, output *[]T) error {
	paths, err := filepath.Glob(pattern)
	if err != nil {
		return err
	}
	for _, path := range paths {
		var value T
		if err := readJSON(path, &value); err != nil {
			return err
		}
		*output = append(*output, value)
	}
	return nil
}

func writeIfMissing(path, content string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return AtomicWrite(path, []byte(content), 0o644)
}

func projectAgentInstructions() string {
	return "# DramaOps Project Instructions\n\n" +
		"- Treat `dramaops.project.json`, episode manifests, bibles, shots, assets, edits, and runs as durable project data.\n" +
		"- Read the series through `dramaops_project_read`.\n" +
		"- Use `dramaops_script_apply` and `dramaops_shotplan_apply` for initial structured writes.\n" +
		"- Ask for explicit approval before agent writes, paid generation, or cancellation.\n" +
		"- Never expose credentials or write outside this project root.\n"
}
