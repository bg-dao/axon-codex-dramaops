import { FormEvent, useEffect, useMemo, useReducer, useState } from 'react';
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
  KeyRound,
  LayoutGrid,
  LoaderCircle,
  MessageSquareText,
  MoreHorizontal,
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
import { AgentEvent, initialAgentState, reduceAgentEvent, sortedStoryboard } from './state';

type Page = 'projects' | 'workbench' | 'versions' | 'runs' | 'settings';
type Approval = { id: string; action: string; summary: string; requestedAt: string; details?: Record<string, unknown>; source: 'sceneops' | 'codex' };

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
  const [drawerOpen, setDrawerOpen] = useState(true);
  const [error, setError] = useState('');

  const refresh = async () => {
    try {
      const current = await ProjectAPI.Current();
      setSnapshot(current);
      if (!selectedShotId && current.shots[0]) setSelectedShotId(current.shots[0].id);
    } catch {
      // No project is open yet.
    }
  };

  useEffect(() => {
    const cancelAgent = EventsOn('sceneops:agent-event', (event: AgentEvent) => dispatchAgent(event));
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
  }, [selectedShotId]);

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

  const selectedShot = snapshot?.shots.find((shot) => shot.id === selectedShotId) ?? snapshot?.shots[0];
  const selectedAssets = snapshot?.assets.filter((asset) => asset.shotId === selectedShot?.id) ?? [];

  return (
    <div className="app-shell">
      <aside className="rail">
        <div className="brand-mark"><Aperture size={21} /></div>
        <nav>
          {navItems.map((item) => {
            const Icon = item.icon;
            return (
              <button key={item.id} className={page === item.id ? 'rail-button active' : 'rail-button'} onClick={() => setPage(item.id)} title={item.label}>
                <Icon size={19} />
                <span>{item.label}</span>
              </button>
            );
          })}
        </nav>
        <div className="rail-spacer" />
        <div className="status-orb" title="Local-first"><span /></div>
      </aside>

      <main className="main-stage">
        <TopBar snapshot={snapshot} onExport={async () => {
          try { await ProjectAPI.Export(); } catch (cause) { setError(String(cause)); }
        }} />
        {error && <div className="error-banner"><CircleAlert size={15} />{error}<button onClick={() => setError('')}><X size={14} /></button></div>}
        {page === 'projects' && <ProjectsPage snapshot={snapshot} onOpened={(value) => { setSnapshot(value); setPage('workbench'); setSelectedShotId(value.shots[0]?.id ?? ''); }} onError={setError} />}
        {page === 'workbench' && <Workbench snapshot={snapshot} selectedShot={selectedShot} selectedAssets={selectedAssets} onSelectShot={setSelectedShotId} onRefresh={refresh} onError={setError} />}
        {page === 'versions' && <VersionsPage snapshot={snapshot} onSelectShot={(id) => { setSelectedShotId(id); setPage('workbench'); }} />}
        {page === 'runs' && <RunsPage snapshot={snapshot} approvals={approvals} onApproval={async (id, approved) => {
          try {
            const request = approvals.find((item) => item.id === id);
            if (request?.source === 'codex') await AgentAPI.ResolveApproval(id, approved ? 'accept' : 'decline');
            else await AssetAPI.ResolveApproval(id, approved);
            setApprovals((current) => current.filter((item) => item.id !== id));
            await refresh();
          } catch (cause) { setError(String(cause)); }
        }} />}
        {page === 'settings' && <SettingsPage onError={setError} />}

        {page !== 'projects' && (
          <AgentDrawer state={agentState} threadId={snapshot?.project.activeThreadId} open={drawerOpen} approvals={approvals.length} onToggle={() => setDrawerOpen((value) => !value)} onError={setError} />
        )}
      </main>
    </div>
  );
}

function TopBar({ snapshot, onExport }: { snapshot: ProjectSnapshot | null; onExport: () => void }) {
  return (
    <header className="topbar">
      <div className="wordmark"><span>SceneOps</span><small>by Axon</small></div>
      <div className="project-crumb">
        <span className="muted">Project</span><span>/</span><strong>{snapshot?.project.name ?? 'No project open'}</strong><ChevronDown size={14} />
      </div>
      <div className="topbar-actions">
        <div className="local-pill"><ShieldCheck size={14} /> Local-first</div>
        <button className="button secondary" disabled={!snapshot} onClick={onExport}><Download size={15} /> Export</button>
      </div>
    </header>
  );
}

