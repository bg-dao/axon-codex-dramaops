# Security model

## Credentials and voice consent

The OpenAI media key is stored under macOS Keychain service `dev.bg-dao.dramaops`, account `openai-media-api-key`. The UI receives only configured/not-configured state. Codex authentication remains inside the Codex runtime.

Custom voices require explicit authorization plus a consent recording and sample. The provider voice ID and consent ID are bound to separate device-local Keychain entries. Provider voice IDs, consent IDs, recordings, samples, and API keys are excluded from snapshots, manifests, SQLite, logs, and exports.

## Project boundary

Project paths are normalized and relative. Absolute paths, parent traversal, and symlink components are rejected. Manifest and media writes use temporary sibling files, sync, and atomic rename. Imported or downloaded bytes are hashed before the manifest becomes durable.

## Approval boundary

A fresh approval is required for:

- Agent script apply;
- Agent shot-plan apply;
- image generation;
- video generation;
- speech generation;
- custom voice creation;
- provider-job cancellation.

Direct UI edits and local FFmpeg rendering do not trigger a second approval. There is no permanent approval option for paid work.

## Runtime supply chain

DramaOps prefers compatible system runtimes. Codex fallback installation is locked by repository manifest, architecture, version, archive type, and SHA-256; archive traversal and post-install version mismatches fail closed.

FFmpeg execution requires both `ffmpeg` and `ffprobe` plus `h264_videotoolbox`. A managed FFmpeg fallback may be enabled only by a checked-in artifact manifest with exact SHA-256 and bundled license/source-offer notices. An absent or unverified artifact is never downloaded or launched.

## Provider and export boundary

Provider/model details are isolated to adapters and provenance. Reference assets are resolved through the project path boundary and re-hashed before they are sent. Provider errors pass through secret-pattern redaction.

Project packages include story and edit manifests, assets, renders, SRT, Fountain, continuity reports, and provenance. `.dramaops/`, Git metadata, API keys, device-only voice bindings, consent recordings, and samples are excluded.
