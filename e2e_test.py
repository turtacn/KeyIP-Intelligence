#!/usr/bin/env python3
"""
KeyIP Intelligence — E2E Test Suite (Self-contained)
Runs inside --network container:keyip-chrome for Chrome access.
Auto-detects web server IP.
"""
import json, time, base64, os, socket, urllib.request, urllib.error
from websocket import create_connection

# Detect web server IP via Docker DNS or env
def detect_web():
    for host in ["keyip-web"]:
        try:
            ip = socket.gethostbyname(host)
            print(f"Web server resolved: {host} -> {ip}")
            return f"http://{ip}"
        except:
            pass
    # Fallback: try common Docker IPs
    for ip in ["172.18.0.5", "172.17.0.5", "172.18.0.2"]:
        try:
            urllib.request.urlopen(f"http://{ip}/api/v1/healthz", timeout=2)
            print(f"Web server found at: {ip}")
            return f"http://{ip}"
        except:
            pass
    print("FATAL: Cannot find web server")
    return None

BASE = None
CHROME = "http://127.0.0.1:9222"
SCREENSHOT_DIR = "/tmp/keyip-e2e-screenshots"

def cdp(path, method="GET"):
    req = urllib.request.Request(f"{CHROME}{path}", method=method)
    try:
        with urllib.request.urlopen(req, timeout=10) as r:
            return json.loads(r.read().decode())
    except Exception as e:
        print(f"  [CDP ERR] {path}: {e}")
        return None

def new_tab(url="about:blank"):
    return cdp(f"/json/new?url={url}", method="PUT")

def ws_connect(ws_url):
    # Chrome listens on port 9222 directly (no socat)
    # We share Chrome's netns, so localhost works
    return create_connection(ws_url, timeout=10)

def cmd(ws, method, params=None, mid=1):
    ws.send(json.dumps({"id": mid, "method": method, "params": params or {}}))
    while True:
        r = json.loads(ws.recv())
        if r.get("id") == mid:
            return r

def wait_event(ws, method, timeout=15):
    ws.settimeout(timeout)
    deadline = time.time() + timeout
    while time.time() < deadline:
        try:
            r = json.loads(ws.recv())
            if r.get("method") == method:
                return r
        except:
            break
    return None

def navigate(ws, url):
    cmd(ws, "Page.enable")
    result = cmd(ws, "Page.navigate", {"url": url})
    if result.get("result", {}).get("errorText"):
        print(f"  Navigate error: {result['result']['errorText']}")
        # Try anyway
    wait_event(ws, "Page.loadEventFired", timeout=15)
    time.sleep(1.5)

def js(ws, expr):
    r = cmd(ws, "Runtime.evaluate", {"expression": expr, "returnByValue": True}, mid=200)
    return r.get("result", {}).get("result", {}).get("value")

def html(ws):
    return js(ws, "document.documentElement.outerHTML") or ""

def snap(ws, name):
    r = cmd(ws, "Page.captureScreenshot", {"format": "png"}, mid=100)
    if r.get("result", {}).get("data"):
        path = os.path.join(SCREENSHOT_DIR, name)
        with open(path, "wb") as f:
            f.write(base64.b64decode(r["result"]["data"]))
        print(f"  [SNAP] {name}")
        return path
    return None

def curl(endpoint):
    """Test API using curl from Python"""
    url = f"{BASE}{endpoint}"
    try:
        req = urllib.request.Request(url)
        with urllib.request.urlopen(req, timeout=10) as r:
            raw = r.read()
            try:
                data = json.loads(raw.decode())
                return r.status, data
            except:
                return r.status, {"_raw": raw.decode()[:200]}
    except urllib.error.HTTPError as e:
        raw = e.read()
        try:
            data = json.loads(raw.decode())
            return e.code, data
        except:
            return e.code, {"error": str(e), "_raw": raw.decode()[:200]}
    except Exception as e:
        return None, {"error": str(e)}

