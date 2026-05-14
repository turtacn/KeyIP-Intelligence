# KeyIP-Intelligence CDP Chrome Debug SOP

> **受众**: 前端开发、全栈开发、QA 测试  
> **前置条件**: macOS (本文档以 macOS 为例), Chrome 已安装, Python ≥ 3.11, `websocket-client` 包  
> **核心思路**: 启动独立 Chrome Debug 实例 → 通过 CDP (Chrome DevTools Protocol) 自动遍历页面 → 收集 console 错误 / 网络请求失败 / 页面渲染问题 → 输出诊断报告  
> **适用场景**: 前端页面白屏排查、API 请求异常定位、UI 错误诊断、登录流程 debug

---

## 目录

1. [CDP 原理简述](#1-cdp-原理简述)
2. [环境准备](#2-环境准备)
3. [启动独立 Chrome Debug 实例](#3-启动独立-chrome-debug-实例)
4. [三种调试模式](#4-三种调试模式)
5. [自动化诊断脚本](#5-自动化诊断脚本)
6. [手动 CDP 交互 (REPL)](#6-手动-cdp-交互-repl)
7. [诊断报告解读](#7-诊断报告解读)
8. [常见问题模式与修复](#8-常见问题模式与修复)
9. [附录：CDP 命令速查表](#9-附录cdp-命令速查表)

---

## 1. CDP 原理简述

```
┌─────────────────────┐     WebSocket      ┌─────────────────────┐
│  Python Script      │ ◄────────────────► │  Chrome (Debug)     │
│  (CDP Client)       │   localhost:9222   │  --remote-debugging │
│                     │                    │  --user-data-dir    │
│  • Page.navigate    │                    │                     │
│  • Runtime.evaluate │                    │  KeyIP Web @ :80    │
│  • Network.enable   │                    │  KeyIP API @ :8080  │
│  • Console.enable   │                    │                     │
└─────────────────────┘                    └─────────────────────┘
```

Chrome DevTools Protocol (CDP) 允许外部程序通过 WebSocket 连接 Chrome，以编程方式控制浏览器：导航页面、执行 JavaScript、监听 console 日志、抓取网络请求。这对自动化诊断前端问题极为高效。

### 关键优势

- **不影响日常开发用的 Chrome** — 使用独立 `--user-data-dir` 启动隔离实例
- **无需手动操作** — 脚本自动遍历所有页面，收集错误
- **可复现** — 诊断流程完全脚本化，CI 可集成
- **实时交互** — 可在终端 REPL 中直接 JS 注入、点击元素、检查状态

---

## 2. 环境准备

```bash
# 安装 Python WebSocket 客户端
pip3 install websocket-client

# 确认 Chrome 路径
ls "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
# 输出: /Applications/Google Chrome.app/Contents/MacOS/Google Chrome

# 备选：Chromium
ls "/Applications/Chromium.app/Contents/MacOS/Chromium"
```

### 自定义连接地址

如果你使用 `docker-machine` (如 VirtualBox `192.168.99.100`)，需配置环境变量：

```bash
# 可在 ~/.zshrc 中持久化
export KEYIP_BASE_URL="http://192.168.99.100"
export KEYIP_API_URL="http://192.168.99.100:8080"
```

默认连接 `http://localhost`（Docker Desktop / OrbStack 环境）。

---

## 3. 启动独立 Chrome Debug 实例

### 3.1 手动启动（推荐：保持浏览器打开，自己操作 + 脚本诊断）

```bash
# 【关键】先杀掉已有的 debug 实例（避免端口占用）
pkill -f "remote-debugging-port" 2>/dev/null || true
sleep 1

# 启动独立 Chrome Debug 实例
"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome" \
  --remote-debugging-port=9222 \
  --user-data-dir=/tmp/chrome-debug-profile \
  --no-first-run \
  --no-default-browser-check \
  --disable-extensions \
  --disable-background-networking \
  --disable-sync \
  --disable-translate \
  --disable-features=TranslateUI \
  --window-size=1400,900 \
  "about:blank"
```

### 3.2 命令行参数说明

| 参数 | 说明 |
|------|------|
| `--remote-debugging-port=9222` | 开启 CDP WebSocket 监听端口 |
| `--user-data-dir=/tmp/chrome-debug-profile` | 独立用户目录，与日常 Chrome 完全隔离（不会干扰书签/插件/登录态） |
| `--no-first-run` | 跳过首次启动向导 |
| `--no-default-browser-check` | 不检查是否为默认浏览器 |
| `--disable-extensions` | 禁用插件（避免干扰） |
| `--disable-background-networking` | 减少后台请求噪音 |
| `--disable-sync` | 禁用 Chrome Sync |
| `--disable-translate` / `--disable-features=TranslateUI` | 避免翻译弹窗干扰 |
| `--window-size=1400,900` | 设置视口大小 |
| `"about:blank"` | 启动空白页，减少无关请求 |

### 3.3 验证 Debug 端口

```bash
# 检查 CDP 是否就绪
curl -s http://localhost:9222/json | python3 -m json.tool | head -20
```

预期输出：

```json
[
    {
        "id": "...",
        "type": "page",
        "title": "about:blank",
        "url": "about:blank",
        "webSocketDebuggerUrl": "ws://localhost:9222/devtools/page/..."
    }
]
```

### 3.4 用完后关闭

```bash
pkill -f "chrome-debug-profile"
# 或直接关闭浏览器窗口
```

---

## 4. 三种调试模式

项目提供了 3 个 Python 脚本，覆盖不同使用场景：

### 模式 A：全自动诊断 (`debug_drive.py`)

**用途**: 一键遍历所有页面，生成完整诊断报告

```bash
cd Playwright
python3 debug_drive.py
```

**流程**：
1. 自动启动 Chrome Debug 实例
2. 遍历 11 个页面（Dashboard、Patent Mining、Portfolios、Lifecycle、FTO、Knowledge Graph、Molecule、Partners、Settings、Login、Infringement Watch）
3. 每页收集：Console errors、Failed network requests、Page title、Content elements
4. 测试侧边栏导航、登录流程、TopBar 用户显示
5. 输出 `/tmp/keyip_debug_results.json` 并在终端打印摘要

### 模式 B：附加到已有实例 (`debug_attach.py`)

**用途**: Chrome 已经手动打开（你可以在浏览器中自由操作），脚本附加诊断

```bash
cd Playwright
python3 debug_attach.py
```

**典型工作流**：
1. 按 §3.1 手动启动 Chrome Debug
2. 在 Chrome 中手动操作 — 登录、点击导航、切换页面
3. 运行 `debug_attach.py` 获取当前状态诊断
4. 继续手动操作 + 反复运行脚本

### 模式 C：AppleScript 驱动 (`debug_applescript.py`)

**用途**: macOS 原生 AppleScript 控制 Chrome（无需 WebSocket / CDP port）

```bash
cd Playwright
python3 debug_applescript.py
```

**优势**: 无需 `--remote-debugging-port`，直接控制当前 Chrome
**劣势**: 仅 macOS，速度较慢

### 模式对比速查

| 模式 | 脚本 | 需要 CDP port? | 可交互? | 速度 | 平台 |
|------|------|:---:|:--:|:--:|:--:|
| **A: 全自动** | `debug_drive.py` | ✅ | ❌ | ⚡ 快 | 跨平台 |
| **B: 附加诊断** | `debug_attach.py` | ✅ | ✅ | ⚡ 快 | 跨平台 |
| **C: AppleScript** | `debug_applescript.py` | ❌ | ✅ | 🐢 慢 | 仅 macOS |

**推荐组合**: **模式 B** — 手动启动 Chrome，自己操作调试，同时运行脚本收集日志。

---

## 5. 自动化诊断脚本

### 5.1 核心测试用例

`debug_drive.py` 和 `debug_attach.py` 包含以下诊断逻辑：

| 测试项 | 检测内容 | 关键指标 |
|--------|---------|---------|
| **全页面遍历** | 11 个路由页面是否正常加载 | Page.loadEventFired, title 非空, body 非空 |
| **Console 错误** | JS 运行时异常、未捕获错误、console.error | 收集 `__cdp_errors[]` |
| **网络请求** | 4xx/5xx HTTP 响应、CORS 错误、超时 | `performance.getEntriesByType('resource')` 过滤 |
| **登录流程** | /login 页面渲染 → 填写表单 → 点击 Sign In → token 存储 | Sign In button 存在、token 写入 |
| **侧边栏导航** | 所有 sidebar link 可点击、点击后无错误 | link count, click 执行结果 |
| **TopBar 用户** | 硬编码 "John Doe" 检测、用户信息显示 | `document.body.innerText.includes('John Doe')` |
| **KPI/Metric** | Dashboard 统计卡片是否加载数据 | `[class*='kpi']` 元素数 |
| **API Mode** | localStorage 中 `keyip-api-mode` 值 | mock / proxy / live |

### 5.2 自定义诊断

编辑脚本中的 `PAGES` 字典添加/删除页面：

```python
PAGES = {
    "Dashboard":        "/dashboard",
    "Patent Mining":    "/patent-mining",
    # ... 添加你自己的页面
    "New Feature":      "/new-feature",
}
```

### 5.3 在诊断中注入自定义 JS

```python
def my_custom_check(ws):
    # 检查特定元素是否存在
    send_cdp(ws, "Runtime.evaluate", {
        "expression": "document.querySelectorAll('.my-component').length",
        "returnByValue": True
    })
    result = recv_cdp(ws)
    count = result["result"]["result"].get("value", 0)
    print(f"MyComponent instances: {count}")
```

---

## 6. 手动 CDP 交互 (REPL)

### 6.1 启动交互式 CDP 会话

Chrome Debug 实例启动后，可以直接用 Python REPL 或脚本交互：

```python
import json
from websocket import create_connection
from urllib.request import urlopen

# 获取 WebSocket URL
resp = urlopen("http://localhost:9222/json")
pages = json.loads(resp.read())
ws_url = pages[0]["webSocketDebuggerUrl"]

# 连接
ws = create_connection(ws_url)

# 启用域
def send(method, params=None):
    msg = {"id": 1, "method": method}
    if params: msg["params"] = params
    ws.send(json.dumps(msg))

def recv(timeout=5):
    ws.settimeout(timeout)
    return json.loads(ws.recv())

send("Runtime.enable"); recv()
send("Network.enable"); recv()
send("Page.enable");   recv()

# ─── 导航 ───
send("Page.navigate", {"url": "http://localhost/dashboard"})
# 等待 loadEventFired...
recv(15)

# ─── 执行 JS ───
send("Runtime.evaluate", {"expression": "document.title", "returnByValue": True})
result = recv()
print(result["result"]["result"]["value"])  # -> "KeyIP Intelligence"

# ─── 检查特定元素 ───
send("Runtime.evaluate", {
    "expression": "document.querySelector('h1')?.textContent",
    "returnByValue": True
})
print(recv()["result"]["result"]["value"])

# ─── 收集所有 console 错误 ───
send("Runtime.evaluate", {
    "expression": """
    (window.__cdp_errors = []);
    const orig = console.error;
    console.error = (...args) => {
        window.__cdp_errors.push(args.map(String).join(' '));
        orig.apply(console, args);
    };
    'ok'
    """,
    "returnByValue": True
})
recv()

# 稍后获取错误
send("Runtime.evaluate", {
    "expression": "JSON.stringify(window.__cdp_errors || [])",
    "returnByValue": True
})
errors = recv()
print(errors["result"]["result"]["value"])
```

### 6.2 常用交互操作速查

```python
# 点击元素
send("Runtime.evaluate", {
    "expression": "document.querySelector('.signin-btn')?.click()",
    "returnByValue": True
})

# 填写输入框 (React 受控组件)
send("Runtime.evaluate", {
    "expression": """
    const el = document.querySelector('input[type="email"]');
    const setter = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value').set;
    setter.call(el, 'turta@keyip.io');
    el.dispatchEvent(new Event('input', {bubbles: true}));
    'filled'
    """,
    "returnByValue": True
})

# 截图 (获取视口截图 base64)
send("Page.captureScreenshot", {"format": "png"})
data = recv()["result"]["data"]  # base64 PNG

# 检查 localStorage
send("Runtime.evaluate", {
    "expression": "localStorage.getItem('keyip_token')",
    "returnByValue": True
})

# 检查网络请求
send("Runtime.evaluate", {
    "expression": """
    JSON.stringify(performance.getEntriesByType('resource')
        .filter(r => r.name.includes('/api/'))
        .map(r => ({url: r.name, status: r.responseStatus, duration: r.duration})))
    """,
    "returnByValue": True
})
```

---

## 7. 诊断报告解读

### 7.1 报告结构

```json
{
  "errors": [],           // Console 错误列表
  "warnings": [],         // Console 警告
  "pages": {},            // 每页诊断结果
  "network_failures": [], // 失败的 HTTP 请求
  "findings": []          // 关键发现（登录、用户信息等）
}
```

### 7.2 终端输出示例

```
═══════════════════════════════════════════════════════════════
📄 Pages tested: 11
  ✅ Dashboard: title='Executive Dashboard - KeyIP', errors=0, failed_req=0
  ✅ Patent Mining: title='Patent Mining - KeyIP', errors=0, failed_req=0
  ✅ Infringement Watch: title='Infringement Watch - KeyIP', errors=0, failed_req=2
  ✅ Portfolio: title='Portfolio Optimizer - KeyIP', errors=0, failed_req=0
  ✅ Lifecycle: title='Patent Lifecycle - KeyIP', errors=0, failed_req=0
  ✅ FTO Search: title='FTO Search - KeyIP', errors=0, failed_req=0
  ✅ Knowledge Graph: title='Knowledge Graph - KeyIP', errors=0, failed_req=0
  ✅ Molecule: title='Molecule Detail - KeyIP', errors=2, failed_req=0
  ✅ Partners: title='Partners - KeyIP', errors=0, failed_req=0
  ✅ Settings: title='Settings - KeyIP', errors=0, failed_req=0
  ✅ Login: title='Login - KeyIP', errors=0, failed_req=0

❌ CONSOLE ERRORS (2):
  [Molecule] Warning: RDKit minimal library not loaded
  [Infringement Watch] Failed to load resource: the server responded with status of 404

⚠️ NETWORK FAILURES (2):
  [Infringement Watch] /api/v1/infringement/alerts → 404
  [Infringement Watch] /api/v1/infringement/watch → 404

🔑 LOGIN FLOW:
  Login form detected ✓
  Sign-in click: 'clicked'
  After login URL: /dashboard
  Token stored: YES
  'John Doe' visible: false

👤 USER INFO:
  Display name: turta (not John Doe) ✓
  localStorage: keyip_token, keyip_token_expiry, keyip_user_info
```

### 7.3 分级判定

| 严重度 | 判定标准 | 行动 |
|:------:|---------|------|
| 🟢 PASS | 所有页面 errors=0, network_failures=0 | 继续其他验证 |
| 🟡 WARN | 有 console.warn 或非关键 API 404 | 记录 issue，根据优先级修复 |
| 🔴 FAIL | 页面白屏、JS 运行时异常、关键 API 500 | **立即修复**，阻塞发布 |

---

## 8. 常见问题模式与修复

### 8.1 页面白屏 / Skeleton 一直加载

**CDP 表现**: `Page.loadEventFired` 触发，但 `body.innerText` 为空或仅含 Skeleton 动画

**诊断步骤**:
```python
# 检查 console 错误
send("Runtime.evaluate", {
    "expression": "JSON.stringify(window.__cdp_errors || [])",
    "returnByValue": True
})

# 检查 React 根节点
send("Runtime.evaluate", {
    "expression": "document.getElementById('root').innerHTML.substring(0, 500)",
    "returnByValue": True
})
```

**常见原因**:
1. API 返回格式不匹配（前端期望 `{code, message, data}` 但收到 `[]`）
2. useAuth 在 mock 模式下仍尝试 Keycloak redirect
3. i18n 翻译 key 缺失导致 React 渲染异常

### 8.2 "No access_token in response" (登录失败)

**CDP 表现**: `/api/v1/auth/signin` 返回 200 但 body 不含 access_token

**诊断步骤**:
```python
# 拦截登录请求
send("Network.enable", {"maxPostDataSize": 65536})
# 导航到登录页 → 填写表单 → 点击 Sign In
# 查看 Network.responseReceived
```

**常见原因**:
1. API Server 不在 proxy 模式 — Nginx stub 返回了固定 JSON
2. JWT secret 不匹配 — API Server 用随机 secret 但前端期望固定 secret
3. Content-Type header 不对 — 前端发了 `multipart/form-data`

### 8.3 Console Error: "Cannot read properties of undefined (reading 'data')"

**CDP 表现**: `Runtime.exceptionThrown` → 前端尝试读取 API 响应的 `.data` 但 API 返回了裸数据

**诊断步骤**: 检查 API 适配器是否对 response 做了正确的解包
```python
send("Runtime.evaluate", {
    "expression": """
    fetch('/api/v1/dashboard/metrics').then(r => r.json()).then(d => {
        return JSON.stringify({has_code: 'code' in d, has_data: 'data' in d, keys: Object.keys(d)});
    })
    """,
    "returnByValue": True
})
```

### 8.4 硬编码 "John Doe" 检测

```python
# 检测是否仍显示 John Doe
has_john = eval_js("document.body.innerText.includes('John Doe')")
# 检测是否显示了登录用户信息
has_real_user = eval_js("document.body.innerText.includes('turta')")
```

**修复**: 检查 `web/src/components/layout/Sidebar.tsx` 和 `TopBar.tsx` — 若 `isAuthenticated` 为 false 且 API 模式非 mock，应显示登录用户信息。

### 8.5 网络请求 CORS 错误

**CDP 表现**: `Network.responseReceived` status=0, `Network.loadingFailed` with `blockedReason: cors`

```python
# 检测 CORS
send("Runtime.evaluate", {
    "expression": """
    JSON.stringify(performance.getEntriesByType('resource')
        .filter(r => r.responseStatus === 0 && r.name.includes('/api/'))
        .map(r => r.name))
    """,
    "returnByValue": True
})
```

**常见原因**:
1. 前端在 `localhost:5173` 但 API 在 `localhost:8080` — 需 Vite proxy 配置
2. Docker 容器环境未配置 CORS middleware — 检查 `cmd/apiserver/main.go` 中 CORSMiddleware 是否启用

---

## 9. 附录：CDP 命令速查表

| Domain | Method | 用途 |
|--------|--------|------|
| `Page` | `enable` | 启用页面事件 |
| `Page` | `navigate` | 导航到 URL |
| `Page` | `captureScreenshot` | 截图 (base64) |
| `Page` | `reload` | 刷新页面 |
| `Runtime` | `enable` | 启用运行时 |
| `Runtime` | `evaluate` | 执行 JS 表达式 |
| `Runtime` | `awaitPromise` | 等待 Promise resolve |
| `Network` | `enable` | 启用网络监听 |
| `Network` | `getResponseBody` | 获取响应体 |
| `Console` | `enable` | 启用 console 消息 |
| `Log` | `enable` | 启用日志条目 |
| `Log` | `startViolationsReport` | 长任务/重排/重绘监控 |

### 诊断涉及的 CDP 事件

| Event | 触发时机 |
|-------|---------|
| `Page.loadEventFired` | 页面 load 完成 |
| `Page.frameStoppedLoading` | frame 停止加载 |
| `Runtime.consoleAPICalled` | console.log/warn/error 调用 |
| `Runtime.exceptionThrown` | 未捕获异常 |
| `Log.entryAdded` | 任何日志条目 (含 console + 浏览器内部) |
| `Network.requestWillBeSent` | 请求即将发送 |
| `Network.responseReceived` | 收到响应头 |
| `Network.loadingFinished` | 请求完成 |
| `Network.loadingFailed` | 请求失败 |

---

## 总结：典型调试闭环

```
1. 发现问题（页面白屏 / 登录失败 / 数据显示异常）
          │
2. 启动 CDP Chrome Debug 实例
   $ pkill -f "remote-debugging-port"
   $ "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome" \
     --remote-debugging-port=9222 --user-data-dir=/tmp/chrome-debug-profile &
          │
3. 在 Chrome Debug 窗口中手动复现问题
          │
4. 运行诊断脚本收集证据
   $ cd Playwright && python3 debug_attach.py
          │
5. 解读 /tmp/keyip_debug_results.json
          │
6. 修复代码 → 刷新 Chrome → 重新运行步骤 4
          │
7. 确认所有页面 errors=0, network_failures=0
          │
8. 关闭 Chrome Debug 实例
   $ pkill -f "chrome-debug-profile"
```
