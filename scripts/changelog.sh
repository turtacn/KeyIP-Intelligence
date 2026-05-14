#!/usr/bin/env bash
# =============================================================================
#  KeyIP-Intelligence Changelog Generator
# =============================================================================
#  Generates a Markdown changelog from git commits, grouped by category.
#
#  Usage:
#    ./scripts/changelog.sh                  # All commits (no tags needed)
#    ./scripts/changelog.sh v1.0.0..HEAD     # From tag v1.0.0 to HEAD
#    ./scripts/changelog.sh v0.9.0..v1.0.0   # Between two tags
#    ./scripts/changelog.sh --since 2025-01-01  # Since a specific date
#    ./scripts/changelog.sh --help           # Show this help
# =============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
PROJECT_ROOT="$SCRIPT_DIR"

usage() {
    sed -n '3,13p' "$0"
    exit 0
}

# --- Parse arguments ----------------------------------------------------------

RANGE=""
SINCE_DATE=""
OUTPUT_FILE=""

while [[ $# -gt 0 ]]; do
    case "$1" in
        --help) usage ;;
        --since)
            shift
            SINCE_DATE="$1"
            ;;
        --output)
            shift
            OUTPUT_FILE="$1"
            ;;
        -*)
            echo "Unknown option: $1"
            usage
            ;;
        *)
            RANGE="$1"
            ;;
    esac
    shift
done

# --- Gather commits -----------------------------------------------------------

cd "$PROJECT_ROOT"

# Build git-log arguments
LOG_ARGS=(--no-merges --pretty=format:"%H||%s||%an||%ai")

if [[ -n "$RANGE" ]]; then
    LOG_ARGS+=("$RANGE")
elif [[ -n "$SINCE_DATE" ]]; then
    LOG_ARGS+=("--since=$SINCE_DATE")
fi

COMMITS=$(git log "${LOG_ARGS[@]}" 2>/dev/null) || {
    echo "Error: Failed to get git log. Check your range or date." >&2
    exit 1
}

if [[ -z "$COMMITS" ]]; then
    echo "No commits found."
    exit 0
fi

# --- Categorize commits by conventional-commit type ---------------------------

declare -A CATEGORIES=(
    [feat]="Features"
    [fix]="Bug Fixes"
    [chore]="Chores"
    [docs]="Documentation"
    [refactor]="Refactoring"
    [test]="Tests"
    [style]="Style"
    [perf]="Performance"
    [ci]="CI/CD"
    [build]="Build"
    [revert]="Reverts"
)

# Define display order
CAT_ORDER=(feat fix perf refactor test docs chore ci build style revert)

declare -A CAT_BUCKETS
for cat in "${!CATEGORIES[@]}"; do
    CAT_BUCKETS["$cat"]=""
done

OTHER_BUCKET=""
VERSION=""

while IFS= read -r line; do
    [[ -z "$line" ]] && continue

    HASH="${line%%||*}"
    rest="${line#*||}"
    SUBJECT="${rest%%||*}"
    rest="${rest#*||}"
    AUTHOR="${rest%%||*}"
    DATE="${rest#*||}"

    # Extract conventional-commit type: "feat: msg" or "feat(scope): msg"
    TYPE=""
    CONV_COMMIT_REGEX='^([a-zA-Z]+)(\([^)]*\))?!?:'
    if [[ "$SUBJECT" =~ $CONV_COMMIT_REGEX ]]; then
        TYPE="${BASH_REMATCH[1]}"
    fi

    LINK="[${HASH:0:7}](https://github.com/turtacn/KeyIP-Intelligence/commit/$HASH)"
    ENTRY="- ${LINK} ${SUBJECT} (@${AUTHOR})"

    if [[ -n "$TYPE" ]] && [[ -n "${CATEGORIES[$TYPE]:-}" ]]; then
        CAT_BUCKETS["$TYPE"]+="${ENTRY}"$'\n'
    else
        OTHER_BUCKET+="${ENTRY}"$'\n'
    fi

    # Try to extract version from tag-ish commits
    if [[ "$SUBJECT" =~ ^chore\(release\):\ v?([0-9]+\.[0-9]+\.[0-9]+) ]]; then
        VERSION="${BASH_REMATCH[1]}"
    fi
done <<< "$COMMITS"

# --- Generate Markdown --------------------------------------------------------

NOW="$(date -u +%Y-%m-%d)"
TITLE="Changelog"
OUTPUT=""

# Heading
if [[ -n "$VERSION" ]]; then
    OUTPUT+="# ${TITLE} — v${VERSION}\n\n"
else
    OUTPUT+="# ${TITLE}\n\n"
fi
OUTPUT+="_Generated on ${NOW}_\n\n"

# Summary counts
TOTAL=$(echo "$COMMITS" | wc -l)
OUTPUT+="**Total commits:** ${TOTAL}  \n"
OUTPUT+="\n---\n\n"

# Per-category sections
HAS_ANY="false"
for cat in "${CAT_ORDER[@]}"; do
    entries="${CAT_BUCKETS[$cat]}"
    [[ -z "$entries" ]] && continue
    HAS_ANY="true"
    OUTPUT+="## ${CATEGORIES[$cat]}\n\n"
    OUTPUT+="${entries}"
    OUTPUT+="\n"
done

# Uncategorized
if [[ -n "$OTHER_BUCKET" ]]; then
    OUTPUT+="## Uncategorized\n\n"
    OUTPUT+="${OTHER_BUCKET}"
    OUTPUT+="\n"
fi

if [[ "$HAS_ANY" == "false" ]] && [[ -z "$OTHER_BUCKET" ]]; then
    OUTPUT+="_No changes found._\n"
fi

# --- Output -------------------------------------------------------------------

if [[ -n "$OUTPUT_FILE" ]]; then
    echo -e "$OUTPUT" > "$OUTPUT_FILE"
    echo "Changelog written to: $OUTPUT_FILE"
else
    echo -e "$OUTPUT"
fi
