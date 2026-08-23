import { describe, expect, it } from 'vitest';
import { activeVideoRunIDs, briefHasContent, initialAgentState, partitionShotAssets, reduceAgentEvent, sortedStoryboard, workflowSummary } from './state';

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

describe('core workflow state', () => {
  it('treats the empty brief template as incomplete', () => {
    expect(briefHasContent('# Creative brief\n\n')).toBe(false);
    expect(briefHasContent('# Creative brief\n\nA quiet launch film.')).toBe(true);
  });

  it('derives the next action without persisted workflow state', () => {
    expect(workflowSummary({ brief: '# Creative brief\n\nFilm', scenes: [], shots: [], assets: [] }).next).toBe('storyboard');
    expect(workflowSummary({
      brief: '# Creative brief\n\nFilm',
      scenes: [{}],
      shots: [{ selectedAssetId: 'image-1' }],
      assets: [{ id: 'image-1', kind: 'image' }, { id: 'video-1', kind: 'video' }],
    }).next).toBe('export');
  });

  it('separates references, versions, and videos and polls only active video jobs', () => {
    const groups = partitionShotAssets([
      { id: 'image-1', kind: 'image' },
      { id: 'reference-1', kind: 'reference' },
      { id: 'video-1', kind: 'video' },
    ]);
    expect(groups.images).toHaveLength(1);
    expect(groups.references).toHaveLength(1);
    expect(groups.videos).toHaveLength(1);
    expect(activeVideoRunIDs([
      { id: 'run-1', operation: 'video_generate', status: 'running', providerJobId: 'job-1' },
      { id: 'run-2', operation: 'image_generate', status: 'running', providerJobId: 'job-2' },
      { id: 'run-3', operation: 'video_generate', status: 'succeeded', providerJobId: 'job-3' },
    ])).toEqual(['run-1']);
  });
});
