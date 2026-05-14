#!/usr/bin/env bash
# =============================================================================
# KeyIP-Intelligence: OpenSearch Indexer
# =============================================================================
# Creates search indexes and pushes patent/molecule data from PostgreSQL
# into OpenSearch for full-text search capabilities.
#
# Prerequisites: PostgreSQL seeded, OpenSearch running
# =============================================================================

set -euo pipefail

OS_HOST="${KEYIP_OS_HOST:-localhost}"
OS_PORT="${KEYIP_OS_PORT:-9200}"
OS_USER="${KEYIP_OS_USER:-admin}"
OS_PASS="${KEYIP_OS_PASS:-admin}"
OS_BASE="http://${OS_USER}:${OS_PASS}@${OS_HOST}:${OS_PORT}"

RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m'

section() { echo -e "\n${BLUE}── $* ──${NC}"; }
success() { echo -e "  ${GREEN}✔${NC} $1"; }
warn() { echo -e "  ${RED}✘${NC} $1"; }

# ── Helper: curl with auth ──────────────────────────────────────────────────────
os_put() { curl -s -X PUT "$OS_BASE/$1" -H 'Content-Type: application/json' -d "$2" | python3 -m json.tool 2>/dev/null || true; }
os_post() { curl -s -X POST "$OS_BASE/$1" -H 'Content-Type: application/json' -d "$2" | python3 -m json.tool 2>/dev/null || true; }
os_get() { curl -s "$OS_BASE/$1" | python3 -m json.tool 2>/dev/null || true; }
os_delete() { curl -s -X DELETE "$OS_BASE/$1" 2>/dev/null || true; }

# ── 1. Delete existing indexes (idempotent) ─────────────────────────────────────
section "Cleaning up existing indexes"
for idx in keyip-patents keyip-molecules keyip-lifecycle; do
    echo "  Deleting $idx..."
    os_delete "$idx" >/dev/null
done

# ── 2. Create Patent Index ──────────────────────────────────────────────────────
section "Creating keyip-patents index"
os_put "keyip-patents" '{
  "settings": {
    "number_of_shards": 1,
    "number_of_replicas": 0,
    "analysis": {
      "analyzer": {
        "patent_analyzer": {
          "type": "custom",
          "tokenizer": "standard",
          "filter": ["lowercase", "asciifolding"]
        }
      }
    }
  },
  "mappings": {
    "properties": {
      "patent_id": { "type": "keyword" },
      "patent_number": { "type": "keyword" },
      "title": { "type": "text", "analyzer": "patent_analyzer", "fields": { "raw": { "type": "keyword" } } },
      "abstract": { "type": "text", "analyzer": "patent_analyzer" },
      "title_en": { "type": "text", "analyzer": "patent_analyzer" },
      "abstract_en": { "type": "text", "analyzer": "patent_analyzer" },
      "jurisdiction": { "type": "keyword" },
      "status": { "type": "keyword" },
      "filing_date": { "type": "date", "format": "yyyy-MM-dd" },
      "grant_date": { "type": "date", "format": "yyyy-MM-dd" },
      "ipc_codes": { "type": "keyword" },
      "assignee_name": { "type": "text", "analyzer": "patent_analyzer", "fields": { "raw": { "type": "keyword" } } },
      "family_id": { "type": "keyword" },
      "claims_text": { "type": "text", "analyzer": "patent_analyzer" }
    }
  }
}'
success "keyip-patents index created"

