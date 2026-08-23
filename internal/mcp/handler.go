package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/bg-dao/axon-codex-sceneops/internal/approval"
	"github.com/bg-dao/axon-codex-sceneops/internal/domain"
	"github.com/bg-dao/axon-codex-sceneops/internal/media"
	"github.com/bg-dao/axon-codex-sceneops/internal/project"
	"github.com/bg-dao/axon-codex-sceneops/internal/provider"
)

const (
	ToolProjectRead     = "sceneops_project_read"
	ToolStoryboardApply = "sceneops_storyboard_apply"
	ToolImageGenerate   = "sceneops_image_generate"
	ToolVideoGenerate   = "sceneops_video_generate"
	ToolJobStatus       = "sceneops_job_status"
	ToolJobCancel       = "sceneops_job_cancel"
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
	storyScene := object(map[string]any{
		"id":      map[string]any{"type": "string", "pattern": "^[A-Za-z0-9][A-Za-z0-9_-]{0,127}$"},
		"title":   map[string]any{"type": "string", "minLength": 1},
		"summary": map[string]any{"type": "string"},
	}, "id", "title")
	storyShot := object(map[string]any{
		"id":              map[string]any{"type": "string", "pattern": "^[A-Za-z0-9][A-Za-z0-9_-]{0,127}$"},
		"sceneId":         map[string]any{"type": "string"},
		"title":           map[string]any{"type": "string", "minLength": 1},
		"prompt":          map[string]any{"type": "string", "minLength": 1},
		"durationSeconds": map[string]any{"type": "integer", "enum": []int{4, 8, 12}},
		"aspectRatio":     map[string]any{"type": "string", "enum": []string{"16:9", "9:16", "1:1"}},
	}, "id", "sceneId", "title", "prompt")
	return []Tool{
		{Name: ToolProjectRead, Description: "Read the current SceneOps project, storyboard, assets, and runs.", InputSchema: object(map[string]any{})},
		{Name: ToolStoryboardApply, Description: "Apply a structured storyboard after explicit user approval.", InputSchema: object(map[string]any{
			"scenes": map[string]any{"type": "array", "minItems": 1, "items": storyScene},
			"shots":  map[string]any{"type": "array", "minItems": 1, "items": storyShot},
		}, "scenes", "shots")},
		{Name: ToolImageGenerate, Description: "Generate and persist a paid keyframe image after explicit user approval.", InputSchema: object(map[string]any{
			"shotId": map[string]any{"type": "string"}, "prompt": map[string]any{"type": "string"}, "model": map[string]any{"type": "string"}, "size": map[string]any{"type": "string"}, "quality": map[string]any{"type": "string"},
		}, "shotId", "prompt")},
		{Name: ToolVideoGenerate, Description: "Generate a paid video shot after explicit user approval when the provider reports video capability.", InputSchema: object(map[string]any{
			"shotId": map[string]any{"type": "string"}, "prompt": map[string]any{"type": "string"}, "model": map[string]any{"type": "string"}, "seconds": map[string]any{"type": "integer", "enum": []int{4, 8, 12}}, "size": map[string]any{"type": "string"}, "referenceAssetId": map[string]any{"type": "string"},
		}, "shotId", "prompt")},
		{Name: ToolJobStatus, Description: "Read and refresh a SceneOps media run.", InputSchema: object(map[string]any{"runId": map[string]any{"type": "string"}}, "runId")},
		{Name: ToolJobCancel, Description: "Cancel a media run after explicit user approval.", InputSchema: object(map[string]any{"runId": map[string]any{"type": "string"}}, "runId")},
	}
}

func (h *Handler) Call(ctx context.Context, name string, arguments json.RawMessage) (any, error) {
	switch name {
	case ToolProjectRead:
		return h.Store.Open(h.Root)
	case ToolStoryboardApply:
		var input struct {
			Scenes []domain.Scene `json:"scenes"`
			Shots  []domain.Shot  `json:"shots"`
		}
		if err := decodeArguments(arguments, &input); err != nil {
			return nil, err
		}
		if _, err := h.Approval.Request(ctx, approval.StoryboardApply, "Apply a structured storyboard to the project", map[string]any{"sceneCount": len(input.Scenes), "shotCount": len(input.Shots)}); err != nil {
			return nil, err
		}
		return h.Store.ApplyStoryboard(h.Root, input.Scenes, input.Shots)
	case ToolImageGenerate:
		var input struct {
			ShotID  string `json:"shotId"`
			Prompt  string `json:"prompt"`
			Model   string `json:"model"`
			Size    string `json:"size"`
			Quality string `json:"quality"`
		}
		if err := decodeArguments(arguments, &input); err != nil {
			return nil, err
		}
		return h.Media.GenerateImage(ctx, input.ShotID, provider.ImageRequest{Prompt: input.Prompt, Model: input.Model, Size: input.Size, Quality: input.Quality})
	case ToolVideoGenerate:
		var input struct {
			ShotID           string `json:"shotId"`
			Prompt           string `json:"prompt"`
			Model            string `json:"model"`
			Seconds          int    `json:"seconds"`
			Size             string `json:"size"`
			ReferenceAssetID string `json:"referenceAssetId"`
		}
		if err := decodeArguments(arguments, &input); err != nil {
			return nil, err
		}
		return h.Media.GenerateVideo(ctx, input.ShotID, provider.VideoRequest{Prompt: input.Prompt, Model: input.Model, Seconds: input.Seconds, Size: input.Size, ReferenceAssetID: input.ReferenceAssetID})
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
		return nil, fmt.Errorf("unknown SceneOps tool %q", name)
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
