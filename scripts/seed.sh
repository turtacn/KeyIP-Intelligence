#!/usr/bin/env bash
set -euo pipefail

# KeyIP-Intelligence - Seed Data Import Script

DATA_DIR="test/testdata"
TARGET="all"
CLEAN=false

usage() {
  echo "Usage: $0 [options]"
  echo "Options:"
  echo "  --target [postgres|neo4j|opensearch|milvus|all] (default: all)"
  echo "  --data-dir <dir>                               (default: test/testdata)"
  echo "  --clean                                        (default: false)"
  exit 1
}

while [[ $# -gt 0 ]]; do
  case $1 in
    --target)   TARGET="$2"; shift 2 ;;
    --data-dir) DATA_DIR="$2"; shift 2 ;;
    --clean)    CLEAN=true; shift ;;
    --help)     usage ;;
    *)          echo "Unknown argument: $1"; usage ;;
  esac
done

seed_postgres() {
  echo ">> Seeding PostgreSQL..."
  # Placeholder for real seeding logic, e.g., psql -f ${DATA_DIR}/postgres_seed.sql
}

seed_neo4j() {
  echo ">> Seeding Neo4j..."
  # Placeholder for real seeding logic, e.g., cypher-shell -f ${DATA_DIR}/neo4j_seed.cypher
}

seed_opensearch() {
  echo ">> Seeding OpenSearch..."
}

seed_milvus() {
  echo ">> Seeding Milvus..."
}

if [ "${CLEAN}" = true ]; then
  echo ">> Cleaning target data stores before seeding..."
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

echo "Seeding process completed."

# //Personal.AI order the ending
