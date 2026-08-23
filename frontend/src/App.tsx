import {
  FormEvent,
  ReactNode,
  useCallback,
  useEffect,
  useMemo,
  useReducer,
  useState,
} from "react";
import {
  Aperture,
  BookOpenText,
  Check,
  ChevronDown,
  CircleAlert,
  CircleStop,
  Clock3,
  Download,
  Film,
  FolderOpen,
  Image as ImageIcon,
  Languages,
  LayoutGrid,
  LoaderCircle,
  MessageSquareText,
  Mic2,
  Music2,
  Plus,
  RefreshCw,
  Save,
  Scissors,
  Send,
  Settings,
  ShieldCheck,
  Sparkles,
  Upload,
  Video,
  WandSparkles,
  X,
} from "lucide-react";
import {
  AgentAPI,
  Asset,
  AssetAPI,
  BrowserOpenURL,
  Character,
  domain,
  Episode,
  EpisodeEdit,
  EventsOn,
  Location,
  ProjectAPI,
  ProjectSnapshot,
  Prop,
  provider,
  project,
  RenderAPI,
  Run,
  RuntimeAPI,
  SettingsAPI,
  Shot,
  StoryAPI,
} from "./lib/backend";
import {
  activeProviderRunIDs,
  AgentEvent,
  initialAgentState,
  Locale,
  partitionAssets,
  reduceAgentEvent,
  resolveLocale,
  sortedByOrder,
  sortedEpisodes,
  workflowSummary,
} from "./state";

type Page = "projects" | "story" | "shots" | "edit" | "runs" | "settings";
type Approval = Awaited<
  ReturnType<typeof AssetAPI.PendingApprovals>
>[number] & { source: "dramaops" | "codex" };
type Notice = { title: string; detail?: string };

const label = (locale: Locale, en: string, zh: string) =>
  locale === "zh-CN" ? zh : en;

function App() {
  const [locale, setLocale] = useState<Locale>(() =>
    resolveLocale(localStorage.getItem("dramaops.locale"), navigator.language),
  );
  const [page, setPage] = useState<Page>("projects");
  const [snapshot, setSnapshot] = useState<ProjectSnapshot | null>(null);
  const [episodeID, setEpisodeID] = useState("");
  const [shotID, setShotID] = useState("");
  const [agent, dispatchAgent] = useReducer(
    reduceAgentEvent,
    initialAgentState,
  );
  const [approvals, setApprovals] = useState<Approval[]>([]);
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState<Notice | null>(null);

  const changeLocale = (value: Locale) => {
    localStorage.setItem("dramaops.locale", value);
    setLocale(value);
  };
  const refresh = useCallback(async () => {
    try {
      const current = await ProjectAPI.Current();
      setSnapshot(current);
      setEpisodeID((selected) =>
        current.episodes.some((item) => item.id === selected)
          ? selected
          : current.project.activeEpisodeId || current.episodes[0]?.id || "",
      );
      setShotID((selected) =>
        current.shots.some((item) => item.id === selected)
          ? selected
          : current.shots[0]?.id || "",
      );
    } catch {
      /* no active project */
    }
  }, []);

  useEffect(() => {
    const offAgent = EventsOn("dramaops:agent-event", (event: AgentEvent) => {
      dispatchAgent(event);
      if (event.method === "turn/completed") void refresh();
    });
    const offProject = EventsOn(
      "dramaops:project-changed",
      () => void refresh(),
    );
    const offRun = EventsOn("dramaops:render-progress", () => void refresh());
    const offApproval = EventsOn(
      "dramaops:approval-requested",
      (value: Approval | AgentEvent) => {
        if ("id" in value) {
          const next = Object.assign(value, {
            source: "dramaops" as const,
          }) as Approval;
          setApprovals((items) =>
            items.some((item) => item.id === next.id)
              ? items
              : [...items, next],
          );
        } else if (value.requestId) {
          const next = {
            id: value.requestId,
            action: value.method,
            summary: label(
              locale,
              "Codex requests access to project files or a command.",
              "Codex 请求访问项目文件或执行命令。",
            ),
            requestedAt: value.timestamp ?? new Date().toISOString(),
            details: value.params as Record<string, unknown>,
            source: "codex" as const,
          } as Approval;
          setApprovals((items) =>
            items.some((item) => item.id === next.id)
              ? items
              : [...items, next],
          );
        }
      },
    );
    const offResolved = EventsOn(
      "dramaops:approval-resolved",
      (value: { requestId?: string; id?: string }) => {
        const id = value.requestId ?? value.id;
        if (id) setApprovals((items) => items.filter((item) => item.id !== id));
      },
    );
    return () => {
      offAgent();
      offProject();
      offRun();
      offApproval();
      offResolved();
    };
  }, [locale, refresh]);

  useEffect(() => {
    if (!snapshot) return;
    const timer = window.setInterval(
      () =>
        void AssetAPI.PendingApprovals()
          .then((pending) => {
            const local = pending.map(
              (item) =>
                Object.assign(item, {
                  source: "dramaops" as const,
                }) as Approval,
            );
            setApprovals((items) => [
              ...items.filter((item) => item.source === "codex"),
              ...local,
            ]);
          })
          .catch(() => undefined),
      900,
    );
    return () => window.clearInterval(timer);
  }, [snapshot?.root]);

  const providerRuns = activeProviderRunIDs(snapshot?.runs ?? []);
  useEffect(() => {
    if (providerRuns.length === 0) return;
    const poll = () =>
      void Promise.allSettled(
        providerRuns.map((id) => AssetAPI.GetRun(id)),
      ).then(refresh);
    poll();
    const timer = window.setInterval(poll, 2000);
    return () => window.clearInterval(timer);
  }, [providerRuns.join(","), refresh]);

  const resolveApproval = async (approval: Approval, approved: boolean) => {
    try {
      if (approval.source === "codex")
        await AgentAPI.ResolveApproval(
          approval.id,
          approved ? "accept" : "decline",
        );
      else await AssetAPI.ResolveApproval(approval.id, approved);
      setApprovals((items) => items.filter((item) => item.id !== approval.id));
      await refresh();
    } catch (cause) {
      setError(String(cause));
    }
  };
  const exportProject = async () => {
    try {
      const result = await ProjectAPI.Export();
      setNotice({
        title: label(
          locale,
          `Exported ${result.files} files`,
          `已导出 ${result.files} 个文件`,
        ),
        detail: `${result.path} · SHA-256 ${result.sha256.slice(0, 16)}…`,
      });
    } catch (cause) {
      setError(String(cause));
    }
  };

  const activeEpisode =
    snapshot?.episodes.find((item) => item.id === episodeID) ??
    snapshot?.episodes[0];
  const activeShots =
    snapshot?.shots.filter((item) => item.episodeId === activeEpisode?.id) ??
    [];
  const activeShot =
    activeShots.find((item) => item.id === shotID) ?? activeShots[0];
  const nav = [
    ["projects", LayoutGrid, label(locale, "Projects", "项目")],
    ["story", BookOpenText, label(locale, "Story", "剧本")],
    ["shots", Film, label(locale, "Shots", "镜头")],
    ["edit", Scissors, label(locale, "Edit", "剪辑")],
    ["runs", Clock3, label(locale, "Runs", "任务")],
    ["settings", Settings, label(locale, "Settings", "设置")],
  ] as const;

  return (
    <div className="app-shell">
      <aside className="rail">
        <div className="brand-mark">
          <Aperture size={22} />
        </div>
        <nav>
          {nav.map(([id, Icon, title]) => (
            <button
              key={id}
              className={page === id ? "rail-button active" : "rail-button"}
              onClick={() => setPage(id)}
              title={title}
            >
              <Icon size={20} />
              <span>{title}</span>
            </button>
          ))}
        </nav>
        <div className="rail-spacer" />
        <div className="status-orb">
          <span />
        </div>
      </aside>
      <main className={drawerOpen ? "main-stage drawer-open" : "main-stage"}>
        <TopBar
          locale={locale}
          snapshot={snapshot}
          onLocale={changeLocale}
          onExport={() => void exportProject()}
        />
        {error && (
          <Banner kind="error" title={error} onClose={() => setError("")} />
        )}
        {notice && (
          <Banner
            kind="success"
            title={notice.title}
            detail={notice.detail}
            onClose={() => setNotice(null)}
          />
        )}
        {page === "projects" && (
          <ProjectsPage
            locale={locale}
            snapshot={snapshot}
            onOpen={(value) => {
              setSnapshot(value);
              setEpisodeID(
                value.project.activeEpisodeId || value.episodes[0]?.id || "",
              );
              setPage("story");
            }}
            onError={setError}
          />
        )}
        {page === "story" && (
          <StoryPage
            locale={locale}
            snapshot={snapshot}
            episode={activeEpisode}
            onEpisode={setEpisodeID}
            onRefresh={refresh}
            onAgent={() => setDrawerOpen(true)}
            onNotice={setNotice}
            onError={setError}
          />
        )}
        {page === "shots" && (
          <ShotsPage
            locale={locale}
            snapshot={snapshot}
            episode={activeEpisode}
            shot={activeShot}
            onShot={setShotID}
            onRefresh={refresh}
            onNotice={setNotice}
            onError={setError}
          />
        )}
        {page === "edit" && (
          <EditPage
            locale={locale}
            snapshot={snapshot}
            episode={activeEpisode}
            onRefresh={refresh}
            onNotice={setNotice}
            onError={setError}
          />
        )}
        {page === "runs" && (
          <RunsPage
            locale={locale}
            snapshot={snapshot}
            approvals={approvals}
            onApproval={(item, approved) =>
              void resolveApproval(item, approved)
            }
            onRefresh={refresh}
            onError={setError}
          />
        )}
        {page === "settings" && (
          <SettingsPage
            locale={locale}
            snapshot={snapshot}
            onRefresh={refresh}
            onError={setError}
          />
        )}
        {approvals[0] && (
          <ApprovalCard
            locale={locale}
            approval={approvals[0]}
            queued={approvals.length - 1}
            onResolve={(approved) =>
              void resolveApproval(approvals[0], approved)
            }
          />
        )}
        {page !== "projects" && (
          <AgentDrawer
            locale={locale}
            state={agent}
            threadID={snapshot?.project.activeThreadId}
            open={drawerOpen}
            approvals={approvals.length}
            onToggle={() => setDrawerOpen((value) => !value)}
            onError={setError}
          />
        )}
      </main>
    </div>
  );
}

