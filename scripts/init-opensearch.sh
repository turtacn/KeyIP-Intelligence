#!/usr/bin/env bash
# ==============================================================================
# KeyIP-Intelligence -- OpenSearch 索引初始化脚本
# ==============================================================================
# 用法:
#   ./scripts/init-opensearch.sh                    # 默认模式（完整初始化）
#   ./scripts/init-opensearch.sh --check-only       # 仅健康检查
#   ./scripts/init-opensearch.sh --recreate         # 删除并重建索引
# ==============================================================================
# 前置条件:
#   - OpenSearch 容器已在运行
#   - curl, jq 已安装
#   - IK Analysis 插件已安装（用于中文分词）
# ==============================================================================
# 环境变量覆写:
#   OPENSEARCH_URL      默认 http://localhost:9200
#   OPENSEARCH_AUTH     可选 Basic Auth (-u user:pass)
#   PATENT_INDEX        默认 patents-v1
#   MOLECULE_INDEX      默认 molecules-v1
#   PATENT_ALIAS        默认 patents
#   MOLECULE_ALIAS      默认 molecules
# ==============================================================================

set -euo pipefail

# ---------------------------------------------------------------------------
# 默认值
# ---------------------------------------------------------------------------
OS_URL="${OPENSEARCH_URL:-http://localhost:9200}"
OS_AUTH="${OPENSEARCH_AUTH:-}"
PATENT_INDEX="${PATENT_INDEX:-patents-v1}"
MOLECULE_INDEX="${MOLECULE_INDEX:-molecules-v1}"
PATENT_ALIAS="${PATENT_ALIAS:-patents}"
MOLECULE_ALIAS="${MOLECULE_ALIAS:-molecules}"

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
# OpenSearch REST API 封装
# ---------------------------------------------------------------------------
ES_GET() {
  curl -sf ${OS_AUTH} -H "Content-Type: application/json" "${OS_URL}$1" 2>/dev/null
}

ES_PUT() {
  curl -sf -X PUT ${OS_AUTH} -H "Content-Type: application/json" -d "$2" "${OS_URL}$1" 2>/dev/null
}

ES_POST() {
  curl -sf -X POST ${OS_AUTH} -H "Content-Type: application/json" -d "$2" "${OS_URL}$1" 2>/dev/null
}

ES_DELETE() {
  curl -sf -X DELETE ${OS_AUTH} -H "Content-Type: application/json" "${OS_URL}$1" 2>/dev/null
}

# ---------------------------------------------------------------------------
# 1. 等待 OpenSearch 就绪
# ---------------------------------------------------------------------------
wait_for_opensearch() {
  _title "1/6  等待 OpenSearch 就绪"
  local retries=60
  local i=0
  until curl -sf ${OS_AUTH} "${OS_URL}/_cluster/health" >/dev/null 2>&1; do
    i=$((i + 1))
    if [ "$i" -ge "$retries" ]; then
      _fail "OpenSearch 未在预期时间内就绪。请确认容器已启动:\n  docker start keyip-opensearch\n  docker logs -f keyip-opensearch"
    fi
    _warn "等待 OpenSearch 启动... (${i}/${retries})"
    sleep 2
  done
  local health
  health=$(ES_GET "/_cluster/health" | jq -r '.status')
  _log "OpenSearch 已就绪 (${OS_URL})，集群状态: ${health}"

  # 检查 IK 插件是否可用
  if ES_GET "/_cat/plugins?h=component&format=json" | jq -e '.[] | select(.component | test("ik"; "i"))' >/dev/null 2>&1; then
    _log "IK Analysis 插件已安装"
  else
    _warn "未检测到 IK Analysis 插件。中文分词将回退到 standard 分析器。"
    _warn "如需中文支持，请安装: docker exec keyip-opensearch /usr/share/opensearch/bin/opensearch-plugin install https://github.com/medcl/elasticsearch-analysis-ik/releases/download/v7.17.0/elasticsearch-analysis-ik-7.17.0.zip"
  fi
}

# ---------------------------------------------------------------------------
# 2. 检查索引是否存在
# ---------------------------------------------------------------------------
index_exists() {
  local index="$1"
  local code
  code=$(curl -s -o /dev/null -w "%{http_code}" ${OS_AUTH} "${OS_URL}/${index}")
  [ "$code" = "200" ]
}

