# Deployment Guide

> Version: 0.1.0-alpha | Last Updated: 2026-05

This guide covers deploying KeyIP-Intelligence in development, staging, and production environments.

---

## Table of Contents

1. [Architecture Overview](#architecture-overview)
2. [Docker Compose Deployment](#docker-compose-development-deployment)
3. [Kubernetes Deployment](#kubernetes-deployment)
4. [Configuration and Environment Variables](#configuration-and-environment-variables)
5. [Database Migration](#database-migration)
6. [SSL/TLS Setup](#ssltls-setup)
7. [Monitoring Setup](#monitoring-setup)
8. [Backup and Recovery](#backup-and-recovery)
9. [Troubleshooting](#troubleshooting)

---

## Architecture Overview

KeyIP-Intelligence consists of three main services plus supporting infrastructure:

```
┌──────────────┐    ┌──────────────┐    ┌──────────────┐
│  apiserver    │    │   worker     │    │   keyip CLI  │
│  (HTTP/gRPC)  │    │ (background) │    │  (separate)  │
└──────┬───────┘    └──────┬───────┘    └──────────────┘
       │                   │
       └───────────────────┘
               │
    ┌──────────┴──────────┐
    │   Data Services     │
    │  PG  Neo4j  OS  M  │
    │  Redis  Kafka  MinIO│
    └─────────────────────┘
```

- **apiserver**: HTTP (gin) and gRPC server handling API requests. Stateless, horizontally scalable.
- **worker**: Background worker consuming Kafka events for async tasks (patent monitoring, report generation).
- **keyip CLI**: Standalone command-line tool for local operations.

---

## Docker Compose (Development Deployment)

### Infrastructure Services

The file `deployments/docker/docker-compose.yml` provides all infrastructure services needed for local development.

```bash
# Start all infra services
docker compose -f deployments/docker/docker-compose.yml up -d

# Verify status
docker compose -f deployments/docker/docker-compose.yml ps

# View logs
docker compose -f deployments/docker/docker-compose.yml logs -f

# Stop all services
docker compose -f deployments/docker/docker-compose.yml down --remove-orphans
```

### Application Services

For development, run the application binaries directly:

```bash
# Build and run API server
make build && bin/apiserver

# In another terminal, run the worker
bin/worker
```

### Building Docker Images

```bash
# Build both images
make docker-build

# Build and push to registry
make docker-push
```

This produces two images:

- `ghcr.io/turtacn/keyip-apiserver:latest`
- `ghcr.io/turtacn/keyip-worker:latest`

Both use multi-stage builds (`deployments/docker/Dockerfile.apiserver`, `deployments/docker/Dockerfile.worker`):

1. **Stage 1 (builder)**: Compile the Go binary with version info injected via `-ldflags`.
2. **Stage 2 (runtime)**: Minimal `alpine:3.19` image with `ca-certificates` and `tzdata`.

```dockerfile
FROM golang:1.22-alpine AS builder
# ... compile with CGO_ENABLED=0 ...

FROM alpine:3.19
COPY --from=builder /usr/local/bin/apiserver /usr/local/bin/apiserver
EXPOSE 8080
ENTRYPOINT ["apiserver"]
```

### Composing Application + Infrastructure Together

For a fully self-contained setup, create a `docker-compose.prod.yml` that includes both the application images and the infrastructure services. This is useful for staging/demo environments.

```yaml
version: "3.9"
services:
  apiserver:
    image: ghcr.io/turtacn/keyip-apiserver:latest
    ports:
      - "8080:8080"
    environment:
      - CONFIG_FILE=/etc/keyip/config.yaml
    volumes:
      - ./configs/prod.yaml:/etc/keyip/config.yaml
    depends_on:
      postgres:
        condition: service_healthy
      neo4j:
        condition: service_healthy
      redis:
        condition: service_healthy

  worker:
    image: ghcr.io/turtacn/keyip-worker:latest
    volumes:
      - ./configs/prod.yaml:/etc/keyip/config.yaml
    depends_on:
      kafka:
        condition: service_healthy

  # ... infrastructure services from deployments/docker/docker-compose.yml
```

---

## Kubernetes Deployment

### Prerequisites

- Kubernetes 1.28+
- kubectl configured with cluster access
- Ingress controller (nginx-ingress or similar)
- cert-manager for TLS certificates
- StorageClass for persistent volumes

### Manifests

Kubernetes manifests are in `deployments/kubernetes/` using Kustomize:

```
deployments/kubernetes/
├── base/
│   ├── kustomization.yaml
│   ├── namespace.yaml
│   ├── apiserver-deployment.yaml
│   ├── worker-deployment.yaml
│   └── configmap.yaml
└── overlays/
    ├── dev/
    │   └── kustomization.yaml
    └── prod/
        └── kustomization.yaml
```

### Deploying to Dev

```bash
# Preview resources
kubectl kustomize deployments/kubernetes/overlays/dev

# Apply
kubectl apply -k deployments/kubernetes/overlays/dev

# Verify
kubectl -n keyip-intelligence get pods
```

### Deploying to Production

```bash
# Preview first
kubectl kustomize deployments/kubernetes/overlays/prod

# Apply with strict validation
kubectl apply --validate=strict -k deployments/kubernetes/overlays/prod
```

### API Server Deployment

The `apiserver-deployment.yaml` definition:

- **Replicas**: 3 (production), 1 (dev)
- **Service**: ClusterIP on port 8080
- **Probes**: HTTP readiness/liveness on `/api/v1/health`
- **Resources**: Request 500m CPU / 512Mi memory, limit 2 CPU / 2Gi memory
- **Graceful shutdown**: `terminationGracePeriodSeconds: 60`

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: apiserver
  namespace: keyip-intelligence
spec:
  replicas: 3
  selector:
    matchLabels:
      app: apiserver
  template:
    spec:
      containers:
        - name: apiserver
          image: ghcr.io/turtacn/keyip-apiserver:latest
          ports:
            - containerPort: 8080
          envFrom:
            - configMapRef:
                name: keyip-config
          livenessProbe:
            httpGet:
              path: /api/v1/health
              port: 8080
            initialDelaySeconds: 10
          readinessProbe:
            httpGet:
              path: /api/v1/health
              port: 8080
            initialDelaySeconds: 5
```

### Worker Deployment

The worker deployment is similar but does not expose network ports. It runs as a `Deployment` with a single replica (use a `CronJob` for scheduled tasks or `HorizontalPodAutoscaler` for event-driven scaling).

### Ingress

Expose the API server through an Ingress resource:

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: keyip-ingress
  namespace: keyip-intelligence
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-prod
    nginx.ingress.kubernetes.io/ssl-redirect: "true"
spec:
  ingressClassName: nginx
  tls:
    - hosts:
        - api.keyip-intelligence.com
      secretName: keyip-tls
  rules:
    - host: api.keyip-intelligence.com
      http:
        paths:
          - path: /api
            pathType: Prefix
            backend:
              service:
                name: apiserver
                port:
                  number: 8080
```

### Infrastructure on Kubernetes

For production, run infrastructure services as StatefulSets with persistent volumes:

| Service    | Recommended Approach                      |
| :--------- | :---------------------------------------- |
| PostgreSQL | Cloud provider managed service (RDS, Cloud SQL) or `cloudnative-pg` operator |
| Neo4j      | Neo4j Helm chart or AuraDB managed        |
| OpenSearch | OpenSearch Helm chart or managed service  |
| Milvus     | Milvus Helm chart (milvus-operator)       |
| Redis      | Redis Helm chart or ElastiCache           |
| Kafka      | Strimzi operator or Confluent for Kubernetes |
| MinIO      | MinIO Operator or S3-compatible cloud storage |
| Keycloak   | Keycloak Helm chart                       |

### Production Best Practices

- **Resource limits**: Set CPU/memory requests and limits on all containers.
- **PodDisruptionBudget**: Ensure API server has `minAvailable: 2`.
- **HorizontalPodAutoscaler**: Scale API server based on CPU utilization.
- **Network policies**: Restrict pod-to-pod traffic to necessary flows.
- **Secrets**: Use external secrets operator (e.g., External Secrets, Vault) rather than plain ConfigMaps.
- **Priority classes**: Assign higher priority to API server and worker pods.
- **Tolerations**: Taint nodes for ML-workload nodes if running model inference.

---

## Configuration and Environment Variables

### Configuration File

The primary configuration mechanism is `configs/config.yaml`. The application loads it via `github.com/spf13/viper`. Path resolution:

1. `$CONFIG_FILE` environment variable
2. `./configs/config.yaml`
3. `./config.yaml`

### Configuration Structure

Full reference: `configs/config.example.yaml`

Key sections:

```yaml
server:
  http:
    host: "0.0.0.0"
    port: 8080
  grpc:
    host: "0.0.0.0"
    port: 9090

database:
  postgres:
    host: "localhost"
    port: 5432
    user: "keyip_app"
    password: "ChangeMe!"
    dbname: "keyip_intelligence"
    sslmode: "require"           # disable | require | verify-ca | verify-full
  neo4j:
    uri: "bolt://localhost:7687"
    user: "neo4j"
    password: "ChangeMe!"

search:
  opensearch:
    addresses: ["http://localhost:9200"]
    username: "admin"
  milvus:
    address: "localhost"
    port: 19530

messaging:
  kafka:
    brokers: ["localhost:9092"]

storage:
  minio:
    endpoint: "localhost:9000"
    access_key: "minioadmin"
    secret_key: "minioadmin"
    bucket_name: "keyip-documents"

auth:
  keycloak:
    base_url: "http://localhost:8180"
    realm: "keyip"
  jwt:
    secret: "ChangeMe!"

intelligence:
  models_dir: "./models"
  molpatent_gnn:
    model_path: "molpatent_gnn.pt"
    device: "cpu"                # cpu | cuda
  strategy_gpt:
    endpoint: "https://api.openai.com/v1"
    model_name: "gpt-4"
```

### Environment Variable Override

All config keys can be overridden via environment variables using viper's automatic binding:

| Config Key                        | Environment Variable                       |
| :-------------------------------- | :----------------------------------------- |
| `database.postgres.host`          | `KEYIP_DATABASE_POSTGRES_HOST`             |
| `database.postgres.password`      | `KEYIP_DATABASE_POSTGRES_PASSWORD`         |
| `auth.jwt.secret`                 | `KEYIP_AUTH_JWT_SECRET`                    |
| `messaging.kafka.brokers`         | `KEYIP_MESSAGING_KAFKA_BROKERS`            |
| `intelligence.strategy_gpt.api_key` | `KEYIP_INTELLIGENCE_STRATEGY_GPT_API_KEY` |

All vars are prefixed with `KEYIP_` and use underscores as separators.

### Recommended Production Settings

```yaml
server:
  http:
    host: "0.0.0.0"
    port: 8080
    read_timeout: 30s
    write_timeout: 30s

database:
  postgres:
    max_open_conns: 25
    max_idle_conns: 10
    conn_max_lifetime: 5m
    sslmode: "require"

monitoring:
  logging:
    level: "info"
    format: "json"               # Structured JSON for log aggregation
```

---

## Database Migration

### Tool

Migrations use `golang-migrate/migrate` with the PostgreSQL driver.

### Migration Files

Located in `internal/infrastructure/database/postgres/migrations/`:

```
migrations/
├── 001_create_patents.sql
├── 002_create_molecules.sql
├── 003_create_portfolios.sql
├── 004_create_lifecycle.sql
├── 005_create_users.sql
└── 006_create_workspaces.sql
```

### Commands

```bash
# Install the migration tool
make install-tools

# Apply all pending migrations
make migrate-up

# Roll back the last migration
make migrate-down

# Roll back all migrations
make migrate-down-all

# Check current migration version
make migrate-status

# Create a new migration
make migrate-create NAME=add_entity_indexes
```

### Migration Script

The `scripts/migrate.sh` script handles migration execution:

```bash
# Using explicit DATABASE_URL
DATABASE_URL="postgres://user:password@host:5432/dbname?sslmode=require" \
  ./scripts/migrate.sh up

# Or relying on configs/config.yaml extraction
./scripts/migrate.sh up
```

### Migration Best Practices

- **Always backup** before running migrations in production.
- **Test migrations** against a staging copy of production data.
- **Down migrations**: Write reversible migrations for the first 30 days after deployment.
- **Zero-downtime**: For large tables, use `CREATE INDEX CONCURRENTLY` and batched ALTER TABLE.
- **Version pinning**: Use the migration version in deployment manifests to ensure the right migration runs with the right code version.

### Migration in CI/CD

In the release pipeline, run migrations as a separate job before deploying new code:

```bash
# Kubernetes job example
kubectl create job --from=cronjob/keyip-migrate keyip-migrate-$(VERSION)
```

---

## SSL/TLS Setup

### For API Endpoints

#### Option 1: Ingress Termination (Kubernetes)

Use cert-manager with Let's Encrypt to automatically provision TLS certificates. The ingress annotation `cert-manager.io/cluster-issuer: letsencrypt-prod` handles this.

#### Option 2: Reverse Proxy (Non-Kubernetes)

Place the API server behind nginx or Caddy with TLS termination:

```nginx
# /etc/nginx/sites-available/keyip
server {
    listen 443 ssl;
    server_name api.keyip-intelligence.com;

    ssl_certificate     /etc/letsencrypt/live/api.keyip-intelligence.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/api.keyip-intelligence.com/privkey.pem;
    ssl_protocols       TLSv1.2 TLSv1.3;
    ssl_ciphers         HIGH:!aNULL:!MD5;

    location /api/ {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

#### Option 3: Direct TLS (Go Server)

The API server can terminate TLS directly. Uncomment and configure in `config.yaml`:

```yaml
server:
  http:
    tls_enabled: true
    tls_cert_file: /etc/keyip/certs/tls.crt
    tls_key_file: /etc/keyip/certs/tls.key
```

### For Database Connections

- **PostgreSQL**: Set `sslmode: require` (or `verify-ca` / `verify-full` for mTLS).
- **Neo4j**: Use `neo4j+s://` or `neo4j+ssc://` URI scheme for TLS.
- **OpenSearch**: Enable TLS plugin and use `https://` addresses.
- **Kafka**: Configure SSL listener and set `KAFKA_LISTENER_SECURITY_PROTOCOL_MAP: SSL:SSL`.

### For Internal Services

Internal traffic (apiserver -> PostgreSQL, worker -> Kafka, etc.) within a Kubernetes cluster or Docker network can use the pod/container network without encryption since it is isolated.

---

## Monitoring Setup

### Metrics (Prometheus + Grafana)

The API server and worker expose Prometheus metrics at `/metrics` on port 9091.

```yaml
monitoring:
  prometheus:
    enabled: true
    port: 9091
    path: "/metrics"
    namespace: "keyip"
```

#### Available Metrics

| Metric                                   | Type    | Labels                          | Description                           |
| :--------------------------------------- | :------ | :------------------------------ | :------------------------------------ |
| `keyip_http_requests_total`              | Counter | method, path, status            | Total HTTP requests                   |
| `keyip_http_request_duration_seconds`    | Histogram | method, path                 | Request latency                       |
| `keyip_intelligence_inference_total`     | Counter | model, status                   | Model inference count                 |
| `keyip_intelligence_inference_duration`  | Histogram | model                        | Inference latency                     |
| `keyip_kafka_messages_total`             | Counter | topic, action                   | Kafka message count                   |
| `keyip_db_connections_open`              | Gauge   | database                        | Open database connections             |

#### Prometheus Configuration

```yaml
# prometheus.yml
scrape_configs:
  - job_name: "keyip-apiserver"
    static_configs:
      - targets: ["apiserver:9091"]

  - job_name: "keyip-worker"
    static_configs:
      - targets: ["worker:9091"]
```

#### Grafana Dashboard

Import the provided dashboard template (once created, it will be in `deployments/grafana/`). Key panels:

- Request rate and latency (p50/p95/p99)
- Error rate by endpoint
- Active DB connections
- Model inference throughput and latency
- Kafka consumer lag
- Go runtime metrics (goroutines, GC, memory)

### Logging

Logs are structured JSON via `go.uber.org/zap` (not standard `slog`):

```json
{
  "level": "info",
  "time": "2026-05-09T10:00:00Z",
  "logger": "http",
  "msg": "request completed",
  "method": "POST",
  "path": "/api/v1/molecules/similarity-search",
  "status": 200,
  "duration_ms": 342,
  "request_id": "req_mol_001"
}
```

Configuration:

```yaml
monitoring:
  logging:
    level: "info"              # debug | info | warn | error
    format: "json"             # json | text
    output: "stdout"           # stdout | file
    file_path: "/var/log/keyip.log"
    max_size: 100              # MB
    max_backups: 3
    max_age: 28                # days
    compress: true
```

For production, always use `format: json` and send logs to a centralized log aggregator (Loki, Elasticsearch, or Datadog).

### Distributed Tracing

OpenTelemetry tracing is available but disabled by default:

```yaml
monitoring:
  tracing:
    enabled: true
    endpoint: "http://localhost:4317"   # OpenTelemetry Collector
    sample_rate: 0.1
    service_name: "keyip-intelligence"
    environment: "production"
```

### Health Checks

The API server provides a health endpoint:

```bash
# Liveness check
GET /api/v1/health
# {"status":"ok","version":"0.1.0-alpha","uptime":"5h32m"}

# Readiness check (same endpoint)
# Returns 503 if dependencies are unavailable
```

Configure Kubernetes probes:

```yaml
livenessProbe:
  httpGet:
    path: /api/v1/health
    port: 8080
  initialDelaySeconds: 30
  periodSeconds: 10

readinessProbe:
  httpGet:
    path: /api/v1/health
    port: 8080
  initialDelaySeconds: 5
  periodSeconds: 5
```

### Alerting

Recommended Prometheus alerting rules:

```yaml
groups:
  - name: keyip
    rules:
      - alert: HighErrorRate
        expr: rate(keyip_http_requests_total{status=~"5.."}[5m]) > 0.05
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: "API error rate above 5%"

      - alert: HighInferenceLatency
        expr: histogram_quantile(0.99, rate(keyip_intelligence_inference_duration_bucket[5m])) > 30
        for: 5m
        labels:
          severity: warning

      - alert: KafkaConsumerLag
        expr: keyip_kafka_consumer_lag > 1000
        for: 10m
        labels:
          severity: warning
```

---

## Backup and Recovery

### PostgreSQL

```bash
# Backup
pg_dump -h localhost -U keyip -d keyip_intelligence > backup_$(date +%Y%m%d).sql

# Restore
psql -h localhost -U keyip -d keyip_intelligence < backup_20260509.sql
```

### Neo4j

```bash
# Backup (requires Enterprise Edition or dump)
neo4j-admin database dump neo4j --to-path=/backups/

# Restore
neo4j-admin database load neo4j --from-path=/backups/ --force
```

### MinIO

Use `mc` mirror for S3-compatible backup:

```bash
mc mirror --watch minio/keyip-documents backup-bucket/keyip-documents
```

### Milvus

Milvus does not have a built-in backup tool. Use `milvus-backup` (community tool) or re-index from the source molecule data.

---

## Troubleshooting

### Common Issues

| Issue                          | Likely Cause                         | Resolution                                    |
| :----------------------------- | :----------------------------------- | :-------------------------------------------- |
| API server fails to start      | Database unreachable                 | Check `docker compose ps` for PostgreSQL      |
| Migration fails                | Wrong DATABASE_URL or missing tool   | Run `make tools` to install migrate           |
| Kafka consumer not receiving   | Wrong broker address or group ID     | Verify `config.yaml` brokers                  |
| Milvus connection timeout      | Milvus not yet healthy               | Milvus takes 60+ seconds to start             |
| Auth returns 401               | Wrong Keycloak realm or client       | Verify Keycloak configuration in config.yaml  |
| OpenSearch index not found     | Migration not run, or index deleted  | Run `make seed` to recreate indices           |
| Worker idle, no messages       | No patents being published           | Normal in development; publish a test event   |
| Port conflicts                 | Docker ports clash with local services| Modify port mappings in docker-compose.yml     |

### Checking Service Health

```bash
# Docker Compose
docker compose -f deployments/docker/docker-compose.yml ps

# Application health
curl -s http://localhost:8080/api/v1/health | jq .

# Database
docker exec keyip-postgres pg_isready -U keyip

# Kafka
docker exec keyip-kafka kafka-topics --bootstrap-server localhost:9092 --list

# OpenSearch
curl -s http://localhost:9200/_cluster/health | jq .
```

### Logs

```bash
# All services
make docker-logs

# Specific service
docker compose -f deployments/docker/docker-compose.yml logs -f apiserver

# Application logs (if file logging configured)
tail -f /var/log/keyip.log
```