function TopBar({
  locale,
  snapshot,
  onLocale,
  onExport,
}: {
  locale: Locale;
  snapshot: ProjectSnapshot | null;
  onLocale: (value: Locale) => void;
  onExport: () => void;
}) {
  return (
    <header className="topbar">
      <div className="wordmark">
        <span>DramaOps</span>
        <small>by Axon</small>
      </div>
      <div className="project-crumb">
        <strong>
          {snapshot?.project.name ??
            label(locale, "No series open", "未打开项目")}
        </strong>
        {snapshot && (
          <small>
            {snapshot.project.output.orientation === "portrait"
              ? "9:16"
              : "16:9"}{" "}
            · {snapshot.project.output.fps}fps
          </small>
        )}
      </div>
      <div className="topbar-actions">
        <button
          className="icon-label"
          onClick={() => onLocale(locale === "en" ? "zh-CN" : "en")}
        >
          <Languages size={16} />
          {locale === "en" ? "中文" : "EN"}
        </button>
        <span className="local-pill">
          <ShieldCheck size={15} />
          {label(locale, "Local-first", "本地优先")}
        </span>
        <button
          className="button secondary"
          disabled={!snapshot}
          onClick={onExport}
        >
          <Download size={16} />
          {label(locale, "Export", "导出")}
        </button>
      </div>
    </header>
  );
}

function ProjectsPage({
  locale,
  snapshot,
  onOpen,
  onError,
}: {
  locale: Locale;
  snapshot: ProjectSnapshot | null;
  onOpen: (value: ProjectSnapshot) => void;
  onError: (value: string) => void;
}) {
  const [root, setRoot] = useState("");
  const [name, setName] = useState(
    label(locale, "Untitled Short Drama", "未命名短剧"),
  );
  const [orientation, setOrientation] = useState<"portrait" | "landscape">(
    "portrait",
  );
  const [busy, setBusy] = useState(false);
  const recent = useMemo<string[]>(
    () => JSON.parse(localStorage.getItem("dramaops.recent") ?? "[]"),
    [snapshot?.root],
  );
  const remember = (value: string) =>
    localStorage.setItem(
      "dramaops.recent",
      JSON.stringify(
        [value, ...recent.filter((item) => item !== value)].slice(0, 8),
      ),
    );
  const open = async (path: string) => {
    if (!path) return;
    setBusy(true);
    try {
      const value = await ProjectAPI.Open(path);
      remember(value.root);
      onOpen(value);
    } catch (cause) {
      onError(String(cause));
    } finally {
      setBusy(false);
    }
  };
  const create = async () => {
    setBusy(true);
    try {
      const options = new project.CreateOptions({
        name,
        contentLanguage: locale,
        orientation,
      });
      const value = await ProjectAPI.Create(root, options);
      remember(value.root);
      onOpen(value);
    } catch (cause) {
      onError(String(cause));
    } finally {
      setBusy(false);
    }
  };
  return (
    <Page
      title={label(locale, "Short-drama series", "AI 短剧项目")}
      subtitle={label(
        locale,
        "One local project keeps episodes, bibles, shots, voices, edits, and renders together.",
        "一个本地项目统一管理分集、设定、镜头、声音、剪辑与成片。",
      )}
    >
      <div className="project-grid">
        <article className="card accent-card">
          <h2>{label(locale, "New series", "新建短剧")}</h2>
          <label>
            {label(locale, "Series name", "项目名称")}
            <input
              value={name}
              onChange={(event) => setName(event.target.value)}
            />
          </label>
          <label>
            {label(locale, "Format", "画幅")}
            <select
              value={orientation}
              onChange={(event) =>
                setOrientation(event.target.value as typeof orientation)
              }
            >
              <option value="portrait">9:16 · 1080×1920</option>
              <option value="landscape">16:9 · 1920×1080</option>
            </select>
          </label>
          <label>
            {label(locale, "Project folder", "项目目录")}
            <div className="file-field">
              <code>{root || label(locale, "Not selected", "未选择")}</code>
              <button
                onClick={async () => {
                  const value = await ProjectAPI.ChooseDirectory(
                    label(locale, "Choose project folder", "选择项目目录"),
                  );
                  if (value) setRoot(value);
                }}
              >
                <FolderOpen size={16} />
              </button>
            </div>
          </label>
          <button
            className="button primary wide"
            disabled={busy || !root || !name.trim()}
            onClick={() => void create()}
          >
            {busy ? (
              <LoaderCircle className="spin" size={16} />
            ) : (
              <WandSparkles size={16} />
            )}
            {label(locale, "Create series", "创建项目")}
          </button>
        </article>
        <article className="card">
          <h2>{label(locale, "Open series", "打开项目")}</h2>
          <p>
            {label(
              locale,
              "Choose a folder containing dramaops.project.json.",
              "选择包含 dramaops.project.json 的目录。",
            )}
          </p>
          <button
            className="button secondary wide"
            disabled={busy}
            onClick={async () => {
              const value = await ProjectAPI.ChooseDirectory(
                label(locale, "Open DramaOps series", "打开 DramaOps 项目"),
              );
              if (value) await open(value);
            }}
          >
            <FolderOpen size={16} />
            {label(locale, "Choose folder", "选择目录")}
          </button>
          <div className="recent-list">
            {recent.map((path) => (
              <button key={path} onClick={() => void open(path)}>
                <Film size={17} />
                <span>{path.split("/").at(-1)}</span>
                <small>{path}</small>
              </button>
            ))}
          </div>
        </article>
      </div>
    </Page>
  );
}

