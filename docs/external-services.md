# KeyIP-Intelligence 外部服务说明

> 本文档描述 KeyIP-Intelligence 平台依赖的全部外部基础设施服务。

---

## 总览

| 服务 | 端口 | 用途 | 必需 |
|------|------|------|------|
| PostgreSQL 16 | 5432 | 关系数据库（专利、分子、用户） | 是 |
| Neo4j 5 | 7687/7474 | 知识图谱（引用网络、专利族） | 否 |
| Redis 7 | 6379 | 缓存、会话、分布式锁 | 是 |
| OpenSearch 2 | 9200 | 全文搜索、聚合分析 | 是 |
| Milvus 2.4 | 19530 | 向量相似度搜索（分子指纹） | 否 |
| MinIO | 9002 | S3 对象存储（专利 PDF、报告） | 否 |
| Kafka 7.6 | 9092 | 事件流（专利更新、通知） | 否 |
| Keycloak 24 | 8180 | 统一认证、RBAC | 是 |
| MailHog | 1025/8025 | 开发邮件测试 | 否 |

---

## 1. PostgreSQL 16

**关系数据库**，存储所有业务数据：专利信息、分子元数据、组合数据、生命周期记录、用户账户。

```bash
docker run -d --name keyip-postgres \
  --restart=unless-stopped \
  -p 5432:5432 \
  -e POSTGRES_USER=keyip \
  -e POSTGRES_PASSWORD=keyip_dev \
  -e POSTGRES_DB=keyip_dev \
  -v keyip-postgres-data:/var/lib/postgresql/data \
  postgres:16-alpine
```

| 参数 | 值 |
|------|-----|
| Host | localhost:5432 |
| 用户 | keyip |
| 密码 | keyip_dev |
| 数据库 | keyip_dev |
| 健康检查 | `pg_isready -U keyip -d keyip_dev` |

**开发配置**（`configs/config.yaml`）：
```yaml
database:
  postgres:
    host: "localhost"
    port: 5432
    user: "keyip"
    password: "keyip_dev"
    dbname: "keyip_dev"
    sslmode: "disable"
    max_open_conns: 25
    max_idle_conns: 10
    conn_max_lifetime: 5m
```

---

## 2. Neo4j 5（社区版）

**图数据库**，用于专利引用网络、专利族关系、发明人协作网络、知识图谱遍历。

```bash
docker run -d --name keyip-neo4j \
  --restart=unless-stopped \
  -p 7687:7687 -p 7474:7474 \
  -e NEO4J_AUTH=neo4j/neo4j_dev \
  -e NEO4J_PLUGINS='["apoc"]' \
  -e NEO4J_dbms_memory_pagecache_size=512M \
  -e NEO4J_dbms_memory_heap_max__size=1G \
  -v keyip-neo4j-data:/data \
  -v keyip-neo4j-logs:/logs \
  neo4j:5-community
```

| 参数 | 值 |
|------|-----|
| Bolt | bolt://localhost:7687 |
| HTTP 浏览器 | http://localhost:7474 |
| 用户 | neo4j |
| 密码 | neo4j_dev |

**开发配置**：
```yaml
database:
  neo4j:
    uri: "bolt://localhost:7687"
    user: "neo4j"
    password: "neo4j_dev"
    max_connection_pool_size: 50
    connection_acquisition_timeout: 60s
```

---

## 3. Redis 7

**内存缓存**，用于会话存储、速率限制计数、分布式锁、热路径数据缓存（关闭持久化）。

```bash
docker run -d --name keyip-redis \
  --restart=unless-stopped \
  -p 6379:6379 \
  redis:7-alpine \
  redis-server --loglevel warning --save "" --appendonly no
```

| 参数 | 值 |
|------|-----|
| 地址 | localhost:6379 |
| 密码 | 无 |
| 数据库 | 0 |
| 持久化 | 关闭（仅缓存） |

