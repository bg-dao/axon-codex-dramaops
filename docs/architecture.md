# DramaOps architecture

DramaOps is a local Wails application with four narrow subsystems:

```text
Desktop process
├── Wails APIs + dramaops:* event bridge
├── Manifest store + rebuildable SQLite index
├── Codex app-server process + DramaOps stdio MCP
├── Image / Video / Speech provider adapters
└── FFmpeg probe + fixed-timeline render engine
```

## Agent boundary

Codex app-server owns ChatGPT authentication, durable threads, turns, streaming events, command/file approvals, and sandbox policy. DramaOps communicates over newline-delimited JSON-RPC using stable app-server methods; it does not opt into `experimentalApi` or dynamic tools.

Each active project uses its root as `cwd`, `workspaceWrite` sandboxing, and `onRequest` approval. The MCP server is injected with temporary command-line configuration. DramaOps never edits `~/.codex/config.toml`.

Agent actions are limited to reading the current series and proposing initial structured scripts or shot plans through the eight public MCP tools. Script writes, shot-plan writes, paid media, paid speech, and provider-job cancellation require one-time approval.

## Data and capability boundaries

JSON manifests and media files are durable truth. `.dramaops/index.sqlite` is disposable derived state. Workflow progress and continuity issues are recomputed from a `Snapshot` rather than stored as another state machine.

Image, video, speech, and optional sound providers are independent capability interfaces. A provider never has to fake a feature it does not implement. Provider/model/request details live in asset provenance; project and timeline types remain provider-neutral.

The OpenAI media key is separate from ChatGPT authentication. Custom Voice Profile bindings are device-local secrets. Neither secret enters project manifests, the index, logs, or exports.

## Consistency model

- Image requests assemble style, character, location, prop, and shot references within provider limits.
- Video requests use the selected keyframe first and may add supported continuity roles such as a previous-shot tail frame.
- Dialogue is generated only with the speaking character's locked Voice Profile.
- Environment audio, motifs, and BGM are reused by asset ID through the series Sound Palette.
- The continuity checker derives missing-reference, voice, wardrobe, prop, screen-direction, media-spec, timeline, and dialogue warnings.

Consistency means locked references, metadata, adjacent-shot checks, Voice Profiles, and media conforming. It does not claim that a generative model will reproduce a subject perfectly.

## Fixed edit and recovery

Each episode has one ordered video track and dialogue, SFX, BGM, and subtitle lanes. The renderer supports trim, `cut | dissolve | fade`, `cover | contain`, gain, loop, BGM ducking, SRT output, optional subtitle burn-in, loudness normalization, and H.264/AAC delivery.

Provider job IDs and render runs are persisted. Completed provider downloads are idempotent by run ID. An interrupted local render is marked failed on restart and relaunched as a new run that records `recoveredFrom`, avoiding ambiguous reuse of a partial output.

## References

- [Codex app-server](https://developers.openai.com/codex/app-server)
- [OpenAI image generation](https://developers.openai.com/api/docs/guides/image-generation)
- [OpenAI speech](https://developers.openai.com/api/reference/resources/audio/subresources/speech/methods/create)
- [OpenAI voice consent](https://developers.openai.com/api/reference/resources/audio/subresources/voice_consents/methods/create)
- [OpenAI Videos API](https://developers.openai.com/api/reference/typescript/resources/videos/methods/create)
