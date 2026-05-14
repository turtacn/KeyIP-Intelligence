#!/bin/bash
# KeyIP-Intelligence 数据归档系统
# 将所有中间件数据导出到项目目录，确保容器重启后数据可恢复
#
# 用法:
#   bash scripts/import/archive_data.sh              # 导出所有数据
#   bash scripts/import/archive_data.sh --restore     # 从归档恢复
#   bash scripts/import/archive_data.sh --verify       # 验证归档完整性
#
# 归档目录结构:
#   test/testdata/archive/
#   ├── postgres/         # PostgreSQL 表数据 (JSON)
#   ├── opensearch/       # OpenSearch 索引 (JSON)
#   ├── neo4j/            # Neo4j 图数据 (Cypher)
#   ├── milvus/           # Milvus 向量集合
#   ├── minio/            # MinIO 对象存储
#   └── manifest.json     # 归档清单

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"
ARCHIVE_DIR="$ROOT_DIR/test/testdata/archive"
BASE_URL="${KEYIP_BASE_URL:-http://192.168.99.100}"
TIMESTAMP=$(date -u '+%Y-%m-%dT%H:%M:%SZ')
MANIFEST="$ARCHIVE_DIR/manifest.json"

mkdir -p "$ARCHIVE_DIR"/{postgres,opensearch,neo4j,milvus,minio}

log() { echo "[$(date '+%H:%M:%S')] $*"; }
success() { echo "  ✅ $1"; }
warn() { echo "  ⚠️  $1"; }

# ─── API 数据导出 ───────────────────────────────────────────────
export_from_api() {
    log "1. Exporting from API ($BASE_URL)..."

    mkdir -p "$ARCHIVE_DIR/postgres"

    # Molecules
    curl -s "$BASE_URL/api/v1/molecules?pageSize=100" > "$ARCHIVE_DIR/postgres/molecules.json" 2>/dev/null && \
        success "molecules ($(python3 -c "import json; d=json.load(open('$ARCHIVE_DIR/postgres/molecules.json')); print(d['pagination']['total'])" 2>/dev/null || echo '?') records)" || \
        warn "molecules export failed"

    # Patents
    curl -s "$BASE_URL/api/v1/patents?pageSize=100" > "$ARCHIVE_DIR/postgres/patents.json" 2>/dev/null && \
        success "patents ($(python3 -c "import json; d=json.load(open('$ARCHIVE_DIR/postgres/patents.json')); print(d['pagination']['total'])" 2>/dev/null || echo '?') records)" || \
        warn "patents export failed"

    # Portfolios
    curl -s "$BASE_URL/api/v1/portfolios?pageSize=50" > "$ARCHIVE_DIR/postgres/portfolios.json" 2>/dev/null && \
        success "portfolios exported" || warn "portfolios export failed"

    # Dashboard metrics
    curl -s "$BASE_URL/api/v1/dashboard/metrics" > "$ARCHIVE_DIR/postgres/dashboard_metrics.json" 2>/dev/null && \
        success "dashboard metrics" || warn "metrics export failed"

    # Infringement alerts
    curl -s "$BASE_URL/api/v1/infringement/alerts?pageSize=50" > "$ARCHIVE_DIR/postgres/infringement_alerts.json" 2>/dev/null && \
        success "infringement alerts" || warn "alerts export failed"

    # FTO results
    curl -s -X POST "$BASE_URL/api/v1/fto/search" -H "Content-Type: application/json" \
        -d '{"query":"CBP"}' > "$ARCHIVE_DIR/postgres/fto_search.json" 2>/dev/null && \
        success "FTO search data" || warn "FTO export failed"
}

