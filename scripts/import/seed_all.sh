#!/usr/bin/env bash
# =============================================================================
# KeyIP-Intelligence: Master Data Seeder
# =============================================================================
# Orchestrates seeding all 7 data sources.
# Requires: docker compose services running (postgres, opensearch, milvus, etc.)
#
# Usage:
#   ./seed_all.sh              # Seed everything
#   ./seed_all.sh --skip-milvus  # Skip Milvus (needs special HW)
#   ./seed_all.sh --skip-neo4j   # Skip Neo4j
#   ./seed_all.sh --help         # Show help
# =============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"
MIGRATIONS_DIR="$ROOT_DIR/internal/infrastructure/database/postgres/migrations"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Defaults
SKIP_MILVUS=false
SKIP_NEO4J=false
SKIP_OPENSEARCH=false
SKIP_MINIO=false
SKIP_PG=false

# Parse flags
for arg in "$@"; do
    case "$arg" in
        --skip-milvus)     SKIP_MILVUS=true ;;
        --skip-neo4j)      SKIP_NEO4J=true ;;
        --skip-opensearch) SKIP_OPENSEARCH=true ;;
        --skip-minio)      SKIP_MINIO=true ;;
        --skip-pg)         SKIP_PG=true ;;
        --help|-h)
            echo "KeyIP-Intelligence Master Data Seeder"
            echo "Usage: $0 [flags]"
            echo "  --skip-pg          Skip PostgreSQL seeding"
            echo "  --skip-neo4j         Skip Neo4j graph seeding"
            echo "  --skip-opensearch    Skip OpenSearch indexing"
            echo "  --skip-milvus        Skip Milvus vector indexing"
            echo "  --skip-minio         Skip MinIO sample uploads"
            echo "  --help               Show this help"
            exit 0
            ;;
        *)
            echo -e "${RED}Unknown flag: $arg${NC}"
            exit 1
            ;;
    esac
done

section() {
    echo ""
    echo -e "${BLUE}═══════════════════════════════════════════════════════════════${NC}"
    echo -e "${BLUE}  $*${NC}"
    echo -e "${BLUE}═══════════════════════════════════════════════════════════════${NC}"
}

success() { echo -e "  ${GREEN}✔${NC} $1"; }
warn()   { echo -e "  ${YELLOW}⚠${NC} $1"; }
fail()   { echo -e "  ${RED}✘${NC} $1"; }

check_service() {
    local name="$1" host="$2" port="$3"
    if nc -z -w2 "$host" "$port" 2>/dev/null; then
        success "$name is running ($host:$port)"
        return 0
    else
        warn "$name is NOT reachable at $host:$port"
        return 1
    fi
}

# ── Pre-flight checks ──────────────────────────────────────────────────────────
section "Pre-flight: Service Availability"
ALL_OK=true

check_service "PostgreSQL"  "${KEYIP_PG_HOST:-localhost}"  "${KEYIP_PG_PORT:-5432}" || ALL_OK=false
check_service "OpenSearch"  "${KEYIP_OS_HOST:-localhost}"  "${KEYIP_OS_PORT:-9200}"  || ALL_OK=false
check_service "Milvus"      "${KEYIP_MV_HOST:-localhost}"  "${KEYIP_MV_PORT:-19530}" || ALL_OK=false
check_service "Neo4j"       "${KEYIP_N4_HOST:-localhost}"  "${KEYIP_N4_PORT:-7687}"  || ALL_OK=false
check_service "Redis"       "${KEYIP_RD_HOST:-localhost}"  "${KEYIP_RD_PORT:-6379}"  || ALL_OK=false
check_service "MinIO"       "${KEYIP_MN_HOST:-localhost}"  "${KEYIP_MN_PORT:-9000}"  || ALL_OK=false

if [ "$ALL_OK" = false ]; then
    echo ""
    echo -e "${YELLOW}Some services are not running. Start them with:${NC}"
    echo "  docker compose -f deployments/docker/docker-compose.yml up -d --wait"
    echo ""
    echo -e "${YELLOW}Continuing with available services...${NC}"
    echo ""
