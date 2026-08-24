<p align="center">
  <img src="assets/brand/mnemo-logo.png" alt="mnemo 标志" width="128" height="128">
</p>

<h1 align="center">mnemo</h1>

<p align="center">
  <strong>面向 AI 编程代理的持久化记忆。</strong>
</p>

<p align="center">
  为 Claude Code、Codex、Cursor、Windsurf、OpenCode 和 fx 提供一套共享的本地记忆，跨会话、压缩和代理切换仍然保留。
</p>

<p align="center">
  <a href="README.md">English</a> · <a href="README.es.md">Español</a> · <a href="README.zh-CN.md">简体中文</a>
</p>

<p align="center">
  <a href="https://go.dev"><img alt="Go" src="https://img.shields.io/badge/go-1.26-00ADD8?logo=go&logoColor=white"></a>
  <a href="https://github.com/jmeiracorbal/mnemo"><img alt="状态" src="https://img.shields.io/badge/status-stable-brightgreen"></a>
  <a href="https://sqlite.org"><img alt="存储" src="https://img.shields.io/badge/storage-SQLite%2BFTS5-003B57?logo=sqlite&logoColor=white"></a>
  <a href="https://github.com/jmeiracorbal/mnemo"><img alt="平台" src="https://img.shields.io/badge/platform-macOS%20%7C%20Linux-lightgrey"></a>
  <a href="LICENSE"><img alt="许可证" src="https://img.shields.io/badge/license-Apache%202.0-blue"></a>
</p>

<p align="center">
  <a href="#快速开始">快速开始</a> ·
  <a href="#为什么选择-mnemo">为什么选择 mnemo?</a> ·
  <a href="#支持的代理">代理</a> ·
  <a href="#文档">文档</a> ·
  <a href="ROADMAP.md">路线图</a>
</p>

---

## 什么是 mnemo？

mnemo 是面向代理式开发的本地记忆层。它把决策、Bug、约定、发现和会话摘要保存在 SQLite 中，并通过 MCP 工具、hooks 和可移植的 Agent Skills 暴露给代理使用。

你不再需要把项目知识分散在 `MEMORY.md`、编辑器原生记忆、聊天记录和人工笔记里。mnemo 为所有支持的代理提供同一个按项目隔离的可信记忆来源。

> [!IMPORTANT]
> mnemo 并不会自动支持所有 harness；它提供的是一个稳定的记忆契约，任何 harness 都可以实现并验证这个契约。


## 快速开始

安装二进制文件并配置检测到的代理：

```bash
curl -sSf https://raw.githubusercontent.com/jmeiracorbal/mnemo/main/install.sh | bash
```

在项目中启用 mnemo：

```bash
cd your-project
mnemo init --agent=all
```

检查所有集成是否正确连接：

```bash
mnemo doctor --agent=all --path=.
```

也可以从 CLI 手动保存和搜索记忆：

```bash
mnemo save "使用 SQLite FTS5" "搜索保持本地、快速且依赖很少。" --type decision --project myapp
mnemo search "SQLite" --project myapp
```

## 为什么选择 mnemo？

| 问题 | mnemo 提供 |
|---|---|
| 代理在不同会话之间忘记决策 | 保存在 `~/.mnemo/memory.db` 中的持久化项目记忆 |
| 不同代理维护不同的记忆 | Claude Code、Codex、Cursor、Windsurf、OpenCode 和 fx 共享同一层记忆 |
| Markdown 记忆文件容易漂移或冲突 | 结构化 observations、tags、topic keys 和 review 状态 |
| 全局 hooks 可能有风险 | 通过 `.mnemo` 标记按项目显式启用；没有标记的项目会被忽略 |
| 安装问题可能静默失败 | `mnemo doctor` 和 `mnemo setup status` 会说明当前配置状态 |
| 重复项目和记忆不断累积 | 项目合并工具和记忆整理流程 |

## 功能

| 功能 | 作用 |
|---|---|
| **按项目启用** | 全局 hooks 只会在项目包含有效 `.mnemo` 标记时运行。 |
| **MCP 工具** | 代理可以调用 `mem_save`、`mem_search`、`mem_context`、`mem_current_project`、`mem_doctor` 等工具。 |
| **会话 hooks** | 记录会话活动、注入上下文，并自动捕获学习内容。 |
| **可移植 Agent Skills** | 教会兼容代理何时以及如何使用 mnemo，而不是回退到原生记忆。 |
| **被动捕获** | 从 transcript 和子代理输出中提取有价值的学习内容。 |
| **代理溯源** | 对提供相关信息的写入，记录可通过 SQL 查询的代理、来源、工具、模型和 MCP 客户端元数据。 |
| **诊断** | `mnemo doctor` 检查项目启用、全局配置、MCP、hooks、竞争记忆表面和存储健康状态。 |
| **项目维护** | `mnemo projects list`、`mnemo projects merge` 和 `mnemo projects rename` 帮助整理重复或不清晰的项目身份。 |
| **记忆整理** | `mnemo memories review` 显示可能重复或冲突的 observations，便于经过确认后修复。 |

