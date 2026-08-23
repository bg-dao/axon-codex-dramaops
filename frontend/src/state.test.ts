import { describe, expect, it } from "vitest";
import {
  activeProviderRunIDs,
  fixedTimelineValid,
  initialAgentState,
  partitionAssets,
  reduceAgentEvent,
  resolveLocale,
  sortedByOrder,
  sortedEpisodes,
  workflowSummary,
} from "./state";

describe("agent events", () => {
  it("assembles streamed output and clears the active turn", () => {
    let state = reduceAgentEvent(initialAgentState, {
      method: "turn/started",
      params: { turn: { id: "turn-1" } },
    });
    state = reduceAgentEvent(state, {
      method: "item/agentMessage/delta",
      params: { delta: "Episode " },
    });
    state = reduceAgentEvent(state, {
      method: "item/agentMessage/delta",
      params: { delta: "one" },
    });
    expect(state.streamingText).toBe("Episode one");
    expect(
      reduceAgentEvent(state, { method: "turn/completed" }).activeTurnId,
    ).toBeUndefined();
  });

  it("retains only the latest 250 events", () => {
    let state = initialAgentState;
    for (let index = 0; index < 260; index += 1)
      state = reduceAgentEvent(state, { method: `event/${index}` });
    expect(state.events).toHaveLength(250);
    expect(state.events[0].method).toBe("event/10");
  });
});

describe("locale and ordering", () => {
  it("persists an explicit language and otherwise follows the system", () => {
    expect(resolveLocale("en", "zh-CN")).toBe("en");
    expect(resolveLocale(null, "zh-Hans")).toBe("zh-CN");
    expect(resolveLocale(null, "fr-FR")).toBe("en");
  });

  it("sorts episodes and shots without mutating manifests", () => {
    const episodes = [{ number: 3 }, { number: 1 }, { number: 2 }];
    const shots = [{ order: 2 }, { order: 0 }, { order: 1 }];
    expect(sortedEpisodes(episodes).map((item) => item.number)).toEqual([
      1, 2, 3,
    ]);
    expect(sortedByOrder(shots).map((item) => item.order)).toEqual([0, 1, 2]);
    expect(episodes[0].number).toBe(3);
  });
});

describe("derived short-drama workflow", () => {
  const base = {
    episodes: [
      {
        id: "episode-001",
        scriptBlocks: [] as {
          id: string;
          kind?: string;
          selectedVoiceAssetId?: string;
        }[],
      },
    ],
    shots: [] as {
      episodeId: string;
      selectedKeyframeAssetId?: string;
      selectedVideoAssetId?: string;
    }[],
    assets: [] as {
      id: string;
      episodeId?: string;
      shotId?: string;
      scriptBlockId?: string;
      kind: string;
    }[],
    edits: [{ episodeId: "episode-001", videoTrack: [] as unknown[] }],
  };

  it("derives each next step without persisting a workflow state machine", () => {
    expect(workflowSummary(base, "episode-001").next).toBe("script");
    const script = {
      ...base,
      episodes: [
        {
          id: "episode-001",
          scriptBlocks: [{ id: "block-1", kind: "dialogue" }],
        },
      ],
    };
    expect(workflowSummary(script, "episode-001").next).toBe("shots");
    const shots = { ...script, shots: [{ episodeId: "episode-001" }] };
    expect(workflowSummary(shots, "episode-001").next).toBe("keyframes");
    const keyframe = {
      ...shots,
      shots: [{ episodeId: "episode-001", selectedKeyframeAssetId: "image-1" }],
    };
    expect(workflowSummary(keyframe, "episode-001").next).toBe("videos");
    const video = {
      ...keyframe,
      shots: [
        {
          episodeId: "episode-001",
          selectedKeyframeAssetId: "image-1",
          selectedVideoAssetId: "video-1",
        },
      ],
    };
    expect(workflowSummary(video, "episode-001").next).toBe("voices");
    const voice = {
      ...video,
      episodes: [
        {
          id: "episode-001",
          scriptBlocks: [
            {
              id: "block-1",
              kind: "dialogue",
              selectedVoiceAssetId: "audio-1",
            },
          ],
        },
      ],
      assets: [
        {
          id: "audio-1",
          episodeId: "episode-001",
          scriptBlockId: "block-1",
          kind: "audio",
        },
      ],
    };
    expect(workflowSummary(voice, "episode-001").next).toBe("edit");
    const edit = {
      ...voice,
      edits: [{ episodeId: "episode-001", videoTrack: [{}] }],
    };
    expect(workflowSummary(edit, "episode-001").next).toBe("render");
  });

  it("classifies assets and polls only active provider video runs", () => {
    const groups = partitionAssets([
      { id: "i", kind: "image" },
      { id: "r", kind: "reference" },
      { id: "v", kind: "video" },
      { id: "a", kind: "audio" },
      { id: "o", kind: "render" },
    ]);
    expect([
      groups.images.length,
      groups.references.length,
      groups.videos.length,
      groups.audio.length,
      groups.renders.length,
    ]).toEqual([1, 1, 1, 1, 1]);
    expect(
      activeProviderRunIDs([
        {
          id: "run-1",
          operation: "video_generate",
          status: "running",
          providerJobId: "job-1",
        },
        { id: "run-2", operation: "episode_render", status: "running" },
        {
          id: "run-3",
          operation: "video_generate",
          status: "succeeded",
          providerJobId: "job-3",
        },
      ]),
    ).toEqual(["run-1"]);
  });

  it("rejects gaps and invalid trims in the fixed video track", () => {
    expect(
      fixedTimelineValid([
        { order: 0, inSeconds: 0, outSeconds: 4 },
        { order: 1, inSeconds: 1, outSeconds: 3 },
      ]),
    ).toBe(true);
    expect(
      fixedTimelineValid([
        { order: 0, inSeconds: 0, outSeconds: 4 },
        { order: 2, inSeconds: 0, outSeconds: 4 },
      ]),
    ).toBe(false);
    expect(
      fixedTimelineValid([{ order: 0, inSeconds: 4, outSeconds: 4 }]),
    ).toBe(false);
  });
});
