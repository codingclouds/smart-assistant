# Execute Log: 智能助手客户端套件

> **Slug**: `smart-assistant-client`
> **主责角色**: `architect`（P1，执行期间以开发角色实施）
> **日期**: 2026-09-10
> **里程碑**: M0 引擎骨架 + M1 部分（桌面脚手架）

---

## 1. 计划 vs 实际

| 工作项 | 计划 | 实际 | 偏差说明 |
|--------|------|------|---------|
| W01 Go 项目初始化 | 1 人天 | 已完成（半天） | 创建议程内完成 |
| W02 SQLite schema | 1 人天 | 已完成 | 7 表 + 1 索引，含 models 表补充 |
| W03 Agent 引擎骨架 | 3 人天 | 已完成（核心循环 + 流式 SSE） | loop.go ~180 行 |
| W04 Model provider 抽象层 | 2 人天 | 已完成 | OpenAI-compatible provider |
| W05 SSE streaming endpoint | 2 人天 | 已完成 | SSE 端到端验证通过 |
| W06 认证模块 | 2 人天 | 已完成 | JWT + bcrypt |
| W07 Wails 项目脚手架 | 1 人天 | 已完成 | React + TypeScript + Tailwind + Vite |
| W08 Chat UI | 3 人天 | 已完成（初版） | 完整对话界面含 login |

**汇总**: M0 核心项全部完成，M1 前端部分提前完成。比计划进度快 ~2 天。

---

## 2. 关键决定

1. **Wails v3 脚手架超时**：`go mod tidy` 因 goproxy.cn 代理在 `wails3 init` 执行期间未被导入导致。手动在项目目录下重新执行 `GOPROXY=https://goproxy.cn,direct go mod tidy` 解决。
2. **Engine 与 Desktop 分拆为两个独立 Go module**：Engine 作为独立 HTTP 服务，Desktop 通过 Wails WebView 加载前端，前端通过 HTTP API 调用 Engine。开发阶段前端 Vite 代理 `/api` 到 Engine（:8080），生产阶段 Desktop 内置前端静态文件并启动 Engine 子进程。
3. **记忆模块 v1 为骨架**：M3 阶段完整实现 LLM 摘要提取和向量检索。
4. **SSE token 事件编码方案**：token 事件直接发送文本内容（不封装 JSON），其他事件（meta/done/error）使用 JSON 编码，保证前端逐字渲染流畅且结构化事件可解析。

---

## 3. 阻塞与解决

| 阻塞 | 根因 | 解决 |
|------|------|------|
| `wails3 init` 超时失败 | Go proxy 默认使用 proxy.golang.org 超时 | 手动 `GOPROXY=https://goproxy.cn,direct go mod tidy` |
| Chat API 500: FOREIGN KEY constraint failed | workspace_id 为空时使用硬编码 "default" | 新增 `workspace.GetByUser()` 查询用户默认空间 |
| Chat API 500: no such table: models | Schema 迁移缺少 models 表 | 补充 models 表到 migrate.go |
| TypeScript strict unused variable errors | 未使用的 import 和变量 | 逐个清理 |

---

## 4. 影响面

### 已创建模块
- `engine/`：Go module，HTTP API 服务（14 个端点）
- `desktop/`：Wails v3 桌面应用 + React 前端
- `docs/artifacts/smart-assistant-client/`：PRD、Delivery Plan、Arch Design

### 数据库表（SQLite）
users, workspaces, sessions, messages, memories, skills, traces, token_stats, models

---

## 5. 未完成项

| 项 | 原因 | 计划 |
|----|------|------|
| 多 Agent 编排（dispatch） | M3 里程碑 | v1 接口已定义，实现延后 |
| 记忆提取与注入 | M3 里程碑 | 骨架已就位 |
| Skill 市场仓库拉取 | M3 里程碑 | HTTP API 接口已就位 |
| Trace DAG 可视化 | v2 推迟 | JSON 文本日志已实现 |
| Win/Mac 打包 | M4 里程碑 | Wails build 已验证可编译 |
| 私有部署文档 | M4 里程碑 | 待编写 |

---

## 6. 当前构建验证

```
✅ Go Engine:    go build ./cmd/server  (15MB)
✅ Desktop App:  go build ./desktop     (22MB, 嵌入式引擎)  
✅ Frontend:     npm run build          (98KB JS + 4KB CSS gzip)
✅ Auth API:     POST /api/auth/register + /api/auth/login → JWT 200
✅ Models API:   POST /api/models + GET /api/models → 200/201
✅ SSE Chat:     POST /api/chat → event:meta → event:token → event:done
✅ Workspaces:   GET /api/workspaces → 200 [{id, name}]
✅ Sessions:     GET /api/sessions → 200 []
✅ Token Stats:  GET /api/stats/tokens → 200 []
✅ Skills:       GET /api/skills → 200 []
```

---

> **关联 Artifact**: 交付计划见 `delivery-plan.md`，架构设计见 `arch-design.md`。