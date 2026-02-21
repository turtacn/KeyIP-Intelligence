#!/usr/bin/env bash
set -euo pipefail

# KeyIP-Intelligence - Test Execution and Reporting Script

LEVEL="unit"
COVERAGE=false
RACE=false
VERBOSE=false
PACKAGE="./..."
TIMEOUT="10m"

usage() {
  echo "Usage: $0 [options]"
  echo "Options:"
  echo "  --level [unit|integration|e2e|all] (default: unit)"
  echo "  --coverage                        (default: false)"
  echo "  --race                            (default: false)"
  echo "  --verbose                         (default: false)"
  echo "  --package <path>                  (default: ./...)"
  echo "  --timeout <duration>              (default: 10m)"
  exit 1
}

while [[ $# -gt 0 ]]; do
  case $1 in
    --level)    LEVEL="$2"; shift 2 ;;
    --coverage) COVERAGE=true; shift ;;
    --race)     RACE=true; shift ;;
    --verbose)  VERBOSE=true; shift ;;
    --package)  PACKAGE="$2"; shift 2 ;;
    --timeout)  TIMEOUT="$2"; shift 2 ;;
    --help)     usage ;;
    *)          echo "Unknown argument: $1"; usage ;;
  esac
done

FLAGS="-timeout ${TIMEOUT} -count=1"
if [ "${RACE}" = true ]; then FLAGS="${FLAGS} -race"; fi
if [ "${VERBOSE}" = true ]; then FLAGS="${FLAGS} -v"; fi
if [ "${COVERAGE}" = true ]; then
  FLAGS="${FLAGS} -coverprofile=coverage.out -covermode=atomic"
fi

run_suite() {
  local suite_tags=$1
  echo ">> Running ${suite_tags} tests..."
  go test -tags="${suite_tags}" ${FLAGS} "${PACKAGE}"
}

case ${LEVEL} in
  unit)
    run_suite "unit"
    ;;
  integration)
    run_suite "integration"
    ;;
  e2e)
    run_suite "e2e"
    ;;
  all)
    run_suite "unit"
    run_suite "integration"
    run_suite "e2e"
    ;;
  *)
    echo "Unknown level: ${LEVEL}"; usage ;;
esac

if [ "${COVERAGE}" = true ] && [ -f "coverage.out" ]; then
  echo ">> Generating HTML coverage report..."
  go tool cover -html=coverage.out -o coverage.html
fi

echo "Test execution completed successfully."

# //Personal.AI order the ending
