package project

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/bg-dao/axon-codex-dramaops/internal/domain"
)

func TestDramaProjectRoundTripScriptShotPlanAndIndex(t *testing.T) {
	root := filepath.Join(t.TempDir(), "series")
	store := NewStore()
	snapshot, err := store.CreateWithOptions(root, CreateOptions{Name: "Vertical Drama", ContentLanguage: "zh-CN", Orientation: domain.OrientationPortrait})
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Project.Output.Width != 1080 || snapshot.Project.Output.Height != 1920 || snapshot.Project.Output.FPS != 25 || len(snapshot.Episodes) != 1 {
		t.Fatalf("unexpected defaults: %+v", snapshot)
	}
	plan := testScriptPlan(snapshot.Episodes[0])
	snapshot, err = store.ApplyScript(root, plan)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Scenes) != 2 || len(snapshot.Episodes[0].ScriptBlocks) != 3 || len(snapshot.Characters) != 1 {
		t.Fatalf("script was not persisted: %+v", snapshot)
	}
	if snapshot.Episodes[0].ScriptBlocks[0].ID == "" || snapshot.Episodes[0].ScriptBlocks[0].Order != 0 {
		t.Fatalf("script IDs/order were not normalized: %+v", snapshot.Episodes[0].ScriptBlocks)
	}
	if _, err := store.ApplyScript(root, plan); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("second script apply should fail closed: %v", err)
	}
	shots := []domain.Shot{
		{ID: "shot-001", SceneID: "scene-001", Title: "Door opens", Prompt: "A tense doorway", ShotSize: domain.ShotMS, CameraAngle: domain.AngleEyeLevel, CameraMovement: domain.MovementDolly, ScriptBlockIDs: []string{"block-001"}},
		{ID: "shot-002", SceneID: "scene-001", Title: "Reaction", Prompt: "Close reaction", ShotSize: domain.ShotCU, CameraAngle: domain.AngleEyeLevel, CameraMovement: domain.MovementStatic, ScriptBlockIDs: []string{"block-002"}},
		{ID: "shot-003", SceneID: "scene-002", Title: "Reveal", Prompt: "Reveal the phone", ShotSize: domain.ShotMCU, CameraAngle: domain.AngleHigh, CameraMovement: domain.MovementTilt, ScriptBlockIDs: []string{"block-003"}},
	}
	snapshot, err = store.ApplyShotPlan(root, snapshot.Episodes[0].ID, shots)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Shots) != 3 || snapshot.Shots[0].AspectRatio != "9:16" || snapshot.Shots[0].DurationSeconds != 4 {
		t.Fatalf("shot defaults/order invalid: %+v", snapshot.Shots)
	}
	if snapshot.Shots[0].Order != 0 || snapshot.Shots[1].Order != 1 || snapshot.Shots[2].Order != 0 {
		t.Fatalf("shot order is not per-scene: %+v", snapshot.Shots)
	}
	if _, err := store.ApplyShotPlan(root, snapshot.Episodes[0].ID, shots); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("second shot plan should fail closed: %v", err)
	}
	indexPath, _ := IndexPath(root)
	if err := os.Remove(indexPath); err != nil {
		t.Fatal(err)
	}
	if err := RebuildIndex(root); err != nil {
		t.Fatal(err)
	}
	if count, err := CountIndexed(root, "shots"); err != nil || count != 3 {
		t.Fatalf("rebuilt count = %d, err = %v", count, err)
	}
}

