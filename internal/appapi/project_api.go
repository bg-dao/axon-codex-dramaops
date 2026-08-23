package appapi

import (
	"errors"
	"strings"

	"github.com/bg-dao/axon-codex-sceneops/internal/domain"
	"github.com/bg-dao/axon-codex-sceneops/internal/exporter"
	"github.com/bg-dao/axon-codex-sceneops/internal/project"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type ProjectAPI struct{ backend *Backend }

func NewProjectAPI(backend *Backend) *ProjectAPI { return &ProjectAPI{backend: backend} }

func (a *ProjectAPI) ChooseDirectory(title string) (string, error) {
	if strings.TrimSpace(title) == "" {
		title = "Choose a SceneOps project folder"
	}
	return wailsruntime.OpenDirectoryDialog(a.backend.context(), wailsruntime.OpenDialogOptions{Title: title})
}

func (a *ProjectAPI) Create(root, name string) (domain.Snapshot, error) {
	snapshot, err := a.backend.store.Create(root, name)
	if err != nil {
		return domain.Snapshot{}, err
	}
	if err := a.backend.SetProject(snapshot.Root); err != nil {
		return domain.Snapshot{}, err
	}
	a.backend.emit(EventProjectChanged, snapshot)
	return snapshot, nil
}

func (a *ProjectAPI) Open(root string) (domain.Snapshot, error) {
	snapshot, err := a.backend.store.Open(root)
	if err != nil {
		return domain.Snapshot{}, err
	}
	if err := a.backend.SetProject(snapshot.Root); err != nil {
		return domain.Snapshot{}, err
	}
	return snapshot, nil
}

func (a *ProjectAPI) Current() (domain.Snapshot, error) {
	root, err := a.backend.Root()
	if err != nil {
		return domain.Snapshot{}, err
	}
	return a.backend.store.Open(root)
}

func (a *ProjectAPI) SaveBrief(markdown string) (domain.Snapshot, error) {
	root, err := a.backend.Root()
	if err != nil {
		return domain.Snapshot{}, err
	}
	if err := a.backend.store.SaveBrief(root, markdown); err != nil {
		return domain.Snapshot{}, err
	}
	snapshot, err := a.Current()
	if err == nil {
		a.backend.emit(EventProjectChanged, snapshot)
	}
	return snapshot, err
}

func (a *ProjectAPI) SaveScene(scene domain.Scene) (domain.Snapshot, error) {
	root, err := a.backend.Root()
	if err != nil {
		return domain.Snapshot{}, err
	}
	if err := a.backend.store.SaveScene(root, scene); err != nil {
		return domain.Snapshot{}, err
	}
	if err := project.RebuildIndex(root); err != nil {
		return domain.Snapshot{}, err
	}
	return a.Current()
}

func (a *ProjectAPI) SaveShot(shot domain.Shot) (domain.Snapshot, error) {
	root, err := a.backend.Root()
	if err != nil {
		return domain.Snapshot{}, err
	}
	if err := a.backend.store.SaveShot(root, shot); err != nil {
		return domain.Snapshot{}, err
	}
	if err := project.RebuildIndex(root); err != nil {
		return domain.Snapshot{}, err
	}
	return a.Current()
}

func (a *ProjectAPI) ApplyStoryboard(scenes []domain.Scene, shots []domain.Shot, approved bool) (domain.Snapshot, error) {
	if !approved {
		return domain.Snapshot{}, errors.New("storyboard write requires explicit approval")
	}
	root, err := a.backend.Root()
	if err != nil {
		return domain.Snapshot{}, err
	}
	snapshot, err := a.backend.store.ApplyStoryboard(root, scenes, shots)
	if err == nil {
		a.backend.emit(EventProjectChanged, snapshot)
	}
	return snapshot, err
}

func (a *ProjectAPI) RebuildIndex() error {
	root, err := a.backend.Root()
	if err != nil {
		return err
	}
	return project.RebuildIndex(root)
}

func (a *ProjectAPI) Export() (exporter.Result, error) {
	root, err := a.backend.Root()
	if err != nil {
		return exporter.Result{}, err
	}
	return exporter.Project(root)
}
