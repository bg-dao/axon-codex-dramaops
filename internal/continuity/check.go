package continuity

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/bg-dao/axon-codex-dramaops/internal/domain"
)

// Check derives continuity warnings from durable manifests. The result is never
// persisted, so deleting the SQLite index or changing a manifest cannot leave a
// stale workflow state behind.
func Check(snapshot domain.Snapshot) []domain.ContinuityIssue {
	issues := make([]domain.ContinuityIssue, 0)
	assets := make(map[string]domain.Asset, len(snapshot.Assets))
	characters := make(map[string]domain.Character, len(snapshot.Characters))
	shots := make(map[string]domain.Shot, len(snapshot.Shots))
	for _, asset := range snapshot.Assets {
		assets[asset.ID] = asset
	}
	for _, character := range snapshot.Characters {
		characters[character.ID] = character
	}
	for _, shot := range snapshot.Shots {
		shots[shot.ID] = shot
	}

	for _, character := range snapshot.Characters {
		if len(character.ReferenceAssets) == 0 {
			issues = append(issues, issue("character_reference_missing", domain.ContinuityWarning, "", "", "", fmt.Sprintf("Character %s has no visual reference", character.Name)))
		}
		if character.VoiceProfile.ID == "" {
			issues = append(issues, issue("voice_profile_missing", domain.ContinuityError, "", "", "", fmt.Sprintf("Character %s has no voice profile", character.Name)))
		}
	}

	for _, episode := range snapshot.Episodes {
		coveredBlocks := map[string]bool{}
		for _, shot := range snapshot.Shots {
			if shot.EpisodeID != episode.ID {
				continue
			}
			for _, blockID := range shot.ScriptBlockIDs {
				coveredBlocks[blockID] = true
			}
		}
		for _, block := range episode.ScriptBlocks {
			if !coveredBlocks[block.ID] {
				issues = append(issues, issue("script_block_uncovered", domain.ContinuityWarning, episode.ID, block.SceneID, "", fmt.Sprintf("Script block %s is not covered by a shot", block.ID)))
			}
			if block.Kind != domain.ScriptDialogue && block.Kind != domain.ScriptVoiceOver {
				continue
			}
			character, ok := characters[block.CharacterID]
			if !ok {
				issues = append(issues, issue("dialogue_character_missing", domain.ContinuityError, episode.ID, block.SceneID, "", fmt.Sprintf("Dialogue block %s references a missing character", block.ID)))
				continue
			}
			if block.SelectedVoiceAssetID == "" {
				issues = append(issues, issue("dialogue_voice_missing", domain.ContinuityWarning, episode.ID, block.SceneID, "", fmt.Sprintf("%s has dialogue without selected voice audio", character.Name)))
				continue
			}
			asset, ok := assets[block.SelectedVoiceAssetID]
			if !ok || asset.Kind != domain.AssetKindAudio || asset.ScriptBlockID != block.ID {
				issues = append(issues, issue("dialogue_voice_invalid", domain.ContinuityError, episode.ID, block.SceneID, "", fmt.Sprintf("Dialogue block %s has an invalid voice asset", block.ID)))
				continue
			}
			profileID, _ := asset.Provenance.Parameters["voiceProfileId"].(string)
			if profileID != "" && profileID != character.VoiceProfile.ID {
				issues = append(issues, issue("voice_profile_mismatch", domain.ContinuityError, episode.ID, block.SceneID, "", fmt.Sprintf("%s dialogue uses a different voice profile", character.Name)))
			}
		}
	}

	byScene := make(map[string][]domain.Shot)
	for _, shot := range snapshot.Shots {
		byScene[shot.SceneID] = append(byScene[shot.SceneID], shot)
	}
	for sceneID, sceneShots := range byScene {
		sort.Slice(sceneShots, func(i, j int) bool { return sceneShots[i].Order < sceneShots[j].Order })
		for i, shot := range sceneShots {
			if shot.SelectedKeyframeAssetID == "" {
				issues = append(issues, issue("keyframe_missing", domain.ContinuityWarning, shot.EpisodeID, sceneID, shot.ID, "Shot has no selected keyframe"))
			}
			if shot.SelectedVideoAssetID == "" {
				issues = append(issues, issue("video_missing", domain.ContinuityInfo, shot.EpisodeID, sceneID, shot.ID, "Shot has no selected video clip"))
			} else if asset, ok := assets[shot.SelectedVideoAssetID]; ok {
				if !hasExtractedFrame(snapshot.Assets, asset.ID, "first") || !hasExtractedFrame(snapshot.Assets, asset.ID, "tail") {
					issues = append(issues, issue("continuity_frames_missing", domain.ContinuityInfo, shot.EpisodeID, sceneID, shot.ID, "Selected video has no complete first/tail continuity frame pair"))
				}
				if asset.MediaInfo.Width > 0 && (asset.MediaInfo.Width != snapshot.Project.Output.Width || asset.MediaInfo.Height != snapshot.Project.Output.Height) {
					issues = append(issues, issue("video_spec_mismatch", domain.ContinuityWarning, shot.EpisodeID, sceneID, shot.ID, "Video dimensions differ from project output and will be conformed"))
				}
				if asset.MediaInfo.FPS > 0 && math.Abs(asset.MediaInfo.FPS-float64(snapshot.Project.Output.FPS)) > 0.01 {
					issues = append(issues, issue("video_fps_mismatch", domain.ContinuityWarning, shot.EpisodeID, sceneID, shot.ID, "Video frame rate differs from project output and will be conformed"))
				}
			}
			if i == 0 {
				continue
			}
			previous := sceneShots[i-1]
			if oppositeDirection(previous.ScreenDirection, shot.ScreenDirection) {
				issues = append(issues, issue("screen_direction_conflict", domain.ContinuityWarning, shot.EpisodeID, sceneID, shot.ID, "Screen direction flips between adjacent shots"))
			}
			if previous.WardrobeContinuity != "" && shot.WardrobeContinuity != "" && previous.WardrobeContinuity != shot.WardrobeContinuity {
				issues = append(issues, issue("wardrobe_state_changed", domain.ContinuityInfo, shot.EpisodeID, sceneID, shot.ID, "Wardrobe continuity note changes from the previous shot"))
			}
			if previous.PropContinuity != "" && shot.PropContinuity != "" && previous.PropContinuity != shot.PropContinuity {
				issues = append(issues, issue("prop_state_changed", domain.ContinuityInfo, shot.EpisodeID, sceneID, shot.ID, "Prop continuity note changes from the previous shot"))
			}
		}
	}

	for _, edit := range snapshot.Edits {
		if len(edit.VideoTrack) == 0 {
			issues = append(issues, issue("timeline_empty", domain.ContinuityWarning, edit.EpisodeID, "", "", "Episode timeline has no video clips"))
			continue
		}
		clipShots := map[string]bool{}
		total := 0.0
		for i, clip := range edit.VideoTrack {
			clipShots[clip.ShotID] = true
			shot, ok := shots[clip.ShotID]
			if !ok || shot.SelectedVideoAssetID != clip.AssetID {
				issues = append(issues, issue("timeline_selection_mismatch", domain.ContinuityWarning, edit.EpisodeID, "", clip.ShotID, "Timeline clip does not match the selected shot video"))
			}
			if i > 0 && clip.Order != edit.VideoTrack[i-1].Order+1 {
				issues = append(issues, issue("timeline_order_gap", domain.ContinuityError, edit.EpisodeID, "", clip.ShotID, "Video track order contains a gap or overlap"))
			}
			duration := clip.OutSeconds - clip.InSeconds - clip.TransitionSeconds
			if duration <= 0 {
				issues = append(issues, issue("timeline_clip_invalid", domain.ContinuityError, edit.EpisodeID, "", clip.ShotID, "Video clip duration or transition is invalid"))
			} else {
				total += duration
			}
		}
		for _, shot := range snapshot.Shots {
			if shot.EpisodeID == edit.EpisodeID && shot.SelectedVideoAssetID != "" && !clipShots[shot.ID] {
				issues = append(issues, issue("timeline_shot_missing", domain.ContinuityWarning, edit.EpisodeID, shot.SceneID, shot.ID, "Selected shot video is missing from the timeline"))
			}
		}
		for _, cue := range edit.AudioCues {
			if cue.StartSeconds+cue.DurationSeconds > total+0.01 {
				issues = append(issues, issue("timeline_audio_out_of_range", domain.ContinuityWarning, edit.EpisodeID, "", "", fmt.Sprintf("Audio cue %s extends beyond the video track", cue.ID)))
			}
		}
		for _, cue := range edit.SubtitleCues {
			if cue.StartSeconds+cue.DurationSeconds > total+0.01 {
				issues = append(issues, issue("timeline_subtitle_out_of_range", domain.ContinuityWarning, edit.EpisodeID, "", "", fmt.Sprintf("Subtitle cue %s extends beyond the video track", cue.ID)))
			}
		}
		for _, overlap := range overlappingAudioCues(edit.AudioCues, domain.LaneDialogue) {
			issues = append(issues, issue("timeline_dialogue_overlap", domain.ContinuityWarning, edit.EpisodeID, "", "", fmt.Sprintf("Dialogue cues %s and %s overlap", overlap[0], overlap[1])))
		}
		for _, overlap := range overlappingSubtitleCues(edit.SubtitleCues) {
			issues = append(issues, issue("timeline_subtitle_overlap", domain.ContinuityWarning, edit.EpisodeID, "", "", fmt.Sprintf("Subtitle cues %s and %s overlap", overlap[0], overlap[1])))
		}
	}
	return issues
}

