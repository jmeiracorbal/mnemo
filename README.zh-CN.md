<p align="center">
  <a href="https://jmeiracorbal.github.io/mnemo/">
    <img src="site/favicon.svg" alt="mnemo" width="96" height="96">
  </a>
</p>

<p align="center">
  <a href="https://jmeiracorbal.github.io/mnemo/">
    <img src="assets/brand/mnemo-banner.png" alt="mnemo — 面向 AI 编程代理的持久化记忆" width="920">
  </a>
</p>

<p align="center">
  <strong>在 Claude Code、Codex、Cursor、OpenCode 和 Pi 之间共享同一个可信来源。</strong>
</p>

<p align="center">
  <a href="README.md">English</a> ·
  <a href="README.es.md">Español</a> ·
  <a href="README.zh-CN.md">简体中文</a>
</p>

<p align="center">
  <a href="LICENSE"><img alt="许可证" src="https://img.shields.io/badge/license-Apache%202.0-0f1f38?labelColor=e2eaf2&color=0f1f38"></a>
  <a href="https://github.com/jmeiracorbal/mnemo/releases"><img alt="Release" src="https://img.shields.io/github/v/release/jmeiracorbal/mnemo?include_prereleases&label=release&labelColor=e2eaf2&color=0f1f38"></a>
  <a href="https://github.com/jmeiracorbal/mnemo/stargazers"><img alt="Stars" src="https://img.shields.io/github/stars/jmeiracorbal/mnemo?style=flat&labelColor=e2eaf2&color=0f1f38"></a>
  <a href="https://go.dev"><img alt="Go" src="https://img.shields.io/badge/go-1.26-0f1f38?labelColor=e2eaf2&color=0f1f38"></a>
  <a href="https://sqlite.org"><img alt="存储" src="https://img.shields.io/badge/storage-SQLite%2BFTS5-0f1f38?labelColor=e2eaf2&color=0f1f38"></a>
  <a href="https://github.com/jmeiracorbal/mnemo"><img alt="平台" src="https://img.shields.io/badge/platform-macOS%20%7C%20Linux-0f1f38?labelColor=e2eaf2&color=0f1f38"></a>
</p>

<p align="center">
  <img alt="Claude Code" src="https://img.shields.io/badge/Claude%20Code-supported-0f1f38?labelColor=e2eaf2&color=0f1f38">
  <img alt="Codex" src="https://img.shields.io/badge/Codex-supported-0f1f38?labelColor=e2eaf2&color=0f1f38">
  <img alt="Cursor" src="https://img.shields.io/badge/Cursor-supported-0f1f38?labelColor=e2eaf2&color=0f1f38">
  <img alt="OpenCode" src="https://img.shields.io/badge/OpenCode-supported-0f1f38?labelColor=e2eaf2&color=0f1f38">
  <img alt="Pi" src="https://img.shields.io/badge/Pi-supported-0f1f38?labelColor=e2eaf2&color=0f1f38">
</p>

<p align="center">
  <a href="#快速开始">快速开始</a> ·
  <a href="#为什么选择-mnemo">为什么选择 mnemo?</a> ·
  <a href="#工作原理">工作原理</a> ·
  <a href="#支持的代理">代理</a> ·
  <a href="#文档">文档</a> ·
  <a href="#社区">社区</a> ·
  <a href="ROADMAP.md">路线图</a>
</p>

<p align="center">
  <img src="assets/brand/mnemo-terminal.png" alt="mnemo 终端演示" width="920">
</p>

---

## 为什么选择 mnemo？

代理会遗忘。Markdown 记忆会漂移。Hooks 会竞争。配置会静默失败。

| 没有 mnemo | 使用 mnemo |
|---|---|
| 决策在会话之间消失 | 本地 SQLite 中的持久化项目记忆 |
| `MEMORY.md`、编辑器记忆和聊天笔记分叉 | 所有支持代理共享同一个按项目隔离的可信来源 |
| 全局 hooks 到处运行，或几乎无用 | 通过 `.mnemo` 显式启用；无标记项目会被忽略 |
| 静默配置错误 | `mnemo doctor` 明确说明当前接线状态 |

mnemo 并不声称支持每一种 harness。它提供稳定的记忆契约，任何 harness 都可以实现并验证。

## 快速开始

```bash
# 1. 安装（固定当前 alpha）
curl -sSf https://raw.githubusercontent.com/jmeiracorbal/mnemo/main/install.sh | MNEMO_VERSION=v1.0.0-alpha.4 bash

# 2. 在项目中启用
cd your-project
mnemo init --agent=all

# 3. 验证
mnemo doctor --agent=all --path=.
```

然后在代理中：用 `mem_save` 保存决策，关闭会话，再打开另一个 — `mem_search` / `mem_context` 就能找到它。

```bash
mnemo search "SQLite" --project "$(mnemo json id < .mnemo)"
```