function StoryPage({
  locale,
  snapshot,
  episode,
  onEpisode,
  onRefresh,
  onAgent,
  onNotice,
  onError,
}: {
  locale: Locale;
  snapshot: ProjectSnapshot | null;
  episode?: Episode;
  onEpisode: (id: string) => void;
  onRefresh: () => Promise<void>;
  onAgent: () => void;
  onNotice: (value: Notice) => void;
  onError: (value: string) => void;
}) {
  const [busy, setBusy] = useState("");
  const [bibleTab, setBibleTab] = useState<
    "characters" | "locations" | "props" | "style"
  >("characters");
  if (!snapshot || !episode) return <Empty locale={locale} />;
  const saveEpisode = async () => {
    setBusy("save");
    try {
      await StoryAPI.SaveEpisode(episode);
      await onRefresh();
      onNotice({ title: label(locale, "Episode saved", "分集已保存") });
    } catch (cause) {
      onError(String(cause));
    } finally {
      setBusy("");
    }
  };
  const generate = async () => {
    setBusy("agent");
    onAgent();
    try {
      await AgentAPI.GenerateScript(episode.id);
      onNotice({
        title: label(locale, "Script turn started", "剧本生成已开始"),
        detail: label(
          locale,
          "Approve the structured write once when requested.",
          "出现审批卡后仅批准本次结构化写入。",
        ),
      });
    } catch (cause) {
      onError(String(cause));
    } finally {
      setBusy("");
    }
  };
  return (
    <div className="three-pane page-fill">
      <aside className="side-panel">
        <PanelTitle
          title={label(locale, "Episodes", "分集")}
          detail={`${snapshot.episodes.length}`}
        />
        <div className="list-scroll">
          {sortedEpisodes(snapshot.episodes).map((item) => (
            <button
              key={item.id}
              className={
                item.id === episode.id ? "list-row active" : "list-row"
              }
              onClick={() => onEpisode(item.id)}
            >
              <span>{String(item.number).padStart(2, "0")}</span>
              <div>
                <strong>{item.title}</strong>
                <small>{human(item.status)}</small>
              </div>
            </button>
          ))}
        </div>
        <button
          className="panel-add"
          onClick={() =>
            void StoryAPI.CreateEpisode(label(locale, "New episode", "新分集"))
              .then((value) => {
                onEpisode(
                  value.project.activeEpisodeId ||
                    value.episodes.at(-1)?.id ||
                    "",
                );
                void onRefresh();
              })
              .catch((cause) => onError(String(cause)))
          }
        >
          <Plus size={15} />
          {label(locale, "Add episode", "添加分集")}
        </button>
      </aside>
      <section className="center-panel story-editor">
        <div className="editor-head">
          <div>
            <span>
              {label(
                locale,
                `Episode ${episode.number}`,
                `第 ${episode.number} 集`,
              )}
            </span>
            <input
              className="title-input"
              value={episode.title}
              onChange={(event) => {
                episode.title = event.target.value;
                onEpisode(episode.id);
              }}
            />
          </div>
          <div className="row-actions">
            <button
              className="button secondary"
              onClick={() =>
                void StoryAPI.ImportFountain(episode.id)
                  .then(onRefresh)
                  .catch((cause) => onError(String(cause)))
              }
            >
              <Upload size={15} />
              Fountain
            </button>
            <button
              className="button secondary"
              disabled={episode.scriptBlocks.length === 0}
              onClick={() =>
                void StoryAPI.ExportFountain(episode.id)
                  .then((value) =>
                    onNotice({
                      title: label(
                        locale,
                        "Fountain exported",
                        "Fountain 已导出",
                      ),
                      detail: value.path,
                    }),
                  )
                  .catch((cause) => onError(String(cause)))
              }
            >
              <Download size={15} />
              Fountain
            </button>
            <button
              className="button primary"
              disabled={busy === "save"}
              onClick={() => void saveEpisode()}
            >
              <Save size={15} />
              {label(locale, "Save", "保存")}
            </button>
          </div>
        </div>
        <div className="episode-meta">
          <label>
            {label(locale, "Logline", "一句话梗概")}
            <input
              value={episode.logline ?? ""}
              onChange={(event) => {
                episode.logline = event.target.value;
                onEpisode(episode.id);
              }}
              placeholder={label(
                locale,
                "The hook in one sentence…",
                "一句话说清核心钩子…",
              )}
            />
          </label>
          <label>
            {label(locale, "Synopsis", "剧情梗概")}
            <textarea
              value={episode.synopsis ?? ""}
              onChange={(event) => {
                episode.synopsis = event.target.value;
                onEpisode(episode.id);
              }}
              placeholder={label(
                locale,
                "Conflict, escalation, reversal, cliffhanger…",
                "冲突、升级、反转、结尾钩子…",
              )}
            />
          </label>
        </div>
        {episode.scriptBlocks.length === 0 ? (
          <div className="stage-empty">
            <BookOpenText size={34} />
            <h2>
              {label(locale, "Create the episode script", "生成分集剧本")}
            </h2>
            <p>
              {label(
                locale,
                "Save a logline or synopsis, then let the agent produce scenes, dialogue, and the first series bible.",
                "先保存一句话梗概或剧情梗概，再由 Agent 生成场次、台词和首版设定。",
              )}
            </p>
            <button
              className="button primary"
              disabled={busy !== "" || (!episode.logline && !episode.synopsis)}
              onClick={() => void generate()}
            >
              {busy === "agent" ? (
                <LoaderCircle className="spin" size={16} />
              ) : (
                <Sparkles size={16} />
              )}
              {label(locale, "Generate structured script", "生成结构化剧本")}
            </button>
          </div>
        ) : (
          <ScriptBlocks
            locale={locale}
            snapshot={snapshot}
            episode={episode}
            onRefresh={onRefresh}
            onError={onError}
          />
        )}
      </section>
      <aside className="right-panel">
        <div className="tabs compact">
          {(["characters", "locations", "props", "style"] as const).map(
            (tab) => (
              <button
                key={tab}
                className={bibleTab === tab ? "active" : ""}
                onClick={() => setBibleTab(tab)}
              >
                {label(
                  locale,
                  {
                    characters: "Cast",
                    locations: "Places",
                    props: "Props",
                    style: "Style",
                  }[tab],
                  {
                    characters: "人物",
                    locations: "场景",
                    props: "道具",
                    style: "风格",
                  }[tab],
                )}
              </button>
            ),
          )}
        </div>
        <BiblePanel
          locale={locale}
          tab={bibleTab}
          snapshot={snapshot}
          onRefresh={onRefresh}
          onError={onError}
        />
      </aside>
    </div>
  );
}

function ScriptBlocks({
  locale,
  snapshot,
  episode,
  onRefresh,
  onError,
}: {
  locale: Locale;
  snapshot: ProjectSnapshot;
  episode: Episode;
  onRefresh: () => Promise<void>;
  onError: (value: string) => void;
}) {
  const scenes = sortedByOrder(
    snapshot.scenes.filter((item) => item.episodeId === episode.id),
  );
  const [busy, setBusy] = useState("");
  const voice = async (blockID: string, mode: "generate" | "import") => {
    setBusy(blockID);
    try {
      if (mode === "generate")
        await AssetAPI.GenerateSpeech(
          episode.id,
          blockID,
          new provider.SpeechRequest({ responseFormat: "wav" }),
        );
      else await AssetAPI.ImportDialogue(episode.id, blockID);
      await onRefresh();
    } catch (cause) {
      onError(String(cause));
    } finally {
      setBusy("");
    }
  };
  return (
    <div className="script-scroll">
      {scenes.map((scene) => (
        <section className="script-scene" key={scene.id}>
          <header>
            <span>{String(scene.order + 1).padStart(2, "0")}</span>
            <div>
              <strong>{scene.title}</strong>
              <small>{scene.timeOfDay || scene.summary}</small>
            </div>
          </header>
          {episode.scriptBlocks
            .filter((block) => block.sceneId === scene.id)
            .sort((a, b) => a.order - b.order)
            .map((block) => (
              <article key={block.id} className={`script-block ${block.kind}`}>
                <span>{human(block.kind)}</span>
                <div>
                  {block.characterId && (
                    <strong>
                      {snapshot.characters.find(
                        (item) => item.id === block.characterId,
                      )?.name ?? block.characterId}
                    </strong>
                  )}
                  <textarea
                    value={block.text}
                    onChange={(event) => {
                      block.text = event.target.value;
                    }}
                  />
                </div>
                {(block.kind === "dialogue" || block.kind === "voice_over") && (
                  <div className="block-actions">
                    <button
                      disabled={busy === block.id}
                      onClick={() => void voice(block.id, "generate")}
                    >
                      <Mic2 size={14} />
                      {label(locale, "Voice", "配音")}
                    </button>
                    <button
                      disabled={busy === block.id}
                      onClick={() => void voice(block.id, "import")}
                    >
                      <Upload size={14} />
                    </button>
                    {block.selectedVoiceAssetId && (
                      <Check size={15} className="ok" />
                    )}
                  </div>
                )}
              </article>
            ))}
        </section>
      ))}
    </div>
  );
}

