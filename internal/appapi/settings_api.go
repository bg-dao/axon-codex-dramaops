package appapi

import (
	"context"
	"time"

	"github.com/bg-dao/axon-codex-sceneops/internal/provider"
	"github.com/bg-dao/axon-codex-sceneops/internal/secret"
)

type SettingsStatus struct {
	OpenAIKeyConfigured bool                  `json:"openaiKeyConfigured"`
	KeychainService     string                `json:"keychainService"`
	Capabilities        provider.Capabilities `json:"capabilities"`
}

type SettingsAPI struct{ backend *Backend }

func NewSettingsAPI(backend *Backend) *SettingsAPI { return &SettingsAPI{backend: backend} }

func (a *SettingsAPI) Status() SettingsStatus {
	return SettingsStatus{OpenAIKeyConfigured: a.backend.secrets.Exists(secret.OpenAIKeyEntry), KeychainService: secret.ServiceName}
}

func (a *SettingsAPI) SetOpenAIKey(value string) error {
	return a.backend.secrets.Set(secret.OpenAIKeyEntry, value)
}

func (a *SettingsAPI) DeleteOpenAIKey() error {
	return a.backend.secrets.Delete(secret.OpenAIKeyEntry)
}

func (a *SettingsAPI) ProbeCapabilities() (provider.Capabilities, error) {
	ctx, cancel := context.WithTimeout(a.backend.context(), 20*time.Second)
	defer cancel()
	return a.backend.provider.Capabilities(ctx)
}
