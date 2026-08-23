# SceneOps by Axon

English | [简体中文](README.zh-CN.md)

**SceneOps — An open-source multimodal scene and asset workbench powered by the Codex agent harness.**

SceneOps is a local-first desktop workbench for independent creators. It turns a brief into a structured storyboard, keyframes, video shots, version decisions, and an exportable asset/provenance package while keeping project files understandable and portable.

> Status: early v0.1 implementation. The project format, local persistence, Codex app-server bridge, SceneOps MCP tools, OpenAI media adapter, and desktop workbench are under active development.

## Why SceneOps

- **Agent-native workflow:** Codex app-server provides login, durable threads, turns, streaming events, and human approvals.
- **Local-first projects:** JSON manifests and media files are the source of truth; SQLite is only a rebuildable index.
- **Provider-independent assets:** provider and model details live in provenance, not in the core project schema.
- **Human-controlled spend:** storyboard writes and paid media actions require an explicit approval each time.
- **Portable output:** exported projects retain prompts, parameters, parent relationships, hashes, and provider request IDs.

## v0.1 workflow

```text
Brief -> Storyboard -> Keyframes -> Video shots -> Version selection -> Export
```

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
