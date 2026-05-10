#!/usr/bin/env bash
# ==============================================================================
# KeyIP-Intelligence -- Keycloak 自动初始化脚本
# ==============================================================================
# 用法:
#   ./scripts/init-keycloak.sh                    # 默认模式（完整初始化）
#   ./scripts/init-keycloak.sh --check-only       # 仅健康检查
#   ./scripts/init-keycloak.sh --recreate         # 删除并重新创建 realm
# ==============================================================================
# 前置条件:
#   - Keycloak 容器已在运行（docker start keyip-keycloak）
#   - curl, jq 已安装
# ==============================================================================
# 默认值（与 configs/config.yaml 保持一致）
# ==============================================================================
set -euo pipefail

KC_BASE_URL="${KEYCLOAK_URL:-http://localhost:8180}"
KC_REALM="${KEYCLOAK_REALM:-keyip}"
KC_CLIENT_ID="${KEYCLOAK_CLIENT_ID:-keyip-api}"
KC_CLIENT_SECRET="${KEYCLOAK_CLIENT_SECRET:-dev-secret}"
KC_ADMIN_USER="${KEYCLOAK_ADMIN_USER:-admin}"
KC_ADMIN_PASSWORD="${KEYCLOAK_ADMIN_PASSWORD:-admin}"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

_log()    { echo -e "${GREEN}[INFO]${NC}  $*"; }
_warn()   { echo -e "${YELLOW}[WARN]${NC}  $*"; }
_fail()   { echo -e "${RED}[FAIL]${NC}  $*"; exit 1; }
_title()  { echo -e "\n${CYAN}======== $* ========${NC}"; }

# ---------------------------------------------------------------------------
# 工具函数
# ---------------------------------------------------------------------------

# 等待 Keycloak 就绪（最多重试 60 次，间隔 2 秒）
wait_for_keycloak() {
  _title "等待 Keycloak 就绪"
  local retries=60
  local i=0
  until curl -sf "${KC_BASE_URL}/realms/master" >/dev/null 2>&1; do
    i=$((i + 1))
    if [ "$i" -ge "$retries" ]; then
      _fail "Keycloak 未在预期时间内就绪。请确认容器已启动:\n  docker start keyip-keycloak\n  docker logs -f keyip-keycloak"
    fi
    _warn "等待 Keycloak 启动... (${i}/${retries})"
    sleep 2
  done
  _log "Keycloak 已就绪: ${KC_BASE_URL}"
}

# 获取 admin 令牌
get_admin_token() {
  _log "获取 admin 令牌 (realm=master)"
  local resp
  resp=$(curl -sf -X POST "${KC_BASE_URL}/realms/master/protocol/openid-connect/token" \
    -H "Content-Type: application/x-www-form-urlencoded" \
    -d "client_id=admin-cli" \
    -d "username=${KC_ADMIN_USER}" \
    -d "password=${KC_ADMIN_PASSWORD}" \
    -d "grant_type=password") || _fail "获取 admin 令牌失败。请确认管理员凭据正确。"
  ADMIN_TOKEN=$(echo "$resp" | jq -r '.access_token')
  if [ -z "$ADMIN_TOKEN" ] || [ "$ADMIN_TOKEN" = "null" ]; then
    _fail "admin 令牌为空，响应: $(echo "$resp" | jq -c .)"
  fi
  _log "admin 令牌获取成功"
}

# Keycloak Admin API 封装
kc_get() {
  curl -sf -H "Authorization: Bearer ${ADMIN_TOKEN}" -H "Content-Type: application/json" "${KC_BASE_URL}/admin/realms/${KC_REALM}/$1" 2>/dev/null
}

kc_get_master() {
  curl -sf -H "Authorization: Bearer ${ADMIN_TOKEN}" -H "Content-Type: application/json" "${KC_BASE_URL}/admin/realms/master/$1" 2>/dev/null
}

kc_post() {
  curl -sf -X POST -H "Authorization: Bearer ${ADMIN_TOKEN}" -H "Content-Type: application/json" -d "$2" "${KC_BASE_URL}/admin/realms/${KC_REALM}/$1" 2>/dev/null
}

kc_post_master() {
  curl -sf -X POST -H "Authorization: Bearer ${ADMIN_TOKEN}" -H "Content-Type: application/json" -d "$2" "${KC_BASE_URL}/admin/realms/master/$1" 2>/dev/null
}

