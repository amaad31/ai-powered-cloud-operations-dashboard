# AI-Powered Cloud Operations Dashboard

Full-stack platform for monitoring, debugging, and managing AI/cloud workloads at scale.

## Status

### Phase 1: foundations - done
- [x] Repo structure
- [x] Docker Compose (Postgres, Redis, backend-api)
- [x] `backend-api` Go service with `/health`
- [x] Initial Postgres schema
- [x] `kind` cluster running locally
- [x] CI passing on GitHub

### Phase 2: metrics ingestion - done
- [x] Collector connects to the `kind` cluster and polls pod status every 15s
- [x] Pod name, namespace, restart count written to `metrics` table
- [x] `metrics-server` installed, CPU/memory usage collected per pod
- [x] `GET /metrics` endpoint exposes collected data as JSON
- [x] `GET /alerts` endpoint exposes triggered alerts as JSON
- [x] Threshold-based alerting (restart spikes, memory usage), with auto-resolve
      when a condition clears. One open alert per (pod, alert type) at a time.
- [x] Unit tests for alert decision logic and HTTP handlers (`go test ./...`)

### Phase 3: backend API + auth - not started
- [ ] JWT-based authentication (register/login)
- [ ] Password hashing (bcrypt)
- [ ] Role-based access control (viewer vs. admin)
- [ ] Deployment control endpoints (restart/scale via K8s API)

### Later phases
- [ ] Phase 4: React + TypeScript frontend
- [ ] Phase 5: AI root-cause suggestions (OpenAI API) on triggered alerts
- [ ] Phase 6: deploy to AWS (EKS), secrets management, audit logging, TLS hardening

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

This is the local Kubernetes cluster the collector service scrapes, separate
from the app stack below. It's not started by docker-compose.

**Note:** `kind` assigns a random local port to the cluster's API server on
each creation (visible via `docker ps`, e.g. `127.0.0.1:53774->6443/tcp`).
If the cluster is recreated, or if Docker Desktop is restarted and the context
is lost, re-run:
```bash
kind export kubeconfig --name cloudops
```

### Install metrics-server (for CPU/memory data)

`kind` doesn't include this by default:

```bash
kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml
kubectl patch deployment metrics-server -n kube-system --type='json' \
  -p='[{"op":"add","path":"/spec/template/spec/containers/0/args/-","value":"--kubelet-insecure-tls"}]'
```

The `--kubelet-insecure-tls` flag is needed because `kind` nodes don't have
kubelet certs `metrics-server` would otherwise trust. Fine for local dev,
never do this in production. Verify with:
```bash
kubectl top pods
```

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
   (e.g. `127.0.0.1:53774->6443/tcp` -> use `53774`).
2. Replace the `certificate-authority-data: ...` line with `insecure-skip-tls-verify: true`
   (safe for local dev only, the cluster's cert isn't issued for `host.docker.internal`).

This file contains private key material. It's already excluded via `.gitignore`
and must never be committed. **Note:** the port above changes if you ever
recreate the cluster, so this file needs regenerating whenever that happens.

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
# [{"pod_name":"nginx-test-...","namespace":"default","cpu_millicores":0,"memory_bytes":18874368,"restart_count":0,"recorded_at":"..."}, ...]

curl http://localhost:8080/alerts
# [{"pod_name":"...","alert_type":"memory_threshold","severity":"warning","message":"...","resolved":false,"created_at":"..."}, ...]
```

Collector logs (`docker compose up`, no `-d`) should show:
```
collector started, polling every 15s
collector: recorded metrics for N pods
ALERT opened: memory_threshold for pod ... - Memory usage 262.4 MB exceeds threshold

## Running tests

```bash
cd backend-api
go test ./... -v
```

Covers: alert threshold decision logic (pure functions, no DB needed) and
HTTP handlers (`/health`, `/metrics`) using `sqlmock` to simulate Postgres
without a real database. Kubernetes API interaction (`collector.go`'s
`collectOnce`) is not unit tested. It would require a real or heavily mocked
cluster; that's earmarked as an integration-test concern for Phase 6.

CI runs the same test suite automatically on every push (see
`.github/workflows/ci.yml`).

## Repo layout

```
backend-api/
main.go HTTP server, /health, /metrics, /alerts
collector.go background goroutine polling the kind cluster every 15s,
threshold-based alerting
main_test.go handler tests (sqlmock)
collector_test.go alert decision logic tests
frontend/ React + TypeScript dashboard (Phase 4)
infra/
docker-compose.yml app stack: postgres, redis, backend-api
kind-config.yaml defines the monitored kind cluster (not used by compose)
kubeconfig-docker.yaml generated, lets backend-api container reach the cluster (gitignored)
db/migrations/ SQL schema migrations
```