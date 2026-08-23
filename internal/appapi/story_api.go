package appapi

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/bg-dao/axon-codex-dramaops/internal/domain"
	"github.com/bg-dao/axon-codex-dramaops/internal/fountain"
	"github.com/bg-dao/axon-codex-dramaops/internal/project"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type StoryAPI struct{ backend *Backend }

type FountainResult struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

func NewStoryAPI(backend *Backend) *StoryAPI { return &StoryAPI{backend: backend} }

func (a *StoryAPI) CreateEpisode(title string) (domain.Snapshot, error) {
	root, err := a.backend.Root()
	if err != nil {
		return domain.Snapshot{}, err
	}
	snapshot, err := a.backend.store.Open(root)
	if err != nil {
		return domain.Snapshot{}, err
	}
	number := 1
	for _, episode := range snapshot.Episodes {
		if episode.Number >= number {
			number = episode.Number + 1
		}
	}
	id := fmt.Sprintf("episode-%03d", number)
	now := time.Now().UTC()
	episode := domain.Episode{SchemaVersion: domain.SchemaVersion, ID: id, Number: number, Title: strings.TrimSpace(title), Status: domain.EpisodeDraft, SceneIDs: []string{}, ScriptBlocks: []domain.ScriptBlock{}, CreatedAt: now, UpdatedAt: now}
	if episode.Title == "" {
		episode.Title = fmt.Sprintf("Episode %d", number)
	}
	if err := a.backend.store.SaveEpisode(root, episode); err != nil {
		return domain.Snapshot{}, err
	}
	if err := a.backend.store.SaveEdit(root, domain.EpisodeEdit{SchemaVersion: domain.SchemaVersion, EpisodeID: id, VideoTrack: []domain.VideoClip{}, AudioCues: []domain.AudioCue{}, SubtitleCues: []domain.SubtitleCue{}, Output: snapshot.Project.Output, UpdatedAt: now}); err != nil {
		return domain.Snapshot{}, err
	}
	snapshot.Project.ActiveEpisodeID = id
	if err := a.backend.store.SaveProject(root, snapshot.Project); err != nil {
		return domain.Snapshot{}, err
	}
	return a.refresh(root)
}

func (a *StoryAPI) SaveEpisode(value domain.Episode) (domain.Snapshot, error) {
	return a.save(func(root string) error { return a.backend.store.SaveEpisode(root, value) })
}
func (a *StoryAPI) SaveCharacter(value domain.Character) (domain.Snapshot, error) {
	return a.save(func(root string) error { return a.backend.store.SaveCharacter(root, value) })
}
func (a *StoryAPI) SaveLocation(value domain.Location) (domain.Snapshot, error) {
	return a.save(func(root string) error { return a.backend.store.SaveLocation(root, value) })
}
func (a *StoryAPI) SaveProp(value domain.Prop) (domain.Snapshot, error) {
	return a.save(func(root string) error { return a.backend.store.SaveProp(root, value) })
}
func (a *StoryAPI) SaveScene(value domain.Scene) (domain.Snapshot, error) {
	return a.save(func(root string) error { return a.backend.store.SaveScene(root, value) })
}
func (a *StoryAPI) SaveShot(value domain.Shot) (domain.Snapshot, error) {
	return a.save(func(root string) error { return a.backend.store.SaveShot(root, value) })
}
func (a *StoryAPI) SaveEdit(value domain.EpisodeEdit) (domain.Snapshot, error) {
	return a.save(func(root string) error { return a.backend.store.SaveEdit(root, value) })
}

func (a *StoryAPI) ImportFountain(episodeID string) (domain.Snapshot, error) {
	root, err := a.backend.Root()
	if err != nil {
		return domain.Snapshot{}, err
	}
	snapshot, err := a.backend.store.Open(root)
	if err != nil {
		return domain.Snapshot{}, err
	}
	var current domain.Episode
	found := false
	for _, episode := range snapshot.Episodes {
		if episode.ID == episodeID {
			current, found = episode, true
			break
		}
	}
	if !found {
		return domain.Snapshot{}, fmt.Errorf("episode %s not found", episodeID)
	}
	if len(current.ScriptBlocks) > 0 {
		return domain.Snapshot{}, errors.New("Fountain import is available only for an empty episode")
	}
	sourcePath, err := wailsruntime.OpenFileDialog(a.backend.context(), wailsruntime.OpenDialogOptions{Title: "Import Fountain screenplay", Filters: []wailsruntime.FileFilter{{DisplayName: "Fountain", Pattern: "*.fountain;*.txt"}}})
	if err != nil || sourcePath == "" {
		return snapshot, err
	}
	data, err := os.ReadFile(sourcePath)
	if err != nil {
		return domain.Snapshot{}, err
	}
	episode, scenes, err := fountain.Parse(episodeID, current.Title, string(data))
	if err != nil {
		return domain.Snapshot{}, err
	}
	episode.Number, episode.CreatedAt = current.Number, current.CreatedAt
	characterIDs := map[string]bool{}
	for _, block := range episode.ScriptBlocks {
		if block.CharacterID != "" {
			characterIDs[block.CharacterID] = true
		}
	}
	characters := make([]domain.Character, 0, len(characterIDs))
	now := time.Now().UTC()
	for id := range characterIDs {
		characters = append(characters, domain.Character{SchemaVersion: domain.SchemaVersion, ID: id, Name: humanName(id), VoiceProfile: domain.VoiceProfile{ID: "voice-" + id, Kind: domain.VoiceBuiltIn, Name: humanName(id), BuiltInVoice: "alloy"}, CreatedAt: now, UpdatedAt: now})
	}
	sort.Slice(characters, func(i, j int) bool { return characters[i].ID < characters[j].ID })
	result, err := a.backend.store.ApplyScript(root, project.ScriptPlan{Episode: episode, Scenes: scenes, Characters: characters})
	if err == nil {
		a.backend.emit(EventProjectChanged, result)
	}
	return result, err
}

func (a *StoryAPI) ExportFountain(episodeID string) (FountainResult, error) {
	root, err := a.backend.Root()
	if err != nil {
		return FountainResult{}, err
	}
	snapshot, err := a.backend.store.Open(root)
	if err != nil {
		return FountainResult{}, err
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
		return FountainResult{}, fmt.Errorf("episode %s not found", episodeID)
	}
	content := fountain.Format(episode, snapshot.Scenes, snapshot.Characters)
	relative := filepath.Join("exports", episode.ID+".fountain")
	path, err := project.ResolveRelative(root, relative)
	if err != nil {
		return FountainResult{}, err
	}
	if err := project.AtomicWrite(path, []byte(content), 0o644); err != nil {
		return FountainResult{}, err
	}
	return FountainResult{Path: path, Content: content}, nil
}

func (a *StoryAPI) save(operation func(root string) error) (domain.Snapshot, error) {
	root, err := a.backend.Root()
	if err != nil {
		return domain.Snapshot{}, err
	}
	if err := operation(root); err != nil {
		return domain.Snapshot{}, err
	}
	return a.refresh(root)
}

func (a *StoryAPI) refresh(root string) (domain.Snapshot, error) {
	if err := project.RebuildIndex(root); err != nil {
		return domain.Snapshot{}, err
	}
	snapshot, err := a.backend.store.Open(root)
	if err == nil {
		a.backend.emit(EventProjectChanged, snapshot)
	}
	return snapshot, err
}

func humanName(id string) string {
	value := strings.NewReplacer("-", " ", "_", " ").Replace(id)
	return strings.Title(value)
}
