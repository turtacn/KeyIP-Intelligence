#!/usr/bin/env bash
set -euo pipefail

# =============================================================================
# KeyIP-Intelligence - Test Coverage Report Generator
# =============================================================================
# Runs all tests with coverage profiling, generates an HTML report, and
# prints per-package coverage statistics.
# =============================================================================

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

COVERAGE_FILE="coverage.out"
COVERAGE_HTML="coverage.html"
TIMEOUT="300s"

usage() {
  echo "Usage: $0 [options]"
  echo "Options:"
  echo "  --profile    Coverage output file (default: coverage.out)"
  echo "  --html       HTML report output file (default: coverage.html)"
  echo "  --timeout    Test timeout (default: 300s)"
  echo "  --help       Show this help"
  exit 1
}

while [[ $# -gt 0 ]]; do
  case $1 in
    --profile)   COVERAGE_FILE="$2"; shift 2 ;;
    --html)      COVERAGE_HTML="$2"; shift 2 ;;
    --timeout)   TIMEOUT="$2"; shift 2 ;;
    --help)      usage ;;
    *)           echo "Unknown argument: $1"; usage ;;
  esac
done

echo -e "${BOLD}${CYAN}============================================================================${NC}"
echo -e "${BOLD}${CYAN}  KeyIP-Intelligence Test Coverage Report${NC}"
echo -e "${BOLD}${CYAN}  $(date -u '+%Y-%m-%d %H:%M:%S UTC')${NC}"
echo -e "${BOLD}${CYAN}============================================================================${NC}"
echo ""

# -------------------------------------------------------------------------
# Step 1: Run all tests with coverage
# -------------------------------------------------------------------------
echo -e "${CYAN}>> Running all tests with coverage profiling...${NC}"
echo ""

if ! go test -coverprofile="${COVERAGE_FILE}" -covermode=atomic -count=1 \
    -timeout="${TIMEOUT}" ./... 2>&1 | tail -20; then
  echo ""
  echo -e "${YELLOW}⚠  Some tests failed (coverage report will still be generated)${NC}"
fi

echo ""
echo -e "${GREEN}>> Coverage data written to ${COVERAGE_FILE}${NC}"
echo ""

# -------------------------------------------------------------------------
# Step 2: Generate HTML report
# -------------------------------------------------------------------------
echo -e "${CYAN}>> Generating HTML coverage report...${NC}"
go tool cover -html="${COVERAGE_FILE}" -o "${COVERAGE_HTML}"
echo -e "${GREEN}>> HTML report saved to ${COVERAGE_HTML}${NC}"
echo ""

# -------------------------------------------------------------------------
# Step 3: Parse per-package coverage
# -------------------------------------------------------------------------
echo -e "${BOLD}${CYAN}============================================================================${NC}"
echo -e "${BOLD}${CYAN}  Per-Package Coverage${NC}"
echo -e "${BOLD}${CYAN}============================================================================${NC}"
echo ""

# Use go tool cover to get per-function coverage, parse per-package summary
go tool cover -func="${COVERAGE_FILE}" | while IFS= read -r line; do
  # Lines like: "github.com/turtacn/KeyIP-Intelligence/pkg/client/client.go:42:\tGetClient\t\t100.0%"
  # and last line: "total:\t(statements)\t45.2%"
  echo "  $line"
done

echo ""

# -------------------------------------------------------------------------
# Step 4: Identify packages with zero coverage
# -------------------------------------------------------------------------
echo -e "${BOLD}${CYAN}============================================================================${NC}"
echo -e "${BOLD}${CYAN}  Coverage Overview${NC}"
echo -e "${BOLD}${CYAN}============================================================================${NC}"
echo ""

# Extract total coverage percentage
TOTAL_COVERAGE=$(go tool cover -func="${COVERAGE_FILE}" | grep '^total:' | awk '{print $3}' | sed 's/%//')
if [[ -n "$TOTAL_COVERAGE" ]]; then
  # Color based on coverage level
  if (echo "$TOTAL_COVERAGE < 30" | bc -l 2>/dev/null | grep -q 1); then
    cov_color="${RED}"
  elif (echo "$TOTAL_COVERAGE < 60" | bc -l 2>/dev/null | grep -q 1); then
    cov_color="${YELLOW}"
  else
    cov_color="${GREEN}"
  fi
  echo -e "  Total coverage: ${cov_color}${BOLD}${TOTAL_COVERAGE}%${NC}"
fi
echo ""

# Find packages with zero coverage
echo -e "${BOLD}Uncovered packages (0% coverage):${NC}"
UNCOVERED=0
go tool cover -func="${COVERAGE_FILE}" | grep '0.0%$' | while IFS= read -r line; do
  # Extract package path from the line (format: "path/file.go:line:\tfunc\t0.0%")
  func_line=$(echo "$line" | sed 's/[[:space:]]*0\.0%$//')
  echo "    ${func_line}"
done

# Also check for packages that have no test files at all (not in coverage.out)
echo ""
echo -e "${BOLD}Note:${NC} Packages without any test files do not appear in coverage output."
echo -e "      Use 'go list -f {{.TestGoFiles}} ./...' to identify them."
echo ""

# -------------------------------------------------------------------------
# Step 5: Open HTML report if possible
# -------------------------------------------------------------------------
if which xdg-open > /dev/null 2>&1; then
  echo -e "${CYAN}>> Opening HTML report in browser...${NC}"
  xdg-open "${COVERAGE_HTML}" 2>/dev/null || true
elif which open > /dev/null 2>&1; then
  echo -e "${CYAN}>> Opening HTML report in browser...${NC}"
  open "${COVERAGE_HTML}" 2>/dev/null || true
fi

echo ""
echo -e "${GREEN}Coverage report generated successfully.${NC}"
echo ""

exit 0
