#!/usr/bin/env bash
# ==============================================================================
# KeyIP-Intelligence — Docker 服务健康检查验证脚本
# ==============================================================================
# 检查所有 9 个外部服务（PostgreSQL、Redis、Neo4j、OpenSearch/ES、
# Milvus、MinIO、Kafka、Keycloak、MailHog）的 TCP 端口及 HTTP 健康端点。
#
# 用法:
#   ./scripts/health-check.sh              # 彩色文本输出
#   ./scripts/health-check.sh --json       # JSON 格式（适合 CI 集成）
#   ./scripts/health-check.sh --wait       # 等待所有服务健康（超时 120s）
#   ./scripts/health-check.sh --wait=60    # 自定义超时秒数
#   ./scripts/health-check.sh --json --wait
# ==============================================================================

set -euo pipefail

# =============================================================================
# 颜色与图标
# =============================================================================
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

# =============================================================================
# 全局配置
# =============================================================================
HOST="localhost"
CHECK_TIMEOUT=5            # 单次 TCP/HTTP 检查超时（秒）
WAIT_TIMEOUT=120           # --wait 模式最长等待时间（秒）
POLL_INTERVAL=5            # --wait 模式轮询间隔（秒）
PRINT_JSON=false           # --json 开关
WAIT_MODE=false            # --wait 开关

# =============================================================================
# 结果存储（关联数组，bash 4+）
# =============================================================================
declare -A RESULTS         # status: pass|fail|warn
declare -A MESSAGES        # 人类可读描述
TOTAL=9
PASSED=0
FAILED=0
WARNINGS=0

# =============================================================================
# 工具函数
# =============================================================================

# TCP 端口连通性检查
# 优先使用 bash /dev/tcp 内置虚拟设备；回退到 nc
tcp_check() {
  local host="$1" port="$2"
  # bash /dev/tcp 内置（需编译支持）
  if timeout "$CHECK_TIMEOUT" bash -c "echo >/dev/tcp/${host}/${port}" 2>/dev/null; then
    return 0
  fi
  # 回退：nc
  if command -v nc &>/dev/null && nc -z -w "$CHECK_TIMEOUT" "$host" "$port" 2>/dev/null; then
    return 0
  fi
  return 1
}

# HTTP GET — 返回 HTTP 状态码（000 表示连接失败）
http_get() {
  local url="$1"
  local code
  code=$(curl -s --max-time "$CHECK_TIMEOUT" -o /dev/null -w "%{http_code}" "$url" 2>/dev/null) || code="000"
  echo "$code"
}

# HTTP GET — 返回响应体（空字符串表示失败）
http_body() {
  local url="$1"
  curl -s --max-time "$CHECK_TIMEOUT" "$url" 2>/dev/null || echo ""
}

# JSON 字符串转义
json_escape() {
  local s="$1"
  s="${s//\\/\\\\}"    # backslash -> double backslash
  s="${s//\"/\\\"}"    # double quote -> \"
  s="${s//$'\n'/\\n}"  # newline -> \n
  s="${s//$'\t'/\\t}"  # tab -> \t
  echo "$s"
}

# 记录单个服务检查结果
record() {
  local svc="$1" status="$2" msg="$3"
  RESULTS["$svc"]="$status"
  MESSAGES["$svc"]="$msg"
  case "$status" in
    pass) PASSED=$((PASSED + 1)) ;;
    fail) FAILED=$((FAILED + 1)) ;;
    warn) WARNINGS=$((WARNINGS + 1)) ;;
  esac
}

# 打印单行状态（文本模式）
print_status_line() {
  local svc="$1"
  local status="${RESULTS[$svc]}"
  local msg="${MESSAGES[$svc]}"
  case "$status" in
    pass) printf "  ${GREEN}${BOLD} ✓${NC} %-15s %s\n" "$svc" "$msg" ;;
    fail) printf "  ${RED}${BOLD} ✗${NC} %-15s %s\n" "$svc" "$msg" ;;
    warn) printf "  ${YELLOW}${BOLD} ⚠${NC} %-15s %s\n" "$svc" "$msg" ;;
  esac
}

# =============================================================================
# 服务健康检查函数
# =============================================================================

