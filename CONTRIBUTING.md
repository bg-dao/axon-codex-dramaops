# Contributing

Thanks for helping build DramaOps.

1. Open an issue for user-visible behavior or schema changes.
2. Keep changes focused and include tests for storage, protocol, provider, or UI state behavior.
3. Run `go test -race ./...`, `npm --prefix frontend test`, and `npm --prefix frontend run build`.
4. Never commit credentials, creator media, `.dramaops/` runtime state, or generated exports.

Core schema changes require an explicit versioning decision and must preserve the ability to rebuild SQLite from manifests alone. Provider additions implement only the capability interfaces they support (`ImageProvider`, `VideoProvider`, `SpeechProvider`, or `SoundProvider`) and must not add provider-specific fields to core types.
