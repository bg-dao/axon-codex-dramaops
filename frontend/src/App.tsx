import { FormEvent, useCallback, useEffect, useMemo, useReducer, useState } from 'react';
import {
  Aperture,
  ArrowUpRight,
  Boxes,
  Check,
  ChevronDown,
  CircleAlert,
  CircleStop,
  Clapperboard,
  Clock3,
  Code2,
  Download,
  Film,
  FolderOpen,
  Image as ImageIcon,
  LayoutGrid,
  LoaderCircle,
  MessageSquareText,
  Play,
  Plus,
  RefreshCw,
  Send,
  Settings,
  ShieldCheck,
  Sparkles,
  Video,
  WandSparkles,
  X,
} from 'lucide-react';
import {
  AgentAPI,
  Asset,
  AssetAPI,
  BrowserOpenURL,
  EventsOn,
  ProjectAPI,
  ProjectSnapshot,
  RuntimeAPI,
  SettingsAPI,
  Shot,
} from './lib/backend';
import {
  activeVideoRunIDs,
  AgentEvent,
  briefHasContent,
  initialAgentState,
  partitionShotAssets,
  reduceAgentEvent,
  sortedStoryboard,
  workflowSummary,
} from './state';

type Page = 'projects' | 'workbench' | 'versions' | 'runs' | 'settings';
type WorkbenchMode = 'storyboard' | 'keyframes' | 'video';
type Approval = { id: string; action: string; summary: string; requestedAt: string; details?: Record<string, unknown>; source: 'sceneops' | 'codex' };
type Notice = { title: string; detail?: string };
type MediaCapabilities = Awaited<ReturnType<typeof AssetAPI.Capabilities>>;

const navItems: { id: Page; label: string; icon: typeof LayoutGrid }[] = [
  { id: 'projects', label: 'Projects', icon: LayoutGrid },
  { id: 'workbench', label: 'Storyboard', icon: Clapperboard },
  { id: 'versions', label: 'Versions', icon: Boxes },
  { id: 'runs', label: 'Runs', icon: Clock3 },
  { id: 'settings', label: 'Settings', icon: Settings },
];

