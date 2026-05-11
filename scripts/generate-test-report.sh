#!/usr/bin/env bash
set -euo pipefail

# =============================================================================
# KeyIP-Intelligence - E2E Test Report Generator
# =============================================================================
# Runs E2E tests with JSON output, parses results, and generates a colored
# summary report grouped by scenario.
# =============================================================================

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

OUTPUT_FILE="test-output.json"
VERBOSE=false

usage() {
  echo "Usage: $0 [options]"
  echo "Options:"
  echo "  --verbose    Print detailed test output alongside summary"
  echo "  --output     Output file path (default: test-output.json)"
  exit 1
}

while [[ $# -gt 0 ]]; do
  case $1 in
    --verbose)   VERBOSE=true; shift ;;
    --output)    OUTPUT_FILE="$2"; shift 2 ;;
    --help)      usage ;;
    *)           echo "Unknown argument: $1"; usage ;;
  esac
done

echo -e "${BLUE}${BOLD}============================================================================${NC}"
echo -e "${BLUE}${BOLD}  KeyIP-Intelligence E2E Test Report${NC}"
echo -e "${BLUE}${BOLD}  $(date -u '+%Y-%m-%d %H:%M:%S UTC')${NC}"
echo -e "${BLUE}${BOLD}============================================================================${NC}"
echo ""

# -------------------------------------------------------------------------
# Step 1: Run tests with JSON output
# -------------------------------------------------------------------------
echo -e "${CYAN}>> Running E2E tests (JSON output → ${OUTPUT_FILE})...${NC}"

if ! go test -tags=e2e -v -json -count=1 -timeout=600s ./test/e2e/... 2>&1 | tee "${OUTPUT_FILE}"; then
  echo ""
  echo -e "${YELLOW}⚠  Some tests failed (continuing with report generation)${NC}"
fi

echo ""
echo -e "${GREEN}>> Raw output saved to ${OUTPUT_FILE}${NC}"
echo ""

# -------------------------------------------------------------------------
# Step 2: Parse JSON output
# -------------------------------------------------------------------------
echo -e "${CYAN}>> Parsing results...${NC}"

TOTAL=0
PASSED=0
FAILED=0
SKIPPED=0

# Associative arrays: scenario -> counts
declare -A SCENARIO_TOTAL
declare -A SCENARIO_PASSED
declare -A SCENARIO_FAILED
declare -A SCENARIO_SKIPPED
# Array of scenario names in order of appearance
SCENARIO_NAMES=()

# Map from test name -> scenario group
# Scenario groups are the module name: "TestMoleculeSearchE2E_..." → "MoleculeSearch"
get_scenario() {
  local test_name="$1"
  # Extract the base module name: Test<Name>E2E or Test<Name>E2E_
  if [[ "$test_name" =~ ^Test([A-Za-z]+)E2E ]]; then
    echo "${BASH_REMATCH[1]}"
  else
    echo "Other"
  fi
}

# Read JSON lines, collect pass/fail/skip per test
while IFS= read -r line; do
  action=$(echo "$line" | jq -r '.action // empty' 2>/dev/null)
  test_name=$(echo "$line" | jq -r '.test // empty' 2>/dev/null)
  pkg=$(echo "$line" | jq -r '.Package // empty' 2>/dev/null)

  # Only process test-scoped actions (not package-level)
  if [[ -z "$test_name" ]]; then
    continue
  fi

  case "$action" in
    pass)
      TOTAL=$((TOTAL + 1))
      PASSED=$((PASSED + 1))
      scenario=$(get_scenario "$test_name")
      SCENARIO_TOTAL["$scenario"]=$((SCENARIO_TOTAL["$scenario"] + 1))
      SCENARIO_PASSED["$scenario"]=$((SCENARIO_PASSED["$scenario"] + 1))
      ;;
    fail)
      TOTAL=$((TOTAL + 1))
      FAILED=$((FAILED + 1))
      scenario=$(get_scenario "$test_name")
      SCENARIO_TOTAL["$scenario"]=$((SCENARIO_TOTAL["$scenario"] + 1))
      SCENARIO_FAILED["$scenario"]=$((SCENARIO_FAILED["$scenario"] + 1))
      ;;
    skip)
      TOTAL=$((TOTAL + 1))
      SKIPPED=$((SKIPPED + 1))
      scenario=$(get_scenario "$test_name")
      SCENARIO_TOTAL["$scenario"]=$((SCENARIO_TOTAL["$scenario"] + 1))
      SCENARIO_SKIPPED["$scenario"]=$((SCENARIO_SKIPPED["$scenario"] + 1))
      ;;
  esac
done < <(cat "${OUTPUT_FILE}")

# Collect scenario names (maintain insertion order)
while IFS= read -r line; do
  test_name=$(echo "$line" | jq -r '.test // empty' 2>/dev/null)
  if [[ -n "$test_name" ]]; then
    scenario=$(get_scenario "$test_name")
    # Only add if not already present
    already=false
    for s in "${SCENARIO_NAMES[@]}"; do
      if [[ "$s" == "$scenario" ]]; then
        already=true
        break
      fi
    done
    if [[ "$already" == false ]]; then
      SCENARIO_NAMES+=("$scenario")
    fi
  fi