**开发配置**：
```yaml
cache:
  redis:
    addr: "localhost:6379"
    password: ""
    db: 0
    pool_size: 20
    min_idle_conns: 5
    dial_timeout: 5s
    read_timeout: 3s
    write_timeout: 3s
```

---

## 4. OpenSearch 2.14

**全文搜索引擎**，用于专利标题、摘要、权利要求的 CJK 分析和 BM25 全文检索，支持混合搜索（BM25 + 向量）。

```bash
docker run -d --name keyip-opensearch \
  --restart=unless-stopped \
  -p 9200:9200 -p 9600:9600 \
  -e discovery.type=single-node \
  -e DISABLE_SECURITY_PLUGIN=true \
  -e DISABLE_INSTALL_DEMO_CONFIG=true \
  -e OPENSEARCH_JAVA_OPTS="-Xms512m -Xmx512m" \
  -e plugins.security.disabled=true \
  -v keyip-opensearch-data:/usr/share/opensearch/data \
  opensearchproject/opensearch:2.14.0

# 可选：可视化控制台
docker run -d --name keyip-os-dashboards \
  --restart=unless-stopped \
  -p 5601:5601 \
  -e "OPENSEARCH_HOSTS=[\"http://opensearch:9200\"]" \
  -e DISABLE_SECURITY_DASHBOARDS_PLUGIN=true \
  opensearchproject/opensearch-dashboards:2.14.0
```

| 参数 | 值 |
|------|-----|
| REST API | http://localhost:9200 |
| 安全 | 关闭（仅开发环境） |
| JVM 内存 | 512M |

**Linux 系统前置**：
```bash
sudo sysctl -w vm.max_map_count=262144
```

**开发配置**：
```yaml
search:
  opensearch:
    addresses:
      - "http://localhost:9200"
    username: "admin"
    password: "admin"
    max_retries: 3
    retry_on_status: [502, 503, 504]
    compress_request_body: true
```

---

## 5. Milvus 2.4（独立模式）

**向量数据库**，用于分子指纹（Morgan 2048 维、GNN embedding 256 维）的十亿级 ANN 搜索。

> Milvus 独立模式需要 etcd（元数据）和 MinIO（对象存储）两个内部依赖。

```bash
# 1. etcd（元数据存储）
docker run -d --name keyip-milvus-etcd \
  --restart=unless-stopped \
  -e ETCD_AUTO_COMPACTION_MODE=revision \
  -e ETCD_AUTO_COMPACTION_RETENTION=1000 \
  -e ETCD_QUOTA_BACKEND_BYTES=4294967296 \
  -v keyip-milvus-etcd-data:/etcd \
  quay.io/coreos/etcd:v3.5.14 \
  etcd --data-dir /etcd \
    --advertise-client-urls http://127.0.0.1:2379 \
    --listen-client-urls http://0.0.0.0:2379 \
    --log-level warn

# 2. milvus-minio（内部对象存储，端口 9000/9001）
docker run -d --name keyip-milvus-minio \
  --restart=unless-stopped \
  -p 9000:9000 -p 9001:9001 \
  -e MINIO_ROOT_USER=minioadmin \
  -e MINIO_ROOT_PASSWORD=minioadmin \
  -v keyip-milvus-minio-data:/minio-data \
  minio/minio:RELEASE.2024-08-17T01-24-54Z \
  minio server /minio-data --console-address ":9001"

# 3. Milvus
docker run -d --name keyip-milvus \
  --restart=unless-stopped \
  -p 19530:19530 \
  -e ETCD_ENDPOINTS=milvus-etcd:2379 \
  -e MINIO_ADDRESS=milvus-minio:9000 \
  -e MINIO_ACCESS_KEY=minioadmin \
  -e MINIO_SECRET_KEY=minioadmin \
  -e common__storage__type=minio \
  -v keyip-milvus-data:/var/lib/milvus \
  milvusdb/milvus:v2.4.6 \
  milvus run standalone
```

| 参数 | 值 |
|------|-----|
| gRPC | localhost:19530 |
| 索引类型 | IVF_SQ8 (Morgan), HNSW (GNN) |

