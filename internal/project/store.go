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

	"github.com/bg-dao/axon-codex-sceneops/internal/domain"
	"github.com/google/uuid"
)

const ProjectManifest = "sceneops.project.json"

const MaxBriefBytes = 1 << 20

type Store struct {
	now func() time.Time
}

func NewStore() *Store { return &Store{now: func() time.Time { return time.Now().UTC() }} }

func (s *Store) Create(root, name string) (domain.Snapshot, error) {
	if strings.TrimSpace(name) == "" {
		return domain.Snapshot{}, errors.New("project name is required")
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return domain.Snapshot{}, fmt.Errorf("resolve root: %w", err)
	}
	manifestPath := filepath.Join(absRoot, ProjectManifest)
	if _, err := os.Lstat(manifestPath); err == nil {
		return domain.Snapshot{}, fmt.Errorf("a SceneOps project already exists at %s", absRoot)
	} else if !errors.Is(err, os.ErrNotExist) {
		return domain.Snapshot{}, fmt.Errorf("inspect project manifest: %w", err)
	}
	if err := os.MkdirAll(absRoot, 0o755); err != nil {
		return domain.Snapshot{}, fmt.Errorf("create project root: %w", err)
	}
	for _, rel := range []string{"scenes", "shots", "assets", "runs", "exports", ".sceneops", ".sceneops/approvals"} {
		path, err := ResolveRelative(absRoot, rel)
		if err != nil {
			return domain.Snapshot{}, err
		}
		if err := os.MkdirAll(path, 0o755); err != nil {
			return domain.Snapshot{}, fmt.Errorf("create %s: %w", rel, err)
		}
	}
	now := s.now()
	manifest := domain.Project{
		SchemaVersion: domain.SchemaVersion,
		ID:            uuid.NewString(),
		Name:          strings.TrimSpace(name),
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := s.SaveProject(absRoot, manifest); err != nil {
		return domain.Snapshot{}, err
	}
	if err := writeIfMissing(filepath.Join(absRoot, "brief.md"), "# Creative brief\n\n"); err != nil {
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
		return domain.Snapshot{}, fmt.Errorf("read project manifest: %w", err)
	}
	if err := requireSchema(manifest.SchemaVersion); err != nil {
		return domain.Snapshot{}, err
	}
	snapshot := domain.Snapshot{
		Root:    absRoot,
		Project: manifest,
		Scenes:  []domain.Scene{},
		Shots:   []domain.Shot{},
		Assets:  []domain.Asset{},
		Runs:    []domain.Run{},
	}
	briefPath, err := ResolveRelative(absRoot, "brief.md")
	if err != nil {
		return domain.Snapshot{}, err
	}
	brief, err := os.ReadFile(briefPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return domain.Snapshot{}, fmt.Errorf("read creative brief: %w", err)
	}
	if len(brief) > MaxBriefBytes {
		return domain.Snapshot{}, fmt.Errorf("creative brief exceeds %d bytes", MaxBriefBytes)
	}
	snapshot.Brief = string(brief)
	if err := loadJSONGlob(filepath.Join(absRoot, "scenes", "*.json"), &snapshot.Scenes); err != nil {
		return domain.Snapshot{}, err
	}
	if err := loadJSONGlob(filepath.Join(absRoot, "shots", "*.json"), &snapshot.Shots); err != nil {
		return domain.Snapshot{}, err
	}
	if err := loadJSONGlob(filepath.Join(absRoot, "assets", "*", "asset.json"), &snapshot.Assets); err != nil {
		return domain.Snapshot{}, err
	}
	if err := loadJSONGlob(filepath.Join(absRoot, "runs", "*.json"), &snapshot.Runs); err != nil {
		return domain.Snapshot{}, err
	}
	sort.Slice(snapshot.Scenes, func(i, j int) bool { return snapshot.Scenes[i].Order < snapshot.Scenes[j].Order })
	sort.Slice(snapshot.Shots, func(i, j int) bool {
		if snapshot.Shots[i].SceneID == snapshot.Shots[j].SceneID {
			return snapshot.Shots[i].Order < snapshot.Shots[j].Order
		}
		return snapshot.Shots[i].SceneID < snapshot.Shots[j].SceneID
	})
	sort.Slice(snapshot.Assets, func(i, j int) bool { return snapshot.Assets[i].CreatedAt.Before(snapshot.Assets[j].CreatedAt) })
	sort.Slice(snapshot.Runs, func(i, j int) bool { return snapshot.Runs[i].CreatedAt.After(snapshot.Runs[j].CreatedAt) })
	return snapshot, nil
}

func (s *Store) SaveProject(root string, value domain.Project) error {
	if value.SchemaVersion == 0 {
		value.SchemaVersion = domain.SchemaVersion
	}
	if err := requireSchema(value.SchemaVersion); err != nil {
		return err
	}
	value.UpdatedAt = s.now()
	path, err := ResolveRelative(root, ProjectManifest)
	if err != nil {
		return err
	}
	return AtomicWriteJSON(path, value)
}

func (s *Store) SaveBrief(root, markdown string) error {
	if len(markdown) > MaxBriefBytes {
		return fmt.Errorf("creative brief exceeds %d bytes", MaxBriefBytes)
	}
	path, err := ResolveRelative(root, "brief.md")
	if err != nil {
		return err
	}
	return AtomicWrite(path, []byte(markdown), 0o644)
}

func (s *Store) SaveScene(root string, value domain.Scene) error {
	if err := ValidateID(value.ID); err != nil {
		return err
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
	if err := ValidateID(value.SceneID); err != nil {
		return fmt.Errorf("scene id: %w", err)
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
	value.UpdatedAt = s.now()
	path, err := ResolveRelative(root, filepath.Join("shots", value.ID+".json"))
	if err != nil {
		return err
	}
	return AtomicWriteJSON(path, value)
}

func (s *Store) SaveAsset(root string, value domain.Asset) error {
	if err := ValidateID(value.ID); err != nil {
		return err
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
	if _, err := ResolveRelative(root, value.RelativePath); err != nil {
		return fmt.Errorf("asset path: %w", err)
	}
	path, err := ResolveRelative(root, filepath.Join("assets", value.ID, "asset.json"))
	if err != nil {
		return err
	}
	return AtomicWriteJSON(path, value)
}

func (s *Store) SelectImageVersion(root, shotID, assetID string) (domain.Shot, error) {
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
	validAsset := false
	for _, asset := range snapshot.Assets {
		if asset.ID == assetID && asset.ShotID == shotID && asset.Kind == domain.AssetKindImage {
			validAsset = true
			break
		}
	}
	if !validAsset {
		return domain.Shot{}, fmt.Errorf("asset %s is not an image version of shot %s", assetID, shotID)
	}
	for _, shot := range snapshot.Shots {
		if shot.ID == shotID {
			shot.SelectedAssetID = assetID
			if err := s.SaveShot(root, shot); err != nil {
				return domain.Shot{}, err
			}
			return shot, nil
		}
	}
	return domain.Shot{}, fmt.Errorf("shot %s not found", shotID)
}

func (s *Store) AddReferenceAsset(root, shotID, assetID string) (domain.Shot, error) {
	snapshot, err := s.Open(root)
	if err != nil {
		return domain.Shot{}, err
	}
	validAsset := false
	for _, asset := range snapshot.Assets {
		if asset.ID == assetID && asset.ShotID == shotID && asset.Kind == domain.AssetKindReference {
			validAsset = true
			break
		}
	}
	if !validAsset {
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

func (s *Store) SaveRun(root string, value domain.Run) error {
	if err := ValidateID(value.ID); err != nil {
		return err
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
	run.Status = next
	run.Error = message
	if err := s.SaveRun(root, run); err != nil {
		return domain.Run{}, err
	}
	return run, nil
}

func (s *Store) ApplyStoryboard(root string, scenes []domain.Scene, shots []domain.Shot) (domain.Snapshot, error) {
	existing, err := s.Open(root)
	if err != nil {
		return domain.Snapshot{}, err
	}
	if len(existing.Scenes) > 0 || len(existing.Shots) > 0 {
		return domain.Snapshot{}, errors.New("storyboard already exists; edit existing scenes and shots instead")
	}
	if len(scenes) == 0 || len(shots) == 0 {
		return domain.Snapshot{}, errors.New("storyboard requires at least one scene and one shot")
	}
	now := s.now()
	sceneIDs := make(map[string]struct{}, len(scenes))
	for i := range scenes {
		if scenes[i].ID == "" {
			scenes[i].ID = uuid.NewString()
		}
		scenes[i].SchemaVersion = domain.SchemaVersion
		scenes[i].Order = i
		scenes[i].ShotIDs = nil
		if scenes[i].CreatedAt.IsZero() {
			scenes[i].CreatedAt = now
		}
		if err := ValidateID(scenes[i].ID); err != nil {
			return domain.Snapshot{}, fmt.Errorf("scene id: %w", err)
		}
		if _, duplicate := sceneIDs[scenes[i].ID]; duplicate {
			return domain.Snapshot{}, fmt.Errorf("duplicate scene id %q", scenes[i].ID)
		}
		sceneIDs[scenes[i].ID] = struct{}{}
	}
	shotIDs := make(map[string]struct{}, len(shots))
	shotOrder := make(map[string]int, len(scenes))
	for i := range shots {
		if shots[i].ID == "" {
			shots[i].ID = uuid.NewString()
		}
		if _, ok := sceneIDs[shots[i].SceneID]; !ok {
			return domain.Snapshot{}, fmt.Errorf("shot %q references unknown scene %q", shots[i].ID, shots[i].SceneID)
		}
		if err := ValidateID(shots[i].ID); err != nil {
			return domain.Snapshot{}, fmt.Errorf("shot id: %w", err)
		}
		if _, duplicate := shotIDs[shots[i].ID]; duplicate {
			return domain.Snapshot{}, fmt.Errorf("duplicate shot id %q", shots[i].ID)
		}
		shotIDs[shots[i].ID] = struct{}{}
		shots[i].SchemaVersion = domain.SchemaVersion
		shots[i].Order = shotOrder[shots[i].SceneID]
		shotOrder[shots[i].SceneID]++
		if shots[i].DurationSeconds == 0 {
			shots[i].DurationSeconds = 4
		}
		if shots[i].AspectRatio == "" {
			shots[i].AspectRatio = "16:9"
		}
		if shots[i].CreatedAt.IsZero() {
			shots[i].CreatedAt = now
		}
		for j := range scenes {
			if scenes[j].ID == shots[i].SceneID {
				scenes[j].ShotIDs = append(scenes[j].ShotIDs, shots[i].ID)
				break
			}
		}
	}
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

func (s *Store) ImportAsset(root, source, shotID string, kind domain.AssetKind) (domain.Asset, error) {
	return s.ImportAssetWithParent(root, source, shotID, kind, "")
}

func (s *Store) ImportAssetWithParent(root, source, shotID string, kind domain.AssetKind, parentAssetID string) (domain.Asset, error) {
	if kind != domain.AssetKindImage && kind != domain.AssetKindVideo && kind != domain.AssetKindReference {
		return domain.Asset{}, fmt.Errorf("unsupported asset kind %q", kind)
	}
	var snapshot domain.Snapshot
	if shotID != "" {
		if err := ValidateID(shotID); err != nil {
			return domain.Asset{}, err
		}
		var err error
		snapshot, err = s.Open(root)
		if err != nil {
			return domain.Asset{}, err
		}
		shotFound := false
		for _, shot := range snapshot.Shots {
			if shot.ID == shotID {
				shotFound = true
				break
			}
		}
		if !shotFound {
			return domain.Asset{}, fmt.Errorf("shot %s not found", shotID)
		}
	}
	if parentAssetID != "" {
		if err := ValidateID(parentAssetID); err != nil {
			return domain.Asset{}, fmt.Errorf("parent asset id: %w", err)
		}
		parentFound := false
		for _, asset := range snapshot.Assets {
			if asset.ID == parentAssetID && asset.ShotID == shotID && (asset.Kind == domain.AssetKindImage || asset.Kind == domain.AssetKindReference) {
				parentFound = true
				break
			}
		}
		if !parentFound {
			return domain.Asset{}, fmt.Errorf("parent asset %s is not a visual asset of shot %s", parentAssetID, shotID)
		}
	}
	input, err := os.Open(source)
	if err != nil {
		return domain.Asset{}, fmt.Errorf("open imported asset: %w", err)
	}
	defer input.Close()
	assetID := uuid.NewString()
	ext := strings.ToLower(filepath.Ext(source))
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
	tmp, err := os.CreateTemp(filepath.Dir(destination), ".sceneops-import-*")
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
		SchemaVersion: domain.SchemaVersion,
		ID:            assetID,
		ShotID:        shotID,
		Kind:          kind,
		RelativePath:  filepath.ToSlash(rel),
		SHA256:        hex.EncodeToString(hash.Sum(nil)),
		ParentAssetID: parentAssetID,
		Provenance: domain.Provenance{
			Provider:    "external-import",
			GeneratedAt: s.now(),
		},
		CreatedAt: s.now(),
	}
	if err := s.SaveAsset(root, asset); err != nil {
		return domain.Asset{}, err
	}
	if kind == domain.AssetKindReference && shotID != "" {
		if _, err := s.AddReferenceAsset(root, shotID, asset.ID); err != nil {
			return domain.Asset{}, err
		}
	}
	if err := RebuildIndex(root); err != nil {
		return domain.Asset{}, err
	}
	return asset, nil
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
	if err := json.Unmarshal(data, output); err != nil {
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
	return "# SceneOps Project Instructions\n\n" +
		"- Treat `sceneops.project.json`, `scenes/`, `shots/`, `assets/`, and `runs/` as structured project data.\n" +
		"- Read the project through `sceneops_project_read`.\n" +
		"- Use `sceneops_storyboard_apply` for structured storyboard changes.\n" +
		"- Ask for explicit approval before storyboard writes, image generation, video generation, or cancellation.\n" +
		"- Never expose credentials or write outside this project root.\n"
}