function BiblePanel({
  locale,
  tab,
  snapshot,
  onRefresh,
  onError,
}: {
  locale: Locale;
  tab: "characters" | "locations" | "props" | "style";
  snapshot: ProjectSnapshot;
  onRefresh: () => Promise<void>;
  onError: (value: string) => void;
}) {
  const [selected, setSelected] = useState("");
  if (tab === "style") {
    const bible = snapshot.project.styleBible;
    return (
      <div className="inspector-scroll form-stack">
        <label>
          {label(locale, "Visual style", "视觉风格")}
          <textarea
            value={bible.visualStyle ?? ""}
            onChange={(event) => {
              bible.visualStyle = event.target.value;
              setSelected(event.target.value);
            }}
          />
        </label>
        <label>
          {label(locale, "Lighting rules", "光线规则")}
          <textarea
            value={bible.lightingRules ?? ""}
            onChange={(event) => {
              bible.lightingRules = event.target.value;
              setSelected(event.target.value);
            }}
          />
        </label>
        <label>
          {label(locale, "Negative constraints", "负向约束")}
          <textarea
            value={bible.negativePrompt ?? ""}
            onChange={(event) => {
              bible.negativePrompt = event.target.value;
              setSelected(event.target.value);
            }}
          />
        </label>
        <button
          className="button primary"
          onClick={() =>
            void ProjectAPI.SaveSettings(snapshot.project)
              .then(onRefresh)
              .catch((cause) => onError(String(cause)))
          }
        >
          <Save size={15} />
          {label(locale, "Save style", "保存风格")}
        </button>
      </div>
    );
  }
  const items =
    tab === "characters"
      ? snapshot.characters
      : tab === "locations"
        ? snapshot.locations
        : snapshot.props;
  const item = items.find((value) => value.id === selected) ?? items[0];
  const create = async () => {
    const id = `${tab.slice(0, -1)}-${Date.now()}`;
    try {
      if (tab === "characters")
        await StoryAPI.SaveCharacter(
          new domain.Character({
            id,
            name: label(locale, "New character", "新人物"),
            voiceProfile: new domain.VoiceProfile({
              id: `voice-${id}`,
              kind: "built_in",
              name: id,
              builtInVoice: "alloy",
            }),
            referenceAssets: [],
          }),
        );
      if (tab === "locations")
        await StoryAPI.SaveLocation(
          new domain.Location({
            id,
            name: label(locale, "New location", "新场景"),
            referenceAssets: [],
          }),
        );
      if (tab === "props")
        await StoryAPI.SaveProp(
          new domain.Prop({
            id,
            name: label(locale, "New prop", "新道具"),
            referenceAssets: [],
          }),
        );
      setSelected(id);
      await onRefresh();
    } catch (cause) {
      onError(String(cause));
    }
  };
  const save = async () => {
    if (!item) return;
    try {
      if (tab === "characters") await StoryAPI.SaveCharacter(item as Character);
      if (tab === "locations") await StoryAPI.SaveLocation(item as Location);
      if (tab === "props") await StoryAPI.SaveProp(item as Prop);
      await onRefresh();
    } catch (cause) {
      onError(String(cause));
    }
  };
  return (
    <div className="bible-panel">
      <div className="compact-list">
        {items.map((value) => (
          <button
            key={value.id}
            className={item?.id === value.id ? "active" : ""}
            onClick={() => setSelected(value.id)}
          >
            {value.name}
          </button>
        ))}
        <button onClick={() => void create()}>
          <Plus size={14} />
          {label(locale, "Add", "添加")}
        </button>
      </div>
      {item && (
        <div className="inspector-scroll form-stack">
          <label>
            {label(locale, "Name", "名称")}
            <input
              value={item.name}
              onChange={(event) => {
                item.name = event.target.value;
                setSelected(item.id);
              }}
            />
          </label>
          <label>
            {label(locale, "Description", "描述")}
            <textarea
              value={item.description ?? ""}
              onChange={(event) => {
                item.description = event.target.value;
                setSelected(item.id);
              }}
            />
          </label>
          {tab === "characters" && (
            <CharacterFields
              locale={locale}
              character={item as Character}
              onError={onError}
              onRefresh={onRefresh}
            />
          )}
          {tab === "locations" && (
            <label>
              {label(locale, "Continuity notes", "连续性规则")}
              <textarea
                value={(item as Location).continuityNotes ?? ""}
                onChange={(event) => {
                  (item as Location).continuityNotes = event.target.value;
                  setSelected(item.id);
                }}
              />
            </label>
          )}
          {tab === "props" && (
            <label>
              {label(locale, "Continuity state", "道具状态")}
              <textarea
                value={(item as Prop).continuityState ?? ""}
                onChange={(event) => {
                  (item as Prop).continuityState = event.target.value;
                  setSelected(item.id);
                }}
              />
            </label>
          )}
          <button className="button primary" onClick={() => void save()}>
            <Save size={15} />
            {label(locale, "Save bible", "保存设定")}
          </button>
        </div>
      )}
    </div>
  );
}

function CharacterFields({
  locale,
  character,
  onError,
  onRefresh,
}: {
  locale: Locale;
  character: Character;
  onError: (value: string) => void;
  onRefresh: () => Promise<void>;
}) {
  const [custom, setCustom] = useState(false);
  return (
    <>
      <label>
        {label(locale, "Appearance anchor", "外观锚点")}
        <textarea
          value={character.appearance ?? ""}
          onChange={(event) => {
            character.appearance = event.target.value;
          }}
        />
      </label>
      <label>
        {label(locale, "Wardrobe", "服装")}
        <textarea
          value={character.wardrobe ?? ""}
          onChange={(event) => {
            character.wardrobe = event.target.value;
          }}
        />
      </label>
      <label>
        {label(locale, "Voice", "固定声音")}
        <select
          value={character.voiceProfile.builtInVoice || "alloy"}
          disabled={character.voiceProfile.kind === "custom"}
          onChange={(event) => {
            character.voiceProfile.kind = "built_in";
            character.voiceProfile.builtInVoice = event.target.value;
          }}
        >
          {[
            "alloy",
            "ash",
            "ballad",
            "coral",
            "echo",
            "fable",
            "nova",
            "onyx",
            "sage",
            "shimmer",
          ].map((voice) => (
            <option key={voice}>{voice}</option>
          ))}
        </select>
      </label>
      <button
        className="button secondary"
        onClick={() => setCustom((value) => !value)}
      >
        <Mic2 size={15} />
        {label(locale, "Bind authorized custom voice", "绑定已授权自定义声音")}
      </button>
      {custom && (
        <div className="consent-box">
          <p>
            {label(
              locale,
              "You must own the voice or have explicit permission. Consent and sample files stay outside the project.",
              "必须拥有该声音或取得明确授权。授权录音和样本不会写入项目。",
            )}
          </p>
          <button
            className="button primary"
            onClick={async () => {
              try {
                const consent = await AssetAPI.ChooseAudioFile(
                  label(locale, "Choose consent recording", "选择授权录音"),
                );
                if (!consent) return;
                const sample = await AssetAPI.ChooseAudioFile(
                  label(locale, "Choose voice sample", "选择声音样本"),
                );
                if (!sample) return;
                await AssetAPI.CreateCustomVoice(
                  character.id,
                  consent,
                  sample,
                  true,
                );
                await onRefresh();
                setCustom(false);
              } catch (cause) {
                onError(String(cause));
              }
            }}
          >
            {label(locale, "I confirm authorization", "我确认已获授权")}
          </button>
        </div>
      )}
    </>
  );
}

