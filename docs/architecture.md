# SceneOps architecture

SceneOps is a local Wails desktop application with two deliberately separate AI paths:

1. Codex app-server owns ChatGPT authentication, durable agent threads, turns, streamed events, command/file approvals, and the Codex sandbox.
2. `MediaProvider` owns paid image and video generation using a separate provider credential.

The separation prevents a ChatGPT login from becoming an implicit media-spend credential.

## Runtime topology

```text
SceneOps desktop process
├── Wails APIs and event bridge
├── Project store and rebuildable SQLite index
├── Codex app-server child process (one per active project)
│   └── SceneOps MCP child mode
└── Keychain-backed MediaProvider
```

The Go process starts `codex app-server --stdio` and communicates with newline-delimited JSON messages. It performs `initialize`/`initialized`, then uses stable `account/*`, `thread/*`, and `turn/*` methods. It does not opt into `experimentalApi` or dynamic tools.

Each thread starts with:

- project root as `cwd`;
- `approvalPolicy: on-request`;
- `sandbox: workspaceWrite`;
- `serviceName: sceneops`.

The SceneOps MCP server is injected with command-line `--config` overrides for `mcp_servers.sceneops`. SceneOps never writes `~/.codex/config.toml`.

## Crash and recovery boundary

The app-server process is restarted once after an unexpected exit. The active thread ID is stored in `sceneops.project.json`; after restart, SceneOps sends `thread/resume`. A second exit fails visibly with diagnostics instead of entering a restart loop.

Media jobs store the provider job ID in `runs/<run-id>.json`. The desktop app polls active video runs every two seconds while a project is open. Polling after restart refreshes the provider status and downloads a completed result idempotently. An asset with the same run ID is reused instead of duplicated.

## Data ownership

JSON manifests and media bytes are durable truth. `.sceneops/index.sqlite` contains only derived list/search state. `RebuildIndex` drops derived rows and repopulates them from project manifests.

Core types do not name OpenAI, GPT Image, or a video model. Those values exist only in `Provenance` or the OpenAI adapter.

The guided workflow is derived from the current snapshot rather than persisted as another state machine. `brief.md`, scenes, shots, selected image assets, videos, and runs determine the next suggested action.

External video import is the stable v0.1 path. A provider may optionally accept the selected keyframe as a verified local reference. The OpenAI Videos adapter is exposed only as an experimental, capability-gated implementation because that API is deprecated and scheduled to shut down on September 24, 2026.

## References

- [Codex app-server](https://developers.openai.com/codex/app-server)
- [GPT Image 2](https://developers.openai.com/api/docs/models/gpt-image-2)
- [Create video API](https://developers.openai.com/api/reference/typescript/resources/videos/methods/create)
