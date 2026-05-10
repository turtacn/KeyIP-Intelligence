#!/usr/bin/env bash
# ==============================================================================
# KeyIP-Intelligence — 外部服务 Docker 启动脚本
# ==============================================================================
# 用法:
#   ./scripts/start-services.sh              # 启动全部服务
#   ./scripts/start-services.sh postgres     # 仅启动 PostgreSQL
#   ./scripts/start-services.sh minimal       # 最小集合 (postgres+redis+opensearch)
#   ./scripts/start-services.sh stop         # 停止并清理全部容器
# ==============================================================================

set -euo pipefail

NETWORK="keyip-network"
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

# ---- docker-machine 检测 ----
IS_DOCKER_MACHINE=false
if docker info 2>/dev/null | grep -q "docker-machine\|boot2docker"; then
  IS_DOCKER_MACHINE=true
fi

# ---- 公共参数 ----
LOGGING="--log-driver=json-file --log-opt max-size=10m --log-opt max-file=3"
RESTART="--restart=unless-stopped"

# 低内存模式（docker-machine 默认 VM 仅 1-2G 内存）
if $IS_DOCKER_MACHINE; then
  _warn "检测到 docker-machine，使用低内存配置"
  NEO4J_HEAP_INIT="128M"
  NEO4J_HEAP_MAX="256M"
  NEO4J_PAGECACHE="128M"
  OS_MEM="256m"
  MILVUS_MEM="256m"
else
  NEO4J_HEAP_INIT="512M"
  NEO4J_HEAP_MAX="1G"
  NEO4J_PAGECACHE="512M"
  OS_MEM="512m"
  MILVUS_MEM="512m"
fi

_log()  { echo -e "${GREEN}[INFO]${NC}  $*"; }
_warn() { echo -e "${YELLOW}[WARN]${NC}  $*"; }
_fail() { echo -e "${RED}[FAIL]${NC}  $*"; exit 1; }

# ---- 创建共享网络 ----
create_network() {
  if ! docker network inspect "$NETWORK" >/dev/null 2>&1; then
    _log "创建 Docker 网络: $NETWORK"
    docker network create "$NETWORK"
  fi
}

# =============================================================================
# 服务定义
# =============================================================================

start_postgres() {
  _log "启动 PostgreSQL 16"
  docker run -d --name keyip-postgres \
    $RESTART $LOGGING --network "$NETWORK" \
    -p 5432:5432 \
    -e POSTGRES_USER=keyip \
    -e POSTGRES_PASSWORD=keyip_dev \
    -e POSTGRES_DB=keyip_dev \
    -v keyip-postgres-data:/var/lib/postgresql/data \
    postgres:16-alpine
}

start_neo4j() {
  _log "启动 Neo4j 5 (Bolt: 7687, HTTP: 7474)"
  docker run -d --name keyip-neo4j \
    $RESTART $LOGGING --network "$NETWORK" \
    -p 7687:7687 -p 7474:7474 \
    -e NEO4J_AUTH=neo4j/neo4j_dev \
    -e NEO4J_PLUGINS='["apoc"]' \
    -e NEO4J_dbms_memory_heap_initial__size=$NEO4J_HEAP_INIT \
    -e NEO4J_dbms_memory_pagecache_size=$NEO4J_PAGECACHE \
    -e NEO4J_dbms_memory_heap_max__size=$NEO4J_HEAP_MAX \
    -v keyip-neo4j-data:/data \
    -v keyip-neo4j-logs:/logs \
    neo4j:5-community
}

start_redis() {
  _log "启动 Redis 7"
  docker run -d --name keyip-redis \
    $RESTART $LOGGING --network "$NETWORK" \
    -p 6379:6379 \
    redis:7-alpine \
    redis-server --loglevel warning --save "" --appendonly no
}