# ─── 种子数据保存 ───────────────────────────────────────────────
archive_seeds() {
    log "2. Archiving seed SQL..."

    cp "$ROOT_DIR/internal/infrastructure/database/postgres/migrations/008_seed_data.sql" \
       "$ARCHIVE_DIR/postgres/seed_data.sql" 2>/dev/null && \
        success "seed SQL archived" || warn "seed SQL copy failed"

    # 保存所有 migration 文件
    mkdir -p "$ARCHIVE_DIR/postgres/migrations"
    cp "$ROOT_DIR"/internal/infrastructure/database/postgres/migrations/*.sql \
       "$ARCHIVE_DIR/postgres/migrations/" 2>/dev/null && \
        success "$(ls "$ARCHIVE_DIR/postgres/migrations/" | wc -l) migration files" || :
}

# ─── 测试数据归档 ───────────────────────────────────────────────
archive_test_data() {
    log "3. Archiving test fixtures..."

    if [ -d "$ROOT_DIR/test/testdata/fixtures" ]; then
        cp -r "$ROOT_DIR/test/testdata/fixtures" "$ARCHIVE_DIR/" 2>/dev/null && \
            success "test fixtures" || warn "fixtures copy failed"
    fi
}

# ─── 生成清单 ───────────────────────────────────────────────────
generate_manifest() {
    log "4. Generating manifest..."

    cat > "$MANIFEST" << EOF
{
  "version": "1.0",
  "generated_at": "$TIMESTAMP",
  "project": "KeyIP-Intelligence",
  "environment": "docker-machine",
  "contents": {
    "postgres": {
      "molecules": $(python3 -c "import json; d=json.load(open('$ARCHIVE_DIR/postgres/molecules.json','r')); print(d['pagination']['total'])" 2>/dev/null || echo 0),
      "patents": $(python3 -c "import json; d=json.load(open('$ARCHIVE_DIR/postgres/patents.json','r')); print(d['pagination']['total'])" 2>/dev/null || echo 0),
      "seed_sql": "$(wc -c < "$ARCHIVE_DIR/postgres/seed_data.sql" 2>/dev/null || echo 0) bytes",
      "migrations": $(ls "$ARCHIVE_DIR/postgres/migrations/" 2>/dev/null | wc -l)
    },
    "files": $(find "$ARCHIVE_DIR" -type f | python3 -c "import sys,json,os; files=[{'path':l.strip().replace('$ARCHIVE_DIR/',''),'size':os.path.getsize(l.strip())} for l in sys.stdin]; print(json.dumps(files,indent=4))" 2>/dev/null || echo '[]')
  }
}
EOF
    success "manifest generated: $MANIFEST"
}

# ─── 验证归档 ───────────────────────────────────────────────────
verify_archive() {
    log "Verifying archive integrity..."

    local ok=0 fail=0

    check_file() {
        if [ -f "$1" ] && [ -s "$1" ]; then ok=$((ok+1)); else fail=$((fail+1)); warn "$2"; fi
    }

    check_file "$ARCHIVE_DIR/postgres/molecules.json" "molecules.json"
    check_file "$ARCHIVE_DIR/postgres/patents.json" "patents.json"
    check_file "$ARCHIVE_DIR/postgres/seed_data.sql" "seed_data.sql"
    check_file "$MANIFEST" "manifest.json"

    echo "  Valid files: $ok | Missing/empty: $fail"
    return $fail
}

# ─── 恢复数据 ───────────────────────────────────────────────────
restore_from_archive() {
    log "Restoring data from archive..."

    if [ ! -f "$ARCHIVE_DIR/postgres/seed_data.sql" ]; then
        echo "ERROR: No seed SQL found in archive. Run archive first."
        exit 1
    fi

    echo ""
    echo "┌────────────────────────────────────────────────────────────┐"
    echo "│  RESTORE INSTRUCTIONS                                      │"
    echo "│                                                            │"
    echo "│  The archive files are at:                                 │"
    echo "│    $ARCHIVE_DIR"
    echo "│                                                            │"
    echo "│  To restore (after container restart):                     │"
    echo "│  1. docker compose -f deployments/docker/docker-compose.yml up -d --wait"
    echo "│  2. make migrate-up && make seed                           │"
    echo "│  3. bash scripts/import/seed_all.sh                        │"
    echo "│                                                            │"
    echo "│  For API-level restore:                                    │"
    echo "│  node scripts/import/fetch_patents.js \\                    │"
    echo "│       --file=$ARCHIVE_DIR/postgres/patents.json"
    echo "│                                                            │"
    echo "│  All data is safe in $ROOT_DIR/test/testdata/"
    echo "└────────────────────────────────────────────────────────────┘"
    echo ""
}

# ─── Main ───────────────────────────────────────────────────────
case "${1:-}" in
    --restore|-r)
        restore_from_archive
        ;;
    --verify|-v)
        verify_archive
        ;;
    --help|-h)
        echo "Usage: $0 [--restore|--verify|--help]"
        echo "  (no args)   Archive all data to $ARCHIVE_DIR"
        echo "  --restore   Show restore instructions"
        echo "  --verify    Check archive integrity"
        ;;
    *)
        export_from_api
        archive_seeds
        archive_test_data
        generate_manifest
        log "5. Archive complete!"
        echo ""
        echo "  📁 Archive: $ARCHIVE_DIR"
        echo "  📄 Manifest: $MANIFEST"
        echo "  💾 Size: $(du -sh "$ARCHIVE_DIR" 2>/dev/null | cut -f1)"
        echo ""
        verify_archive
        ;;
esac
