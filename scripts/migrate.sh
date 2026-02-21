#!/usr/bin/env bash
set -euo pipefail

# KeyIP-Intelligence - Database Migration Management Script

MIGRATIONS_DIR="internal/infrastructure/database/postgres/migrations"
DB_URL=${DATABASE_URL:-""}

# Try to extract DB_URL from config.yaml if not set
if [ -z "${DB_URL}" ] && [ -f "configs/config.yaml" ]; then
  # Note: This is a simplistic extractor. Real use might need yq or similar.
  HOST=$(grep -A 10 "postgres:" configs/config.yaml | grep "host:" | awk '{print $2}' | tr -d '"' || true)
  PORT=$(grep -A 10 "postgres:" configs/config.yaml | grep "port:" | awk '{print $2}' | tr -d '"' || true)
  USER=$(grep -A 10 "postgres:" configs/config.yaml | grep "user:" | awk '{print $2}' | tr -d '"' || true)
  PASS=$(grep -A 10 "postgres:" configs/config.yaml | grep "password:" | awk '{print $2}' | tr -d '"' || true)
  DBNAME=$(grep -A 10 "postgres:" configs/config.yaml | grep "dbname:" | awk '{print $2}' | tr -d '"' || true)

  if [ -n "${HOST}" ] && [ -n "${DBNAME}" ]; then
    DB_URL="postgres://${USER}:${PASS}@${HOST}:${PORT}/${DBNAME}?sslmode=disable"
  fi
fi

COMMAND=${1:-"help"}

check_migrate() {
  if ! command -v migrate &> /dev/null; then
    echo "Error: 'migrate' tool not found. Please install it or use 'make tools'."
    exit 1
  fi
}

case ${COMMAND} in
  up)
    check_migrate
    echo "Applying all pending migrations..."
    migrate -path "${MIGRATIONS_DIR}" -database "${DB_URL}" up
    ;;
  down)
    check_migrate
    echo "Rolling back last migration..."
    migrate -path "${MIGRATIONS_DIR}" -database "${DB_URL}" down 1
    ;;
  down-all)
    check_migrate
    echo "Rolling back all migrations..."
    migrate -path "${MIGRATIONS_DIR}" -database "${DB_URL}" down -all
    ;;
  status)
    check_migrate
    echo "Current migration status:"
    migrate -path "${MIGRATIONS_DIR}" -database "${DB_URL}" version
    ;;
  create)
    check_migrate
    NAME=${2:-"new_migration"}
    echo "Creating migration: ${NAME}"
    migrate create -ext sql -dir "${MIGRATIONS_DIR}" -seq "${NAME}"
    ;;
  force)
    check_migrate
    VERSION=${2:-""}
    if [ -z "${VERSION}" ]; then echo "Error: VERSION required"; exit 1; fi
    echo "Forcing migration version to ${VERSION}..."
    migrate -path "${MIGRATIONS_DIR}" -database "${DB_URL}" force "${VERSION}"
    ;;
  help|*)
    echo "Usage: $0 {up|down|down-all|status|create NAME|force VERSION}"
    exit 1
    ;;
esac

# //Personal.AI order the ending
