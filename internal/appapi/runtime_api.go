package appapi

import (
	"context"
	"time"

	codexruntime "github.com/bg-dao/axon-codex-sceneops/internal/runtime"
)

type RuntimeAPI struct{ backend *Backend }

func NewRuntimeAPI(backend *Backend) *RuntimeAPI { return &RuntimeAPI{backend: backend} }

func (a *RuntimeAPI) DetectCodex() codexruntime.Status {
	ctx, cancel := context.WithTimeout(a.backend.context(), 5*time.Second)
	defer cancel()
	return a.backend.runtimeManager.Detect(ctx)
}

func (a *RuntimeAPI) EnsureCodex() (codexruntime.Status, error) {
	return a.backend.runtimeManager.Ensure(a.backend.context())
}
