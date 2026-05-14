#!/usr/bin/env python3
"""Generate nginx stubs.conf with actual seed data from PostgreSQL."""

import subprocess, json

def pg(sql):
    r = subprocess.run(
        ["docker", "exec", "-i", "keyip-postgres", "psql", "-U", "keyip", "-d", "keyip_dev",
         "-t", "-A", "-F", "\t", "-c", sql],
        capture_output=True, text=True, timeout=15
    )
    if r.returncode != 0:
        err = r.stderr.strip()
        if 'does not exist' in err or 'no such table' in err.lower():
            return []  # Table doesn't exist - expected for some
        return []  # Other errors - silently ignore
    lines = [l.strip() for l in r.stdout.strip().split('\n') if l.strip()]
    if len(lines) == 1 and lines[0] == '':
        return []
    return [l.split('\t') for l in lines]

# --- Patents ---
patent_rows = pg("SELECT id, patent_number, title, abstract, status, filing_date::text, jurisdiction, ipc_codes, assignee_name FROM patents ORDER BY filing_date DESC")

patent_list = []
for r in patent_rows:
    patent_list.append({
        "id": r[0], "patentNumber": r[1], "title": r[2], "abstract": r[3] or "",
        "status": r[4], "filingDate": r[5] or "N/A", "jurisdiction": r[6],
        "ipcCodes": r[7] if r[7] else [], "assigneeName": r[8] or ""
    })

# --- Portfolios ---
portfolio_rows = pg("SELECT id, name, description FROM portfolios")
pf_ids = [r[0] for r in portfolio_rows]
pf_names = [r[1] for r in portfolio_rows]

# Portfolio summary  
total_patents = len(patent_rows)
granted = sum(1 for r in patent_rows if r[4] == 'granted')
pending = sum(1 for r in patent_rows if r[4] in ('filed','under_examination','pending'))
lapsed = sum(1 for r in patent_rows if r[4] in ('expired','abandoned','invalidated'))

jdx = {}
for r in patent_rows:
    j = r[6]; jdx[j] = jdx.get(j, 0) + 1

sdx = {}
for r in patent_rows:
    s = r[4]; sdx[s] = sdx.get(s, 0) + 1

# --- Health ---
health_svc = pg("SELECT COUNT(*) FROM patents")
hc = int(health_svc[0][0]) if health_svc else 0

# --- Alerts (from DB) ---
alert_rows = pg("SELECT id, title, message, severity, created_at::text FROM alerts LIMIT 5")
alert_list = []
for r in alert_rows:
    alert_list.append({"id": r[0], "title": r[1], "message": r[2] or "", "severity": r[3], "createdAt": r[4]})

# --- Lifecycle deadlines ---
deadline_rows = pg("SELECT id, patent_id, title, deadline_type, due_date::text, status FROM patent_deadlines LIMIT 10")
deadline_list = []
for r in deadline_rows:
    deadline_list.append({"id": r[0], "patentId": r[1], "title": r[2], "deadlineType": r[3], "dueDate": r[4], "status": r[5]})

# --- Lifecycle events ---
event_rows = pg("SELECT id, patent_id, event_type, description, event_date::text FROM patent_lifecycle_events LIMIT 10")
event_list = []
for r in event_rows:
    event_list.append({"id": r[0], "patentId": r[1], "eventType": r[2], "description": r[3], "eventDate": r[4]})

# --- Partners (organizations) ---
org_rows = pg("SELECT id, name, slug FROM organizations")
partner_list = []
for r in org_rows:
    partner_list.append({"id": r[0], "name": r[1], "slug": r[2] or ""})

# --- Molecules (from DB, served as nginx stubs since apiserver handler is WIP) ---
mol_rows = pg("SELECT id, name, smiles, molecular_formula, molecular_weight FROM molecules")
mol_list = []
for r in mol_rows:
    mw = float(r[4]) if r[4] else 0.0
    mol_list.append({"id": r[0], "name": r[1], "smiles": r[2], "molecularFormula": r[3] or "", "molecularWeight": mw})

# --- Patents short list for dashboard ---
patent_short = []
for p in patent_list[:10]:
    patent_short.append({"id": p["id"], "title": p["title"], "patentNumber": p["patentNumber"],
                          "status": p["status"], "filingDate": p["filingDate"], "jurisdiction": p["jurisdiction"]})

# --- Valuation data ---
val_rows = pg("""SELECT pv.patent_id, pv.monetary_value_mid, p.title 
    FROM patent_valuations pv JOIN patents p ON p.id = pv.patent_id""")
total_value = sum(float(r[1]) for r in val_rows if r[1]) if val_rows else 0

# --- Write stubs.conf ---
def jd(obj):
    return json.dumps(obj, ensure_ascii=False)

def wrap(data_key, obj, paginated=False):
    if paginated:
        return jd({"code": 0, "message": "ok", "data": obj, "pagination": {"page": 1, "pageSize": 20, "total": len(obj)}})
    return jd({"code": 0, "message": "ok", "data": obj})

out = []
def add(loc, body):
    out.append(f"    location {loc} {{\n        default_type application/json;\n        return 200 '{body}';\n    }}")

