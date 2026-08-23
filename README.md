# SceneOps by Axon

English | [简体中文](README.zh-CN.md)

**SceneOps — An open-source multimodal scene and asset workbench powered by the Codex agent harness.**

SceneOps is a local-first desktop workbench for independent creators. It turns a brief into a structured storyboard, keyframes, video shots, version decisions, and an exportable asset/provenance package while keeping project files understandable and portable.

> Status: v0.1 core workflow implemented. Real OpenAI media calls remain manual opt-in while packaging and creator examples are still in progress.

## Why SceneOps

- **Agent-native workflow:** Codex app-server provides login, durable threads, turns, streaming events, and human approvals.
- **Local-first projects:** JSON manifests and media files are the source of truth; SQLite is only a rebuildable index.
- **Provider-independent assets:** provider and model details live in provenance, not in the core project schema.
- **Human-controlled spend:** storyboard writes and paid media actions require an explicit approval each time.
- **Portable output:** exported projects retain prompts, parameters, parent relationships, hashes, and provider request IDs.

## v0.1 workflow

```text
Brief -> Storyboard -> Keyframe versions -> Select -> Generate or import video -> Export
```

The desktop workflow is intentionally guided and small:

1. Choose a local folder and create or open a project.
2. Write and explicitly save `brief.md`.
3. Ask the Codex agent to create the initial 3-scene / 6-shot storyboard through the SceneOps MCP tools.
4. Approve the storyboard write once.
5. Generate keyframe versions, attach reference images, and select the preferred image.
6. Import a finished video or use a capability-gated video provider.
7. Export deterministic manifests, media, runs, and provenance with a package SHA-256.

Approvals appear in the active workspace as well as the Runs page. Paid generation and cancellation never have a permanent allow option.

The first release targets macOS on Apple Silicon. Windows-compatible boundaries are retained, but Windows packaging is not a v0.1 release gate.

## Architecture

```text
React + TypeScript UI
        |
Wails-generated bindings and events
        |
Go application services
  |              |                 |
Project store    Codex app-server  MediaProvider
(JSON + media)   (JSONL JSON-RPC)  (OpenAI first)
  |              |
SQLite index     SceneOps stdio MCP
```

SceneOps starts one Codex app-server per active project. It passes the project root as `cwd`, uses the `workspaceWrite` sandbox and `on-request` approval policy, and injects the SceneOps MCP server with command-line config overrides. SceneOps never modifies the user's global Codex config.

## Develop

Prerequisites:

- Go 1.25 or newer
- Node.js 20 or newer
- Wails 2.15 or newer
- Codex CLI with `app-server` support

```bash
npm --prefix frontend install
npm --prefix frontend test
npm --prefix frontend run build
go test -race ./...
wails dev
```

To run the MCP server directly:

```bash
go run ./cmd/sceneops-mcp --project /absolute/path/to/a/sceneops/project
```

To run the real app-server walking-skeleton gate without making project changes:

```bash
go run ./cmd/sceneops-harness-smoke \
  --project /absolute/path/to/a/sceneops/project \
  --mcp-command /absolute/path/to/SceneOps \
  --prompt "Reply with exactly: SceneOps harness ready. Do not call tools or modify files."
```

This uses the pinned verified runtime by default. The smoke command's prerelease compatibility flags are diagnostic-only and are not used by the desktop application.

OpenAI media generation uses a separate API key stored by the operating system keychain under service `dev.bg-dao.sceneops`. The key is never written to a SceneOps project, SQLite index, log, or export.

Video import is the stable v0.1 path. The current OpenAI Videos adapter is marked experimental because the [official API](https://developers.openai.com/api/reference/typescript/resources/videos/methods/create) is deprecated and scheduled to shut down on September 24, 2026. Its optional image input uses the selected keyframe, while provider-neutral asset lineage remains valid when another adapter replaces it.

## Project layout

```text
my-film/
├── sceneops.project.json
├── brief.md
├── AGENTS.md
├── scenes/
├── shots/
├── assets/<asset-id>/asset.json
├── runs/<run-id>.json
├── exports/
└── .sceneops/index.sqlite
```

See [docs/architecture.md](docs/architecture.md), [docs/project-format.md](docs/project-format.md), and [docs/security.md](docs/security.md) for the contracts behind the implementation.

## Relationship to Axon

SceneOps is developed as a self-contained open-source project under Axon. Its source, dependencies, CI, releases, and public roadmap live in this repository.

## Contributing and security

See [CONTRIBUTING.md](CONTRIBUTING.md) for the development workflow. Please report vulnerabilities according to [SECURITY.md](SECURITY.md), not in public issues.

## License

Apache License 2.0. See [LICENSE](LICENSE).
