#!/usr/bin/env bash
set -euo pipefail

# KeyIP-Intelligence - Cross-platform Build Script

PROJECT_ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
MODULE=$(grep "module" "${PROJECT_ROOT}/go.mod" | cut -d' ' -f2)
VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME=$(date -u '+%Y-%m-%dT%H:%M:%SZ')
GO_VERSION=$(go version | cut -d' ' -f3)

TARGETS=("apiserver" "worker" "keyip")
TARGET="all"
OS=$(go env GOOS)
ARCH=$(go env GOARCH)
OUTPUT="bin"

usage() {
  echo "Usage: $0 [options]"
  echo "Options:"
  echo "  --target [apiserver|worker|keyip|all] (default: all)"
  echo "  --os [linux|darwin|windows]          (default: current)"
  echo "  --arch [amd64|arm64]                 (default: current)"
  echo "  --output <dir>                       (default: bin)"
  exit 1
}

while [[ $# -gt 0 ]]; do
  case $1 in
    --target) TARGET="$2"; shift 2 ;;
    --os)     OS="$2"; shift 2 ;;
    --arch)   ARCH="$2"; shift 2 ;;
    --output) OUTPUT="$2"; shift 2 ;;
    --help)   usage ;;
    *)        echo "Unknown argument: $1"; usage ;;
  esac
done

LDFLAGS="-s -w -X ${MODULE}/internal/config.Version=${VERSION} \
         -X ${MODULE}/internal/config.CommitSHA=${COMMIT} \
         -X ${MODULE}/internal/config.BuildTime=${BUILD_TIME} \
         -X ${MODULE}/internal/config.GoVersion=${GO_VERSION}"

mkdir -p "${OUTPUT}"

build_target() {
  local target=$1
  local binary="${OUTPUT}/${target}"
  if [ "${OS}" == "windows" ]; then binary="${binary}.exe"; fi

  echo ">> Building ${target} for ${OS}/${ARCH}..."
  start_time=$(date +%s)
  CGO_ENABLED=0 GOOS=${OS} GOARCH=${ARCH} go build -ldflags "${LDFLAGS}" -o "${binary}" "./cmd/${target}/"
  end_time=$(date +%s)
  duration=$((end_time - start_time))

  size=$(du -h "${binary}" | cut -f1)
  echo "<< Built ${target} (${size}) in ${duration}s"
}

if [ "${TARGET}" == "all" ]; then
  for t in "${TARGETS[@]}"; do
    build_target "$t"
  done
else
  build_target "${TARGET}"
fi

echo "Build process completed successfully."

# //Personal.AI order the ending
