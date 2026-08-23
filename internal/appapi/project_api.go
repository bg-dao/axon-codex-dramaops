package appapi

import (
	"strings"

	"github.com/bg-dao/axon-codex-dramaops/internal/domain"
	"github.com/bg-dao/axon-codex-dramaops/internal/exporter"
	"github.com/bg-dao/axon-codex-dramaops/internal/project"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type ProjectAPI struct{ backend *Backend }

func NewProjectAPI(backend *Backend) *ProjectAPI { return &ProjectAPI{backend: backend} }

func (a *ProjectAPI) ChooseDirectory(title string) (string, error) {
	if strings.TrimSpace(title) == "" {
		title = "Choose a DramaOps series folder"
	}
	return wailsruntime.OpenDirectoryDialog(a.backend.context(), wailsruntime.OpenDialogOptions{Title: title})
}

func (a *ProjectAPI) Create(root string, options project.CreateOptions) (domain.Snapshot, error) {
	snapshot, err := a.backend.store.CreateWithOptions(root, options)
	if err != nil {
		return domain.Snapshot{}, err
	}
	if err := a.backend.SetProject(snapshot.Root); err != nil {
		return domain.Snapshot{}, err
	}
	snapshot, err = a.backend.store.Open(snapshot.Root)
	if err != nil {
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
	return a.backend.store.Open(snapshot.Root)
}

func (a *ProjectAPI) Current() (domain.Snapshot, error) {
	root, err := a.backend.Root()
	if err != nil {
		return domain.Snapshot{}, err
	}
	return a.backend.store.Open(root)
}

func (a *ProjectAPI) SaveSettings(projectManifest domain.Project) (domain.Snapshot, error) {
	root, err := a.backend.Root()
	if err != nil {
		return domain.Snapshot{}, err
	}
	current, err := a.backend.store.Open(root)
	if err != nil {
		return domain.Snapshot{}, err
	}
	// Settings owns only content language and output. Preserve the active Codex
	// thread and series bibles even if the UI submitted an older snapshot.
	current.Project.ContentLanguage = projectManifest.ContentLanguage
	current.Project.Output = projectManifest.Output
	if err := a.backend.store.SaveProject(root, current.Project); err != nil {
		return domain.Snapshot{}, err
	}
	for _, edit := range current.Edits {
		edit.Output = current.Project.Output
		if err := a.backend.store.SaveEdit(root, edit); err != nil {
			return domain.Snapshot{}, err
		}
	}
	return a.refresh(root)
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

func (a *ProjectAPI) refresh(root string) (domain.Snapshot, error) {
	if err := project.RebuildIndex(root); err != nil {
		return domain.Snapshot{}, err
	}
	snapshot, err := a.backend.store.Open(root)
	if err == nil {
		a.backend.emit(EventProjectChanged, snapshot)
	}
	return snapshot, err
}