alias_exists() {
  local alias="$1"
  local code
  code=$(curl -s -o /dev/null -w "%{http_code}" ${OS_AUTH} "${OS_URL}/_alias/${alias}")
  [ "$code" = "200" ]
}

# ---------------------------------------------------------------------------
# 3. 创建专利索引
# ---------------------------------------------------------------------------
create_patent_index() {
  _title "2/6  创建专利索引: ${PATENT_INDEX}"

  if index_exists "${PATENT_INDEX}"; then
    _log "索引 '${PATENT_INDEX}' 已存在，跳过创建"
    return 0
  fi

  local body
  body=$(cat <<JSON
{
  "settings": {
    "number_of_shards": 3,
    "number_of_replicas": 1,
    "index.refresh_interval": "30s",
    "analysis": {
      "analyzer": {
        "ik_max_word": {
          "type": "custom",
          "tokenizer": "ik_max_word"
        }
      }
    }
  },
  "mappings": {
    "dynamic": "strict",
    "properties": {
      "patent_number": {
        "type": "keyword"
      },
      "title": {
        "type": "text",
        "analyzer": "ik_max_word",
        "search_analyzer": "ik_smart",
        "fields": {
          "keyword": {
            "type": "keyword"
          }
        }
      },
      "abstract": {
        "type": "text",
        "analyzer": "ik_max_word",
        "search_analyzer": "ik_smart"
      },
      "claims": {
        "type": "text"
      },
      "assignee": {
        "type": "keyword"
      },
      "inventors": {
        "type": "keyword"
      },
      "filing_date": {
        "type": "date",
        "format": "yyyy-MM-dd||yyyy-MM-dd HH:mm:ss||epoch_millis"
      },
      "publication_date": {
        "type": "date",
        "format": "yyyy-MM-dd||yyyy-MM-dd HH:mm:ss||epoch_millis"
      },
      "ipc_codes": {
        "type": "keyword"
      },
      "cpc_codes": {
        "type": "keyword"
      },
      "legal_status": {
        "type": "keyword"
      },
      "full_text": {
        "type": "text",
        "analyzer": "ik_max_word",
        "search_analyzer": "ik_smart"
      },
      "tech_domain": {
        "type": "keyword"
      },
      "cited_patents": {
        "type": "keyword"
      },
      "family_ids": {
        "type": "keyword"
      },
      "created_at": {
        "type": "date",
        "format": "yyyy-MM-dd HH:mm:ss||epoch_millis"
      },
      "updated_at": {
        "type": "date",
        "format": "yyyy-MM-dd HH:mm:ss||epoch_millis"
      }
    }
  }
}
JSON
  )

  ES_PUT "/${PATENT_INDEX}" "${body}" | jq '.' || _fail "创建索引 '${PATENT_INDEX}' 失败"
  _log "专利索引 '${PATENT_INDEX}' 创建成功"
}

# ---------------------------------------------------------------------------
# 4. 创建分子索引
# ---------------------------------------------------------------------------
create_molecule_index() {
  _title "3/6  创建分子索引: ${MOLECULE_INDEX}"

  if index_exists "${MOLECULE_INDEX}"; then
    _log "索引 '${MOLECULE_INDEX}' 已存在，跳过创建"
    return 0
  fi

  local body
  body=$(cat <<JSON
{
  "settings": {
    "number_of_shards": 3,
    "number_of_replicas": 1,
    "index.refresh_interval": "30s"
  },
  "mappings": {
    "dynamic": "strict",
    "properties": {
      "smiles": {
        "type": "keyword"
      },
      "inchi": {
        "type": "keyword"
      },
      "inchi_key": {
        "type": "keyword"
      },
      "molecular_formula": {
        "type": "keyword"
      },
      "molecular_weight": {
        "type": "float"
      },
      "exact_mass": {
        "type": "float"
      },
      "name": {
        "type": "text",
        "fields": {
          "keyword": {
            "type": "keyword"
          }
        }
      },
      "synonyms": {
        "type": "text"
      },
      "source_patents": {
        "type": "keyword"
      },
      "fingerprint": {
        "type": "binary",
        "doc_values": true
      },
      "created_at": {
        "type": "date",
        "format": "yyyy-MM-dd HH:mm:ss||epoch_millis"
      },
      "updated_at": {
        "type": "date",
        "format": "yyyy-MM-dd HH:mm:ss||epoch_millis"
      }
    }
  }
}
JSON
  )

  ES_PUT "/${MOLECULE_INDEX}" "${body}" | jq '.' || _fail "创建索引 '${MOLECULE_INDEX}' 失败"
  _log "分子索引 '${MOLECULE_INDEX}' 创建成功"
}

