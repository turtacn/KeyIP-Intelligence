# KeyIP-Intelligence Data Import Pipeline

## Data Source Overview

The KeyIP-Intelligence platform uses **8 data sources** across 3 tiers:

```
┌──────────────────────────────────────────────────────────────┐
│                    Frontend (React SPA)                       │
│                 http://localhost:80 (nginx)                   │
└──────────────────────────┬───────────────────────────────────┘
                           │ /api/v1/*
┌──────────────────────────▼───────────────────────────────────┐
│                API Server (Go)                               │
│             http://localhost:8080                             │
│              gRPC: localhost:9090                             │
└──┬───────┬────────┬────────┬────────┬────────┬────────┬──────┘
   │       │        │        │        │        │        │
   ▼       ▼        ▼        ▼        ▼        ▼        ▼
┌──────┐┌──────┐┌──────┐┌──────┐┌──────┐┌──────┐┌──────────┐
│PG 16 ││Neo4j ││Open- ││Milvus││Redis ││Kafka ││MinIO     │
│      ││5.x   ││Search││2.3   ││7.x   ││3.x   ││RELEASE   │
│:5432 ││:7687 ││:9200 ││:19530││:6379 ││:9092 ││:9000     │
└──────┘└──────┘└──────┘└──────┘└──────┘└──────┘└──────────┘
```

## Data Sources in Detail

### 1. PostgreSQL 16 (Primary Relational Database) — :5432
**Tables:** 24 tables across 7 migrations + 1 seed migration

| Domain       | Tables                                                            | Description                                |
|-------------|-------------------------------------------------------------------|--------------------------------------------|
| Patents     | `patents`, `patent_claims`, `patent_inventors`, `patent_priority_claims` | Patent bibliographic data, claims hierarchy, inventor records |
| Molecules   | `molecules`, `molecule_fingerprints`, `molecule_properties`, `patent_molecule_relations` | Chemical structures (SMILES/InChI), computed properties, patent-molecule links |
| Portfolios  | `portfolios`, `portfolio_patents`, `patent_valuations`, `portfolio_health_scores`, `portfolio_optimization_suggestions` | Portfolio management, patent valuation, health scoring, strategy recommendations |
| Lifecycle   | `patent_annuities`, `patent_deadlines`, `patent_lifecycle_events`, `patent_cost_records` | Patent renewal fees, deadlines, lifecycle tracking, cost management |
| Users       | `users`, `organizations`, `organization_members`, `roles`, `user_roles`, `api_keys` | Auth, RBAC, organizations, API access |
| Workspaces  | `workspaces`, `workspace_members`, `workspace_projects`, `project_patents`, `project_molecules` | Collaboration spaces, project management |
| Social      | `comments`, `notifications`, `saved_searches`                    | Collaboration features, alerts, saved queries |
| Audit       | `audit_logs`                                                     | Activity tracking |

**Seed Data Provided by:** `internal/infrastructure/database/postgres/migrations/008_seed_data.sql`
- 2 organizations, 6 users, 15 molecules, 14 patents, 3 portfolios
- Valuations, deadlines, lifecycle events, cost records, health scores
- Realistic OLED material domain data across CN/US/EP/JP/KR jurisdictions

### 2. Neo4j 5.x (Graph Database) — :7687
**Node Types:** Patent, Molecule, Inventor, Assignee, IPC_Class, Jurisdiction
**Relationship Types:** CITES, INVENTED_BY, ASSIGNED_TO, CONTAINS_MOLECULE, BELONGS_TO_FAMILY, CLASSIFIED_AS, FILED_IN
**Purpose:** Knowledge graph visualization, citation network analysis, inventor collaboration graphs, molecule-structure relationships

### 3. OpenSearch 2.x (Full-Text Search) — :9200
**Indexes:**
- `keyip-patents`: Full-text patent search (title, abstract, claims)
- `keyip-molecules`: Molecule name, SMILES, molecular formula search
- `keyip-lifecycle`: Deadline/event search
**Schema:** 768-dim dense vector embeddings for semantic search + BM25 text fields

### 4. Milvus 2.3 (Vector Database) — :19530
**Collections:**
- `molecule_vectors`: 512-dim Morgan fingerprint vectors for chemical similarity search
- `claim_vectors`: 768-dim scope embeddings for patent claim semantic search
**Purpose:** AI-powered molecular similarity search, claim scope comparison