function ShotsPage({
  locale,
  snapshot,
  episode,
  shot,
  onShot,
  onRefresh,
  onNotice,
  onError,
}: {
  locale: Locale;
  snapshot: ProjectSnapshot | null;
  episode?: Episode;
  shot?: Shot;
  onShot: (id: string) => void;
  onRefresh: () => Promise<void>;
  onNotice: (value: Notice) => void;
  onError: (value: string) => void;
}) {
  const [mode, setMode] = useState<"keyframe" | "video">("keyframe");
  const [busy, setBusy] = useState("");
  const [capabilities, setCapabilities] =
    useState<provider.Capabilities | null>(null);
  useEffect(() => {
    if (!snapshot?.root) {
      setCapabilities(null);
      return;
    }
    void AssetAPI.Capabilities()
      .then(setCapabilities)
      .catch(() => setCapabilities(null));
  }, [snapshot?.root]);
  if (!snapshot || !episode) return <Empty locale={locale} />;
  const scenes = sortedByOrder(
    snapshot.scenes.filter((item) => item.episodeId === episode.id),
  );
  const assets = partitionAssets(
    snapshot.assets.filter((item) => item.shotId === shot?.id),
  );
  const activeID =
    mode === "keyframe"
      ? shot?.selectedKeyframeAssetId
      : shot?.selectedVideoAssetId;
  const versions = mode === "keyframe" ? assets.images : assets.videos;
  const generatePlan = async () => {
    setBusy("plan");
    try {
      await AgentAPI.GenerateShotPlan(episode.id);
      onNotice({
        title: label(locale, "Shot-plan turn started", "镜头表生成已开始"),
      });
    } catch (cause) {
      onError(String(cause));
    } finally {
      setBusy("");
    }
  };
  const mediaAction = async (
    kind: "image" | "video" | "reference" | "import-video",
  ) => {
    if (!shot) return;
    setBusy(kind);
    try {
      if (kind === "image")
        await AssetAPI.GenerateImage(
          shot.id,
          new provider.ImageRequest({
            prompt: shot.prompt || shot.title,
            quality: "medium",
          }),
        );
      if (kind === "video")
        await AssetAPI.GenerateVideo(
          shot.id,
          new provider.VideoRequest({
            prompt: shot.prompt || shot.title,
            seconds: Math.round(shot.durationSeconds || 4),
          }),
        );
      if (kind === "reference") await AssetAPI.ImportReference(shot.id);
      if (kind === "import-video") await AssetAPI.ImportExternalVideo(shot.id);
      await onRefresh();
      if (kind.includes("video")) setMode("video");
    } catch (cause) {
      onError(String(cause));
    } finally {
      setBusy("");
    }
  };
  return (
    <div className="three-pane page-fill">
      <aside className="side-panel">
        <PanelTitle
          title={label(locale, "Scenes & shots", "场次与镜头")}
          detail={`${snapshot.shots.filter((item) => item.episodeId === episode.id).length}`}
        />
        <div className="list-scroll">
          {scenes.map((scene) => (
            <div className="scene-group" key={scene.id}>
              <div className="scene-label">
                <ChevronDown size={13} />
                <strong>{scene.title}</strong>
              </div>
              {sortedByOrder(
                snapshot.shots.filter((item) => item.sceneId === scene.id),
              ).map((item) => (
                <button
                  key={item.id}
                  className={
                    shot?.id === item.id ? "shot-row active" : "shot-row"
                  }
                  onClick={() => onShot(item.id)}
                >
                  <span>{String(item.order + 1).padStart(2, "0")}</span>
                  <div>
                    <strong>{item.title}</strong>
                    <small>
                      {item.shotSize} · {human(item.cameraMovement)}
                    </small>
                  </div>
                </button>
              ))}
            </div>
          ))}
        </div>
        {snapshot.shots.filter((item) => item.episodeId === episode.id)
          .length === 0 && (
          <button
            className="button primary panel-cta"
            disabled={busy !== "" || episode.scriptBlocks.length === 0}
            onClick={() => void generatePlan()}
          >
            <Sparkles size={15} />
            {label(locale, "Generate 8-shot plan", "生成 8 镜专业镜头表")}
          </button>
        )}
      </aside>
      <section className="center-panel media-workspace">
        <div className="canvas-head">
          <div className="tabs">
            {(["keyframe", "video"] as const).map((value) => (
              <button
                className={mode === value ? "active" : ""}
                key={value}
                onClick={() => setMode(value)}
              >
                {label(
                  locale,
                  value === "keyframe" ? "Keyframes" : "Video clips",
                  value === "keyframe" ? "关键帧" : "视频片段",
                )}
              </button>
            ))}
          </div>
          <small>
            {shot ? `${shot.aspectRatio} · ${shot.durationSeconds}s` : ""}
          </small>
        </div>
        <AssetPreview
          asset={
            versions.find((item) => item.id === activeID) ?? versions.at(-1)
          }
          empty={label(
            locale,
            mode === "keyframe" ? "No keyframe yet" : "No video clip yet",
            mode === "keyframe" ? "暂无关键帧" : "暂无视频片段",
          )}
        />
        <div className="version-strip">
          {versions.map((asset, index) => (
            <button
              key={asset.id}
              className={asset.id === activeID ? "selected" : ""}
              onClick={() =>
                void (
                  mode === "keyframe"
                    ? AssetAPI.SelectKeyframe(shot!.id, asset.id)
                    : AssetAPI.SelectVideo(shot!.id, asset.id)
                )
                  .then(onRefresh)
                  .catch((cause) => onError(String(cause)))
              }
            >
              <span>
                {mode === "keyframe" ? `V${index + 1}` : `C${index + 1}`}
              </span>
              {asset.id === activeID && <Check size={13} />}
            </button>
          ))}
        </div>
      </section>
      <aside className="right-panel">
        <PanelTitle
          title={label(locale, "Shot inspector", "镜头参数")}
          detail={shot?.title ?? label(locale, "Select a shot", "选择镜头")}
        />
        {shot ? (
          <div className="inspector-scroll form-stack">
            <label>
              {label(locale, "Title", "镜头名称")}
              <input
                value={shot.title}
                onChange={(event) => {
                  shot.title = event.target.value;
                  onShot(shot.id);
                }}
              />
            </label>
            <div className="field-grid">
              <label>
                {label(locale, "Shot size", "景别")}
                <select
                  value={shot.shotSize}
                  onChange={(event) => {
                    shot.shotSize = event.target.value;
                    onShot(shot.id);
                  }}
                >
                  {["ECU", "CU", "MCU", "MS", "MLS", "LS", "ELS"].map(
                    (value) => (
                      <option key={value}>{value}</option>
                    ),
                  )}
                </select>
              </label>
              <label>
                {label(locale, "Lens", "焦段")}
                <input
                  type="number"
                  value={shot.lensMm || 35}
                  onChange={(event) => {
                    shot.lensMm = Number(event.target.value);
                    onShot(shot.id);
                  }}
                />
              </label>
            </div>
            <label>
              {label(locale, "Camera angle", "机位")}
              <select
                value={shot.cameraAngle}
                onChange={(event) => {
                  shot.cameraAngle = event.target.value;
                  onShot(shot.id);
                }}
              >
                {[
                  "eye_level",
                  "high",
                  "low",
                  "overhead",
                  "dutch",
                  "pov",
                  "over_the_shoulder",
                ].map((value) => (
                  <option key={value} value={value}>
                    {human(value)}
                  </option>
                ))}
              </select>
            </label>
            <label>
              {label(locale, "Camera movement", "运镜")}
              <select
                value={shot.cameraMovement}
                onChange={(event) => {
                  shot.cameraMovement = event.target.value;
                  onShot(shot.id);
                }}
              >
                {[
                  "static",
                  "pan",
                  "tilt",
                  "dolly",
                  "truck",
                  "pedestal",
                  "orbit",
                  "handheld",
                  "crane",
                  "zoom",
                ].map((value) => (
                  <option key={value} value={value}>
                    {human(value)}
                  </option>
                ))}
              </select>
            </label>
            {[
              ["composition", label(locale, "Composition", "构图")],
              ["focusSubject", label(locale, "Focus subject", "对焦对象")],
              ["blocking", label(locale, "Blocking", "人物调度")],
              ["lighting", label(locale, "Lighting", "光线")],
              [
                "screenDirection",
                label(locale, "Screen direction", "屏幕方向"),
              ],
              ["eyeLine", label(locale, "Eye line", "视线")],
              [
                "wardrobeContinuity",
                label(locale, "Wardrobe continuity", "服装连续性"),
              ],
              [
                "propContinuity",
                label(locale, "Prop continuity", "道具连续性"),
              ],
            ].map(([field, title]) => (
              <label key={field}>
                {title}
                <input
                  value={String(shot[field as keyof Shot] ?? "")}
                  onChange={(event) => {
                    (shot as unknown as Record<string, unknown>)[field] =
                      event.target.value;
                    onShot(shot.id);
                  }}
                />
              </label>
            ))}
            <label>
              {label(locale, "Transition", "转场")}
              <select
                value={shot.transition}
                onChange={(event) => {
                  shot.transition = event.target.value;
                  onShot(shot.id);
                }}
              >
                {[
                  ["cut", label(locale, "Cut", "硬切")],
                  ["dissolve", label(locale, "Dissolve", "叠化")],
                  ["fade", label(locale, "Fade", "淡化")],
                ].map(([value, title]) => (
                  <option key={value} value={value}>
                    {title}
                  </option>
                ))}
              </select>
            </label>
            <label>
              {label(locale, "Generation prompt", "生成提示词")}
              <textarea
                className="tall"
                value={shot.prompt ?? ""}
                onChange={(event) => {
                  shot.prompt = event.target.value;
                  onShot(shot.id);
                }}
              />
            </label>
            <button
              className="button primary"
              onClick={() =>
                void StoryAPI.SaveShot(shot)
                  .then(onRefresh)
                  .catch((cause) => onError(String(cause)))
              }
            >
              <Save size={15} />
              {label(locale, "Save shot", "保存镜头")}
            </button>
            <div className="action-grid">
              <button
                disabled={busy !== ""}
                onClick={() => void mediaAction("reference")}
              >
                <Upload size={16} />
                {label(locale, "Reference", "参考图")}
              </button>
              <button
                disabled={busy !== ""}
                onClick={() => void mediaAction("image")}
              >
                <ImageIcon size={16} />
                {label(locale, "Keyframe", "关键帧")}
              </button>
              <button
                disabled={busy !== ""}
                onClick={() => void mediaAction("import-video")}
              >
                <Download size={16} />
                {label(locale, "Import video", "导入视频")}
              </button>
              {capabilities?.videoGeneration && (
                <button
                  disabled={busy !== "" || !shot.selectedKeyframeAssetId}
                  onClick={() => void mediaAction("video")}
                >
                  <Video size={16} />
                  {label(locale, "Generate video", "生成视频")}
                </button>
              )}
            </div>
            <small className="muted">
              {capabilities?.videoGeneration
                ? capabilities.videoReferenceRoles?.includes("previous_tail")
                  ? label(
                      locale,
                      "Video generation can use the keyframe and previous shot tail.",
                      "视频生成可使用关键帧与上一镜尾帧。",
                    )
                  : label(
                      locale,
                      "This provider uses the selected keyframe only; import clips when stronger continuity control is required.",
                      "当前供应商仅使用选中关键帧；需要更强连续性控制时请导入视频。",
                    )
                : label(
                    locale,
                    "Video generation is unavailable; imported clips keep the production flow complete.",
                    "视频生成不可用；导入片段仍可完成完整制作流程。",
                  )}
            </small>
            {capabilities?.videoNotice && (
              <small className="muted">{capabilities.videoNotice}</small>
            )}
            <small className="muted">
              {label(
                locale,
                `${assets.references.length} shot references; series references are attached automatically.`,
                `${assets.references.length} 个镜头参考；系列级人物、场景、道具参考会自动附加。`,
              )}
            </small>
          </div>
        ) : (
          <div className="stage-empty small">
            {label(locale, "Select a shot", "请选择镜头")}
          </div>
        )}
      </aside>
    </div>
  );
}

