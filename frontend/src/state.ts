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