function App() {
  const [page, setPage] = useState<Page>('projects');
  const [snapshot, setSnapshot] = useState<ProjectSnapshot | null>(null);
  const [selectedShotId, setSelectedShotId] = useState('');
  const [agentState, dispatchAgent] = useReducer(reduceAgentEvent, initialAgentState);
  const [approvals, setApprovals] = useState<Approval[]>([]);
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [error, setError] = useState('');
  const [notice, setNotice] = useState<Notice | null>(null);

  const refresh = useCallback(async () => {
    try {
      const current = await ProjectAPI.Current();
      setSnapshot(current);
      setSelectedShotId((selected) => current.shots.some((shot) => shot.id === selected) ? selected : (current.shots[0]?.id ?? ''));
    } catch {
      // No project is open yet, or it changed while the request was in flight.
    }
  }, []);

  useEffect(() => {
    const cancelAgent = EventsOn('sceneops:agent-event', (event: AgentEvent) => {
      dispatchAgent(event);
      if (event.method === 'turn/completed') void refresh();
    });
    const cancelProject = EventsOn('sceneops:project-changed', () => void refresh());
    const cancelApproval = EventsOn('sceneops:approval-requested', (request: Approval | AgentEvent) => {
      if ('id' in request) {
        const approval = { ...request, source: 'sceneops' as const };
        setApprovals((current) => current.some((item) => item.id === approval.id) ? current : [...current, approval]);
      } else if (request.requestId) {
        const approval: Approval = {
          id: request.requestId,
          action: request.method,
          summary: request.method.includes('fileChange') ? 'Codex wants to modify project files' : 'Codex wants to execute a command',
          requestedAt: request.timestamp ?? new Date().toISOString(),
          details: request.params as Record<string, unknown>,
          source: 'codex',
        };
        setApprovals((current) => current.some((item) => item.id === approval.id) ? current : [...current, approval]);
      }
    });
    const cancelApprovalResolved = EventsOn('sceneops:approval-resolved', (decision: { requestId?: string; id?: string }) => {
      const id = decision.requestId ?? decision.id;
      if (id) setApprovals((current) => current.filter((item) => item.id !== id));
    });
    return () => {
      cancelAgent();
      cancelProject();
      cancelApproval();
      cancelApprovalResolved();
    };
  }, [refresh]);

  useEffect(() => {
    if (!snapshot) return;
    const timer = window.setInterval(async () => {
      try {
        const pending = await AssetAPI.PendingApprovals();
        const sceneOps = (pending as Omit<Approval, 'source'>[]).map((item) => ({ ...item, source: 'sceneops' as const }));
        setApprovals((current) => [...current.filter((item) => item.source === 'codex'), ...sceneOps]);
      } catch {
        // The active project can change while this poll is in flight.
      }
    }, 900);
    return () => window.clearInterval(timer);
  }, [snapshot?.root]);

  const videoRunIDs = activeVideoRunIDs(snapshot?.runs ?? []);
  const videoRunKey = videoRunIDs.join(',');
  useEffect(() => {
    if (!snapshot || videoRunIDs.length === 0) return;
    let active = true;
    let polling = false;
    const poll = async () => {
      if (!active || polling) return;
      polling = true;
      await Promise.allSettled(videoRunIDs.map((runID) => AssetAPI.GetRun(runID)));
      polling = false;
      if (active) await refresh();
    };
    void poll();
    const timer = window.setInterval(() => void poll(), 2000);
    return () => {
      active = false;
      window.clearInterval(timer);
    };
  }, [snapshot?.root, videoRunKey, refresh]);

  const resolveApproval = async (id: string, approved: boolean) => {
    try {
      const request = approvals.find((item) => item.id === id);
      if (request?.source === 'codex') await AgentAPI.ResolveApproval(id, approved ? 'accept' : 'decline');
      else await AssetAPI.ResolveApproval(id, approved);
      setApprovals((current) => current.filter((item) => item.id !== id));
      await refresh();
    } catch (cause) {
      setError(String(cause));
    }
  };

  const exportProject = async () => {
    try {
      const result = await ProjectAPI.Export();
      setNotice({ title: `Exported ${result.files} files`, detail: `${result.path} · SHA-256 ${result.sha256.slice(0, 16)}…` });
    } catch (cause) {
      setError(String(cause));
    }
  };

  const selectedShot = snapshot?.shots.find((shot) => shot.id === selectedShotId) ?? snapshot?.shots[0];
  const selectedAssets = snapshot?.assets.filter((asset) => asset.shotId === selectedShot?.id) ?? [];

  return (
    <div className="app-shell">
      <aside className="rail">
        <div className="brand-mark"><Aperture size={21} /></div>
        <nav>
          {navItems.map((item) => {
            const Icon = item.icon;
            return <button key={item.id} className={page === item.id ? 'rail-button active' : 'rail-button'} onClick={() => setPage(item.id)} title={item.label}><Icon size={19} /><span>{item.label}</span></button>;
          })}
        </nav>
        <div className="rail-spacer" />
        <div className="status-orb" title="Local-first"><span /></div>
      </aside>

      <main className={drawerOpen && page !== 'projects' ? 'main-stage drawer-open' : 'main-stage'}>
        <TopBar snapshot={snapshot} onExport={() => void exportProject()} />
        {error && <div className="error-banner"><CircleAlert size={15} />{error}<button onClick={() => setError('')}><X size={14} /></button></div>}
        {notice && <div className="success-banner"><Check size={15} /><div><strong>{notice.title}</strong>{notice.detail && <small>{notice.detail}</small>}</div><button onClick={() => setNotice(null)}><X size={14} /></button></div>}

        {page === 'projects' && <ProjectsPage snapshot={snapshot} onOpened={(value) => { setSnapshot(value); setPage('workbench'); setSelectedShotId(value.shots[0]?.id ?? ''); }} onError={setError} />}
        {page === 'workbench' && <Workbench snapshot={snapshot} selectedShot={selectedShot} selectedAssets={selectedAssets} agentWorking={Boolean(agentState.activeTurnId)} onSelectShot={setSelectedShotId} onRefresh={refresh} onExport={() => void exportProject()} onShowAgent={() => setDrawerOpen(true)} onNotice={setNotice} onError={setError} />}
        {page === 'versions' && <VersionsPage snapshot={snapshot} onSelectShot={(id) => { setSelectedShotId(id); setPage('workbench'); }} />}
        {page === 'runs' && <RunsPage snapshot={snapshot} approvals={approvals} onApproval={(id, approved) => void resolveApproval(id, approved)} onCancel={async (runID) => {
          try {
            await AssetAPI.CancelRun(runID);
            setNotice({ title: 'Media run cancelled' });
            await refresh();
          } catch (cause) {
            setError(String(cause));
          }
        }} />}
        {page === 'settings' && <SettingsPage onError={setError} />}

        {approvals[0] && <ApprovalOverlay approval={approvals[0]} queued={approvals.length - 1} onResolve={(approved) => void resolveApproval(approvals[0].id, approved)} />}
        {page !== 'projects' && <AgentDrawer state={agentState} threadId={snapshot?.project.activeThreadId} open={drawerOpen} approvals={approvals.length} onToggle={() => setDrawerOpen((value) => !value)} onError={setError} />}
      </main>
    </div>
  );
}

function TopBar({ snapshot, onExport }: { snapshot: ProjectSnapshot | null; onExport: () => void }) {
  return <header className="topbar">
    <div className="wordmark"><span>SceneOps</span><small>by Axon</small></div>
    <div className="project-crumb"><strong>{snapshot?.project.name ?? 'No project open'}</strong></div>
    <div className="topbar-actions"><div className="local-pill"><ShieldCheck size={14} /> Local-first</div><button className="button secondary" disabled={!snapshot} onClick={onExport}><Download size={15} /> Export</button></div>
  </header>;
}