# ── 3. Create Molecule Index ──────────────────────────────────────────────────
section "Creating keyip-molecules index"
os_put "keyip-molecules" '{
  "settings": { "number_of_shards": 1, "number_of_replicas": 0 },
  "mappings": {
    "properties": {
      "molecule_id": { "type": "keyword" },
      "name": { "type": "text", "fields": { "raw": { "type": "keyword" } } },
      "smiles": { "type": "keyword" },
      "molecular_formula": { "type": "keyword" },
      "inchi_key": { "type": "keyword" },
      "molecular_weight": { "type": "double" },
      "logp": { "type": "double" },
      "tpsa": { "type": "double" },
      "status": { "type": "keyword" },
      "num_aromatic_rings": { "type": "integer" },
      "num_rotatable_bonds": { "type": "integer" },
      "aliases": { "type": "keyword" },
      "metadata": { "type": "object", "enabled": false }
    }
  }
}'
success "keyip-molecules index created"

# ── 4. Push Patent Data ─────────────────────────────────────────────────────────
section "Indexing patents from PostgreSQL"

PGHOST="${KEYIP_PG_HOST:-localhost}"
PGPORT="${KEYIP_PG_PORT:-5432}"

# Export patents as JSON and push to OpenSearch
PGPASSWORD="keyip_dev" psql -h "$PGHOST" -p "$PGPORT" -U keyip -d keyip_dev -t -A -c "
SELECT json_build_object(
  'patent_id', p.id::text,
  'patent_number', p.patent_number,
  'title', p.title,
  'abstract', p.abstract,
  'title_en', p.title_en,
  'abstract_en', p.abstract_en,
  'jurisdiction', p.jurisdiction,
  'status', p.status::text,
  'filing_date', p.filing_date,
  'grant_date', p.grant_date,
  'ipc_codes', p.ipc_codes,
  'assignee_name', p.assignee_name,
  'family_id', p.family_id,
  'claims_text', (SELECT string_agg(pc.claim_text, ' ') FROM patent_claims pc WHERE pc.patent_id = p.id)
) FROM patents p;
" 2>/dev/null | while IFS= read -r line; do
    [ -z "$line" ] && continue
    patent_id=$(echo "$line" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d['patent_id'])" 2>/dev/null || true)
    [ -z "$patent_id" ] && continue
    # Skip if patent_id is empty or null
    os_put "keyip-patents/_doc/$patent_id" "$line" >/dev/null
    echo -n "."
done
echo ""
success "Patents indexed"

# ── 5. Push Molecule Data ───────────────────────────────────────────────────────
section "Indexing molecules from PostgreSQL"

PGPASSWORD="keyip_dev" psql -h "$PGHOST" -p "$PGPORT" -U keyip -d keyip_dev -t -A -c "
SELECT json_build_object(
  'molecule_id', m.id::text,
  'name', m.name,
  'smiles', m.canonical_smiles,
  'molecular_formula', m.molecular_formula,
  'inchi_key', m.inchi_key,
  'molecular_weight', m.molecular_weight,
  'logp', m.logp,
  'tpsa', m.tpsa,
  'status', m.status::text,
  'num_aromatic_rings', m.num_aromatic_rings,
  'num_rotatable_bonds', m.num_rotatable_bonds,
  'aliases', m.aliases
) FROM molecules m;
" 2>/dev/null | while IFS= read -r line; do
    [ -z "$line" ] && continue
    mol_id=$(echo "$line" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d['molecule_id'])" 2>/dev/null || true)
    [ -z "$mol_id" ] && continue
    os_put "keyip-molecules/_doc/$mol_id" "$line" >/dev/null
    echo -n "."
done
echo ""
success "Molecules indexed"

# ── 6. Verify ────────────────────────────────────────────────────────────────────
section "Verification"
doc_count=$(curl -s "$OS_BASE/keyip-patents/_count" | python3 -c "import sys,json; print(json.load(sys.stdin)['count'])" 2>/dev/null || echo "0")
success "keyip-patents: $doc_count documents"

doc_count=$(curl -s "$OS_BASE/keyip-molecules/_count" | python3 -c "import sys,json; print(json.load(sys.stdin)['count'])" 2>/dev/null || echo "0")
success "keyip-molecules: $doc_count documents"

echo ""
echo -e "${GREEN}OpenSearch indexing complete!${NC}"
