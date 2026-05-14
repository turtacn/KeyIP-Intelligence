#!/usr/bin/env bash
# =============================================================================
# KeyIP-Intelligence: Neo4j Knowledge Graph Seeder
# =============================================================================
# Creates citation network, inventor-assignee relationships, patent-molecule
# links, and jurisdiction filing graphs in Neo4j.
#
# Prerequisites: PostgreSQL seeded, Neo4j running
# =============================================================================

set -euo pipefail

N4_HOST="${KEYIP_N4_HOST:-localhost}"
N4_PORT="${KEYIP_N4_PORT:-7687}"
N4_USER="${KEYIP_N4_USER:-neo4j}"
N4_PASS="${KEYIP_N4_PASS:-neo4j_dev}"
N4_URL="bolt://${N4_HOST}:${N4_PORT}"

BLUE='\033[0;34m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

section() { echo -e "\n${BLUE}── $* ──${NC}"; }
success() { echo -e "  ${GREEN}✔${NC} $1"; }
warn() { echo -e "  ${YELLOW}⚠${NC} $1"; }

# ── Check Neo4j connectivity ────────────────────────────────────────────────────
section "Checking Neo4j connection"
if curl -s "http://${N4_HOST}:7474" >/dev/null 2>&1; then
    success "Neo4j HTTP is reachable"
else
    warn "Neo4j is not reachable — skipping graph seeding"
    echo "  Start with: docker compose -f deployments/docker/docker-compose.yml up -d neo4j"
    exit 0
fi

N4_AUTH="${N4_USER}:${N4_PASS}"
N4_HTTP="http://${N4_HOST}:7474"

run_cypher() {
    curl -s -X POST "$N4_HTTP/db/neo4j/tx/commit" \
        -u "$N4_AUTH" \
        -H 'Content-Type: application/json' \
        -d "{\"statements\": [{\"statement\": \"$1\"}]}" >/dev/null 2>&1
}

# ── Clear existing graph (idempotent) ───────────────────────────────────────────
section "Clearing existing graph"
run_cypher "MATCH (n) DETACH DELETE n"
success "Graph cleared"

# ── Create constraints ─────────────────────────────────────────────────────────
section "Creating constraints"
run_cypher "CREATE CONSTRAINT IF NOT EXISTS FOR (p:Patent) REQUIRE p.patent_id IS UNIQUE"
run_cypher "CREATE CONSTRAINT IF NOT EXISTS FOR (m:Molecule) REQUIRE m.molecule_id IS UNIQUE"
run_cypher "CREATE CONSTRAINT IF NOT EXISTS FOR (a:Assignee) REQUIRE a.name IS UNIQUE"
run_cypher "CREATE CONSTRAINT IF NOT EXISTS FOR (i:Inventor) REQUIRE (i.name, i.patent_id) IS NODE KEY"
run_cypher "CREATE CONSTRAINT IF NOT EXISTS FOR (j:Jurisdiction) REQUIRE j.code IS UNIQUE"
run_cypher "CREATE CONSTRAINT IF NOT EXISTS FOR (c:IPC_Class) REQUIRE c.code IS UNIQUE"
success "Constraints created"

# ── Fetch data from PostgreSQL ──────────────────────────────────────────────────
section "Loading data from PostgreSQL"

PGHOST="${KEYIP_PG_HOST:-localhost}"
PGPORT="${KEYIP_PG_PORT:-5432}"

# Export patents as Cypher statements
PGPASSWORD="keyip_dev" psql -h "$PGHOST" -p "$PGPORT" -U keyip -d keyip_dev -t -A -F'|' -c "
SELECT 
    p.id::text,
    COALESCE(p.patent_number, 'UNKNOWN-' || p.id),
    COALESCE(p.title, 'Untitled'),
    p.jurisdiction,
    COALESCE(p.family_id, p.id::text),
    COALESCE(p.status::text, 'draft'),
    COALESCE(p.assignee_name, 'Unknown Assignee')
FROM patents p;
" 2>/dev/null | while IFS='|' read -r pid pnum title jx family status assignee; do
    [ -z "$pid" ] && continue

    # Create Patent node
    escaped_title=$(echo "$title" | sed "s/'/\\\\'/g")
    escaped_pnum=$(echo "$pnum" | sed "s/'/\\\\'/g")
    escaped_assignee=$(echo "$assignee" | sed "s/'/\\\\'/g")

    run_cypher "
        MERGE (p:Patent {patent_id: '$pid'})
        SET p.patent_number = '$escaped_pnum',
            p.title = '$escaped_title',
            p.status = '$status'
    "

    # Create Jurisdiction node and relationship
    run_cypher "
        MERGE (j:Jurisdiction {code: '$jx'})
        MERGE (p:Patent {patent_id: '$pid'})
        MERGE (p)-[:FILED_IN]->(j)
    "

    # Create Assignee node and relationship
    run_cypher "
        MERGE (a:Assignee {name: '$escaped_assignee'})
        MERGE (p:Patent {patent_id: '$pid'})
        MERGE (a)-[:ASSIGNED_TO {role: 'assignee'}]->(p)
    "

    # Family relationships
    if [ -n "$family" ] && [ "$family" != "\\N" ]; then
        run_cypher "
            MERGE (p:Patent {patent_id: '$pid'})
            SET p.family_id = '$family'
        "
    fi

    echo -n "."