### 5. Redis 7.x (Cache & Session Store) — :6379
**Keys/Patterns:**
- `session:*` — User session tokens (TTL: 24h)
- `ratelimit:*` — API rate limiting counters
- `cache:patent:*` — Hot patent data cache (TTL: 1h)
- `cache:molecule:*` — Hot molecule data cache (TTL: 1h)
- `lock:*` — Distributed locks for portfolio calculations
**Purpose:** Session management, rate limiting, hot-data caching, distributed coordination

### 6. Kafka 3.x (Message Queue) — :9092
**Topics:**
- `keyip.patent.events` — Patent lifecycle events (filing, grant, expiry)
- `keyip.molecule.events` — Molecule creation/update events
- `keyip.portfolio.events` — Portfolio changes, valuation updates
- `keyip.notification.events` — Email, WeChat Work, Slack notifications
- `keyip.search.index` — OpenSearch index update commands
- `keyip.vector.index` — Milvus vector embedding updates
**Purpose:** Event-driven architecture, async indexing, notifications

### 7. MinIO (Object Storage) — :9000
**Buckets:**
- `keyip-documents` — Patent PDFs, office action documents, certificates
- `keyip-reports` — Generated analysis reports, FTO reports, landscape analysis
- `keyip-molecule-images` — 2D/3D molecular structure renderings
**Purpose:** Document management, report storage, chemical structure images

### 8. DeepSeek API (External AI Service)
**Endpoint:** `https://api.deepseek.com/v1`
**Model:** `deepseek-chat`
**Purpose:** LLM-powered patent strategy analysis, claim interpretation, FTO risk assessment

---

## Import Scripts

| Script | Purpose | Prerequisites |
|--------|---------|---------------|
| `seed_all.sh` | **Master orchestrator** — seeds all data sources | Docker services running |
| `seed_pg.sh` | Run PostgreSQL migrations + seed data | PostgreSQL running |
| `seed_opensearch.sh` | Create OpenSearch indexes + index patent/molecule data | OpenSearch running, PostgreSQL seeded |
| `seed_milvus.sh` | Create Milvus collections + insert vector data | Milvus running, PostgreSQL seeded |
| `seed_neo4j.sh` | Load knowledge graph (citation network, molecule relationships) | Neo4j running, PostgreSQL seeded |
| `seed_minio.sh` | Upload sample patent documents to MinIO | MinIO running |
| `reset_db.sh` | Drop all data, re-run migrations, re-seed everything | ⚠️ DESTRUCTIVE |
| `verify_data.sh` | Run verification queries across all data sources | All data sources seeded |

## Quick Start

```bash
# 1. Start all infrastructure services
docker compose -f deployments/docker/docker-compose.yml up -d --wait

# 2. Run ALL imports (PG, OpenSearch, Milvus, Neo4j, MinIO)
./scripts/import/seed_all.sh

# 3. Verify data integrity
./scripts/import/verify_data.sh
```

## Development Workflow

```bash
# Fresh start: reset everything + re-seed
./scripts/import/reset_db.sh && ./scripts/import/seed_all.sh

# Just re-seed PostgreSQL (fastest)
./scripts/import/seed_pg.sh

# Seed a specific data source
./scripts/import/seed_opensearch.sh  # index patent full-text
./scripts/import/seed_milvus.sh      # index molecular fingerprints
./scripts/import/seed_neo4j.sh       # load knowledge graph
./scripts/import/seed_minio.sh       # upload sample documents
```

## Seed Data Summary

| Entity | Count | Highlights |
|--------|-------|------------|
| Organizations | 2 | OLED Material Tech (CN) + Luminara Materials (US) |
| Users | 6 | turta(admin), Zhang Wei(IP Manager), Li Jing(Researcher), Chen Yu(VP), Wang Fang(Patent Agent), Kim Min-Soo(Partner) |
| Roles | 5 | super_admin, org_admin, patent_analyst, researcher, viewer |
| Molecules | 15 | CBP, mCP, Ir(ppy)₃, DMAC-TRZ, 4CzIPN, Alq₃ + OLED host materials |
| Patents | 14 | CN/US/EP/JP/KR — granted/examination/expired/revoked |
| Patent Claims | 6+ | Independent + dependent, Markush formulas |
| Patent Inventors | 24 | Chinese, English, Japanese, Korean names |
| Portfolios | 3 | Blue Emitter Core, HTL, Licensing Revenue |
| Valuations | 6 | S/A/B/D tiers with monetary value ranges |
| Deadlines | 9 | critical/high/medium priority |
| Lifecycle Events | 11 | filing → grant → revocation full lifecycle |
| Annuities | 17 | CNY/USD/EUR/JPY currencies |
| Cost Records | 14 | Filing, prosecution, annuity, translation costs |
