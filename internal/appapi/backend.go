package appapi

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"sync"
	"time"

	"github.com/bg-dao/axon-codex-sceneops/internal/approval"
	"github.com/bg-dao/axon-codex-sceneops/internal/appserver"
	"github.com/bg-dao/axon-codex-sceneops/internal/media"
	"github.com/bg-dao/axon-codex-sceneops/internal/project"
	"github.com/bg-dao/axon-codex-sceneops/internal/provider"
	codexruntime "github.com/bg-dao/axon-codex-sceneops/internal/runtime"
	"github.com/bg-dao/axon-codex-sceneops/internal/secret"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	EventAgent             = "sceneops:agent-event"
	EventApprovalRequested = "sceneops:approval-requested"
	EventApprovalResolved  = "sceneops:approval-resolved"
	EventRunUpdated        = "sceneops:run-updated"
	EventProjectChanged    = "sceneops:project-changed"
	EventRuntimeProgress   = "sceneops:runtime-progress"
)

type Backend struct {
	mu             sync.RWMutex
	ctx            context.Context
	store          *project.Store
	secrets        secret.Store
	runtimeManager *codexruntime.Manager
	provider       provider.MediaProvider
	root           string
	gate           *approval.FileGate
	media          *media.Service
	session        *appserver.Session
	monitorCancel  context.CancelFunc
}

func NewBackend() *Backend {
	backend := &Backend{store: project.NewStore(), secrets: secret.NewKeyringStore()}
	backend.runtimeManager = &codexruntime.Manager{Progress: func(progress codexruntime.Progress) {
		backend.emit(EventRuntimeProgress, progress)
	}}
	backend.provider = provider.NewOpenAI(func() (string, error) { return backend.secrets.Get(secret.OpenAIKeyEntry) })
	return backend
}

func NewBackendForTest(secrets secret.Store, mediaProvider provider.MediaProvider) *Backend {
	backend := NewBackend()
	backend.secrets = secrets
	backend.provider = mediaProvider
	return backend
}

func (b *Backend) Startup(ctx context.Context) { b.ctx = ctx }

func (b *Backend) Shutdown(_ context.Context) {
	b.mu.Lock()
	if b.monitorCancel != nil {
		b.monitorCancel()
	}
	session := b.session
	b.session = nil
	b.mu.Unlock()
	if session != nil {
		_ = session.Close()
	}
}

func (b *Backend) SetProject(root string) error {
	if _, err := b.store.Open(root); err != nil {
		return err
	}
	b.mu.Lock()
	if b.monitorCancel != nil {
		b.monitorCancel()
	}
	b.root = root
	b.gate = approval.NewFileGate(root)
	b.media = &media.Service{Root: root, Store: b.store, Provider: b.provider, Approval: b.gate}
	monitorCtx, cancel := context.WithCancel(b.context())
	b.monitorCancel = cancel
	b.mu.Unlock()
	go b.monitor(monitorCtx, root)
	return nil
}

func (b *Backend) Root() (string, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.root == "" {
		return "", errors.New("no SceneOps project is open")
	}
	return b.root, nil
}

func (b *Backend) Media() (*media.Service, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.media == nil {
		return nil, errors.New("no SceneOps project is open")
	}
	return b.media, nil
}

func (b *Backend) Gate() (*approval.FileGate, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.gate == nil {
		return nil, errors.New("no SceneOps project is open")
	}
	return b.gate, nil
}

func (b *Backend) Session() (*appserver.Session, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.session == nil {
		return nil, errors.New("Codex app-server is not running")
	}
	return b.session, nil
}

func (b *Backend) StartSession(mcpCommand string) error {
	root, err := b.Root()
	if err != nil {
		return err
	}
	session := appserver.NewSession(b.runtimeManager, b.store, func(event appserver.Event) {
		b.emit(EventAgent, event)
		if event.RequestID != "" && (event.Method == "item/commandExecution/requestApproval" || event.Method == "item/fileChange/requestApproval") {
			b.emit(EventApprovalRequested, event)
		}
	})
	if err := session.Start(b.context(), root, mcpCommand); err != nil {
		return err
	}
	b.mu.Lock()
	old := b.session
	b.session = session
	b.mu.Unlock()
	if old != nil {
		_ = old.Close()
	}
	return nil
}

func (b *Backend) context() context.Context {
	if b.ctx != nil {
		return b.ctx
	}
	return context.Background()
}

func (b *Backend) emit(name string, payload any) {
	if b.ctx != nil {
		runtime.EventsEmit(b.ctx, name, payload)
	}
}

func (b *Backend) monitor(ctx context.Context, root string) {
	ticker := time.NewTicker(600 * time.Millisecond)
	defer ticker.Stop()
	seenApprovals := make(map[string]struct{})
	seenRuns := make(map[string]string)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		b.mu.RLock()
		if b.root != root || b.gate == nil {
			b.mu.RUnlock()
			return
		}
		gate := b.gate
		b.mu.RUnlock()
		if pending, err := gate.Pending(); err == nil {
			for _, request := range pending {
				if _, ok := seenApprovals[request.ID]; !ok {
					seenApprovals[request.ID] = struct{}{}
					b.emit(EventApprovalRequested, request)
				}
			}
		}
		if snapshot, err := b.store.Open(root); err == nil {
			changed := false
			for _, run := range snapshot.Runs {
				encoded, _ := json.Marshal(run)
				current := string(encoded)
				if seenRuns[run.ID] != current {
					seenRuns[run.ID] = current
					b.emit(EventRunUpdated, run)
					changed = true
				}
			}
			if changed {
				b.emit(EventProjectChanged, map[string]any{"root": root})
			}
		}
	}
}

func defaultMCPCommand() string {
	path, _ := os.Executable()
	return path
}
