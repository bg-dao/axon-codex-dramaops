# DramaOps project format v1

DramaOps v0.2 starts a deliberately incompatible `schemaVersion: 1`. It accepts only `dramaops.project.json`; unsupported or legacy project formats fail with a clear error and are never modified.

## Layout

```text
project/
├── dramaops.project.json
├── episodes/<episode-id>/episode.json
├── episodes/<episode-id>/edit.json
├── characters/<character-id>.json
├── locations/<location-id>.json
├── props/<prop-id>.json
├── scenes/<scene-id>.json
├── shots/<shot-id>.json
├── assets/<asset-id>/
│   ├── asset.json
│   └── source.<ext> | result.<ext>
├── runs/<run-id>.json
├── renders/
├── exports/*.dramaops.zip
└── .dramaops/
    ├── index.sqlite
    └── approvals/
```

## Story model

A project is one series. `Project` owns content language, output settings, `StyleBible`, and `SoundPalette`. Each `Episode` owns ordered scene IDs and stable `ScriptBlock` values of `action | dialogue | voice_over | sfx | music`.

Episode JSON is canonical. Fountain import generates stable scene/block IDs; Fountain export embeds DramaOps ID metadata so semantic round trips preserve identity and character bindings.

Characters, locations, and props are series resources. Each character owns exactly one `VoiceProfile` of `built_in | custom | external`. Provider custom-voice IDs are intentionally absent from this schema.

## Shot and edit model

Shots record professional production fields: shot size, camera angle and movement, lens, composition, focus, blocking, lighting, screen direction, eye line, character/prop participation, wardrobe/prop continuity, transition, references, and selected keyframe/video assets.

Each `EpisodeEdit` contains:

- one contiguous ordered `VideoClip` track;
- `dialogue | sfx | bgm` audio cues;
- subtitle cues;
- output settings.

Video clips support trim, `cut | dissolve | fade`, and `cover | contain`. Audio cues support start, duration, gain, loop, and dialogue-driven BGM ducking.

## Asset provenance

Every asset records its exact SHA-256, generating run, optional media probe data, and provider-neutral provenance. `Asset.inputs` is a role-labelled list rather than a single parent, so a result can trace to style, character, location, prop, keyframe, or previous-tail inputs.

Asset kinds are `image | video | reference | audio | subtitle | render`. Imported bytes are copied into the project and use `external-import` provenance; absolute source paths are not retained.

## Paths, writes, and index

IDs use ASCII letters, digits, `_`, and `-`, up to 128 characters. Stored media paths are normalized relative paths. Absolute paths, `..`, and every symlink escape are rejected.

Writes use a temporary sibling file, sync, and atomic replacement. Deleting `.dramaops/index.sqlite` is safe: `RebuildIndex` recreates episode, scene, shot, character, location, prop, edit, asset, and run rows from manifests.

## Run state

```text
queued → awaiting_approval → running → succeeded
   └──────────────┬──────────────→ failed
                  └──────────────→ cancelled
```

Terminal runs never return to an active state. A recovered render is a new run linked through metadata rather than a mutation of a terminal run.