# ---------------------------------------------------------------------------
# 5. 设置索引别名
# ---------------------------------------------------------------------------
setup_aliases() {
  _title "4/6  设置索引别名"

  local actions="["
  local need_update=false

  # --- 专利别名 ---
  if alias_exists "${PATENT_ALIAS}"; then
    local current_idx
    current_idx=$(ES_GET "/_alias/${PATENT_ALIAS}" | jq -r 'keys[]' 2>/dev/null)
    _log "别名 '${PATENT_ALIAS}' 已指向 '${current_idx}'，跳过"
  else
    actions+="{\"add\":{\"index\":\"${PATENT_INDEX}\",\"alias\":\"${PATENT_ALIAS}\",\"is_write_index\":true}},"
    _log "将创建别名 '${PATENT_ALIAS}' -> '${PATENT_INDEX}'"
    need_update=true
  fi

  # --- 分子别名 ---
  if alias_exists "${MOLECULE_ALIAS}"; then
    local current_idx
    current_idx=$(ES_GET "/_alias/${MOLECULE_ALIAS}" | jq -r 'keys[]' 2>/dev/null)
    _log "别名 '${MOLECULE_ALIAS}' 已指向 '${current_idx}'，跳过"
  else
    actions+="{\"add\":{\"index\":\"${MOLECULE_INDEX}\",\"alias\":\"${MOLECULE_ALIAS}\",\"is_write_index\":true}},"
    _log "将创建别名 '${MOLECULE_ALIAS}' -> '${MOLECULE_INDEX}'"
    need_update=true
  fi

  actions="${actions%,}]"

  if [ "$need_update" = true ]; then
    ES_POST "/_aliases" "{\"actions\":${actions}}" | jq '.' || _warn "别名创建部分失败"
    _log "别名设置完成"
  else
    _log "所有别名已存在，无需更新"
  fi
}

# ---------------------------------------------------------------------------
# 6. 设置刷新间隔
# ---------------------------------------------------------------------------
set_refresh_interval() {
  _title "5/6  设置索引刷新间隔"

  local interval="${REFRESH_INTERVAL:-30s}"

  for idx in "${PATENT_INDEX}" "${MOLECULE_INDEX}"; do
    if index_exists "${idx}"; then
      local body
      body=$(cat <<JSON
{
  "index": {
    "refresh_interval": "${interval}"
  }
}
JSON
      )
      ES_PUT "/${idx}/_settings" "${body}" >/dev/null && \
        _log "索引 '${idx}' refresh_interval = ${interval}"
    else
      _warn "索引 '${idx}' 不存在，跳过 refresh_interval 设置"
    fi
  done
}

