# AI-Powered Cloud Operations Dashboard

Full-stack platform for monitoring, debugging, and managing AI/cloud workloads at scale.

## Phase 1 status: foundations
- [x] Repo structure
- [x] Docker Compose (Postgres, Redis, backend-api)
- [x] `backend-api` Go service with `/health`
- [x] Initial Postgres schema
- [ ] `kind` cluster running locally
- [ ] CI passing on GitHub

## Prerequisites (Mac, Apple Silicon)

```bash
brew install go
brew install --cask docker        # Docker Desktop, must be running
brew install kind
brew install kubectl
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
```

## Local dev: create the monitored kind cluster

```bash
cd infra
kind create cluster --config kind-config.yaml
kubectl cluster-info --context kind-cloudops
```

This is the target cluster the collector service will scrape metrics from,
starting in Phase 2.

## Repo layout

```
backend-api/    Go service — auth, REST API, metrics ingestion (merged for now)
frontend/       React + TypeScript dashboard (Phase 4)
infra/          docker-compose.yml, kind-config.yaml
db/migrations/  SQL schema migrations
```
