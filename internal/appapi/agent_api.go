package appapi

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/bg-dao/axon-codex-sceneops/internal/appserver"
)

type AgentAPI struct{ backend *Backend }

const storyboardPrompt = `Build the initial SceneOps storyboard from the current creative brief.

Rules:
1. Call sceneops_project_read first and use its brief as the source of truth.
2. Create exactly 3 scenes and 6 shots total. Use stable IDs such as scene-01 and shot-01.
3. Give every shot a concise title and a production-ready visual prompt covering composition, lighting, camera, mood, and motion.
4. Default every shot to 16:9 and 4 seconds.
5. Call sceneops_storyboard_apply exactly once.
6. Do not edit files, execute shell commands, generate paid media, or call any other SceneOps write tool.`

func NewAgentAPI(backend *Backend) *AgentAPI { return &AgentAPI{backend: backend} }

func (a *AgentAPI) Start() error {
	if session, err := a.backend.Session(); err == nil && session.Running() {
		return nil
	}
	return a.backend.StartSession(defaultMCPCommand())
}

func (a *AgentAPI) Account() (map[string]any, error) {
	session, err := a.backend.Session()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(a.backend.context(), 20*time.Second)
	defer cancel()
	return session.Account(ctx)
}

func (a *AgentAPI) StartChatGPTLogin() (map[string]any, error) {
	session, err := a.backend.Session()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(a.backend.context(), 20*time.Second)
	defer cancel()
	return session.StartChatGPTLogin(ctx)
}

func (a *AgentAPI) StartTurn(prompt string) (appserver.Turn, error) {
	session, err := a.backend.Session()
	if err != nil {
		return appserver.Turn{}, err
	}
	ctx, cancel := context.WithTimeout(a.backend.context(), 45*time.Second)
	defer cancel()
	return session.StartTurn(ctx, prompt)
}

func (a *AgentAPI) GenerateStoryboard() (appserver.Turn, error) {
	root, err := a.backend.Root()
	if err != nil {
		return appserver.Turn{}, err
	}
	snapshot, err := a.backend.store.Open(root)
	if err != nil {
		return appserver.Turn{}, err
	}
	brief := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(snapshot.Brief), "# Creative brief"))
	if brief == "" || brief == "Describe the story, audience, visual language, and delivery constraints." {
		return appserver.Turn{}, errors.New("save a creative brief before generating a storyboard")
	}
	if len(snapshot.Scenes) > 0 || len(snapshot.Shots) > 0 {
		return appserver.Turn{}, errors.New("storyboard already exists; refine the existing scenes and shots instead")
	}
	if err := a.Start(); err != nil {
		return appserver.Turn{}, err
	}
	return a.StartTurn(storyboardPrompt)
}

func (a *AgentAPI) InterruptTurn(threadID, turnID string) error {
	session, err := a.backend.Session()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(a.backend.context(), 10*time.Second)
	defer cancel()
	return session.InterruptTurn(ctx, threadID, turnID)
}

func (a *AgentAPI) ResolveApproval(requestID, decision string) error {
	session, err := a.backend.Session()
	if err != nil {
		return err
	}
	err = session.ResolveApproval(requestID, decision)
	if err == nil {
		a.backend.emit(EventApprovalResolved, map[string]any{"requestId": requestID, "decision": decision})
	}
	return err
}
