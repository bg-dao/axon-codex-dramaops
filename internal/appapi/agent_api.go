package appapi

import (
	"context"
	"time"

	"github.com/bg-dao/axon-codex-sceneops/internal/appserver"
)

type AgentAPI struct{ backend *Backend }

func NewAgentAPI(backend *Backend) *AgentAPI { return &AgentAPI{backend: backend} }

func (a *AgentAPI) Start() error { return a.backend.StartSession(defaultMCPCommand()) }

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