function ProjectsPage({ snapshot, onOpened, onError }: { snapshot: ProjectSnapshot | null; onOpened: (value: ProjectSnapshot) => void; onError: (value: string) => void }) {
  const [createRoot, setCreateRoot] = useState('');
  const [name, setName] = useState('Untitled Film');
  const [busy, setBusy] = useState(false);
  const recent = useMemo<string[]>(() => JSON.parse(localStorage.getItem('sceneops.recent') ?? '[]'), [snapshot?.root]);
  const remember = (path: string) => localStorage.setItem('sceneops.recent', JSON.stringify([path, ...recent.filter((item) => item !== path)].slice(0, 8)));
  const open = async (path: string) => {
    if (!path) return;
    setBusy(true);
    try {
      const value = await ProjectAPI.Open(path);
      remember(value.root);
      onOpened(value);
    } catch (cause) {
      onError(String(cause));
    } finally {
      setBusy(false);
    }
  };
  const create = async () => {
    setBusy(true);
    try {
      const value = await ProjectAPI.Create(createRoot, name);
      remember(value.root);
      onOpened(value);
    } catch (cause) {
      onError(String(cause));
    } finally {
      setBusy(false);
    }
  };

  return <section className="projects-page page-scroll">
    <div className="page-title"><div><h1>Projects</h1><p>Create a project or continue from a local SceneOps folder.</p></div></div>
    <div className="project-actions-grid">
      <article className="action-card featured">
        <h2>New project</h2><p>Choose a local folder and create a portable SceneOps project.</p>
        <label>Project name<input value={name} onChange={(event) => setName(event.target.value)} /></label>
        <label>Project folder<div className="directory-field"><code>{createRoot || 'No folder selected'}</code><button className="button secondary" onClick={async () => { const path = await ProjectAPI.ChooseDirectory('Choose a folder for the new SceneOps project'); if (path) setCreateRoot(path); }}><FolderOpen size={15} /> Choose</button></div></label>
        <button className="button primary" disabled={busy || !createRoot || !name.trim()} onClick={() => void create()}>{busy ? <LoaderCircle className="spin" size={16} /> : <WandSparkles size={16} />} Create project</button>
      </article>
      <article className="action-card">
        <h2>Open project</h2><p>Choose a folder containing <code>sceneops.project.json</code>.</p>
        <button className="button secondary wide choose-open" disabled={busy} onClick={async () => { const path = await ProjectAPI.ChooseDirectory('Open a SceneOps project'); if (path) await open(path); }}><FolderOpen size={16} /> Choose folder and open</button>
      </article>
    </div>
    <div className="recent-block"><div className="section-heading"><div><span>Recent projects</span><small>Local folders</small></div></div><div className="recent-list">{recent.length === 0 && <div className="recent-empty">Recent projects will appear here.</div>}{recent.map((path) => <button key={path} onClick={() => void open(path)}><Film size={18} /><span>{path.split('/').at(-1)}</span><small>{path}</small><ArrowUpRight size={15} /></button>)}</div></div>
  </section>;
}

type WorkbenchProps = {
  snapshot: ProjectSnapshot | null;
  selectedShot?: Shot;
  selectedAssets: Asset[];
  agentWorking: boolean;
  onSelectShot: (id: string) => void;
  onRefresh: () => Promise<void>;
  onExport: () => void;
  onShowAgent: () => void;
  onNotice: (notice: Notice) => void;
  onError: (value: string) => void;
};