done
echo ""
success "Patents loaded"

# ── Load inventors ──────────────────────────────────────────────────────────────
section "Loading inventors"

PGPASSWORD="keyip_dev" psql -h "$PGHOST" -p "$PGPORT" -U keyip -d keyip_dev -t -A -F'|' -c "
SELECT pi.patent_id::text, pi.inventor_name 
FROM patent_inventors pi JOIN patents p ON p.id = pi.patent_id;
" 2>/dev/null | while IFS='|' read -r pid name; do
    [ -z "$pid" ] && continue
    escaped_name=$(echo "$name" | sed "s/'/\\\\'/g")
    run_cypher "
        MERGE (i:Inventor {name: '$escaped_name', patent_id: '$pid'})
        MERGE (p:Patent {patent_id: '$pid'})
        MERGE (i)-[:INVENTED_BY]->(p)
    "
    echo -n "."
done
echo ""
success "Inventors loaded"

# ── Load molecules & patent-molecule links ──────────────────────────────────────
section "Loading molecules & patent-molecule links"

PGPASSWORD="keyip_dev" psql -h "$PGHOST" -p "$PGPORT" -U keyip -d keyip_dev -t -A -F'|' -c "
SELECT m.id::text, COALESCE(m.name, 'Unnamed'), COALESCE(m.molecular_formula, '')
FROM molecules m;
" 2>/dev/null | while IFS='|' read -r mid name formula; do
    [ -z "$mid" ] && continue
    escaped_name=$(echo "$name" | sed "s/'/\\\\'/g")
    run_cypher "
        MERGE (m:Molecule {molecule_id: '$mid'})
        SET m.name = '$escaped_name',
            m.formula = '$formula'
    "
    echo -n "."
done
echo ""

# Patent-molecule relations
PGPASSWORD="keyip_dev" psql -h "$PGHOST" -p "$PGPORT" -U keyip -d keyip_dev -t -A -F'|' -c "
SELECT patent_id::text, molecule_id::text, relation_type
FROM patent_molecule_relations;
" 2>/dev/null | while IFS='|' read -r pid mid rel_type; do
    [ -z "$pid" ] && continue
    run_cypher "
        MERGE (p:Patent {patent_id: '$pid'})
        MERGE (m:Molecule {molecule_id: '$mid'})
        MERGE (p)-[:CONTAINS_MOLECULE {type: '$rel_type'}]->(m)
    "
    echo -n "."
done
echo ""
success "Molecules & relations loaded"

# ── Load IPC codes ──────────────────────────────────────────────────────────────
section "Loading IPC codes"

PGPASSWORD="keyip_dev" psql -h "$PGHOST" -p "$PGPORT" -U keyip -d keyip_dev -t -A -c "
SELECT DISTINCT unnest(p.ipc_codes), p.id::text
FROM patents p WHERE p.ipc_codes IS NOT NULL AND array_length(p.ipc_codes, 1) > 0;
" 2>/dev/null | while IFS='|' read -r ipc pid; do
    [ -z "$ipc" ] && continue
    section_code=$(echo "$ipc" | cut -c1-4)
    run_cypher "
        MERGE (c:IPC_Class {code: '$ipc'})
        SET c.section = '$section_code'
        MERGE (p:Patent {patent_id: '$pid'})
        MERGE (p)-[:CLASSIFIED_AS]->(c)
    "
    echo -n "."
done
echo ""
success "IPC codes loaded"

# ── Create family relationships ─────────────────────────────────────────────────
section "Creating family relationships"

run_cypher "
    MATCH (p1:Patent), (p2:Patent)
    WHERE p1.family_id = p2.family_id AND p1.patent_id < p2.patent_id
    MERGE (p1)-[:BELONGS_TO_FAMILY]->(p2)
"
success "Family relationships created"

# ── Verify ──────────────────────────────────────────────────────────────────────
section "Verification"

node_counts=$(curl -s -X POST "$N4_HTTP/db/neo4j/tx/commit" \
    -u "$N4_AUTH" \
    -H 'Content-Type: application/json' \
    -d '{"statements": [{"statement": "MATCH (n) RETURN labels(n)[0] AS label, count(n) AS cnt ORDER BY cnt DESC"}]}' | \
    python3 -c "
import sys, json
data = json.load(sys.stdin)
for row in data.get('results',[{}])[0].get('data',[]):
    print(f\"  {row['row'][0]:20s}: {row['row'][1]:4d}\")
" 2>/dev/null || echo "  (verification query failed — graph may still be loaded)")

success "Knowledge graph seeding complete!"