未指定版本的安装和 `mnemo update` 默认跟随稳定版。使用 `mnemo update --prerelease` 可选 alphas/betas。完整安装路径、更新与按代理配置见 [Installation](https://github.com/jmeiracorbal/mnemo/wiki/Installation)。

## 工作原理

<p align="center">
  <img src="assets/brand/mnemo-flow.png" alt="代理 → mnemo → 本地 SQLite 记忆" width="920">
</p>

1. **代理** 通过 MCP 工具、hooks 和可移植 Agent Skills 与 mnemo 交互。
2. **本地事件控制器** 是正常运行时唯一的 SQLite 写入者 — 发布者不直接打开数据库。
3. **记忆** 留在你的机器上：结构化 observations、tags、topic keys 和会话摘要，存于 SQLite + FTS5。

```text
project/
├── .mnemo      # 项目 ID + 已启用代理（gitignored）
├── AGENTS.md   # 共享记忆权威
├── CLAUDE.md   # 选择 Claude 时的专用规则
├── .cursor/    # 选择 Cursor 时的规则
└── .pi/        # 选择 Pi 时的 prompt 扩展
```

投递保证与故障行为见 [Durable Events](https://github.com/jmeiracorbal/mnemo/wiki/Durable-Events)。

## 亮点

- **按项目启用** — 没有有效 `.mnemo` 时全局 hooks 保持惰性
- **MCP + 原生工具** — `mem_save`、`mem_search`、`mem_context`、`mem_doctor` 等
- **持久事件控制器** — 每用户 JetStream；幂等 SQLite 事务
- **控制器管理的会话** — 代理原生执行 ID 绑定到规范会话
- **可移植 skills** — 引导代理使用 mnemo，而不是回退原生记忆
- **被动捕获** — 从代理输出中提取有价值的学习
- **溯源** — 可通过 SQL 查询的代理、工具、模型和 MCP 客户端元数据
- **诊断与修复** — `mnemo doctor`、项目 merge/rename、`mnemo memories review`
- **安全迁移与自更新** — 打开时升级 schema；`mnemo update` 管理 releases

## 支持的代理

| 代理 | MCP | Hooks / runtime | 全局指令 | Skill | 状态 |
|---|---:|---:|---:|---:|---|
| Claude Code | 是 | 插件 hooks 或安装器配置 | 是 | 是 | 支持 |
| Codex | 是 | 会话 hooks | 是 | 是 | 支持 |
| Cursor | 是 | 提示词 hook | 是 | 是 | 支持 |
| OpenCode | 是 | 插件事件 | 是 | 是 | 支持 |
| Pi | 原生工具 | 原生扩展 | 是 | 是 | 支持 |

Codex 安装后需要交互式 hook 信任审查 — 详见 [Agent Integrations](https://github.com/jmeiracorbal/mnemo/wiki/Agent-Integrations)。

## 关联模块

| 模块 | 用途 |
|---|---|
| [`mnemo-adapters`](https://github.com/jmeiracorbal/mnemo-adapters) | CLI、控制器与 MCP 使用的代理 adapter 接口与映射 |
| [`mnemo-events`](https://github.com/jmeiracorbal/mnemo-events) | 发布者、控制器与存储共享的持久事件/命令契约 |

## 安装选项

| 方式 | 命令 |
|---|---|
| 当前 alpha (`v1.0.0-alpha.4`) | <code>curl -sSf https://raw.githubusercontent.com/jmeiracorbal/mnemo/main/install.sh &#124; MNEMO_VERSION=v1.0.0-alpha.4 bash</code> |
| 最新稳定版 | <code>curl -sSf https://raw.githubusercontent.com/jmeiracorbal/mnemo/main/install.sh &#124; bash</code> |
| 指定代理 | `bash -s -- --agent=codex` |
| 所有代理 | `bash -s -- --agent=all` |
| Claude plugin | `claude plugin install mnemo@mnemo` |
| 源码构建 | `go build -o ~/.local/bin/mnemo ./cmd/mnemo/` |

更新参数与卸载步骤：[Installation](https://github.com/jmeiracorbal/mnemo/wiki/Installation)。

## 文档

| 指南 | 内容 |
|---|---|
| [Wiki 首页](https://github.com/jmeiracorbal/mnemo/wiki) | 用户文档导航 |
| [安装](https://github.com/jmeiracorbal/mnemo/wiki/Installation) | 二进制、控制器、启用、更新 |
| [代理集成](https://github.com/jmeiracorbal/mnemo/wiki/Agent-Integrations) | 原生 ID、hooks、工具、Codex 信任 |
| [持久事件](https://github.com/jmeiracorbal/mnemo/wiki/Durable-Events) | 控制器架构与故障行为 |
| [CLI 参考](https://github.com/jmeiracorbal/mnemo/wiki/CLI-Reference) | 命令、MCP 工具、搜索模式 |
| [故障排查](https://github.com/jmeiracorbal/mnemo/wiki/Troubleshooting) | 诊断与控制器恢复 |
| [存储](https://github.com/jmeiracorbal/mnemo/wiki/Storage-and-Migrations) | SQLite、迁移、sqlc |
| [路线图](ROADMAP.md) | 计划中的产品与维护工作 |

## 设计原则

- **本地优先** — 记忆保存在你机器上的 SQLite 中
- **代理中立** — 所有支持的编程代理共享同一个记忆权威
- **按项目 opt-in** — 没有 `.mnemo` 的项目不会触发全局集成
- **可诊断** — 每个 setup surface 都可在不修改状态的情况下检查
- **可修复** — 重复项目身份与记忆冲突可通过 CLI 查看并修复

## 社区

如果 mnemo 改善了你的代理工作流，欢迎给仓库点 Star —— 这表示本地、代理中立的记忆值得继续建设。

- [Contributing](CONTRIBUTING.md) — 构建、测试与提交 PR
- [Code of Conduct](CODE_OF_CONDUCT.md) — 社区行为准则
- [Security](SECURITY.md) — 私密漏洞报告
- [Site](https://jmeiracorbal.github.io/mnemo/) · [Wiki](https://github.com/jmeiracorbal/mnemo/wiki) · [Roadmap](ROADMAP.md)

## 许可证

[Apache 2.0](LICENSE)：你可以自由使用、修改和分发，但必须保留版权声明，并在所有分发中包含 [NOTICE](NOTICE) 文件。
