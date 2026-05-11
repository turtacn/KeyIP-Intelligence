#!/usr/bin/env bash
# ==============================================================================
# KeyIP-Intelligence -- 真实数据库初始化与验证流程
# ==============================================================================
# 按顺序执行以下步骤：
#   1. 等待所有外部服务健康就绪
#   2. 执行 PostgreSQL 数据库迁移
#   3. 加载种子数据到所有数据源
#   4. 创建 OpenSearch 索引（索引模板与别名）
#   5. 最终验证汇总
#
# 用法:
#   ./scripts/init-all.sh                     # 完整初始化
#   ./scripts/init-all.sh --skip-health       # 跳过健康检查（服务已运行时）
#   ./scripts/init-all.sh --skip-seed         # 跳过种子数据加载
#   ./scripts/init-all.sh --skip-opensearch   # 跳过 OpenSearch 索引创建
#   ./scripts/init-all.sh --skip-migration    # 跳过数据库迁移
#   ./scripts/init-all.sh --recreate          # 重建模式（clean + 重新初始化）
#   ./scripts/init-all.sh --help              # 显示帮助
#
# 特性:
#   - 幂等：可重复执行，不会重复创建已存在的对象
#   - 每步显示进度和结果
#   - 失败时给出清晰的排查指引
# ==============================================================================

set -euo pipefail

# ---------------------------------------------------------------------------
# 颜色与样式
# ---------------------------------------------------------------------------
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

OK="  ${GREEN}${BOLD}✓${NC}"
FAIL="  ${RED}${BOLD}✗${NC}"
WARN="  ${YELLOW}${BOLD}⚠${NC}"
INFO="  ${CYAN}→${NC}"

# ---------------------------------------------------------------------------
# 脚本位置与项目根目录
# ---------------------------------------------------------------------------
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

cd "${PROJECT_ROOT}"

# ---------------------------------------------------------------------------
# 默认配置
# ---------------------------------------------------------------------------
SKIP_HEALTH=false
SKIP_MIGRATION=false
SKIP_SEED=false
SKIP_OPENSEARCH=false
RECREATE=false

# ---------------------------------------------------------------------------
# 帮助函数
# ---------------------------------------------------------------------------
usage() {
  cat <<EOF
用法: $0 [选项]

选项:
  --skip-health       跳过健康检查（服务已运行时使用）
  --skip-migration    跳过数据库迁移
  --skip-seed         跳过种子数据加载
  --skip-opensearch   跳过 OpenSearch 索引创建
  --recreate          重建模式（先清理再初始化）
  --help              显示此帮助信息

示例:
  $0                         # 完整初始化
  $0 --skip-health           # 跳过健康检查
  $0 --skip-seed             # 不加载种子数据
  $0 --recreate              # 重建所有数据
EOF
  exit 0
}

# ---------------------------------------------------------------------------
# 参数解析
# ---------------------------------------------------------------------------
parse_args() {
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --skip-health)     SKIP_HEALTH=true;      shift ;;
      --skip-migration)  SKIP_MIGRATION=true;    shift ;;
      --skip-seed)       SKIP_SEED=true;         shift ;;
      --skip-opensearch) SKIP_OPENSEARCH=true;   shift ;;
      --recreate)        RECREATE=true;          shift ;;
      --help)            usage ;;
      *)
        echo -e "${RED}未知参数: $1${NC}"
        usage
        ;;
    esac
  done
}

# ---------------------------------------------------------------------------
# 计时辅助
# ---------------------------------------------------------------------------
START_TIME=0
step_start() { START_TIME=$(date +%s%N); }
step_end() {
  local elapsed
  elapsed=$(( ($(date +%s%N) - START_TIME) / 1000000 ))
  if (( elapsed >= 60000 )); then
    printf "%d 分 %d 秒" $(( elapsed / 60000 )) $(( (elapsed % 60000) / 1000 ))
  elif (( elapsed >= 1000 )); then
    printf "%d 秒" $(( elapsed / 1000 ))
  else
    printf "%d 毫秒" "${elapsed}"
  fi
}