start_opensearch() {
  if $IS_DOCKER_MACHINE; then
    # docker-machine 旧内核 + OpenSearch 捆绑 JDK 不兼容，用 Elasticsearch 替代
    _warn "docker-machine: 使用 Elasticsearch 8.11 替代 OpenSearch（API 兼容）"
    docker rm -f keyip-opensearch 2>/dev/null || true
    docker run -d --name keyip-opensearch \
      $RESTART $LOGGING --network "$NETWORK" \
      -p 9200:9200 -p 9600:9600 \
      -e "discovery.type=single-node" \
      -e "xpack.security.enabled=false" \
      -e "ES_JAVA_OPTS=-Xms${OS_MEM} -Xmx${OS_MEM}" \
      -v keyip-opensearch-data:/usr/share/elasticsearch/data \
      docker.elastic.co/elasticsearch/elasticsearch:8.11.0
    return 0
  fi

  _log "启动 OpenSearch 2.14 (REST: 9200, 性能分析: 9600)"
  docker run -d --name keyip-opensearch \
    $RESTART $LOGGING --network "$NETWORK" \
    -p 9200:9200 -p 9600:9600 \
    -e discovery.type=single-node \
    -e DISABLE_SECURITY_PLUGIN=true \
    -e DISABLE_INSTALL_DEMO_CONFIG=true \
    -e OPENSEARCH_JAVA_OPTS="-Xms${OS_MEM} -Xmx${OS_MEM}" \
    -e plugins.security.disabled=true \
    -v keyip-opensearch-data:/usr/share/opensearch/data \
    opensearchproject/opensearch:2.14.0

  _log "启动 OpenSearch Dashboards (UI: 5601)"
  # docker-machine 下 Dashboards 可能缺 Node.js，跳过不影响后端
  if $IS_DOCKER_MACHINE; then
    _warn "docker-machine 环境跳过 Dashboards（可能缺少 Node.js 运行时）"
    return 0
  fi
  docker run -d --name keyip-os-dashboards \
    $RESTART $LOGGING --network "$NETWORK" \
    -p 5601:5601 \
    -e "OPENSEARCH_HOSTS=[\"http://opensearch:9200\"]" \
    -e DISABLE_SECURITY_DASHBOARDS_PLUGIN=true \
    opensearchproject/opensearch-dashboards:2.14.0
}

start_milvus() {
  _log "启动 Milvus 2.4 依赖 (etcd + minio)"
  # etcd
  docker run -d --name keyip-milvus-etcd \
    $RESTART $LOGGING --network "$NETWORK" \
    -e ETCD_AUTO_COMPACTION_MODE=revision \
    -e ETCD_AUTO_COMPACTION_RETENTION=1000 \
    -e ETCD_QUOTA_BACKEND_BYTES=4294967296 \
    -e ETCD_SNAPSHOT_COUNT=50000 \
    -v keyip-milvus-etcd-data:/etcd \
    quay.io/coreos/etcd:v3.5.14 \
    etcd --data-dir /etcd \
      --advertise-client-urls http://127.0.0.1:2379 \
      --listen-client-urls http://0.0.0.0:2379 \
      --log-level warn

  # milvus-minio (内部对象存储)
  docker run -d --name keyip-milvus-minio \
    $RESTART $LOGGING --network "$NETWORK" \
    -p 9000:9000 -p 9001:9001 \
    -e MINIO_ROOT_USER=minioadmin \
    -e MINIO_ROOT_PASSWORD=minioadmin \
    -v keyip-milvus-minio-data:/minio-data \
    minio/minio:RELEASE.2024-08-17T01-24-54Z \
    minio server /minio-data --console-address ":9001"

  _log "启动 Milvus 2.4 (gRPC: 19530)"
  docker run -d --name keyip-milvus \
    $RESTART $LOGGING --network "$NETWORK" \
    -p 19530:19530 \
    -e ETCD_ENDPOINTS=milvus-etcd:2379 \
    -e MINIO_ADDRESS=milvus-minio:9000 \
    -e MINIO_ACCESS_KEY=minioadmin \
    -e MINIO_SECRET_KEY=minioadmin \
    -e common__storage__type=minio \
    -v keyip-milvus-data:/var/lib/milvus \
    milvusdb/milvus:v2.4.6 \
    milvus run standalone
}