function EditPage({
  locale,
  snapshot,
  episode,
  onRefresh,
  onNotice,
  onError,
}: {
  locale: Locale;
  snapshot: ProjectSnapshot | null;
  episode?: Episode;
  onRefresh: () => Promise<void>;
  onNotice: (value: Notice) => void;
  onError: (value: string) => void;
}) {
  const [selectedClip, setSelectedClip] = useState("");
  const [busy, setBusy] = useState("");
  const [validation, setValidation] = useState<Awaited<
    ReturnType<typeof RenderAPI.Validate>
  > | null>(null);
  if (!snapshot || !episode) return <Empty locale={locale} />;
  const edit = snapshot.edits.find((item) => item.episodeId === episode.id);
  const clip =
    edit?.videoTrack.find((item) => item.id === selectedClip) ??
    edit?.videoTrack[0];
  const renderAsset = snapshot.assets
    .filter((item) => item.episodeId === episode.id && item.kind === "render")
    .at(-1);
  const activeRender = snapshot.runs.find(
    (item) =>
      item.episodeId === episode.id &&
      item.operation === "episode_render" &&
      item.status === "running",
  );
  const saveEdit = async () => {
    if (!edit) return;
    try {
      await StoryAPI.SaveEdit(edit);
      await onRefresh();
    } catch (cause) {
      onError(String(cause));
    }
  };
  const build = async () => {
    setBusy("build");
    try {
      await RenderAPI.BuildTimeline(episode.id);
      await onRefresh();
      onNotice({
        title: label(locale, "Fixed timeline built", "固定剧情时间线已生成"),
      });
    } catch (cause) {
      onError(String(cause));
    } finally {
      setBusy("");
    }
  };
  const render = async () => {
    setBusy("render");
    try {
      await RenderAPI.Start(episode.id);
      await onRefresh();
      onNotice({
        title: label(locale, "Local render started", "本地成片渲染已开始"),
      });
    } catch (cause) {
      onError(String(cause));
    } finally {
      setBusy("");
    }
  };
  const addSound = async (role: "sfx" | "bgm") => {
    if (!edit) return;
    try {
      const asset = await AssetAPI.ImportSound(episode.id, role);
      const duration = edit.videoTrack.reduce(
        (sum, item) =>
          sum +
          item.outSeconds -
          item.inSeconds -
          (item.transitionSeconds ?? 0),
        0,
      );
      edit.audioCues.push(
        new domain.AudioCue({
          id: `${role}-${Date.now()}`,
          lane: role,
          assetId: asset.id,
          startSeconds: 0,
          durationSeconds: Math.max(duration, 1),
          loop: role === "bgm",
          gainDb: role === "bgm" ? -12 : 0,
        }),
      );
      await StoryAPI.SaveEdit(edit);
      await onRefresh();
    } catch (cause) {
      onError(String(cause));
    }
  };
  return (
    <div className="edit-layout page-fill">
      <section className="edit-main">
        <div className="editor-head">
          <div>
            <span>{label(locale, "Episode edit", "分集剪辑")}</span>
            <strong>{episode.title}</strong>
          </div>
          <div className="row-actions">
            <button className="button secondary" onClick={() => void build()}>
              <RefreshCw size={15} />
              {label(locale, "Build timeline", "生成时间线")}
            </button>
            <button
              className="button secondary"
              disabled={!edit?.videoTrack.length}
              onClick={() =>
                void RenderAPI.Validate(episode.id)
                  .then(setValidation)
                  .catch((cause) => onError(String(cause)))
              }
            >
              <ShieldCheck size={15} />
              {label(locale, "Check", "检查")}
            </button>
            <button
              className="button primary"
              disabled={
                !edit?.videoTrack.length || Boolean(activeRender) || busy !== ""
              }
              onClick={() => void render()}
            >
              {activeRender ? (
                <LoaderCircle className="spin" size={15} />
              ) : (
                <Film size={15} />
              )}
              {activeRender
                ? `${activeRender.progress}%`
                : label(locale, "Render MP4", "渲染成片")}
            </button>
          </div>
        </div>
        <AssetPreview
          asset={renderAsset}
          empty={label(
            locale,
            "Build the fixed timeline, then render the episode.",
            "先生成固定剧情时间线，再渲染成片。",
          )}
        />
        {edit && (
          <Timeline
            locale={locale}
            edit={edit}
            selected={clip?.id}
            onSelect={setSelectedClip}
          />
        )}
      </section>
      <aside className="edit-inspector">
        <PanelTitle
          title={label(locale, "Edit inspector", "剪辑参数")}
          detail={clip?.id ?? ""}
        />
        <div className="inspector-scroll form-stack">
          <div className="action-grid">
            <button onClick={() => void addSound("sfx")}>
              <Music2 size={15} />
              {label(locale, "Add SFX", "添加音效")}
            </button>
            <button onClick={() => void addSound("bgm")}>
              <Music2 size={15} />
              {label(locale, "Add BGM", "添加配乐")}
            </button>
          </div>
          {clip && edit && (
            <>
              <div className="field-grid">
                <label>
                  {label(locale, "In", "入点")}
                  <input
                    type="number"
                    step="0.1"
                    value={clip.inSeconds}
                    onChange={(event) => {
                      clip.inSeconds = Number(event.target.value);
                      setSelectedClip(clip.id);
                    }}
                  />
                </label>
                <label>
                  {label(locale, "Out", "出点")}
                  <input
                    type="number"
                    step="0.1"
                    value={clip.outSeconds}
                    onChange={(event) => {
                      clip.outSeconds = Number(event.target.value);
                      setSelectedClip(clip.id);
                    }}
                  />
                </label>
              </div>
              <label>
                {label(locale, "Transition", "转场")}
                <select
                  value={clip.transition}
                  onChange={(event) => {
                    clip.transition = event.target.value;
                    setSelectedClip(clip.id);
                  }}
                >
                  <option value="cut">Cut</option>
                  <option value="dissolve">Dissolve</option>
                  <option value="fade">Fade</option>
                </select>
              </label>
              <label>
                {label(locale, "Fit", "画面适配")}
                <select
                  value={clip.fit}
                  onChange={(event) => {
                    clip.fit = event.target.value;
                    setSelectedClip(clip.id);
                  }}
                >
                  <option value="cover">Cover</option>
                  <option value="contain">Contain</option>
                </select>
              </label>
              <button
                className="button primary"
                onClick={() => void saveEdit()}
              >
                <Save size={15} />
                {label(locale, "Save edit", "保存剪辑")}
              </button>
            </>
          )}
          {validation && (
            <div className={validation.valid ? "validation ok" : "validation"}>
              <strong>
                {validation.valid
                  ? label(locale, "Ready to render", "可以渲染")
                  : label(locale, "Needs attention", "需要处理")}
              </strong>
              <small>
                {validation.durationSeconds.toFixed(1)}s ·{" "}
                {validation.issues.length} {label(locale, "issues", "项问题")}
              </small>
            </div>
          )}
          <div className="continuity-list">
            <h3>{label(locale, "Continuity", "连续性检查")}</h3>
            {snapshot.continuityIssues
              .filter(
                (item) => !item.episodeId || item.episodeId === episode.id,
              )
              .slice(0, 10)
              .map((item, index) => (
                <div key={`${item.code}-${index}`} className={item.severity}>
                  <CircleAlert size={14} />
                  <span>{item.message}</span>
                </div>
              ))}
          </div>
        </div>
      </aside>
    </div>
  );
}

