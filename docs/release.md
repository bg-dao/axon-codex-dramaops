# v0.1 release gates

## Automated

- `go test -race ./...`
- `npm --prefix frontend test`
- `npm --prefix frontend run build`
- Wails production build for `darwin/arm64`
- no real OpenAI calls in default CI

## Apple Silicon acceptance

On a clean Apple Silicon Mac:

1. detect a compatible Codex or install the pinned verified runtime;
2. complete ChatGPT login through app-server;
3. configure the separate media API key in Keychain;
4. create at least three scenes and six shots;
5. approve the storyboard write;
6. generate two keyframe versions and one four-second 720p video shot;
7. force quit, reopen, and resume the project, Codex thread, and provider run;
8. delete `.sceneops/index.sqlite` and rebuild it;
9. export manifests, media, and provenance and verify hashes.

Credential leakage, unverified runtime installation, path escape, and unapproved paid generation are release blockers.

## Distribution

`scripts/build-macos.sh` always produces a local pre-release build. A broadly distributed DMG requires an Apple Developer signing identity and notarization profile. Without those credentials, publish source and clearly labelled pre-release artifacts only.
