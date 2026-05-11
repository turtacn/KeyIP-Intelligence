#!/usr/bin/env bash
set -euo pipefail

# KeyIP-Intelligence - Seed Data Import Script
# Loads fixture data from test/testdata into the configured databases.
#
# Usage:
#   ./scripts/seed.sh [options]
#
# Options:
#   --target [postgres|neo4j|opensearch|milvus|all]   (default: all)
#   --data-dir <dir>                                   (default: test/testdata)
#   --clean                                            drop & recreate before seeding (default: false)
#   --config  <path>                                   path to config YAML (default: configs/config.yaml)
#   --help                                             show this message

DATA_DIR="test/testdata"
TARGET="all"
CLEAN=false
CONFIG_FILE="configs/config.yaml"

usage() {
  echo "Usage: $0 [options]"
  echo ""
  echo "Load fixture data from test/testdata into databases for local development."
  echo ""
  echo "Options:"
  echo "  --target [postgres|neo4j|opensearch|milvus|all]   (default: all)"
  echo "  --data-dir <dir>                                   (default: test/testdata)"
  echo "  --config  <path>                                   (default: configs/config.yaml)"
  echo "  --clean                                            drop existing data before seeding"
  echo "  --help                                             show this message"
  echo ""
  echo "Examples:"
  echo "  $0 --target postgres          # seed only PostgreSQL"
  echo "  $0 --target all               # seed every data store"
  echo "  $0 --clean --target milvus    # re-seed Milvus from scratch"
  exit 0
}

while [[ $# -gt 0 ]]; do
  case $1 in
    --target)   TARGET="$2";   shift 2 ;;
    --data-dir) DATA_DIR="$2"; shift 2 ;;
    --config)   CONFIG_FILE="$2"; shift 2 ;;
    --clean)    CLEAN=true;    shift ;;
    --help)     usage ;;
    *)          echo "Unknown argument: $1"; usage ;;
  esac
done

# ---------------------------------------------------------------------------
# Paths
# ---------------------------------------------------------------------------
FIXTURES_DIR="${DATA_DIR}/fixtures"
MOLECULES_DIR="${DATA_DIR}/molecules"
PATENTS_DIR="${DATA_DIR}/patents"

MOLECULE_FIXTURES="${FIXTURES_DIR}/molecule_fixtures.json"
PATENT_FIXTURES="${FIXTURES_DIR}/patent_fixtures.json"
PORTFOLIO_FIXTURES="${FIXTURES_DIR}/portfolio_fixtures.json"

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------
info()  { printf "\e[34m>>\e[0m %s\n" "$*"; }
ok()    { printf "\e[32m>>\e[0m %s\n" "$*"; }
warn()  { printf "\e[33m>>\e[0m %s\n" "$*"; }
err()   { printf "\e[31m>>\e[0m %s\n" "$*"; }

# Ensure required fixture files exist before we start.
require_fixtures() {
  local missing=0
  for f in "$MOLECULE_FIXTURES" "$PATENT_FIXTURES" "$PORTFOLIO_FIXTURES"; do
    if [ ! -f "$f" ]; then
      err "Missing fixture: $f"
      missing=1
    fi
  done
  if [ "$missing" -eq 1 ]; then
    err "One or more fixture files are missing. Did you run from the project root?"
    exit 1
  fi
  ok "All fixture files present."
}

# Extract a value from config.yaml using grep/awk.
# Usage: config_val <yaml_path> e.g. config_val database.postgres.host
config_val() {
  local key="$1"
  # Flatten nested key into whitespace-indented lookup
  # e.g. "database.postgres.host" matches lines under "postgres:" then "host:"
  awk -v k="$key" '
    BEGIN { split(k, parts, "."); level=0; }
    /^[a-z]/ { gsub(/:/,""); current=$1; }
    /^[a-z]/ && current==parts[1] { found=1; next }
    found && /^  [a-z]/ { gsub(/:/,""); sub(/^  /,""); current=$1; }
    found && current==parts[2] && /^    [a-z]/ { gsub(/:/,""); sub(/^    /,""); current=$1; }
    found && current==parts[3] && $1 ~ parts[3]":" { gsub(/^ */,""); sub(/^[a-z_]+: /,""); print; exit; }
  ' "$CONFIG_FILE"
}