function ProjectsPage({ snapshot, onOpened, onError }: { snapshot: ProjectSnapshot | null; onOpened: (value: ProjectSnapshot) => void; onError: (value: string) => void }) {
  const [root, setRoot] = useState('');
  const [name, setName] = useState('Untitled Film');
  const [busy, setBusy] = useState(false);
  const recent = useMemo<string[]>(() => JSON.parse(localStorage.getItem('sceneops.recent') ?? '[]'), [snapshot?.root]);

  const remember = (path: string) => localStorage.setItem('sceneops.recent', JSON.stringify([path, ...recent.filter((item) => item !== path)].slice(0, 8)));
  const open = async (path: string) => {
    setBusy(true);
    try { const value = await ProjectAPI.Open(path); remember(value.root); onOpened(value); }
    catch (cause) { onError(String(cause)); }
    finally { setBusy(false); }
  };
  const create = async () => {
    setBusy(true);
    try { const value = await ProjectAPI.Create(root, name); remember(value.root); onOpened(value); }
    catch (cause) { onError(String(cause)); }
    finally { setBusy(false); }
  };

  return (
    <section className="projects-page page-scroll">
      <div className="hero-copy">
        <div className="eyebrow"><Sparkles size={14} /> MULTIMODAL PRODUCTION WORKBENCH</div>
        <h1>From a brief to a<br /><em>traceable scene.</em></h1>
        <p>Build storyboards, keyframes, and video shots with Codex while every asset, prompt, decision, and hash stays in your project folder.</p>
      </div>
      <div className="project-actions-grid">
        <article className="action-card featured">
          <div className="action-icon"><Plus size={22} /></div><h2>New SceneOps project</h2>
          <p>Create a portable project with manifests, local assets, and an isolated agent thread.</p>
          <label>Project name<input value={name} onChange={(event) => setName(event.target.value)} /></label>
          <label>Folder path<input value={root} onChange={(event) => setRoot(event.target.value)} placeholder="/Users/me/Projects/my-film" /></label>
          <button className="button primary" disabled={busy || !root || !name} onClick={create}>{busy ? <LoaderCircle className="spin" size={16} /> : <WandSparkles size={16} />} Create project</button>
        </article>
        <article className="action-card">
          <div className="action-icon"><FolderOpen size={22} /></div><h2>Open an existing project</h2>
          <p>Select a folder containing <code>sceneops.project.json</code>. SQLite will be rebuilt whenever needed.</p>
          <label>Project folder<input value={root} onChange={(event) => setRoot(event.target.value)} placeholder="/Users/me/Projects/my-film" /></label>
          <button className="button secondary wide" disabled={busy || !root} onClick={() => void open(root)}><FolderOpen size={16} /> Open project</button>
        </article>
      </div>
      <div className="recent-block">
        <div className="section-heading"><div><span>Recent projects</span><small>Local folders only</small></div><MoreHorizontal size={18} /></div>
        <div className="recent-list">
          {recent.length === 0 && <div className="recent-empty">Your recent SceneOps projects will appear here.</div>}
          {recent.map((path) => <button key={path} onClick={() => void open(path)}><Film size={18} /><span>{path.split('/').at(-1)}</span><small>{path}</small><ArrowUpRight size={15} /></button>)}
        </div>
      </div>
    </section>
  );
}

