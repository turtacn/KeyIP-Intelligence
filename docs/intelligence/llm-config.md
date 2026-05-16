# LLM Backend 与配置设计文档

> 模块: `internal/intelligence/common/llm_backend.go`, `internal/config/config.go`  
> 版本: 0.2.0 | 最后更新: 2026-05

---

## 1. 概述

LLM Backend 是 KeyIP-Intelligence 的统一 LLM 调用层。所有 AI 功能（报告生成、RAG 检索、分子分析、Embedding 生成）通过同一个配置入口接入 provider，支持热切换 primary/fallback 提供者。

**核心设计原则**：
- **配置驱动**：所有 LLM 参数通过 `config.yaml` 注入，零硬编码
- **多 Provider**：Anthropic / OpenAI / DeepSeek 统一接口
- **优雅降级**：primary 不可用时自动 fallback，全部不可用时 AI 功能静默禁用

---

## 2. 架构

```
┌─────────────────────────────────────────────────────────────┐
│                     application layer                       │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌───────────┐  │
│  │Reporting │  │RAG Engine│  │Molecule  │  │Embedding  │  │
│  │Service   │  │          │  │Analyzer  │  │GenTask    │  │
│  └────┬─────┘  └────┬─────┘  └────┬─────┘  └─────┬─────┘  │
└───────┼──────────────┼─────────────┼──────────────┼────────┘
        │              │             │              │
        ▼              ▼             ▼              ▼
┌─────────────────────────────────────────────────────────────┐
│                   intelligence/common                        │
│                                                             │
│  ┌──────────────────────────────────────────────────────┐   │
│  │                 ModelBackend (interface)              │   │
│  │  Predict(ctx, *PredictRequest) (*PredictResponse, error) │
│  └──────────────────────────────────────────────────────┘   │
│           ▲                     ▲                           │
│           │                     │                           │
│  ┌────────┴────────┐  ┌────────┴────────────┐              │
│  │ AnthropicBackend │  │ OpenAI-compat Backend│              │
│  │ (Messages API)   │  │ (Chat Completions)  │              │
│  └─────────────────┘  └─────────────────────┘              │
│                                                             │
│  ┌──────────────────────────────────────────────────────┐   │
│  │        EmbeddingClient (复用相同 provider)            │   │
│  │  Embed(ctx, text) ([]float32, error)                 │   │
│  └──────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────┘
        │              │             │              │
        ▼              ▼             ▼              ▼
   [Anthropic]    [OpenAI]     [DeepSeek]    (future providers)
```

---

## 3. ModelBackend 接口

```go
// ModelBackend is the unified interface for all LLM inference calls.
type ModelBackend interface {
    Predict(ctx context.Context, req *PredictRequest) (*PredictResponse, error)
}

type PredictRequest struct {
    ModelName   string
    Messages    []Message       // Chat messages (system, user, assistant)
    MaxTokens   int
    Temperature float64
    TopP        float64
}

type Message struct {
    Role    string // "system" | "user" | "assistant"
    Content string
}

type PredictResponse struct {
    Content      string
    StopReason   string
    InputTokens  int
    OutputTokens int
}
```

---

## 4. Provider 实现

### 4.1 AnthropicBackend

- **API**: Anthropic Messages API (`POST /v1/messages`)
- **认证**: `x-api-key` header (maps to `ANTHROPIC_API_KEY`)
- **模型**: `claude-sonnet-4-20250514` (default)
- **特点**: 通过系统提示词 (system) 进行角色约束

### 4.2 OpenAIBackend (OpenAI-compat)

- **API**: OpenAI Chat Completions (`POST /v1/chat/completions`)
- **认证**: `Authorization: Bearer {key}` header
- **兼容**: OpenAI, DeepSeek, 及任何 OpenAI-compatible API
- **特点**: 通过 `system` role message 进行角色约束

---

## 5. 配置规范

### 5.1 config.yaml 完整结构

```yaml
llm:
  primary:
    provider: anthropic                    # "anthropic" | "openai" | "deepseek"
    endpoint: ""                           # 空=使用默认端点
    api_key: ${ANTHROPIC_API_KEY}          # ${ENV_VAR} 插值支持
    api_key_env: ""                        # 或直接指定环境变量名
    model_name: claude-sonnet-4-20250514   # 空=使用 provider 默认
    max_tokens: 8192
    temperature: 0.7
    top_p: 1.0
    timeout_sec: 120
    retry_count: 3
    retry_delay_ms: 1000

    # Embedding 专用 (可选)
    embedding_model_name: ""               # 空=使用 provider 默认
    embedding_dimensions: 768              # 空=使用 provider 默认

  fallback:
    provider: deepseek                     # primary 失败时使用
    api_key: ${DEEPSEEK_API_KEY}
    model_name: deepseek-chat
    max_tokens: 8192
    temperature: 0.7
    timeout_sec: 120
```

### 5.2 环境变量

| 变量 | Provider | 用途 |
|:-----|:---------|:-----|
| `ANTHROPIC_API_KEY` | Anthropic | API 认证 |
| `OPENAI_API_KEY` | OpenAI | API 认证 |
| `DEEPSEEK_API_KEY` | DeepSeek | API 认证 |
| `KEYIP_JWT_SECRET` | (Auth) | JWT 签名密钥 |

### 5.3 默认端点

| Provider | 默认 Base URL |
|:---------|:-------------|
| Anthropic | `https://api.anthropic.com/v1` |
| OpenAI | `https://api.openai.com/v1` |
| DeepSeek | `https://api.deepseek.com/v1` |

---

## 6. 初始化流程

```
NewLLMBackend(cfg):
  1. 读取 cfg.LLM.Primary
  2. 若未配置 provider → 默认 "anthropic"
  3. 解析 API key:
     a. primary.ResolvedAPIKey() → api_key (支持 ${ENV_VAR})
     b. 若仍为空 → os.Getenv("{PROVIDER}_API_KEY")
  4. 填充 endpoint, model name, max_tokens, temperature, timeout
  5. 根据 provider 创建具体 Backend 实例
  6. 返回 ModelBackend + error
```

---

## 7. 降级策略

```
┌──────────────┐
│  调用方      │
│ (handler/svc)│
└──────┬───────┘
       │
       ▼
┌──────────────┐   success?
│  Primary     │───────► 返回结果
│  (Anthropic) │
└──────┬───────┘
       │ error
       ▼
┌──────────────┐   success?
│  Fallback    │───────► 返回结果
│  (DeepSeek)  │
└──────┬───────┘
       │ error
       ▼
┌──────────────┐
│  返回 error  │ → AI 功能静默降级 (不阻塞 API 响应)
└──────────────┘
```

---

## 8. EmbeddingClient 复用

`EmbeddingClient` 共享 LLM Backend 的 API 配置：

| 场景 | 行为 |
|:-----|:-----|
| provider = openai | 调用 `POST /v1/embeddings` (原生) |
| provider = deepseek | 调用 `POST /v1/embeddings` (OpenAI-compat) |
| provider = anthropic | 通过 `backend.Predict()` 进行 prompt-based 向量提取 |

详见 [Embedding 模块设计文档](embedding-design.md)。

---

## 9. 安全约束

| 约束 | 实现 |
|:-----|:-----|
| API Key 不落地日志 | `ResolvedAPIKey()` 返回后仅用于 HTTP header |
| env-var 插值 | `${ENV_VAR}` 语法, 不在 yaml 中硬编码密钥 |
| 请求超时 | `context.WithTimeout` + `timeout_sec` 配置 |
| 重试幂等 | `retry_count` / `retry_delay_ms` 控制 |