# ---------------------------------------------------------------------------
# PostgreSQL
# ---------------------------------------------------------------------------
seed_postgres() {
  info "Seeding PostgreSQL..."
  local host port user password dbname
  host=$(config_val database.postgres.host)     || host="localhost"
  port=$(config_val database.postgres.port)     || port="5432"
  user=$(config_val database.postgres.user)     || user="keyip"
  password=$(config_val database.postgres.password) || password="keyip_dev"
  dbname=$(config_val database.postgres.dbname) || dbname="keyip_dev"

  export PGHOST="$host" PGPORT="$port" PGUSER="$user" PGPASSWORD="$password" PGDATABASE="$dbname"

  if ! psql -c "SELECT 1" >/dev/null 2>&1; then
    warn "PostgreSQL not reachable at $host:$port. Skipping."
    return
  fi

  if [ "${CLEAN}" = true ]; then
    info "  Cleaning existing data..."
    psql -c "TRUNCATE TABLE molecules, patents, portfolios, patent_claims, patent_inventors CASCADE" 2>/dev/null || true
  fi

  # Seed molecules fixture
  info "  Importing molecules..."
  if command -v jq >/dev/null 2>&1; then
    jq -c '.molecules[]' "$MOLECULE_FIXTURES" | while read -r mol; do
      id=$(echo "$mol" | jq -r '.id')
      name=$(echo "$mol" | jq -r '.name' | sed "s/'/''/g")
      smiles=$(echo "$mol" | jq -r '.smiles' | sed "s/'/''/g")
      formula=$(echo "$mol" | jq -r '.molecular_formula // ""' | sed "s/'/''/g")
      psql -c "INSERT INTO molecules (id, name, smiles, molecular_formula, metadata)
               VALUES ('$id', '$name', '$smiles', '$formula', '$(echo "$mol" | sed "s/'/''/g")')
               ON CONFLICT (id) DO UPDATE SET metadata = EXCLUDED.metadata;" 2>/dev/null || true
    done
  else
    warn "  jq not found; install jq for structured seeding. Falling back to bulk copy."
    # Minimal fallback: copy the whole file as reference
    psql -c "CREATE TABLE IF NOT EXISTS _seed_ref (fixture_name text, metadata jsonb);" 2>/dev/null
    psql -c "TRUNCATE _seed_ref;" 2>/dev/null
    psql -c "INSERT INTO _seed_ref (fixture_name, metadata) VALUES ('molecule_fixtures', '$(cat "$MOLECULE_FIXTURES" | sed "s/'/''/g")');" 2>/dev/null || true
  fi

  # Seed patents fixture
  info "  Importing patents..."
  if command -v jq >/dev/null 2>&1; then
    jq -c '.patents[]' "$PATENT_FIXTURES" | while read -r pat; do
      pat_id=$(echo "$pat" | jq -r '.id // ""')
      pat_num=$(echo "$pat" | jq -r '.patent_number // ""' | sed "s/'/''/g")
      title=$(echo "$pat" | jq -r '.title // ""' | sed "s/'/''/g")
      psql -c "INSERT INTO patents (id, patent_number, title, raw_data)
               VALUES ('$pat_id', '$pat_num', '$title', '$(echo "$pat" | sed "s/'/''/g")')
               ON CONFLICT (id) DO UPDATE SET raw_data = EXCLUDED.raw_data;" 2>/dev/null || true
    done
  else
    psql -c "INSERT INTO _seed_ref (fixture_name, raw_data) VALUES ('patent_fixtures', '$(cat "$PATENT_FIXTURES" | sed "s/'/''/g")');" 2>/dev/null || true
  fi

  ok "PostgreSQL seeding completed."
}

