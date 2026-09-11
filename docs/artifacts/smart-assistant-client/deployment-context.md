# Deployment Context: 智能助手客户端套件（Smart Assistant Client）

> **Slug**: `smart-assistant-client`
> **版本**: v0.1
> **环境**: 桌面端（单机部署）+ 引擎后端（Docker / 本地二进制）
> **主责角色**: `devops-engineer` / `tech-lead`
> **更新日期**: 2026-09-10

---

## 1. 环境清单

| 环境 | 用途 | 访问入口 | 部署目标 |
|------|------|---------|---------|
| 本地开发 | Go 引擎开发与前端联调 | `ws://localhost:8080/ws` | macOS / Linux 本地二进制 |
| 本地桌面 | Wails 桌面应用 + 内嵌前端 | Wails 启动后台自动拉起 Engine | macOS / Windows 桌面打包 |
| Docker 部署 | 私有化部署 Engine 后端服务 | `ws://<host>:8080/ws` | Docker Compose 单机 |

---

## 2. 部署入口

### 2.1 主入口：Wails 桌面应用（用户侧）

```bash
cd desktop
go build -o SmartAssistant .
# macOS: ./SmartAssistant
# Windows: SmartAssistant.exe
```

Wails 桌面应用启动时**自动拉起内嵌的 Engine 后端**，用户无需手动管理进程。
Engine 二进制随桌面应用一起打包，SQLite 数据文件存储于用户数据目录。

### 2.2 手工入口：Docker Compose（私有部署）

```bash
cd engine

# 构建镜像并启动
docker compose up -d --build

# 查看日志
docker compose logs -f engine

# 停止
docker compose down

# 带数据卷清理
docker compose down -v
```

**前置条件**：
- Docker Engine 24.0+
- Docker Compose v2
- 主机端口 8080 未被占用

### 2.3 手工入口：本地二进制（开发/调试）

```bash
cd engine

# 构建
go build -o bin/server ./cmd/server/

# 启动
ADDR=:8080 DB_PATH=data/smart-assistant.db ./bin/server
```

### 2.4 回退入口

| 场景 | 操作 |
|------|------|
| 新版本异常 | `docker compose down && docker compose up -d`（重置到旧镜像需先 `docker tag` 保留） |
| 数据损坏 | 停止服务 → 替换 `data/` 目录下 SQLite 备份 → 重启 |
| Docker 不可用 | 回退到本地二进制部署 |

---

## 3. 配置与密钥

### 3.1 环境变量

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `ADDR` | `:8080` | 引擎监听地址 |
| `DB_PATH` | `data/smart-assistant.db` | SQLite 数据库文件路径 |

### 3.2 模型 API 配置

模型配置（名称、Base URL、API Key）**不在环境变量中管理**，而是通过桌面应用 UI 的"模型配置"页面增删改，持久化到 SQLite `model_configs` 表。

- API Key 以明文存储于 SQLite（v1 简化方案）
- v2 规划：迁移到系统 Keychain / 环境变量注入

### 3.3 用户认证

- 本地账号体系：用户名 + bcrypt 密码哈希，存储于 SQLite `users` 表
- WebSocket 连接级认证：`auth.login` 成功后绑定 `user_id` 到连接上下文
- 无外部 OAuth / SSO 依赖

### 3.4 数据目录

| 路径 | 内容 | 备份策略 |
|------|------|---------|
| `data/smart-assistant.db` | SQLite 主数据库（含 WAL） | 每日定时 `sqlite3 .backup` 或直接复制 db 文件 |
| `data/smart-assistant.db-wal` | WAL 日志 | 随主文件一起备份 |
| `data/smart-assistant.db-shm` | 共享内存索引 | 无需备份 |

---

## 4. 运行保障

### 4.1 健康检查

```bash
# WebSocket 端点可达性
wscat -c ws://localhost:8080/ws
# 或
curl -i -N -H "Connection: Upgrade" -H "Upgrade: websocket" http://localhost:8080/ws
```

### 4.2 监控与日志

| 维度 | v0.1 方案 | v2 规划 |
|------|----------|---------|
| 应用日志 | `stdout` JSON 格式（Docker 通过 `docker logs`） | 结构化日志 + 日志聚合 |
| 错误追踪 | Go `log.Printf` 输出到 stderr | Sentry / 自建错误收集 |
| Token 用量 | 前端实时统计面板（`stats.tokens`） | 持久化 + 图表 |
| Trace 链路 | 按 Session 存储的 JSON trace，前端面板查看 | 导出到外部追踪系统 |
| 资源监控 | Docker `docker stats` | Prometheus + Grafana |

### 4.3 值守安排

| 时段 | 角色 | 响应时间 |
|------|------|---------|
| 工作时段 | 开发团队 | 4h |
| 非工作时段 | —（v0.1 无 SLA） | best-effort |

### 4.4 观察窗口

发布后观察 **24 小时**，关注：
- WebSocket 连接稳定性（重连频率）
- SQLite 数据库增长速率
- 模型 API 调用错误率
- 前端渲染异常（白屏、崩溃）

---

## 5. 恢复能力

### 5.1 回滚触发条件

| 条件 | 操作 |
|------|------|
| 引擎启动失败（3 次重试后） | 回滚到上一个稳定镜像 / 二进制 |
| WebSocket 连接成功率 < 90%（1h 窗口） | 回滚并排查 |
| 数据损坏（SQLite 文件无法打开） | 从最新备份恢复 |
| 安全漏洞（CVSS ≥ 7.0） | 立即回滚 + 密钥轮换 |

### 5.2 回滚路径

**Docker Compose 部署**：
```bash
# 1. 停止当前服务
docker compose down

# 2. 切换到上一个稳定镜像 tag
docker tag smart-assistant-engine:latest smart-assistant-engine:previous
# 编辑 docker-compose.yml 指定 image: smart-assistant-engine:previous

# 3. 重新启动
docker compose up -d
```

**本地二进制部署**：
```bash
# 替换为上一个版本的 bin/server 并重启
```

### 5.3 回滚验证

| 检查项 | 验证方式 |
|--------|---------|
| 引擎启动正常 | `docker compose ps` 状态为 healthy |
| WebSocket 可连接 | `wscat -c ws://localhost:8080/ws` |
| 认证正常 | 桌面应用可登录 |
| 对话可用 | 发送消息 → 流式响应正常 |
| 历史数据完整 | 会话列表和消息历史可见 |

---

## 6. 桌面端打包

### 6.1 macOS

```bash
cd desktop
wails build -platform darwin/universal
# 产出: desktop/build/bin/SmartAssistant.app
```

### 6.2 Windows

```bash
cd desktop
wails build -platform windows/amd64
# 产出: desktop/build/bin/SmartAssistant.exe
```

### 6.3 前端资源

前端构建产物已通过 Wails `embed.FS` 嵌入到 Go 二进制中，无需额外的静态文件部署。
若需独立开发时使用 Vite dev server：
```bash
cd desktop/frontend
npx vite --port 5173
# Wails 开发模式自动代理到 http://localhost:5173
```

---

## 7. 门禁状态

| Gate | 状态 |
|------|------|
| Pre-flight | ✅ 部署入口明确、配置项已识别 |
| Revision | 0 项 |
| Escalation | 0 项 |
| Abort | 无阻塞 |