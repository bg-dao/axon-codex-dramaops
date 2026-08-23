# Security model

## Credentials

The OpenAI media API key is stored under Keychain service `dev.bg-dao.sceneops`, account `openai-media-api-key`. Wails exposes only configured/not-configured state. The plaintext value is never returned to React.

Codex authentication remains inside the Codex runtime and is accessed through the stable app-server account methods. SceneOps does not copy Codex tokens into its own settings.

## Project boundary

`ResolveRelative` rejects absolute paths, `..`, and symlink traversal. New writes use a temporary file in the destination directory, `fsync`, and atomic rename. Downloads are hashed before their manifest is committed.

## Approval boundary

The following operations require a fresh approval:

- structured storyboard apply;
- image generation;
- video generation;
- media job cancellation.

There is no "always approve paid generation" option. Codex command and file approvals expose only `accept`, `decline`, and `cancel` in SceneOps; session-wide acceptance is intentionally not supported.

## Runtime supply chain

SceneOps first detects a compatible system Codex. Fallback installation uses the checked-in `internal/runtime/codex-runtime.json`; it never follows `latest` at runtime. The downloaded archive must match its checked-in SHA-256 before extraction, archive traversal is rejected, and the installed binary must report the pinned version.

## Logging and export

Known API-key and bearer-token patterns are redacted from runtime and provider errors. `.sceneops/` is excluded from exports. No telemetry or cloud backend is enabled by default.
