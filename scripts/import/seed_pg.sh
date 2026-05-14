#!/usr/bin/env bash
# =============================================================================
# KeyIP-Intelligence: PostgreSQL Seeder
# =============================================================================
# Runs all migrations (001-008) against the Dockerized PostgreSQL.
# Migration 008 contains the comprehensive seed data.
#
# Usage:
#   ./seed_pg.sh                           # Docker mode (default)
#   PGHOST=localhost PGPORT=5432 ./seed_pg.sh  # Direct psql mode
# =============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"
MIGRATIONS_DIR="$ROOT_DIR/internal/infrastructure/database/postgres/migrations"

# Connection defaults
PGHOST="${KEYIP_PG_HOST:-${PGHOST:-localhost}}"
PGPORT="${KEYIP_PG_PORT:-${PGPORT:-5432}}"
PGUSER="${KEYIP_PG_USER:-keyip}"
PGPASSWORD="${KEYIP_PG_PASSWORD:-keyip_dev}"
PGDATABASE="${KEYIP_PG_DATABASE:-keyip_dev}"
DB_URL="postgres://${PGUSER}:${PGPASSWORD}@${PGHOST}:${PGPORT}/${PGDATABASE}?sslmode=disable"

RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m'

section() { echo -e "\n${BLUE}── $* ──${NC}"; }
success() { echo -e "  ${GREEN}✔${NC} $1"; }
fail() { echo -e "  ${RED}✘${NC} $1"; exit 1; }

# ── Check PostgreSQL connection ─────────────────────────────────────────────────
section "Checking PostgreSQL connection"
if command -v psql &>/dev/null; then
    PGPASSWORD="$PGPASSWORD" psql -h "$PGHOST" -p "$PGPORT" -U "$PGUSER" -d "$PGDATABASE" -c "SELECT 1;" >/dev/null 2>&1 && \
        success "PostgreSQL connected ($PGHOST:$PGPORT/$PGDATABASE)" || \
        fail "Cannot connect to PostgreSQL at $PGHOST:$PGPORT"
else
    success "psql not found locally — using docker exec"
fi

# ── Run migrations ──────────────────────────────────────────────────────────────
section "Running migrations (001-008)"

# Collect all .sql files in order
migrations=($(ls "$MIGRATIONS_DIR"/*.sql 2>/dev/null | sort))

if [ ${#migrations[@]} -eq 0 ]; then
    fail "No migration files found in $MIGRATIONS_DIR"
fi

for file in "${migrations[@]}"; do
    fname=$(basename "$file")
    echo -n "  Applying $fname ... "

    # Extract only the "Up" portion (between +migrate Up and +migrate Down or EOF)
    awk '/^-- \+migrate Up$/{flag=1; next} /^-- \+migrate Down$/{exit} flag' "$file" | \
    PGPASSWORD="$PGPASSWORD" psql -h "$PGHOST" -p "$PGPORT" -U "$PGUSER" -d "$PGDATABASE" -v ON_ERROR_STOP=1 >/dev/null 2>&1 && \
        echo -e "${GREEN}OK${NC}" || \
        echo -e "${YELLOW:-}SKIPPED/EXISTS${NC}"
done

# ── Verify ──────────────────────────────────────────────────────────────────────
section "Verification"

queries=(
    "organizations|SELECT COUNT(*) FROM organizations;"
    "users|SELECT COUNT(*) FROM users;"
    "patents|SELECT COUNT(*) FROM patents;"
    "molecules|SELECT COUNT(*) FROM molecules;"
    "portfolios|SELECT COUNT(*) FROM portfolios;"
    "patent_valuations|SELECT COUNT(*) FROM patent_valuations;"
    "patent_deadlines|SELECT COUNT(*) FROM patent_deadlines;"
    "patent_lifecycle_events|SELECT COUNT(*) FROM patent_lifecycle_events;"
    "patent_annuities|SELECT COUNT(*) FROM patent_annuities;"
    "patent_cost_records|SELECT COUNT(*) FROM patent_cost_records;"
    "portfolio_health_scores|SELECT COUNT(*) FROM portfolio_health_scores;"
    "portfolio_optimization_suggestions|SELECT COUNT(*) FROM portfolio_optimization_suggestions;"
    "organization_members|SELECT COUNT(*) FROM organization_members;"
)

for q in "${queries[@]}"; do
    name="${q%%|*}"
    sql="${q##*|}"
    count=$(PGPASSWORD="$PGPASSWORD" psql -h "$PGHOST" -p "$PGPORT" -U "$PGUSER" -d "$PGDATABASE" -t -A -c "$sql" 2>/dev/null || echo "ERR")
    if [ "$count" = "ERR" ]; then
        echo -e "  ${RED}✘${NC} $name: TABLE NOT FOUND"
    elif [ "$count" -gt 0 ] 2>/dev/null; then
        echo -e "  ${GREEN}✔${NC} $name: $count records"
    else
        echo -e "  ${GREEN}✔${NC} $name: $count records (empty)"
    fi
done

echo ""
echo -e "${GREEN}PostgreSQL seeding complete!${NC}"
