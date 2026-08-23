package project

import (
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/bg-dao/axon-codex-dramaops/internal/domain"
)

func validateProject(value domain.Project) error {
	if err := ValidateID(value.ID); err != nil {
		return fmt.Errorf("project id: %w", err)
	}
	if strings.TrimSpace(value.Name) == "" {
		return errors.New("series name is required")
	}
	if strings.TrimSpace(value.ContentLanguage) == "" {
		return errors.New("content language is required")
	}
	if value.ActiveEpisodeID != "" {
		if err := ValidateID(value.ActiveEpisodeID); err != nil {
			return fmt.Errorf("active episode id: %w", err)
		}
	}
	return validateOutput(value.Output)
}

func validateOutput(value domain.OutputSettings) error {
	if value.Orientation != domain.OrientationPortrait && value.Orientation != domain.OrientationLandscape {
		return fmt.Errorf("unsupported output orientation %q", value.Orientation)
	}
	if value.Width <= 0 || value.Height <= 0 || value.FPS <= 0 || value.FPS > 120 {
		return errors.New("output dimensions and frame rate must be positive")
	}
	if strings.TrimSpace(value.VideoCodec) == "" || strings.TrimSpace(value.AudioCodec) == "" || value.AudioSampleRate <= 0 || value.AudioChannels < 1 || value.AudioChannels > 8 {
		return errors.New("output codec and audio settings are invalid")
	}
	if value.LoudnessLUFS > 0 || value.LoudnessLUFS < -70 || value.TruePeakDBTP > 0 || value.TruePeakDBTP < -20 {
		return errors.New("output loudness settings are invalid")
	}
	if value.SubtitleSafeArea < 0 || value.SubtitleSafeArea > 0.5 {
		return errors.New("subtitle safe area is invalid")
	}
	return nil
}

func validateEpisode(value domain.Episode) error {
	if err := ValidateID(value.ID); err != nil {
		return err
	}
	if strings.TrimSpace(value.Title) == "" || value.Number < 1 {
		return errors.New("episode title and positive number are required")
	}
	switch value.Status {
	case domain.EpisodeDraft, domain.EpisodePlanning, domain.EpisodeProduction, domain.EpisodeEditing, domain.EpisodeComplete:
	default:
		return fmt.Errorf("unsupported episode status %q", value.Status)
	}
	if err := validateUniqueIDs(value.SceneIDs, "episode scene"); err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, block := range value.ScriptBlocks {
		if err := ValidateID(block.ID); err != nil {
			return fmt.Errorf("script block: %w", err)
		}
		if seen[block.ID] {
			return fmt.Errorf("duplicate script block id %q", block.ID)
		}
		seen[block.ID] = true
		if err := ValidateID(block.SceneID); err != nil {
			return fmt.Errorf("script block scene: %w", err)
		}
		if block.CharacterID != "" {
			if err := ValidateID(block.CharacterID); err != nil {
				return fmt.Errorf("script block character: %w", err)
			}
		}
		if block.SelectedVoiceAssetID != "" {
			if err := ValidateID(block.SelectedVoiceAssetID); err != nil {
				return fmt.Errorf("selected voice asset: %w", err)
			}
		}
		if err := validateScriptBlock(block); err != nil {
			return err
		}
	}
	return nil
}

func validateCharacter(value domain.Character) error {
	if err := ValidateID(value.ID); err != nil {
		return err
	}
	if strings.TrimSpace(value.Name) == "" {
		return errors.New("character name is required")
	}
	if err := ValidateID(value.VoiceProfile.ID); err != nil {
		return fmt.Errorf("voice profile: %w", err)
	}
	switch value.VoiceProfile.Kind {
	case domain.VoiceBuiltIn:
		if strings.TrimSpace(value.VoiceProfile.BuiltInVoice) == "" {
			return errors.New("built-in Voice Profile requires a voice")
		}
	case domain.VoiceCustom:
		if !value.VoiceProfile.ConsentConfirmed {
			return errors.New("custom Voice Profile requires confirmed consent")
		}
	case domain.VoiceExternal:
		if err := ValidateID(value.VoiceProfile.ExternalAssetID); err != nil {
			return errors.New("external Voice Profile requires a valid audio asset")
		}
	default:
		return fmt.Errorf("unsupported Voice Profile kind %q", value.VoiceProfile.Kind)
	}
	return validateUniqueIDs(value.ReferenceAssets, "character reference")
}

