# Smart Assistant / 智能助手

A local-first desktop AI assistant client built with Go + Wails v3 + React, supporting multi-model chat, multi-agent orchestration, workspaces, skills, memory, and execution tracing.

基于 Go + Wails v3 + React 构建的本地优先桌面 AI 助手客户端，支持多模型对话、多智能体编排、工作空间、技能市场、记忆管理和执行追踪。

## Features / 功能特性

- **Multi-Model Chat / 多模型对话** — Support for OpenAI, Anthropic, and other API-compatible LLM providers with model configuration management / 支持 OpenAI、Anthropic 及其他兼容 API 的大模型，提供模型配置管理
- **Multi-Agent Orchestration / 多智能体编排** — Serial multi-agent task dispatch and coordination (v1) / 多智能体串行任务调度与协作
- **Streaming Responses / 流式响应** — Real-time SSE streaming from LLM providers / 大模型实时 SSE 流式输出
- **Conversation History / 对话历史** — Full conversation session management with searchable history / 完整对话会话管理，支持历史回溯
- **Workspaces / 工作空间** — Isolated workspace contexts for organizing different projects or topics / 隔离的工作空间上下文，用于组织不同项目或主题
- **Skill Marketplace / 技能市场** — Install, uninstall, enable/disable skills to extend agent capabilities / 安装、卸载、启用、禁用技能，扩展智能体能力
- **Long-term Memory / 长期记忆** — Automatic memory extraction and selective context injection / 自动记忆提取与选择性上下文注入
- **Execution Tracing / 执行追踪** — JSON-based execution trace logging for debugging agent runs / 基于 JSON 的执行追踪日志，用于调试智能体运行
- **Token Statistics / Token 统计** — Track token usage across models and conversations / 跨模型和对话的 Token 用量统计
- **Human-in-the-Loop / 人工干预** — Cancel in-progress agent executions / 取消正在执行的智能体任务
- **Local First / 本地优先** — SQLite-based local storage; no cloud dependency required / 基于 SQLite 的本地存储，无需云端依赖

## Tech Stack / 技术栈