def run():
    global BASE
    BASE = detect_web()
    if not BASE:
        return
    
    os.makedirs(SCREENSHOT_DIR, exist_ok=True)
    
    print("=" * 60)
    print("KeyIP Intelligence — E2E Test Suite")
    print(f"Web: {BASE}")
    print("=" * 60)
    
    passed = 0
    failed = 0
    
    def test(label, condition, detail=""):
        nonlocal passed, failed
        if condition:
            passed += 1
            print(f"  ✓ {label}")
        else:
            failed += 1
            print(f"  ✗ {label}  |  {detail[:120]}")
    
    # ---- PHASE 1: API Tests ----
    print(f"\n{'='*60}")
    print("PHASE 1: API Endpoint Tests")
    print("=" * 60)
    
    apis = [
        ("Health Liveness", "/api/v1/healthz", 200),
        ("Health Readiness", "/api/v1/readyz", 200),
        ("Dashboard Metrics", "/api/v1/dashboard/metrics", 200),
        ("Alerts", "/api/v1/alerts", 200),
        ("Patent Search", "/api/v1/patents/search", 200),
        ("Lifecycle Events", "/api/v1/lifecycle/events", 200),
        ("Lifecycle Deadlines", "/api/v1/lifecycle/deadlines", 200),
        ("FTO Search API", "/api/v1/fto/search", 200),
        ("Infringement Watch", "/api/v1/infringement/watch", 200),
        ("Infringement Alerts", "/api/v1/infringement/alerts", 200),
        ("Portfolio Summary", "/api/v1/portfolios/summary", 200),
        ("Portfolio Scores", "/api/v1/portfolios/scores", 200),
        ("Portfolio Coverage", "/api/v1/portfolios/coverage", 200),
        ("Knowledge Graph API", "/api/v1/knowledge-graph", 200),
        ("Partners API", "/api/v1/partners", 200),
        ("Settings API", "/api/v1/settings", 200),
        ("Auth Signin", "/api/v1/auth/signin", 200),
        ("Auth Me", "/api/v1/auth/me", 200),
        ("Portfolio Constellation", "/api/v1/portfolios/pf-001/constellation", 200),
        ("Patent Detail CN115650927B", "/api/v1/patents/CN115650927B", 200),
        ("Patent Detail US11678901B2", "/api/v1/patents/US11678901B2", 200),
    ]
    
    for label, ep, expected_status in apis:
        status, data = curl(ep)
        ok = status == expected_status and isinstance(data, dict) and data.get("code") == 0
        detail = f"status={status} code={data.get('code')} hasData={'data' in data}"
        test(f"API {label}", ok, detail)
        if not ok:
            print(f"    Response: {json.dumps(data)[:200]}")
    
    # ---- PHASE 2: Invalid/Edge Case Tests ----
    print(f"\n{'='*60}")
    print("PHASE 2: Invalid Input / Edge Case Tests")
    print("=" * 60)
    
    edge_cases = [
        ("Non-existent page (SPA fallback)", "/nonexistent-xyz", 200, True),
        ("Invalid patent (server error)", "/api/v1/patents/INVALID-99999ZZ", 500, False),
        ("Empty search", "/api/v1/patents/search?q=", 200, True),
        ("Health detail", "/api/v1/healthz/detail", 200, True),
        ("Auth me always returns", "/api/v1/auth/me", 200, True),
        ("Bare healthz (SPA fallback)", "/healthz", 200, True),
    ]
    
    for label, ep, expected_status, expect_success in edge_cases:
        status, data = curl(ep)
        ok = status == expected_status
        detail = f"status={status}"
        test(f"Edge: {label}", ok, detail)
        if not ok:
            print(f"    Response: {json.dumps(data)[:150]}")
    
    # ---- PHASE 3: Chrome CDP Page Tests ----
    print(f"\n{'='*60}")
    print("PHASE 3: Browser Page Tests (Chrome CDP)")
    print("=" * 60)
    
    tab = new_tab("about:blank")
    if not tab:
        print("  SKIP: Cannot open Chrome tab (CDP not available)")
        test("CDP Tab Open", False, "Cannot open tab - is --network container:keyip-chrome set?")
    else:
        ws_url = tab.get("webSocketDebuggerUrl")
        ws = ws_connect(ws_url)
        cmd(ws, "Page.enable")
        cmd(ws, "Runtime.enable")
        cmd(ws, "Network.enable")
        cmd(ws, "Emulation.setDeviceMetricsOverride", {
            "width": 1440, "height": 900, "deviceScaleFactor": 1, "mobile": False
        })
        
        pages = [
            ("Sign In", "/"),
            ("Dashboard", "/dashboard"),
            ("Patents", "/patents"),
            ("Molecules", "/molecules"),
            ("Portfolios", "/portfolios"),
            ("Lifecycle", "/lifecycle"),
            ("FTO Search", "/fto"),
            ("Infringement", "/infringement"),
            ("Knowledge Graph", "/knowledge-graph"),
            ("Workspaces", "/workspaces"),
            ("Reports", "/reports"),
            ("Partners", "/partners"),
            ("Settings", "/settings"),
        ]
        
        for label, path in pages:
            try:
                navigate(ws, f"{BASE}{path}")
                h = html(ws)
                snap(ws, f"{label.lower().replace(' ','-')}.png")
                has_content = len(h) > 200
                test(f"Page {label}", has_content, f"HTML len={len(h)}")
                if not has_content:
                    print(f"    HTML preview: {h[:200]}")
            except Exception as e:
                test(f"Page {label}", False, str(e))
        
        ws.close()
    
    # ---- SUMMARY ----
    total = passed + failed
    print(f"\n{'='*60}")
    print(f"RESULTS: {passed}/{total} passed ({failed} failed)")
    if failed > 0:
        print("ACTION REQUIRED: Review failures above")
    else:
        print("ALL TESTS PASSED ✓")
    print("=" * 60)
    
    # List screenshots
    if os.path.exists(SCREENSHOT_DIR):
        files = sorted(os.listdir(SCREENSHOT_DIR))
        if files:
            print(f"\nScreenshots saved to {SCREENSHOT_DIR}:")
            for f in files:
                path = os.path.join(SCREENSHOT_DIR, f)
                size = os.path.getsize(path)
                print(f"  {f} ({size:,} bytes)")

if __name__ == "__main__":
    run()