# ---------------------------------------------------------------------------
# Neo4j
# ---------------------------------------------------------------------------
seed_neo4j() {
  info "Seeding Neo4j..."
  local uri user password
  uri=$(config_val database.neo4j.uri)      || uri="bolt://localhost:7687"
  user=$(config_val database.neo4j.user)    || user="neo4j"
  password=$(config_val database.neo4j.password) || password="neo4j_dev"

  if ! command -v cypher-shell >/dev/null 2>&1; then
    warn "cypher-shell not found. Skipping Neo4j."
    return
  fi

  CYPHER="cypher-shell -a $uri -u $user -p $password"

  if ! $CYPHER "RETURN 1" >/dev/null 2>&1; then
    warn "Neo4j not reachable at $uri. Skipping."
    return
  fi

  if [ "${CLEAN}" = true ]; then
    info "  Cleaning existing data..."
    $CYPHER "MATCH (n) DETACH DELETE n" 2>/dev/null || true
  fi

  # Create constraints (idempotent)
  $CYPHER "CREATE CONSTRAINT mol_id IF NOT EXISTS FOR (m:Molecule) REQUIRE m.id IS UNIQUE" 2>/dev/null || true
  $CYPHER "CREATE CONSTRAINT pat_id IF NOT EXISTS FOR (p:Patent) REQUIRE p.id IS UNIQUE" 2>/dev/null || true

  # Load molecules as nodes
  if command -v jq >/dev/null 2>&1; then
    jq -c '.molecules[]' "$MOLECULE_FIXTURES" | while read -r mol; do
      mid=$(echo "$mol" | jq -r '.id')
      mname=$(echo "$mol" | jq -r '.name' | sed "s/'/\\\\'/g")
      $CYPHER "MERGE (m:Molecule {id: '$mid'}) SET m.name = '$mname'" 2>/dev/null || true
    done

    # Load patents as nodes
    jq -c '.patents[]' "$PATENT_FIXTURES" | while read -r pat; do
      pid=$(echo "$pat" | jq -r '.id // ""')
      pnum=$(echo "$pat" | jq -r '.patent_number // ""' | sed "s/'/\\\\'/g")
      [ -z "$pid" ] && continue
      $CYPHER "MERGE (p:Patent {id: '$pid'}) SET p.patent_number = '$pnum'" 2>/dev/null || true
    done

    # Create relationships between patents and molecules (where referenced)
    jq -c '.patents[] | select(.molecules != null)' "$PATENT_FIXTURES" | while read -r pat; do
      pid=$(echo "$pat" | jq -r '.id // ""')
      [ -z "$pid" ] && continue
      echo "$pat" | jq -c '.molecules[]' 2>/dev/null | while read -r mref; do
        mid=$(echo "$mref" | jq -r '.molecule_id // ""')
        role=$(echo "$mref" | jq -r '.role // "related"' | sed "s/'/\\\\'/g")
        [ -z "$mid" ] && continue
        $CYPHER "MATCH (p:Patent {id: '$pid'}), (m:Molecule {id: '$mid'}) MERGE (p)-[:REFERENCES {role: '$role'}]->(m)" 2>/dev/null || true
      done
    done
  else
    warn "  jq not found. Loading molecules as bulk JSON property."
    # Read the entire molecule_fixtures as a single property on a reference node
    local mol_json
    mol_json=$(cat "$MOLECULE_FIXTURES" | sed "s/'/\\\\'/g")
    $CYPHER "MERGE (r:SeedReference {name: 'molecule_fixtures'}) SET r.json = '$mol_json'" 2>/dev/null || true
    local pat_json
    pat_json=$(cat "$PATENT_FIXTURES" | sed "s/'/\\\\'/g")
    $CYPHER "MERGE (r:SeedReference {name: 'patent_fixtures'}) SET r.json = '$pat_json'" 2>/dev/null || true
  fi

  ok "Neo4j seeding completed."
}