function Workbench({ snapshot, selectedShot, selectedAssets, agentWorking, onSelectShot, onRefresh, onExport, onShowAgent, onNotice, onError }: WorkbenchProps) {
  const [mode, setMode] = useState<WorkbenchMode>('storyboard');
  const [prompt, setPrompt] = useState(selectedShot?.prompt ?? '');
  const [briefDraft, setBriefDraft] = useState(snapshot?.brief ?? '');
  const [busyAction, setBusyAction] = useState('');
  const [capabilities, setCapabilities] = useState<MediaCapabilities | null>(null);
  const [previewVideoID, setPreviewVideoID] = useState('');

  useEffect(() => setPrompt(selectedShot?.prompt ?? ''), [selectedShot?.id, selectedShot?.prompt]);
  useEffect(() => setBriefDraft(snapshot?.brief ?? ''), [snapshot?.root, snapshot?.brief]);
  useEffect(() => setPreviewVideoID(''), [selectedShot?.id]);
  useEffect(() => {
    if (!snapshot) return;
    let active = true;
    void AssetAPI.Capabilities().then((value) => { if (active) setCapabilities(value); }).catch(() => { if (active) setCapabilities(null); });
    return () => { active = false; };
  }, [snapshot?.root]);

  if (!snapshot) return <EmptyProject />;
  const scenes = sortedStoryboard(snapshot.scenes);
  const groups = partitionShotAssets(selectedAssets);
  const selectedImage = groups.images.find((asset) => asset.id === selectedShot?.selectedAssetId) ?? groups.images.at(-1);
  const previewVideo = groups.videos.find((asset) => asset.id === previewVideoID) ?? groups.videos.at(-1);
  const activeAsset = mode === 'video' ? previewVideo : selectedImage;
  const summary = workflowSummary(snapshot);
  const briefDirty = briefDraft !== snapshot.brief;
  const activeImageRun = snapshot.runs.find((run) => run.shotId === selectedShot?.id && run.operation === 'image_generate' && (run.status === 'awaiting_approval' || run.status === 'running'));
  const activeVideoRun = snapshot.runs.find((run) => run.shotId === selectedShot?.id && run.operation === 'video_generate' && (run.status === 'awaiting_approval' || run.status === 'running'));

  const saveBrief = async () => {
    setBusyAction('brief');
    try {
      await ProjectAPI.SaveBrief(briefDraft);
      await onRefresh();
      onNotice({ title: 'Creative brief saved' });
    } catch (cause) {
      onError(String(cause));
    } finally {
      setBusyAction('');
    }
  };
  const generateStoryboard = async () => {
    setBusyAction('storyboard');
    onShowAgent();
    try {
      await AgentAPI.GenerateStoryboard();
      onNotice({ title: 'Storyboard turn started', detail: 'Review the write request when the approval card appears.' });
    } catch (cause) {
      onError(String(cause));
    } finally {
      setBusyAction('');
    }
  };
  const saveShot = async (changes: Partial<Shot>) => {
    if (!selectedShot) return;
    try {
      Object.assign(selectedShot, changes);
      await ProjectAPI.SaveShot(selectedShot);
      await onRefresh();
    } catch (cause) {
      onError(String(cause));
    }
  };
  const generateImage = async () => {
    if (!selectedShot) return;
    setBusyAction('image');
    try {
      await AssetAPI.GenerateImage(selectedShot.id, { prompt: prompt || selectedShot.title, size: '1536x1024', quality: 'medium' });
      await onRefresh();
      onNotice({ title: 'Keyframe generated and saved' });
      setMode('keyframes');
    } catch (cause) {
      onError(String(cause));
    } finally {
      setBusyAction('');
    }
  };
  const generateVideo = async () => {
    if (!selectedShot) return;
    setBusyAction('video');
    try {
      await AssetAPI.GenerateVideo(selectedShot.id, { prompt: prompt || selectedShot.title, seconds: selectedShot.durationSeconds || 4, size: selectedShot.aspectRatio === '9:16' ? '720x1280' : '1280x720', referenceAssetId: selectedShot.selectedAssetId });
      await onRefresh();
      onNotice({ title: 'Experimental video run submitted', detail: 'SceneOps will keep refreshing it while the project is open.' });
      setMode('video');
    } catch (cause) {
      onError(String(cause));
    } finally {
      setBusyAction('');
    }
  };
  const importAsset = async (kind: 'reference' | 'video') => {
    if (!selectedShot) return;
    setBusyAction(`import-${kind}`);
    try {
      if (kind === 'reference') await AssetAPI.ImportReference(selectedShot.id);
      else await AssetAPI.ImportExternalVideo(selectedShot.id);
      await onRefresh();
      if (kind === 'video') setMode('video');
    } catch (cause) {
      onError(String(cause));
    } finally {
      setBusyAction('');
    }
  };
  const nextAction = async () => {
    if (briefDirty) return saveBrief();
    switch (summary.next) {
      case 'brief':
        document.getElementById('brief-editor')?.focus();
        return;
      case 'storyboard':
        return generateStoryboard();
      case 'keyframes':
      case 'versions':
        setMode('keyframes');
        return;
      case 'video':
        setMode('video');
        return;
      case 'export':
        onExport();
    }
  };

  return <section className="workbench-shell">
    <WorkflowBar summary={summary} briefDirty={briefDirty} busy={Boolean(busyAction) || agentWorking} onNext={() => void nextAction()} />
    <div className="workbench">
      <aside className="story-tree panel">
        <div className="panel-title"><div><span>Scenes & shots</span><small>{snapshot.scenes.length} scenes · {snapshot.shots.length} shots</small></div></div>
        <div className="tree-scroll">
          {scenes.map((scene, sceneIndex) => <div className="scene-group" key={scene.id}>
            <div className="scene-row"><ChevronDown size={13} /><span className="scene-number">{String(sceneIndex + 1).padStart(2, '0')}</span><strong>{scene.title}</strong><small>{scene.shotIds.length}</small></div>
            {sortedStoryboard(snapshot.shots.filter((shot) => shot.sceneId === scene.id)).map((shot, shotIndex) => <button key={shot.id} className={selectedShot?.id === shot.id ? 'shot-row active' : 'shot-row'} onClick={() => onSelectShot(shot.id)}><span className="shot-thumb"><Clapperboard size={13} /></span><span><strong>{shot.title}</strong><small>Shot {shotIndex + 1} · {shot.durationSeconds || 4}s</small></span></button>)}
          </div>)}
          {scenes.length === 0 && <div className="tree-empty">Save a brief to generate the first storyboard.</div>}
        </div>
      </aside>

      <div className="canvas panel">
        <div className="canvas-toolbar"><div className="segmented">{(['storyboard', 'keyframes', 'video'] as const).map((value) => <button key={value} className={mode === value ? 'active' : ''} disabled={value !== 'storyboard' && snapshot.shots.length === 0} onClick={() => setMode(value)}>{value}</button>)}</div><div className="canvas-tools"><button title="Refresh" onClick={() => void onRefresh()}><RefreshCw size={15} /></button></div></div>
        {mode === 'storyboard' && <StoryboardCanvas snapshot={snapshot} briefDraft={briefDraft} briefDirty={briefDirty} agentWorking={agentWorking} busyAction={busyAction} onBriefChange={setBriefDraft} onSaveBrief={() => void saveBrief()} onGenerate={() => void generateStoryboard()} onSelectShot={(id) => { onSelectShot(id); setMode('keyframes'); }} />}
        {mode === 'keyframes' && <MediaCanvas mode="keyframes" shot={selectedShot} assets={groups.images} selectedAssetID={selectedShot?.selectedAssetId} references={groups.references} onSelect={async (assetID) => { if (!selectedShot) return; try { await AssetAPI.SelectVersion(selectedShot.id, assetID); await onRefresh(); } catch (cause) { onError(String(cause)); } }} onAdd={() => void importAsset('reference')} />}
        {mode === 'video' && <MediaCanvas mode="video" shot={selectedShot} assets={groups.videos} selectedAssetID={previewVideo?.id} references={[]} onSelect={setPreviewVideoID} onAdd={() => void importAsset('video')} />}
      </div>

      <aside className="inspector panel">
        <div className="panel-title"><div><span>Shot inspector</span><small>{selectedShot?.title ?? 'Select a shot'}</small></div></div>
        <div className="inspector-scroll">
          <section><h3>Direction</h3><label>Prompt<textarea value={prompt} disabled={!selectedShot} onChange={(event) => setPrompt(event.target.value)} placeholder="Describe the composition, lighting, motion, and mood…" /></label><div className="prompt-actions"><small>{prompt.length} chars</small><button disabled={!selectedShot} onClick={() => void saveShot({ prompt })}><Check size={13} /> Save</button></div></section>
          <section><h3>Format</h3><div className="field-grid"><label>Ratio<select value={selectedShot?.aspectRatio || '16:9'} disabled={!selectedShot} onChange={(event) => void saveShot({ aspectRatio: event.target.value })}><option>16:9</option><option>9:16</option><option>1:1</option></select></label><label>Duration<select value={selectedShot?.durationSeconds || 4} disabled={!selectedShot} onChange={(event) => void saveShot({ durationSeconds: Number(event.target.value) })}><option value={4}>4 sec</option><option value={8}>8 sec</option><option value={12}>12 sec</option></select></label></div></section>
          <section><h3>Reference assets</h3><div className="reference-summary"><span>{groups.references.length} attached</span><button disabled={!selectedShot || busyAction === 'import-reference'} onClick={() => void importAsset('reference')}><Plus size={13} /> Import reference</button></div></section>
          <section><h3>Generate</h3><div className="generate-grid">
            <button disabled={!selectedShot || Boolean(activeImageRun) || Boolean(busyAction)} onClick={() => void generateImage()}>{busyAction === 'image' ? <LoaderCircle className="spin" size={17} /> : <ImageIcon size={17} />}<span>Keyframe<small>{activeImageRun ? humanize(activeImageRun.status) : 'Paid · approve once'}</small></span></button>
            {capabilities?.videoGeneration && <button disabled={!selectedShot || Boolean(activeVideoRun) || Boolean(busyAction)} onClick={() => void generateVideo()}>{busyAction === 'video' ? <LoaderCircle className="spin" size={17} /> : <Video size={17} />}<span>Video shot<small>{activeVideoRun ? humanize(activeVideoRun.status) : 'Experimental · approve once'}</small></span></button>}
            <button disabled={!selectedShot || Boolean(busyAction)} onClick={() => void importAsset('video')}><Download size={17} /><span>Import video<small>Primary v0.1 path</small></span></button>
          </div><p className="capability-note">{capabilities?.videoGeneration ? 'Video generation is experimental. External import is recommended.' : 'Video generation is unavailable. Import a finished video.'}</p></section>
          <section><h3>Provenance</h3>{!activeAsset ? <div className="provenance-empty"><Code2 size={17} />Select or create an asset to inspect its lineage.</div> : <div className="provenance-card"><div><span>Provider</span><strong>{activeAsset.provenance.provider}</strong></div><div><span>Model</span><strong>{activeAsset.provenance.model || '—'}</strong></div>{activeAsset.parentAssetId && <div><span>Parent</span><code>{activeAsset.parentAssetId.slice(0, 12)}…</code></div>}<div><span>SHA-256</span><code>{activeAsset.sha256.slice(0, 15)}…</code></div></div>}</section>
        </div>
      </aside>
    </div>
  </section>;
}