kc_put() {
  curl -sf -X PUT -H "Authorization: Bearer ${ADMIN_TOKEN}" -H "Content-Type: application/json" -d "$2" "${KC_BASE_URL}/admin/realms/${KC_REALM}/$1" 2>/dev/null
}

kc_delete() {
  curl -sf -X DELETE -H "Authorization: Bearer ${ADMIN_TOKEN}" "${KC_BASE_URL}/admin/realms/${KC_REALM}/$1" 2>/dev/null
}

# ---------------------------------------------------------------------------
# 1. 检查 Keycloak 是否运行
# ---------------------------------------------------------------------------
check_health() {
  _title "1/5  健康检查"
  wait_for_keycloak
  get_admin_token
}

# ---------------------------------------------------------------------------
# 2. 创建 Realm
# ---------------------------------------------------------------------------
create_realm() {
  _title "2/5  创建 Realm: ${KC_REALM}"

  # 检查 realm 是否已存在
  if kc_get "" | jq -e '.realm == "'"${KC_REALM}"'"' >/dev/null 2>&1; then
    _log "Realm '${KC_REALM}' 已存在，跳过创建"
    return 0
  fi

  _log "创建 realm '${KC_REALM}'..."
  kc_post_master "" "$(cat <<JSON
{
  "realm": "${KC_REALM}",
  "enabled": true,
  "displayName": "KeyIP Intelligence",
  "displayNameHtml": "<b>KeyIP Intelligence</b>",
  "loginWithEmailAllowed": false,
  "registrationAllowed": false,
  "resetPasswordAllowed": false,
  "rememberMe": true,
  "sslRequired": "external",
  "accessTokenLifespan": 3600,
  "ssoSessionMaxLifespan": 86400,
  "ssoSessionIdleTimeout": 28800,
  "offlineSessionMaxLifespan": 518400,
  "revokeRefreshToken": true,
  "refreshTokenMaxReuse": 0,
  "defaultSignatureAlgorithm": "RS256",
  "bruteForceProtected": true,
  "failureFactor": 5,
  "waitIncrementSeconds": 60,
  "minimumQuickLoginWaitSeconds": 60,
  "maxDeltaTimeSeconds": 43200,
  "maxFailureWaitSeconds": 900
}
JSON
  )" || {
    _warn "Realm 创建可能失败，尝试检查是否已存在..."
    if kc_get "" | jq -e '.realm' >/dev/null 2>&1; then
      _log "Realm '${KC_REALM}' 已存在"
      return 0
    fi
    _fail "创建 realm 失败"
  }
  _log "Realm '${KC_REALM}' 创建成功"
}

