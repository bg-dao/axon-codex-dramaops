# SceneOps Agent Guide

This repository is a self-contained public project. Keep its source, dependencies, CI, and releases independently maintainable.

## Boundaries

- JSON manifests and media files are durable truth. SQLite must remain disposable and rebuildable.
- Never log, persist, export, or return a plaintext OpenAI API key.
- Project paths must be normalized relative paths. Reject absolute paths, `..`, and symlink traversal.
- Paid media generation, storyboard writes, and job cancellation require a fresh approval. Do not add a permanent allow option.
- Keep core project types provider-neutral. Provider and model names belong in provenance and provider adapters.
- Keep Codex stable-surface integration free of `experimentalApi` and dynamic tools.

## Verification

Run from the repository root:

```bash
go test -race ./...
npm --prefix frontend test
npm --prefix frontend run build
```

Real OpenAI calls are manual opt-in tests and must never run in default CI.