## 支持的代理

<p align="center">
  <img alt="Claude Code" src="https://img.shields.io/badge/Claude%20Code-supported-6B46C1?logo=claudecode&logoColor=white">
  <img alt="Codex" src="https://img.shields.io/badge/Codex-supported-00A67E?logo=openai&logoColor=white">
  <img alt="Cursor" src="https://img.shields.io/badge/Cursor-supported-111111?logo=cursor&logoColor=white">
  <img alt="Windsurf" src="https://img.shields.io/badge/Windsurf-supported-2563EB?logo=windsurf&logoColor=white">
  <img alt="OpenCode" src="https://img.shields.io/badge/OpenCode-supported-F97316?logo=opencode&logoColor=white">
  <img alt="fx" src="https://img.shields.io/badge/fx-supported-7C3AED?logo=vercel&logoColor=white">
</p>

| 代理 | MCP | Hooks / runtime | 全局指令 | Skill 访问 | 状态 |
|---|---:|---:|---:|---:|---|
| Claude Code | ✅ | Plugin 或通过 `install.sh` 为 n/a | ✅ | ✅ | 支持 |
| Codex | ✅ | ✅ | ✅ | ✅ | 支持 |
| Cursor | ✅ | ✅ | ✅ | ✅ | 支持 |
| Windsurf | ✅ | ✅ | ✅ | ✅ | 支持 |
| OpenCode | ✅ | ✅ | ✅ | ✅ | 支持 |
| fx | ✅ | n/a | ✅ | 通过标准路径 ✅ | 支持 |

全局配置只需安装一次。项目启用仍然是本地、显式的 opt-in：

```text
project/
├── .mnemo      # 项目 ID + 已启用代理，忽略提交到 git
├── AGENTS.md   # 共享项目记忆权威
├── CLAUDE.md   # 选择 Claude 时使用的专用规则
└── .cursor/    # 选择 Cursor 时使用的规则
```

## 实际效果

```text
$ mnemo doctor --agent=all --path=.
status: ok
checks: project marker, binary, MCP, hooks, instructions, store

$ mnemo context myapp
## Memory from Previous Sessions
- Chose SQLite FTS5 for local search.
- Refresh hooks must keep executable permissions.

$ mnemo memories review --project=myapp
No potential memory conflicts found.
```

## 安装选项

| 方式 | 适用场景 | 命令 |
|---|---|---|
| 自动安装 | 需要二进制文件并配置检测到的代理 | <code>curl -sSf https://raw.githubusercontent.com/jmeiracorbal/mnemo/main/install.sh &#124; bash</code> |
| 指定代理 | 只想配置某一个集成 | `bash -s -- --agent=codex` |
| 所有代理 | 想准备所有支持的集成 | `bash -s -- --agent=all` |
| Claude plugin | 使用 Claude Code 的 plugin marketplace | `claude plugin install mnemo@mnemo` |
| 源码构建 | 开发 mnemo 本身 | `go build -o ~/.local/bin/mnemo ./cmd/mnemo/` |

完整安装指南见 [docs/INSTALLATION.md](docs/INSTALLATION.md)。

## 文档

| 指南 | 内容 |
|---|---|
| [文档索引](docs/README.md) | 完整文档地图和研究笔记。 |
| [安装](docs/INSTALLATION.md) | install script、plugin setup、项目启用和验证。 |
| [代理集成](docs/AGENT_INTEGRATION.md) | Hook 行为、全局路径、`.mnemo` 标记和 Agent Skills。 |
| [CLI 参考](docs/CLI.md) | 命令、示例、MCP 工具和搜索模式。 |
| [故障排查](docs/TROUBLESHOOTING.md) | `doctor`、`setup status`、手动检查和幂等性验证。 |
| [存储](docs/STORAGE.md) | SQLite 位置、schema 说明和 sqlc 工作流。 |
| [路线图](ROADMAP.md) | 计划中的产品和维护工作。 |

## 设计原则

- **本地优先：** 记忆保存在你机器上的 SQLite 中。
- **代理中立：** 所有支持的编程代理共享同一个记忆权威。
- **按项目 opt-in：** 没有 `.mnemo` 的项目不会触发全局集成。
- **可诊断：** 每个 setup surface 都可以在不修改状态的情况下检查。
- **可修复：** 重复项目身份和记忆冲突都可以通过 CLI primitives 查看并修复。

## 许可证

[Apache 2.0](LICENSE)：你可以自由使用、修改和分发，但必须保留版权声明，并在所有分发中包含 [NOTICE](NOTICE) 文件。
