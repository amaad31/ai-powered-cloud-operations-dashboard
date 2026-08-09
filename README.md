# AI-Powered Cloud Operations Dashboard

Full-stack platform for monitoring, debugging, and managing AI/cloud workloads at scale.

## Status

### Phase 1: foundations
- [x] Repo structure
- [x] Docker Compose (Postgres, Redis, backend-api)
- [x] `backend-api` Go service with `/health`
- [x] Initial Postgres schema
- [x] `kind` cluster running locally
- [x] CI passing on GitHub

### Phase 2: metrics ingestion (in progress)
- [x] Collector connects to the `kind` cluster and polls pod status every 15s
- [x] Pod name, namespace, restart count written to `metrics` table
- [x] `GET /metrics` endpoint exposes collected data as JSON
- [ ] CPU/memory metrics (requires `metrics-server`)
- [ ] Threshold-based alerting

## Prerequisites (Mac, Apple Silicon)

```bash
brew install go
brew install --cask docker        # Docker Desktop, must be running
brew install kind
brew install kubectl
brew install gh                   # GitHub CLI, optional
```

## Local dev: create the monitored kind cluster

```bash
cd infra
kind create cluster --config kind-config.yaml
kind export kubeconfig --name cloudops
```

This is the local Kubernetes cluster the collector service scrapes
from the app stack below. It's not started by docker-compose.

### One-time: generate the Docker-reachable kubeconfig

The backend runs inside its own container and can't use `~/.kube/config`
directly (it points at `127.0.0.1`, which inside a container means "itself,"
not the Mac host). Generate a container-friendly copy:

```bash
cd infra
kubectl config view --minify --flatten --context=kind-cloudops > kubeconfig-docker.yaml
```

Then open `kubeconfig-docker.yaml` and:
1. Replace the `server:` line's host/port with `host.docker.internal:<PORT>`,
   where `<PORT>` is the port shown in `docker ps` for `cloudops-control-plane`
   (e.g. `127.0.0.1:53774->6443/tcp` → use `53774`).
2. Replace the `certificate-authority-data: ...` line with `insecure-skip-tls-verify: true`
   (safe for local dev only, the cluster's cert isn't issued for `host.docker.internal`).

This file contains private key material, it's already excluded via `.gitignore`
and must never be committed. **Note:** the port above changes if you ever
recreate the cluster (`kind delete cluster` + `kind create cluster`), so this
file needs regenerating whenever that happens.

### Register the cluster in Postgres

Once the app stack (below) is running:

```bash
docker exec -it infra-postgres-1 psql -U postgres -d cloudops \
  -c "INSERT INTO clusters (name, context_name) VALUES ('local-dev', 'kind-cloudops');"
```

### Deploy a test workload

```bash
kubectl create deployment nginx-test --image=nginx --replicas=2
```

## Local dev: run the backend stack

```bash
cd infra
docker compose up --build
```

Then check:

```bash
curl http://localhost:8080/health
# {"status":"ok","db":"connected"}

curl http://localhost:8080/metrics
# [{"pod_name":"nginx-test-...","namespace":"default","restart_count":0,"recorded_at":"..."}, ...]
```

Collector logs (`docker compose up`, no `-d`) should show:
```
collector started, polling every 15s
collector: recorded metrics for N pods
```

## Repo layout

```
backend-api/    Go service: auth, REST API, metrics ingestion (merged for now)
  main.go         HTTP server, /health, /metrics
  collector.go    background goroutine polling the kind cluster every 15s
frontend/       React + TypeScript dashboard (Phase 4)
infra/
  docker-compose.yml       app stack: postgres, redis, backend-api
  kind-config.yaml          defines the monitored kind cluster (not used by compose)
  kubeconfig-docker.yaml    generated, lets backend-api container reach the cluster (gitignored)
db/migrations/  SQL schema migrations
```