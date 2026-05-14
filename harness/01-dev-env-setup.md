# KeyIP-Intelligence 开发环境搭建与数据导入 SOP

> **受众**: 后端开发、前端开发、QA 测试  
> **前置条件**: Docker Desktop / OrbStack ≥ 4.x, Go ≥ 1.22, Node.js ≥ 20, Python ≥ 3.11  
> **适用场景**: 首次搭建本地环境、中间件数据丢失后重建、全新机器上初始化

---

## 目录

1. [架构概览](#1-架构概览)
2. [基础设施一键启动](#2-基础设施一键启动)
3. [数据导入（完整流程）](#3-数据导入完整流程)
4. [数据持久化策略](#4-数据持久化策略)
5. [应用启动](#5-应用启动)
6. [前端本地热重载开发](#6-前端本地热重载开发)
7. [故障排查清单](#7-故障排查清单)
8. [日常开发工作流](#8-日常开发工作流)
9. [附录：数据源参照表](#9-附录数据源参照表)

---

## 1. 架构概览

```
┌──────────────────────────────────────────────────────────────────┐
│  localhost:80  ── KeyIP Web (nginx → React SPA)                 │
│  localhost:8080 ── KeyIP API Server (Go)                        │
│  localhost:9090 ── KeyIP gRPC Server                            │
│  localhost:9091 ── Prometheus Metrics                           │
└──────────────┬───────────────────────────────────────────────────┘
               │
    ┌──────────┼──────────┬──────────┬──────────┬──────────┬──────────┐
    ▼          ▼          ▼          ▼          ▼          ▼          ▼
┌─────────┐┌─────────┐┌─────────┐┌─────────┐┌─────────┐┌─────────┐┌─────────┐
│PostgreSQL││  Neo4j  ││OpenSearch││ Milvus  ││  Redis  ││  Kafka  ││  MinIO  │
│  pg16   ││  5.x    ││  2.14   ││  2.4    ││  7.x    ││  7.6    ││ 2024-08 │
│ :5432   ││ :7474   ││ :9200   ││ :19530  ││ :6379   ││ :9092   ││ :9000   │
│(relational)│(graph) ││(fulltext)││(vector) ││(cache)  ││(queue)  ││(object) │
└─────────┘└─────────┘└─────────┘└─────────┘└─────────┘└─────────┘└─────────┘
```

---

## 2. 基础设施一键启动

### 2.1 启动所有中间件容器

```bash
# 在项目根目录执行
docker compose -f deployments/docker/docker-compose.yml up -d --wait
```

该命令会启动 **12 个容器**：

| 容器名 | 服务 | 端口 | 健康检查 |
|--------|------|------|:------:|
| `keyip-postgres` | PostgreSQL 16 + pgvector | 5432 | ✅ |
| `keyip-redis` | Redis 7 (Alpine) | 6379 | ✅ |
| `keyip-neo4j` | Neo4j 5 Community | 7474 / 7687 | — |
| `keyip-opensearch` | OpenSearch 2.14 | 9200 | ✅ |
| `keyip-os-dashboards` | OpenSearch Dashboards | 5601 | — |
| `keyip-milvus` | Milvus 2.4 Standalone | 19530 / 9091 | ✅ |
| `keyip-milvus-etcd` | etcd (Milvus 元数据) | — | ✅ |
| `keyip-milvus-minio` | MinIO (Milvus 存储) | — | ✅ |
| `keyip-minio` | MinIO (业务存储) | 9000 / 9001 | ✅ |
| `keyip-minio-init` | MinIO 初始化 (mc) | — | 一次性 |
| `keyip-kafka` | Kafka (KRaft) | 9092 | ✅ |
| `keyip-mailhog` | MailHog (邮件捕获) | 8025 | ✅ |

**容器也内建了 apiserver + web 服务**（从 Dockerfile 构建）。

### 2.2 常用管理命令

```bash
# 查看所有容器状态
docker compose -f deployments/docker/docker-compose.yml ps

# 查看某服务日志
docker compose -f deployments/docker/docker-compose.yml logs -f postgres
docker compose -f deployments/docker/docker-compose.yml logs -f opensearch

# 停止所有服务（保留数据 volumes）
docker compose -f deployments/docker/docker-compose.yml down

# 完全销毁（删除数据 volumes）
docker compose -f deployments/docker/docker-compose.yml down -v

# 重启某服务
docker compose -f deployments/docker/docker-compose.yml restart postgres
```

### 2.3 最小化启动（节省资源）

如果仅需 PostgreSQL + Redis 进行基本开发：

```bash
docker compose -f deployments/docker/docker-compose.yml up -d postgres redis
```

完整功能需要全部服务，否则搜索 / 知识图谱 / 向量相似度功能跳过。

---

## 3. 数据导入（完整流程）

### 3.1 数据库迁移

```bash
# 方式一：Makefile（推荐）
make migrate-up

# 方式二：Docker 运行 migrate 工具
docker run --rm \
  --network keyip-network \
  -v "$(pwd)/internal/infrastructure/database/postgres/migrations:/migrations" \
  migrate/migrate:v4.17.0 \
  -path=/migrations \
  -database="postgres://keyip:keyip_dev@keyip-postgres:5432/keyip_dev?sslmode=disable" \
  up
```

### 3.2 导入所有种子数据

```bash
# 一键全量导入（PostgreSQL + OpenSearch + Milvus + Neo4j + MinIO）
./scripts/import/seed_all.sh

# 跳过某些服务（如无 GPU 跳过 Milvus）
./scripts/import/seed_all.sh --skip-milvus

# 仅导入 PostgreSQL 数据
./scripts/import/seed_pg.sh
```

### 3.3 验证数据完整性

```bash
./scripts/import/verify_data.sh
```

预期输出示例：

```
organizations:    2
users:            6
patents:         14
molecules:       15
portfolios:       3
valuations:       6
deadlines:        9
lifecycle_events: 11
annuities:       17
cost_records:    14
```

### 3.4 按数据源导入脚本参照

| 脚本 | 作用 | 依赖 |
|------|------|------|
| `scripts/import/seed_pg.sh` | 运行迁移 001-008，PG 全表写入 | PostgreSQL running |
| `scripts/import/seed_opensearch.sh` | 创建索引 + 批量索引文档 | PG 已 seed |
| `scripts/import/seed_milvus.sh` | 创建 collection + 插入向量 | Milvus running, PG 已 seed |
| `scripts/import/seed_neo4j.sh` | Cypher 加载图谱节点与关系 | Neo4j running, PG 已 seed |
| `scripts/import/seed_minio.sh` | 创建 bucket + 上传样本文件 | MinIO running |
| `scripts/import/reset_db.sh` | 清空所有表 + 重建 + 重新 seed | ⚠️ 销毁性操作 |

### 3.5 种子数据实体总览

| 实体类别 | 数量 | 说明 |
|----------|:--:|------|
| 组织 (Organizations) | 2 | OLED Material Tech (CN), Luminara Materials (US) |
| 用户 (Users) | 6 | admin / IP Manager / Researcher / VP / Patent Agent / Partner |
| 角色 (Roles) | 5 | super_admin, org_admin, patent_analyst, researcher, viewer |
| 专利 (Patents) | 14 | CN/US/EP/JP/KR — 含 granted/examination/expired/revoked |
| 分子 (Molecules) | 15 | OLED 发光材料: CBP, mCP, Ir(ppy)₃, DMAC-TRZ, 4CzIPN, Alq₃ 等 |
| 组合 (Portfolios) | 3 | Blue Emitter Core, HTL, Licensing Revenue |
| 估值 (Valuations) | 6 | S/A/B/D tier with monetary values |
| 截止日 (Deadlines) | 9 | critical/high/medium priority |
| 生命周期事件 | 11 | filing → grant → revocation 全流程 |
| 年金 (Annuities) | 17 | CNY/USD/EUR/JPY multi-currency |
| 费用记录 (Costs) | 24+ | Filing, prosecution, annuity, translation costs |

---

## 4. 数据持久化策略

### 4.1 容器数据生命周期

| 操作 | 数据保留？ |
|------|:--:|
| `docker compose stop` + `start` | ✅ 全部保留 |
| `docker compose down` + `up` | ✅ 全部保留 |
| `docker compose restart <service>` | ✅ 全部保留 |
| `docker compose down -v` | ❌ 全部销毁 |
| Docker Desktop "Reset to factory defaults" | ❌ 全部销毁 |
| 磁盘故障 / 宿主机重装 | ❌ 全部销毁 |

### 4.2 数据恢复流程

数据丢失后，**无需备份**，直接从代码仓库重建：

```bash
# 1. 确保中间件运行
docker compose -f deployments/docker/docker-compose.yml up -d --wait

# 2. 完整重建数据库（迁移 + 种子数据）
./scripts/import/reset_db.sh

# 3. 导入搜索 / 向量 / 图谱 / 对象存储
./scripts/import/seed_all.sh

# 4. 验证
./scripts/import/verify_data.sh
```

**核心原则**：代码仓库即是 Source of Truth。所有种子数据都在以下位置：

```
项目根目录
├── internal/infrastructure/database/postgres/migrations/008_seed_data.sql   (64KB 全量 SQL)
├── test/testdata/fixtures/     (JSON fixtures — E2E 测试 / Playwright 用)
└── scripts/import/             (导入脚本)
```

### 4.3 Docker Volumes 与持久化

容器使用 **Docker named volumes**，数据存储在 Docker 管理的宿主磁盘上，不随容器删除而丢失（除非加 `-v` flag）。

关键 volumes：

| Volume 名 | 挂载到容器路径 | 内容 |
|-----------|---------------|------|
| `postgres-data` | `/var/lib/postgresql/data` | 所有关系数据 |
| `neo4j-data` | `/data` | 图谱数据 |
| `opensearch-data` | `/usr/share/opensearch/data` | 全文索引 |
| `milvus-data` | `/var/lib/milvus` | 向量集合 |
| `milvus-minio-data` | `/minio-data` | Milvus 内部存储 |
| `milvus-etcd-data` | `/etcd` | Milvus 元数据 |
| `kafka-data` | `/var/lib/kafka/data` | 消息队列数据 |
| `minio-data` | `/data` | 业务对象存储 |

---

## 5. 应用启动

### 5.1 启动 API Server (Go)

```bash
# 方式一：编译 + 运行
make build-apiserver
./bin/apiserver

# 方式二：go run
go run ./cmd/apiserver/

# 方式三：Docker 内运行
docker compose -f deployments/docker/docker-compose.yml up -d apiserver
```

API Server 启动后监听：
- `http://localhost:8080` — REST API
- `http://localhost:9090` — gRPC
- `http://localhost:9091` — Prometheus metrics

### 5.2 启动 Worker (Go)

```bash
make build-worker && ./bin/worker
# 或
go run ./cmd/worker/
```

### 5.3 验证 API 可访问

```bash
# 健康检查
curl http://localhost:8080/healthz

# 详细健康信息
curl http://localhost:8080/healthz/detail

# 分子查询
curl http://localhost:8080/api/v1/molecules

# 专利查询
curl http://localhost:8080/api/v1/patents
```

---

## 6. 前端本地热重载开发

```bash
cd web

# 安装依赖
npm install

# 启动 Vite 开发服务器（HMR 热更新）
npm run dev
# 默认 http://localhost:5173

# 生产构建
npm run build
# 输出到 web/dist/
```

### 前端连接 API

前端开发默认以 **proxy 模式** 运行，`/api/v1/*` 请求经由 nginx 转发到 `keyip-apiserver:8080`（Docker 内部 DNS）。

API 模式在浏览器 localStorage 中存储（`keyip-api-mode`）：

| 模式 | 值 | API 目标 | 使用场景 |
|------|----|----------|---------|
| Mock | `mock` | `/api/v1` → nginx 本地 stub 响应 | 🔴 已废弃 |
| Proxy | `proxy` | `/api/v1` → nginx → `keyip-apiserver:8080` | **Docker 环境 (默认)** |
| Live | `live` | `https://api.keyip.io/api/v1` | 生产联调 |

#### Nginx API Stubs（可选 Mock 模式）

nginx 内建了一套 API stub（硬编码 JSON 响应）。通过环境变量控制：

| `NGINX_USE_STUBS` | 行为 |
|:---:|---|
| `true` | nginx 返回 stub JSON，**不访问 apiserver**。适合纯前端 UI 调试 |
| (unset / false) | **默认**：所有 `/api/` 请求 `proxy_pass` 到 apiserver，真实后端验证 |

```bash
# Docker Compose 中启用 stubs（仅 UI 调试）
# 编辑 docker-compose.yml 中 web 服务的 environment:
#   NGINX_USE_STUBS: "true"

# docker-machine 真实验证：不设置此变量（默认 proxy_pass 到 apiserver）
```

> ⚠️ **重要**：stubs 模式下 `location = /api/v1/...` 精确匹配优先级高于 `location /api/` 的 proxy_pass。
> 关掉 stubs 后所有请求直达 apiserver，可用于真实 JWT 登录 + DB 数据验证。

### 本地 Python Dev Server（绕过 nginx）

```bash
# 先构建前端
cd web && npm run build

# 启动简化 dev server
cd ..
python3 dev-server.py
# 访问 http://localhost:3001
# /api/* 代理到 http://192.168.99.100:8080
```

### WebStorm / VS Code 推荐的调试配置

`.vscode/launch.json` 示例：

```json
{
  "version": "0.2.0",
  "configurations": [
    {
      "name": "Launch API Server",
      "type": "go",
      "request": "launch",
      "mode": "debug",
      "program": "${workspaceFolder}/cmd/apiserver",
      "env": { "CONFIG_FILE": "${workspaceFolder}/configs/config.yaml" }
    },
    {
      "name": "Launch Chrome (Frontend)",
      "type": "chrome",
      "request": "launch",
      "url": "http://localhost:5173",
      "webRoot": "${workspaceFolder}/web/src"
    }
  ]
}
```

---

## 7. 故障排查清单

### 7.1 Docker 服务不可达

```bash
# 检查网络
docker network inspect keyip-network

# 检查某个端口是否监听
nc -z localhost 5432 && echo "PG OK" || echo "PG DOWN"
nc -z localhost 9200 && echo "OS OK" || echo "OS DOWN"
```

### 7.2 迁移失败

```bash
# 检查迁移版本状态
docker run --rm \
  --network keyip-network \
  -v "$(pwd)/internal/infrastructure/database/postgres/migrations:/migrations" \
  migrate/migrate:v4.17.0 \
  -path=/migrations \
  -database="postgres://keyip:keyip_dev@keyip-postgres:5432/keyip_dev?sslmode=disable" \
  version

# 强制回退
make migrate-down-all  # 回退全部迁移
make migrate-up        # 重新执行
```

### 7.3 种子数据为 0

```bash
# 直接连接 PG 查询
PGPASSWORD=keyip_dev psql -h localhost -U keyip -d keyip_dev \
  -c "SELECT COUNT(*) FROM patents;"

# 重新执行 seed
./scripts/import/reset_db.sh
```

### 7.4 Java 线程创建失败 (旧版 Docker)

所有中间件容器已配置 `seccomp:unconfined`，修复旧版 Docker 下 JVM 线程创建限制。

### 7.5 OpenSearch 内存不足

```bash
# 编辑 docker-compose.yml 临时降低堆内存
# OPENSEARCH_JAVA_OPTS: "-Xms512m -Xmx512m"
```

---

## 8. 日常开发工作流

### 8.1 从零到可开发（< 5 分钟）

```bash
# Step 1: 启动中间件
docker compose -f deployments/docker/docker-compose.yml up -d --wait

# Step 2: 导入数据（如果首次或数据丢失）
make migrate-up && make seed

# Step 3: 构建 + 启动 API Server
make build-apiserver && ./bin/apiserver &

# Step 4: 启动前端
cd web && npm run dev
```

### 8.2 快速迭代（Docker 模式）

```bash
# 每次代码变更后
make build-apiserver && docker compose -f deployments/docker/docker-compose.yml up -d --build apiserver

# 前端变更（Docker 模式）
cd web && npm run build && docker compose -f deployments/docker/docker-compose.yml up -d --build web
```

### 8.3 完整重置

```bash
# 彻底清空一切
docker compose -f deployments/docker/docker-compose.yml down -v
docker compose -f deployments/docker/docker-compose.yml up -d --wait
make migrate-up && make seed
```

### 8.4 .gitignore 已排除

```
data/                  # 本地数据目录
test/testdata/generated/  # 自动生成的测试数据
```

---

## 9. 附录：数据源参照表

| # | 数据源 | 端口 | 类型 | 账户/密码 | Seeds |
|---|--------|:---:|------|-----------|:---:|
| 1 | PostgreSQL 16 + pgvector | 5432 | 关系数据库 | `keyip` / `keyip_dev` | ✅ |
| 2 | Neo4j 5 Community | 7474/7687 | 图数据库 | `neo4j` / `neo4j_dev` | ✅ |
| 3 | OpenSearch 2.14 | 9200 | 全文搜索 | `admin` / `admin` | ✅ |
| 4 | Milvus 2.4 | 19530/9091 | 向量搜索 | — | ✅ |
| 5 | Redis 7 Alpine | 6379 | 缓存+会话 | — | — |
| 6 | Kafka 7.6 | 9092 | 消息队列 | — | — |
| 7 | MinIO 2024-08 | 9000/9001 | 对象存储 | `minioadmin` / `minioadmin` | ✅ |
| 8 | MailHog | 8025 | 邮件捕获 | — | — |
| 9 | OpenSearch Dashboards | 5601 | 管理面板 | — | — |