function Workbench({ snapshot, selectedShot, selectedAssets, onSelectShot, onRefresh, onError }: { snapshot: ProjectSnapshot | null; selectedShot?: Shot; selectedAssets: Asset[]; onSelectShot: (id: string) => void; onRefresh: () => Promise<void>; onError: (value: string) => void }) {
  const [mode, setMode] = useState<'storyboard' | 'keyframes' | 'video' | 'compare'>('storyboard');
  const [prompt, setPrompt] = useState(selectedShot?.prompt ?? '');
  useEffect(() => setPrompt(selectedShot?.prompt ?? ''), [selectedShot?.id]);
  if (!snapshot) return <EmptyProject />;
  const scenes = sortedStoryboard(snapshot.scenes);
  return (
    <section className="workbench">
      <aside className="story-tree panel">
        <div className="panel-title"><div><span>Scenes & shots</span><small>{snapshot.scenes.length} scenes · {snapshot.shots.length} shots</small></div><button><Plus size={15} /></button></div>
        <div className="tree-scroll">
          {scenes.map((scene, sceneIndex) => (
            <div className="scene-group" key={scene.id}>
              <div className="scene-row"><ChevronDown size={13} /><span className="scene-number">{String(sceneIndex + 1).padStart(2, '0')}</span><strong>{scene.title}</strong><small>{scene.shotIds.length}</small></div>
              {sortedStoryboard(snapshot.shots.filter((shot) => shot.sceneId === scene.id)).map((shot, shotIndex) => (
                <button key={shot.id} className={selectedShot?.id === shot.id ? 'shot-row active' : 'shot-row'} onClick={() => onSelectShot(shot.id)}>
                  <span className="shot-thumb"><Clapperboard size={13} /></span><span><strong>{shot.title}</strong><small>Shot {shotIndex + 1} · {shot.durationSeconds || 4}s</small></span>
                </button>
              ))}
            </div>
          ))}
          {scenes.length === 0 && <div className="tree-empty">Ask the SceneOps agent to turn your brief into a storyboard.</div>}
        </div>
      </aside>
      <div className="canvas panel">
        <div className="canvas-toolbar">
          <div className="segmented">
            {(['storyboard', 'keyframes', 'video', 'compare'] as const).map((value) => <button key={value} className={mode === value ? 'active' : ''} onClick={() => setMode(value)}>{value}</button>)}
          </div>
          <div className="canvas-tools"><button title="Refresh" onClick={() => void onRefresh()}><RefreshCw size={15} /></button><button><MoreHorizontal size={16} /></button></div>
        </div>
        <div className="shot-canvas">
          <div className="canvas-meta"><span>{selectedShot ? `SHOT ${String((selectedShot.order ?? 0) + 1).padStart(2, '0')}` : 'NO SHOT'}</span><small>{selectedShot?.aspectRatio || '16:9'} · {selectedShot?.durationSeconds || 4} sec</small></div>
          <div className="visual-frame">
            <div className="frame-grid" />
            {selectedAssets.length ? <AssetStack assets={selectedAssets} selected={selectedShot?.selectedAssetId} /> : <div className="frame-empty"><div><ImageIcon size={34} /><span>No generated keyframe yet</span><small>Choose a shot and generate a visual version.</small></div></div>}
            <div className="frame-corners"><i /><i /><i /><i /></div>
          </div>
          <div className="version-strip">
            <div className="version-label"><strong>Versions</strong><small>{selectedAssets.length} assets</small></div>
            {selectedAssets.map((asset, index) => <button className={selectedShot?.selectedAssetId === asset.id ? 'version-card selected' : 'version-card'} key={asset.id} onClick={async () => {
              if (!selectedShot) return;
              try { await AssetAPI.SelectVersion(selectedShot.id, asset.id); await onRefresh(); } catch (cause) { onError(String(cause)); }
            }}><span>V{index + 1}</span><small>{asset.kind}</small>{selectedShot?.selectedAssetId === asset.id && <Check size={13} />}</button>)}
            <button className="version-add" title="Import reference image" disabled={!selectedShot} onClick={async () => {
              if (!selectedShot) return;
              try { await AssetAPI.ImportReference(selectedShot.id); await onRefresh(); } catch (cause) { onError(String(cause)); }
            }}><Plus size={18} /></button>
          </div>
        </div>
      </div>
      <aside className="inspector panel">
        <div className="panel-title"><div><span>Shot inspector</span><small>{selectedShot?.title ?? 'Select a shot'}</small></div><button><MoreHorizontal size={16} /></button></div>
        <div className="inspector-scroll">
          <section><h3>Direction</h3><label>Prompt<textarea value={prompt} onChange={(event) => setPrompt(event.target.value)} placeholder="Describe the composition, lighting, motion, and mood…" /></label><div className="prompt-actions"><small>{prompt.length} chars</small><button onClick={async () => {
            if (!selectedShot) return;
            try { selectedShot.prompt = prompt; await ProjectAPI.SaveShot(selectedShot); await onRefresh(); } catch (cause) { onError(String(cause)); }
          }}><Check size={13} /> Save</button></div></section>
          <section><h3>Format</h3><div className="field-grid"><label>Ratio<select value={selectedShot?.aspectRatio || '16:9'} disabled={!selectedShot} onChange={async (event) => {
            if (!selectedShot) return;
            try { selectedShot.aspectRatio = event.target.value; await ProjectAPI.SaveShot(selectedShot); await onRefresh(); } catch (cause) { onError(String(cause)); }
          }}><option>16:9</option><option>9:16</option><option>1:1</option></select></label><label>Duration<select value={selectedShot?.durationSeconds || 4} disabled={!selectedShot} onChange={async (event) => {
            if (!selectedShot) return;
            try { selectedShot.durationSeconds = Number(event.target.value); await ProjectAPI.SaveShot(selectedShot); await onRefresh(); } catch (cause) { onError(String(cause)); }
          }}><option value={4}>4 sec</option><option value={8}>8 sec</option><option value={12}>12 sec</option></select></label></div></section>
          <section><h3>Generate</h3><div className="generate-grid"><button disabled={!selectedShot} onClick={async () => {
            if (!selectedShot) return;
            try { await AssetAPI.GenerateImage(selectedShot.id, { prompt: prompt || selectedShot.title, size: '1536x1024', quality: 'medium' }); await onRefresh(); } catch (cause) { onError(String(cause)); }
          }}><ImageIcon size={17} /><span>Keyframe<small>GPT Image 2</small></span></button><button disabled={!selectedShot} onClick={async () => {
            if (!selectedShot) return;
            try { await AssetAPI.GenerateVideo(selectedShot.id, { prompt: prompt || selectedShot.title, seconds: 4, size: '1280x720' }); await onRefresh(); } catch (cause) { onError(String(cause)); }
          }}><Video size={17} /><span>Video shot<small>Capability-gated</small></span></button><button disabled={!selectedShot} onClick={async () => {
            if (!selectedShot) return;
            try { await AssetAPI.ImportExternalVideo(selectedShot.id); await onRefresh(); } catch (cause) { onError(String(cause)); }
          }}><Download size={17} /><span>Import video<small>Continue without provider</small></span></button></div></section>
          <section><h3>Provenance</h3>{selectedAssets.length === 0 ? <div className="provenance-empty"><Code2 size={17} />Generated asset metadata appears here.</div> : <div className="provenance-card"><div><span>Provider</span><strong>{selectedAssets.at(-1)?.provenance.provider}</strong></div><div><span>Model</span><strong>{selectedAssets.at(-1)?.provenance.model}</strong></div><div><span>SHA-256</span><code>{selectedAssets.at(-1)?.sha256.slice(0, 15)}…</code></div></div>}</section>
        </div>
      </aside>
    </section>
  );
}