**开发配置**：
```yaml
search:
  milvus:
    address: "localhost"
    port: 19530
    connect_timeout: 10s
    read_timeout: 10s
```

---

## 6. MinIO（独立实例）

**S3 兼容对象存储**，用于专利 PDF、分子结构文件、生成的报告。独立实例使用 9002 端口避免与 Milvus 内部 MinIO（9000）冲突。

```bash
docker run -d --name keyip-minio \
  --restart=unless-stopped \
  -p 9002:9000 -p 9003:9001 \
  -e MINIO_ROOT_USER=minioadmin \
  -e MINIO_ROOT_PASSWORD=minioadmin \
  -v keyip-minio-data:/data \
  minio/minio:RELEASE.2024-08-17T01-24-54Z \
  minio server /data --console-address ":9001"

# 初始化默认存储桶
docker run --rm --name keyip-minio-init \
  minio/mc:latest \
  /bin/sh -c "
    mc alias set local http://minio:9000 minioadmin minioadmin;
    mc mb local/keyip-documents --ignore-existing;
  "
```

| 参数 | 值 |
|------|-----|
| S3 API | localhost:9002 |
| 控制台 | http://localhost:9003 |
| Access Key | minioadmin |
| Secret Key | minioadmin |
| 默认 Bucket | keyip-documents |

**开发配置**：
```yaml
storage:
  minio:
    endpoint: "localhost:9002"
    access_key: "minioadmin"
    secret_key: "minioadmin"
    use_ssl: false
    bucket_name: "keyip-documents"
    region: "us-east-1"
```

---

## 7. Kafka 7.6（KRaft 模式）

**事件流平台**，用于异步处理专利监控、侵权检测、通知推送。使用 KRaft 模式（无需 ZooKeeper）。

```bash
docker run -d --name keyip-kafka \
  --restart=unless-stopped \
  -p 9092:9092 \
  -e CLUSTER_ID=keyip-dev-cluster \
  -e KAFKA_NODE_ID=1 \
  -e KAFKA_PROCESS_ROLES=broker,controller \
  -e KAFKA_CONTROLLER_QUORUM_VOTERS="1@kafka:29093" \
  -e KAFKA_LISTENERS=PLAINTEXT://0.0.0.0:9092,CONTROLLER://0.0.0.0:29093,PLAINTEXT_INTERNAL://0.0.0.0:29092 \
  -e KAFKA_ADVERTISED_LISTENERS=PLAINTEXT://localhost:9092,PLAINTEXT_INTERNAL://kafka:29092 \
  -e KAFKA_LISTENER_SECURITY_PROTOCOL_MAP=PLAINTEXT:PLAINTEXT,CONTROLLER:PLAINTEXT,PLAINTEXT_INTERNAL:PLAINTEXT \
  -e KAFKA_INTER_BROKER_LISTENER_NAME=PLAINTEXT_INTERNAL \
  -e KAFKA_CONTROLLER_LISTENER_NAMES=CONTROLLER \
  -e KAFKA_OFFSETS_TOPIC_REPLICATION_FACTOR=1 \
  -e KAFKA_TRANSACTION_STATE_LOG_MIN_ISR=1 \
  -e KAFKA_TRANSACTION_STATE_LOG_REPLICATION_FACTOR=1 \
  -e KAFKA_LOG_RETENTION_HOURS=24 \
  -v keyip-kafka-data:/var/lib/kafka/data \
  confluentinc/cp-kafka:7.6.1
```

| 参数 | 值 |
|------|-----|
| Broker | localhost:9092 |
| 消费者组 | keyip-dev-group |
| 日志保留 | 24 小时 |

**开发配置**：
```yaml
messaging:
  kafka:
    brokers:
      - "localhost:9092"
    consumer_group: "keyip-dev-group"
    auto_offset_reset: "earliest"
    session_timeout: 30s
    rebalance_timeout: 60s
```

---

## 8. Keycloak 24

**统一认证与授权**，提供 OAuth 2.0 / OIDC、RBAC、多租户 SSO。

