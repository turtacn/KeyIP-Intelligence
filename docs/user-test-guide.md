# KeyIP Intelligence — Production Test Guide (v4.0)

**Last Updated:** 2026-05-18
**Mode:** Production — Real Go apiserver, no stubs
**Environment:** Docker Machine (Chrome CDP port 2222)

---

## Table of Contents

1. [Architecture](#1-architecture)
2. [Environment Setup](#2-environment-setup)
3. [Running E2E Tests](#3-running-e2e-tests)
4. [API Endpoint Status](#4-api-endpoint-status)
5. [SPA Page Routes](#5-spa-page-routes)
6. [Real OLED Patent Test Scenarios](#6-real-oled-patent-test-scenarios)
7. [Troubleshooting](#7-troubleshooting)

---

## 1. Architecture

```
Browser (http://192.168.99.100)
         │
         ▼
    ┌─────────┐      /api/*       ┌──────────────┐      ┌───────────┐
    │  nginx  │ ─── proxy_pass ── │  apiserver   │ ───  │ PostgreSQL │
    │  :80    │                   │  :8080 (Go)  │      │  :5432     │
    └─────────┘                   └──────────────┘      └───────────┘
         │                              │
    /assets/*                      Redis :6379
    /index.html                    Neo4j :7687 (optional)
    (SPA fallback)
```

**No stubs. No hardcoded JSON.** All `/api/*` requests go to the Go apiserver which queries the real PostgreSQL database.

---

## 2. Environment Setup

### Prerequisites

- Docker Engine with Docker Machine
- Running infrastructure containers:
  - `keyip-web` (nginx, port 80)
  - `keyip-apiserver` (Go, port 8080)
  - `keyip-postgres` (PostgreSQL 16, port 5432)
  - `keyip-chrome` (headless Chrome, port **2222**) — for browser-based E2E tests
- Real data imported via `scripts/seed.sh` or `scripts/import/`

### Starting Chrome with CDP on Port 2222

```bash
./harness/launch-chrome-cdp.sh
```

### Access Points

| Service | URL |
|---------|-----|
| Web UI | `http://192.168.99.100/dashboard` |
| Chrome CDP | `http://192.168.99.100:2222/json` |
| Apiserver health | `http://192.168.99.100:8080/api/v1/healthz` |

---

## 3. Running E2E Tests

### Production Comprehensive Test (Recommended)

```bash
docker run --rm --network container:keyip-chrome \
  -v $(pwd)/e2e_comprehensive.py:/test/e2e_comprehensive.py:ro \
  python:3.11-alpine sh -c "
    pip install websocket-client -q
    python3 /test/e2e_comprehensive.py
  "
```

**What it validates:**
- Phase 1: Health endpoints (healthz, readyz — real Go handlers)
- Phase 2: Patent CRUD (GET/POST /api/v1/patents — real DB data)
- Phase 3: Molecule CRUD (GET/POST /api/v1/molecules — real DB data)
- Phase 4: Portfolio CRUD (GET/POST /api/v1/portfolios)
- Phase 5: Auth (POST signin → 401 with bad credentials, GET me → 401 without token)
- Phase 6: Workspaces (GET/POST /api/v1/workspaces)
- Phase 7: Reports (POST /api/v1/reports/fto, .../infringement)
- Phase 8: Missing endpoints (expect 404 — no stubs to mask gaps)
- Phase 9: SPA page routing (13 browser pages via Chrome CDP)

### Quick Smoke Test

```bash
docker run --rm --network container:keyip-chrome \
  -v $(pwd)/e2e_test.py:/test/e2e_test.py:ro \
  python:3.11-alpine sh -c "
    pip install websocket-client -q && python3 /test/e2e_test.py
  "
```

---

## 4. API Endpoint Status

### Operational (Go handlers, real DB data)

| Method | Endpoint | Handler | Description |
|--------|----------|---------|-------------|
| GET | `/api/v1/healthz` | HealthHandler | Liveness probe |
| GET | `/api/v1/readyz` | HealthHandler | Readiness probe |
| GET | `/api/v1/patents` | PatentHandler | List patents (DB) |
| POST | `/api/v1/patents` | PatentHandler | Create patent |
| GET | `/api/v1/molecules` | MoleculeHandler | List molecules (DB) |
| POST | `/api/v1/molecules` | MoleculeHandler | Create molecule |
| GET | `/api/v1/portfolios` | PortfolioHandler | List portfolios (DB) |
| POST | `/api/v1/portfolios` | PortfolioHandler | Create portfolio |
| GET | `/api/v1/workspaces` | CollaborationHandler | List workspaces |
| POST | `/api/v1/workspaces` | CollaborationHandler | Create workspace |
| POST | `/api/v1/auth/signin` | AuthHandler | Sign in (requires email+password) |
| GET | `/api/v1/auth/me` | AuthHandler | Current user (requires Bearer token) |
| POST | `/api/v1/reports/fto` | ReportHandler | Generate FTO report |
| POST | `/api/v1/reports/infringement` | ReportHandler | Generate infringement report |
| POST | `/api/v1/ai/analyze-patent` | AIHandler | AI patent analysis |
| POST | `/api/v1/ai/chat` | AIHandler | AI chat |
| GET | `/api/v1/patents/{id}/lifecycle` | LifecycleHandler | Patent lifecycle |
| GET | `/api/version` | VersionHandler | API version |
| GET | `/api/v1/runtime/info` | RuntimeHandler | Runtime info |
| GET | `/api/v1/ws/events` | WSHandler | WebSocket events |

### Pending Implementation (returns 404 — not masked by stubs)

| Endpoint | Frontend uses for | Priority |
|----------|-------------------|----------|
| `/api/v1/dashboard/metrics` | Executive dashboard KPIs | HIGH |
| `/api/v1/alerts` | Alert notifications | HIGH |
| `/api/v1/patents/search` | Patent search with filters | HIGH |
| `/api/v1/patents/{number}` | Patent detail by number | HIGH |
| `/api/v1/lifecycle/events` | Lifecycle console | MEDIUM |
| `/api/v1/lifecycle/deadlines` | Deadline tracking | MEDIUM |
| `/api/v1/fto/search` | FTO analysis | MEDIUM |
| `/api/v1/infringement/alerts` | Infringement monitoring | MEDIUM |
| `/api/v1/infringement/watch` | Infringement watch list | MEDIUM |
| `/api/v1/portfolios/summary` | Portfolio overview | HIGH |
| `/api/v1/portfolios/scores` | Portfolio scoring | MEDIUM |
| `/api/v1/portfolios/coverage` | Jurisdiction coverage | MEDIUM |
| `/api/v1/knowledge-graph` | Citation network graph | LOW |
| `/api/v1/partners` | Partner management | LOW |
| `/api/v1/settings` | User preferences | LOW |
| `/api/v1/molecules/search` | Molecule search | MEDIUM |
| `/api/v1/molecules/{id}` | Molecule detail | MEDIUM |

---

## 5. SPA Page Routes

| Route | Component | APIs Called | Status |
|-------|-----------|-------------|--------|
| `/dashboard` | ExecutiveDashboard | `dashboard/metrics` → **404 (pending)** | Shows error state |
| `/patent-mining` | PatentMining | `patents/search`, `molecules` | Partial — search returns 404 |
| `/infringement-watch` | InfringementWatch | `infringement/alerts` → **404** | Shows error state |
| `/portfolio` | PortfolioOptimizer | `portfolios/summary`, `.../scores` → **404** | Shows error state |
| `/lifecycle` | LifecycleConsole | `lifecycle/events` → **404** | Shows error state |
| `/partners` | PartnerPortal | `partners` → **404** | Shows error state |
| `/knowledge-graph` | KnowledgeGraph | `knowledge-graph` → **404** | Uses mock fallback |
| `/search` | Search | `patents/search`, `molecules/search` → **404** | Shows error state |
| `/health` | Health | `healthz/detail` → **404** | Shows no-data state |
| `/molecules` | Molecules | `molecules` → ✓ | **WORKS** |
| `/fto` | FTOSearch | `fto/search` → **404** | Shows intro state |
| `/workspaces` | Workspaces | `workspaces` → ✓ | **WORKS** (mock data) |
| `/reports` | Reports | — (mock) | Shows static content |
| `/settings` | Settings | `settings` → **404** | Uses defaults |
| `/patents/:id` | PatentDetail | `patents/{id}` → **404** | Shows error state |
| `/molecules/:id` | MoleculeDetail | `molecules/{id}` → **404** | Shows error state |

---

## 6. Real OLED Patent Test Scenarios

### 6.1 Verify Real Database Data

```bash
# List patents from the real database
curl http://192.168.99.100/api/v1/patents

# List molecules
curl http://192.168.99.100/api/v1/molecules

# Create a test patent
curl -X POST http://192.168.99.100/api/v1/patents \
  -H "Content-Type: application/json" \
  -d '{
    "title": "Novel Blue TADF Emitter",
    "abstract": "A thermally activated delayed fluorescence material with ΔEST < 0.10 eV",
    "application_no": "CN20261000001",
    "applicant": "KeyIP OLED Lab",
    "inventors": ["Zhang Wei", "Li Ming"],
    "ipc_codes": ["H10K 50/00", "C09K 11/06"],
    "filing_date": "2026-05-15",
    "jurisdiction": "CN"
  }'
```

### 6.2 Test with Real Web Data

Import real OLED patent data from public sources (CNIPA, USPTO, EPO):
```bash
# Import real data (if import scripts exist)
./scripts/import/import_patents.sh
./scripts/seed.sh
```

### 6.3 Validate API Contract

```bash
# Health check
curl -s http://192.168.99.100/api/v1/healthz | python3 -m json.tool

# List real patents
curl -s http://192.168.99.100/api/v1/patents | python3 -m json.tool

# List real molecules
curl -s http://192.168.99.100/api/v1/molecules | python3 -m json.tool
```

---

## 7. Troubleshooting

### Endpoint returns 404

**Expected for pending endpoints** (see section 4). Not a bug — the Go handler hasn't been implemented yet.

### Endpoint returns HTML instead of JSON

The SPA fallback (`try_files $uri /index.html`) catches unmatched paths. API paths should have `/api/` prefix. If you see HTML for an `/api/` path, check nginx.conf is proxying correctly.

### Chrome CDP Connection Refused

```bash
# Check Chrome is running
./harness/launch-chrome-cdp.sh --status

# Restart if needed
./harness/launch-chrome-cdp.sh --restart
```

### Nginx Stubs Still Active?

Stubs have been **completely removed** from `web/nginx.conf`, `web/nginx-stubs.conf`, and `web/stubs.conf`. All `/api/` requests now proxy to the Go apiserver. If you still see stub data, the Docker image needs rebuilding:

```bash
docker build -t keyip-web:latest -f web/Dockerfile web/
docker restart keyip-web
```

---

*KeyIP Intelligence — Production Test Guide v4.0*
*No stubs. Real backend. Industrial delivery.*