| Layer / 层级 | Technology / 技术 |
|-------|-----------|
| Desktop Framework / 桌面框架 | [Wails v3](https://wails.io) (Go + WebView2/WebKit) |
| Frontend / 前端 | React 18, TypeScript, Tailwind CSS 4 |
| Backend Engine / 后端引擎 | Go 1.24+, Chi router, WebSocket (JSON-RPC) |
| State Management / 状态管理 | Zustand |
| Database / 数据库 | SQLite (go-sqlite3) |
| Auth / 认证 | Local account + JWT / 本地账号 + JWT |
| Markdown Rendering / Markdown 渲染 | react-markdown + remark-gfm + react-syntax-highlighter |
| Document Preview / 文档预览 | docx-preview, xlsx |

## Project Structure / 项目结构

```
smart-assistant/
├── desktop/                    # Wails v3 桌面应用
│   ├── main.go                 # 应用入口
│   ├── app.go                  # Go↔JS 桥接绑定
│   ├── frontend/               # React + TypeScript 前端
│   │   └── src/
│   │       ├── components/     # UI 组件
│   │       │   ├── auth/       # 认证页面
│   │       │   ├── chat/       # 对话界面
│   │       │   ├── detail/     # 详情面板
│   │       │   ├── layout/     # 应用布局与导航
│   │       │   ├── memory/     # 记忆管理
│   │       │   ├── multiagent/ # 多智能体编排
│   │       │   ├── settings/   # 模型与应用设置
│   │       │   ├── skills/     # 技能市场
│   │       │   ├── stats/      # Token 用量统计
│   │       │   ├── tools/      # 工具配置
│   │       │   ├── traces/     # 执行追踪查看器
│   │       │   └── workspaces/ # 工作空间管理
│   │       ├── lib/            # 工具函数与 API 客户端
│   │       └── styles/         # 全局样式与设计 Token
│   ├── go.mod
│   └── wails.json
├── engine/                     # Go 后端引擎
│   ├── cmd/server/             # HTTP/SSE 服务入口
│   ├── internal/
│   │   ├── agent/              # 智能体核心循环与调度
│   │   ├── auth/               # 认证模块
│   │   ├── memory/             # 长期记忆
│   │   ├── model/              # 多厂商模型适配器
│   │   ├── session/            # 对话会话管理
│   │   ├── skill/              # 技能注册表与市场
│   │   ├── tool/               # 工具执行
│   │   ├── trace/              # 执行追踪
│   │   ├── transport/          # HTTP + WebSocket 处理器
│   │   └── workspace/          # 工作空间隔离
│   ├── pkg/
│   │   ├── server/             # 服务启动引导
│   │   └── store/              # SQLite 持久化与迁移
│   ├── api/openapi.yaml        # API 契约
│   ├── Dockerfile
│   └── docker-compose.yml
└── docs/
    └── artifacts/
        └── smart-assistant-client/
            ├── prd.md          # 产品需求文档
            ├── arch-design.md  # 架构设计文档
            └── delivery-plan.md # 交付计划
```

## Quick Start / 快速开始

### Prerequisites / 环境要求

- Go 1.24+
- Node.js 20+
- [Wails v3 CLI](https://wails.io/docs/gettingstarted/installation)

### Development / 开发

```bash
# 克隆仓库
git clone https://github.com/codingclouds/smart-assistant.git
cd smart-assistant

# 启动前端开发服务器
cd desktop/frontend
npm install
npm run dev

# 构建前端（嵌入模式）
npm run build

# 构建并运行桌面应用
cd ..
go mod tidy
wails dev
```

### Build / 构建

```bash
# 构建前端资源
cd desktop/frontend
npm run build

# 构建桌面二进制文件
cd ..
go build -o smart-assistant .
```

### Engine (standalone) / 引擎（独立运行）

```bash
cd engine
go run cmd/server/main.go
```

The engine exposes / 引擎暴露以下接口：
- HTTP REST API on `:8080`
- WebSocket JSON-RPC on `/ws`

## Architecture / 架构

The application follows a local-first desktop architecture / 应用采用本地优先的桌面架构：

1. **React Frontend / React 前端** — 在嵌入式 WebView 中渲染 UI
2. **Go Backend / Go 后端** — Wails 层 + 引擎处理业务逻辑、智能体循环和数据持久化
3. **SQLite** — 本地存储所有数据：对话、配置、Token 和追踪日志
4. **External LLM APIs / 外部大模型 API** — 引擎通过统一的 Provider 适配器直接调用

详见架构文档：[docs/artifacts/smart-assistant-client/arch-design.md](docs/artifacts/smart-assistant-client/arch-design.md)

## API

The engine exposes a WebSocket-based JSON-RPC API with method routing / 引擎暴露基于 WebSocket 的 JSON-RPC API，支持方法路由。详见 API 契约：[engine/api/openapi.yaml](engine/api/openapi.yaml)

## Roadmap / 路线图

### v0.1 (Current / 当前)
- [x] Desktop app (Windows/macOS) / 桌面应用（Windows/macOS）
- [x] Single-agent chat with streaming / 单智能体流式对话
- [x] Conversation history / 对话历史
- [x] Workspace isolation / 工作空间隔离
- [x] Multi-model management / 多模型管理
- [x] Token statistics / Token 统计
- [x] Memory & skills / 记忆与技能
- [x] Basic execution tracing / 基础执行追踪
- [x] Cancel-based human-in-the-loop / 取消式人工干预

### v2 (Planned / 计划中)
- [ ] Mobile clients (Android/iOS) / 移动端（Android/iOS）
- [ ] Expert voting panels (parallel agent review) / 专家投票评审（并行智能体评审）
- [ ] A2A/ACP protocol support / A2A/ACP 协议支持
- [ ] DAG-based trace visualization / 基于 DAG 的追踪可视化
- [ ] Full HITL intervention & queue management / 完整人工干预与队列管理

## License / 许可证

Private — All rights reserved. / 私有 — 保留所有权利。