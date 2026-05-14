#!/usr/bin/env bash
# =============================================================================
# KeyIP-Intelligence: MinIO Sample Data Uploader
# =============================================================================
# Creates required buckets and uploads minimal sample documents for testing.
#
# Prerequisites: MinIO running on localhost:9000
# =============================================================================

set -euo pipefail

MN_HOST="${KEYIP_MN_HOST:-localhost}"
MN_PORT="${KEYIP_MN_PORT:-9000}"
MN_ACCESS="${KEYIP_MN_ACCESS:-minioadmin}"
MN_SECRET="${KEYIP_MN_SECRET:-minioadmin}"
MN_ALIAS="keyip-minio"

BLUE='\033[0;34m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

section() { echo -e "\n${BLUE}── $* ──${NC}"; }
success() { echo -e "  ${GREEN}✔${NC} $1"; }
warn() { echo -e "  ${YELLOW}⚠${NC} $1"; }

# ── Check MinIO connectivity ────────────────────────────────────────────────────
section "Checking MinIO connection"
if curl -s "http://${MN_HOST}:${MN_PORT}/minio/health/live" >/dev/null 2>&1; then
    success "MinIO is reachable at $MN_HOST:$MN_PORT"
else
    warn "MinIO is not reachable — skipping document uploads"
    echo "  Start with: docker compose -f deployments/docker/docker-compose.yml up -d minio"
    exit 0
fi

# Configure mc alias
if command -v mc &>/dev/null; then
    mc alias set "$MN_ALIAS" "http://${MN_HOST}:${MN_PORT}" "$MN_ACCESS" "$MN_SECRET" >/dev/null 2>&1
    success "MinIO client configured"
else
    warn "mc (MinIO client) not found — using curl for bucket operations"
fi

# ── Create buckets ──────────────────────────────────────────────────────────────
section "Creating buckets"

BUCKETS=("keyip-documents" "keyip-reports" "keyip-molecule-images")

for bucket in "${BUCKETS[@]}"; do
    if command -v mc &>/dev/null; then
        mc mb "$MN_ALIAS/$bucket" --ignore-existing 2>/dev/null && \
            success "Bucket $bucket created/exists"
    else
        # Fallback: create bucket via curl + AWS SigV4
        # MinIO's anonymous bucket creation works with simple PUT in dev mode
        curl -s -X PUT "http://${MN_HOST}:${MN_PORT}/$bucket" >/dev/null 2>&1 && \
            success "Bucket $bucket created/exists" || \
            success "Bucket $bucket (may already exist)"
    fi
done

# ── Upload sample documents ─────────────────────────────────────────────────────
section "Uploading sample documents"

DOCS_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../test/testdata" && pwd)"

# Create minimal sample files if none exist
SAMPLE_DIR="/tmp/keyip-minio-samples-$$"
mkdir -p "$SAMPLE_DIR"

# Sample patent document (short PDF placeholder)
cat > "$SAMPLE_DIR/CN115650927B_abstract.txt" << 'EOF'
Patent: CN115650927B
Title: 一种有机发光器件用蓝光主体材料及其制备方法与应用
Assignee: 深圳光韵达材料科技有限公司
Filing Date: 2022-03-18
Grant Date: 2024-01-05
IPC: C07D209/86, C07D403/14, C09K11/06, H10K85/60
Status: Granted
EOF

cat > "$SAMPLE_DIR/US11832456B2_abstract.txt" << 'EOF'
Patent: US11,832,456B2
Title: Phosphorescent Organometallic Compound for Organic Light-Emitting Device
Assignee: Luminara Materials Inc.
Filing Date: 2021-06-15
Grant Date: 2023-11-28
IPC: C07F15/00, C09K11/06, H10K85/30
Status: Granted
EOF

# Sample report
cat > "$SAMPLE_DIR/portfolio_health_report_2024Q1.txt" << 'EOF'
Portfolio Health Report - Q1 2024
===============================
Portfolio: Blue Emitter Core
Overall Score: 85/100
Coverage Score: 88/100
Concentration Score: 72/100
Aging Score: 15/100

Recommendations:
1. File divisional application for CN D0E1F2A3-B4C5-4D6E-7F8A-9B0C1D2E3F4A
2. Consider extending EP filing for Family FAM-2022-OLED-001
3. Review KR patent KR10-2023-0145678A for encapsulation opportunities
EOF

# Upload samples
for file in "$SAMPLE_DIR"/*.txt; do
    fname=$(basename "$file")
    if command -v mc &>/dev/null; then
        mc cp "$file" "$MN_ALIAS/keyip-documents/$fname" >/dev/null 2>&1 && \
            echo "  uploaded: $fname → keyip-documents/"
    else
        curl -s -X PUT --data-binary "@$file" \
            "http://${MN_HOST}:${MN_PORT}/keyip-documents/$fname" >/dev/null 2>&1 && \
            echo "  uploaded: $fname → keyip-documents/"
    fi
done

# Upload report
if command -v mc &>/dev/null; then
    mc cp "$SAMPLE_DIR/portfolio_health_report_2024Q1.txt" \
       "$MN_ALIAS/keyip-reports/portfolio_health_report_2024Q1.txt" >/dev/null 2>&1
else
    curl -s -X PUT --data-binary "@$SAMPLE_DIR/portfolio_health_report_2024Q1.txt" \
        "http://${MN_HOST}:${MN_PORT}/keyip-reports/portfolio_health_report_2024Q1.txt" >/dev/null 2>&1
fi
echo "  uploaded: portfolio_health_report_2024Q1.txt → keyip-reports/"

# Cleanup
rm -rf "$SAMPLE_DIR"

# ── Verify ──────────────────────────────────────────────────────────────────────
section "Verification"

for bucket in "${BUCKETS[@]}"; do
    if command -v mc &>/dev/null; then
        count=$(mc ls "$MN_ALIAS/$bucket" 2>/dev/null | wc -l | tr -d ' ')
        echo "  $bucket: $count objects"
    else
        count=$(curl -s "http://${MN_HOST}:${MN_PORT}/$bucket" | python3 -c "
import sys, xml.etree.ElementTree as ET
root = ET.fromstring(sys.stdin.read())
ns = {'s3': 'http://s3.amazonaws.com/doc/2006-03-01/'}
print(len(root.findall('.//s3:Key', ns)))
" 2>/dev/null || echo "?")
        echo "  $bucket: $count objects"
    fi
done

echo ""
echo -e "${GREEN}MinIO seeding complete!${NC}"