func testScriptPlan(episode domain.Episode) ScriptPlan {
	episode.Logline = "A woman receives a message from her future self."
	episode.ScriptBlocks = []domain.ScriptBlock{
		{ID: "block-001", SceneID: "scene-001", Kind: domain.ScriptAction, Text: "Lin enters the empty apartment."},
		{ID: "block-002", SceneID: "scene-001", Kind: domain.ScriptDialogue, CharacterID: "character-lin", Text: "Who sent this?", Emotion: "uneasy"},
		{ID: "block-003", SceneID: "scene-002", Kind: domain.ScriptSFX, Text: "The phone alarm screams."},
	}
	return ScriptPlan{
		Episode:    episode,
		Scenes:     []domain.Scene{{ID: "scene-001", Title: "INT. APARTMENT - NIGHT", LocationID: "location-apartment"}, {ID: "scene-002", Title: "INT. BEDROOM - NIGHT", LocationID: "location-bedroom"}},
		Characters: []domain.Character{{ID: "character-lin", Name: "Lin", VoiceProfile: domain.VoiceProfile{ID: "voice-lin", Kind: domain.VoiceBuiltIn, Name: "Lin", BuiltInVoice: "coral"}}},
		Locations:  []domain.Location{{ID: "location-apartment", Name: "Apartment"}, {ID: "location-bedroom", Name: "Bedroom"}},
	}
}