# ---- 1. PostgreSQL 16 (5432) ----
# 健康端点: pg_isready / psql SELECT 1
check_postgresql() {
  local svc="PostgreSQL" port=5432

  if ! tcp_check "$HOST" "$port"; then
    record "$svc" "fail" "端口 $port 无法连接"
    return
  fi
  if command -v pg_isready &>/dev/null; then
    if pg_isready -h "$HOST" -p "$port" -U keyip -d keyip_dev &>/dev/null; then
      record "$svc" "pass" "pg_isready: 正常接受连接"
      return
    else
      record "$svc" "fail" "pg_isready: 服务未就绪"
      return
    fi
  fi
  if command -v psql &>/dev/null; then
    local out
    out=$(PGPASSWORD=keyip_dev psql -h "$HOST" -p "$port" -U keyip -d keyip_dev -Atc "SELECT 1" 2>/dev/null) || out=""
    if [[ "$out" == "1" ]]; then
      record "$svc" "pass" "psql SELECT 1: 正常"
      return
    else
      record "$svc" "fail" "psql 查询无结果"
      return
    fi
  fi
  record "$svc" "warn" "端口 $port 开放（pg_isready/psql 不可用，仅验证 TCP）"
}

# ---- 2. Redis 7 (6379) ----
# 健康端点: redis-cli PING
check_redis() {
  local svc="Redis" port=6379

  if ! tcp_check "$HOST" "$port"; then
    record "$svc" "fail" "端口 $port 无法连接"
    return
  fi
  if command -v redis-cli &>/dev/null; then
    local reply
    reply=$(redis-cli -h "$HOST" -p "$port" PING 2>/dev/null) || reply=""
    if [[ "$reply" == "PONG" ]]; then
      record "$svc" "pass" "PING: PONG"
      return
    else
      record "$svc" "fail" "PING 未返回 PONG (回复: ${reply:-空})"
      return
    fi
  fi
  record "$svc" "warn" "端口 $port 开放（redis-cli 不可用，仅验证 TCP）"
}

# ---- 3. Neo4j 5 (Bolt 7687 + HTTP 7474) ----
# 健康端点: Bolt TCP 连接 + HTTP GET /
check_neo4j() {
  local svc="Neo4j" bolt_port=7687 http_port=7474
  local bolt_ok=false http_ok=false http_code="000"

  tcp_check "$HOST" "$bolt_port" && bolt_ok=true
  http_code=$(http_get "http://${HOST}:${http_port}/")
  [[ "$http_code" == "200" ]] && http_ok=true

  if $bolt_ok && $http_ok; then
    record "$svc" "pass" "Bolt $bolt_port + HTTP $http_port: 正常"
  elif $bolt_ok; then
    record "$svc" "warn" "Bolt $bolt_port 开放但 HTTP $http_port ($http_code)"
  elif $http_ok; then
    record "$svc" "warn" "HTTP $http_port 正常但 Bolt $bolt_port 不可达"
  else
    record "$svc" "fail" "端口 $bolt_port (Bolt) / $http_port (HTTP) 均无法连接"
  fi
}

# ---- 4. OpenSearch / Elasticsearch (9200) ----
# 健康端点: GET /
check_opensearch() {
  local svc="OpenSearch" port=9200

  if ! tcp_check "$HOST" "$port"; then
    record "$svc" "fail" "端口 $port 无法连接"
    return
  fi
  local code body
  code=$(http_get "http://${HOST}:${port}/")
  body=$(http_body "http://${HOST}:${port}/")
  if [[ "$code" == "200" ]]; then
    if echo "$body" | grep -q '"cluster_name"\|"tagline"\|"name"'; then
      record "$svc" "pass" "HTTP 200: 响应正常"
    else
      record "$svc" "warn" "HTTP 200 但响应体格式异常"
    fi
  else
    record "$svc" "fail" "HTTP $code (期望 200)"
  fi
}

# ---- 5. Milvus 2.4 (gRPC 19530) ----
# 健康端点: gRPC grpc.health.v1.Health/Check; 回退 TCP
check_milvus() {
  local svc="Milvus" port=19530

  if ! tcp_check "$HOST" "$port"; then
    record "$svc" "fail" "端口 $port (gRPC) 无法连接"
    return
  fi
  if command -v grpcurl &>/dev/null; then
    local reply
    reply=$(grpcurl -plaintext -max-time "$CHECK_TIMEOUT" "${HOST}:${port}" grpc.health.v1.Health/Check 2>/dev/null) || reply=""
    if echo "$reply" | grep -q '"SERVING"'; then
      record "$svc" "pass" "gRPC health: SERVING"
      return
    elif echo "$reply" | grep -q '"NOT_SERVING"'; then
      record "$svc" "fail" "gRPC health: NOT_SERVING"
      return
    else
      record "$svc" "warn" "gRPC health: 未知状态（grpcurl 可用但响应异常）"
      return
    fi
  fi
  record "$svc" "warn" "端口 $port 开放（grpcurl 不可用，仅验证 TCP）"
}

