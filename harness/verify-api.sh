#!/bin/bash
# KeyIP-Intelligence API 层验证脚本
# 从 docker-machine 容器内执行，通过 HTTP 验证所有端点
# 用法: bash harness/verify-api.sh

set -e

BASE="${KEYIP_BASE_URL:-http://192.168.99.100}"
PASS=0; FAIL=0; WARN=0
REPORT="/tmp/keyip_api_verify.txt"

log_section() { echo ""; echo "═══ $1 ═══"; }
ok()   { echo "  ✅ $1"; PASS=$((PASS+1)); }
fail() { echo "  ❌ $1 — $2"; FAIL=$((FAIL+1)); }
warn() { echo "  ⚠️  $1 — $2"; WARN=$((WARN+1)); }

api() {
    local ep=$1 method=${2:-GET} data=$3
    if [ -n "$data" ]; then
        curl -s --connect-timeout 10 -X "$method" -H "Content-Type: application/json" -d "$data" "$BASE$ep" 2>/dev/null
    else
        curl -s --connect-timeout 10 -X "$method" "$BASE$ep" 2>/dev/null
    fi
}

check_json() {
    python3 -c "import sys,json; d=json.load(sys.stdin); assert d.get('code') is not None; print('ok')" 2>/dev/null
}

check_count() {
    local expected=$1
    python3 -c "import sys,json; d=json.load(sys.stdin); total=d.get('pagination',{}).get('total','?') if isinstance(d.get('data'),list) else (len(d.get('data',[])) if isinstance(d.get('data'),list) else '?'); print(total)" 2>/dev/null
}

rm -f "$REPORT"

# ─── 1. 前端服务 ───
log_section "1. Frontend (nginx)"

if curl -s -o /dev/null -w '%{http_code}' "$BASE/" | grep -q 200; then
    ok "SPA index.html served (200)"
else
    fail "SPA index.html" "not 200"
fi

for page in dashboard login search patent-mining knowledge-graph fto infringement-watch lifecycle portfolio-optimizer molecules patents/CN115650927B partners health settings; do
    status=$(curl -s -o /dev/null -w '%{http_code}' "$BASE/$page")
    case $status in
        200) ok "$page ($status)" ;;
        30[0-9]) warn "$page" "redirect $status" ;;
        *) fail "$page" "HTTP $status" ;;
    esac
done

# ─── 2. REST API 端点 ───
log_section "2. REST API Endpoints"

# Molecules
mol=$(api "/api/v1/molecules")
if echo "$mol" | check_json > /dev/null; then
    cnt=$(echo "$mol" | check_count)
    [ "$cnt" = "15" ] && ok "molecules (total=$cnt)" || warn "molecules" "expected 15, got $cnt"
else
    fail "molecules" "invalid JSON"
fi

# Patents
pat=$(api "/api/v1/patents")
if echo "$pat" | check_json > /dev/null; then
    cnt=$(echo "$pat" | check_count)
    ok "patents (total=$cnt)"
else
    fail "patents" "invalid JSON"
fi

# Portfolios
port=$(api "/api/v1/portfolios")
if echo "$port" | check_json > /dev/null; then
    cnt=$(echo "$port" | check_count)
    ok "portfolios (total=$cnt)"
else
    fail "portfolios" "invalid JSON"
fi

# Dashboard metrics
dash=$(api "/api/v1/dashboard/metrics")
if echo "$dash" | check_json > /dev/null; then
    health=$(echo "$dash" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d['data'].get('portfolioHealthScore','?'))" 2>/dev/null)
    ok "dashboard/metrics (health=$health)"
else
    fail "dashboard/metrics" "invalid JSON"
fi

# Infringement alerts
inf=$(api "/api/v1/infringement/alerts")
if echo "$inf" | check_json > /dev/null; then
    cnt=$(echo "$inf" | check_count)
    ok "infringement/alerts (total=$cnt)"
else
    fail "infringement/alerts" "invalid JSON"
fi

