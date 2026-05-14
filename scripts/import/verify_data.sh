#!/usr/bin/env bash
# =============================================================================
# KeyIP-Intelligence: Data Integrity Verification
# =============================================================================
# Runs cross-source verification queries against all data sources.
# Reports record counts and key data integrity checks.
# =============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"

BLUE='\033[0;34m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

section() { echo -e "\n${BLUE}═══════════════════════════════════════════════════════════════${NC}"; echo -e "${BLUE}  $*${NC}"; echo -e "${BLUE}═══════════════════════════════════════════════════════════════${NC}"; }
ok()  { echo -e "  ${GREEN}✔${NC} $1"; }
warn() { echo -e "  ${YELLOW}⚠${NC} $1"; }
err() { echo -e "  ${RED}✘${NC} $1"; }
info() { echo -e "     $1"; }

# Connection params
PGHOST="${KEYIP_PG_HOST:-localhost}"
PGPORT="${KEYIP_PG_PORT:-5432}"
OS_HOST="${KEYIP_OS_HOST:-localhost}"
OS_PORT="${KEYIP_OS_PORT:-9200}"
N4_HOST="${KEYIP_N4_HOST:-localhost}"
MV_HOST="${KEYIP_MV_HOST:-localhost}"
MV_PORT="${KEYIP_MV_PORT:-19530}"
MN_HOST="${KEYIP_MN_HOST:-localhost}"
MN_PORT="${KEYIP_MN_PORT:-9000}"
RD_HOST="${KEYIP_RD_HOST:-localhost}"
RD_PORT="${KEYIP_RD_PORT:-6379}"

pg_q() {
    PGPASSWORD="keyip_dev" psql -h "$PGHOST" -p "$PGPORT" -U keyip -d keyip_dev -t -A -c "$1" 2>/dev/null || echo "N/A"
}

os_count() {
    curl -s "http://admin:admin@$OS_HOST:$OS_PORT/$1/_count" 2>/dev/null | \
        python3 -c "import sys,json; print(json.load(sys.stdin).get('count',0))" 2>/dev/null || echo "N/A"
}

echo "KeyIP-Intelligence Data Integrity Report"
echo "Generated: $(date '+%Y-%m-%d %H:%M:%S')"
echo ""

# ═══════════════════════════════════════════════════════════════════════════════
section "1. PostgreSQL — Record Counts"

pg_q "SELECT 'organizations: ' || COUNT(*) FROM organizations;"
pg_q "SELECT 'users:           ' || COUNT(*) FROM users;"
pg_q "SELECT 'patents:         ' || COUNT(*) FROM patents;"
pg_q "SELECT 'patent_claims:   ' || COUNT(*) FROM patent_claims;"
pg_q "SELECT 'patent_inventors:' || COUNT(*) FROM patent_inventors;"
pg_q "SELECT 'molecules:       ' || COUNT(*) FROM molecules;"
pg_q "SELECT 'molecule_props:  ' || COUNT(*) FROM molecule_properties;"
pg_q "SELECT 'mol_fingerprints:' || COUNT(*) FROM molecule_fingerprints;"
pg_q "SELECT 'pat-mol_relations:'|| COUNT(*) FROM patent_molecule_relations;"
pg_q "SELECT 'portfolios:      ' || COUNT(*) FROM portfolios;"
pg_q "SELECT 'portfolio_patents:'|| COUNT(*) FROM portfolio_patents;"
pg_q "SELECT 'valuations:      ' || COUNT(*) FROM patent_valuations;"
pg_q "SELECT 'health_scores:   ' || COUNT(*) FROM portfolio_health_scores;"
pg_q "SELECT 'opt_suggestions: ' || COUNT(*) FROM portfolio_optimization_suggestions;"
pg_q "SELECT 'annuities:       ' || COUNT(*) FROM patent_annuities;"
pg_q "SELECT 'deadlines:       ' || COUNT(*) FROM patent_deadlines;"
pg_q "SELECT 'lifecycle_events:' || COUNT(*) FROM patent_lifecycle_events;"
pg_q "SELECT 'cost_records:    ' || COUNT(*) FROM patent_cost_records;"
pg_q "SELECT 'org_members:     ' || COUNT(*) FROM organization_members;"

# ═══════════════════════════════════════════════════════════════════════════════
section "2. PostgreSQL — Business Integrity Checks"

echo "  Patent status distribution:"
pg_q "SELECT '    ' || status::text || ': ' || COUNT(*) FROM patents GROUP BY status ORDER BY COUNT(*) DESC;"

echo "  Jurisdiction distribution:"
pg_q "SELECT '    ' || jurisdiction || ': ' || COUNT(*) FROM patents GROUP BY jurisdiction ORDER BY COUNT(*) DESC;"

echo "  Molecule-to-patent cross-reference:"
pg_q "SELECT '    linked molecules: ' || COUNT(DISTINCT molecule_id) || ' / ' || (SELECT COUNT(*) FROM molecules) || ' total' FROM patent_molecule_relations;"

echo "  Portfolio health overview:"
pg_q "SELECT '    ' || p.name || ': score ' || ROUND(h.overall_score) || '/100, ' || h.total_patents || ' patents' FROM portfolio_health_scores h JOIN portfolios p ON p.id = h.portfolio_id ORDER BY h.evaluated_at DESC LIMIT 5;"

echo "  Upcoming critical deadlines (next 90 days):"
pg_q "SELECT '    ' || title || ' [' || priority || '] due ' || due_date::date FROM patent_deadlines WHERE status = 'active' AND due_date < NOW() + INTERVAL '90 days' ORDER BY due_date LIMIT 5;"

