# Smart Assistant

A local-first desktop AI assistant client built with Go + Wails v3 + React, supporting multi-model chat, multi-agent orchestration, workspaces, skills, memory, and execution tracing.

## Features

- **Multi-Model Chat** — Support for OpenAI, Anthropic, and other API-compatible LLM providers with model configuration management
- **Multi-Agent Orchestration** — Serial multi-agent task dispatch and coordination (v1)
- **Streaming Responses** — Real-time SSE streaming from LLM providers
- **Conversation History** — Full conversation session management with searchable history
- **Workspaces** — Isolated workspace contexts for organizing different projects or topics
- **Skill Marketplace** — Install, uninstall, enable/disable skills to extend agent capabilities
- **Long-term Memory** — Automatic memory extraction and selective context injection
- **Execution Tracing** — JSON-based execution trace logging for debugging agent runs
- **Token Statistics** — Track token usage across models and conversations
- **Human-in-the-Loop** — Cancel in-progress agent executions
- **Local First** — SQLite-based local storage; no cloud dependency required

## Tech Stack

| Layer | Technology |
|-------|-----------|
| Desktop Framework | [Wails v3](https://wails.io) (Go + WebView2/WebKit) |
| Frontend | React 18, TypeScript, Tailwind CSS 4 |
| Backend Engine | Go 1.24+, Chi router, WebSocket (JSON-RPC) |
| State Management | Zustand |
| Database | SQLite (go-sqlite3) |
| Auth | Local account + JWT |
| Markdown Rendering | react-markdown + remark-gfm + react-syntax-highlighter |
| Document Preview | docx-preview, xlsx |

## Project Structure

```
smart-assistant/
├── desktop/                    # Wails v3 desktop application
│   ├── main.go                 # Application entry point
│   ├── app.go                  # Go↔JS bridge bindings
│   ├── frontend/               # React + TypeScript frontend
│   │   └── src/
│   │       ├── components/     # UI components
│   │       │   ├── auth/       # Authentication views
│   │       │   ├── chat/       # Chat interface
│   │       │   ├── detail/     # Detail panels
│   │       │   ├── layout/     # App layout & navigation
│   │       │   ├── memory/     # Memory management
│   │       │   ├── multiagent/ # Multi-agent orchestration
│   │       │   ├── settings/   # Model & app settings
│   │       │   ├── skills/     # Skill marketplace
│   │       │   ├── stats/      # Token usage statistics
│   │       │   ├── tools/      # Tool configuration
│   │       │   ├── traces/     # Execution trace viewer
│   │       │   └── workspaces/ # Workspace management
│   │       ├── lib/            # Utilities & API client
│   │       └── styles/         # Global styles & design tokens
│   ├── go.mod
│   └── wails.json
├── engine/                     # Go backend engine
│   ├── cmd/server/             # HTTP/SSE server entry point
│   ├── internal/
│   │   ├── agent/              # Agent core loop & dispatch
│   │   ├── auth/               # Authentication
│   │   ├── memory/             # Long-term memory
│   │   ├── model/              # Multi-provider model adapter
│   │   ├── session/            # Conversation session management
│   │   ├── skill/              # Skill registry & marketplace
│   │   ├── tool/               # Tool execution
│   │   ├── trace/              # Execution tracing
│   │   ├── transport/          # HTTP + WebSocket handlers
│   │   └── workspace/          # Workspace isolation
│   ├── pkg/
│   │   ├── server/             # Server bootstrap
│   │   └── store/              # SQLite persistence & migrations
│   ├── api/openapi.yaml        # API contract
│   ├── Dockerfile
│   └── docker-compose.yml
└── docs/
    └── artifacts/
        └── smart-assistant-client/
            ├── prd.md
            ├── arch-design.md
            └── delivery-plan.md
```

## Quick Start

### Prerequisites

- Go 1.24+
- Node.js 20+
- [Wails v3 CLI](https://wails.io/docs/gettingstarted/installation)

### Development

```bash
# Clone the repository
git clone https://github.com/codingclouds/smart-assistant.git
cd smart-assistant

# Start the frontend dev server
cd desktop/frontend
npm install
npm run dev

# Build the frontend (for embedded mode)
npm run build

# Build and run the desktop app
cd ..
go mod tidy
wails dev
```

### Build

```bash
# Build frontend assets
cd desktop/frontend
npm run build

# Build the desktop binary
cd ..
go build -o smart-assistant .
```

### Engine (standalone)

```bash
cd engine
go run cmd/server/main.go
```

The engine exposes:
- HTTP REST API on `:8080`
- WebSocket JSON-RPC on `/ws`

## Architecture

The application follows a local-first desktop architecture:

1. **React Frontend** renders the UI in an embedded WebView
2. **Go Backend** (Wails layer + engine) handles business logic, agent loops, and data persistence
3. **SQLite** stores all data locally — conversations, configurations, tokens, and traces
4. **External LLM APIs** are called directly from the engine via unified provider adapters

See [docs/artifacts/smart-assistant-client/arch-design.md](docs/artifacts/smart-assistant-client/arch-design.md) for detailed architecture documentation.

## API

The engine exposes a WebSocket-based JSON-RPC API with method routing. See [engine/api/openapi.yaml](engine/api/openapi.yaml) for the full API contract.

## Roadmap

### v0.1 (Current)
- [x] Desktop app (Windows/macOS)
- [x] Single-agent chat with streaming
- [x] Conversation history
- [x] Workspace isolation
- [x] Multi-model management
- [x] Token statistics
- [x] Memory & skills
- [x] Basic execution tracing
- [x] Cancel-based human-in-the-loop

### v2 (Planned)
- [ ] Mobile clients (Android/iOS)
- [ ] Expert voting panels (parallel agent review)
- [ ] A2A/ACP protocol support
- [ ] DAG-based trace visualization
- [ ] Full HITL intervention & queue management

## License

Private — All rights reserved.