package appapi

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/bg-dao/axon-codex-dramaops/internal/appserver"
	"github.com/bg-dao/axon-codex-dramaops/internal/domain"
)

type AgentAPI struct{ backend *Backend }

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
	if strings.TrimSpace(prompt) == "" {
		return appserver.Turn{}, errors.New("prompt is required")
	}
	session, err := a.backend.Session()
	if err != nil {
		return appserver.Turn{}, err
	}
	ctx, cancel := context.WithTimeout(a.backend.context(), 45*time.Second)
	defer cancel()
	return session.StartTurn(ctx, prompt)
}

func (a *AgentAPI) GenerateScript(episodeID string) (appserver.Turn, error) {
	snapshot, episode, err := a.episode(episodeID)
	if err != nil {
		return appserver.Turn{}, err
	}
	if len(episode.ScriptBlocks) > 0 || len(episode.SceneIDs) > 0 {
		return appserver.Turn{}, errors.New("episode script already exists; edit it directly instead")
	}
	if strings.TrimSpace(episode.Logline) == "" && strings.TrimSpace(episode.Synopsis) == "" {
		return appserver.Turn{}, errors.New("add an episode logline or synopsis before generating the script")
	}
	if err := a.Start(); err != nil {
		return appserver.Turn{}, err
	}
	prompt := fmt.Sprintf(`Create the initial structured short-drama script and series bible for episode %s.

Rules:
1. Call dramaops_project_read first. Use the target episode logline, synopsis, content language, output format, and existing series bible as truth.
2. Write a production-ready 60–90 second episode with exactly 3 scenes, clear dramatic escalation, and a final hook.
3. Use stable IDs prefixed by the episode ID. Script blocks may only be action, dialogue, voice_over, sfx, or music.
4. Define only the characters, locations, and props needed for this episode. Every character must have one built_in Voice Profile.
5. Call dramaops_script_apply exactly once for episode %s.
6. Do not write files, execute commands, create custom voices, generate media, or call another write tool.`, episodeID, episodeID)
	_ = snapshot
	return a.StartTurn(prompt)
}

func (a *AgentAPI) GenerateShotPlan(episodeID string) (appserver.Turn, error) {
	snapshot, episode, err := a.episode(episodeID)
	if err != nil {
		return appserver.Turn{}, err
	}
	if len(episode.ScriptBlocks) == 0 {
		return appserver.Turn{}, errors.New("create the episode script before planning shots")
	}
	for _, shot := range snapshot.Shots {
		if shot.EpisodeID == episodeID {
			return appserver.Turn{}, errors.New("episode shot plan already exists; edit shots directly instead")
		}
	}
	if err := a.Start(); err != nil {
		return appserver.Turn{}, err
	}
	prompt := fmt.Sprintf(`Build the professional short-drama shot plan for episode %s.

Rules:
1. Call dramaops_project_read first and use its episode script and bibles as truth.
2. Create exactly 8 shots covering every script block in story order. Match the project's output aspect ratio (normally 9:16) and use a total duration of 60–90 seconds.
3. Every shot must include shotSize, cameraAngle, cameraMovement, lensMm, composition, focusSubject, blocking, lighting, screenDirection, eyeLine, characterIds, propIds, wardrobeContinuity, propContinuity, transition, and a production-ready visual prompt.
4. Maintain the 180-degree rule, eye-line continuity, wardrobe, prop state, and screen direction unless the script explicitly motivates a change.
5. Call dramaops_shotplan_apply exactly once for episode %s.
6. Do not write files, execute commands, generate media, or call another write tool.`, episodeID, episodeID)
	return a.StartTurn(prompt)
}

func (a *AgentAPI) episode(episodeID string) (domain.Snapshot, domain.Episode, error) {
	root, err := a.backend.Root()
	if err != nil {
		return domain.Snapshot{}, domain.Episode{}, err
	}
	snapshot, err := a.backend.store.Open(root)
	if err != nil {
		return domain.Snapshot{}, domain.Episode{}, err
	}
	for _, episode := range snapshot.Episodes {
		if episode.ID == episodeID {
			return snapshot, episode, nil
		}
	}
	return snapshot, domain.Episode{}, fmt.Errorf("episode %s not found", episodeID)
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