# ---------------------------------------------------------------------------
# OpenSearch
# ---------------------------------------------------------------------------
seed_opensearch() {
  info "Seeding OpenSearch..."
  local addr user password
  addr=$(grep -A3 'opensearch:' "$CONFIG_FILE" | grep 'addresses' | head -1 | awk -F'"' '{print $2}' 2>/dev/null) || addr="http://localhost:9200"
  user=$(config_val search.opensearch.username)     || user="admin"
  password=$(config_val search.opensearch.password) || password="admin"

  if ! command -v curl >/dev/null 2>&1; then
    warn "curl not found. Skipping OpenSearch."
    return
  fi

  if ! curl -sf -u "$user:$password" "$addr" >/dev/null 2>&1; then
    warn "OpenSearch not reachable at $addr. Skipping."
    return
  fi

  if [ "${CLEAN}" = true ]; then
    info "  Deleting existing indices..."
    curl -sf -u "$user:$password" -X DELETE "$addr/molecules" >/dev/null 2>&1 || true
    curl -sf -u "$user:$password" -X DELETE "$addr/patents" >/dev/null 2>&1 || true
    curl -sf -u "$user:$password" -X DELETE "$addr/portfolios" >/dev/null 2>&1 || true
  fi

  # Create index with mapping for molecules
  info "  Creating molecule index..."
  curl -sf -u "$user:$password" -X PUT "$addr/molecules" -H "Content-Type: application/json" -d '{
    "settings": { "number_of_shards": 1, "number_of_replicas": 0 },
    "mappings": {
      "properties": {
        "id":         { "type": "keyword" },
        "name":       { "type": "text", "fields": { "keyword": { "type": "keyword" } } },
        "smiles":     { "type": "text" },
        "inchi_key":  { "type": "keyword" },
        "molecular_formula": { "type": "keyword" },
        "status":     { "type": "keyword" },
        "category":   { "type": "keyword" }
      }
    }
  }' >/dev/null 2>&1 || true

  # Create index for patents
  info "  Creating patent index..."
  curl -sf -u "$user:$password" -X PUT "$addr/patents" -H "Content-Type: application/json" -d '{
    "settings": { "number_of_shards": 1, "number_of_replicas": 0 },
    "mappings": {
      "properties": {
        "id":            { "type": "keyword" },
        "patent_number": { "type": "keyword" },
        "title":         { "type": "text" },
        "abstract":      { "type": "text" },
        "filing_date":   { "type": "date" },
        "grant_date":    { "type": "date" },
        "legal_status":  { "type": "keyword" },
        "jurisdiction":  { "type": "keyword" }
      }
    }
  }' >/dev/null 2>&1 || true

  # Bulk-index molecules
  if command -v jq >/dev/null 2>&1; then
    info "  Indexing molecules..."
    local mol_count
    mol_count=$(jq '.molecules | length' "$MOLECULE_FIXTURES")

    # Build bulk payload
    jq -c '.molecules[] | {index: {_index: "molecules", _id: .id}}, .' "$MOLECULE_FIXTURES" > /tmp/_os_mol_bulk.json 2>/dev/null || true
    if [ -s /tmp/_os_mol_bulk.json ]; then
      curl -sf -u "$user:$password" -X POST "$addr/_bulk" \
        -H "Content-Type: application/json" \
        --data-binary @/tmp/_os_mol_bulk.json >/dev/null 2>&1 || warn "  Bulk index molecules failed (OpenSearch may be in single-node mode requiring explicit settings)."
      rm -f /tmp/_os_mol_bulk.json
    fi

    info "  Indexing patents..."
    jq -c '.patents[] | {index: {_index: "patents", _id: .id}}, .' "$PATENT_FIXTURES" > /tmp/_os_pat_bulk.json 2>/dev/null || true
    if [ -s /tmp/_os_pat_bulk.json ]; then
      curl -sf -u "$user:$password" -X POST "$addr/_bulk" \
        -H "Content-Type: application/json" \
        --data-binary @/tmp/_os_pat_bulk.json >/dev/null 2>&1 || warn "  Bulk index patents failed."
      rm -f /tmp/_os_pat_bulk.json
    fi

    info "  Indexing portfolios..."
    jq -c '.portfolios[] | {index: {_index: "portfolios", _id: .id}}, .' "$PORTFOLIO_FIXTURES" > /tmp/_os_prt_bulk.json 2>/dev/null || true
    if [ -s /tmp/_os_prt_bulk.json ]; then
      curl -sf -u "$user:$password" -X POST "$addr/_bulk" \
        -H "Content-Type: application/json" \
        --data-binary @/tmp/_os_prt_bulk.json >/dev/null 2>&1 || warn "  Bulk index portfolios failed."
      rm -f /tmp/_os_prt_bulk.json
    fi

    # Refresh indices
    curl -sf -u "$user:$password" -X POST "$addr/_refresh" >/dev/null 2>&1 || true
    ok "OpenSearch seeding completed ($mol_count molecules, plus patents and portfolios)."
  else
    warn "  jq not found. Skipping OpenSearch indexing."
  fi
}

