-- Phase 1: core schema

CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    email TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS clusters (
    id SERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    context_name TEXT NOT NULL, -- kubeconfig context, e.g. "kind-cloudops"
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS nodes (
    id SERIAL PRIMARY KEY,
    cluster_id INTEGER NOT NULL REFERENCES clusters(id),
    name TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'unknown'
);

CREATE TABLE IF NOT EXISTS deployments (
    id SERIAL PRIMARY KEY,
    cluster_id INTEGER NOT NULL REFERENCES clusters(id),
    name TEXT NOT NULL,
    namespace TEXT NOT NULL DEFAULT 'default',
    replicas INTEGER NOT NULL DEFAULT 1,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS metrics (
    id BIGSERIAL PRIMARY KEY,
    cluster_id INTEGER NOT NULL REFERENCES clusters(id),
    pod_name TEXT NOT NULL,
    namespace TEXT NOT NULL,
    cpu_millicores INTEGER,
    memory_bytes BIGINT,
    restart_count INTEGER DEFAULT 0,
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_metrics_pod_time ON metrics (pod_name, recorded_at DESC);

CREATE TABLE IF NOT EXISTS alerts (
    id SERIAL PRIMARY KEY,
    cluster_id INTEGER NOT NULL REFERENCES clusters(id),
    pod_name TEXT NOT NULL,
    alert_type TEXT NOT NULL, -- e.g. 'memory_threshold', 'restart_spike'
    severity TEXT NOT NULL DEFAULT 'warning',
    message TEXT,
    ai_diagnosis TEXT, -- filled in by Phase 5 AI service, read-only suggestion
    resolved BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