func TestAssetSelectionTypesAndMultiInputProvenance(t *testing.T) {
	root := filepath.Join(t.TempDir(), "series")
	store := NewStore()
	snapshot, err := store.Create(root, "Assets")
	if err != nil {
		t.Fatal(err)
	}
	plan := testScriptPlan(snapshot.Episodes[0])
	snapshot, err = store.ApplyScript(root, plan)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err = store.ApplyShotPlan(root, snapshot.Episodes[0].ID, []domain.Shot{{ID: "shot-001", SceneID: "scene-001", Title: "Shot", Prompt: "Shot", ShotSize: domain.ShotMS, CameraAngle: domain.AngleEyeLevel, CameraMovement: domain.MovementStatic}})
	if err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(t.TempDir(), "frame.png")
	if err := os.WriteFile(source, []byte("frame"), 0o600); err != nil {
		t.Fatal(err)
	}
	image, err := store.ImportAsset(root, ImportOptions{Source: source, EpisodeID: snapshot.Episodes[0].ID, ShotID: "shot-001", Kind: domain.AssetKindImage})
	if err != nil {
		t.Fatal(err)
	}
	reference, err := store.ImportAsset(root, ImportOptions{Source: source, EpisodeID: snapshot.Episodes[0].ID, ShotID: "shot-001", Kind: domain.AssetKindReference})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.SelectKeyframeVersion(root, "shot-001", reference.ID); err == nil {
		t.Fatal("reference must not be selectable as a generated keyframe")
	}
	if _, err := store.SelectKeyframeVersion(root, "shot-001", image.ID); err != nil {
		t.Fatal(err)
	}
	videoSource := filepath.Join(t.TempDir(), "clip.mp4")
	if err := os.WriteFile(videoSource, []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	video, err := store.ImportAsset(root, ImportOptions{Source: videoSource, EpisodeID: snapshot.Episodes[0].ID, ShotID: "shot-001", Kind: domain.AssetKindVideo, Inputs: []domain.AssetInput{{AssetID: image.ID, Role: "keyframe"}, {AssetID: reference.ID, Role: "character"}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(video.Inputs) != 2 {
		t.Fatalf("multi-input provenance lost: %+v", video.Inputs)
	}
	opened, err := store.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if opened.Shots[0].SelectedKeyframeAssetID != image.ID || opened.Shots[0].SelectedVideoAssetID != video.ID || len(opened.Shots[0].ReferenceAssets) != 1 {
		t.Fatalf("asset links invalid: %+v", opened.Shots[0])
	}
}

func TestImportAssetValidatesOwnershipAndLocksDialogueVoice(t *testing.T) {
	root := filepath.Join(t.TempDir(), "series")
	store := NewStore()
	snapshot, err := store.Create(root, "Import ownership")
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err = store.ApplyScript(root, testScriptPlan(snapshot.Episodes[0]))
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err = store.ApplyShotPlan(root, snapshot.Episodes[0].ID, []domain.Shot{{ID: "shot-001", SceneID: "scene-001", Title: "Shot", Prompt: "Shot"}})
	if err != nil {
		t.Fatal(err)
	}
	audioSource := filepath.Join(t.TempDir(), "dialogue.wav")
	if err := os.WriteFile(audioSource, []byte("dialogue"), 0o600); err != nil {
		t.Fatal(err)
	}
	audio, err := store.ImportAsset(root, ImportOptions{Source: audioSource, ScriptBlockID: "block-002", Kind: domain.AssetKindAudio})
	if err != nil {
		t.Fatal(err)
	}
	if audio.EpisodeID != "episode-001" || audio.Provenance.Parameters["voiceProfileId"] != "voice-lin" {
		t.Fatalf("dialogue ownership/provenance = %+v", audio)
	}
	opened, err := store.Open(root)
	if err != nil || opened.Episodes[0].ScriptBlocks[1].SelectedVoiceAssetID != audio.ID {
		t.Fatalf("dialogue selection = %+v, err = %v", opened.Episodes[0].ScriptBlocks, err)
	}

	second := domain.Episode{SchemaVersion: domain.SchemaVersion, ID: "episode-002", Number: 2, Title: "Episode 2", Status: domain.EpisodeDraft, SceneIDs: []string{}, ScriptBlocks: []domain.ScriptBlock{}}
	if err := store.SaveEpisode(root, second); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveEdit(root, domain.EpisodeEdit{SchemaVersion: domain.SchemaVersion, EpisodeID: second.ID, VideoTrack: []domain.VideoClip{}, AudioCues: []domain.AudioCue{}, SubtitleCues: []domain.SubtitleCue{}, Output: snapshot.Project.Output}); err != nil {
		t.Fatal(err)
	}
	videoSource := filepath.Join(t.TempDir(), "clip.mp4")
	if err := os.WriteFile(videoSource, []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ImportAsset(root, ImportOptions{Source: videoSource, EpisodeID: second.ID, ShotID: "shot-001", Kind: domain.AssetKindVideo}); err == nil || !strings.Contains(err.Error(), "does not belong") {
		t.Fatalf("cross-episode asset must fail before writing: %v", err)
	}
	if reopened, err := store.Open(root); err != nil || len(reopened.Assets) != 1 {
		t.Fatalf("failed import damaged project: assets=%d err=%v", len(reopened.Assets), err)
	}
}

func TestResolveRelativeRejectsEscapesAndSymlinks(t *testing.T) {
	root := t.TempDir()
	for _, path := range []string{"../outside", "/tmp/outside", "scenes/../../outside", ""} {
		if _, err := ResolveRelative(root, path); err == nil {
			t.Fatalf("expected %q to be rejected", path)
		}
	}
	if runtime.GOOS == "windows" {
		t.Skip("symlink privileges vary")
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "linked")); err != nil {
		t.Fatal(err)
	}
	if _, err := ResolveRelative(root, "linked/file.json"); err == nil {
		t.Fatal("symlink traversal must be rejected")
	}
}

func TestAtomicWriteAndUnsupportedFormatsFailClosed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "manifest.json")
	if err := AtomicWrite(path, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := AtomicWrite(path, []byte("complete"), 0o644); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "complete" {
		t.Fatalf("content = %q", data)
	}
	entries, _ := filepath.Glob(filepath.Join(filepath.Dir(path), ".dramaops-write-*"))
	if len(entries) != 0 {
		t.Fatalf("temporary files remain: %v", entries)
	}

	future := filepath.Join(t.TempDir(), "future")
	_ = os.MkdirAll(future, 0o755)
	encoded, _ := json.Marshal(domain.Project{SchemaVersion: 99, ID: "future", Name: "Future"})
	_ = os.WriteFile(filepath.Join(future, ProjectManifest), encoded, 0o644)
	if _, err := NewStore().Open(future); err == nil || !strings.Contains(err.Error(), "unsupported schemaVersion") {
		t.Fatalf("future format did not fail closed: %v", err)
	}

	legacy := filepath.Join(t.TempDir(), "legacy")
	_ = os.MkdirAll(legacy, 0o755)
	_ = os.WriteFile(filepath.Join(legacy, "scene"+"ops.project.json"), []byte(`{"schemaVersion":1}`), 0o644)
	if _, err := NewStore().Open(legacy); err == nil || !strings.Contains(err.Error(), "unsupported legacy") {
		t.Fatalf("legacy format did not fail clearly: %v", err)
	}
}

func TestOpenRejectsTrailingJSONAndSymlinkedManifests(t *testing.T) {
	store := NewStore()

	trailingRoot := filepath.Join(t.TempDir(), "trailing")
	if _, err := store.Create(trailingRoot, "Trailing"); err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(trailingRoot, ProjectManifest)
	manifest, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifestPath, append(manifest, []byte("\n{}")...), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Open(trailingRoot); err == nil || !strings.Contains(err.Error(), "trailing JSON value") {
		t.Fatalf("trailing JSON must fail closed: %v", err)
	}

	if runtime.GOOS == "windows" {
		return
	}
	manifestRoot := filepath.Join(t.TempDir(), "manifest-link")
	if _, err := store.Create(manifestRoot, "Manifest link"); err != nil {
		t.Fatal(err)
	}
	outsideCharacter := filepath.Join(t.TempDir(), "outside-character.json")
	if err := AtomicWriteJSON(outsideCharacter, domain.Character{SchemaVersion: domain.SchemaVersion, ID: "outside", Name: "Outside", VoiceProfile: domain.VoiceProfile{ID: "voice-outside", Kind: domain.VoiceBuiltIn, BuiltInVoice: "alloy"}}); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outsideCharacter, filepath.Join(manifestRoot, "characters", "outside.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Open(manifestRoot); err == nil || !strings.Contains(err.Error(), "symlink paths are not allowed") {
		t.Fatalf("symlinked manifest must fail closed: %v", err)
	}

	directoryRoot := filepath.Join(t.TempDir(), "directory-link")
	if _, err := store.Create(directoryRoot, "Directory link"); err != nil {
		t.Fatal(err)
	}
	outsideDirectory := t.TempDir()
	if err := AtomicWriteJSON(filepath.Join(outsideDirectory, "outside.json"), domain.Prop{SchemaVersion: domain.SchemaVersion, ID: "outside", Name: "Outside"}); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(directoryRoot, "props")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outsideDirectory, filepath.Join(directoryRoot, "props")); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Open(directoryRoot); err == nil || !strings.Contains(err.Error(), "symlink paths are not allowed") {
		t.Fatalf("symlinked manifest directory must fail closed: %v", err)
	}
}

func TestConcurrentIndexRebuildKeepsAReadableIndex(t *testing.T) {
	root := filepath.Join(t.TempDir(), "series")
	if _, err := NewStore().Create(root, "Concurrent index"); err != nil {
		t.Fatal(err)
	}
	errorsByWorker := make(chan error, 12)
	var workers sync.WaitGroup
	for i := 0; i < cap(errorsByWorker); i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			errorsByWorker <- RebuildIndex(root)
		}()
	}
	workers.Wait()
	close(errorsByWorker)
	for err := range errorsByWorker {
		if err != nil {
			t.Fatal(err)
		}
	}
	if count, err := CountIndexed(root, "episodes"); err != nil || count != 1 {
		t.Fatalf("rebuilt index count = %d, err = %v", count, err)
	}
}

