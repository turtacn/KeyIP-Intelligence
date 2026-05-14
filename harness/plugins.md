# KeyIP-Intelligence Claude Code 插件分析报告

> 日期: 2026-05-14
> 项目: KeyIP-Intelligence (Go 1.22 + React/TypeScript)
> 仓库: github.com/turtacn/KeyIP-Intelligence

---

## 1. 项目技术栈分析

| 层级 | 技术 | 文件量 |
|------|------|--------|
| 后端 | Go 1.22 (gin, gRPC, cobra) | ~200+ .go 文件 |
| 前端 | React 18.3 + TypeScript + Vite + Tailwind CSS | ~150+ .ts/.tsx 文件 |
| 数据库 | PostgreSQL(pgvector), Neo4j, Redis, OpenSearch, Milvus | 多服务 |
| 消息 | Kafka (confluent 7.6.1) | 异步事件 |
| 存储 | MinIO (S3 兼容) | 对象存储 |
| 容器 | Docker Compose (12 服务), Kubernetes (Kustomize + Helm) | 编排 |
| 测试 | go test, Vitest, Playwright | 三层测试 |
| CI/CD | GitHub Actions (ci.yml + release.yml) | 自动化 |

---

## 2. 推荐插件清单

### 2.1 语言服务器 (LSP) — 代码智能

| 插件 | 用途 | 优先级 | 状态 |
|------|------|:------:|:----:|
| `gopls-lsp` | Go 代码智能：跳转到定义、查找引用、错误检查、重构 | 🔴 必须 | ✅ |
| `typescript-lsp` | TypeScript/JS 代码智能：跳转定义、引用查找、类型错误 | 🔴 必须 | ✅ |

**理由**: 项目 80% 代码为 Go + TypeScript，LSP 是 Claude Code 进行代码修改的基础能力。
没有 LSP，Claude Code 无法准确理解类型系统、符号引用和上下文语义。

### 2.2 工作流自动化 — 提升开发效率

| 插件 | 用途 | 优先级 | 状态 |
|------|------|:------:|:----:|
| `commit-commands` | 简化 git 工作流：/commit、/push、/pr 等快捷命令 | 🟡 推荐 | ✅ |
| `code-review` | 多代理商自动代码审查，可信度评分过滤误报 | 🟡 推荐 | ✅ |
| `pr-review-toolkit` | 全面 PR 审查：测试覆盖、错误处理、类型设计、代码质量 | 🟡 推荐 | ✅ |
| `hookify` | 自定义钩子，防止不良行为模式；根据对话模式生成钩子 | 🟡 推荐 | ✅ |  
| `feature-dev` | 结构化功能开发流程：探索→设计→实现→审查 | 🟢 可选 | ✅ |

**理由**:
- **commit-commands**: 项目有严格的提交规范（见 CONTRIBUTING.md），commit-commands 可以标准化提交流程
- **code-review**: 项目代码库庞大 (~200+ Go files)，自动审查可以减轻人工 review 负担
- **pr-review-toolkit**: GitHub Actions CI 已包含 lint/test/security，PR toolkit 在此基础上增加语义级审查
- **hookify**: 可以设置防止在 main 分支直接修改、防止跳过 pre-commit hooks 等规则
- **feature-dev**: 对于大型新功能开发提供结构化流程

### 2.3 领域专用 — 针对性增强

| 插件 | 用途 | 优先级 | 状态 |
|------|------|:------:|:----:|
| `frontend-design` | 生成独特的、非模板化的生产级前端界面 | 🟡 推荐 | ✅ |
| `security-guidance` | 安全审计：OWASP Top 10、Go 安全最佳实践 | 🟡 推荐 | ✅ |

**理由**:
- **frontend-design**: 前端使用 Tailwind CSS + React，frontend-design 帮助避免 AI 生成通用的 "AI 审美" UI
- **security-guidance**: 项目处理专利数据，安全合规至关重要（Go 安全、API 认证、数据加密）。CI 中已有 gosec + govulncheck，此插件提供对话级安全指导

### 2.4 外部集成 (MCP 服务器)

| 插件/MCP | 用途 | 优先级 | 状态 |
|------|------|:------:|:----:|
| `github` MCP | GitHub PR/Issue 管理、代码搜索、仓库操作 | 🔴 必须 | ✅ |
| `playwright` MCP | 浏览器自动化，配合已有 Playwright E2E 测试 | 🟢 可选 | ⬜ (已配置，默认禁用) |

**理由**:
- **github MCP**: 项目托管在 GitHub，CI 使用 GitHub Actions。github MCP 可以让 Claude Code 直接管理系统 PR、Issue、搜索代码
- **playwright MCP**: 项目已有 11 个 Playwright 测试规范，MCP 可为调试和编写测试提供浏览器自动化能力