function AssetStack({ assets, selected }: { assets: Asset[]; selected?: string }) {
  const asset = assets.find((item) => item.id === selected) ?? assets.at(-1)!;
  const [source, setSource] = useState('');
  useEffect(() => {
    let active = true;
    setSource('');
    void AssetAPI.DataURL(asset.id).then((value) => { if (active) setSource(value); }).catch(() => undefined);
    return () => { active = false; };
  }, [asset.id]);
  return <div className={`asset-abstract ${asset.kind}`}>
    {source && asset.kind === 'video' && <video className="asset-media" src={source} controls preload="metadata" />}
    {source && asset.kind !== 'video' && <img className="asset-media" src={source} alt={`SceneOps asset ${asset.id}`} />}
    {!source && <div className="asset-glow" />}
    <div className="asset-caption"><span>{asset.kind === 'video' ? <Play size={15} /> : <ImageIcon size={15} />}</span><div><strong>{asset.kind === 'video' ? 'Video shot' : 'Generated keyframe'}</strong><small>{asset.provenance.model || asset.provenance.provider}</small></div></div>
  </div>;
}

function VersionsPage({ snapshot, onSelectShot }: { snapshot: ProjectSnapshot | null; onSelectShot: (id: string) => void }) {
  if (!snapshot) return <EmptyProject />;
  return <section className="content-page page-scroll"><div className="page-title"><div><span className="eyebrow">ASSET LIBRARY</span><h1>Version decisions</h1><p>Compare generated and imported assets while retaining every lineage edge.</p></div></div><div className="asset-grid">{snapshot.assets.map((asset) => <button key={asset.id} className="asset-library-card" onClick={() => asset.shotId && onSelectShot(asset.shotId)}><div className={`asset-preview ${asset.kind}`}><AssetStack assets={[asset]} /></div><div><span className="asset-type">{asset.kind}</span><strong>{snapshot.shots.find((shot) => shot.id === asset.shotId)?.title ?? 'Unassigned asset'}</strong><small>{asset.provenance.model || asset.provenance.provider}</small><code>{asset.sha256.slice(0, 12)}…</code></div></button>)}{snapshot.assets.length === 0 && <div className="large-empty"><Boxes size={30} /><h2>No asset versions yet</h2><p>Generate a keyframe or import a reference to start a version set.</p></div>}</div></section>;
}

