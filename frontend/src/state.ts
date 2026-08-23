export type Locale = "en" | "zh-CN";

export function resolveLocale(
  saved: string | null,
  systemLanguage: string,
): Locale {
  if (saved === "en" || saved === "zh-CN") return saved;
  return systemLanguage.toLowerCase().startsWith("zh") ? "zh-CN" : "en";
}

export type AgentEvent = {
  method: string;
  params?: unknown;
  requestId?: string;
  timestamp?: string;
};

export type AgentState = {
  events: AgentEvent[];
  activeTurnId?: string;
  streamingText: string;
  error?: string;
};

export const initialAgentState: AgentState = { events: [], streamingText: "" };

export function reduceAgentEvent(
  state: AgentState,
  event: AgentEvent,
): AgentState {
  const events = [...state.events, event].slice(-250);
  if (event.method === "turn/started") {
    const turn = (event.params as { turn?: { id?: string } } | undefined)?.turn;
    return {
      ...state,
      events,
      activeTurnId: turn?.id,
      streamingText: "",
      error: undefined,
    };
  }
  if (event.method === "item/agentMessage/delta") {
    const delta = (event.params as { delta?: string } | undefined)?.delta ?? "";
    return { ...state, events, streamingText: state.streamingText + delta };
  }
  if (event.method === "turn/completed")
    return { ...state, events, activeTurnId: undefined };
  if (event.method === "error" || event.method === "dramaops/runtime/failed") {
    const value = event.params as
      | { message?: string; error?: { message?: string } }
      | undefined;
    return {
      ...state,
      events,
      activeTurnId: undefined,
      error: value?.message ?? value?.error?.message ?? "Agent error",
    };
  }
  return { ...state, events };
}

export function sortedByOrder<T extends { order?: number }>(items: T[]): T[] {
  return [...items].sort((a, b) => (a.order ?? 0) - (b.order ?? 0));
}

export function sortedEpisodes<T extends { number: number }>(items: T[]): T[] {
  return [...items].sort((a, b) => a.number - b.number);
}

export type AssetLike = {
  id: string;
  episodeId?: string;
  shotId?: string;
  scriptBlockId?: string;
  kind: string;
};
export type RunLike = {
  id: string;
  operation: string;
  status: string;
  providerJobId?: string;
};

export function partitionAssets<T extends AssetLike>(assets: T[]) {
  return {
    images: assets.filter((asset) => asset.kind === "image"),
    references: assets.filter((asset) => asset.kind === "reference"),
    videos: assets.filter((asset) => asset.kind === "video"),
    audio: assets.filter((asset) => asset.kind === "audio"),
    renders: assets.filter((asset) => asset.kind === "render"),
  };
}

export function workflowSummary(
  snapshot: {
    episodes: {
      id: string;
      scriptBlocks: {
        id: string;
        kind?: string;
        selectedVoiceAssetId?: string;
      }[];
    }[];
    shots: {
      episodeId: string;
      selectedKeyframeAssetId?: string;
      selectedVideoAssetId?: string;
    }[];
    assets: AssetLike[];
    edits: { episodeId: string; videoTrack: unknown[] }[];
  },
  episodeId: string,
) {
  const episode = snapshot.episodes.find((item) => item.id === episodeId);
  const shots = snapshot.shots.filter((shot) => shot.episodeId === episodeId);
  const keyframes = shots.filter((shot) =>
    Boolean(shot.selectedKeyframeAssetId),
  ).length;
  const videos = shots.filter((shot) =>
    Boolean(shot.selectedVideoAssetId),
  ).length;
  const dialogueBlocks = (episode?.scriptBlocks ?? []).filter(
    (block) => block.kind === "dialogue" || block.kind === "voice_over",
  );
  const voiceAssets = new Set(
    snapshot.assets
      .filter(
        (asset) =>
          asset.episodeId === episodeId &&
          asset.kind === "audio" &&
          asset.scriptBlockId,
      )
      .map((asset) => asset.id),
  );
  const dialogue = dialogueBlocks.filter(
    (block) =>
      Boolean(block.selectedVoiceAssetId) &&
      voiceAssets.has(block.selectedVoiceAssetId!),
  ).length;
  const edit = snapshot.edits.find((item) => item.episodeId === episodeId);
  const renders = snapshot.assets.filter(
    (asset) => asset.episodeId === episodeId && asset.kind === "render",
  ).length;
  const next =
    !episode || episode.scriptBlocks.length === 0
      ? "script"
      : shots.length === 0
        ? "shots"
        : keyframes < shots.length
          ? "keyframes"
          : videos < shots.length
            ? "videos"
            : dialogue < dialogueBlocks.length
              ? "voices"
              : !edit || edit.videoTrack.length === 0
                ? "edit"
                : renders === 0
                  ? "render"
                  : "export";
  return {
    shots: shots.length,
    keyframes,
    videos,
    dialogue,
    renders,
    next,
  } as const;
}

export function activeProviderRunIDs(runs: RunLike[]): string[] {
  return runs
    .filter(
      (run) =>
        run.operation === "video_generate" &&
        run.status === "running" &&
        Boolean(run.providerJobId),
    )
    .map((run) => run.id);
}

export function fixedTimelineValid(
  clips: { order: number; inSeconds: number; outSeconds: number }[],
): boolean {
  return [...clips]
    .sort((a, b) => a.order - b.order)
    .every(
      (clip, index) =>
        clip.order === index &&
        clip.inSeconds >= 0 &&
        clip.outSeconds > clip.inSeconds,
    );
}
