# WideMinds MCP

WideMinds MCP 是一个围绕“大模型 + 思维扩散”构建的探索式知识导航引擎。后端采用 Go 实现会话管理、思维路径生成与 MCP 工具接口，前端提供可视化思维导图与交互画布，帮助用户从一个概念快速拓展出多条思维路径并逐步深入。

## 功能亮点

- **思维扩散引擎**：集成 LLM 调度器，为给定概念生成多维扩散方向与深入节点。
- **会话管理**：支持创建、查询与持久化用户思维会话，自动统计节点数量、深度与方向分布。
- **MCP 工具集成**：内置 `expand_thought`、`explore_direction`、`create_session`、`get_session` 四种工具，可通过 HTTP 接口调用。
- **前端可视化**：包含思维树与动画画布，实时展示节点路径、支持节点高亮、缩放与拖拽。

## 快速开始

### 先决条件

- Go 1.22 及以上
- Node.js (可选，仅用于前端构建/扩展)

### 克隆仓库

```powershell
# Windows PowerShell
cd <your-workspace>
git clone <repo-url>
cd WideMindsMCP
```

### 配置环境

1. 复制并修改环境变量示例：

   ```powershell
   copy configs\example.env .env
   # 根据需要编辑 .env 填写 LLM_API_KEY 等信息
   ```

2. 校验配置文件 `configs/config.yaml`，可调整监听端口、存储目录等参数。

### 安装依赖

```powershell
Set-Location WideMindsMCP
go mod tidy
```

### 运行服务

```powershell
Set-Location WideMindsMCP
# 启动后端 HTTP + MCP 服务
go run ./cmd/server
```

启动后：
- Web 前端默认监听 `http://localhost:8080`
- MCP 接口监听 `http://localhost:9090`

### 可视化界面

访问 `http://localhost:8080`，输入关键词（例如“机器学习”）即可生成扩散方向、查看思维树与互动画布。点击“深入探索”可以使选定方向继续扩展下游节点。

### API 端点

- `POST /api/sessions`：创建会话 `{ "user_id": "u1", "concept": "机器学习" }`
- `GET /api/sessions/{id}`：获取会话详情
- `POST /api/sessions/{id}`：在会话中继续探索 `{ "direction": {...} }`
- `POST /api/expand`：直接获取扩散建议
- `POST /mcp`：调用 MCP 工具，JSON 体 `{"method": "expand_thought", "params": {...}}`
- `GET /tools`：查看已注册的 MCP 工具

## 测试与质量

```powershell
Set-Location WideMindsMCP
# 代码格式化（go fmt 已在提交前执行）
 gofmt -w ./cmd ./internal
# 运行单元测试
go test ./...
```

测试覆盖核心模型、会话管理与存储逻辑，确保思维路径和元数据均能正确维护。

## 项目结构

- `cmd/server`：服务入口，负责加载配置、初始化依赖、启动 HTTP/MCP 服务
- `internal/models`：领域模型（Thought、Session、Direction）
- `internal/services`：业务逻辑层（ThoughtExpander、LLMOrchestrator、SessionManager）
- `internal/storage`：会话持久化（内存版 + 文件版）
- `internal/mcp`：MCP Server 与工具实现
- `web/`：前端资源，含思维树与交互画布 JS
- `configs/`：配置文件与 env 示例

## 后续规划

- 接入真实的 LLM API，通过 `LLMOrchestrator.CallLLM` 调用远程模型。
- 扩展前端节点编辑能力（拖拽连接、节点备注、导出格式等）。
- 引入持久化数据库与多用户会话隔离策略。
- 增强可视化性能与布局算法（力导向/层次布局）。

欢迎提交 Issue 或 PR，共同完善 WideMinds 思维导航体验！