---

## 3. 已安装的插件 (全部 11 个)

| 插件 | 版本 | 来源 |
|------|------|------|
| `claude-mem@thedotmack` | 13.0.0 | thedotmack/claude-mem |
| `gopls-lsp` | 1.0.0 | claude-plugins-official |
| `typescript-lsp` | 1.0.0 | claude-plugins-official |
| `github` | 1a2f18b | claude-plugins-official |
| `code-review` | 1a2f18b | claude-plugins-official |
| `commit-commands` | 1a2f18b | claude-plugins-official |
| `pr-review-toolkit` | 1a2f18b | claude-plugins-official |
| `hookify` | 1a2f18b | claude-plugins-official |
| `frontend-design` | 1a2f18b | claude-plugins-official |
| `security-guidance` | 1a2f18b | claude-plugins-official |
| `feature-dev` | 1a2f18b | claude-plugins-official |

### 能力矩阵

| 能力 | 提供插件 |
|------|----------|
| 跨会话记忆、时间线、知识库 | claude-mem |
| Go 代码智能 (跳转、引用、重构) | gopls-lsp + gopls v0.15.3 |
| TypeScript 代码智能 (跳转、引用、类型) | typescript-lsp |
| GitHub PR/Issue 管理 | github MCP |
| 自动化代码审查 | code-review + pr-review-toolkit |
| 标准化 git 工作流 | commit-commands |
| 自定义行为钩子 | hookify |
| 前端界面设计 | frontend-design |
| 安全审计指导 | security-guidance |
| 结构化功能开发 | feature-dev |

---

## 4. 不需要的插件（排除说明）

| 插件 | 排除理由 |
|------|----------|
| `agent-sdk-dev` | 当前项目不开发 Anthropic SDK 应用 |
| `claude-md-management` | 项目只有一个 CLAUDE.md，不需要管理工具 |
| `code-modernization` | 项目为新项目，无遗留代码需迁移 |
| `code-simplifier` | 会与 CLAUDE.md 中 "只改必要代码" 原则冲突 |
| `ralph-loop` | 不需要循环执行 |
| `learning-output-style` | 不需要改变输出风格 |
| `claude-code-setup` | 已完成初始化 |
| `cwc-makers` | 不开发 MCP 服务器 |
| `mcp-server-dev` | 不开发自定义 MCP 服务 |
| `math-olympiad` | 不相关 |
| `playground` | 不相关 |
| `skill-creator` | 不需要创建新 skill |
| `laravel-boost` | PHP 框架，不相关 |
| `terraform` | 项目未使用 Terraform |
| `firebase` | 未使用 Firebase |
| `linear` | 未使用 Linear |
| `asana` | 未使用 Asana |
| `gitlab` | 托管在 GitHub，非 GitLab |
| 各 LSP (csharp, java, kotlin, lua, php, python, ruby, rust, swift, clangd, pyright) | 项目不使用这些语言 |

---

## 5. 安装计划 (全部完成 ✅)

### 第一步: 安装必须插件 (3 个) — ✅ 已完成
```
claude plugins install gopls-lsp        ✅
claude plugins install typescript-lsp    ✅
claude plugins install github           ✅ (MCP, 需设置 GITHUB_PERSONAL_ACCESS_TOKEN)
```

### 第二步: 安装推荐插件 (6 个) — ✅ 已完成
```
claude plugins install code-review       ✅
claude plugins install commit-commands   ✅
claude plugins install pr-review-toolkit ✅
claude plugins install hookify           ✅
claude plugins install frontend-design   ✅
claude plugins install security-guidance ✅
```

### 第三步: 安装可选插件 (1 个) — ✅ 已完成
```
claude plugins install feature-dev       ✅
```

---

## 6. 环境变量要求

| 变量 | 用途 | 必需 |
|------|------|:----:|
| `GITHUB_PERSONAL_ACCESS_TOKEN` | GitHub MCP 连接 GitHub API | ✅ |
| `KEYIP_ENV` | 项目环境标识 | 推荐 |
| `KEYIP_CONFIG` | 配置文件路径 | 推荐 |

---

## 7. 验证清单

- [x] `gopls` v0.15.3 二进制在 PATH 中 (`/go/bin/gopls`)
- [x] 11 个插件全部安装并启用
- [x] 项目 settings.json 已部署到 `/workspace/.claude/settings.json`
- [x] MCP 配置已部署到 `/workspace/.mcp.json`
- [ ] GitHub MCP 连接 — 需手动设置 `GITHUB_PERSONAL_ACCESS_TOKEN` 环境变量
- [ ] typescript LSP 需下次会话启动后生效
- [ ] commit-commands 的 `/commit` 需下次会话启动后生效
- [ ] frontend-design 在设计任务时自动激活