# ---------------------------------------------------------------------------
# 标题与分隔线
# ---------------------------------------------------------------------------
title() {
  echo ""
  echo -e "${CYAN}══════════════════════════════════════════════════════════════${NC}"
  echo -e "${CYAN}  $1${NC}"
  echo -e "${CYAN}══════════════════════════════════════════════════════════════${NC}"
}

subtitle() {
  printf "\n${BOLD}--- %s ---${NC}\n" "$1"
}

# ---------------------------------------------------------------------------
# 计算汇总
# ---------------------------------------------------------------------------
TOTAL_STEPS=4
COMPLETED=0
FAILED_STEPS=0

record_success() {
  COMPLETED=$((COMPLETED + 1))
  echo -e "${OK} $1 (耗时: $(step_end))"
}

record_failure() {
  FAILED_STEPS=$((FAILED_STEPS + 1))
  echo -e "${FAIL} $1 (耗时: $(step_end))"
}

# ============================================================================
# 步骤 1: 健康检查
# ============================================================================
step_health_check() {
  title "[1/${TOTAL_STEPS}] 等待所有外部服务健康就绪"

  local health_script="${SCRIPT_DIR}/health-check.sh"
  if [[ ! -x "${health_script}" ]]; then
    echo -e "${FAIL} health-check.sh 不存在或不可执行: ${health_script}"
    echo -e "${WARN} 请确保 scripts/health-check.sh 存在且有执行权限"
    return 1
  fi

  echo -e "${INFO} 正在检查 PostgreSQL、Redis、Neo4j、OpenSearch、Milvus、MinIO、Kafka、Keycloak、MailHog ..."
  echo -e "${INFO} 首次启动可能需要 60-120 秒等待所有容器就绪"
  echo ""

  if ! bash "${health_script}" --wait; then
    echo -e "${FAIL} 部分服务未能在超时时间内就绪"
    echo -e "${WARN} 排查建议:"
    echo -e "${WARN}   1. 确认 Docker 容器正在运行: docker ps"
    echo -e "${WARN}   2. 查看容器日志: docker logs <container-name>"
    echo -e "${WARN}   3. 手动运行: bash ${health_script}"
    return 1
  fi

  record_success "所有 9 个外部服务均已就绪"
}