# ---- 6. MinIO (S3 9002) ----
# 健康端点: GET /minio/health/live
check_minio() {
  local svc="MinIO" port=9002

  if ! tcp_check "$HOST" "$port"; then
    record "$svc" "fail" "端口 $port 无法连接"
    return
  fi
  local code
  code=$(http_get "http://${HOST}:${port}/minio/health/live")
  if [[ "$code" == "200" ]]; then
    record "$svc" "pass" "/minio/health/live HTTP 200"
  else
    record "$svc" "fail" "/minio/health/live HTTP $code (期望 200)"
  fi
}

# ---- 7. Kafka 7.6 (9092) ----
# 健康端点: TCP 端口 + kcat 元数据获取
check_kafka() {
  local svc="Kafka" port=9092

  if ! tcp_check "$HOST" "$port"; then
    record "$svc" "fail" "端口 $port 无法连接"
    return
  fi
  if command -v kcat &>/dev/null; then
    if kcat -b "${HOST}:${port}" -L -t _ &>/dev/null; then
      record "$svc" "pass" "Broker 元数据获取正常"
      return
    else
      record "$svc" "fail" "kcat 获取 broker 元数据失败"
      return
    fi
  fi
  if command -v kafka-broker-api-versions.sh &>/dev/null; then
    if kafka-broker-api-versions.sh --bootstrap-server "${HOST}:${port}" --timeout "$CHECK_TIMEOUT"000 &>/dev/null; then
      record "$svc" "pass" "Broker API 版本查询正常"
      return
    else
      record "$svc" "fail" "Broker API 版本查询失败"
      return
    fi
  fi
  record "$svc" "warn" "端口 $port 开放（kcat/kafka 工具不可用，仅验证 TCP）"
}

# ---- 8. Keycloak 24 (8180) ----
# 健康端点: GET /health; 回退 /realms/master
check_keycloak() {
  local svc="Keycloak" port=8180

  if ! tcp_check "$HOST" "$port"; then
    record "$svc" "fail" "端口 $port 无法连接"
    return
  fi
  local code
  code=$(http_get "http://${HOST}:${port}/health")
  if [[ "$code" == "200" ]]; then
    record "$svc" "pass" "/health HTTP 200"
    return
  fi
  code=$(http_get "http://${HOST}:${port}/realms/master")
  if [[ "$code" == "200" ]]; then
    record "$svc" "pass" "/realms/master HTTP 200"
    return
  fi
  code=$(http_get "http://${HOST}:${port}/")
  if [[ "$code" == "200" ]]; then
    record "$svc" "warn" "根路径 HTTP 200（/health 和 /realms/master 不可用，服务可能未完全就绪）"
    return
  fi
  record "$svc" "fail" "所有健康端点无响应 (HTTP $code)"
}

# ---- 9. MailHog (SMTP 1025 + UI 8025) ----
# 健康端点: TCP SMTP 端口 + HTTP UI
check_mailhog() {
  local svc="MailHog" smtp_port=1025 http_port=8025
  local smtp_ok=false http_ok=false http_code="000"

  tcp_check "$HOST" "$smtp_port" && smtp_ok=true
  http_code=$(http_get "http://${HOST}:${http_port}/")
  [[ "$http_code" == "200" ]] && http_ok=true

  if $smtp_ok && $http_ok; then
    record "$svc" "pass" "SMTP $smtp_port + UI $http_port: 正常"
  elif $smtp_ok; then
    record "$svc" "warn" "SMTP $smtp_port 开放但 UI $http_port ($http_code)"
  elif $http_ok; then
    record "$svc" "warn" "UI $http_port 正常但 SMTP $smtp_port 不可达"
  else
    record "$svc" "fail" "SMTP $smtp_port / UI $http_port 均无法连接"
  fi
}

# =============================================================================
# 运行所有服务检查
# =============================================================================
SERVICE_LIST=(
  "PostgreSQL"
  "Redis"
  "Neo4j"
  "OpenSearch"
  "Milvus"
  "MinIO"
  "Kafka"
  "Keycloak"
  "MailHog"
)

run_all_checks() {
  PASSED=0; FAILED=0; WARNINGS=0

  check_postgresql
  check_redis
  check_neo4j
  check_opensearch
  check_milvus
  check_minio
  check_kafka
  check_keycloak
  check_mailhog
}

# =============================================================================
# 文本模式输出
# =============================================================================
output_text() {
  echo ""
  echo -e "${BOLD}===== KeyIP-Intelligence 服务健康检查 =====${NC}"
  echo ""
  for svc in "${SERVICE_LIST[@]}"; do
    print_status_line "$svc"
  done
  echo ""
  echo -e "${BOLD}============================================${NC}"

  # 汇总
  local color="$GREEN"
  local summary="${PASSED}/${TOTAL}"
  if (( FAILED > 0 )); then
    color="$RED"
  elif (( WARNINGS > 0 )); then
    color="$YELLOW"
  fi
  echo -e "结果: ${color}${BOLD}${summary}${NC} 服务健康 (通过 ${PASSED}, 失败 ${FAILED}, 警告 ${WARNINGS})"
}

