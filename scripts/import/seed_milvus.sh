#!/usr/bin/env bash
# =============================================================================
# KeyIP-Intelligence: Milvus Vector Seeder
# =============================================================================
# Creates Milvus collections and inserts vector embeddings for molecules
# and patent claims. Uses random vectors as placeholder embeddings
# (real embeddings require the ML model pipeline).
#
# Prerequisites: Milvus running on localhost:19530
# =============================================================================

set -euo pipefail

MV_HOST="${KEYIP_MV_HOST:-localhost}"
MV_PORT="${KEYIP_MV_PORT:-19530}"
MV_BASE="http://${MV_HOST}:${MV_PORT}/v2/vectordb"

BLUE='\033[0;34m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

section() { echo -e "\n${BLUE}── $* ──${NC}"; }
success() { echo -e "  ${GREEN}✔${NC} $1"; }
warn() { echo -e "  ${YELLOW}⚠${NC} $1"; }

# ── Check Milvus connectivity ───────────────────────────────────────────────────
section "Checking Milvus connection"
if curl -s "http://${MV_HOST}:${MV_PORT}/api/v1/health" >/dev/null 2>&1; then
    success "Milvus is reachable at $MV_HOST:$MV_PORT"
else
    warn "Milvus is not reachable — skipping vector seeding"
    echo "  Start with: docker compose -f deployments/docker/docker-compose.yml up -d milvus-standalone"
    exit 0
fi

# ── Create molecule_vectors collection ──────────────────────────────────────────
section "Creating molecule_vectors collection (512-dim)"

curl -s -X POST "$MV_BASE/collections/create" \
    -H 'Content-Type: application/json' \
    -d '{
        "collectionName": "molecule_vectors",
        "dimension": 512,
        "metricType": "COSINE",
        "primaryField": "id",
        "vectorField": "vector",
        "schema": {
            "fields": [
                {"fieldName": "id", "dataType": "Int64", "isPrimary": true, "autoID": true},
                {"fieldName": "molecule_id", "dataType": "VarChar", "maxLength": 64},
                {"fieldName": "vector", "dataType": "FloatVector", "dimension": 512}
            ]
        }
    }' >/dev/null 2>&1
success "molecule_vectors collection created (may show 'already exists')"

# ── Insert placeholder vectors for each molecule from PostgreSQL ─────────────────
section "Inserting molecule vectors"

PGHOST="${KEYIP_PG_HOST:-localhost}"
PGPORT="${KEYIP_PG_PORT:-5432}"

# Get molecule IDs from PG
molecule_ids=$(PGPASSWORD="keyip_dev" psql -h "$PGHOST" -p "$PGPORT" -U keyip -d keyip_dev \
    -t -A -c "SELECT id::text FROM molecules LIMIT 100;" 2>/dev/null || echo "")

if [ -z "$molecule_ids" ]; then
    warn "No molecules found in PostgreSQL — cannot insert vectors"
    exit 0
fi

# Generate random 512-dim float vector
gen_vector() {
    python3 -c "
import json, random
vector = [random.uniform(-1, 1) for _ in range(512)]
print(json.dumps(vector))
"
}

count=0
for mol_id in $molecule_ids; do
    vector=$(gen_vector)
    curl -s -X POST "$MV_BASE/entities/insert" \
        -H 'Content-Type: application/json' \
        -d "{
            \"collectionName\": \"molecule_vectors\",
            \"data\": [{
                \"molecule_id\": \"$mol_id\",
                \"vector\": $vector
            }]
        }" >/dev/null 2>&1
    count=$((count + 1))
    echo -n "."
done
echo ""
success "Inserted $count molecule vectors"

# ── Create claim_vectors collection ─────────────────────────────────────────────
section "Creating claim_vectors collection (768-dim)"

curl -s -X POST "$MV_BASE/collections/create" \
    -H 'Content-Type: application/json' \
    -d '{
        "collectionName": "claim_vectors",
        "dimension": 768,
        "metricType": "COSINE",
        "primaryField": "id",
        "vectorField": "vector",
        "schema": {
            "fields": [
                {"fieldName": "id", "dataType": "Int64", "isPrimary": true, "autoID": true},
                {"fieldName": "claim_id", "dataType": "VarChar", "maxLength": 64},
                {"fieldName": "patent_id", "dataType": "VarChar", "maxLength": 64},
                {"fieldName": "vector", "dataType": "FloatVector", "dimension": 768}
            ]
        }
    }' >/dev/null 2>&1
success "claim_vectors collection created (may show 'already exists')"

# ── Insert claim vectors ────────────────────────────────────────────────────────
section "Inserting claim vectors"

gen_vector_768() {
    python3 -c "
import json, random
vector = [random.uniform(-1, 1) for _ in range(768)]
print(json.dumps(vector))
"
}

claim_count=0
PGPASSWORD="keyip_dev" psql -h "$PGHOST" -p "$PGPORT" -U keyip -d keyip_dev \
    -t -A -F'|' -c "SELECT pc.id::text, pc.patent_id::text FROM patent_claims pc LIMIT 200;" 2>/dev/null | \
while IFS='|' read -r claim_id patent_id; do
    [ -z "$claim_id" ] && continue
    vector=$(gen_vector_768)
    curl -s -X POST "$MV_BASE/entities/insert" \
        -H 'Content-Type: application/json' \
        -d "{
            \"collectionName\": \"claim_vectors\",
            \"data\": [{
                \"claim_id\": \"$claim_id\",
                \"patent_id\": \"$patent_id\",
                \"vector\": $vector
            }]
        }" >/dev/null 2>&1
    echo -n "."
done
echo ""
success "Inserted claim vectors"

# ── Create index ────────────────────────────────────────────────────────────────
section "Creating indexes (IVF_FLAT)"

curl -s -X POST "$MV_BASE/indexes/create" \
    -H 'Content-Type: application/json' \
    -d '{
        "collectionName": "molecule_vectors",
        "indexParams": [
            {"fieldName": "vector", "indexName": "vector_idx", "indexType": "IVF_FLAT",
             "metricType": "COSINE", "params": "{\"nlist\": 128}"}
        ]
    }' >/dev/null 2>&1

curl -s -X POST "$MV_BASE/indexes/create" \
    -H 'Content-Type: application/json' \
    -d '{
        "collectionName": "claim_vectors",
        "indexParams": [
            {"fieldName": "vector", "indexName": "vector_idx", "indexType": "IVF_FLAT",
             "metricType": "COSINE", "params": "{\"nlist\": 128}"}
        ]
    }' >/dev/null 2>&1
success "Indexes created"

# ── Load collections into memory ────────────────────────────────────────────────
section "Loading collections into memory"

curl -s -X POST "$MV_BASE/collections/load" \
    -H 'Content-Type: application/json' \
    -d '{"collectionName": "molecule_vectors"}' >/dev/null 2>&1

curl -s -X POST "$MV_BASE/collections/load" \
    -H 'Content-Type: application/json' \
    -d '{"collectionName": "claim_vectors"}' >/dev/null 2>&1
success "Collections loaded into memory"

echo ""
echo -e "${GREEN}Milvus vector seeding complete!${NC}"
echo -e "${YELLOW}Note: Random placeholder vectors used. Real embeddings require the ML model pipeline.${NC}"