start_minio() {
  _log "启动 MinIO 独立实例 (S3: 9002, Console: 9003)"
  docker run -d --name keyip-minio \
    $RESTART $LOGGING --network "$NETWORK" \
    -p 9002:9000 -p 9003:9001 \
    -e MINIO_ROOT_USER=minioadmin \
    -e MINIO_ROOT_PASSWORD=minioadmin \
    -v keyip-minio-data:/data \
    minio/minio:RELEASE.2024-08-17T01-24-54Z \
    minio server /data --console-address ":9001"

  _log "创建 MinIO 默认存储桶 keyip-documents"
  docker run --rm --name keyip-minio-init \
    --network "$NETWORK" \
    minio/mc:latest \
    /bin/sh -c "
      until mc alias set local http://minio:9000 minioadmin minioadmin; do sleep 1; done;
      mc mb local/keyip-documents --ignore-existing;
      echo 'bucket keyip-documents 就绪';
    "
}

start_kafka() {
  _log "启动 Kafka 7.6 (9092)"
  docker run -d --name keyip-kafka \
    $RESTART $LOGGING --network "$NETWORK" \
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
}

start_mailhog() {
  _log "启动 MailHog (SMTP: 1025, UI: 8025)"
  docker run -d --name keyip-mailhog \
    $RESTART $LOGGING --network "$NETWORK" \
    -p 1025:1025 -p 8025:8025 \
    mailhog/mailhog:v1.0.1
}

start_keycloak() {
  _log "启动 Keycloak 24 (8180)"
  docker run -d --name keyip-keycloak \
    $RESTART $LOGGING --network "$NETWORK" \
    -p 8180:8080 \
    -e KEYCLOAK_ADMIN=admin \
    -e KEYCLOAK_ADMIN_PASSWORD=admin \
    -e KC_HOSTNAME=localhost \
    -e KC_HTTP_ENABLED=true \
    quay.io/keycloak/keycloak:24.0 \
    start-dev
}

# =============================================================================
# 服务组
# =============================================================================

start_all() {
  create_network
  start_postgres
  start_redis
  start_opensearch
  start_neo4j
  start_milvus
  start_minio
  start_kafka
  start_mailhog
  start_keycloak
  _log "全部服务启动完成。检查状态: docker ps --filter 'name=keyip-'"
}

start_minimal() {
  create_network
  start_postgres
  start_redis
  start_opensearch
  _log "最小服务集启动完成 (postgres, redis, opensearch)"
}

# =============================================================================
# 停止与清理
# =============================================================================

stop_all() {
  _log "停止并移除全部 keyip- 容器"
  docker ps -a --filter 'name=keyip-' --format '{{.Names}}' | xargs -r docker rm -f 2>/dev/null || true
  _log "容器已清理。"
  _warn "数据卷保留。如需清理: docker volume rm \$(docker volume ls -q --filter 'name=keyip-')"
}

show_status() {
  echo ""
  echo "====== KeyIP 服务状态 ======"
  docker ps --filter 'name=keyip-' --format 'table {{.Names}}\t{{.Status}}\t{{.Ports}}' 2>/dev/null || echo "无运行中的容器"
}

# =============================================================================
# 入口
# =============================================================================

case "${1:-all}" in
  postgres)   create_network; start_postgres;;
  neo4j)      create_network; start_neo4j;;
  redis)      create_network; start_redis;;
  opensearch) create_network; start_opensearch;;
  milvus)     create_network; start_milvus;;
  minio)      create_network; start_minio;;
  kafka)      create_network; start_kafka;;
  mailhog)    create_network; start_mailhog;;
  keycloak)   create_network; start_keycloak;;
  minimal)    start_minimal;;
  all)        start_all;;
  stop)       stop_all;;
  status)     show_status;;
  fix-vm)
    _log "修复 docker-machine 内核参数..."
    docker-machine ssh default sudo sysctl -w vm.max_map_count=262144
    docker-machine ssh default "echo 'vm.max_map_count=262144' | sudo tee -a /etc/sysctl.conf"
    _log "vm.max_map_count 已设为 262144（持久化）"
    ;;
  *)
    echo "用法: $0 {postgres|neo4j|redis|opensearch|milvus|minio|kafka|mailhog|keycloak|minimal|all|stop|status|fix-vm}"
    exit 1
    ;;
esac
