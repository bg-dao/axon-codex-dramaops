package secret

import (
	"errors"

	"github.com/zalando/go-keyring"
)

const (
	ServiceName    = "dev.bg-dao.sceneops"
	OpenAIKeyEntry = "openai-media-api-key"
)

var ErrNotFound = errors.New("secret not found")

type Store interface {
	Set(account, value string) error
	Get(account string) (string, error)
	Delete(account string) error
	Exists(account string) bool
}

type KeyringStore struct{}

func NewKeyringStore() *KeyringStore { return &KeyringStore{} }

func (s *KeyringStore) Set(account, value string) error {
	if value == "" {
		return errors.New("secret value is empty")
	}
	return keyring.Set(ServiceName, account, value)
}

func (s *KeyringStore) Get(account string) (string, error) {
	value, err := keyring.Get(ServiceName, account)
	if errors.Is(err, keyring.ErrNotFound) {
		return "", ErrNotFound
	}
	return value, err
}

func (s *KeyringStore) Delete(account string) error {
	err := keyring.Delete(ServiceName, account)
	if errors.Is(err, keyring.ErrNotFound) {
		return nil
	}
	return err
}

func (s *KeyringStore) Exists(account string) bool {
	value, err := s.Get(account)
	return err == nil && value != ""
}

type MemoryStore struct {
	values map[string]string
}

func NewMemoryStore() *MemoryStore { return &MemoryStore{values: make(map[string]string)} }

func (s *MemoryStore) Set(account, value string) error {
	if value == "" {
		return errors.New("secret value is empty")
	}
	s.values[account] = value
	return nil
}

func (s *MemoryStore) Get(account string) (string, error) {
	value, ok := s.values[account]
	if !ok {
		return "", ErrNotFound
	}
	return value, nil
}

func (s *MemoryStore) Delete(account string) error {
	delete(s.values, account)
	return nil
}

func (s *MemoryStore) Exists(account string) bool { _, ok := s.values[account]; return ok }
