package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/bg-dao/axon-codex-dramaops/internal/approval"
	"github.com/bg-dao/axon-codex-dramaops/internal/domain"
	"github.com/bg-dao/axon-codex-dramaops/internal/media"
	"github.com/bg-dao/axon-codex-dramaops/internal/project"
	"github.com/bg-dao/axon-codex-dramaops/internal/provider"
)

const (
	ToolProjectRead    = "dramaops_project_read"
	ToolScriptApply    = "dramaops_script_apply"
	ToolShotPlanApply  = "dramaops_shotplan_apply"
	ToolImageGenerate  = "dramaops_image_generate"
	ToolVideoGenerate  = "dramaops_video_generate"
	ToolSpeechGenerate = "dramaops_speech_generate"
	ToolJobStatus      = "dramaops_job_status"
	ToolJobCancel      = "dramaops_job_cancel"
)

type Handler struct {
	Root     string
	Store    *project.Store
	Media    *media.Service
	Approval approval.Gate
}

func (h *Handler) Tools() []Tool {
	object := func(properties map[string]any, required ...string) map[string]any {
		schema := map[string]any{"type": "object", "properties": properties, "additionalProperties": false}
		if len(required) > 0 {
			schema["required"] = required
		}
		return schema
	}
	stringID := map[string]any{"type": "string", "pattern": "^[A-Za-z0-9][A-Za-z0-9_-]{0,127}$"}
	stringValue := map[string]any{"type": "string"}
	stringList := map[string]any{"type": "array", "items": stringID}
	block := object(map[string]any{
		"id": stringID, "sceneId": stringID,
		"kind":        map[string]any{"type": "string", "enum": []string{"action", "dialogue", "voice_over", "sfx", "music"}},
		"characterId": stringID, "text": map[string]any{"type": "string", "minLength": 1}, "emotion": stringValue,
	}, "id", "sceneId", "kind", "text")
	episode := object(map[string]any{
		"id": stringID, "title": map[string]any{"type": "string", "minLength": 1}, "logline": stringValue, "synopsis": stringValue,
		"scriptBlocks": map[string]any{"type": "array", "minItems": 1, "items": block},
	}, "id", "title", "scriptBlocks")
	scene := object(map[string]any{
		"id": stringID, "title": map[string]any{"type": "string", "minLength": 1}, "summary": stringValue,
		"locationId": stringID, "timeOfDay": stringValue,
	}, "id", "title")
	voice := object(map[string]any{
		"id": stringID, "kind": map[string]any{"type": "string", "enum": []string{"built_in", "external"}},
		"name": stringValue, "builtInVoice": stringValue, "externalAssetId": stringID,
	}, "id", "kind", "name")
	character := object(map[string]any{
		"id": stringID, "name": map[string]any{"type": "string", "minLength": 1}, "description": stringValue,
		"appearance": stringValue, "wardrobe": stringValue, "negativePrompt": stringValue,
		"referenceAssets": stringList, "voiceProfile": voice,
	}, "id", "name")
	location := object(map[string]any{
		"id": stringID, "name": map[string]any{"type": "string", "minLength": 1}, "description": stringValue,
		"continuityNotes": stringValue, "referenceAssets": stringList,
	}, "id", "name")
	prop := object(map[string]any{
		"id": stringID, "name": map[string]any{"type": "string", "minLength": 1}, "description": stringValue,
		"continuityState": stringValue, "referenceAssets": stringList,
	}, "id", "name")
	shot := object(map[string]any{
		"id": stringID, "sceneId": stringID, "title": map[string]any{"type": "string", "minLength": 1}, "scriptBlockIds": stringList,
		"prompt": map[string]any{"type": "string", "minLength": 1}, "durationSeconds": map[string]any{"type": "number", "minimum": 0.1, "maximum": 30},
		"aspectRatio":    map[string]any{"type": "string", "enum": []string{"9:16", "16:9"}},
		"shotSize":       map[string]any{"type": "string", "enum": []string{"ECU", "CU", "MCU", "MS", "MLS", "LS", "ELS"}},
		"cameraAngle":    map[string]any{"type": "string", "enum": []string{"eye_level", "high", "low", "overhead", "dutch", "pov", "over_the_shoulder"}},
		"cameraMovement": map[string]any{"type": "string", "enum": []string{"static", "pan", "tilt", "dolly", "truck", "pedestal", "orbit", "handheld", "crane", "zoom"}},
		"lensMm":         map[string]any{"type": "integer", "minimum": 8, "maximum": 400}, "composition": stringValue, "focusSubject": stringValue,
		"blocking": stringValue, "lighting": stringValue, "screenDirection": stringValue, "eyeLine": stringValue,
		"characterIds": stringList, "propIds": stringList, "wardrobeContinuity": stringValue, "propContinuity": stringValue,
		"transition": map[string]any{"type": "string", "enum": []string{"cut", "dissolve", "fade"}},
	}, "id", "sceneId", "title", "scriptBlockIds", "prompt", "durationSeconds", "aspectRatio", "shotSize", "cameraAngle", "cameraMovement", "lensMm", "composition", "focusSubject", "blocking", "lighting", "screenDirection", "eyeLine", "characterIds", "propIds", "wardrobeContinuity", "propContinuity", "transition")
	return []Tool{
		{Name: ToolProjectRead, Description: "Read the DramaOps series, active episodes, bibles, shot plan, timeline, assets, runs, and derived continuity issues.", InputSchema: object(map[string]any{})},
		{Name: ToolScriptApply, Description: "Apply the initial structured episode script and series bible after explicit approval.", InputSchema: object(map[string]any{
			"episode": episode, "scenes": map[string]any{"type": "array", "minItems": 1, "items": scene},
			"characters": map[string]any{"type": "array", "items": character}, "locations": map[string]any{"type": "array", "items": location}, "props": map[string]any{"type": "array", "items": prop},
		}, "episode", "scenes")},
		{Name: ToolShotPlanApply, Description: "Apply the initial professional shot plan for an episode after explicit approval.", InputSchema: object(map[string]any{"episodeId": stringID, "shots": map[string]any{"type": "array", "minItems": 1, "items": shot}}, "episodeId", "shots")},
		{Name: ToolImageGenerate, Description: "Generate a consistency-aware paid keyframe after explicit approval.", InputSchema: object(map[string]any{"shotId": stringID, "prompt": stringValue, "model": stringValue, "size": stringValue, "quality": stringValue}, "shotId", "prompt")},
		{Name: ToolVideoGenerate, Description: "Generate an experimental paid video clip after explicit approval when available.", InputSchema: object(map[string]any{"shotId": stringID, "prompt": stringValue, "model": stringValue, "seconds": map[string]any{"type": "integer", "enum": []int{4, 8, 12}}, "size": stringValue}, "shotId", "prompt")},
		{Name: ToolSpeechGenerate, Description: "Generate paid dialogue using the character's locked Voice Profile after explicit approval.", InputSchema: object(map[string]any{"episodeId": stringID, "scriptBlockId": stringID, "model": stringValue, "instructions": stringValue, "speed": map[string]any{"type": "number", "minimum": 0.25, "maximum": 4}}, "episodeId", "scriptBlockId")},
		{Name: ToolJobStatus, Description: "Read and refresh a DramaOps provider job.", InputSchema: object(map[string]any{"runId": stringID}, "runId")},
		{Name: ToolJobCancel, Description: "Cancel an active provider job after explicit approval.", InputSchema: object(map[string]any{"runId": stringID}, "runId")},
	}
}

