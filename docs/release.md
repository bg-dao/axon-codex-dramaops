# v0.2 release gates

## Automated

- `go test -race ./...`
- `npm --prefix frontend test`
- `npm --prefix frontend run build`
- Wails production build for `darwin/arm64`
- FFmpeg filter/command construction tests
- rejection of unsupported project formats and old technical identifiers
- no real OpenAI calls in default CI

## Apple Silicon acceptance

On a clean Apple Silicon Mac:

1. detect or install the pinned verified Codex runtime and complete ChatGPT login;
2. detect a compatible FFmpeg/ffprobe pair with `h264_videotoolbox`;
3. configure the separate OpenAI media key in Keychain;
4. create a portrait series with at least two characters, three scenes, and eight shots;
5. approve a structured script and professional shot plan;
6. create two keyframe versions and select one;
7. generate or import all video clips and bind fixed voices to both characters;
8. add ambience/BGM, subtitles, and a fixed episode timeline;
9. run continuity checks and render a 60–90 second MP4;
10. force quit, reopen, recover active jobs, delete/rebuild SQLite, and render again;
11. export the project package and verify Fountain, SRT, MP4, continuity report, provenance, and SHA-256 values.

Secret disclosure, missing consent, project-root escape, symlink traversal, unapproved paid work, unverified runtime execution, duplicate provider downloads, and non-rebuildable SQLite state are release blockers.

## Distribution

`scripts/build-macos.sh` produces an Apple Silicon pre-release. A general-user build requires an Apple Developer signing identity, notarization, a fully licensed FFmpeg distribution manifest, and a verified clean-Mac acceptance run. Without those items, publish source and clearly labelled unsigned pre-release artifacts only.