# ---------------------------------------------------------------------------
# Milvus
# ---------------------------------------------------------------------------
seed_milvus() {
  info "Seeding Milvus..."
  local milvus_addr milvus_port
  milvus_addr=$(config_val search.milvus.address) || milvus_addr="localhost"
  milvus_port=$(config_val search.milvus.port)     || milvus_port="19530"

  if ! command -v pymilvus >/dev/null 2>&1 && ! python3 -c "import pymilvus" 2>/dev/null; then
    warn "pymilvus Python package not available. Trying grpcurl health check..."
    if ! command -v grpcurl >/dev/null 2>&1; then
      warn "  Neither pymilvus nor grpcurl found. Skipping Milvus."
      warn "  Install with: pip install pymilvus"
      return
    fi
    if ! grpcurl -plaintext "$milvus_addr:$milvus_port" milvus.proto.milvus.MilvusService/Check >/dev/null 2>&1; then
      warn "Milvus not reachable at $milvus_addr:$milvus_port. Skipping."
      return
    fi
  fi

  # Use a Python script for Milvus seeding (the SDK is the most reliable approach)
  python3 -c "
import json, sys, os

try:
    from pymilvus import connections, utility, Collection, CollectionSchema, FieldSchema, DataType
except ImportError:
    print('>> pymilvus not installed. Skipping Milvus.')
    sys.exit(0)

HOST = '$milvus_addr'
PORT = '$milvus_port'
MOL_FILE = '$MOLECULE_FIXTURES'

try:
    connections.connect(host=HOST, port=PORT)
except Exception as e:
    print(f'>> Cannot connect to Milvus at {HOST}:{PORT}: {e}')
    sys.exit(0)

COLL_NAME = 'molecule_fingerprints'

exists = utility.has_collection(COLL_NAME)
if exists and '$CLEAN' == 'true':
    utility.drop_collection(COLL_NAME)
    exists = False
    print('>> Dropped existing collection.')

if not exists:
    fields = [
        FieldSchema(name='id', dtype=DataType.VARCHAR, max_length=64, is_primary=True),
        FieldSchema(name='name', dtype=DataType.VARCHAR, max_length=256),
        FieldSchema(name='smiles', dtype=DataType.VARCHAR, max_length=1024),
        FieldSchema(name='molecular_formula', dtype=DataType.VARCHAR, max_length=64),
        FieldSchema(name='embedding', dtype=DataType.FLOAT_VECTOR, dim=128),
    ]
    schema = CollectionSchema(fields, description='Molecule fingerprint vectors')
    collection = Collection(name=COLL_NAME, schema=schema)
    index_params = {'metric_type': 'COSINE', 'index_type': 'IVF_FLAT', 'params': {'nlist': 128}}
    collection.create_index(field_name='embedding', index_params=index_params)
    print(f'>> Created collection {COLL_NAME}.')

collection = Collection(name=COLL_NAME)
collection.load()

with open(MOL_FILE) as f:
    data = json.load(f)

molecules = data.get('molecules', [])
entities = []
for mol in molecules:
    # Use a simple deterministic mock embedding (128 floats) based on the molecule id hash
    import hashlib
    h = hashlib.sha256(mol['id'].encode()).hexdigest()
    seed_vals = [int(h[i:i+2], 16) for i in range(0, min(64, len(h)), 2)]
    emb = [(v / 255.0) for v in seed_vals]
    # Pad or truncate to 128
    while len(emb) < 128:
        emb.extend(emb[:8])
    emb = emb[:128]

    entities.append([
        mol.get('id', ''),
        mol.get('name', ''),
        mol.get('smiles', ''),
        mol.get('molecular_formula', ''),
        emb,
    ])

if entities:
    collection.insert(entities)
    collection.flush()
    print(f'>> Inserted {len(entities)} molecule vectors into Milvus.')
else:
    print('>> No molecules to insert.')
" 2>&1 || warn "Milvus seeding encountered an issue (non-fatal)."

  ok "Milvus seeding completed."
}

# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------

require_fixtures

# Check config file exists
if [ ! -f "$CONFIG_FILE" ]; then
  warn "Config file not found at $CONFIG_FILE. Using defaults."
fi

if [ "${CLEAN}" = true ]; then
  info "Clean mode enabled: existing data will be dropped before seeding."
fi

case ${TARGET} in
  postgres)   seed_postgres ;;
  neo4j)      seed_neo4j ;;
  opensearch) seed_opensearch ;;
  milvus)     seed_milvus ;;
  all)
    seed_postgres
    seed_neo4j
    seed_opensearch
    seed_milvus
    ;;
  *) echo "Unknown target: ${TARGET}"; usage ;;
esac

echo ""
ok "Seeding process completed."
echo ""
echo "Summary:"
echo "  Fixture files used:"
echo "    Molecules:  $MOLECULE_FIXTURES"
echo "    Patents:    $PATENT_FIXTURES"
echo "    Portfolios: $PORTFOLIO_FIXTURES"
echo ""
echo "  For full patent documents (CN/EP/US), see:"
echo "    $PATENTS_DIR/"
echo ""
echo "  For SMILES datasets, see:"
echo "    $MOLECULES_DIR/"
echo ""

# //Personal.AI order the ending