function Timeline({
  locale,
  edit,
  selected,
  onSelect,
}: {
  locale: Locale;
  edit: EpisodeEdit;
  selected?: string;
  onSelect: (id: string) => void;
}) {
  const duration = Math.max(
    1,
    edit.videoTrack.reduce(
      (sum, clip) =>
        sum + clip.outSeconds - clip.inSeconds - (clip.transitionSeconds ?? 0),
      0,
    ),
  );
  const width = (seconds: number) =>
    `${Math.max(4, (seconds / duration) * 100)}%`;
  return (
    <div className="timeline">
      <div className="lane">
        <strong>{label(locale, "Video", "视频")}</strong>
        <div>
          {edit.videoTrack.map((clip) => (
            <button
              key={clip.id}
              className={selected === clip.id ? "clip selected" : "clip"}
              style={{ width: width(clip.outSeconds - clip.inSeconds) }}
              onClick={() => onSelect(clip.id)}
            >
              {clip.order + 1}
              <small>{clip.transition}</small>
            </button>
          ))}
        </div>
      </div>
      {(["dialogue", "sfx", "bgm"] as const).map((lane) => (
        <div className="lane" key={lane}>
          <strong>{human(lane)}</strong>
          <div>
            {edit.audioCues
              .filter((cue) => cue.lane === lane)
              .map((cue) => (
                <span
                  key={cue.id}
                  className={`audio-cue ${lane}`}
                  style={{
                    marginLeft: width(cue.startSeconds),
                    width: width(cue.durationSeconds),
                  }}
                >
                  {cue.id.slice(0, 8)}
                </span>
              ))}
          </div>
        </div>
      ))}
      <div className="lane">
        <strong>{label(locale, "Subtitles", "字幕")}</strong>
        <div>
          {edit.subtitleCues.map((cue) => (
            <span
              key={cue.id}
              className="subtitle-cue"
              style={{
                marginLeft: width(cue.startSeconds),
                width: width(cue.durationSeconds),
              }}
            >
              {cue.text}
            </span>
          ))}
        </div>
      </div>
    </div>
  );
}

function RunsPage({
  locale,
  snapshot,
  approvals,
  onApproval,
  onRefresh,
  onError,
}: {
  locale: Locale;
  snapshot: ProjectSnapshot | null;
  approvals: Approval[];
  onApproval: (item: Approval, approved: boolean) => void;
  onRefresh: () => Promise<void>;
  onError: (value: string) => void;
}) {
  if (!snapshot) return <Empty locale={locale} />;
  const cancel = async (run: Run) => {
    try {
      if (run.operation === "episode_render") await RenderAPI.Cancel(run.id);
      else await AssetAPI.CancelRun(run.id);
      await onRefresh();
    } catch (cause) {
      onError(String(cause));
    }
  };
  return (
    <Page
      title={label(locale, "Runs & approvals", "任务与审批")}
      subtitle={label(
        locale,
        `${snapshot.runs.length} runs · ${approvals.length} pending`,
        `${snapshot.runs.length} 个任务 · ${approvals.length} 个待审批`,
      )}
    >
      {approvals.length > 0 && (
        <div className="approval-list">
          {approvals.map((item) => (
            <article key={item.id}>
              <ShieldCheck size={18} />
              <div>
                <strong>{human(item.action)}</strong>
                <p>{item.summary}</p>
              </div>
              <button onClick={() => onApproval(item, false)}>
                {label(locale, "Decline", "拒绝")}
              </button>
              <button
                className="primary"
                onClick={() => onApproval(item, true)}
              >
                {label(locale, "Approve once", "仅批准本次")}
              </button>
            </article>
          ))}
        </div>
      )}
      <div className="runs-table">
        <div className="table-head">
          <span>ID</span>
          <span>{label(locale, "Operation", "操作")}</span>
          <span>{label(locale, "Episode / shot", "分集 / 镜头")}</span>
          <span>{label(locale, "Status", "状态")}</span>
          <span>{label(locale, "Progress", "进度")}</span>
          <span />
        </div>
        {snapshot.runs.map((run) => (
          <div className="table-row" key={run.id}>
            <code>{run.id.slice(0, 8)}</code>
            <span>{human(run.operation)}</span>
            <span>{run.episodeId || run.shotId || "—"}</span>
            <span>
              <i className={`status-dot ${run.status}`} />
              {human(run.status)}
            </span>
            <span>
              {run.progress || (run.status === "succeeded" ? 100 : 0)}%
            </span>
            <span>
              {run.status === "running" && (
                <button onClick={() => void cancel(run)}>
                  {label(locale, "Cancel", "取消")}
                </button>
              )}
            </span>
          </div>
        ))}
      </div>
    </Page>
  );
}

function SettingsPage({
  locale,
  snapshot,
  onRefresh,
  onError,
}: {
  locale: Locale;
  snapshot: ProjectSnapshot | null;
  onRefresh: () => Promise<void>;
  onError: (value: string) => void;
}) {
  const [status, setStatus] = useState<Awaited<
    ReturnType<typeof SettingsAPI.Status>
  > | null>(null);
  const [codex, setCodex] = useState<Awaited<
    ReturnType<typeof RuntimeAPI.DetectCodex>
  > | null>(null);
  const [ffmpeg, setFFmpeg] = useState<Awaited<
    ReturnType<typeof RenderAPI.Runtime>
  > | null>(null);
  const [key, setKey] = useState("");
  const [orientation, setOrientation] = useState("portrait");
  const [contentLanguage, setContentLanguage] = useState("zh-CN");
  const [burnSubtitles, setBurnSubtitles] = useState(true);
  const load = async () => {
    try {
      setStatus(await SettingsAPI.Status());
      setCodex(await RuntimeAPI.DetectCodex());
      setFFmpeg(await RenderAPI.Runtime());
    } catch (cause) {
      onError(String(cause));
    }
  };
  useEffect(() => {
    void load();
  }, []);
  useEffect(() => {
    if (!snapshot) return;
    setOrientation(snapshot.project.output.orientation);
    setContentLanguage(snapshot.project.contentLanguage);
    setBurnSubtitles(snapshot.project.output.burnSubtitles);
  }, [snapshot?.root, snapshot?.project.updatedAt]);
  const saveKey = async (event: FormEvent) => {
    event.preventDefault();
    try {
      await SettingsAPI.SetOpenAIKey(key);
      setKey("");
      await load();
    } catch (cause) {
      onError(String(cause));
    }
  };
  return (
    <Page
      title={label(locale, "Settings", "设置")}
      subtitle={label(
        locale,
        "Runtimes, output, credentials, and privacy.",
        "运行时、成片规格、凭据与隐私。",
      )}
    >
      <div className="settings-stack">
        <article className="card">
          <div>
            <h2>Codex & ChatGPT</h2>
            <p>
              {label(
                locale,
                "Agent runtime and ChatGPT sign-in are managed by Codex app-server.",
                "Agent 运行时和 ChatGPT 登录由 Codex app-server 管理。",
              )}
            </p>
            <StatusLine
              ok={Boolean(codex?.compatible)}
              text={
                codex?.compatible
                  ? `${codex.version} · ${codex.source}`
                  : codex?.error || label(locale, "Checking…", "检查中…")
              }
            />
          </div>
          <div className="row-actions">
            <button
              className="button secondary"
              onClick={() =>
                void RuntimeAPI.EnsureCodex()
                  .then(setCodex)
                  .catch((cause) => onError(String(cause)))
              }
            >
              {label(locale, "Verify runtime", "验证运行时")}
            </button>
            <button
              className="button primary"
              onClick={async () => {
                try {
                  await AgentAPI.Start();
                  try {
                    await AgentAPI.Account();
                  } catch {
                    const login = await AgentAPI.StartChatGPTLogin();
                    if (login.authUrl) BrowserOpenURL(String(login.authUrl));
                  }
                } catch (cause) {
                  onError(String(cause));
                }
              }}
            >
              {label(locale, "Connect ChatGPT", "连接 ChatGPT")}
            </button>
          </div>
        </article>
        <article className="card">
          <div>
            <h2>FFmpeg</h2>
            <p>
              {label(
                locale,
                "Required for probing, fixed-timeline rendering, subtitles, and audio mix.",
                "用于素材探测、固定时间线渲染、字幕与混音。",
              )}
            </p>
            <StatusLine
              ok={Boolean(ffmpeg?.compatible)}
              text={
                ffmpeg?.compatible
                  ? `${ffmpeg.version} · ${ffmpeg.encoder}`
                  : ffmpeg?.error || label(locale, "Not found", "未找到")
              }
            />
          </div>
          <button
            className="button secondary"
            onClick={() => void RenderAPI.Runtime().then(setFFmpeg)}
          >
            {label(locale, "Check again", "重新检查")}
          </button>
        </article>
        <article className="card">
          <div>
            <h2>{label(locale, "OpenAI media key", "OpenAI 媒体密钥")}</h2>
            <p>
              {label(
                locale,
                `Stored only in macOS Keychain (${status?.keychainService ?? "dev.bg-dao.dramaops"}).`,
                `仅存入 macOS Keychain（${status?.keychainService ?? "dev.bg-dao.dramaops"}）。`,
              )}
            </p>
            <StatusLine
              ok={Boolean(status?.openaiKeyConfigured)}
              text={
                status?.openaiKeyConfigured
                  ? label(locale, "Configured", "已配置")
                  : label(locale, "Not configured", "未配置")
              }
            />
            <form onSubmit={saveKey}>
              <input
                type="password"
                value={key}
                onChange={(event) => setKey(event.target.value)}
                placeholder="sk-…"
              />
              <button className="button primary" disabled={!key}>
                {label(locale, "Save", "保存")}
              </button>
            </form>
          </div>
        </article>
        {snapshot && (
          <article className="card">
            <div>
              <h2>{label(locale, "Series output", "项目成片规格")}</h2>
              <div className="field-grid">
                <label>
                  {label(locale, "Frame format", "画幅")}
                  <select
                    value={orientation}
                    onChange={(event) => setOrientation(event.target.value)}
                  >
                    <option value="portrait">9:16 · 1080×1920</option>
                    <option value="landscape">16:9 · 1920×1080</option>
                  </select>
                </label>
                <label>
                  {label(locale, "Content language", "内容语言")}
                  <select
                    value={contentLanguage}
                    onChange={(event) => setContentLanguage(event.target.value)}
                  >
                    <option value="zh-CN">简体中文</option>
                    <option value="en">English</option>
                  </select>
                </label>
                <label>
                  {label(locale, "Burn subtitles", "烧录字幕")}
                  <select
                    value={burnSubtitles ? "yes" : "no"}
                    onChange={(event) =>
                      setBurnSubtitles(event.target.value === "yes")
                    }
                  >
                    <option value="yes">{label(locale, "Yes", "是")}</option>
                    <option value="no">{label(locale, "No", "否")}</option>
                  </select>
                </label>
              </div>
            </div>
            <button
              className="button primary"
              onClick={() => {
                const settings = domain.Project.createFrom(snapshot.project);
                const portrait = orientation === "portrait";
                settings.contentLanguage = contentLanguage;
                settings.output.orientation = portrait
                  ? "portrait"
                  : "landscape";
                settings.output.width = portrait ? 1080 : 1920;
                settings.output.height = portrait ? 1920 : 1080;
                settings.output.burnSubtitles = burnSubtitles;
                void ProjectAPI.SaveSettings(settings)
                  .then(onRefresh)
                  .catch((cause) => onError(String(cause)));
              }}
            >
              {label(locale, "Save output", "保存规格")}
            </button>
          </article>
        )}
        <article className="card privacy">
          <ShieldCheck size={24} />
          <div>
            <h2>{label(locale, "Private by default", "默认保护隐私")}</h2>
            <p>
              {label(
                locale,
                "No telemetry or cloud project sync. Paid actions require one-time approval. API keys, custom voice IDs, consent recordings, and samples are excluded from project data and exports.",
                "无遥测、无云端项目同步；付费操作逐次审批。API Key、自定义 voice ID、授权录音和声音样本不会进入项目或导出包。",
              )}
            </p>
          </div>
        </article>
      </div>
    </Page>
  );
}