function WorkflowBar({ summary, briefDirty, busy, onNext }: { summary: ReturnType<typeof workflowSummary>; briefDirty: boolean; busy: boolean; onNext: () => void }) {
  const nextLabel = briefDirty ? 'Save brief' : ({ brief: 'Write brief', storyboard: 'Generate storyboard', keyframes: 'Open keyframes', versions: 'Choose version', video: 'Add video', export: 'Export project' } as const)[summary.next];
  return <div className="workflow-bar"><div className="workflow-copy"><strong>Project progress</strong><span>{summary.shots} shots · {summary.images} keyframes · {summary.selected} selected · {summary.videos} video</span></div><button className="button primary" disabled={busy} onClick={onNext}>{busy && <LoaderCircle className="spin" size={14} />}{nextLabel}</button></div>;
}

function StoryboardCanvas({ snapshot, briefDraft, briefDirty, agentWorking, busyAction, onBriefChange, onSaveBrief, onGenerate, onSelectShot }: { snapshot: ProjectSnapshot; briefDraft: string; briefDirty: boolean; agentWorking: boolean; busyAction: string; onBriefChange: (value: string) => void; onSaveBrief: () => void; onGenerate: () => void; onSelectShot: (id: string) => void }) {
  if (snapshot.shots.length === 0) {
    return <div className="brief-workspace"><div className="brief-heading"><div><h2>Creative brief</h2><p>Describe the story, visual direction, key subjects, and delivery constraints.</p></div></div><textarea id="brief-editor" value={briefDraft} onChange={(event) => onBriefChange(event.target.value)} placeholder="# Creative brief&#10;&#10;A 20-second launch film for…" /><div className="brief-actions"><button className="button secondary" disabled={!briefDirty || busyAction === 'brief'} onClick={onSaveBrief}>{busyAction === 'brief' && <LoaderCircle className="spin" size={14} />} Save brief</button><button className="button primary" disabled={briefDirty || !briefHasContent(snapshot.brief) || agentWorking || Boolean(busyAction)} onClick={onGenerate}>{agentWorking ? <LoaderCircle className="spin" size={14} /> : <Sparkles size={14} />} {agentWorking ? 'Agent working' : 'Generate storyboard'}</button></div></div>;
  }
  return <div className="storyboard-overview">{sortedStoryboard(snapshot.scenes).map((scene) => <section key={scene.id} className="storyboard-scene"><div><span>{String(scene.order + 1).padStart(2, '0')}</span><h3>{scene.title}</h3><p>{scene.summary}</p></div><div className="storyboard-shot-grid">{sortedStoryboard(snapshot.shots.filter((shot) => shot.sceneId === scene.id)).map((shot) => {
    const images = snapshot.assets.filter((asset) => asset.shotId === shot.id && asset.kind === 'image');
    return <button key={shot.id} className="storyboard-card" onClick={() => onSelectShot(shot.id)}><div className="storyboard-preview">{images.length > 0 ? <AssetStack assets={images} selected={shot.selectedAssetId} compact /> : <div className="storyboard-placeholder"><Clapperboard size={19} /></div>}</div><div><strong>{shot.title}</strong><small>{images.length} keyframes · {shot.durationSeconds || 4}s</small></div></button>;
  })}</div></section>)}</div>;
}

