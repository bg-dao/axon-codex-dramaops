package mcp

import (
	"context"
	"io"

	"github.com/bg-dao/axon-codex-sceneops/internal/approval"
	"github.com/bg-dao/axon-codex-sceneops/internal/media"
	"github.com/bg-dao/axon-codex-sceneops/internal/project"
	"github.com/bg-dao/axon-codex-sceneops/internal/provider"
	"github.com/bg-dao/axon-codex-sceneops/internal/secret"
)

func Run(ctx context.Context, root string, input io.Reader, output io.Writer) error {
	secrets := secret.NewKeyringStore()
	openAI := provider.NewOpenAI(func() (string, error) { return secrets.Get(secret.OpenAIKeyEntry) })
	gate := approval.NewFileGate(root)
	store := project.NewStore()
	mediaService := &media.Service{Root: root, Store: store, Provider: openAI, Approval: gate}
	server := &Server{Handler: &Handler{Root: root, Store: store, Media: mediaService, Approval: gate}}
	return server.Serve(ctx, input, output)
}