# Dashboard
dashboard = {
    "totalPatents": total_patents, "activePatents": granted + pending, "pendingPatents": pending,
    "highRiskAlerts": 2, "dueThisMonth": 3,
    "monthlyApplicationTrend": [
        {"month": "2024-07", "filed": 2, "granted": 1},
        {"month": "2024-10", "filed": 1, "granted": 0},
        {"month": "2025-01", "filed": 2, "granted": 2},
        {"month": "2025-04", "filed": 1, "granted": 1},
        {"month": "2025-07", "filed": 2, "granted": 0},
        {"month": "2025-10", "filed": 1, "granted": 2}
    ],
    "jurisdictionBreakdown": [{"jurisdiction": k, "count": v} for k, v in sorted(jdx.items())],
    "competitorComparison": [
        {"name": "KeyIP Portfolio", "portfolioSize": total_patents},
        {"name": "Samsung SDI", "portfolioSize": 12500},
        {"name": "LG Chem", "portfolioSize": 9800},
        {"name": "UDC", "portfolioSize": 4500}
    ],
    "portfolioHealthScore": 76, "upcomingDeadlines": [], "recentAlerts": []
}
add("= /api/v1/dashboard/metrics", wrap("data", dashboard))

# Alerts
add("= /api/v1/alerts", wrap("data", alert_list, paginated=True))

# Lifecycle
add("= /api/v1/lifecycle/deadlines", wrap("data", deadline_list, paginated=True))
add("= /api/v1/lifecycle/events", wrap("data", event_list, paginated=True))

# Patents
add("= /api/v1/patents", wrap("data", patent_short, paginated=True))
add("= /api/v1/patents/search", wrap("data", patent_short, paginated=True))

# Patent by ID — need regex. nginx regex $patent_id
for p in patent_list:
    loc = f"~ ^/api/v1/patents/{p['id']}$"
    add(loc, wrap("data", p))

# Portfolios
add("= /api/v1/portfolios", wrap("data", [{"id": r[0], "name": r[1], "description": r[2] or "", "totalPatents": total_patents} for r in portfolio_rows], paginated=True))

summary = {
    "id": pf_ids[0] if pf_ids else "default",
    "name": pf_names[0] if pf_names else "OLED Portfolio",
    "description": "OLED materials and device patents",
    "totalPatents": total_patents, "granted": granted, "pending": pending, "lapsed": lapsed,
    "totalValue": int(total_value),
    "healthGrade": "A-",
    "byJurisdiction": jdx, "byStatus": sdx,
    "topIPCCodes": list(set(r[7].strip('{}').split(',')[0] if r[7] else "" for r in patent_rows)),
    "recommendations": []
}
add("= /api/v1/portfolios/summary", wrap("data", summary))

# Constellation
constellation = {
    "portfolioId": pf_ids[0] if pf_ids else "default",
    "points": [{"x": 0.3 + i*0.05, "y": 0.4 + i*0.04, "r": 10 + i, "color": "green", "label": pl["patentNumber"]}
               for i, pl in enumerate(patent_list[:8])],
    "clusters": [{"name": "OLED Emitters", "centerX": 0.45, "centerY": 0.55}],
    "whiteSpaces": [{"x": 0.85, "y": 0.15, "r": 18, "label": "Perovskite LEDs"}],
    "totalPoints": len(patent_list)
}
add("~ ^/api/v1/portfolios/[^/]+/constellation$", wrap("data", constellation))

add("= /api/v1/portfolios/scores", jd({"code":0,"message":"ok","data":{"overallScore":76,"technicalScore":78,"legalScore":72,"commercialScore":74}}))
add("= /api/v1/portfolios/coverage", jd({"code":0,"message":"ok","data":jdx}))

# Partners
add("= /api/v1/partners", wrap("data", partner_list, paginated=True))

# Knowledge Graph
kg_nodes = []
kg_edges = []
for i, p in enumerate(patent_list[:6]):
    kg_nodes.append({"id": p["id"], "label": p["patentNumber"], "type": "patent", "title": p["title"]})
for i, m in enumerate(mol_list[:4]):
    kg_nodes.append({"id": m["id"], "label": m["name"], "type": "molecule"})
for i in range(min(3, len(patent_list))):
    if i < len(mol_list):
        kg_edges.append({"source": patent_list[i]["id"], "target": mol_list[i]["id"], "type": "contains"})
add("= /api/v1/knowledge-graph", jd({"code":0,"message":"ok","data":{"nodes": kg_nodes, "edges": kg_edges, "totalNodes": len(kg_nodes), "totalEdges": len(kg_edges)}}))

# FTO
add("= /api/v1/fto/search", wrap("data", patent_short, paginated=True))

# Infringement
add("= /api/v1/infringement/alerts", wrap("data", alert_list, paginated=True))
add("= /api/v1/infringement/watch", wrap("data", patent_short, paginated=True))

# Settings
add("= /api/v1/settings", jd({"code":0,"message":"ok","data":{"theme":"light","language":"en","notifications":True}}))

# Molecules (served as nginx stubs)
add("= /api/v1/molecules", wrap("data", mol_list, paginated=True))
add("= /api/v1/molecules/search", wrap("data", mol_list, paginated=True))
for m in mol_list:
    add(f"~ ^/api/v1/molecules/{m['id']}$", wrap("data", m))

# Portfolio optimizer
add("= /api/v1/portfolio/optimize", jd({"code":0,"message":"ok","data":{"recommendations":[],"score":total_patents*5}}))

# Health (served as nginx stubs since apiserver handler is WIP)
add("= /api/v1/healthz", jd({"code":0,"message":"ok","data":{"status":"healthy"}}))
add("= /api/v1/healthz/detail", jd({"code":0,"message":"ok","data":{"status":"healthy","uptime":86400,"timestamp":"2026-05-14T00:00:00Z","services":{"PostgreSQL":{"status":"healthy","responseTime":12,"version":"16"},"Redis":{"status":"healthy","responseTime":3,"version":"7.2"},"OpenSearch":{"status":"healthy","responseTime":25,"version":"2.11"},"Milvus":{"status":"healthy","responseTime":15,"version":"2.3"},"MinIO":{"status":"healthy","responseTime":10,"version":"2024"}}}}))

# Auth is NOT stubbed — it proxies to apiserver

stubs = "\n".join(out)
print(stubs)