done < <(grep '"action":"\(pass\|fail\|skip\)"' "${OUTPUT_FILE}")

# -------------------------------------------------------------------------
# Step 3: Print grouped report
# -------------------------------------------------------------------------
echo ""
echo -e "${BOLD}${BLUE}============================================================================${NC}"
echo -e "${BOLD}${BLUE}  Test Results by Scenario${NC}"
echo -e "${BOLD}${BLUE}============================================================================${NC}"
echo ""

for scenario in "${SCENARIO_NAMES[@]}"; do
  s_total=${SCENARIO_TOTAL["$scenario"]:-0}
  s_passed=${SCENARIO_PASSED["$scenario"]:-0}
  s_failed=${SCENARIO_FAILED["$scenario"]:-0}
  s_skipped=${SCENARIO_SKIPPED["$scenario"]:-0}

  # Scenario header
  echo -e "  ${BOLD}${scenario}${NC}"
  echo -e "  ${BOLD}$(printf '%*s' "${#scenario}" '' | tr ' ' '-')${NC}"

  # Determine color based on result
  if [[ "$s_failed" -gt 0 ]]; then
    scenario_color="${RED}"
  elif [[ "$s_skipped" -gt 0 ]]; then
    scenario_color="${YELLOW}"
  else
    scenario_color="${GREEN}"
  fi

  echo -e "    Total:  ${s_total}"
  echo -e "    Passed: ${GREEN}${s_passed}${NC}"
  echo -e "    Failed: ${s_failed}"
  echo -e "    Skipped:${s_skipped}"

  # Show failed test names for this scenario
  if [[ "$s_failed" -gt 0 ]]; then
    echo ""
    while IFS= read -r line; do
      test_name=$(echo "$line" | jq -r '.test // empty' 2>/dev/null)
      if [[ -n "$test_name" ]]; then
        scenario_check=$(get_scenario "$test_name")
        if [[ "$scenario_check" == "$scenario" ]]; then
          echo -e "      ${RED}✗ ${test_name}${NC}"
        fi
      fi
    done < <(grep '"action":"fail"' "${OUTPUT_FILE}")
  fi

  echo ""
done

# -------------------------------------------------------------------------
# Step 4: Print failed test details (verbose)
# -------------------------------------------------------------------------
if [[ "$VERBOSE" == true ]] && [[ "$FAILED" -gt 0 ]]; then
  echo -e "${BOLD}${RED}============================================================================${NC}"
  echo -e "${BOLD}${RED}  Failed Test Details${NC}"
  echo -e "${BOLD}${RED}============================================================================${NC}"
  echo ""

  current_test=""
  while IFS= read -r line; do
    action=$(echo "$line" | jq -r '.action // empty' 2>/dev/null)
    test_name=$(echo "$line" | jq -r '.test // empty' 2>/dev/null)
    output=$(echo "$line" | jq -r '.Output // empty' 2>/dev/null)

    if [[ "$action" == "fail" ]] && [[ -n "$test_name" ]]; then
      current_test="$test_name"
    fi

    if [[ "$action" == "output" ]] && [[ -n "$output" ]] && [[ -n "$current_test" ]]; then
      # Only show output for the most recent failed test
      if [[ -n "$current_test" ]]; then
        echo -en "${RED}${current_test}:${NC} " >&2
        echo -e "$output"
      fi
    fi
  done < <(cat "${OUTPUT_FILE}")
  echo ""
fi

# -------------------------------------------------------------------------
# Step 5: Print summary header
# -------------------------------------------------------------------------
echo -e "${BOLD}${BLUE}============================================================================${NC}"
echo -e "${BOLD}${BLUE}  Summary${NC}"
echo -e "${BOLD}${BLUE}============================================================================${NC}"
echo ""

if [[ "$FAILED" -gt 0 ]]; then
  summary_color="${RED}"
  status="FAILED"
elif [[ "$SKIPPED" -gt 0 ]]; then
  summary_color="${YELLOW}"
  status="PASSED WITH SKIPS"
else
  summary_color="${GREEN}"
  status="PASSED"
fi

echo -e "  Status:  ${summary_color}${BOLD}${status}${NC}"
echo -e "  Total:   ${TOTAL}"
echo -e "  Passed:  ${GREEN}${PASSED}${NC}"
echo -e "  Failed:  ${RED}${FAILED}${NC}"
echo -e "  Skipped: ${YELLOW}${SKIPPED}${NC}"

# Pass rate
if [[ "$TOTAL" -gt 0 ]]; then
  pass_rate=$(echo "scale=1; ${PASSED} * 100 / ${TOTAL}" | bc 2>/dev/null || echo "0")
  echo -e "  Rate:    ${pass_rate}%"
fi

echo ""
echo -e "${BLUE}Report generated: $(date -u '+%Y-%m-%d %H:%M:%S UTC')${NC}"
echo ""

# Exit with failure if any test failed
if [[ "$FAILED" -gt 0 ]]; then
  exit 1
fi
exit 0