function AssetPreview({ asset, empty }: { asset?: Asset; empty: string }) {
  const [source, setSource] = useState("");
  useEffect(() => {
    let active = true;
    setSource("");
    if (asset)
      void AssetAPI.DataURL(asset.id)
        .then((value) => {
          if (active) setSource(value);
        })
        .catch(() => undefined);
    return () => {
      active = false;
    };
  }, [asset?.id]);
  if (!asset)
    return (
      <div className="asset-stage empty">
        <Film size={34} />
        <span>{empty}</span>
      </div>
    );
  return (
    <div className="asset-stage">
      {source && (asset.kind === "video" || asset.kind === "render") && (
        <video src={source} controls preload="metadata" />
      )}
      {source && (asset.kind === "image" || asset.kind === "reference") && (
        <img src={source} alt={`DramaOps asset ${asset.id}`} />
      )}
      {source && asset.kind === "audio" && <audio src={source} controls />}
      {!source && <LoaderCircle className="spin" size={24} />}
      <small>
        {asset.provenance.model || asset.provenance.provider} ·{" "}
        {asset.sha256.slice(0, 12)}…
      </small>
    </div>
  );
}

function ApprovalCard({
  locale,
  approval,
  queued,
  onResolve,
}: {
  locale: Locale;
  approval: Approval;
  queued: number;
  onResolve: (approved: boolean) => void;
}) {
  return (
    <aside className="approval-overlay">
      <header>
        <ShieldCheck size={20} />
        <div>
          <span>{label(locale, "Approval required", "需要审批")}</span>
          <strong>{human(approval.action)}</strong>
        </div>
        {queued > 0 && <small>+{queued}</small>}
      </header>
      <p>{approval.summary}</p>
      <div className="approval-actions">
        <button onClick={() => onResolve(false)}>
          {label(locale, "Decline", "拒绝")}
        </button>
        <button className="primary" onClick={() => onResolve(true)}>
          {label(locale, "Approve once", "仅批准本次")}
        </button>
      </div>
    </aside>
  );
}

function AgentDrawer({
  locale,
  state,
  threadID,
  open,
  approvals,
  onToggle,
  onError,
}: {
  locale: Locale;
  state: typeof initialAgentState;
  threadID?: string;
  open: boolean;
  approvals: number;
  onToggle: () => void;
  onError: (value: string) => void;
}) {
  const [prompt, setPrompt] = useState("");
  const send = async () => {
    if (!prompt.trim()) return;
    try {
      await AgentAPI.Start();
      await AgentAPI.StartTurn(prompt);
      setPrompt("");
    } catch (cause) {
      onError(String(cause));
    }
  };
  return (
    <aside className={open ? "agent-drawer open" : "agent-drawer"}>
      <button className="drawer-handle" onClick={onToggle}>
        <div>
          <MessageSquareText size={16} />
          <strong>DramaOps Agent</strong>
          {state.activeTurnId && (
            <span className="working">
              ● {label(locale, "working", "处理中")}
            </span>
          )}
          {approvals > 0 && <span className="approval-badge">{approvals}</span>}
        </div>
        <ChevronDown size={16} />
      </button>
      {open && (
        <div className="drawer-content">
          <div className="agent-output">
            {state.streamingText ||
              state.error ||
              label(
                locale,
                "Agent output, tool calls, and approvals remain inspectable here.",
                "Agent 输出、工具调用与审批会显示在这里。",
              )}
          </div>
          <div className="agent-compose">
            <Sparkles size={16} />
            <input
              value={prompt}
              onChange={(event) => setPrompt(event.target.value)}
              onKeyDown={(event) => {
                if (event.key === "Enter") void send();
              }}
              placeholder={label(
                locale,
                "Ask DramaOps about this episode…",
                "让 DramaOps 优化本集…",
              )}
            />
            {state.activeTurnId ? (
              <button
                onClick={() => {
                  if (threadID)
                    void AgentAPI.InterruptTurn(
                      threadID,
                      state.activeTurnId!,
                    ).catch((cause) => onError(String(cause)));
                }}
              >
                <CircleStop size={18} />
              </button>
            ) : (
              <button onClick={() => void send()}>
                <Send size={18} />
              </button>
            )}
          </div>
        </div>
      )}
    </aside>
  );
}

function Page({
  title,
  subtitle,
  children,
}: {
  title: string;
  subtitle: string;
  children: ReactNode;
}) {
  return (
    <section className="content-page">
      <div className="page-title">
        <div>
          <h1>{title}</h1>
          <p>{subtitle}</p>
        </div>
      </div>
      <div className="page-scroll">{children}</div>
    </section>
  );
}
function PanelTitle({ title, detail }: { title: string; detail?: string }) {
  return (
    <div className="panel-title">
      <strong>{title}</strong>
      {detail && <small>{detail}</small>}
    </div>
  );
}
function Empty({ locale }: { locale: Locale }) {
  return (
    <div className="stage-empty page-fill">
      <FolderOpen size={34} />
      <h2>{label(locale, "Open a DramaOps series", "打开 DramaOps 项目")}</h2>
      <p>
        {label(
          locale,
          "Create or open a local project first.",
          "请先创建或打开本地项目。",
        )}
      </p>
    </div>
  );
}
function Banner({
  kind,
  title,
  detail,
  onClose,
}: {
  kind: "error" | "success";
  title: string;
  detail?: string;
  onClose: () => void;
}) {
  return (
    <div className={`banner ${kind}`}>
      {kind === "error" ? <CircleAlert size={16} /> : <Check size={16} />}
      <div>
        <strong>{title}</strong>
        {detail && <small>{detail}</small>}
      </div>
      <button onClick={onClose}>
        <X size={15} />
      </button>
    </div>
  );
}
function StatusLine({ ok, text }: { ok: boolean; text: string }) {
  return (
    <div className={ok ? "status-line ok" : "status-line"}>
      {ok ? <Check size={14} /> : <CircleAlert size={14} />}
      {text}
    </div>
  );
}
function human(value: string) {
  return value
    .replaceAll("_", " ")
    .replaceAll("/", " ")
    .replace(/\b\w/g, (char) => char.toUpperCase());
}

export default App;