func validateScene(value domain.Scene) error {
	if err := ValidateID(value.ID); err != nil {
		return err
	}
	if err := ValidateID(value.EpisodeID); err != nil {
		return fmt.Errorf("scene episode: %w", err)
	}
	if strings.TrimSpace(value.Title) == "" || value.Order < 0 {
		return errors.New("scene title and non-negative order are required")
	}
	if value.LocationID != "" {
		if err := ValidateID(value.LocationID); err != nil {
			return fmt.Errorf("scene location: %w", err)
		}
	}
	return validateUniqueIDs(value.ShotIDs, "scene shot")
}

func validateShot(value domain.Shot) error {
	if err := ValidateID(value.ID); err != nil {
		return err
	}
	if err := ValidateID(value.EpisodeID); err != nil {
		return fmt.Errorf("shot episode: %w", err)
	}
	if err := ValidateID(value.SceneID); err != nil {
		return fmt.Errorf("shot scene: %w", err)
	}
	if strings.TrimSpace(value.Title) == "" || value.Order < 0 || value.DurationSeconds <= 0 || value.DurationSeconds > 30 {
		return errors.New("shot title, order, and duration are invalid")
	}
	if value.AspectRatio != "9:16" && value.AspectRatio != "16:9" {
		return fmt.Errorf("unsupported shot aspect ratio %q", value.AspectRatio)
	}
	switch value.ShotSize {
	case domain.ShotECU, domain.ShotCU, domain.ShotMCU, domain.ShotMS, domain.ShotMLS, domain.ShotLS, domain.ShotELS:
	default:
		return fmt.Errorf("unsupported shot size %q", value.ShotSize)
	}
	switch value.CameraAngle {
	case domain.AngleEyeLevel, domain.AngleHigh, domain.AngleLow, domain.AngleOverhead, domain.AngleDutch, domain.AnglePOV, domain.AngleOTS:
	default:
		return fmt.Errorf("unsupported camera angle %q", value.CameraAngle)
	}
	switch value.CameraMovement {
	case domain.MovementStatic, domain.MovementPan, domain.MovementTilt, domain.MovementDolly, domain.MovementTruck, domain.MovementPedestal, domain.MovementOrbit, domain.MovementHandheld, domain.MovementCrane, domain.MovementZoom:
	default:
		return fmt.Errorf("unsupported camera movement %q", value.CameraMovement)
	}
	if value.LensMM != 0 && (value.LensMM < 8 || value.LensMM > 400) {
		return errors.New("shot lens must be between 8mm and 400mm")
	}
	switch value.Transition {
	case domain.TransitionCut, domain.TransitionDissolve, domain.TransitionFade:
	default:
		return fmt.Errorf("unsupported transition %q", value.Transition)
	}
	for label, ids := range map[string][]string{
		"script block": value.ScriptBlockIDs, "character": value.CharacterIDs, "prop": value.PropIDs, "reference": value.ReferenceAssets,
	} {
		if err := validateUniqueIDs(ids, "shot "+label); err != nil {
			return err
		}
	}
	for label, id := range map[string]string{"selected keyframe": value.SelectedKeyframeAssetID, "selected video": value.SelectedVideoAssetID} {
		if id != "" {
			if err := ValidateID(id); err != nil {
				return fmt.Errorf("%s: %w", label, err)
			}
		}
	}
	return nil
}

