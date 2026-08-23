package appserver

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bg-dao/axon-codex-dramaops/internal/project"
	codexruntime "github.com/bg-dao/axon-codex-dramaops/internal/runtime"
)

type Session struct {
	mu             sync.Mutex
	ctx            context.Context
	cancel         context.CancelFunc
	root           string
	mcpCommand     string
	runtimeManager *codexruntime.Manager
	store          *project.Store
	client         *Client
	onEvent        func(Event)
	restarts       int
	closed         bool
}

type Thread struct {
	ID string `json:"id"`
}

type ThreadResult struct {
	Thread Thread `json:"thread"`
}

type Turn struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

type TurnResult struct {
	Turn Turn `json:"turn"`
}

func NewSession(runtimeManager *codexruntime.Manager, store *project.Store, onEvent func(Event)) *Session {
	return &Session{runtimeManager: runtimeManager, store: store, onEvent: onEvent}
}

func (s *Session) Start(parent context.Context, root, mcpCommand string) error {
	if _, err := s.store.Open(root); err != nil {
		return err
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	if mcpCommand == "" {
		mcpCommand, err = os.Executable()
		if err != nil {
			return err
		}
	}
	ctx, cancel := context.WithCancel(parent)
	s.mu.Lock()
	if s.client != nil {
		s.mu.Unlock()
		cancel()
		return errors.New("project app-server session is already running")
	}
	s.ctx = ctx
	s.cancel = cancel
	s.root = absRoot
	s.mcpCommand = mcpCommand
	s.closed = false
	s.restarts = 0
	s.mu.Unlock()
	return s.boot()
}

func (s *Session) boot() error {
	status, err := s.runtimeManager.Ensure(s.ctx)
	if err != nil {
		return err
	}
	args := AppServerArgs(s.mcpCommand, s.root)
	client, err := StartClient(s.ctx, status.Path, args, s.onEvent)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.client = client
	s.mu.Unlock()
	snapshot, openErr := s.store.Open(s.root)
	if openErr == nil && snapshot.Project.ActiveThreadID != "" {
		resumeCtx, cancel := context.WithTimeout(s.ctx, 20*time.Second)
		var resumed ThreadResult
		resumeErr := client.Request(resumeCtx, "thread/resume", map[string]any{
			"threadId": snapshot.Project.ActiveThreadID,
			"cwd":      s.root,
		}, &resumed)
		cancel()
		if resumeErr != nil {
			s.onEventSafe(Event{Method: "dramaops/thread/resumeFailed", Params: mustJSON(map[string]any{"threadId": snapshot.Project.ActiveThreadID, "message": resumeErr.Error()}), Timestamp: time.Now().UTC()})
		}
	}
	go s.monitor(client)
	return nil
}

func (s *Session) monitor(client *Client) {
	<-client.Done()
	s.mu.Lock()
	if s.closed || s.client != client {
		s.mu.Unlock()
		return
	}
	if s.restarts >= 1 {
		s.client = nil
		s.mu.Unlock()
		s.onEventSafe(Event{Method: "dramaops/runtime/failed", Params: mustJSON(map[string]any{"message": "Codex app-server exited after its one automatic restart"}), Timestamp: time.Now().UTC()})
		return
	}
	s.restarts++
	s.client = nil
	s.mu.Unlock()
	s.onEventSafe(Event{Method: "dramaops/runtime/restarting", Params: mustJSON(map[string]any{"attempt": 1}), Timestamp: time.Now().UTC()})
	select {
	case <-s.ctx.Done():
		return
	case <-time.After(350 * time.Millisecond):
	}
	if err := s.boot(); err != nil {
		s.onEventSafe(Event{Method: "dramaops/runtime/failed", Params: mustJSON(map[string]any{"message": err.Error()}), Timestamp: time.Now().UTC()})
	}
}

func (s *Session) Account(ctx context.Context) (map[string]any, error) {
	client, err := s.activeClient()
	if err != nil {
		return nil, err
	}
	var result map[string]any
	err = client.Request(ctx, "account/read", map[string]any{"refreshToken": false}, &result)
	return result, err
}

func (s *Session) StartChatGPTLogin(ctx context.Context) (map[string]any, error) {
	client, err := s.activeClient()
	if err != nil {
		return nil, err
	}
	var result map[string]any
	err = client.Request(ctx, "account/login/start", map[string]any{"type": "chatgpt", "useHostedLoginSuccessPage": true, "appBrand": "chatgpt"}, &result)
	return result, err
}

func (s *Session) EnsureThread(ctx context.Context) (Thread, error) {
	snapshot, err := s.store.Open(s.root)
	if err != nil {
		return Thread{}, err
	}
	client, err := s.activeClient()
	if err != nil {
		return Thread{}, err
	}
	if snapshot.Project.ActiveThreadID != "" {
		var resumed ThreadResult
		if err := client.Request(ctx, "thread/resume", map[string]any{"threadId": snapshot.Project.ActiveThreadID, "cwd": s.root}, &resumed); err == nil {
			return resumed.Thread, nil
		}
	}
	var started ThreadResult
	if err := client.Request(ctx, "thread/start", map[string]any{
		"cwd":            s.root,
		"approvalPolicy": "onRequest",
		"sandbox":        "workspaceWrite",
		"serviceName":    "dramaops",
	}, &started); err != nil {
		return Thread{}, err
	}
	if started.Thread.ID == "" {
		return Thread{}, errors.New("app-server returned an empty thread id")
	}
	snapshot.Project.ActiveThreadID = started.Thread.ID
	if err := s.store.SaveProject(s.root, snapshot.Project); err != nil {
		return Thread{}, err
	}
	return started.Thread, nil
}

func (s *Session) StartTurn(ctx context.Context, prompt string) (Turn, error) {
	if strings.TrimSpace(prompt) == "" {
		return Turn{}, errors.New("turn prompt is required")
	}
	thread, err := s.EnsureThread(ctx)
	if err != nil {
		return Turn{}, err
	}
	client, err := s.activeClient()
	if err != nil {
		return Turn{}, err
	}
	var result TurnResult
	if err := client.Request(ctx, "turn/start", map[string]any{
		"threadId":       thread.ID,
		"input":          []map[string]any{{"type": "text", "text": prompt}},
		"cwd":            s.root,
		"approvalPolicy": "onRequest",
		"sandboxPolicy": map[string]any{
			"type":          "workspaceWrite",
			"writableRoots": []string{s.root},
			"networkAccess": true,
		},
	}, &result); err != nil {
		return Turn{}, err
	}
	return result.Turn, nil
}

func (s *Session) InterruptTurn(ctx context.Context, threadID, turnID string) error {
	client, err := s.activeClient()
	if err != nil {
		return err
	}
	return client.Request(ctx, "turn/interrupt", map[string]any{"threadId": threadID, "turnId": turnID}, nil)
}

func (s *Session) ResolveApproval(requestID, decision string) error {
	client, err := s.activeClient()
	if err != nil {
		return err
	}
	return client.RespondServerRequest(requestID, decision)
}

func (s *Session) Running() bool {
	_, err := s.activeClient()
	return err == nil
}

func (s *Session) Close() error {
	s.mu.Lock()
	s.closed = true
	cancel := s.cancel
	client := s.client
	s.client = nil
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if client != nil {
		return client.Close()
	}
	return nil
}

func (s *Session) activeClient() (*Client, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.client == nil {
		return nil, errors.New("Codex app-server is not running")
	}
	return s.client, nil
}

func (s *Session) onEventSafe(event Event) {
	if s.onEvent != nil {
		s.onEvent(event)
	}
}

func AppServerArgs(mcpCommand, projectRoot string) []string {
	quoted := func(value string) string { return strconv.Quote(value) }
	argsValue := "[" + strings.Join([]string{quoted("mcp"), quoted("--project"), quoted(projectRoot)}, ",") + "]"
	return []string{
		"app-server",
		"--stdio",
		"--config", fmt.Sprintf("mcp_servers.dramaops.command=%s", quoted(mcpCommand)),
		"--config", fmt.Sprintf("mcp_servers.dramaops.args=%s", argsValue),
		"--config", "mcp_servers.dramaops.required=true",
	}
}