> Keycloak 24 使用 8180 端口避免与 API 服务器（8080）冲突。

```bash
docker run -d --name keyip-keycloak \
  --restart=unless-stopped \
  -p 8180:8080 \
  -e KEYCLOAK_ADMIN=admin \
  -e KEYCLOAK_ADMIN_PASSWORD=admin \
  -e KC_HOSTNAME=localhost \
  -e KC_HTTP_ENABLED=true \
  quay.io/keycloak/keycloak:24.0 \
  start-dev
```

| 参数 | 值 |
|------|-----|
| 管理控制台 | http://localhost:8180/admin |
| Realm | keyip |
| 管理员 | admin / admin |

**开发配置**：
```yaml
auth:
  keycloak:
    base_url: "http://localhost:8180"
    realm: "keyip"
    client_id: "keyip-api"
    client_secret: "dev-secret"
```

### Keycloak 自动初始化

容器启动后，运行初始化脚本自动完成 realm、客户端、角色和测试用户的创建：

```bash
# 等待 Keycloak 完全就绪后执行初始化
./scripts/init-keycloak.sh

# 仅健康检查
./scripts/init-keycloak.sh --check-only

# 删除现有 realm 并重建
./scripts/init-keycloak.sh --recreate
```

**初始化内容**：

| 资源 | 名称 | 说明 |
|------|------|------|
| Realm | `keyip` | 平台统一认证域 |
| Client | `keyip-api` | OIDC 客户端（支持 authorization_code + client_credentials + password grants） |
| 角色 | `researcher` | 研究人员 — 专利检索与告警查看 |
| 角色 | `ip_manager` | IP 管理员 — 专利全生命周期管理 |
| 角色 | `executive` | 高管 — 报表与仪表板只读访问 |
| 角色 | `partner_agent` | 合作伙伴代理 — 受限范围的专利与分析读取 |
| 角色 | `super_admin` | 超级管理员 — 系统全部权限 |
| 用户 | `admin / admin` | 测试管理员（`super_admin` 角色） |
| 用户 | `researcher / researcher` | 测试研究人员（`researcher` 角色） |
| 用户 | `manager / manager` | 测试 IP 管理员（`ip_manager` 角色） |

**脚本说明**：
- 幂等设计，可重复执行不会产生重复资源
- 依赖 `curl` + `jq`，无需安装额外工具
- 通过环境变量覆写默认值：`KEYCLOAK_URL`, `KEYCLOAK_REALM`, `KEYCLOAK_CLIENT_ID`, `KEYCLOAK_CLIENT_SECRET`
- 角色定义与 `internal/infrastructure/auth/keycloak/rbac.go` 中的 `DefaultRolePermissionMapping()` 保持一致

**验证令牌获取**：
```bash
# 获取 admin 用户 access_token
curl -s -X POST http://localhost:8180/realms/keyip/protocol/openid-connect/token \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d 'client_id=keyip-api' \
  -d 'client_secret=dev-secret' \
  -d 'username=admin' -d 'password=admin' -d 'grant_type=password' | jq .
```

---

## 9. MailHog

## 9. MailHog

**邮件测试工具**，拦截应用发出的所有邮件，提供 Web UI 查看。

```bash
docker run -d --name keyip-mailhog \
  --restart=unless-stopped \
  -p 1025:1025 -p 8025:8025 \
  mailhog/mailhog:v1.0.1
```

| 参数 | 值 |
|------|-----|
| SMTP | localhost:1025 |
| Web UI | http://localhost:8025 |

---

## 快速启动

```bash
# 全部服务
./scripts/start-services.sh

# 最小集合（PostgreSQL + Redis + OpenSearch）
./scripts/start-services.sh minimal

# 单个服务
./scripts/start-services.sh postgres

# 查看状态
./scripts/start-services.sh status

# 停止并清理
./scripts/start-services.sh stop
```

## 许可

所有外部服务使用其各自的开源许可，与本项目（Apache 2.0）无关。
