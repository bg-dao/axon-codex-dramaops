# SceneOps by Axon

[English](README.md) | 简体中文

**SceneOps — 一个由 Codex agent harness 驱动的开源多模态场景与资产工作台。**

SceneOps 是一款面向独立创作者的本地优先桌面工作台。它将创作简报转化为结构化分镜、关键帧、视频镜头、版本决策以及可导出的资产与溯源包，同时确保项目文件清晰易懂且便于迁移。

> 状态：v0.1 核心流程已经实现。真实 OpenAI 媒体调用仍保持手动 opt-in，安装包和创作者示例仍在完善中。

## 为什么选择 SceneOps

- **Agent 原生工作流：** Codex app-server 提供登录、持久化 thread、turn、流式事件和人工审批。
- **本地优先项目：** JSON manifests 和媒体文件是唯一持久化真相；SQLite 仅作为可重建索引。
- **供应商无关资产：** provider 和 model 信息存放在 provenance 中，不进入核心项目 schema。
- **人工控制成本：** 分镜写入和付费媒体操作每次都必须得到明确审批。
- **可迁移输出：** 导出的项目保留 prompt、参数、父资产关系、哈希以及 provider request ID。

## v0.1 工作流

```text
创作简报 -> 分镜 -> 关键帧版本 -> 选择版本 -> 生成或导入视频 -> 导出
```

桌面端流程刻意保持引导式与简洁：

1. 选择本地目录，创建或打开项目。
2. 编写并明确保存 `brief.md`。
3. 让 Codex Agent 通过 SceneOps MCP 工具生成首个 3 scenes / 6 shots 分镜。
4. 对分镜写入执行一次性审批。
5. 生成关键帧版本、附加参考图并选择首选图片。
6. 导入已完成的视频，或使用通过能力探测的视频 provider。
7. 导出确定性的 manifests、媒体、runs 与 provenance，并显示导出包 SHA-256。

审批会同时出现在当前工作区和 Runs 页面。付费生成与任务取消都不会提供永久允许选项。

首个版本面向 Apple Silicon Mac。架构会保留 Windows 兼容边界，但 Windows 打包不属于 v0.1 发布门槛。

## 架构

```text
React + TypeScript UI
        |
Wails 生成的 bindings 和事件
        |
Go 应用服务
  |              |                 |
项目存储         Codex app-server  MediaProvider
(JSON + 媒体)    (JSONL JSON-RPC)  (首个实现为 OpenAI)
  |              |
SQLite 索引      SceneOps stdio MCP
```

SceneOps 为每个活动项目启动一个 Codex app-server。它将项目根目录作为 `cwd`，使用 `workspaceWrite` sandbox 和 `on-request` approval policy，并通过命令行配置覆盖注入 SceneOps MCP server。SceneOps 不会修改用户的全局 Codex 配置。

## 开发

前置要求：

- Go 1.25 或更高版本
- Node.js 20 或更高版本
- Wails 2.15 或更高版本
- 支持 `app-server` 的 Codex CLI

```bash
npm --prefix frontend install
npm --prefix frontend test
npm --prefix frontend run build
go test -race ./...
wails dev
```

直接运行 MCP server：

```bash
go run ./cmd/sceneops-mcp --project /absolute/path/to/a/sceneops/project
```

在不修改项目的情况下运行真实 app-server walking-skeleton 技术闸门：

```bash
go run ./cmd/sceneops-harness-smoke \
  --project /absolute/path/to/a/sceneops/project \
  --mcp-command /absolute/path/to/SceneOps \
  --prompt "Reply with exactly: SceneOps harness ready. Do not call tools or modify files."
```

该命令默认使用锁定且经过校验的 runtime。smoke 命令中的预发布兼容参数只用于诊断，桌面应用不会使用这些参数。

OpenAI 媒体生成使用单独的 API key，并由操作系统 Keychain 以 service `dev.bg-dao.sceneops` 保存。该 key 永远不会写入 SceneOps 项目、SQLite 索引、日志或导出包。

视频导入是 v0.1 的稳定主路径。当前 OpenAI Videos adapter 被明确标记为实验能力，因为[官方 API](https://developers.openai.com/api/reference/typescript/resources/videos/methods/create) 已弃用并计划于 2026 年 9 月 24 日关闭。它可以使用已选关键帧作为可选图片输入；未来替换 provider 时，供应商无关的资产关系仍然有效。

## 项目目录

```text
my-film/
├── sceneops.project.json
├── brief.md
├── AGENTS.md
├── scenes/
├── shots/
├── assets/<asset-id>/asset.json
├── runs/<run-id>.json
├── exports/
└── .sceneops/index.sqlite
```

具体实现契约请参阅 [docs/architecture.md](docs/architecture.md)、[docs/project-format.md](docs/project-format.md) 和 [docs/security.md](docs/security.md)。

## 与 Axon 的关系

SceneOps 是 Axon 旗下一个自包含的开源项目。它的源码、依赖、CI、版本发布和公开路线图均在本仓库中独立维护。

## 贡献与安全

开发流程请参阅 [CONTRIBUTING.md](CONTRIBUTING.md)。安全漏洞请按照 [SECURITY.md](SECURITY.md) 私下报告，不要提交公开 issue。

## 许可证

Apache License 2.0，详见 [LICENSE](LICENSE)。
