#!/usr/bin/env bash
# =============================================================================
# KeyIP-Intelligence: Database Reset & Re-seed
# =============================================================================
# ⚠️ DESTRUCTIVE: Drops all data and re-applies all migrations.
# Use this for fresh development starts or CI reset.
#
# Usage:
#   ./reset_db.sh            # Full reset + re-seed (interactive confirm)
#   ./reset_db.sh --force    # Skip confirmation
# =============================================================================

set -euo pipefail

FORCE=false
if [ "${1:-}" = "--force" ]; then
    FORCE=true
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

section() { echo -e "\n${BLUE}── $* ──${NC}"; }
success() { echo -e "  ${GREEN}✔${NC} $1"; }
fail() { echo -e "  ${RED}✘${NC} $1"; }

# ── Warning / Confirmation ──────────────────────────────────────────────────────
section "⚠️  Database Reset Warning"

if [ "$FORCE" = false ]; then
    echo ""
    echo -e "  ${RED}This will DELETE ALL DATA in PostgreSQL, OpenSearch, Neo4j, and Milvus.${NC}"
    echo "  This includes: patents, molecules, portfolios, users, lifecycle data, and more."
    echo ""
    echo -n "  Type 'yes' to confirm: "
    read -r confirm
    if [ "$confirm" != "yes" ]; then
        echo "  Aborted."
        exit 0
    fi
fi

# ── 1. PostgreSQL ──────────────────────────────────────────────────────────────
section "1/6: Resetting PostgreSQL"

PGHOST="${KEYIP_PG_HOST:-localhost}"
PGPORT="${KEYIP_PG_PORT:-5432}"

echo "  Dropping public schema..."
PGPASSWORD="keyip_dev" psql -h "$PGHOST" -p "$PGPORT" -U keyip -d keyip_dev -c "
    DROP SCHEMA public CASCADE;
    CREATE SCHEMA public;
    GRANT ALL ON SCHEMA public TO keyip;
    GRANT ALL ON SCHEMA public TO public;
" >/dev/null 2>&1 && success "PostgreSQL schema dropped" || fail "Failed to drop schema"

echo "  Re-running migrations..."
"$SCRIPT_DIR/seed_pg.sh"
success "PostgreSQL re-seeded"

# ── 2. OpenSearch ──────────────────────────────────────────────────────────────
section "2/6: Resetting OpenSearch"

OS_HOST="${KEYIP_OS_HOST:-localhost}"
OS_PORT="${KEYIP_OS_PORT:-9200}"

if curl -s "http://$OS_HOST:$OS_PORT" >/dev/null 2>&1; then
    for idx in keyip-patents keyip-molecules keyip-lifecycle; do
        echo "  Deleting index: $idx"
        curl -s -X DELETE "http://admin:admin@$OS_HOST:$OS_PORT/$idx" >/dev/null 2>&1
    done
    success "OpenSearch indexes cleared"

    echo "  Re-indexing..."
    "$SCRIPT_DIR/seed_opensearch.sh"
    success "OpenSearch re-indexed"
else
    echo "  OpenSearch not reachable — skipped"
fi

# ── 3. Milvus ──────────────────────────────────────────────────────────────────
section "3/6: Resetting Milvus"

MV_HOST="${KEYIP_MV_HOST:-localhost}"
MV_PORT="${KEYIP_MV_PORT:-19530}"

if curl -s "http://$MV_HOST:$MV_PORT/api/v1/health" >/dev/null 2>&1; then
    echo "  Dropping collections..."
    for coll in molecule_vectors claim_vectors; do
        curl -s -X DELETE "http://$MV_HOST:$MV_PORT/v2/vectordb/collections/drop" \
            -H 'Content-Type: application/json' \
            -d "{\"collectionName\": \"$coll\"}" >/dev/null 2>&1
    done
    success "Milvus collections dropped"

    echo "  Re-seeding vectors..."
    "$SCRIPT_DIR/seed_milvus.sh"
    success "Milvus re-seeded"
else
    echo "  Milvus not reachable — skipped"
fi

# ── 4. Neo4j ───────────────────────────────────────────────────────────────────
section "4/6: Resetting Neo4j"

N4_HOST="${KEYIP_N4_HOST:-localhost}"
N4_PORT="${KEYIP_N4_PORT:-7474}"

if curl -s "http://$N4_HOST:$N4_PORT" >/dev/null 2>&1; then
    echo "  Clearing graph..."
    curl -s -X POST "http://$N4_HOST:$N4_PORT/db/neo4j/tx/commit" \
        -u "neo4j:neo4j_dev" \
        -H 'Content-Type: application/json' \
        -d '{"statements": [{"statement": "MATCH (n) DETACH DELETE n"}]}' >/dev/null 2>&1
    success "Neo4j graph cleared"

    echo "  Re-loading..."
    "$SCRIPT_DIR/seed_neo4j.sh"
    success "Neo4j re-loaded"
else
    echo "  Neo4j not reachable — skipped"
fi

# ── 5. Redis ──────────────────────────────────────────────────────────────────
section "5/6: Flushing Redis cache"

RD_HOST="${KEYIP_RD_HOST:-localhost}"
RD_PORT="${KEYIP_RD_PORT:-6379}"

if command -v redis-cli &>/dev/null; then
    redis-cli -h "$RD_HOST" -p "$RD_PORT" FLUSHDB >/dev/null 2>&1 && \
        success "Redis cache flushed"
else
    echo "  redis-cli not found — skipped"
fi

# ── 6. MinIO ───────────────────────────────────────────────────────────────────
section "6/6: Resetting MinIO"

MN_HOST="${KEYIP_MN_HOST:-localhost}"
MN_PORT="${KEYIP_MN_PORT:-9000}"

if curl -s "http://$MN_HOST:$MN_PORT/minio/health/live" >/dev/null 2>&1; then
    if command -v mc &>/dev/null; then
        mc alias set keyip-minio "http://$MN_HOST:$MN_PORT" minioadmin minioadmin >/dev/null 2>&1
        for bucket in keyip-documents keyip-reports keyip-molecule-images; do
            mc rb "keyip-minio/$bucket" --force 2>/dev/null || true
        done
        success "MinIO buckets cleared"
    else
        echo "  mc (MinIO client) not found — skipped bucket cleanup"
    fi

    echo "  Re-seeding MinIO..."
    "$SCRIPT_DIR/seed_minio.sh"
    success "MinIO re-seeded"
else
    echo "  MinIO not reachable — skipped"
fi

# ── Summary ────────────────────────────────────────────────────────────────────
section "Reset Complete"
echo "  All data sources have been reset and re-seeded."
echo ""
echo "  Verify with:"
echo "    $SCRIPT_DIR/verify_data.sh"
echo ""
