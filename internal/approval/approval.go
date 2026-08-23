package approval

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/bg-dao/axon-codex-dramaops/internal/project"
	"github.com/google/uuid"
)

type Action string

const (
	ScriptApply    Action = "script_apply"
	ShotPlanApply  Action = "shotplan_apply"
	ImageGenerate  Action = "image_generate"
	VideoGenerate  Action = "video_generate"
	SpeechGenerate Action = "speech_generate"
	VoiceCreate    Action = "voice_create"
	JobCancel      Action = "job_cancel"
)

type Request struct {
	ID          string         `json:"id"`
	Action      Action         `json:"action"`
	Summary     string         `json:"summary"`
	Details     map[string]any `json:"details,omitempty"`
	RequestedAt time.Time      `json:"requestedAt"`
}

type Decision struct {
	RequestID string    `json:"requestId"`
	Approved  bool      `json:"approved"`
	DecidedAt time.Time `json:"decidedAt"`
}

type Gate interface {
	Request(ctx context.Context, action Action, summary string, details map[string]any) (Request, error)
}

type FileGate struct {
	Root         string
	PollInterval time.Duration
}

func NewFileGate(root string) *FileGate {
	return &FileGate{Root: root, PollInterval: 200 * time.Millisecond}
}

func (g *FileGate) Request(ctx context.Context, action Action, summary string, details map[string]any) (Request, error) {
	request := Request{ID: uuid.NewString(), Action: action, Summary: summary, Details: details, RequestedAt: time.Now().UTC()}
	path, err := g.requestPath(request.ID)
	if err != nil {
		return Request{}, err
	}
	if err := project.AtomicWriteJSON(path, request); err != nil {
		return Request{}, err
	}
	ticker := time.NewTicker(g.PollInterval)
	defer ticker.Stop()
	for {
		decision, found, err := g.readDecision(request.ID)
		if err != nil {
			return request, err
		}
		if found {
			if decision.Approved {
				return request, nil
			}
			return request, errors.New("operation declined by user")
		}
		select {
		case <-ctx.Done():
			return request, ctx.Err()
		case <-ticker.C:
		}
	}
}

func (g *FileGate) Resolve(id string, approved bool) (Decision, error) {
	if err := project.ValidateID(id); err != nil {
		return Decision{}, err
	}
	requestPath, err := g.requestPath(id)
	if err != nil {
		return Decision{}, err
	}
	if _, err := os.Stat(requestPath); err != nil {
		return Decision{}, fmt.Errorf("approval request not found: %w", err)
	}
	decision := Decision{RequestID: id, Approved: approved, DecidedAt: time.Now().UTC()}
	path, err := g.decisionPath(id)
	if err != nil {
		return Decision{}, err
	}
	if err := project.AtomicWriteJSON(path, decision); err != nil {
		return Decision{}, err
	}
	return decision, nil
}

func (g *FileGate) Pending() ([]Request, error) {
	root, err := filepath.Abs(g.Root)
	if err != nil {
		return nil, err
	}
	dir, err := project.ResolveRelative(root, filepath.Join(".dramaops", "approvals"))
	if err != nil {
		return nil, err
	}
	entries, err := filepath.Glob(filepath.Join(dir, "*.request.json"))
	if err != nil {
		return nil, err
	}
	result := make([]Request, 0, len(entries))
	for _, path := range entries {
		relative, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return nil, relErr
		}
		resolved, resolveErr := project.ResolveRelative(root, relative)
		if resolveErr != nil {
			return nil, resolveErr
		}
		var request Request
		data, readErr := os.ReadFile(resolved)
		if readErr != nil || json.Unmarshal(data, &request) != nil {
			continue
		}
		if _, found, readErr := g.readDecision(request.ID); readErr != nil {
			return nil, readErr
		} else if !found {
			result = append(result, request)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].RequestedAt.Before(result[j].RequestedAt) })
	return result, nil
}

func (g *FileGate) requestPath(id string) (string, error) {
	if err := project.ValidateID(id); err != nil {
		return "", err
	}
	return project.ResolveRelative(g.Root, filepath.Join(".dramaops", "approvals", id+".request.json"))
}

func (g *FileGate) decisionPath(id string) (string, error) {
	if err := project.ValidateID(id); err != nil {
		return "", err
	}
	return project.ResolveRelative(g.Root, filepath.Join(".dramaops", "approvals", id+".decision.json"))
}

func (g *FileGate) readDecision(id string) (Decision, bool, error) {
	path, err := g.decisionPath(id)
	if err != nil {
		return Decision{}, false, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Decision{}, false, nil
	}
	if err != nil {
		return Decision{}, false, err
	}
	var decision Decision
	if err := json.Unmarshal(data, &decision); err != nil {
		return Decision{}, false, err
	}
	return decision, true, nil
}

type AutoGate struct {
	Approved bool
}

func (g AutoGate) Request(_ context.Context, action Action, summary string, details map[string]any) (Request, error) {
	request := Request{ID: uuid.NewString(), Action: action, Summary: summary, Details: details, RequestedAt: time.Now().UTC()}
	if !g.Approved {
		return request, errors.New("operation declined by user")
	}
	return request, nil
}