func validateAsset(root string, value domain.Asset) error {
	if err := ValidateID(value.ID); err != nil {
		return err
	}
	if !supportedAssetKind(value.Kind) {
		return fmt.Errorf("unsupported asset kind %q", value.Kind)
	}
	if _, err := ResolveRelative(root, value.RelativePath); err != nil {
		return fmt.Errorf("asset path: %w", err)
	}
	digest, err := hex.DecodeString(value.SHA256)
	if err != nil || len(digest) != 32 {
		return errors.New("asset SHA-256 is invalid")
	}
	for label, id := range map[string]string{
		"episode": value.EpisodeID, "shot": value.ShotID, "script block": value.ScriptBlockID, "run": value.RunID,
	} {
		if id != "" {
			if err := ValidateID(id); err != nil {
				return fmt.Errorf("asset %s: %w", label, err)
			}
		}
	}
	seen := map[string]bool{}
	for _, input := range value.Inputs {
		if err := ValidateID(input.AssetID); err != nil {
			return fmt.Errorf("asset input: %w", err)
		}
		if input.AssetID == value.ID {
			return errors.New("asset cannot reference itself")
		}
		if strings.TrimSpace(input.Role) == "" {
			return errors.New("asset input role is required")
		}
		key := input.AssetID + "\x00" + input.Role
		if seen[key] {
			return fmt.Errorf("duplicate asset input %s (%s)", input.AssetID, input.Role)
		}
		seen[key] = true
	}
	if strings.TrimSpace(value.Provenance.Provider) == "" {
		return errors.New("asset provenance provider is required")
	}
	return nil
}

func validateRun(value domain.Run) error {
	if err := ValidateID(value.ID); err != nil {
		return err
	}
	if strings.TrimSpace(value.Operation) == "" {
		return errors.New("run operation is required")
	}
	switch value.Status {
	case domain.RunQueued, domain.RunAwaitingApproval, domain.RunRunning, domain.RunSucceeded, domain.RunFailed, domain.RunCancelled:
	default:
		return fmt.Errorf("unsupported run status %q", value.Status)
	}
	if value.Progress < 0 || value.Progress > 100 {
		return errors.New("run progress must be between 0 and 100")
	}
	for label, id := range map[string]string{"episode": value.EpisodeID, "shot": value.ShotID, "script block": value.ScriptBlockID} {
		if id != "" {
			if err := ValidateID(id); err != nil {
				return fmt.Errorf("run %s: %w", label, err)
			}
		}
	}
	return nil
}

func validateUniqueIDs(values []string, label string) error {
	seen := map[string]bool{}
	for _, id := range values {
		if err := ValidateID(id); err != nil {
			return fmt.Errorf("%s: %w", label, err)
		}
		if seen[id] {
			return fmt.Errorf("duplicate %s id %q", label, id)
		}
		seen[id] = true
	}
	return nil
}

func validateSnapshotSemantics(snapshot domain.Snapshot) error {
	if err := validateProject(snapshot.Project); err != nil {
		return fmt.Errorf("project: %w", err)
	}
	for _, value := range snapshot.Episodes {
		if err := validateEpisode(value); err != nil {
			return fmt.Errorf("episode %s: %w", value.ID, err)
		}
	}
	for _, value := range snapshot.Characters {
		if err := validateCharacter(value); err != nil {
			return fmt.Errorf("character %s: %w", value.ID, err)
		}
	}
	for _, value := range snapshot.Locations {
		if err := ValidateID(value.ID); err != nil || strings.TrimSpace(value.Name) == "" {
			return fmt.Errorf("location %s is invalid", value.ID)
		}
		if err := validateUniqueIDs(value.ReferenceAssets, "location reference"); err != nil {
			return err
		}
	}
	for _, value := range snapshot.Props {
		if err := ValidateID(value.ID); err != nil || strings.TrimSpace(value.Name) == "" {
			return fmt.Errorf("prop %s is invalid", value.ID)
		}
		if err := validateUniqueIDs(value.ReferenceAssets, "prop reference"); err != nil {
			return err
		}
	}
	for _, value := range snapshot.Scenes {
		if err := validateScene(value); err != nil {
			return fmt.Errorf("scene %s: %w", value.ID, err)
		}
	}
	for _, value := range snapshot.Shots {
		if err := validateShot(value); err != nil {
			return fmt.Errorf("shot %s: %w", value.ID, err)
		}
	}
	for _, value := range snapshot.Edits {
		if err := validateEdit(value); err != nil {
			return fmt.Errorf("edit %s: %w", value.EpisodeID, err)
		}
		if err := validateOutput(value.Output); err != nil {
			return fmt.Errorf("edit %s output: %w", value.EpisodeID, err)
		}
	}
	for _, value := range snapshot.Assets {
		if err := validateAsset(snapshot.Root, value); err != nil {
			return fmt.Errorf("asset %s: %w", value.ID, err)
		}
	}
	for _, value := range snapshot.Runs {
		if err := validateRun(value); err != nil {
			return fmt.Errorf("run %s: %w", value.ID, err)
		}
	}
	return validateSnapshotRelationships(snapshot)
}

