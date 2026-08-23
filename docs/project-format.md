# SceneOps project format v1

Every manifest contains `schemaVersion: 1`. SceneOps rejects unsupported future versions instead of guessing.

## Layout

```text
project/
├── sceneops.project.json
├── brief.md
├── AGENTS.md
├── scenes/<scene-id>.json
├── shots/<shot-id>.json
├── assets/<asset-id>/
│   ├── asset.json
│   └── result.png | result.mp4 | source.<ext>
├── runs/<run-id>.json
├── exports/*.sceneops.zip
└── .sceneops/
    ├── index.sqlite
    └── approvals/
```

## Identity and paths

Manifest IDs are ASCII letters, digits, `_`, or `-`, with a maximum length of 128. Media paths are normalized project-relative paths. Absolute paths, parent traversal, and every symlink component are rejected.

## Asset provenance

Each asset records:

- SHA-256 of the exact media bytes;
- optional parent asset and generating run;
- provider and model;
- prompt and provider parameters;
- provider request ID;
- generation timestamp.

Imported files use provider `external-import` and are copied into the project rather than referenced by an external absolute path.

`Shot.referenceAssets` contains imported reference-image IDs. `Shot.selectedAssetId` accepts generated image assets only. A generated or imported video records the selected keyframe in `parentAssetId` when one exists, preserving the keyframe-to-video lineage without adding provider-specific fields.

## Run state machine

```text
queued -> awaiting_approval -> running -> succeeded
   |              |              |-----> failed
   |              |              |-----> cancelled
   |              |--------------------> cancelled
   |-----------------------------------> failed/cancelled
```

Terminal states never transition back to a live state.

## SQLite rebuild

Deleting `.sceneops/index.sqlite` is safe. Opening the project and calling `RebuildIndex` restores scenes, shots, assets, and runs from manifests without changing source data.
