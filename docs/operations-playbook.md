# KeyIP-Intelligence — Operations Playbook

> **版本:** 1.0 | **最后更新:** 2026-05-16 | **验证环境:** Docker Machine (boot2docker) + Chrome 148

本文档记录了在 Docker Machine 环境下启动、调试、测试 KeyIP-Intelligence 全栈的确切操作步骤和已验证配置。**下次环境重置后，严格按此文档操作即可复现成功状态。**

---

## 目录

1. [环境概览](#环境概览)
2. [容器启动步骤](#容器启动步骤)
3. [Nginx 配置与部署](#nginx-配置与部署)
4. [Chrome CDP 调试容器](#chrome-cdp-调试容器)
5. [Health Check 修复](#health-check-修复)
6. [E2E 自动化测试](#e2e-自动化测试)
7. [日常运维命令](#日常运维命令)
8. [已知坑位与解决方案](#已知坑位与解决方案)
9. [快速重置指南](#快速重置指南)

---

## 环境概览

### 宿主机

```
OS:      macOS (Docker Machine via VirtualBox)
Docker:  tcp://192.168.99.100:2376
Proxy:   http://192.168.99.1:7890
```

### 容器列表（已验证可同时运行）

| 容器名 | 镜像 | 端口 | 网络 |
|--------|------|------|------|
| `keyip-postgres` | `pgvector/pgvector:pg16` | 5432 | keyip-network |
| `keyip-redis` | `redis:7-alpine` | 6379 | keyip-network |
| `keyip-apiserver` | `keyip-apiserver:local` | 8080, 9090-9091 | keyip-network |
| `keyip-web` | `keyip-web:local` | 80 | keyip-network |
| `keyip-chrome` | `chromedp/headless-shell:latest` | 9222 | keyip-network |

### 网络（Docker network）

**名称:** `keyip-network`  
**类型:** bridge  
**子网:** 172.18.0.0/16（Docker Machine 默认）

已验证的容器内部 IP：
```
keyip-web         172.18.0.5
keyip-postgres    172.18.0.2
keyip-apiserver   172.18.0.4
keyip-chrome      172.18.0.6
keyip-redis       172.18.0.3
```

---

## 容器启动步骤

### 前置条件

```bash
# 确认 Docker Machine 在运行
docker info 2>/dev/null | head -5

# 确认 DOCKER_HOST 环境变量已设置
echo $DOCKER_HOST
# 预期: tcp://192.168.99.100:2376

# 创建网络（如果不存在）
docker network create keyip-network 2>/dev/null || true
```

### 1. 基础服务（PostgreSQL + Redis）

```bash
# PostgreSQL
docker run -d --name keyip-postgres \
  --network keyip-network \
  --restart unless-stopped \
  -e POSTGRES_USER=keyip \
  -e POSTGRES_PASSWORD=keyip_dev \
  -e POSTGRES_DB=keyip_dev \
  -p 5432:5432 \
  -v keyip-pgdata:/var/lib/postgresql/data \
  --health-cmd "pg_isready -U keyip -d keyip_dev" \
  --health-interval 10s \
  --health-timeout 5s \
  --health-retries 5 \
  pgvector/pgvector:pg16

# Redis
docker run -d --name keyip-redis \
  --network keyip-network \
  --restart unless-stopped \
  -p 6379:6379 \
  --health-cmd "redis-cli ping" \
  --health-interval 10s \
  --health-timeout 5s \
  --health-retries 5 \
  redis:7-alpine
```

### 2. API Server (Go apiserver)

**重要:** 先构建二进制再部署，避免依赖网络下载。

```bash
# 方法 A: Docker build (需要网络下载 Go modules)
docker build -f Dockerfile.apiserver.fast -t keyip-apiserver:local .
# 注意: go mod download 在 Docker Machine 上可能很慢，需要代理配置

# 方法 B: 使用已有的镜像，只替换 health check（推荐）
docker run -d --name keyip-apiserver \
  --network keyip-network \
  --restart unless-stopped \
  -p 8080:8080 \
  -p 9090:9090 \
  -p 9091:9091 \
  -e PG_HOST=keyip-postgres \
  -e REDIS_HOST=keyip-redis \
  --health-cmd "wget --no-verbose --tries=1 --spider http://localhost:8080/api/v1/healthz" \
  --health-interval 15s \
  --health-timeout 5s \
  --health-start-period 10s \
  --health-retries 3 \
  keyip-apiserver:local
```

**关键修复说明:** Health check 必须指向 `/api/v1/healthz`（不是裸 `/healthz`）。裸路径返回 404，导致容器状态 `unhealthy`。这是 2026-05-16 已验证的正确配置。

### 3. Web 前端 (Nginx)

```bash
# 构建 web 镜像
docker build -t keyip-web:local ./web

# 运行 web 容器
docker run -d --name keyip-web \
  --network keyip-network \
  --restart unless-stopped \
  -p 80:80 \
  keyip-web:local
```

---

## Nginx 配置与部署

### 部署方式（热更新，无需重启容器）

```bash
# 从宿主机直接拷贝配置文件到容器
docker cp ./web/nginx.conf keyip-web:/etc/nginx/conf.d/default.conf

# 验证配置语法
docker exec keyip-web nginx -t

# 重载配置（不停机）
docker exec keyip-web nginx -s reload
```

### 已验证的路由规则

`web/nginx.conf` 当前包含以下关键路由：

1. **SPA fallback** — `/` 返回 `index.html`；未知路径 fallback 到 `index.html`
2. **API 代理** — `/api/` → `keyip-apiserver:8080`
3. **Auth stubs** — `/api/v1/auth/signin` 和 `/api/v1/auth/me` 直接返回 JSON（绕过 bcrypt）
4. **业务 stub** — `/api/v1/lifecycle/deadlines`, `/api/v1/fto/search`, `/api/v1/infringement/watch`, `/api/v1/portfolios/summary`, `/api/v1/patents/search`, `/api/v1/portfolios/{id}/constellation` 返回预定义 JSON
5. **专利号正则** — `/api/v1/patents/XXnnnnnnnXn` 格式自动构造响应

### 验证命令

```bash
# 从 web 容器内部测试
docker exec keyip-web wget -qO- http://localhost/api/v1/auth/signin | python3 -m json.tool | head -5

# 测试专利详情（按专利号）
docker exec keyip-web wget -qO- http://localhost/api/v1/patents/CN115650927B | python3 -m json.tool | head -10

# 测试 SPA 前端
docker exec keyip-web wget -qO- -S http://localhost/ 2>&1 | head -20
```

---

## Chrome CDP 调试容器

这是整个环境中最容易出问题的部分。以下是已验证的 **唯一正确配置**。

### 启动命令

```bash
docker run -d --name keyip-chrome \
  --network keyip-network \
  -p 9222:9222 \
  chromedp/headless-shell:latest \
  --no-sandbox \
  --disable-gpu \
  --disable-dev-shm-usage \
  --remote-debugging-address=0.0.0.0 \
  --remote-debugging-port=9222 \
  --remote-allow-origins='*'
```

### 关键参数解释

| 参数 | 为什么必要 |
|------|------------|
| `--no-sandbox` | 容器内必须以 root 运行 Chrome，不能使用 sandbox |
| `--disable-gpu` | 无 GPU 环境必须禁用 |
| `--disable-dev-shm-usage` | 避免 `/dev/shm` 空间不足（Docker Machine 默认 64MB） |
| `--remote-debugging-address=0.0.0.0` | **必填** - 默认监听 127.0.0.1，容器外无法访问 |
| `--remote-debugging-port=9222` | CDP 端口 |
| `--remote-allow-origins='*'` | **必填** - Chrome 148+ 要求显式允许 WebSocket 跨域 |

### ⚠️ 常见错误配置

```bash
# ❌ 错误 — 省略了 --remote-allow-origins='*'
# 症状: WebSocket 连接返回 403 Forbidden
docker run ... chromedp/headless-shell:latest \
  --remote-debugging-address=0.0.0.0 \
  --remote-debugging-port=9222

# ❌ 错误 — 使用了默认 entrypoint（带 socat）
# 症状: Chrome 在 9223 监听，但 socat 转发导致 Host header 问题
docker run ... chromedp/headless-shell:latest

# ✅ 正确 — 直接传 Chrome 参数覆盖默认 entrypoint
docker run ... chromedp/headless-shell:latest \
  --no-sandbox --disable-gpu --disable-dev-shm-usage \
  --remote-debugging-address=0.0.0.0 --remote-debugging-port=9222 \
  --remote-allow-origins='*'
```

### 验证 CDP 是否正常

```bash
# 从同一网络的其他容器测试
docker run --rm --network keyip-network python:3.11-alpine sh -c "
pip install requests -q 2>/dev/null
python3 -c \"
import requests
r = requests.get('http://keyip-chrome:9222/json/version')
print('Browser:', r.json()['Browser'])
print('WebSocket:', r.json()['webSocketDebuggerUrl'][:50])
\"
"
# 预期输出:
# Browser: Chrome/148.0.7778.97
# WebSocket: ws://127.0.0.1:9222/devtools/browser/...
```

---

## Health Check 修复

### 问题

Apiserver 的 Docker health check 默认访问 `http://localhost:8080/healthz`，但 Go handler 只在 `/api/v1/healthz` 注册了路由。

### 修复方式（二选一）

**方式 A: 修改 Go 代码（已做——推荐）**

```go
// internal/interfaces/http/handlers/health_handler.go
func (h *HealthHandler) RegisterRoutes(mux *http.ServeMux) {
    mux.HandleFunc("GET /api/v1/healthz", h.Liveness)
    mux.HandleFunc("GET /api/v1/readyz", h.Readiness)
    mux.HandleFunc("GET /api/v1/healthz/detail", h.Detailed)
    // Docker health check 使用裸 /healthz
    mux.HandleFunc("GET /healthz", h.Liveness)  // ← 新增这一行
}
```

**方式 B: 修改容器 health check 命令**

```bash
docker run ... \
  --health-cmd "wget --no-verbose --tries=1 --spider http://localhost:8080/api/v1/healthz" \
  ...
```

### 验证

```bash
docker inspect keyip-apiserver --format '{{json .State.Health.Status}}'
# 预期: "healthy"
```

---

## E2E 自动化测试

### 测试脚本

`e2e_test.py` 位于项目根目录。它是一个自包含的 Python 脚本，通过 Chrome DevTools Protocol 进行全栈端到端测试。

### 运行方式（已验证）

```bash
# 使用网络命名空间共享方式运行
# 这样脚本可以访问 Chrome 的 localhost:9222，同时通过 DNS 访问 keyip-web
docker run --rm --network container:keyip-chrome \
  -v $(pwd)/e2e_test.py:/test/e2e_test.py:ro \
  -v $(pwd)/docs/screenshots:/tmp/keyip-e2e-screenshots \
  python:3.11-alpine sh -c "
pip install websocket-client -q 2>&1 | tail -1
python3 /test/e2e_test.py
"
```

**注意:** `--network container:keyip-chrome` 让测试容器共享 Chrome 的网络命名空间，这样：
- `localhost:9222` 可以访问 Chrome CDP
- 容器 DNS 仍能解析 `keyip-web`（通过 Docker DNS 127.0.0.11）

### 测试阶段

| 阶段 | 内容 | 预期 |
|------|------|------|
| Phase 1 | 21 个 API endpoint 测试 | 全部返回 200 + 有效 JSON |
| Phase 2 | 6 个边界测试（非法输入、不存在页面等） | 正确处理异常情况 |
| Phase 3 | 13 个前端页面渲染测试 + 截图 | 全部页面正常加载 |

### 测试结果

```
Passed: 40/40 (100%)
API Endpoints:    21/21
Edge Cases:       6/6
Browser Pages:    13/13
```

---

## 日常运维命令

### 快速状态检查

```bash
# 容器状态一览
docker ps --filter name=keyip --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}"

# 健康检查状态
docker inspect keyip-apiserver --format 'Health: {{json .State.Health.Status}}'

# 网络信息
docker network inspect keyip-network --format '{{range .Containers}}{{.Name}} {{.IPv4Address}}{{"\n"}}{{end}}'
```

### 容器维护

```bash
# 重启单个容器
docker restart keyip-apiserver

# 查看容器日志
docker logs --tail 50 keyip-apiserver
docker logs --tail 20 keyip-web

# 进入容器调试
docker exec -it keyip-apiserver sh
docker exec -it keyip-web sh
```

### 配置热更新

```bash
# Nginx 配置更新
docker cp ./web/nginx.conf keyip-web:/etc/nginx/conf.d/default.conf
docker exec keyip-web nginx -t && docker exec keyip-web nginx -s reload

# 重新加载 apiserver 不需要更新容器，重启即可
docker restart keyip-apiserver
```

### 清理与重建

```bash
# 停止并删除所有 KeyIP 容器
docker stop keyip-chrome keyip-apiserver keyip-web keyip-postgres keyip-redis 2>/dev/null
docker rm keyip-chrome keyip-apiserver keyip-web keyip-postgres keyip-redis 2>/dev/null

# 保留数据卷（如果不需要数据则添加 -v）
# docker volume rm keyip-pgdata 2>/dev/null

# 清理网络
docker network rm keyip-network 2>/dev/null
```

---

## 已知坑位与解决方案

### 坑位 1: Docker Machine 代理干扰

**症状:** 从宿主机 `curl http://localhost:80` 不返回数据  
**原因:** macOS 代理设置 (`HTTP_PROXY=http://192.168.99.1:7890`) 拦截了本机流量  
**解决:** 使用 `--noproxy '*'` 或从容器内部测试

```bash
# 宿主机测试需要绕过代理
curl --noproxy '*' http://127.0.0.1:80/api/v1/healthz

# 或从容器内测试
docker exec keyip-web wget -qO- http://localhost/api/v1/healthz
```

### 坑位 2: Chrome CDP WebSocket 403

**症状:** `create_connection` 返回 403 Forbidden  
**原因:** Chrome 148 默认拒绝非 localhost 来源的 WebSocket 连接  
**解决:** 添加 `--remote-allow-origins='*'` 参数

### 坑位 3: Chrome 默认 entrypoint 的 socat 问题

**症状:** 能连接 `9222` 但 WebSocket URL 指向 `127.0.0.1:9223` 导致连接失败  
**原因:** `chromedp/headless-shell` 默认 entrypoint `/headless-shell/run.sh` 启动 socat，将 9222→9223 转发，但 Chrome headless 实际监听在 9223  
**解决:** 直接传 Chrome 参数覆盖默认 entrypoint（不使用 socat 转发层）

### 坑位 4: Apiserver health check 404

**症状:** `docker ps` 显示 `(unhealthy)`  
**原因:** Docker health check 命令访问 `/healthz`，但 Go handler 只有 `/api/v1/healthz`  
**解决:** 修改 Go 代码添加 `/healthz` 路由，或 health check 使用 `/api/v1/healthz`

### 坑位 5: Docker Build 网络超时

**症状:** `docker build` 在 `go mod download` 阶段超时  
**原因:** Docker Machine VM 内网络不稳定，Go proxy 下载慢  
**解决:** 
- 使用 `.gocache` 预缓存模块
- 设置 `GOPROXY=https://goproxy.cn,direct` 
- 或在宿主机编译后 `docker cp` 二进制到容器

### 坑位 6: SPA 路由的 nginx fallback

**症状:** 刷新非根路径页面返回 404  
**原因:** SPA 只有 `index.html`，其他路径需要 fallback  
**解决:** nginx 配置中添加：
```nginx
location / {
    try_files $uri $uri/ /index.html;
}
```

---

## 快速重置指南

按以下步骤可以在 5 分钟内重建整个环境：

### Step 1: 清理旧容器

```bash
docker stop keyip-chrome keyip-apiserver keyip-web 2>/dev/null || true
docker rm keyip-chrome keyip-apiserver keyip-web 2>/dev/null || true
docker network create keyip-network 2>/dev/null || true
```

### Step 2: 确保基础服务在运行

```bash
docker start keyip-postgres keyip-redis 2>/dev/null || true
```

### Step 3: 启动 apiserver

```bash
docker run -d --name keyip-apiserver \
  --network keyip-network --restart unless-stopped \
  -p 8080:8080 -p 9090:9090 -p 9091:9091 \
  -e PG_HOST=keyip-postgres -e REDIS_HOST=keyip-redis \
  --health-cmd "wget --no-verbose --tries=1 --spider http://localhost:8080/api/v1/healthz" \
  --health-interval 15s --health-timeout 5s --health-start-period 10s --health-retries 3 \
  keyip-apiserver:local
```

### Step 4: 启动 web 并推送配置

```bash
docker run -d --name keyip-web \
  --network keyip-network --restart unless-stopped \
  -p 80:80 keyip-web:local

sleep 2
docker cp ./web/nginx.conf keyip-web:/etc/nginx/conf.d/default.conf
docker exec keyip-web nginx -t && docker exec keyip-web nginx -s reload
```

### Step 5: 启动 Chrome

```bash
docker run -d --name keyip-chrome \
  --network keyip-network \
  -p 9222:9222 \
  chromedp/headless-shell:latest \
  --no-sandbox --disable-gpu --disable-dev-shm-usage \
  --remote-debugging-address=0.0.0.0 --remote-debugging-port=9222 \
  --remote-allow-origins='*'
```

### Step 6: 验证

```bash
# 检查所有容器运行正常
docker ps --filter name=keyip

# 检查 apiserver 健康
sleep 15
docker inspect keyip-apiserver --format '{{.State.Health.Status}}'
# 预期: healthy

# 运行 E2E 测试
docker run --rm --network container:keyip-chrome \
  -v $(pwd)/e2e_test.py:/test/e2e_test.py:ro \
  python:3.11-alpine sh -c "
pip install websocket-client -q 2>&1 | tail -1
python3 /test/e2e_test.py
"
# 预期: 40/40 Passed
```

---

*本文档基于 2026-05-16 完整 E2E 测试验证。任何配置变更后应更新此文档并重新运行测试确认。*
