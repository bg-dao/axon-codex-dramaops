import { describe, expect, it } from 'vitest';
import { initialAgentState, reduceAgentEvent, sortedStoryboard } from './state';

describe('agent event reducer', () => {
  it('assembles streamed agent text and clears the active turn', () => {
    let state = reduceAgentEvent(initialAgentState, { method: 'turn/started', params: { turn: { id: 'turn_1' } } });
    state = reduceAgentEvent(state, { method: 'item/agentMessage/delta', params: { delta: 'Frame ' } });
    state = reduceAgentEvent(state, { method: 'item/agentMessage/delta', params: { delta: 'one' } });
    expect(state.streamingText).toBe('Frame one');
    state = reduceAgentEvent(state, { method: 'turn/completed' });
    expect(state.activeTurnId).toBeUndefined();
  });

  it('retains only the latest 250 events', () => {
    let state = initialAgentState;
    for (let index = 0; index < 260; index += 1) {
      state = reduceAgentEvent(state, { method: `event/${index}` });
    }
    expect(state.events).toHaveLength(250);
    expect(state.events[0].method).toBe('event/10');
  });
});

describe('storyboard sorting', () => {
  it('does not mutate the manifest order', () => {
    const source = [{ order: 2 }, { order: 0 }, { order: 1 }];
    expect(sortedStoryboard(source).map((item) => item.order)).toEqual([0, 1, 2]);
    expect(source[0].order).toBe(2);
  });
});
