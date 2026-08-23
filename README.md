# DramaOps by Axon

English | [简体中文](README.zh-CN.md)

**DramaOps — An open-source AI short-drama production workbench powered by the Codex agent harness.**

DramaOps is a local-first desktop workbench for producing consistent AI short dramas. It connects structured scripts, character and location bibles, professional shot plans, reference-aware keyframes, video clips, locked character voices, sound, subtitles, a fixed story timeline, continuity checks, and reproducible exports.

> Status: v0.2 alpha implementation. The core project, agent, media, voice, continuity, timeline, render, and export paths are implemented. Real provider calls remain manual opt-in, and signed distribution is not yet a stable release.

## Core workflow

```text
Series setup → Episode script → Story bible → Shot plan → Keyframes
→ Video clips → Dialogue and sound → Fixed timeline → Continuity check → Export
```

DramaOps defaults to vertical short drama: `9:16`, `1080×1920`, `25fps`, and roughly 1–3 minutes per episode. Landscape projects are also supported.

- **Series-level continuity:** reuse characters, Voice Profiles, locations, props, visual style, ambience, motifs, and BGM across episodes.
- **Structured story truth:** episode JSON is canonical; Fountain can be imported and regenerated without becoming a second source of truth.
- **Professional shots:** shot size, angle, movement, lens, composition, focus, blocking, lighting, screen direction, eye line, wardrobe, props, and transitions are explicit.
- **Reference-aware generation:** keyframe requests assemble relevant style, character, location, prop, and shot references. Video requests prefer the selected keyframe.
- **Voice consistency:** each character is locked to one built-in, custom, or external Voice Profile. Custom provider voice and consent IDs stay in macOS Keychain.
- **Focused editing:** one ordered video track plus dialogue, SFX, BGM, and subtitle lanes—without turning DramaOps into a general-purpose NLE.
- **Reproducible delivery:** MP4, SRT, Fountain, manifests, continuity report, hashes, and provenance are exported together.

## Codex agent harness

The Codex agent handles structured script, story-bible, and shot-plan work. Media generation, voice generation, timeline changes, and rendering remain explicit user actions.

DramaOps talks directly to `codex app-server` over JSONL/JSON-RPC. One app-server process is associated with the active project, using the project root as `cwd`, `workspaceWrite` sandboxing, and `onRequest` approval. The app injects its stdio MCP server with temporary command-line configuration and does not modify the user's global Codex configuration.

The public MCP contract contains eight tools:

```text
dramaops_project_read       dramaops_script_apply
dramaops_shotplan_apply     dramaops_image_generate
dramaops_video_generate     dramaops_speech_generate
dramaops_job_status         dramaops_job_cancel
```

Agent script and shot-plan writes, paid media or custom-voice operations, and provider-job cancellation require a fresh one-time approval. There is no permanent approval for paid actions.

## Local-first project and render model

JSON manifests and media bytes are durable truth. `.dramaops/index.sqlite` is a disposable index and can be deleted and rebuilt without losing project data.

The local render engine uses `ffprobe` for media validation and FFmpeg for trim, conform, cut/dissolve/fade transitions, subtitle burn-in, dialogue/SFX/BGM mix, BGM ducking, loudness normalization, and H.264/AAC output. The default target is 1080×1920, 25fps, 48kHz stereo, `-16 LUFS`, and `-1 dBTP`.

OpenAI is the first image and speech provider. GPT Image uses multi-reference image editing when references exist. Video generation is capability-gated and experimental; external video import is the stable path because the [OpenAI Videos API](https://developers.openai.com/api/reference/typescript/resources/videos/methods/create) is deprecated and scheduled to shut down on September 24, 2026.

## Architecture

```text
React + TypeScript UI
        │ Wails-generated bindings and dramaops:* events
Go application services
   ├── Project store + rebuildable SQLite index
   ├── Codex app-server + DramaOps stdio MCP
   ├── Image / Video / Speech provider capabilities
   └── FFmpeg fixed-timeline render engine
```

Provider and model names live in provenance and adapters, not in the core schema. The OpenAI media key and custom voice/consent bindings are stored separately in macOS Keychain under service `dev.bg-dao.dramaops`; plaintext secrets never enter snapshots, manifests, logs, SQLite, or exports.

## Develop

Requirements:

- Go 1.25+
- Node.js 20+
- Wails 2.15+
- Codex CLI with `app-server` support
- FFmpeg with `h264_videotoolbox` for local macOS rendering

```bash
npm --prefix frontend ci
npm --prefix frontend test
npm --prefix frontend run build
go test -race ./...
wails dev
```

Run the MCP server directly:

```bash
go run ./cmd/dramaops-mcp --project /absolute/path/to/a/dramaops/project
```

Run the real app-server smoke gate without changing project files:

```bash
go run ./cmd/dramaops-harness-smoke \
  --project /absolute/path/to/a/dramaops/project \
  --mcp-command /absolute/path/to/DramaOps \
  --prompt "Reply with exactly: DramaOps harness ready. Do not call tools or modify files."
```

Default tests never make real OpenAI calls.

## Project layout

```text
my-series/
├── dramaops.project.json
├── episodes/<episode-id>/episode.json
├── episodes/<episode-id>/edit.json
├── characters/<character-id>.json
├── locations/<location-id>.json
├── props/<prop-id>.json
├── scenes/<scene-id>.json
├── shots/<shot-id>.json
├── assets/<asset-id>/asset.json
├── runs/<run-id>.json
├── renders/
├── exports/
└── .dramaops/index.sqlite
```

See [the example series](examples/README.md), [architecture](docs/architecture.md), [project format](docs/project-format.md), [security model](docs/security.md), and [release gates](docs/release.md).

## Contributing and security

See [CONTRIBUTING.md](CONTRIBUTING.md). Report vulnerabilities through [private vulnerability reporting](SECURITY.md), not public issues.

## License

Apache License 2.0. See [LICENSE](LICENSE).