function RunsPage({ snapshot, approvals, onApproval }: { snapshot: ProjectSnapshot | null; approvals: Approval[]; onApproval: (id: string, approved: boolean) => void }) {
  if (!snapshot) return <EmptyProject />;
  return <section className="content-page page-scroll"><div className="page-title"><div><span className="eyebrow">EXECUTION CONTROL</span><h1>Runs & approvals</h1><p>Review every paid or project-writing action before it proceeds.</p></div></div>{approvals.length > 0 && <div className="approval-list"><h2>Needs your approval <span>{approvals.length}</span></h2>{approvals.map((item) => <article key={item.id}><div className="approval-icon"><ShieldCheck size={19} /></div><div><strong>{humanize(item.action)}</strong><p>{item.summary}</p><small>{new Date(item.requestedAt).toLocaleTimeString()}</small></div><div className="approval-actions"><button className="button ghost" onClick={() => onApproval(item.id, false)}>Decline</button><button className="button primary" onClick={() => onApproval(item.id, true)}>Approve once</button></div></article>)}</div>}<div className="runs-table"><div className="table-head"><span>Run</span><span>Operation</span><span>Shot</span><span>Status</span><span>Updated</span></div>{snapshot.runs.map((run) => <div className="table-row" key={run.id}><code>{run.id.slice(0, 8)}</code><span>{humanize(run.operation)}</span><span>{snapshot.shots.find((shot) => shot.id === run.shotId)?.title ?? '—'}</span><span><i className={`status-dot ${run.status}`} />{humanize(run.status)}</span><small>{new Date(run.updatedAt).toLocaleString()}</small></div>)}{snapshot.runs.length === 0 && <div className="table-empty">No runs recorded for this project.</div>}</div></section>;
}

function SettingsPage({ onError }: { onError: (value: string) => void }) {
  const [status, setStatus] = useState<{ openaiKeyConfigured: boolean; keychainService: string } | null>(null);
  const [runtimeStatus, setRuntimeStatus] = useState<{ compatible: boolean; version?: string; required?: string; source?: string; error?: string } | null>(null);
  const [key, setKey] = useState('');
  const [account, setAccount] = useState<Record<string, unknown> | null>(null);
  const [busy, setBusy] = useState(false);
  const load = async () => {
    try { setStatus(await SettingsAPI.Status()); setRuntimeStatus(await RuntimeAPI.DetectCodex()); }
    catch (cause) { onError(String(cause)); }
  };
  useEffect(() => { void load(); }, []);
  const save = async (event: FormEvent) => {
    event.preventDefault(); setBusy(true);
    try { await SettingsAPI.SetOpenAIKey(key); setKey(''); await load(); }
    catch (cause) { onError(String(cause)); }
    finally { setBusy(false); }
  };
  return <section className="content-page settings-page page-scroll"><div className="page-title"><div><span className="eyebrow">LOCAL CONFIGURATION</span><h1>Settings</h1><p>Codex login and paid media credentials are deliberately separate.</p></div></div><div className="settings-stack"><article><div className="settings-icon"><Code2 size={20} /></div><div className="settings-copy"><h2>Codex agent runtime & ChatGPT</h2><p>System Codex is preferred. A pinned, SHA-256 verified private runtime is used only when required. ChatGPT authentication is owned by Codex app-server.</p><div className="settings-state">{runtimeStatus?.compatible ? <><Check size={14} /> Codex {runtimeStatus.version} · {runtimeStatus.source}{account ? ` · ${String(account.authMode ?? 'signed in')}` : ''}</> : <><CircleAlert size={14} /> {runtimeStatus?.error ?? 'Checking runtime…'}</>}</div></div><div className="settings-buttons"><button className="button secondary" onClick={async () => { try { setRuntimeStatus(await RuntimeAPI.EnsureCodex()); } catch (cause) { onError(String(cause)); } }}>Verify runtime</button><button className="button primary" onClick={async () => { try { await AgentAPI.Start(); try { setAccount(await AgentAPI.Account()); } catch { const login = await AgentAPI.StartChatGPTLogin(); const authUrl = String(login.authUrl ?? ''); if (authUrl) BrowserOpenURL(authUrl); } } catch (cause) { onError(String(cause)); } }}>Connect ChatGPT</button></div></article><article><div className="settings-icon"><KeyRound size={20} /></div><div className="settings-copy"><h2>OpenAI media API key</h2><p>Stored in macOS Keychain as <code>{status?.keychainService ?? 'dev.bg-dao.sceneops'}</code>. SceneOps can check whether it exists but never reads it into the UI.</p><div className="settings-state">{status?.openaiKeyConfigured ? <><Check size={14} /> Key configured</> : <><CircleAlert size={14} /> Key not configured</>}</div><form onSubmit={save}><input type="password" autoComplete="off" value={key} onChange={(event) => setKey(event.target.value)} placeholder="sk-…" /><button className="button primary" disabled={busy || !key}>Save to Keychain</button></form></div>{status?.openaiKeyConfigured && <button className="button ghost danger" onClick={async () => { await SettingsAPI.DeleteOpenAIKey(); await load(); }}>Remove</button>}</article><article><div className="settings-icon"><ShieldCheck size={20} /></div><div className="settings-copy"><h2>Privacy defaults</h2><p>No telemetry, cloud project sync, or account backend. Prompts leave the device only when sent to Codex or the selected media provider.</p><div className="privacy-grid"><span><Check size={14} /> No telemetry</span><span><Check size={14} /> Local manifests</span><span><Check size={14} /> Explicit paid approvals</span></div></div></article></div></section>;
}

