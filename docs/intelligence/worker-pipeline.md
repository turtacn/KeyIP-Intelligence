# Worker Scheduler & Data Pipeline 设计文档

> 模块: `internal/worker/`, `internal/worker/tasks/`, `internal/infrastructure/datasource/`  
> 版本: 0.2.0 | 最后更新: 2026-05

---

## 1. 概述

Worker Scheduler 是 KeyIP-Intelligence 的后台任务调度引擎，负责将多个中间件（PostgreSQL、Neo4j、OpenSearch、Milvus）的数据自动流转，消除数据孤岛。

**核心目标**：确保外部数据源 → PostgreSQL → OpenSearch 索引 → Neo4j 图 → Milvus 向量的单向管道持续运行。

---

## 2. 数据流转管道

```
                       ┌─────────────────────────────────────────┐
                       │         External Data Sources            │
                       │  PubChem · EPO OPS · USPTO · CNIPA      │
                       │  (adapter pattern, plug-in on API key)   │
                       └──────────────┬──────────────────────────┘
                                      │
                        ┌─────────────▼─────────────┐
                        │    DataSource Registry     │
                        │  (datasource.NewRegistry)  │
                        └─────────────┬─────────────┘
                                      │
              ┌───────────────────────┼───────────────────────┐
              │                       │                       │
              ▼                       ▼                       ▼
   ┌──────────────────┐   ┌──────────────────┐   ┌──────────────────┐
   │ PatentSyncTask   │   │ MoleculeSyncTask │   │ (future:         │
   │ every 6h         │   │ every 12h        │   │  LegislationTask) │
   └────────┬─────────┘   └────────┬─────────┘   └──────────────────┘
            │                      │
            ▼                      ▼
   ┌──────────────────────────────────────────────────────────────┐
   │                    PostgreSQL (source of truth)               │
   │  molecules · patents · portfolios · lifecycle · users        │
   └──────────────┬───────────────┬───────────────┬───────────────┘
                  │               │               │
       ┌──────────▼─────┐  ┌──────▼──────┐  ┌─────▼──────────┐
       │ IndexRefresh   │  │ GraphBuild  │  │ EmbeddingGen   │
       │ Task           │  │ Task        │  │ Task           │
       │ daily @ 02:00  │  │ daily@03:00 │  │ daily @ 04:00  │
       └──────┬─────────┘  └──────┬──────┘  └─────┬──────────┘
              │                   │                │
              ▼                   ▼                ▼
   ┌──────────────┐   ┌──────────────┐   ┌──────────────┐
   │  OpenSearch  │   │    Neo4j     │   │   Milvus     │
   │ full-text    │   │   knowledge  │   │   vector     │
   │ index        │   │   graph      │   │   search     │
   └──────────────┘   └──────────────┘   └──────────────┘
```

---

## 3. 组件详解

### 3.1 Scheduler (`internal/worker/scheduler.go`)

```go
type Scheduler struct {
    tasks   []Task
    cron    *cron.Cron
    mu      sync.RWMutex
}

func NewScheduler() *Scheduler
func (s *Scheduler) Register(task Task)
func (s *Scheduler) Start(ctx context.Context)
func (s *Scheduler) Stop()
func (s *Scheduler) Tasks() []Task
```