func validateSnapshotRelationships(snapshot domain.Snapshot) error {
	episodes := make(map[string]domain.Episode, len(snapshot.Episodes))
	characters := make(map[string]domain.Character, len(snapshot.Characters))
	locations := make(map[string]domain.Location, len(snapshot.Locations))
	props := make(map[string]domain.Prop, len(snapshot.Props))
	scenes := make(map[string]domain.Scene, len(snapshot.Scenes))
	shots := make(map[string]domain.Shot, len(snapshot.Shots))
	assets := make(map[string]domain.Asset, len(snapshot.Assets))
	runs := make(map[string]domain.Run, len(snapshot.Runs))
	for _, value := range snapshot.Episodes {
		episodes[value.ID] = value
	}
	for _, value := range snapshot.Characters {
		characters[value.ID] = value
	}
	for _, value := range snapshot.Locations {
		locations[value.ID] = value
	}
	for _, value := range snapshot.Props {
		props[value.ID] = value
	}
	for _, value := range snapshot.Scenes {
		scenes[value.ID] = value
	}
	for _, value := range snapshot.Shots {
		shots[value.ID] = value
	}
	for _, value := range snapshot.Assets {
		assets[value.ID] = value
	}
	for _, value := range snapshot.Runs {
		runs[value.ID] = value
	}
	if snapshot.Project.ActiveEpisodeID != "" {
		if _, ok := episodes[snapshot.Project.ActiveEpisodeID]; !ok {
			return fmt.Errorf("active episode %s does not exist", snapshot.Project.ActiveEpisodeID)
		}
	}
	assetKind := func(id, label string, allowed ...domain.AssetKind) (domain.Asset, error) {
		asset, ok := assets[id]
		if !ok {
			return domain.Asset{}, fmt.Errorf("%s references missing asset %s", label, id)
		}
		for _, kind := range allowed {
			if asset.Kind == kind {
				return asset, nil
			}
		}
		return domain.Asset{}, fmt.Errorf("%s references asset %s with invalid kind %s", label, id, asset.Kind)
	}
	visualReferences := func(ids []string, label string) error {
		for _, id := range ids {
			if _, err := assetKind(id, label, domain.AssetKindReference, domain.AssetKindImage); err != nil {
				return err
			}
		}
		return nil
	}
	if err := visualReferences(snapshot.Project.StyleBible.ReferenceAssets, "style bible"); err != nil {
		return err
	}
	for _, id := range append(append([]string{}, snapshot.Project.SoundPalette.AmbienceAssetIDs...), snapshot.Project.SoundPalette.BGMAssetIDs...) {
		if _, err := assetKind(id, "sound palette", domain.AssetKindAudio); err != nil {
			return err
		}
	}
	for _, id := range snapshot.Project.SoundPalette.Motifs {
		if _, err := assetKind(id, "sound motif", domain.AssetKindAudio); err != nil {
			return err
		}
	}
	for _, character := range snapshot.Characters {
		if err := visualReferences(character.ReferenceAssets, "character "+character.ID); err != nil {
			return err
		}
		if character.VoiceProfile.Kind == domain.VoiceExternal {
			if _, err := assetKind(character.VoiceProfile.ExternalAssetID, "external voice profile "+character.VoiceProfile.ID, domain.AssetKindAudio); err != nil {
				return err
			}
		}
	}
	for _, location := range snapshot.Locations {
		if err := visualReferences(location.ReferenceAssets, "location "+location.ID); err != nil {
			return err
		}
	}
	for _, prop := range snapshot.Props {
		if err := visualReferences(prop.ReferenceAssets, "prop "+prop.ID); err != nil {
			return err
		}
	}
	for _, episode := range snapshot.Episodes {
		listedScenes := make(map[string]bool, len(episode.SceneIDs))
		for _, sceneID := range episode.SceneIDs {
			scene, ok := scenes[sceneID]
			if !ok || scene.EpisodeID != episode.ID {
				return fmt.Errorf("episode %s references invalid scene %s", episode.ID, sceneID)
			}
			listedScenes[sceneID] = true
		}
		for _, block := range episode.ScriptBlocks {
			scene, ok := scenes[block.SceneID]
			if !ok || scene.EpisodeID != episode.ID || !listedScenes[block.SceneID] {
				return fmt.Errorf("script block %s references invalid scene %s", block.ID, block.SceneID)
			}
			if block.CharacterID != "" {
				if _, ok := characters[block.CharacterID]; !ok {
					return fmt.Errorf("script block %s references missing character %s", block.ID, block.CharacterID)
				}
			}
			if block.SelectedVoiceAssetID != "" {
				asset, err := assetKind(block.SelectedVoiceAssetID, "script block "+block.ID, domain.AssetKindAudio)
				if err != nil || asset.EpisodeID != episode.ID || asset.ScriptBlockID != block.ID {
					return fmt.Errorf("script block %s references invalid voice asset %s", block.ID, block.SelectedVoiceAssetID)
				}
			}
		}
	}
	for _, scene := range snapshot.Scenes {
		episode, ok := episodes[scene.EpisodeID]
		if !ok || !containsID(episode.SceneIDs, scene.ID) {
			return fmt.Errorf("scene %s is not linked to episode %s", scene.ID, scene.EpisodeID)
		}
		if scene.LocationID != "" {
			if _, ok := locations[scene.LocationID]; !ok {
				return fmt.Errorf("scene %s references missing location %s", scene.ID, scene.LocationID)
			}
		}
		for _, shotID := range scene.ShotIDs {
			shot, ok := shots[shotID]
			if !ok || shot.SceneID != scene.ID || shot.EpisodeID != scene.EpisodeID {
				return fmt.Errorf("scene %s references invalid shot %s", scene.ID, shotID)
			}
		}
	}
	for _, shot := range snapshot.Shots {
		scene, ok := scenes[shot.SceneID]
		if !ok || scene.EpisodeID != shot.EpisodeID || !containsID(scene.ShotIDs, shot.ID) {
			return fmt.Errorf("shot %s is not linked to scene %s", shot.ID, shot.SceneID)
		}
		episode := episodes[shot.EpisodeID]
		for _, blockID := range shot.ScriptBlockIDs {
			if !episodeHasBlock(episode, blockID) {
				return fmt.Errorf("shot %s references missing script block %s", shot.ID, blockID)
			}
		}
		for _, characterID := range shot.CharacterIDs {
			if _, ok := characters[characterID]; !ok {
				return fmt.Errorf("shot %s references missing character %s", shot.ID, characterID)
			}
		}
		for _, propID := range shot.PropIDs {
			if _, ok := props[propID]; !ok {
				return fmt.Errorf("shot %s references missing prop %s", shot.ID, propID)
			}
		}
		if err := visualReferences(shot.ReferenceAssets, "shot "+shot.ID); err != nil {
			return err
		}
		if shot.SelectedKeyframeAssetID != "" {
			asset, err := assetKind(shot.SelectedKeyframeAssetID, "shot "+shot.ID, domain.AssetKindImage)
			if err != nil || asset.ShotID != shot.ID {
				return fmt.Errorf("shot %s references invalid selected keyframe %s", shot.ID, shot.SelectedKeyframeAssetID)
			}
		}
		if shot.SelectedVideoAssetID != "" {
			asset, err := assetKind(shot.SelectedVideoAssetID, "shot "+shot.ID, domain.AssetKindVideo)
			if err != nil || asset.ShotID != shot.ID {
				return fmt.Errorf("shot %s references invalid selected video %s", shot.ID, shot.SelectedVideoAssetID)
			}
		}
	}
	for _, asset := range snapshot.Assets {
		if asset.EpisodeID != "" {
			if _, ok := episodes[asset.EpisodeID]; !ok {
				return fmt.Errorf("asset %s references missing episode %s", asset.ID, asset.EpisodeID)
			}
		}
		if asset.ShotID != "" {
			shot, ok := shots[asset.ShotID]
			if !ok || (asset.EpisodeID != "" && shot.EpisodeID != asset.EpisodeID) {
				return fmt.Errorf("asset %s references invalid shot %s", asset.ID, asset.ShotID)
			}
		}
		if asset.ScriptBlockID != "" {
			episode, ok := episodes[asset.EpisodeID]
			if !ok || !episodeHasBlock(episode, asset.ScriptBlockID) {
				return fmt.Errorf("asset %s references invalid script block %s", asset.ID, asset.ScriptBlockID)
			}
		}
		if asset.RunID != "" {
			if _, ok := runs[asset.RunID]; !ok {
				return fmt.Errorf("asset %s references missing run %s", asset.ID, asset.RunID)
			}
		}
		for _, input := range asset.Inputs {
			if _, ok := assets[input.AssetID]; !ok {
				return fmt.Errorf("asset %s references missing input %s", asset.ID, input.AssetID)
			}
		}
	}
	for _, run := range snapshot.Runs {
		if run.EpisodeID != "" {
			if _, ok := episodes[run.EpisodeID]; !ok {
				return fmt.Errorf("run %s references missing episode %s", run.ID, run.EpisodeID)
			}
		}
		if run.ShotID != "" {
			shot, ok := shots[run.ShotID]
			if !ok || (run.EpisodeID != "" && shot.EpisodeID != run.EpisodeID) {
				return fmt.Errorf("run %s references invalid shot %s", run.ID, run.ShotID)
			}
		}
		if run.ScriptBlockID != "" {
			episode, ok := episodes[run.EpisodeID]
			if !ok || !episodeHasBlock(episode, run.ScriptBlockID) {
				return fmt.Errorf("run %s references invalid script block %s", run.ID, run.ScriptBlockID)
			}
		}
	}
	for _, edit := range snapshot.Edits {
		if _, ok := episodes[edit.EpisodeID]; !ok {
			return fmt.Errorf("edit references missing episode %s", edit.EpisodeID)
		}
		for _, clip := range edit.VideoTrack {
			shot, ok := shots[clip.ShotID]
			asset, assetOK := assets[clip.AssetID]
			if !ok || shot.EpisodeID != edit.EpisodeID || !assetOK || asset.Kind != domain.AssetKindVideo || asset.ShotID != clip.ShotID {
				return fmt.Errorf("edit %s contains invalid video clip %s", edit.EpisodeID, clip.ID)
			}
		}
		for _, cue := range edit.AudioCues {
			if _, err := assetKind(cue.AssetID, "audio cue "+cue.ID, domain.AssetKindAudio); err != nil {
				return err
			}
		}
	}
	return nil
}

func containsID(values []string, id string) bool {
	for _, value := range values {
		if value == id {
			return true
		}
	}
	return false
}

func episodeHasBlock(episode domain.Episode, id string) bool {
	for _, block := range episode.ScriptBlocks {
		if block.ID == id {
			return true
		}
	}
	return false
}