function AgentDrawer({ state, threadId, open, approvals, onToggle, onError }: { state: typeof initialAgentState; threadId?: string; open: boolean; approvals: number; onToggle: () => void; onError: (value: string) => void }) {
  const [prompt, setPrompt] = useState('');
  const [started, setStarted] = useState(false);
  const submit = async () => {
    if (!prompt.trim()) return;
    try {
      if (!started) { await AgentAPI.Start(); setStarted(true); }
      await AgentAPI.StartTurn(prompt);
      setPrompt('');
    } catch (cause) { onError(String(cause)); }
  };
  return <aside className={open ? 'agent-drawer open' : 'agent-drawer'}><button className="drawer-handle" onClick={onToggle}><div><MessageSquareText size={15} /><strong>SceneOps agent</strong>{state.activeTurnId && <span className="working"><i /> working</span>}{approvals > 0 && <span className="approval-badge">{approvals} approval</span>}</div><ChevronDown size={16} /></button>{open && <div className="drawer-content"><div className="agent-stream"><div className="event-list">{state.events.slice(-6).map((event, index) => <div key={`${event.method}-${index}`}><Code2 size={12} /><span>{event.method}</span></div>)}{state.events.length === 0 && <span>Open a project, then ask Codex to structure the brief or generate scene assets.</span>}</div><div className="agent-answer">{state.streamingText || 'The agent stream will appear here. Tool calls and approvals remain inspectable.'}</div></div><div className="agent-compose"><Sparkles size={16} /><input value={prompt} onChange={(event) => setPrompt(event.target.value)} onKeyDown={(event) => { if (event.key === 'Enter') void submit(); }} placeholder="Ask SceneOps to turn the brief into 3 scenes and 6 shots…" />{state.activeTurnId ? <button className="stop" onClick={async () => { if (threadId) { try { await AgentAPI.InterruptTurn(threadId, state.activeTurnId!); } catch (cause) { onError(String(cause)); } } }}><CircleStop size={18} /></button> : <button onClick={() => void submit()}><Send size={17} /></button>}</div></div>}</aside>;
}

function EmptyProject() { return <div className="large-empty full"><FolderOpen size={34} /><h2>Open a SceneOps project</h2><p>Create or open a local project from the Projects page.</p></div>; }
function humanize(value: string) { return value.replaceAll('_', ' ').replace(/\b\w/g, (char) => char.toUpperCase()); }

export default App;