# ---------------------------------------------------------------------------
# 3. 创建客户端
# ---------------------------------------------------------------------------
create_client() {
  _title "3/5  创建客户端: ${KC_CLIENT_ID}"

  # 检查客户端是否已存在
  local existing_client
  existing_client=$(kc_get "clients?clientId=${KC_CLIENT_ID}" | jq -c '.[0] // empty')
  if [ -n "$existing_client" ]; then
    local client_id
    client_id=$(echo "$existing_client" | jq -r '.id')
    _log "客户端 '${KC_CLIENT_ID}' (id=${client_id}) 已存在，跳过创建"
    CLIENT_UUID="$client_id"
    return 0
  fi

  _log "创建客户端 '${KC_CLIENT_ID}'..."
  # POST 创建客户端，返回 Location header 中包含 ID
  local location
  location=$(curl -sf -w '%{redirect_url}' -o /dev/null -X POST \
    -H "Authorization: Bearer ${ADMIN_TOKEN}" \
    -H "Content-Type: application/json" \
    -d "$(cat <<JSON
{
  "clientId": "${KC_CLIENT_ID}",
  "name": "KeyIP API",
  "description": "KeyIP-Intelligence 平台 API 客户端",
  "enabled": true,
  "publicClient": false,
  "secret": "${KC_CLIENT_SECRET}",
  "protocol": "openid-connect",
  "standardFlowEnabled": true,
  "implicitFlowEnabled": false,
  "directAccessGrantsEnabled": true,
  "serviceAccountsEnabled": true,
  "authorizationServicesEnabled": false,
  "redirectUris": ["http://localhost:*", "http://127.0.0.1:*"],
  "webOrigins": ["http://localhost:8080", "http://localhost:3000"],
  "adminUrl": "",
  "fullScopeAllowed": true,
  "attributes": {
    "access.token.lifespan": "3600",
    "post.logout.redirect.uris": "+"
  }
}
JSON
  )" "${KC_BASE_URL}/admin/realms/${KC_REALM}/clients" 2>/dev/null) || true

  # 从 Location 中提取客户端 UUID
  local client_search
  client_search=$(kc_get "clients?clientId=${KC_CLIENT_ID}" | jq -r '.[0].id // empty')
  if [ -z "$client_search" ]; then
    _fail "创建客户端后无法获取其 ID"
  fi
  CLIENT_UUID="$client_search"
  _log "客户端 '${KC_CLIENT_ID}' (id=${CLIENT_UUID}) 创建成功"

  # 为客户端配置 Mappers（添加自定义属性到令牌）
  _log "配置客户端 Mappers..."
  kc_post "clients/${CLIENT_UUID}/protocol-mappers/models" "$(cat <<JSON
{
  "name": "client-roles",
  "protocol": "openid-connect",
  "protocolMapper": "oidc-usermodel-realm-role-mapper",
  "config": {
    "multivalued": true,
    "claim.name": "roles",
    "user.attribute": "roles",
    "access.token.claim": true,
    "id.token.claim": true,
    "jsonType.label": "String"
  }
}
JSON
  )" >/dev/null 2>&1 || _warn "Mapper 'client-roles' 可能已存在"

  kc_post "clients/${CLIENT_UUID}/protocol-mappers/models" "$(cat <<JSON
{
  "name": "client-audience",
  "protocol": "openid-connect",
  "protocolMapper": "oidc-audience-mapper",
  "config": {
    "included.client.audience": "${KC_CLIENT_ID}",
    "id.token.claim": true,
    "access.token.claim": true
  }
}
JSON
  )" >/dev/null 2>&1 || _warn "Mapper 'client-audience' 可能已存在"
  _log "客户端 Mappers 配置完成"
}

# ---------------------------------------------------------------------------
# 4. 创建角色
# ---------------------------------------------------------------------------
create_roles() {
  _title "4/5  创建角色"

  # 定义角色列表（与 internal/infrastructure/auth/keycloak/rbac.go 一致）
  declare -A ROLES
  ROLES[researcher]="研究人员 — 专利检索与告警查看"
  ROLES[ip_manager]="IP 管理员 — 专利全生命周期管理"
  ROLES[executive]="高管 — 报表与仪表板只读访问"
  ROLES[partner_agent]="合作伙伴代理 — 受限范围的专利与分析读取"
  ROLES[super_admin]="超级管理员 — 系统全部权限"

  for role_name in "${!ROLES[@]}"; do
    local desc="${ROLES[$role_name]}"
    # 检查角色是否已存在
    if kc_get "roles/${role_name}" | jq -e '.name' >/dev/null 2>&1; then
      _log "角色 '${role_name}' 已存在，跳过"
    else
      _log "创建角色 '${role_name}' — ${desc}"
      kc_post "roles" "$(cat <<JSON
{
  "name": "${role_name}",
  "description": "${desc}",
  "composite": false,
  "clientRole": false,
  "attributes": {}
}
JSON
      )" || _warn "角色 '${role_name}' 创建可能失败"
    fi
  done
  _log "角色创建完成"
}

