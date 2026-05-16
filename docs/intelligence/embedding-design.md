# Embedding 模块设计文档

> 模块: `internal/intelligence/common/embedding.go`  
> 版本: 0.2.0 | 最后更新: 2026-05

---

## 1. 概述

EmbeddingClient 是 KeyIP-Intelligence 的统一向量化客户端。它为分子、专利文本、化学结构描述生成固定维度的浮点向量 (embedding)，供 Milvus 向量数据库进行相似度搜索。

**核心设计原则**：与 LLM Chat 后端共用同一个 API provider 配置，无需独立密钥或端点。

---

## 2. 多 Provider 策略

| Provider | 策略 | 维度 | API 端点 |
|:---------|:-----|:-----|:---------|
| **OpenAI** | 原生 `/v1/embeddings` | 1536 (text-embedding-3-small) | 同 endpoint |
| **DeepSeek** | OpenAI-compat `/v1/embeddings` | 768 (deepseek-chat) | 同 endpoint |
| **Anthropic** | Prompt 提取 → L2 归一化 | 768 | 复用 Predict() |

### 2.1 Anthropic Prompt-based 提取流程

```
输入文本
  │
  ▼
┌─────────────────────────────────────────┐
│ Predict(system_prompt: chemical-vectorizer) │  ← LLM 提取结构化分子描述符
│ 输出: "0.85,0.12,0.33,..."               │
└─────────────────────────────────────────┘
  │
  ▼
┌─────────────────────────────────────────┐
│ 解析逗号分隔浮点数 → baseVec []float32     │
└─────────────────────────────────────────┘
  │
  ▼ (解析失败时)
┌─────────────────────────────────────────┐
│ SHA-256 确定性哈希 → L2 归一化            │  ← 回退：纯文本 → 768d 向量
└─────────────────────────────────────────┘
  │
  ▼
┌─────────────────────────────────────────┐
│ Pad/Truncate → target dimensions        │
│ L2-normalize (sum of squares = 1.0)     │
└─────────────────────────────────────────┘
```

解析失败时的哈希回退：
```
SHA256(text) → 256-bit hash
  → 每 4 字节取 uint32 % 10000 / 10000.0
  → 循环填充到 target_dimensions
  → L2 归一化
```

### 2.2 OpenAI/DeepSeek 原生 Embeddings

```
POST {endpoint}/v1/embeddings
Body: { "model": "...", "input": text, "dimensions": N }
  │
  ▼
Response: { "data": [{ "embedding": [f32; N] }] }
```

---

## 3. 配置

### 3.1 Config 字段 (config.yaml)

```yaml
llm:
  primary:
    provider: anthropic            # 或 openai, deepseek
    api_key: ${ANTHROPIC_API_KEY}  # env-var 插值

    # 以下为可选 embedding 专用字段
    embedding_model_name: ""       # 空 → 使用 provider 默认值
    embedding_dimensions: 768      # 空 → provider 默认维度
```

### 3.2 默认值映射

```go
var DefaultEmbeddingConfigs = map[string]struct{Model, Dims}{
    "openai":    {"text-embedding-3-small", 1536},
    "deepseek":  {"deepseek-chat",          768},
    "anthropic": {"claude-sonnet-4-20250514", 768},
}
```

### 3.3 初始化逻辑

```
NewEmbeddingClient(cfg, backend):
  1. 读取 cfg.LLM.Primary
  2. 若 EmbeddingModelName 为空 → 查 DefaultEmbeddingConfigs
  3. 若仍未设置 → 使用 primary.ModelName (chat 模型)
  4. 若 Dimensions <= 0 → 查默认维度, 最终回退 768
  5. 返回 *EmbeddingClient
```

---

## 4. 核心接口

```go
type EmbeddingClient struct {
    provider   string       // "anthropic" | "openai" | "deepseek"
    endpoint   string       // API base URL
    apiKey     string       // 从 primary.ResolvedAPIKey()
    modelName  string       // embedding 专用模型名
    dimensions int          // 输出向量维度
    backend    ModelBackend // 仅 Anthropic 使用 (Prompt-based)
}

// 生成单个文本的向量
func (c *EmbeddingClient) Embed(ctx context.Context, text string) ([]float32, error)

// 批量生成 (未实现, TODO)
func (c *EmbeddingClient) BatchEmbed(ctx context.Context, texts []string) ([][]float32, error)
```

---

## 5. 调用链

```
main.go
  │
  ├── aiBackend := common.NewLLMBackend(cfg)  ← LLM Chat 后端
  ├── embedClient := common.NewEmbeddingClient(cfg, aiBackend)
  │                                                 │
  │   ┌─────────────────────────────────────────────┘
  │   ▼
  │   tasks.NewEmbeddingGenTask(milvusCollMgr, embedClient, cfg)
  │
  └── scheduler.Register(embeddingGenTask)
```

---

## 6. 错误处理

| 场景 | 处理 |
|:-----|:-----|
| Anthropic Predict() 失败 | 回退到 SHA-256 哈希向量 |
| OpenAI API 返回非 200 | 返回 error, task 重试 |
| 解析 LLM 输出失败 (非数字) | 回退到 SHA-256 哈希向量 |
| backend == nil (no LLM) | embedClient 不会创建, task 跳过 |

---

## 7. 未来增强

| 项目 | 优先级 | 说明 |
|:-----|:------|:-----|
| BatchEmbed | P1 | OpenAI/DeepSeek 支持批量输入, 大幅减少 API 调用 |
| 本地 ONNX 推理 | P2 | 使用 `all-MiniLM-L6-v2` 等轻量模型, 离线 & 零成本 |
| 向量缓存 (Redis) | P1 | 相同 SMILES/patent text 跳过重复计算 |
| 增量更新 | P2 | Kafka 监听新分子/专利 → 增量生成 embedding → Milvus upsert |