function MediaCanvas({ mode, shot, assets, selectedAssetID, references, onSelect, onAdd }: { mode: 'keyframes' | 'video'; shot?: Shot; assets: Asset[]; selectedAssetID?: string; references: Asset[]; onSelect: (id: string) => void; onAdd: () => void }) {
  return <div className="shot-canvas"><div className="canvas-meta"><span>{shot ? `SHOT ${String((shot.order ?? 0) + 1).padStart(2, '0')} · ${mode.toUpperCase()}` : 'NO SHOT'}</span><small>{shot?.aspectRatio || '16:9'} · {shot?.durationSeconds || 4} sec</small></div><div className="visual-frame">{assets.length > 0 ? <AssetStack assets={assets} selected={selectedAssetID} /> : <div className="frame-empty"><div>{mode === 'video' ? <Video size={34} /> : <ImageIcon size={34} />}<span>{mode === 'video' ? 'No video yet' : 'No keyframe yet'}</span><small>{mode === 'video' ? 'Import a finished video.' : 'Generate a version for this shot.'}</small></div></div>}</div><div className="version-strip"><div className="version-label"><strong>{mode === 'video' ? 'Outputs' : 'Versions'}</strong><small>{assets.length} assets</small></div>{assets.map((asset, index) => <button className={selectedAssetID === asset.id ? 'version-card selected' : 'version-card'} key={asset.id} onClick={() => onSelect(asset.id)}><span>{mode === 'video' ? `O${index + 1}` : `V${index + 1}`}</span><small>{asset.kind}</small>{selectedAssetID === asset.id && <Check size={13} />}</button>)}<button className="version-add" title={mode === 'video' ? 'Import video' : 'Import reference image'} disabled={!shot} onClick={onAdd}><Plus size={18} /></button>{mode === 'keyframes' && references.length > 0 && <div className="reference-count">{references.length} refs</div>}</div></div>;
}

function AssetStack({ assets, selected, compact = false }: { assets: Asset[]; selected?: string; compact?: boolean }) {
  const asset = assets.find((item) => item.id === selected) ?? assets.at(-1)!;
  const [source, setSource] = useState('');
  useEffect(() => {
    let active = true;
    setSource('');
    void AssetAPI.DataURL(asset.id).then((value) => { if (active) setSource(value); }).catch(() => undefined);
    return () => { active = false; };
  }, [asset.id]);
  return <div className={`asset-abstract ${asset.kind}`}>{source && asset.kind === 'video' && <video className="asset-media" src={source} controls={!compact} preload="metadata" />}{source && asset.kind !== 'video' && <img className="asset-media" src={source} alt={`SceneOps asset ${asset.id}`} />}{!source && <div className="asset-glow" />}{!compact && <div className="asset-caption"><span>{asset.kind === 'video' ? <Play size={15} /> : <ImageIcon size={15} />}</span><div><strong>{asset.kind === 'video' ? 'Video shot' : 'Generated keyframe'}</strong><small>{asset.provenance.model || asset.provenance.provider}</small></div></div>}</div>;
}

