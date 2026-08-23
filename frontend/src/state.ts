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

export const initialAgentState: AgentState = { events: [], streamingText: '' };

export function reduceAgentEvent(state: AgentState, event: AgentEvent): AgentState {
  const events = [...state.events, event].slice(-250);
  if (event.method === 'turn/started') {
    const turn = (event.params as { turn?: { id?: string } } | undefined)?.turn;
    return { ...state, events, activeTurnId: turn?.id, streamingText: '', error: undefined };
  }
  if (event.method === 'item/agentMessage/delta') {
    const delta = (event.params as { delta?: string } | undefined)?.delta ?? '';
    return { ...state, events, streamingText: state.streamingText + delta };
  }
  if (event.method === 'turn/completed') {
    return { ...state, events, activeTurnId: undefined };
  }
  if (event.method === 'error' || event.method === 'sceneops/runtime/failed') {
    const error = (event.params as { message?: string; error?: { message?: string } } | undefined);
    return { ...state, events, activeTurnId: undefined, error: error?.message ?? error?.error?.message ?? 'Agent error' };
  }
  return { ...state, events };
}

export function sortedStoryboard<T extends { order?: number }>(items: T[]): T[] {
  return [...items].sort((a, b) => (a.order ?? 0) - (b.order ?? 0));
}

export type AssetLike = { id: string; shotId?: string; kind: string };
export type RunLike = { id: string; operation: string; status: string; providerJobId?: string };

export function briefHasContent(value: string): boolean {
  const body = value
    .replace(/^\s*#\s*Creative brief\s*/i, '')
    .replace('Describe the story, audience, visual language, and delivery constraints.', '')
    .trim();
  return body.length > 0;
}

export function partitionShotAssets<T extends AssetLike>(assets: T[]) {
  return {
    images: assets.filter((asset) => asset.kind === 'image'),
    references: assets.filter((asset) => asset.kind === 'reference'),
    videos: assets.filter((asset) => asset.kind === 'video'),
  };
}

export function workflowSummary(snapshot: {
  brief: string;
  scenes: unknown[];
  shots: { selectedAssetId?: string }[];
  assets: AssetLike[];
}) {
  const images = snapshot.assets.filter((asset) => asset.kind === 'image');
  const videos = snapshot.assets.filter((asset) => asset.kind === 'video');
  const imageIDs = new Set(images.map((asset) => asset.id));
  const selected = snapshot.shots.filter((shot) => shot.selectedAssetId && imageIDs.has(shot.selectedAssetId)).length;
  const next = !briefHasContent(snapshot.brief)
    ? 'brief'
    : snapshot.shots.length === 0
      ? 'storyboard'
      : images.length === 0
        ? 'keyframes'
        : selected === 0
          ? 'versions'
          : videos.length === 0
            ? 'video'
            : 'export';
  return { scenes: snapshot.scenes.length, shots: snapshot.shots.length, images: images.length, selected, videos: videos.length, next } as const;
}

export function activeVideoRunIDs(runs: RunLike[]): string[] {
  return runs
    .filter((run) => run.operation === 'video_generate' && run.status === 'running' && Boolean(run.providerJobId))
    .map((run) => run.id);
}