func (h *Handler) Call(ctx context.Context, name string, arguments json.RawMessage) (any, error) {
	switch name {
	case ToolProjectRead:
		return h.Store.Open(h.Root)
	case ToolScriptApply:
		var input project.ScriptPlan
		if err := decodeArguments(arguments, &input); err != nil {
			return nil, err
		}
		if _, err := h.Approval.Request(ctx, approval.ScriptApply, "Apply a structured episode script and series bible", map[string]any{"episodeId": input.Episode.ID, "sceneCount": len(input.Scenes), "blockCount": len(input.Episode.ScriptBlocks)}); err != nil {
			return nil, err
		}
		return h.Store.ApplyScript(h.Root, input)
	case ToolShotPlanApply:
		var input struct {
			EpisodeID string        `json:"episodeId"`
			Shots     []domain.Shot `json:"shots"`
		}
		if err := decodeArguments(arguments, &input); err != nil {
			return nil, err
		}
		if _, err := h.Approval.Request(ctx, approval.ShotPlanApply, "Apply a professional episode shot plan", map[string]any{"episodeId": input.EpisodeID, "shotCount": len(input.Shots)}); err != nil {
			return nil, err
		}
		return h.Store.ApplyShotPlan(h.Root, input.EpisodeID, input.Shots)
	case ToolImageGenerate:
		var input struct{ ShotID, Prompt, Model, Size, Quality string }
		if err := decodeArguments(arguments, &input); err != nil {
			return nil, err
		}
		return h.Media.GenerateImage(ctx, input.ShotID, provider.ImageRequest{Prompt: input.Prompt, Model: input.Model, Size: input.Size, Quality: input.Quality})
	case ToolVideoGenerate:
		var input struct {
			ShotID, Prompt, Model, Size string
			Seconds                     int
		}
		if err := decodeArguments(arguments, &input); err != nil {
			return nil, err
		}
		return h.Media.GenerateVideo(ctx, input.ShotID, provider.VideoRequest{Prompt: input.Prompt, Model: input.Model, Size: input.Size, Seconds: input.Seconds})
	case ToolSpeechGenerate:
		var input struct {
			EpisodeID     string  `json:"episodeId"`
			ScriptBlockID string  `json:"scriptBlockId"`
			Model         string  `json:"model"`
			Instructions  string  `json:"instructions"`
			Speed         float64 `json:"speed"`
		}
		if err := decodeArguments(arguments, &input); err != nil {
			return nil, err
		}
		return h.Media.GenerateSpeech(ctx, input.EpisodeID, input.ScriptBlockID, provider.SpeechRequest{Model: input.Model, Instructions: input.Instructions, Speed: input.Speed})
	case ToolJobStatus:
		var input struct {
			RunID string `json:"runId"`
		}
		if err := decodeArguments(arguments, &input); err != nil {
			return nil, err
		}
		return h.Media.GetRun(ctx, input.RunID)
	case ToolJobCancel:
		var input struct {
			RunID string `json:"runId"`
		}
		if err := decodeArguments(arguments, &input); err != nil {
			return nil, err
		}
		return h.Media.CancelRun(ctx, input.RunID)
	default:
		return nil, fmt.Errorf("unknown DramaOps tool %q", name)
	}
}

func decodeArguments(raw json.RawMessage, output any) error {
	if len(raw) == 0 {
		raw = []byte("{}")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(output); err != nil {
		return fmt.Errorf("invalid tool arguments: %w", err)
	}
	return nil
}