func TestCreateDoesNotOverwriteExistingProject(t *testing.T) {
	root := filepath.Join(t.TempDir(), "existing")
	store := NewStore()
	first, err := store.Create(root, "First")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Create(root, "Second"); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("expected duplicate create to fail: %v", err)
	}
	opened, err := store.Open(root)
	if err != nil || opened.Project.ID != first.Project.ID || opened.Project.Name != "First" {
		t.Fatalf("existing project changed: %+v, %v", opened.Project, err)
	}
}

func TestOpenRejectsTamperedAssetPathsAndRunStates(t *testing.T) {
	store := NewStore()
	root := filepath.Join(t.TempDir(), "asset-project")
	if _, err := store.Create(root, "Asset validation"); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(t.TempDir(), "frame.png")
	if err := os.WriteFile(source, []byte("frame"), 0o600); err != nil {
		t.Fatal(err)
	}
	asset, err := store.ImportAsset(root, ImportOptions{Source: source, Kind: domain.AssetKindImage})
	if err != nil {
		t.Fatal(err)
	}
	asset.RelativePath = "../../outside.png"
	manifest := filepath.Join(root, "assets", asset.ID, "asset.json")
	if err := AtomicWriteJSON(manifest, asset); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Open(root); err == nil || !strings.Contains(err.Error(), "path escapes project root") {
		t.Fatalf("tampered asset path did not fail closed: %v", err)
	}

	runRoot := filepath.Join(t.TempDir(), "run-project")
	if _, err := store.Create(runRoot, "Run validation"); err != nil {
		t.Fatal(err)
	}
	run := domain.Run{SchemaVersion: domain.SchemaVersion, ID: "run-invalid", Operation: "video_generate", Status: "mystery"}
	if err := AtomicWriteJSON(filepath.Join(runRoot, "runs", run.ID+".json"), run); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Open(runRoot); err == nil || !strings.Contains(err.Error(), "unsupported run status") {
		t.Fatalf("invalid run state did not fail closed: %v", err)
	}
}

