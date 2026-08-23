package continuity

import (
	"testing"

	"github.com/bg-dao/axon-codex-dramaops/internal/domain"
)

func TestCheckFindsVisualVoiceCameraSpecAndTimelineConflicts(t *testing.T) {
	snapshot := domain.Snapshot{
		Project:    domain.Project{Output: domain.DefaultOutputSettings(domain.OrientationPortrait)},
		Characters: []domain.Character{{ID: "character-1", Name: "Lin", VoiceProfile: domain.VoiceProfile{ID: "voice-lin", Kind: domain.VoiceBuiltIn}}},
		Episodes: []domain.Episode{{ID: "episode-1", ScriptBlocks: []domain.ScriptBlock{
			{ID: "block-1", SceneID: "scene-1", Kind: domain.ScriptDialogue, CharacterID: "character-1", Text: "Hello", SelectedVoiceAssetID: "audio-1"},
		}}},
		Shots: []domain.Shot{
			{ID: "shot-1", EpisodeID: "episode-1", SceneID: "scene-1", Order: 0, ScreenDirection: "left to right", WardrobeContinuity: "dry coat", PropContinuity: "phone closed", SelectedVideoAssetID: "video-1"},
			{ID: "shot-2", EpisodeID: "episode-1", SceneID: "scene-1", Order: 1, ScreenDirection: "right to left", WardrobeContinuity: "wet coat", PropContinuity: "phone open"},
		},
		Assets: []domain.Asset{
			{ID: "audio-1", Kind: domain.AssetKindAudio, EpisodeID: "episode-1", ScriptBlockID: "block-1", Provenance: domain.Provenance{Parameters: map[string]any{"voiceProfileId": "voice-other"}}},
			{ID: "video-1", Kind: domain.AssetKindVideo, ShotID: "shot-1", MediaInfo: domain.MediaInfo{Width: 1920, Height: 1080, FPS: 24}},
		},
		Edits: []domain.EpisodeEdit{{EpisodeID: "episode-1", VideoTrack: []domain.VideoClip{{ID: "clip-1", ShotID: "shot-2", AssetID: "video-1", Order: 2}}}},
	}
	issues := Check(snapshot)
	want := map[string]bool{
		"character_reference_missing": false, "voice_profile_mismatch": false, "keyframe_missing": false,
		"video_spec_mismatch": false, "video_fps_mismatch": false, "screen_direction_conflict": false,
		"wardrobe_state_changed": false, "prop_state_changed": false, "timeline_selection_mismatch": false,
	}
	for _, issue := range issues {
		if _, ok := want[issue.Code]; ok {
			want[issue.Code] = true
		}
	}
	for code, found := range want {
		if !found {
			t.Errorf("missing continuity issue %s; got %+v", code, issues)
		}
	}
}

func TestCheckDoesNotPersistStateOrReportConfiguredDialogue(t *testing.T) {
	snapshot := domain.Snapshot{
		Project:    domain.Project{Output: domain.DefaultOutputSettings(domain.OrientationPortrait)},
		Characters: []domain.Character{{ID: "character-1", Name: "Lin", ReferenceAssets: []string{"ref-1"}, VoiceProfile: domain.VoiceProfile{ID: "voice-lin", Kind: domain.VoiceBuiltIn}}},
		Episodes:   []domain.Episode{{ID: "episode-1", ScriptBlocks: []domain.ScriptBlock{{ID: "block-1", SceneID: "scene-1", Kind: domain.ScriptDialogue, CharacterID: "character-1", Text: "Hello", SelectedVoiceAssetID: "audio-1"}}}},
		Assets:     []domain.Asset{{ID: "audio-1", Kind: domain.AssetKindAudio, EpisodeID: "episode-1", ScriptBlockID: "block-1", Provenance: domain.Provenance{Parameters: map[string]any{"voiceProfileId": "voice-lin"}}}},
	}
	for _, issue := range Check(snapshot) {
		if issue.Code == "character_reference_missing" || issue.Code == "dialogue_voice_missing" || issue.Code == "voice_profile_mismatch" {
			t.Fatalf("false positive: %+v", issue)
		}
	}
}

func TestCheckReportsEmptyOutOfRangeAndOverlappingTimelineCues(t *testing.T) {
	empty := domain.Snapshot{Edits: []domain.EpisodeEdit{{EpisodeID: "episode-empty"}}}
	if !hasIssue(Check(empty), "timeline_empty") {
		t.Fatal("empty timeline was not reported")
	}

	snapshot := domain.Snapshot{
		Project: domain.Project{Output: domain.DefaultOutputSettings(domain.OrientationPortrait)},
		Shots: []domain.Shot{
			{ID: "shot-1", EpisodeID: "episode-1", SceneID: "scene-1", SelectedVideoAssetID: "video-1"},
			{ID: "shot-2", EpisodeID: "episode-1", SceneID: "scene-1", SelectedVideoAssetID: "video-2"},
		},
		Assets: []domain.Asset{
			{ID: "video-1", Kind: domain.AssetKindVideo, ShotID: "shot-1"},
			{ID: "video-2", Kind: domain.AssetKindVideo, ShotID: "shot-2"},
		},
		Edits: []domain.EpisodeEdit{{EpisodeID: "episode-1",
			VideoTrack: []domain.VideoClip{{ID: "clip-1", ShotID: "shot-1", AssetID: "video-1", Order: 0, InSeconds: 0, OutSeconds: 4}},
			AudioCues: []domain.AudioCue{
				{ID: "dialogue-1", Lane: domain.LaneDialogue, StartSeconds: 1, DurationSeconds: 2},
				{ID: "dialogue-2", Lane: domain.LaneDialogue, StartSeconds: 2, DurationSeconds: 3},
			},
			SubtitleCues: []domain.SubtitleCue{
				{ID: "subtitle-1", StartSeconds: 1, DurationSeconds: 2},
				{ID: "subtitle-2", StartSeconds: 2, DurationSeconds: 3},
			},
		}},
	}
	issues := Check(snapshot)
	for _, code := range []string{"timeline_shot_missing", "timeline_audio_out_of_range", "timeline_subtitle_out_of_range", "timeline_dialogue_overlap", "timeline_subtitle_overlap"} {
		if !hasIssue(issues, code) {
			t.Errorf("missing issue %s: %+v", code, issues)
		}
	}
}

func hasIssue(issues []domain.ContinuityIssue, code string) bool {
	for _, issue := range issues {
		if issue.Code == code {
			return true
		}
	}
	return false
}
