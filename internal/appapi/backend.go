package appapi

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"sync"
	"time"

	"github.com/bg-dao/axon-codex-dramaops/internal/approval"
	"github.com/bg-dao/axon-codex-dramaops/internal/appserver"
	"github.com/bg-dao/axon-codex-dramaops/internal/media"
	"github.com/bg-dao/axon-codex-dramaops/internal/project"
	"github.com/bg-dao/axon-codex-dramaops/internal/provider"
	renderengine "github.com/bg-dao/axon-codex-dramaops/internal/render"
	codexruntime "github.com/bg-dao/axon-codex-dramaops/internal/runtime"
	"github.com/bg-dao/axon-codex-dramaops/internal/secret"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	EventAgent             = "dramaops:agent-event"
	EventApprovalRequested = "dramaops:approval-requested"
	EventApprovalResolved  = "dramaops:approval-resolved"
	EventRunUpdated        = "dramaops:run-updated"
	EventProjectChanged    = "dramaops:project-changed"
	EventRuntimeProgress   = "dramaops:runtime-progress"
	EventRenderProgress    = "dramaops:render-progress"
)

type Backend struct {
	mu               sync.RWMutex
	continuityMu     sync.Mutex
	ctx              context.Context
	store            *project.Store
	secrets          secret.Store
	runtimeManager   *codexruntime.Manager
	imageProvider    provider.ImageProvider
	videoProvider    provider.VideoProvider
	speechProvider   provider.SpeechProvider
	root             string
	gate             *approval.FileGate
	approvalOverride approval.Gate
	media            *media.Service
	session          *appserver.Session
	monitorCancel    context.CancelFunc
	projectCtx       context.Context
	projectCancel    context.CancelFunc
	renderCancels    map[string]context.CancelFunc
	renderRuntime    renderengine.RuntimeStatus
}

func NewBackend() *Backend {
	backend := &Backend{store: project.NewStore(), secrets: secret.NewKeyringStore(), renderCancels: make(map[string]context.CancelFunc)}
	backend.runtimeManager = &codexruntime.Manager{Progress: func(progress codexruntime.Progress) {
		backend.emit(EventRuntimeProgress, progress)
	}}
	openAI := provider.NewOpenAI(func() (string, error) { return backend.secrets.Get(secret.OpenAIKeyEntry) })
	backend.imageProvider, backend.videoProvider, backend.speechProvider = openAI, openAI, openAI
	return backend
}

func NewBackendForTest(secrets secret.Store, mediaProvider provider.MediaProvider) *Backend {
	backend := NewBackend()
	backend.secrets = secrets
	backend.imageProvider, backend.videoProvider = mediaProvider, mediaProvider
	if speech, ok := mediaProvider.(provider.SpeechProvider); ok {
		backend.speechProvider = speech
	}
	return backend
}

func NewBackendForProviders(secrets secret.Store, image provider.ImageProvider, video provider.VideoProvider, speech provider.SpeechProvider) *Backend {
	backend := NewBackend()
	backend.secrets, backend.imageProvider, backend.videoProvider, backend.speechProvider = secrets, image, video, speech
	return backend
}

func (b *Backend) Startup(ctx context.Context) { b.ctx = ctx }

func (b *Backend) Shutdown(_ context.Context) {
	b.mu.Lock()
	if b.monitorCancel != nil {
		b.monitorCancel()
	}
	if b.projectCancel != nil {
		b.projectCancel()
	}
	for _, cancel := range b.renderCancels {
		cancel()
	}
	b.renderCancels = make(map[string]context.CancelFunc)
	session := b.session
	b.session = nil
	b.mu.Unlock()
	if session != nil {
		_ = session.Close()
	}
}

func (b *Backend) SetProject(root string) error {
	snapshot, err := b.store.Open(root)
	if err != nil {
		return err
	}
	root = snapshot.Root
	b.mu.RLock()
	if b.root == root && b.media != nil {
		b.mu.RUnlock()
		return nil
	}
	b.mu.RUnlock()

	gate := approval.NewFileGate(root)
	if pending, pendingErr := gate.Pending(); pendingErr != nil {
		return pendingErr
	} else {
		for _, request := range pending {
			if _, resolveErr := gate.Resolve(request.ID, false); resolveErr != nil {
				return resolveErr
			}
		}
	}
	mediaService := &media.Service{
		Root: root, Store: b.store, Image: b.imageProvider, Video: b.videoProvider, Speech: b.speechProvider, Approval: gate,
		ResolveVoice: func(profileID string) (string, error) { return secret.ResolveVoiceBinding(b.secrets, profileID) },
	}
	if err := mediaService.RecoverInterruptedRuns(); err != nil {
		return err
	}

	b.mu.Lock()
	if b.root == root && b.media != nil {
		b.mu.Unlock()
		return nil
	}
	if b.monitorCancel != nil {
		b.monitorCancel()
	}
	if b.projectCancel != nil {
		b.projectCancel()
	}
	for _, cancel := range b.renderCancels {
		cancel()
	}
	b.renderCancels = make(map[string]context.CancelFunc)
	oldSession := b.session
	b.session = nil
	b.root = root
	b.gate = gate
	b.media = mediaService
	b.projectCtx, b.projectCancel = context.WithCancel(b.context())
	monitorCtx, cancel := context.WithCancel(b.projectCtx)
	b.monitorCancel = cancel
	b.mu.Unlock()
	if oldSession != nil {
		_ = oldSession.Close()
	}
	go b.monitor(monitorCtx, root)
	go NewRenderAPI(b).recover(root)
	return nil
}

func (b *Backend) Root() (string, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.root == "" {
		return "", errors.New("no DramaOps project is open")
	}
	return b.root, nil
}

func (b *Backend) Media() (*media.Service, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.media == nil {
		return nil, errors.New("no DramaOps project is open")
	}
	return b.media, nil
}

func (b *Backend) Gate() (*approval.FileGate, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.gate == nil {
		return nil, errors.New("no DramaOps project is open")
	}
	return b.gate, nil
}

func (b *Backend) ApprovalGate() (approval.Gate, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.approvalOverride != nil {
		return b.approvalOverride, nil
	}
	if b.gate == nil {
		return nil, errors.New("no DramaOps approval gate is available")
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

func (b *Backend) projectContext() context.Context {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.projectCtx != nil {
		return b.projectCtx
	}
	return b.context()
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