function VersionsPage({ snapshot, onSelectShot }: { snapshot: ProjectSnapshot | null; onSelectShot: (id: string) => void }) {
  if (!snapshot) return <EmptyProject />;
  return <section className="content-page page-scroll"><div className="page-title"><div><h1>Asset versions</h1><p>{snapshot.assets.length} assets in this project</p></div></div><div className="asset-grid">{snapshot.assets.map((asset) => <button key={asset.id} className="asset-library-card" onClick={() => asset.shotId && onSelectShot(asset.shotId)}><div className={`asset-preview ${asset.kind}`}><AssetStack assets={[asset]} compact /></div><div><span className="asset-type">{asset.kind}</span><strong>{snapshot.shots.find((shot) => shot.id === asset.shotId)?.title ?? 'Unassigned asset'}</strong><small>{asset.provenance.model || asset.provenance.provider}</small><code>{asset.sha256.slice(0, 12)}…</code></div></button>)}{snapshot.assets.length === 0 && <div className="large-empty"><Boxes size={30} /><h2>No asset versions</h2><p>Generate a keyframe or import a reference.</p></div>}</div></section>;
}

function RunsPage({ snapshot, approvals, onApproval, onCancel }: { snapshot: ProjectSnapshot | null; approvals: Approval[]; onApproval: (id: string, approved: boolean) => void; onCancel: (runID: string) => void }) {
  if (!snapshot) return <EmptyProject />;
  return <section className="content-page page-scroll"><div className="page-title"><div><h1>Runs</h1><p>{snapshot.runs.length} recorded · {approvals.length} awaiting approval</p></div></div>{approvals.length > 0 && <div className="approval-list"><h2>Needs your approval <span>{approvals.length}</span></h2>{approvals.map((item) => <ApprovalRow key={item.id} approval={item} onResolve={(approved) => onApproval(item.id, approved)} />)}</div>}<div className="runs-table"><div className="table-head"><span>Run</span><span>Operation</span><span>Shot</span><span>Status</span><span>Updated</span><span>Action</span></div>{snapshot.runs.map((run) => <div className="table-row" key={run.id}><code>{run.id.slice(0, 8)}</code><span>{humanize(run.operation)}</span><span>{snapshot.shots.find((shot) => shot.id === run.shotId)?.title ?? '—'}</span><span><i className={`status-dot ${run.status}`} />{humanize(run.status)}</span><small>{new Date(run.updatedAt).toLocaleString()}</small><span>{run.status === 'running' && run.providerJobId ? <button className="table-action" onClick={() => onCancel(run.id)}>Cancel</button> : '—'}</span></div>)}{snapshot.runs.length === 0 && <div className="table-empty">No runs yet.</div>}</div></section>;
}

function ApprovalOverlay({ approval, queued, onResolve }: { approval: Approval; queued: number; onResolve: (approved: boolean) => void }) {
  return <aside className="approval-overlay"><div className="approval-overlay-head"><div className="approval-icon"><ShieldCheck size={19} /></div><div><span>Approval required</span><strong>{humanize(approval.action)}</strong></div>{queued > 0 && <small>+{queued} queued</small>}</div><p>{approval.summary}</p>{approval.details && <div className="approval-details">{Object.entries(approval.details).filter(([, value]) => ['string', 'number'].includes(typeof value)).slice(0, 3).map(([key, value]) => <div key={key}><span>{humanize(key)}</span><code>{String(value)}</code></div>)}</div>}<div className="approval-actions"><button className="button ghost" onClick={() => onResolve(false)}>Decline</button><button className="button primary" onClick={() => onResolve(true)}>Approve once</button></div></aside>;
}

function ApprovalRow({ approval, onResolve }: { approval: Approval; onResolve: (approved: boolean) => void }) {
  return <article><div className="approval-icon"><ShieldCheck size={19} /></div><div><strong>{humanize(approval.action)}</strong><p>{approval.summary}</p><small>{new Date(approval.requestedAt).toLocaleTimeString()}</small></div><div className="approval-actions"><button className="button ghost" onClick={() => onResolve(false)}>Decline</button><button className="button primary" onClick={() => onResolve(true)}>Approve once</button></div></article>;
}