fi

# ── 1. PostgreSQL ──────────────────────────────────────────────────────────────
if [ "$SKIP_PG" = false ]; then
    section "1/5: PostgreSQL — Run migrations + seed data"

    # Run migrations
    echo "  Running database migrations..."
    docker run --rm \
        --network keyip-network \
        -v "$MIGRATIONS_DIR:/migrations" \
        migrate/migrate:v4.17.0 \
        -path=/migrations \
        -database="postgres://keyip:keyip_dev@keyip-postgres:5432/keyip_dev?sslmode=disable" \
        up 2>&1 || true

    # Verify seed
    echo "  Verifying seed data..."
    COUNT=$(docker exec keyip-postgres psql -U keyip -d keyip_dev -t -A -c "SELECT COUNT(*) FROM patents;" 2>/dev/null || echo "0")
    if [ "$COUNT" -gt 0 ] 2>/dev/null; then
        success "PostgreSQL seeded: $COUNT patents, $(docker exec keyip-postgres psql -U keyip -d keyip_dev -t -A -c 'SELECT COUNT(*) FROM molecules;' 2>/dev/null || echo '?') molecules, $(docker exec keyip-postgres psql -U keyip -d keyip_dev -t -A -c 'SELECT COUNT(*) FROM users;' 2>/dev/null || echo '?') users"
    else
        fail "PostgreSQL seeding may have failed (0 patents found)"
    fi
else
    section "1/5: PostgreSQL — SKIPPED (--skip-pg)"
fi

# ── 2. OpenSearch ──────────────────────────────────────────────────────────────
if [ "$SKIP_OPENSEARCH" = false ]; then
    section "2/5: OpenSearch — Create indexes + index data"
    if check_service "OpenSearch" "${KEYIP_OS_HOST:-localhost}" "${KEYIP_OS_PORT:-9200}"; then
        "$SCRIPT_DIR/seed_opensearch.sh"
        success "OpenSearch indexing complete"
    else
        warn "Skipping OpenSearch (service not available)"
    fi
else
    section "2/5: OpenSearch — SKIPPED (--skip-opensearch)"
fi

# ── 3. Milvus ──────────────────────────────────────────────────────────────────
if [ "$SKIP_MILVUS" = false ]; then
    section "3/5: Milvus — Create collections + insert vectors"
    if check_service "Milvus" "${KEYIP_MV_HOST:-localhost}" "${KEYIP_MV_PORT:-19530}"; then
        "$SCRIPT_DIR/seed_milvus.sh"
        success "Milvus vector indexing complete"
    else
        warn "Skipping Milvus (service not available)"
    fi
else
    section "3/5: Milvus — SKIPPED (--skip-milvus)"
fi

# ── 4. Neo4j ───────────────────────────────────────────────────────────────────
if [ "$SKIP_NEO4J" = false ]; then
    section "4/5: Neo4j — Load knowledge graph"
    if check_service "Neo4j" "${KEYIP_N4_HOST:-localhost}" "${KEYIP_N4_PORT:-7687}"; then
        "$SCRIPT_DIR/seed_neo4j.sh"
        success "Neo4j graph loaded"
    else
        warn "Skipping Neo4j (service not available)"
    fi
else
    section "4/5: Neo4j — SKIPPED (--skip-neo4j)"
fi

# ── 5. MinIO ───────────────────────────────────────────────────────────────────
if [ "$SKIP_MINIO" = false ]; then
    section "5/5: MinIO — Create buckets + upload samples"
    if check_service "MinIO" "${KEYIP_MN_HOST:-localhost}" "${KEYIP_MN_PORT:-9000}"; then
        "$SCRIPT_DIR/seed_minio.sh"
        success "MinIO sample data uploaded"
    else
        warn "Skipping MinIO (service not available)"
    fi
else
    section "5/5: MinIO — SKIPPED (--skip-minio)"
fi

# ── Summary ────────────────────────────────────────────────────────────────────
section "Import Pipeline Complete"
echo "  Use the verify script to check data integrity:"
echo "    $SCRIPT_DIR/verify_data.sh"
echo ""