# ---------------------------------------------------------------------------
# 5. 创建用户并分配角色
# ---------------------------------------------------------------------------
create_users() {
  _title "5/5  创建测试用户并分配角色"

  # --- 辅助函数：创建用户 ---
  create_single_user() {
    local username="$1"
    local password="$2"
    local role_name="$3"

    # 检查用户是否已存在
    local existing
    existing=$(kc_get "users?username=${username}" | jq -r '.[0].id // empty')
    if [ -n "$existing" ]; then
      _log "用户 '${username}' (id=${existing}) 已存在，跳过创建"
      USER_ID="$existing"
      return 0
    fi

    _log "创建用户 '${username}/${password}' (角色: ${role_name})"
    local location
    location=$(curl -sf -w '%{redirect_url}' -o /dev/null -X POST \
      -H "Authorization: Bearer ${ADMIN_TOKEN}" \
      -H "Content-Type: application/json" \
      -d "$(cat <<JSON
{
  "username": "${username}",
  "enabled": true,
  "emailVerified": true,
  "email": "${username}@keyip.local",
  "firstName": "${username}",
  "credentials": [
    {
      "type": "password",
      "value": "${password}",
      "temporary": false
    }
  ],
  "requiredActions": [],
  "attributes": {}
}
JSON
    )" "${KC_BASE_URL}/admin/realms/${KC_REALM}/users" 2>/dev/null) || true

    # 获取新创建用户的 ID
    local new_id
    new_id=$(kc_get "users?username=${username}" | jq -r '.[0].id // empty')
    if [ -z "$new_id" ]; then
      _warn "无法获取用户 '${username}' 的 ID，跳过角色分配"
      USER_ID=""
      return 1
    fi
    USER_ID="$new_id"
    _log "用户 '${username}' (id=${USER_ID}) 创建成功"
    return 0
  }

  # --- 辅助函数：分配角色 ---
  assign_role_to_user() {
    local user_id="$1"
    local username="$2"
    local role_name="$3"

    if [ -z "$user_id" ]; then
      _warn "跳过用户 '${username}' 的角色分配（用户 ID 为空）"
      return 1
    fi

    # 获取角色信息
    local role_info
    role_info=$(kc_get "roles/${role_name}" | jq -c '{id: .id, name: .name}' 2>/dev/null) || {
      _warn "角色 '${role_name}' 不存在，无法分配给 '${username}'"
      return 1
    }

    local role_id
    role_id=$(echo "$role_info" | jq -r '.id')
    _log "为 '${username}' 分配角色 '${role_name}' (id=${role_id})"

    kc_post "users/${user_id}/role-mappings/realm" "[${role_info}]" >/dev/null 2>&1 || {
      _warn "为用户 '${username}' 分配角色 '${role_name}' 失败"
      return 1
    }
    _log "角色 '${role_name}' 已分配给 '${username}'"
    return 0
  }

  # 创建 admin 用户（super_admin）
  create_single_user "admin" "admin" "super_admin"
  if [ -n "$USER_ID" ]; then
    assign_role_to_user "$USER_ID" "admin" "super_admin"
  fi

  # 创建 researcher 用户（researcher）
  create_single_user "researcher" "researcher" "researcher"
  if [ -n "$USER_ID" ]; then
    assign_role_to_user "$USER_ID" "researcher" "researcher"
  fi

  # 创建 manager 用户（ip_manager）
  create_single_user "manager" "manager" "ip_manager"
  if [ -n "$USER_ID" ]; then
    assign_role_to_user "$USER_ID" "manager" "ip_manager"
  fi

  _log "用户创建与角色分配完成"
}

# ---------------------------------------------------------------------------
# 验证
# ---------------------------------------------------------------------------
verify() {
  _title "验证初始化结果"

  _log "--- Realm ---"
  kc_get "" | jq '{realm: .realm, enabled: .enabled, displayName: .displayName}'

  _log "--- 客户端 ---"
  kc_get "clients?clientId=${KC_CLIENT_ID}" | jq '.[0] | {clientId: .clientId, id: .id, enabled: .enabled, publicClient: .publicClient, serviceAccountsEnabled: .serviceAccountsEnabled, standardFlowEnabled: .standardFlowEnabled, directAccessGrantsEnabled: .directAccessGrantsEnabled}'

  _log "--- 角色 ---"
  for role in researcher ip_manager executive partner_agent super_admin; do
    echo -n "  ${role}: "
    kc_get "roles/${role}" | jq -r '.name // "NOT FOUND"'
  done

  _log "--- 用户与角色映射 ---"
  for user in admin researcher manager; do
    local uid
    uid=$(kc_get "users?username=${user}" | jq -r '.[0].id // "NOT_FOUND"')
    if [ "$uid" != "NOT_FOUND" ]; then
      local roles_assigned
      roles_assigned=$(kc_get "users/${uid}/role-mappings/realm" | jq -r '[.[].name] | join(", ")')
      echo "  ${user} (${uid:0:8}...): [${roles_assigned}]"
    else
      echo "  ${user}: NOT FOUND"
    fi
  done

  _log "--- 测试令牌获取 ---"
  local test_token
  test_token=$(curl -sf -X POST "${KC_BASE_URL}/realms/${KC_REALM}/protocol/openid-connect/token" \
    -H "Content-Type: application/x-www-form-urlencoded" \
    -d "client_id=${KC_CLIENT_ID}" \
    -d "client_secret=${KC_CLIENT_SECRET}" \
    -d "username=admin" \
    -d "password=admin" \
    -d "grant_type=password" 2>/dev/null) || {
    _warn "令牌获取测试失败。请检查客户端和用户配置。"
    return 1
  }

  local access_token
  access_token=$(echo "$test_token" | jq -r '.access_token // empty')
  if [ -n "$access_token" ]; then
    _log "admin 令牌获取成功"
    # 解码 JWT payload 展示角色
    echo "$access_token" | awk -F. '{print $2}' | base64 -d 2>/dev/null | jq '{sub: .sub, preferred_username: .preferred_username, roles: .realm_access.roles // .roles}'
  fi
}