echo "  Orphaned records check:"
pg_q "SELECT '    portfolio_patents with missing patent: ' || COUNT(*) FROM portfolio_patents pp LEFT JOIN patents p ON p.id = pp.patent_id WHERE p.id IS NULL;"
pg_q "SELECT '    patent_valuations with missing patent: ' || COUNT(*) FROM patent_valuations pv LEFT JOIN patents p ON p.id = pv.patent_id WHERE p.id IS NULL;"
pg_q "SELECT '    deadlines with missing patent: ' || COUNT(*) FROM patent_deadlines pd LEFT JOIN patents p ON p.id = pd.patent_id WHERE p.id IS NULL;"

# ═══════════════════════════════════════════════════════════════════════════════
section "3. OpenSearch — Index Health"

if curl -s "http://$OS_HOST:$OS_PORT" >/dev/null 2>&1; then
    echo "  Documents indexed:"
    for idx in keyip-patents keyip-molecules; do
        count=$(os_count "$idx")
        echo "     $idx: $count"
    done

    echo ""
    echo "  Sample patent search:"
    curl -s "http://admin:admin@$OS_HOST:$OS_PORT/keyip-patents/_search?size=1" 2>/dev/null | \
        python3 -c "
import sys, json
data = json.load(sys.stdin)
hits = data.get('hits',{}).get('hits',[])
if hits:
    src = hits[0]['_source']
    print(f'     {src.get(\"patent_number\",\"?\")} — {src.get(\"title\",\"?\")[:80]}')
else:
    print('     (no results)')
" 2>/dev/null || echo "     (search failed)"
else
    warn "OpenSearch not reachable"
fi

# ═══════════════════════════════════════════════════════════════════════════════
section "4. Neo4j — Graph Health"

if curl -s "http://$N4_HOST:7474" >/dev/null 2>&1; then
    curl -s -X POST "http://$N4_HOST:7474/db/neo4j/tx/commit" \
        -u "neo4j:neo4j_dev" \
        -H 'Content-Type: application/json' \
        -d '{"statements": [{"statement": "MATCH (n) RETURN labels(n)[0] AS label, count(n) AS cnt ORDER BY cnt DESC"}]}' 2>/dev/null | \
        python3 -c "
import sys, json
data = json.load(sys.stdin)
results = data.get('results',[{}])[0].get('data',[])
if results:
    for row in results:
        label, cnt = row['row']
        print(f'     {label:20s}: {cnt:4d}')
    # Relationship counts
else:
    print('     (no graph data)')
" 2>/dev/null || echo "     (graph query failed)"

    curl -s -X POST "http://$N4_HOST:7474/db/neo4j/tx/commit" \
        -u "neo4j:neo4j_dev" \
        -H 'Content-Type: application/json' \
        -d '{"statements": [{"statement": "MATCH ()-[r]->() RETURN type(r) AS rel_type, count(r) AS cnt ORDER BY cnt DESC"}]}' 2>/dev/null | \
        python3 -c "
import sys, json
data = json.load(sys.stdin)
results = data.get('results',[{}])[0].get('data',[])
if results:
    print('')
    print('  Relationships:')
    for row in results:
        rel, cnt = row['row']
        print(f'     {rel:20s}: {cnt:4d}')
" 2>/dev/null || true
else
    warn "Neo4j not reachable"
fi

# ═══════════════════════════════════════════════════════════════════════════════
section "5. Milvus — Vector Collections"

if curl -s "http://$MV_HOST:$MV_PORT/api/v1/health" >/dev/null 2>&1; then
    curl -s -X POST "http://$MV_HOST:$MV_PORT/v2/vectordb/collections/list" \
        -H 'Content-Type: application/json' 2>/dev/null | \
        python3 -c "
import sys, json
data = json.load(sys.stdin)
print(f'     Collections: {json.dumps(data, indent=2)[:500]}')
" 2>/dev/null || echo "     (collection query failed)"
else
    warn "Milvus not reachable"
fi

# ═══════════════════════════════════════════════════════════════════════════════
section "6. MinIO — Bucket Contents"

if curl -s "http://$MN_HOST:$MN_PORT/minio/health/live" >/dev/null 2>&1; then
    for bucket in keyip-documents keyip-reports keyip-molecule-images; do
        count=$(curl -s "http://$MN_HOST:$MN_PORT/$bucket" 2>/dev/null | \
            python3 -c "
import sys
content = sys.stdin.read()
if '<Key>' in content:
    print(content.count('<Key>'))
else:
    print(0)
" 2>/dev/null || echo "?")
        echo "     $bucket: $count objects"
    done
else
    warn "MinIO not reachable"
fi

# ═══════════════════════════════════════════════════════════════════════════════
section "7. Redis — Connection Check"

if command -v redis-cli &>/dev/null && redis-cli -h "$RD_HOST" -p "$RD_PORT" PING >/dev/null 2>&1; then
    ok "Redis: PONG"
else
    warn "Redis not reachable or redis-cli not found"
fi

# ═══════════════════════════════════════════════════════════════════════════════
section "Verification Complete"
echo "  For cross-source integrity, check that:"
echo "  - Patent count matches across PG ↔ OpenSearch ↔ Neo4j"
echo "  - Molecule count matches across PG ↔ OpenSearch ↔ Neo4j ↔ Milvus"
echo "  - No orphaned foreign keys exist"
echo "  - Upcoming deadlines have valid patent references"
echo ""