func overlappingAudioCues(cues []domain.AudioCue, lane domain.AudioLane) [][2]string {
	type interval struct {
		id         string
		start, end float64
	}
	values := make([]interval, 0, len(cues))
	for _, cue := range cues {
		if cue.Lane == lane {
			values = append(values, interval{id: cue.ID, start: cue.StartSeconds, end: cue.StartSeconds + cue.DurationSeconds})
		}
	}
	sort.Slice(values, func(i, j int) bool { return values[i].start < values[j].start })
	result := make([][2]string, 0)
	for i := 1; i < len(values); i++ {
		if values[i].start < values[i-1].end-0.001 {
			result = append(result, [2]string{values[i-1].id, values[i].id})
		}
	}
	return result
}

func overlappingSubtitleCues(cues []domain.SubtitleCue) [][2]string {
	ordered := append([]domain.SubtitleCue(nil), cues...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].StartSeconds < ordered[j].StartSeconds })
	result := make([][2]string, 0)
	for i := 1; i < len(ordered); i++ {
		if ordered[i].StartSeconds < ordered[i-1].StartSeconds+ordered[i-1].DurationSeconds-0.001 {
			result = append(result, [2]string{ordered[i-1].ID, ordered[i].ID})
		}
	}
	return result
}

func issue(code string, severity domain.ContinuitySeverity, episodeID, sceneID, shotID, message string) domain.ContinuityIssue {
	return domain.ContinuityIssue{Code: code, Severity: severity, EpisodeID: episodeID, SceneID: sceneID, ShotID: shotID, Message: message}
}

func oppositeDirection(left, right string) bool {
	left, right = strings.ToLower(strings.TrimSpace(left)), strings.ToLower(strings.TrimSpace(right))
	return left != "" && right != "" && ((strings.Contains(left, "left") && strings.Contains(right, "right")) || (strings.Contains(left, "right") && strings.Contains(right, "left")))
}

func hasExtractedFrame(assets []domain.Asset, videoID, role string) bool {
	for _, asset := range assets {
		if asset.Kind != domain.AssetKindReference || asset.Provenance.Provider != "local-ffmpeg" || asset.Provenance.Parameters["frameRole"] != role {
			continue
		}
		for _, input := range asset.Inputs {
			if input.AssetID == videoID && input.Role == "source_video" {
				return true
			}
		}
	}
	return false
}