- 基于 [robfig/cron](https://github.com/robfig/cron) 的 cron 表达式调度
- 所有 task 注册后统一 `Start()`
- `Stop()` 发送 shutdown 信号，等待当前任务完成

### 3.2 Task 接口

```go
type Task interface {
    Name() string            // 任务名称 (日志 & 监控用)
    CronSchedule() string    // cron 表达式
    Run(ctx context.Context) error
}
```

---

## 4. Task 清单

### 4.1 PatentSyncTask

| 字段 | 值 |
|:-----|:---|
| 文件 | `internal/worker/tasks/patent_sync_task.go` |
| 调度 | `0 */6 * * *` (每 6 小时) |
| 数据流 | DataSource Registry → PostgreSQL |

```go
func NewPatentSyncTask(
    registry *datasource.Registry,  // 外部数据源注册表
    patentRepo PatentRepo,          // PostgreSQL 写入
    kafkaProducer *kafka.Producer,  // 变更事件流
) *PatentSyncTask
```

### 4.2 MoleculeSyncTask

| 字段 | 值 |
|:-----|:---|
| 文件 | `internal/worker/tasks/molecule_sync_task.go` |
| 调度 | `0 */12 * * *` (每 12 小时) |
| 数据流 | DataSource Registry → PostgreSQL |

```go
func NewMoleculeSyncTask(
    registry *datasource.Registry,
    moleculeRepo MoleculeRepo,
    kafkaProducer *kafka.Producer,
) *MoleculeSyncTask
```

### 4.3 IndexRefreshTask

| 字段 | 值 |
|:-----|:---|
| 文件 | `internal/worker/tasks/index_refresh_task.go` |
| 调度 | `0 2 * * *` (每天凌晨 2 点) |
| 数据流 | PostgreSQL → OpenSearch 全文索引 |

```go
func NewIndexRefreshTask(indexer *search_os.Indexer) *IndexRefreshTask
```

执行内容：
1. 从 PostgreSQL 读取自上次索引以来的新增/变更的专利和分子
2. 构建 OpenSearch bulk index payload
3. 提交 bulk 请求
4. 记录最后同步时间戳

### 4.4 GraphBuildTask

| 字段 | 值 |
|:-----|:---|
| 文件 | `internal/worker/tasks/graph_build_task.go` |
| 调度 | `0 3 * * *` (每天凌晨 3 点) |
| 数据流 | PostgreSQL → Neo4j 知识图谱 |

```go
func NewGraphBuildTask(
    neo4jDriver neo4j.Driver,
    kgRepo KnowledgeGraphRepository,
) *GraphBuildTask
```

执行内容：
1. 从 PostgreSQL 读取专利-分子关联 (patent_molecule 表)
2. 在 Neo4j 中创建/更新 `(:Patent)-[:CONTAINS]->(:Molecule)` 关系
3. 创建专利族 (family) 和引用 (cites/cited_by) 关系
4. 移除已删除的节点和关系

### 4.5 EmbeddingGenTask

| 字段 | 值 |
|:-----|:---|
| 文件 | `internal/worker/tasks/embedding_gen_task.go` |
| 调度 | `0 4 * * *` (每天凌晨 4 点) |
| 数据流 | PostgreSQL → EmbeddingClient → Milvus |

```go
func NewEmbeddingGenTask(
    milvusCollMgr  *search_milvus.CollectionManager,
    embedClient    *common.EmbeddingClient,
    cfg            *config.Config,
) *EmbeddingGenTask
```

执行内容：
1. Ensure Milvus collection 存在 (patent_embeddings / molecule_embeddings)
2. 从 PostgreSQL 读取未被向量化的专利/分子
3. 对每条文本调用 `embedClient.Embed()`
4. 批量写入 Milvus (upsert)
5. 创建/刷新 Milvus 索引 (FLAT / IVF_FLAT / HNSW)

---

## 5. DataSource Registry (`internal/infrastructure/datasource/`)

```go
type DataSource interface {
    Name() string
    FetchPatents(ctx context.Context, since time.Time) ([]Patent, error)
    FetchMolecules(ctx context.Context, since time.Time) ([]Molecule, error)
}

type Registry struct {
    sources map[string]DataSource
}

func NewRegistry() *Registry
func (r *Registry) Register(name string, ds DataSource)
func (r *Registry) Get(name string) (DataSource, bool)
```

当前状态：Registry 已初始化但**无已注册的外部数据源**。待 API keys 就绪后，添加：
- `PubChemDataSource` — 分子元数据批量导入
- `EPOOPSDataSource` — 欧洲专利局 OPS API
- `CNIPADataSource` — 中国专利局 API
- `USPTODataSource` — 美国专利局 API

---

## 6. main.go 接线

```go
// Worker Scheduler — background data sync & middleware refresh

// 1. DataSource Registry
dsRegistry := datasource.NewRegistry()

// 2. Neo4j Knowledge Graph repository
var kgRepo neo4j_repos.KnowledgeGraphRepository
if neo4jDriver != nil {
    kgRepo = neo4j_repos.NewNeo4jKnowledgeGraphRepo(neo4jDriver, logger)
}

// 3. OpenSearch Indexer
var osIndexer *search_os.Indexer
if osClient != nil {
    osIndexer = search_os.NewIndexer(osClient, search_os.IndexerConfig{}, logger)
}

// 4. Milvus Collection Manager
var milvusCollMgr *search_milvus.CollectionManager
if milvusClient != nil {
    milvusCollMgr = search_milvus.NewCollectionManager(milvusClient, ...)
}

// 4a. EmbeddingClient
var embedClient *common.EmbeddingClient
if aiBackend != nil && cfg != nil {
    embedClient = common.NewEmbeddingClient(cfg, aiBackend)
}

// 5. Schedule tasks
scheduler := worker.NewScheduler()
scheduler.Register(tasks.NewPatentSyncTask(dsRegistry, patentRepo, kafkaProducer))
scheduler.Register(tasks.NewMoleculeSyncTask(dsRegistry, moleculeRepo, kafkaProducer))
scheduler.Register(tasks.NewIndexRefreshTask(osIndexer))       // if osIndexer != nil
scheduler.Register(tasks.NewGraphBuildTask(neo4jDriver, kgRepo)) // if neo4j != nil
scheduler.Register(tasks.NewEmbeddingGenTask(milvusCollMgr, embedClient, cfg)) // if milvus != nil
scheduler.Start(context.Background())
```

---

## 7. 优雅降级矩阵

| 中间件不可用 | 影响 |
|:------------|:-----|
| Kafka | PatentSync/MoleculeSync 跳过事件发布，同步仍进行 |
| OpenSearch | IndexRefreshTask 不注册，全文搜索回退到 PostgreSQL LIKE |
| Neo4j | GraphBuildTask 不注册，知识图谱页面显示 fallback 数据 |
| Milvus | EmbeddingGenTask 不注册，向量搜索不可用 |
| LLM Backend | EmbeddingGenTask 注册但跳过 embedding 生成 |
| 所有 DataSource 为空 | SyncTask 运行但无数据源可拉取 (waiting for API keys) |

---

## 8. 监控

- 每次 task 执行记录开始/结束时间到日志
- 执行失败时记录错误详情和重试次数
- 未来：Prometheus metrics — `keyip_worker_tasks_total`, `keyip_worker_task_duration_seconds`

---

## 9. 未来增强

| 项目 | 优先级 | 说明 |
|:-----|:------|:-----|
| Kafka 增量更新 | P1 | 新专利/分子 → Kafka → OpenSearch/Milvus 增量更新 (非全量) |
| PubChem DataSource | P1 | 实现 `DataSource` 接口，分子元数据自动入库 |
| EPO OPS DataSource | P2 | 欧洲专利局专利数据拉取 |
| Task 执行历史 (PostgreSQL) | P2 | `worker_task_logs` 表，记录成功/失败/耗时 |
| 死信队列 (DLQ) | P2 | 连续失败 N 次的 task 进入 DLQ 告警 |