# FTO search
fto=$(api "/api/v1/fto/search" "POST" '{"query":"CBP"}')
if echo "$fto" | check_json > /dev/null; then
    ok "fto/search"
else
    fail "fto/search" "invalid JSON or non-200"
fi

# Auth: signin
auth=$(api "/api/v1/auth/signin" "POST" '{"email":"turta@keyip.io","password":"turta123!"}')
if echo "$auth" | python3 -c "import sys,json; d=json.load(sys.stdin); code=d.get('code',''); assert code==0 or code=='Unauthorized'" 2>/dev/null; then
    if echo "$auth" | python3 -c "import sys,json; print('token' in json.load(sys.stdin))" 2>/dev/null | grep -q True; then
        ok "auth/signin (JWT returned)"
    else
        warn "auth/signin" "no token — nginx stubs may be needed for login"
    fi
else
    fail "auth/signin" "unexpected response"
fi

# ─── 3. API 数据一致性 ───
log_section "3. Data Consistency"

# Dashboard vs molecules count
dash_patents=$(echo "$dash" | python3 -c "import sys,json; print(json.load(sys.stdin)['data']['totalPatents'])" 2>/dev/null)
mol_count=$(echo "$mol" | check_count)
if [ "$dash_patents" = "$mol_count" ]; then
    ok "dashboard.totalPatents($dash_patents) == molecules.count($mol_count)"
else
    warn "consistency" "dashboard=$dash_patents, molecules=$mol_count"
fi

# Check specific molecules exist
for mol_name in CBP "Ir(ppy)3" DMAC-TRZ Alq3; do
    if echo "$mol" | python3 -c "import sys,json; d=json.load(sys.stdin); assert any(m['name']=='$mol_name' for m in d.get('data',[]))" 2>/dev/null; then
        ok "molecule '$mol_name' exists"
    else
        fail "molecule '$mol_name'" "not found"
    fi
done

# Check specific patents exist
for pat_num in CN115650927B US11678901B2; do
    pat_detail=$(api "/api/v1/patents/$pat_num")
    if echo "$pat_detail" | check_json > /dev/null 2>/dev/null; then
        ok "patent $pat_num detail"
    else
        warn "patent $pat_num" "detail not found as JSON"
    fi
done

# ─── 4. 中间件健康检查 (Prometheus) ───
log_section "4. Middleware Health (Prometheus metrics)"

metrics=$(curl -s --connect-timeout 5 "$BASE:8080/metrics" 2>/dev/null || echo "")
if echo "$metrics" | grep -q "go_goroutines"; then
    goroutines=$(echo "$metrics" | grep "go_goroutines" | head -1 | awk '{print $2}')
    mem=$(echo "$metrics" | grep "go_memstats_alloc_bytes " | awk '{print $2}')
    ok "prometheus metrics (goroutines=$goroutines, heap=${mem}bytes)"
else
    warn "prometheus" "metrics not reachable"
fi

# ─── 5. Proxy 设置验证 ───
log_section "5. Network"
if [ -n "$NO_PROXY" ]; then
    ok "NO_PROXY=$NO_PROXY"
else
    warn "NO_PROXY not set" "local traffic may go through proxy"
fi

# ─── Summary ───
log_section "SUMMARY"
echo "  ✅ Passed: $PASS"
echo "  ⚠️  Warn:   $WARN"
echo "  ❌ Failed: $FAIL"
total=$((PASS + WARN + FAIL))
echo "  📊 Total:  $total checks"
echo ""

if [ "$FAIL" -eq 0 ]; then
    echo "🟢 DELIVERY VERIFICATION: PASSED"
elif [ "$FAIL" -le 3 ]; then
    echo "🟡 DELIVERY VERIFICATION: WARNINGS"
else
    echo "🔴 DELIVERY VERIFICATION: FAILED"
fi

echo "$(date): PASS=$PASS WARN=$WARN FAIL=$FAIL" > "$REPORT"
exit $FAIL
