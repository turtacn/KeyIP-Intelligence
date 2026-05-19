#!/bin/bash
# =============================================================================
# KeyIP Intelligence — Launch Chrome with CDP Debugging (Port 2222)
# =============================================================================
# This script starts a headless Chrome container with Chrome DevTools Protocol
# enabled on port 2222 for E2E browser testing.
#
# Usage:
#   ./launch-chrome-cdp.sh              # Start Chrome on port 2222
#   ./launch-chrome-cdp.sh --stop       # Stop and remove Chrome container
#   ./launch-chrome-cdp.sh --status     # Check if Chrome is running
#
# Network: keyip-network (must exist — see deployments/docker/docker-compose.yml)
# =============================================================================

set -euo pipefail

CONTAINER_NAME="keyip-chrome"
CDP_PORT="${CDP_PORT:-2222}"
NETWORK="${NETWORK:-keyip-network}"
IMAGE="chromedp/headless-shell:latest"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
NC='\033[0m'

# ─── Functions ────────────────────────────────────────────────────────────

status() {
    if docker ps --format '{{.Names}}' | grep -q "^${CONTAINER_NAME}$"; then
        echo -e "${GREEN}[OK]${NC} Chrome container '${CONTAINER_NAME}' is running"
        local ip="${DOCKER_MACHINE_IP:-192.168.99.100}"
        echo "  CDP endpoint: http://${ip}:${CDP_PORT}/json"
        echo "  WebSocket:    ws://${ip}:${CDP_PORT}/devtools/browser/..."
        return 0
    elif docker ps -a --format '{{.Names}}' | grep -q "^${CONTAINER_NAME}$"; then
        echo -e "${YELLOW}[STOPPED]${NC} Chrome container '${CONTAINER_NAME}' exists but is stopped"
        return 1
    else
        echo -e "${RED}[NOT FOUND]${NC} Chrome container '${CONTAINER_NAME}' does not exist"
        return 1
    fi
}

start() {
    # Ensure network exists
    if ! docker network ls --format '{{.Name}}' | grep -q "^${NETWORK}$"; then
        echo -e "${YELLOW}[WARN]${NC} Network '${NETWORK}' not found. Creating..."
        docker network create "${NETWORK}"
    fi

    # Stop existing container if present
    if docker ps -a --format '{{.Names}}' | grep -q "^${CONTAINER_NAME}$"; then
        echo "Removing existing Chrome container..."
        docker stop "${CONTAINER_NAME}" 2>/dev/null || true
        docker rm "${CONTAINER_NAME}" 2>/dev/null || true
    fi

    echo "Starting Chrome with CDP on port ${CDP_PORT}..."
    docker run -d \
        --name "${CONTAINER_NAME}" \
        --network "${NETWORK}" \
        -p "${CDP_PORT}:${CDP_PORT}" \
        --security-opt seccomp=unconfined \
        --shm-size=2g \
        --entrypoint /headless-shell/headless-shell \
        "${IMAGE}" \
        --no-sandbox \
        --disable-gpu \
        --disable-dev-shm-usage \
        --disable-extensions \
        --disable-background-networking \
        --disable-sync \
        --no-first-run \
        --remote-debugging-address=0.0.0.0 \
        --remote-debugging-port="${CDP_PORT}" \
        --remote-allow-origins='*' \
        --window-size=1440,900

    # Wait for Chrome to be ready
    echo "Waiting for Chrome CDP..."
    for i in $(seq 1 30); do
        if docker exec "${CONTAINER_NAME}" wget -qO- "http://localhost:${CDP_PORT}/json/version" 2>/dev/null | grep -q "Browser"; then
            echo -e "${GREEN}[READY]${NC} Chrome CDP is available on port ${CDP_PORT}"
            local ip="${DOCKER_MACHINE_IP:-192.168.99.100}"
            echo ""
            echo "  CDP Endpoint:  http://${ip}:${CDP_PORT}/json"
            echo "  WebSocket:     ws://${ip}:${CDP_PORT}/devtools/browser/..."
            echo ""
            echo "  Test with:     curl http://${ip}:${CDP_PORT}/json"
            echo "  E2E Test:      python3 e2e_comprehensive.py"
            return 0
        fi
        sleep 2
    done
    echo -e "${RED}[FAIL]${NC} Chrome did not become ready within 60s"
    docker logs "${CONTAINER_NAME}" 2>&1 | tail -20
    return 1
}

stop() {
    if docker ps --format '{{.Names}}' | grep -q "^${CONTAINER_NAME}$"; then
        echo "Stopping Chrome container..."
        docker stop "${CONTAINER_NAME}"
        docker rm "${CONTAINER_NAME}"
        echo -e "${GREEN}[OK]${NC} Chrome container removed"
    else
        echo "Chrome container is not running"
    fi
}

logs() {
    docker logs -f --tail 50 "${CONTAINER_NAME}"
}

# ─── Main ─────────────────────────────────────────────────────────────────

case "${1:-start}" in
    --start|start)
        start
        ;;
    --stop|stop)
        stop
        ;;
    --status|status)
        status
        ;;
    --logs|logs)
        logs
        ;;
    --restart|restart)
        stop
        sleep 2
        start
        ;;
    *)
        echo "Usage: $0 [--start|--stop|--status|--logs|--restart]"
        echo ""
        echo "Environment variables:"
        echo "  CDP_PORT          Chrome CDP port (default: 2222)"
        echo "  NETWORK           Docker network name (default: keyip-network)"
        echo "  DOCKER_MACHINE_IP Docker Machine VM IP (default: 192.168.99.100)"
        exit 1
        ;;
esac