function SettingsPage({ onError }: { onError: (value: string) => void }) {
  const [status, setStatus] = useState<{ openaiKeyConfigured: boolean; keychainService: string } | null>(null);
  const [runtimeStatus, setRuntimeStatus] = useState<{ compatible: boolean; version?: string; required?: string; source?: string; error?: string } | null>(null);
  const [key, setKey] = useState('');
  const [account, setAccount] = useState<Record<string, unknown> | null>(null);
  const [busy, setBusy] = useState(false);
  const load = async () => {
    try {
      setStatus(await SettingsAPI.Status());
      setRuntimeStatus(await RuntimeAPI.DetectCodex());
    } catch (cause) {
      onError(String(cause));
    }
  };
  useEffect(() => { void load(); }, []);
  const save = async (event: FormEvent) => {
    event.preventDefault();
    setBusy(true);
    try {
      await SettingsAPI.SetOpenAIKey(key);
      setKey('');
      await load();
    } catch (cause) {
      onError(String(cause));
    } finally {
      setBusy(false);
    }
  };
  return <section className="content-page settings-page page-scroll"><div className="page-title"><div><h1>Settings</h1><p>Runtime, media credentials, and privacy.</p></div></div><div className="settings-stack">
    <article><div className="settings-copy"><h2>Codex & ChatGPT</h2><p>Use the system Codex runtime or the verified fallback, with ChatGPT authentication handled by Codex.</p><div className="settings-state">{runtimeStatus?.compatible ? <><Check size={14} /> Codex {runtimeStatus.version} · {runtimeStatus.source}{account ? ` · ${String(account.authMode ?? 'signed in')}` : ''}</> : <><CircleAlert size={14} /> {runtimeStatus?.error ?? 'Checking runtime…'}</>}</div></div><div className="settings-buttons"><button className="button secondary" onClick={async () => { try { setRuntimeStatus(await RuntimeAPI.EnsureCodex()); } catch (cause) { onError(String(cause)); } }}>Verify runtime</button><button className="button primary" onClick={async () => { try { await AgentAPI.Start(); try { setAccount(await AgentAPI.Account()); } catch { const login = await AgentAPI.StartChatGPTLogin(); const authUrl = String(login.authUrl ?? ''); if (authUrl) BrowserOpenURL(authUrl); } } catch (cause) { onError(String(cause)); } }}>Connect ChatGPT</button></div></article>
    <article><div className="settings-copy"><h2>Media API key</h2><p>Stored in macOS Keychain as <code>{status?.keychainService ?? 'dev.bg-dao.sceneops'}</code>. The plaintext key never reaches the UI.</p><div className="settings-state">{status?.openaiKeyConfigured ? <><Check size={14} /> Key configured</> : <><CircleAlert size={14} /> Key not configured</>}</div><form onSubmit={save}><input type="password" autoComplete="off" value={key} onChange={(event) => setKey(event.target.value)} placeholder="sk-…" /><button className="button primary" disabled={busy || !key}>Save to Keychain</button></form></div>{status?.openaiKeyConfigured && <button className="button ghost danger" onClick={async () => { await SettingsAPI.DeleteOpenAIKey(); await load(); }}>Remove</button>}</article>
    <article><div className="settings-copy"><h2>Privacy</h2><p>Projects stay local. Prompts leave the device only when sent to Codex or a selected media provider.</p><div className="privacy-grid"><span><Check size={14} /> No telemetry</span><span><Check size={14} /> Local manifests</span><span><Check size={14} /> Explicit paid approvals</span></div></div></article>
  </div></section>;
}

function AgentDrawer({ state, threadId, open, approvals, onToggle, onError }: { state: typeof initialAgentState; threadId?: string; open: boolean; approvals: number; onToggle: () => void; onError: (value: string) => void }) {
  const [prompt, setPrompt] = useState('');
  const submit = async () => {
    if (!prompt.trim()) return;
    try {
      await AgentAPI.Start();
      await AgentAPI.StartTurn(prompt);
      setPrompt('');
    } catch (cause) {
      onError(String(cause));
    }
  };
  return <aside className={open ? 'agent-drawer open' : 'agent-drawer'}><button className="drawer-handle" onClick={onToggle}><div><MessageSquareText size={15} /><strong>SceneOps agent</strong>{state.activeTurnId && <span className="working"><i /> working</span>}{approvals > 0 && <span className="approval-badge">{approvals} approval</span>}</div><ChevronDown size={16} /></button>{open && <div className="drawer-content"><div className="agent-stream"><div className="event-list">{state.events.slice(-6).map((event, index) => <div key={`${event.method}-${index}`}><Code2 size={12} /><span>{event.method}</span></div>)}{state.events.length === 0 && <span>Save a brief, generate the initial storyboard, or ask Codex to refine project files.</span>}</div><div className="agent-answer">{state.streamingText || state.error || 'Agent output, tool calls, and approval events remain inspectable here.'}</div></div><div className="agent-compose"><Sparkles size={16} /><input value={prompt} onChange={(event) => setPrompt(event.target.value)} onKeyDown={(event) => { if (event.key === 'Enter') void submit(); }} placeholder="Ask SceneOps to refine a shot…" />{state.activeTurnId ? <button className="stop" onClick={async () => { if (threadId) { try { await AgentAPI.InterruptTurn(threadId, state.activeTurnId!); } catch (cause) { onError(String(cause)); } } }}><CircleStop size={18} /></button> : <button onClick={() => void submit()}><Send size={17} /></button>}</div></div>}</aside>;
}

function EmptyProject() { return <div className="large-empty full"><FolderOpen size={34} /><h2>Open a SceneOps project</h2><p>Create or open a local project from the Projects page.</p></div>; }
function humanize(value: string) { return value.replaceAll('_', ' ').replaceAll('/', ' ').replace(/\b\w/g, (char) => char.toUpperCase()); }

export default App;