# ============================================================================
# 步骤 2: 数据库迁移
# ============================================================================
step_migration() {
  title "[2/${TOTAL_STEPS}] 执行 PostgreSQL 数据库迁移"

  local migrate_script="${SCRIPT_DIR}/migrate.sh"
  local migrations_dir="internal/infrastructure/database/postgres/migrations"

  # 检查迁移文件是否存在
  if ! ls "${migrations_dir}"/*.sql &>/dev/null; then
    echo -e "${WARN} 未找到迁移文件 (${migrations_dir})"
    echo -e "${WARN} 跳过迁移步骤"
    record_success "数据库迁移（无变更文件，跳过）"
    return 0
  fi

  # 优先使用 golang-migrate CLI
  if command -v migrate &>/dev/null; then
    echo -e "${INFO} 使用 golang-migrate CLI 执行迁移..."
    echo -e "${INFO} 迁移目录: ${migrations_dir}"

    step_start

    # 从 configs/config.yaml 提取连接信息
    local host port user password dbname db_url

    if [[ -f "configs/config.yaml" ]]; then
      host=$(grep -A10 "postgres:" configs/config.yaml | grep "host:" | awk '{print $2}' | tr -d '"' || echo "localhost")
      port=$(grep -A10 "postgres:" configs/config.yaml | grep "port:" | awk '{print $2}' | tr -d '"' || echo "5432")
      user=$(grep -A10 "postgres:" configs/config.yaml | grep "user:" | awk '{print $2}' | tr -d '"' || echo "keyip")
      password=$(grep -A10 "postgres:" configs/config.yaml | grep "password:" | awk '{print $2}' | tr -d '"' || echo "keyip_dev")
      dbname=$(grep -A10 "postgres:" configs/config.yaml | grep "dbname:" | awk '{print $2}' | tr -d '"' || echo "keyip_dev")
    else
      echo -e "${WARN} configs/config.yaml 不存在，使用默认连接参数"
      host="localhost"; port="5432"; user="keyip"; password="keyip_dev"; dbname="keyip_dev"
    fi

    db_url="postgres://${user}:${password}@${host}:${port}/${dbname}?sslmode=disable"

    # 检查连接是否可达
    if ! PGPASSWORD="${password}" psql -h "${host}" -p "${port}" -U "${user}" -d "${dbname}" -c "SELECT 1" &>/dev/null; then
      echo -e "${WARN} PostgreSQL 连接失败，检查服务是否就绪"
      echo -e "${WARN}   host=${host} port=${port} user=${user} dbname=${dbname}"
      return 1
    fi

    # 查询当前迁移状态
    local current_version dirty
    current_version=$(migrate -path "${migrations_dir}" -database "${db_url}" version 2>/dev/null || echo "none")
    if [[ "${current_version}" != "none" ]]; then
      echo -e "${INFO} 当前迁移版本: ${current_version}"
    else
      echo -e "${INFO} 尚未应用任何迁移"
    fi

    # 应用迁移 (幂等：golang-migrate 自动跳过已应用的迁移)
    if bash "${migrate_script}" up; then
      # 获取迁移后的版本
      local new_version
      new_version=$(migrate -path "${migrations_dir}" -database "${db_url}" version 2>/dev/null || echo "unknown")
      record_success "数据库迁移完成 (版本: ${new_version})"
    else
      local exit_code=$?
      echo -e "${FAIL} 数据库迁移失败 (退出码: ${exit_code})"
      echo -e "${WARN} 排查建议:"
      echo -e "${WARN}   1. 检查迁移文件语法: migrate -path ${migrations_dir} -database \"${db_url}\" up"
      echo -e "${WARN}   2. 手动检查迁移状态: migrate -path ${migrations_dir} -database \"${db_url}\" version"
      echo -e "${WARN}   3. 如为 dirty 状态需修复: migrate -path ${migrations_dir} -database \"${db_url}\" force <version>"
      return 1
    fi
  else
    # 回退方案：使用 go run 直接调用 migrator
    echo -e "${WARN} migrate CLI 未安装，尝试使用 Go 运行 migrator..."
    echo -e "${INFO} 可通过以下命令安装: go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest"

    if ! command -v go &>/dev/null; then
      echo -e "${FAIL} go 命令不可用，无法执行迁移"
      echo -e "${WARN} 请安装 Go 1.22+ 或 golang-migrate CLI"
      return 1
    fi

    step_start

    # 直接运行 migrator 包中的 RunMigrations
    # 使用一个简单的 Go 程序调用 RunMigrations
    if go run "${PROJECT_ROOT}/cmd/migrate/main.go" 2>/dev/null; then
      record_success "数据库迁移完成（go run）"
    elif go run -mod=mod "${PROJECT_ROOT}/internal/infrastructure/database/postgres/migrator.go" 2>/dev/null; then
      record_success "数据库迁移完成（go run migrator）"
    else
      echo -e "${FAIL} 无法通过 Go 运行迁移"
      echo -e "${WARN} 排查建议:"
      echo -e "${WARN}   1. 安装 migrate CLI: make tools"
      echo -e "${WARN}   2. 或手动执行迁移: bash ${migrate_script} up"
      return 1
    fi
  fi
}

# ============================================================================
# 步骤 3: 种子数据加载
# ============================================================================
step_seed() {
  title "[3/${TOTAL_STEPS}] 加载种子数据"

  local seed_script="${SCRIPT_DIR}/seed.sh"

  if [[ ! -f "${seed_script}" ]]; then
    echo -e "${WARN} seed.sh 不存在: ${seed_script}"
    echo -e "${WARN} 跳过种子数据加载"
    record_success "种子数据加载（跳过，脚本不存在）"
    return 0
  fi

  if [[ ! -x "${seed_script}" ]]; then
    echo -e "${INFO} 设置 seed.sh 可执行权限"
    chmod +x "${seed_script}"
  fi

  # 检查种子数据文件是否存在
  local data_dir="test/testdata"
  local fixtures_dir="${data_dir}/fixtures"
  local missing=0

  for f in "${fixtures_dir}/molecule_fixtures.json" \
           "${fixtures_dir}/patent_fixtures.json" \
           "${fixtures_dir}/portfolio_fixtures.json"; do
    if [[ ! -f "$f" ]]; then
      echo -e "${WARN} 种子数据文件缺失: $f"
      missing=1
    fi
  done

  if [[ "${missing}" -eq 1 ]]; then
    echo -e "${WARN} 部分种子数据文件缺失，但 seed.sh 内部会跳过缺失文件"
    echo -e "${WARN} 你可以手动运行 bash ${seed_script} 查看详细信息"
  fi

  step_start

  # 如果是 --recreate 模式，传递 --clean
  local seed_args=""
  if [[ "${RECREATE}" == "true" ]]; then
    seed_args="--clean"
    echo -e "${INFO} 重建模式：清理现有数据后重新加载"
  fi

  if bash "${seed_script}" ${seed_args} --target all; then
    record_success "种子数据加载完成（所有数据源）"
  else
    local exit_code=$?
    echo -e "${FAIL} 种子数据加载失败 (退出码: ${exit_code})"
    echo -e "${WARN} 排查建议:"
    echo -e "${WARN}   1. 检查种子数据文件是否存在: ls -la ${fixtures_dir}/"
    echo -e "${WARN}   2. 确认目标服务运行正常"
    echo -e "${WARN}   3. 手动运行: bash ${seed_script}"
    return 1
  fi
}

# ============================================================================
# 步骤 4: OpenSearch 索引创建
# ============================================================================
step_opensearch() {
  title "[4/${TOTAL_STEPS}] 创建 OpenSearch 索引"

  local os_script="${SCRIPT_DIR}/init-opensearch.sh"

  if [[ ! -f "${os_script}" ]]; then
    echo -e "${WARN} init-opensearch.sh 不存在: ${os_script}"
    echo -e "${WARN} 跳过 OpenSearch 索引创建"
    record_success "OpenSearch 索引创建（跳过，脚本不存在）"
    return 0
  fi

  if [[ ! -x "${os_script}" ]]; then
    chmod +x "${os_script}"
  fi

  step_start

  if [[ "${RECREATE}" == "true" ]]; then
    echo -e "${INFO} 重建模式：删除并重建 OpenSearch 索引"
    if bash "${os_script}" --recreate; then
      record_success "OpenSearch 索引重建完成"
    else
      echo -e "${FAIL} OpenSearch 索引重建失败"
      echo -e "${WARN} 手动执行: bash ${os_script} --recreate"
      return 1
    fi
  else
    # 幂等模式：索引已存在则跳过
    if bash "${os_script}"; then
      record_success "OpenSearch 索引初始化完成（幂等）"
    else
      local exit_code=$?
      echo -e "${FAIL} OpenSearch 索引初始化失败 (退出码: ${exit_code})"
      echo -e "${WARN} 手动执行: bash ${os_script}"
      return 1
    fi
  fi
}

# ============================================================================
# 最终验证
# ============================================================================
final_verify() {
  title "验证汇总"

  echo -e "${INFO} 步骤完成情况:"
  echo ""
  echo -e "  ${CYAN}1.${NC} 健康检查         $([[ "${SKIP_HEALTH}" == "true" ]] && echo "${WARN} 已跳过" || echo "${OK} 完成")"
  echo -e "  ${CYAN}2.${NC} 数据库迁移       $([[ "${SKIP_MIGRATION}" == "true" ]] && echo "${WARN} 已跳过" || echo "${OK} 完成")"
  echo -e "  ${CYAN}3.${NC} 种子数据加载     $([[ "${SKIP_SEED}" == "true" ]] && echo "${WARN} 已跳过" || echo "${OK} 完成")"
  echo -e "  ${CYAN}4.${NC} OpenSearch 索引  $([[ "${SKIP_OPENSEARCH}" == "true" ]] && echo "${WARN} 已跳过" || echo "${OK} 完成")"

  echo ""

  if [[ "${FAILED_STEPS}" -gt 0 ]]; then
    echo -e "${FAIL} ${FAILED_STEPS} 个步骤失败，请根据上方错误信息排查。"
    return 1
  fi

  echo -e "${OK} ${BOLD}所有步骤已完成！数据库初始化成功。${NC}"

  # 验证 PostgreSQL 连接
  subtitle "PostgreSQL 验证"
  if command -v psql &>/dev/null; then
    local pg_host pg_port pg_user pg_password pg_dbname
    if [[ -f "configs/config.yaml" ]]; then
      pg_host=$(grep -A10 "postgres:" configs/config.yaml | grep "host:" | awk '{print $2}' | tr -d '"' || echo "localhost")
      pg_port=$(grep -A10 "postgres:" configs/config.yaml | grep "port:" | awk '{print $2}' | tr -d '"' || echo "5432")
      pg_user=$(grep -A10 "postgres:" configs/config.yaml | grep "user:" | awk '{print $2}' | tr -d '"' || echo "keyip")
      pg_password=$(grep -A10 "postgres:" configs/config.yaml | grep "password:" | awk '{print $2}' | tr -d '"' || echo "keyip_dev")
      pg_dbname=$(grep -A10 "postgres:" configs/config.yaml | grep "dbname:" | awk '{print $2}' | tr -d '"' || echo "keyip_dev")
    else
      pg_host="localhost"; pg_port="5432"; pg_user="keyip"; pg_password="keyip_dev"; pg_dbname="keyip_dev"
    fi

    export PGHOST="${pg_host}" PGPORT="${pg_port}" PGUSER="${pg_user}" PGPASSWORD="${pg_password}" PGDATABASE="${pg_dbname}"

    # 检查迁移版本
    local migrations_dir="internal/infrastructure/database/postgres/migrations"
    if command -v migrate &>/dev/null && [[ -d "${migrations_dir}" ]]; then
      local db_url="postgres://${pg_user}:${pg_password}@${pg_host}:${pg_port}/${pg_dbname}?sslmode=disable"
      local ver
      ver=$(migrate -path "${migrations_dir}" -database "${db_url}" version 2>/dev/null || echo "N/A")
      echo -e "${INFO} 迁移版本: ${ver}"
    fi

    # 检查表是否创建
    echo -e "${INFO} 数据库表清单:"
    psql -c "\dt" 2>/dev/null || echo -e "${WARN}  无法查询表清单"
  else
    echo -e "${WARN} psql 未安装，跳过 PostgreSQL 验证"
  fi

  # 验证 OpenSearch 索引
  subtitle "OpenSearch 验证"
  local os_url="${OPENSEARCH_URL:-http://localhost:9200}"
  local os_auth="${OPENSEARCH_AUTH:-}"
  if command -v curl &>/dev/null; then
    if curl -sf ${os_auth} "${os_url}/_cat/indices?format=json" 2>/dev/null | head -1 >/dev/null; then
      echo -e "${INFO} 索引列表:"
      curl -sf ${os_auth} "${os_url}/_cat/indices?format=json" 2>/dev/null | \
        jq -r '.[] | "  \(.index) │ docs: \(.docs.count) │ status: \(.health)"' 2>/dev/null || \
        echo -e "${WARN}  无法解析索引列表"
    else
      echo -e "${WARN}  OpenSearch 不可达: ${os_url}"
    fi
  else
    echo -e "${WARN} curl 未安装，跳过 OpenSearch 验证"
  fi

  # 验证种子数据
  subtitle "种子数据文件"
  echo -e "${INFO} 数据文件位置:"
  echo "  test/testdata/fixtures/"
  for f in molecule_fixtures.json patent_fixtures.json portfolio_fixtures.json; do
    local path="test/testdata/fixtures/${f}"
    if [[ -f "${path}" ]]; then
      echo -e "  ${OK} ${f} ($(wc -c < "${path}" 2>/dev/null || echo "?") 字节)"
    else
      echo -e "  ${WARN} ${f} 不存在"
    fi
  done

  echo ""
  echo -e "${GREEN}${BOLD}========================================${NC}"
  echo -e "${GREEN}${BOLD}  KeyIP-Intelligence 数据库初始化完成${NC}"
  echo -e "${GREEN}${BOLD}  时间: $(date '+%Y-%m-%d %H:%M:%S')${NC}"
  echo -e "${GREEN}${BOLD}========================================${NC}"
}

# ============================================================================
# 主流程
# ============================================================================
main() {
  parse_args "$@"

  echo -e "${CYAN}============================================${NC}"
  echo -e "${CYAN}  KeyIP-Intelligence  数据库初始化与验证  ${NC}"
  echo -e "${CYAN}  工作目录: ${PROJECT_ROOT}${NC}"
  echo -e "${CYAN}  模式: $([[ "${RECREATE}" == "true" ]] && echo "重建 (--recreate)" || echo "幂等 (安全重复执行)")${NC}"
  echo -e "${CYAN}============================================${NC}"
  echo ""

  local skipped_count=0
  local ran_count=0

  # --- 步骤 1: 健康检查 ---
  if [[ "${SKIP_HEALTH}" == "true" ]]; then
    echo -e "${WARN} [1/${TOTAL_STEPS}] 健康检查已跳过 (--skip-health)"
    skipped_count=$((skipped_count + 1))
  else
    step_start
    if step_health_check; then
      ran_count=$((ran_count + 1))
    else
      record_failure "健康检查失败"
    fi
  fi

  # --- 步骤 2: 数据库迁移 ---
  if [[ "${SKIP_MIGRATION}" == "true" ]]; then
    echo -e "${WARN} [2/${TOTAL_STEPS}] 数据库迁移已跳过 (--skip-migration)"
    skipped_count=$((skipped_count + 1))
  else
    step_start
    if step_migration; then
      ran_count=$((ran_count + 1))
    else
      record_failure "数据库迁移失败"
    fi
  fi

  # --- 步骤 3: 种子数据 ---
  if [[ "${SKIP_SEED}" == "true" ]]; then
    echo -e "${WARN} [3/${TOTAL_STEPS}] 种子数据加载已跳过 (--skip-seed)"
    skipped_count=$((skipped_count + 1))
  else
    step_start
    if step_seed; then
      ran_count=$((ran_count + 1))
    else
      record_failure "种子数据加载失败"
    fi
  fi

  # --- 步骤 4: OpenSearch 索引 ---
  if [[ "${SKIP_OPENSEARCH}" == "true" ]]; then
    echo -e "${WARN} [4/${TOTAL_STEPS}] OpenSearch 索引创建已跳过 (--skip-*)"
    skipped_count=$((skipped_count + 1))
  else
    step_start
    if step_opensearch; then
      ran_count=$((ran_count + 1))
    else
      record_failure "OpenSearch 索引创建失败"
    fi
  fi

  echo ""
  echo -e "${CYAN}────────────────────────────────────────────${NC}"

  # 最终验证（无论是否有步骤失败）
  if final_verify; then
    echo ""
    echo -e "${OK} ${BOLD}初始化流程全部成功 (运行 ${ran_count}, 跳过 ${skipped_count}, 失败 ${FAILED_STEPS})${NC}"
    exit 0
  else
    echo ""
    echo -e "${FAIL} ${BOLD}初始化流程存在失败步骤 (运行 ${ran_count}, 跳过 ${skipped_count}, 失败 ${FAILED_STEPS})${NC}"
    exit 1
  fi
}

main "$@"