# ---------------------------------------------------------------------------
# 完整重置（删除并重建 realm）
# ---------------------------------------------------------------------------
recreate() {
  _title "重新创建 Realm: ${KC_REALM}"

  # 检查 realm 是否存在
  if kc_get "" | jq -e '.realm' >/dev/null 2>&1; then
    _log "删除现有 realm '${KC_REALM}'..."
    # 先禁用 realm
    kc_put "" '{"enabled": false}' >/dev/null 2>&1 || true
    # 删除 realm
    kc_delete "" >/dev/null 2>&1 || _fail "删除 realm '${KC_REALM}' 失败"
    _log "Realm '${KC_REALM}' 已删除"
  fi

  _log "Realm 已清理，开始重新初始化"
  create_realm
  create_client
  create_roles
  create_users
  verify
}

# ---------------------------------------------------------------------------
# 主流程
# ---------------------------------------------------------------------------
main() {
  echo ""
  echo -e "${CYAN}============================================${NC}"
  echo -e "${CYAN}  KeyIP-Intelligence  Keycloak 自动初始化${NC}"
  echo -e "${CYAN}============================================${NC}"
  echo -e "  URL:    ${KC_BASE_URL}"
  echo -e "  Realm:  ${KC_REALM}"
  echo -e "  Client: ${KC_CLIENT_ID}"
  echo ""

  check_health
  create_realm
  create_client
  create_roles
  create_users
  verify

  _title "Keycloak 初始化完成"
  echo -e "  ${GREEN}管理控制台:${NC}  ${KC_BASE_URL}/admin/master/console/#/${KC_REALM}"
  echo ""
  echo -e "  ${GREEN}测试用户:${NC}"
  echo -e "    admin     / admin      -> super_admin"
  echo -e "    researcher / researcher -> researcher"
  echo -e "    manager   / manager    -> ip_manager"
  echo ""
  echo -e "  ${GREEN}客户端凭据:${NC}"
  echo -e "    client_id:     ${KC_CLIENT_ID}"
  echo -e "    client_secret: ${KC_CLIENT_SECRET}"
  echo ""
  echo -e "  ${YELLOW}令牌测试:${NC}"
  echo -e "    curl -s -X POST ${KC_BASE_URL}/realms/${KC_REALM}/protocol/openid-connect/token \\"
  echo -e "      -H 'Content-Type: application/x-www-form-urlencoded' \\"
  echo -e "      -d 'client_id=${KC_CLIENT_ID}' \\"
  echo -e "      -d 'client_secret=${KC_CLIENT_SECRET}' \\"
  echo -e "      -d 'username=admin' -d 'password=admin' -d 'grant_type=password' | jq ."
  echo ""
  echo -e "  ${CYAN}初始化完成于: $(date '+%Y-%m-%d %H:%M:%S')${NC}"
}

# =============================================================================
# 入口
# =============================================================================

case "${1:-full}" in
  --check-only)
    _title "Keycloak 健康检查"
    wait_for_keycloak
    get_admin_token
    _log "Keycloak 运行正常"
    ;;
  --recreate)
    wait_for_keycloak
    get_admin_token
    recreate
    ;;
  full|--full|"")
    main
    ;;
  *)
    echo "用法: $0 [--check-only | --recreate | --full]"
    echo ""
    echo "  (无参数)   完整初始化（幂等）"
    echo "  --check-only  仅健康检查"
    echo "  --recreate    删除并重建 realm"
    echo "  --full        完整初始化（同上）"
    exit 1
    ;;
esac