func TestShotPlanValidatesBeforeWritingAndRelationshipsFailClosed(t *testing.T) {
	root := filepath.Join(t.TempDir(), "series")
	store := NewStore()
	snapshot, err := store.Create(root, "Validation")
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err = store.ApplyScript(root, testScriptPlan(snapshot.Episodes[0]))
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.ApplyShotPlan(root, snapshot.Episodes[0].ID, []domain.Shot{
		{ID: "shot-valid", SceneID: "scene-001", Title: "Valid", Prompt: "valid"},
		{ID: "shot-invalid", SceneID: "scene-001", Title: "Invalid", Prompt: "invalid", LensMM: 2},
	})
	if err == nil {
		t.Fatal("malformed shot plan must fail")
	}
	paths, _ := filepath.Glob(filepath.Join(root, "shots", "*.json"))
	if len(paths) != 0 {
		t.Fatalf("partially written shot plan: %v", paths)
	}
	opened, err := store.Open(root)
	if err != nil || len(opened.Shots) != 0 {
		t.Fatalf("project did not remain usable: shots=%d err=%v", len(opened.Shots), err)
	}

	scene := opened.Scenes[0]
	scene.EpisodeID = "episode-missing"
	if err := AtomicWriteJSON(filepath.Join(root, "scenes", scene.ID+".json"), scene); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Open(root); err == nil {
		t.Fatalf("broken scene relationship did not fail closed: %v", err)
	}
}

func TestCheckedInShortDramaExampleOpens(t *testing.T) {
	root := filepath.Join("..", "..", "examples", "nocturne-train")
	snapshot, err := NewStore().Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Project.Output.Orientation != domain.OrientationPortrait || snapshot.Project.Output.Width != 1080 || snapshot.Project.Output.Height != 1920 || snapshot.Project.Output.FPS != 25 {
		t.Fatalf("example output = %+v", snapshot.Project.Output)
	}
	if len(snapshot.Episodes) != 1 || len(snapshot.Characters) != 2 || len(snapshot.Locations) != 3 || len(snapshot.Props) != 1 || len(snapshot.Scenes) != 3 || len(snapshot.Shots) != 8 {
		t.Fatalf("example shape = episodes:%d characters:%d locations:%d props:%d scenes:%d shots:%d", len(snapshot.Episodes), len(snapshot.Characters), len(snapshot.Locations), len(snapshot.Props), len(snapshot.Scenes), len(snapshot.Shots))
	}
	total := 0.0
	for _, shot := range snapshot.Shots {
		total += shot.DurationSeconds
		if shot.ShotSize == "" || shot.CameraAngle == "" || shot.CameraMovement == "" || shot.LensMM == 0 || shot.Composition == "" || shot.ScreenDirection == "" {
			t.Fatalf("example shot lacks professional fields: %+v", shot)
		}
	}
	if total < 60 || total > 90 {
		t.Fatalf("example duration = %.1fs", total)
	}
}