# ---------------------------------------------------------------------------
# 7. 验证索引创建结果
# ---------------------------------------------------------------------------
verify() {
  _title "6/6  验证索引创建结果"

  echo ""
  _log "--- 索引列表 ---"
  ES_GET "/_cat/indices?format=json&expand_wildcards=all" | jq -r '.[] | "  \(.index) │ docs: \(.docs.count) │ size: \(.store.size) │ status: \(.health)"'

  echo ""
  _log "--- 别名列表 ---"
  ES_GET "/_alias?format=json" | jq -r '
    to_entries[]
    | select(.key | startswith(".") | not)
    | "  \(.key) → " + ([.value.aliases | keys[]] | join(", "))
  '

  echo ""
  _log "--- 专利索引 Mapping (字段摘要) ---"
  ES_GET "/${PATENT_INDEX}/_mapping" | jq -r '
    .[].mappings.properties
    | to_entries[]
    | "  \(.key): \(.value.type // "object")"
  ' 2>/dev/null || _warn "无法读取专利索引 mapping"

  echo ""
  _log "--- 分子索引 Mapping (字段摘要) ---"
  ES_GET "/${MOLECULE_INDEX}/_mapping" | jq -r '
    .[].mappings.properties
    | to_entries[]
    | "  \(.key): \(.value.type // "object")"
  ' 2>/dev/null || _warn "无法读取分子索引 mapping"

  echo ""
  _log "--- 索引设置 (refresh_interval / shards) ---"
  for idx in "${PATENT_INDEX}" "${MOLECULE_INDEX}"; do
    echo "  ${idx}:"
    ES_GET "/${idx}/_settings" | jq -r '
      .[].settings.index
      | "    refresh_interval: \(.refresh_interval // "N/A")\n    shards: \(.number_of_shards)\n    replicas: \(.number_of_replicas)"
    ' 2>/dev/null || _warn "无法读取 ${idx} 设置"
  done
}

# ---------------------------------------------------------------------------
# 清理（--recreate 模式使用）
# ---------------------------------------------------------------------------
recreate() {
  _title "重新创建索引与别名"

  # 删除别名（需要先移除别名再删索引）
  for alias in "${PATENT_ALIAS}" "${MOLECULE_ALIAS}"; do
    if alias_exists "${alias}"; then
      local idx
      idx=$(ES_GET "/_alias/${alias}" | jq -r 'keys[]' 2>/dev/null)
      if [ -n "${idx}" ]; then
        _log "移除别名 '${alias}' (指向 '${idx}')"
        ES_POST "/_aliases" "{\"actions\":[{\"remove\":{\"index\":\"${idx}\",\"alias\":\"${alias}\"}}]}" >/dev/null || true
      fi
    fi
  done

  # 删除索引
  for idx in "${PATENT_INDEX}" "${MOLECULE_INDEX}"; do
    if index_exists "${idx}"; then
      _log "删除索引 '${idx}'"
      ES_DELETE "/${idx}" >/dev/null || _warn "删除索引 '${idx}' 失败"
    fi
  done

  _log "索引已全部清理，开始重新创建"
  create_patent_index
  create_molecule_index
  setup_aliases
  set_refresh_interval
  verify

  _title "重建完成"
}

# ---------------------------------------------------------------------------
# 主流程
# ---------------------------------------------------------------------------
main() {
  echo ""
  echo -e "${CYAN}============================================${NC}"
  echo -e "${CYAN}  KeyIP-Intelligence  OpenSearch 索引初始化${NC}"
  echo -e "${CYAN}============================================${NC}"
  echo -e "  URL:              ${OS_URL}"
  echo -e "  专利索引:          ${PATENT_INDEX} -> ${PATENT_ALIAS}"
  echo -e "  分子索引:          ${MOLECULE_INDEX} -> ${MOLECULE_ALIAS}"
  echo ""

  wait_for_opensearch
  create_patent_index
  create_molecule_index
  setup_aliases
  set_refresh_interval
  verify

  _title "OpenSearch 索引初始化完成"
  echo -e "  ${GREEN}索引别名:${NC}"
  echo -e "    ${PATENT_ALIAS}   -> ${PATENT_INDEX}"
  echo -e "    ${MOLECULE_ALIAS} -> ${MOLECULE_INDEX}"
  echo ""
  echo -e "  ${GREEN}验证:${NC}"
  echo -e "    curl -s ${OS_URL}/${PATENT_ALIAS}/_count | jq ."
  echo -e "    curl -s ${OS_URL}/${MOLECULE_ALIAS}/_count | jq ."
  echo ""
  echo -e "  ${CYAN}初始化完成于: $(date '+%Y-%m-%d %H:%M:%S')${NC}"
}

# =============================================================================
# 入口
# =============================================================================

case "${1:-full}" in
  --check-only)
    _title "OpenSearch 健康检查"
    wait_for_opensearch
    _log "OpenSearch 运行正常"
    ;;
  --recreate)
    wait_for_opensearch
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
    echo "  --recreate    删除并重建索引"
    echo "  --full        完整初始化（同上）"
    exit 1
    ;;
esac
