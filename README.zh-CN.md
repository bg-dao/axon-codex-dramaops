# DramaOps by Axon

[English](README.md) | 简体中文

**DramaOps — 基于 Codex agent harness 的开源 AI 短剧工作台。**

DramaOps 是一款本地优先的 AI 短剧制作桌面工作台。它把结构化剧本、人物与场景圣经、专业镜头表、参考图一致性关键帧、视频片段、固定人物声音、音效、字幕、固定剧情时间线、连续性检查和可复现导出连接成一个完整流程。

> 状态：v0.2 alpha 实施阶段。项目、Agent、媒体、配音、连续性、时间线、渲染和导出核心路径已实现；真实 provider 调用仍需手动 opt-in，签名发行尚未达到稳定版本标准。

## 核心流程

```text
系列设定 → 分集剧本 → 角色/场景圣经 → 专业镜头表 → 一致性关键帧
→ 视频片段 → 配音与音效 → 固定剧情时间线 → 连续性检查 → 导出
```

DramaOps 默认面向竖屏短剧：`9:16`、`1080×1920`、`25fps`，单集约 1–3 分钟；同时支持横屏项目。

- **系列级连续性：** 多集复用人物、Voice Profile、场景、道具、视觉风格、环境底噪、motif 和 BGM。
- **结构化剧本真相：** 分集 JSON 是唯一真相；Fountain 可导入、可重新生成，但不会成为第二份易漂移数据源。
- **专业镜头描述：** 明确记录景别、机位、运镜、焦段、构图、对焦、调度、光线、屏幕方向、视线、服装、道具和转场。
- **参考资产组装：** 关键帧请求自动带上相关视觉风格、人物、场景、道具与镜头参考；视频优先使用已选关键帧。
- **人物声音锁定：** 每个角色固定一个内置、自定义或外部 Voice Profile；自定义 provider voice ID 与 consent ID 只保存在 macOS Keychain。
- **聚焦式剪辑：** 一条固定顺序视频轨，加对白、音效、BGM 和字幕 lane，不扩张为通用专业 NLE。
- **可复现交付：** MP4、SRT、Fountain、manifests、连续性报告、哈希和 provenance 一并导出。

## Codex agent harness

Codex Agent 负责结构化剧本、系列圣经和专业镜头表。媒体生成、配音、时间线调整和渲染仍由用户通过明确按钮触发。

DramaOps 通过 JSONL/JSON-RPC 直接连接 `codex app-server`。活动项目关联一个 app-server 进程，项目根目录作为 `cwd`，采用 `workspaceWrite` sandbox 与 `onRequest` approval。应用通过临时命令行配置注入 stdio MCP server，不修改用户的全局 Codex 配置。

公开 MCP 契约固定为八个工具：

```text
dramaops_project_read       dramaops_script_apply
dramaops_shotplan_apply     dramaops_image_generate
dramaops_video_generate     dramaops_speech_generate
dramaops_job_status         dramaops_job_cancel
```

Agent 写入剧本或镜头表、付费媒体或自定义声音操作，以及 provider 任务取消都需要新的一次性审批，不提供付费操作永久批准。

## 本地优先项目与渲染

JSON manifests 和媒体字节是持久化真相；`.dramaops/index.sqlite` 只是可丢弃索引，删除后可从 manifests 无损重建。

本地渲染使用 `ffprobe` 验证素材，使用 FFmpeg 完成裁切、规格统一、硬切/淡化/溶解、字幕烧录、对白/音效/BGM 混音、BGM ducking、响度归一化以及 H.264/AAC 输出。默认成片为 1080×1920、25fps、48kHz stereo、`-16 LUFS`、`-1 dBTP`。

OpenAI 是首个图片和语音 provider。存在参考图时，GPT Image 使用多参考图编辑路径。视频生成按能力探测并标记为实验能力；稳定主路径是外部视频导入，因为 [OpenAI Videos API](https://developers.openai.com/api/reference/typescript/resources/videos/methods/create) 已弃用，并计划于 2026 年 9 月 24 日关闭。

## 架构

```text
React + TypeScript UI
        │ Wails 生成的 bindings 与 dramaops:* events
Go 应用服务
   ├── 项目存储 + 可重建 SQLite 索引
   ├── Codex app-server + DramaOps stdio MCP
   ├── Image / Video / Speech 能力型 provider
   └── FFmpeg 固定时间线渲染引擎
```

provider 和 model 名称只进入 provenance 与 adapter，不进入核心 schema。OpenAI 媒体密钥与自定义 voice/consent 绑定分别存入 macOS Keychain 的 `dev.bg-dao.dramaops` service；明文秘密不会进入 Snapshot、manifests、日志、SQLite 或导出包。

## 开发

前置要求：

- Go 1.25+
- Node.js 20+
- Wails 2.15+
- 支持 `app-server` 的 Codex CLI
- 本地 macOS 渲染需要支持 `h264_videotoolbox` 的 FFmpeg

```bash
npm --prefix frontend ci
npm --prefix frontend test
npm --prefix frontend run build
go test -race ./...
wails dev
```

直接运行 MCP server：

```bash
go run ./cmd/dramaops-mcp --project /absolute/path/to/a/dramaops/project
```

在不修改项目文件的前提下运行真实 app-server smoke gate：

```bash
go run ./cmd/dramaops-harness-smoke \
  --project /absolute/path/to/a/dramaops/project \
  --mcp-command /absolute/path/to/DramaOps \
  --prompt "Reply with exactly: DramaOps harness ready. Do not call tools or modify files."
```

默认测试不会发起真实 OpenAI 调用。

## 项目目录

```text
my-series/
├── dramaops.project.json
├── episodes/<episode-id>/episode.json
├── episodes/<episode-id>/edit.json
├── characters/<character-id>.json
├── locations/<location-id>.json
├── props/<prop-id>.json
├── scenes/<scene-id>.json
├── shots/<shot-id>.json
├── assets/<asset-id>/asset.json
├── runs/<run-id>.json
├── renders/
├── exports/
└── .dramaops/index.sqlite
```

详见[示例短剧](examples/README.md)、[架构](docs/architecture.md)、[项目格式](docs/project-format.md)、[安全模型](docs/security.md)和[发布门槛](docs/release.md)。

## 贡献与安全

开发流程请参阅 [CONTRIBUTING.md](CONTRIBUTING.md)。安全漏洞请按 [SECURITY.md](SECURITY.md) 私下报告，不要创建公开 issue。

## 许可证

Apache License 2.0，详见 [LICENSE](LICENSE)。