# =============================================================================
# JSON 模式输出
# =============================================================================
output_json() {
  local timestamp
  timestamp=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
  local healthy="false"
  [[ "$FAILED" == "0" ]] && healthy="true"

  local json="{"
  json+="\"timestamp\":\"${timestamp}\","
  json+="\"summary\":{"
  json+="\"total\":${TOTAL},"
  json+="\"passed\":${PASSED},"
  json+="\"failed\":${FAILED},"
  json+="\"warnings\":${WARNINGS},"
  json+="\"healthy\":${healthy}"
  json+="},"
  json+="\"services\":{"

  local first=true svc status msg
  for svc in "${SERVICE_LIST[@]}"; do
    $first || json+=","
    first=false
    status="${RESULTS[$svc]}"
    msg="$(json_escape "${MESSAGES[$svc]}")"
    json+="\"${svc}\":{\"status\":\"${status}\",\"message\":\"${msg}\"}"
  done

  json+="}}"
  echo "$json"
}

# =============================================================================
# 输出分发
# =============================================================================
emit_output() {
  if [[ "$PRINT_JSON" == "true" ]]; then
    output_json
  else
    output_text
  fi
}

# =============================================================================
# --wait 模式：轮询等待所有服务健康
# =============================================================================
wait_mode() {
  local start_time elapsed
  start_time=$(date +%s)

  echo -e "${CYAN}[INFO]${NC} 等待全部服务健康（超时 ${WAIT_TIMEOUT}s，轮询间隔 ${POLL_INTERVAL}s）..." >&2

  while true; do
    run_all_checks
    elapsed=$(( $(date +%s) - start_time ))

    # 输出当前状态到 stderr，不干扰 --json 的 stdout
    echo -e "${CYAN}[$(date +%H:%M:%S)]${NC} 检查完成: ${PASSED}/${TOTAL} 健康 (已等待 ${elapsed}s)" >&2

    # 列出仍未通过的服务
    for svc in "${SERVICE_LIST[@]}"; do
      if [[ "${RESULTS[$svc]}" != "pass" ]]; then
        echo -e "    ${YELLOW}⏳${NC} $svc: ${MESSAGES[$svc]}" >&2
      fi
    done

    if (( FAILED == 0 )); then
      echo "" >&2
      echo -e "${GREEN}${BOLD}全部 ${TOTAL} 个服务健康！${NC}" >&2
      emit_output
      return 0
    fi

    if (( elapsed >= WAIT_TIMEOUT )); then
      echo "" >&2
      echo -e "${RED}${BOLD}超时！等待 ${elapsed}s 后仍有 ${FAILED} 个服务异常${NC}" >&2
      emit_output
      return 2
    fi

    sleep "$POLL_INTERVAL"
  done
}

# =============================================================================
# 参数解析
# =============================================================================
usage() {
  cat <<EOF
用法: $0 [选项]

选项:
  --json        以 JSON 格式输出结果（适用于 CI 集成）
  --wait[=秒]   等待所有服务健康后再退出（默认超时 ${WAIT_TIMEOUT} 秒）
  --help, -h    显示此帮助信息

示例:
  $0                     # 彩色文本输出
  $0 --json              # JSON 格式
  $0 --wait              # 等待最多 120s
  $0 --wait=60           # 等待最多 60s
  $0 --json --wait       # JSON 格式 + 等待模式
EOF
}

parse_args() {
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --json)
        PRINT_JSON=true
        shift
        ;;
      --wait|--wait=*)
        WAIT_MODE=true
        if [[ "$1" == --wait=* ]]; then
          local val="${1#*=}"
          if [[ -n "$val" ]]; then
            WAIT_TIMEOUT="$val"
          fi
        fi
        shift
        ;;
      --help|-h)
        usage
        exit 0
        ;;
      *)
        echo -e "${RED}未知参数: $1${NC}" >&2
        usage >&2
        exit 3
        ;;
    esac
  done
}

# =============================================================================
# 入口
# =============================================================================
main() {
  parse_args "$@"

  if [[ "$WAIT_MODE" == "true" ]]; then
    wait_mode
    exit $?
  else
    run_all_checks
    emit_output
    # 退出码: 0 全部健康, 1 至少一个服务失败
    if (( FAILED > 0 )); then
      exit 1
    fi
    exit 0
  fi
}

main "$@"
