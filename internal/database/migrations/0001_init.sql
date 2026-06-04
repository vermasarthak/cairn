CREATE TABLE IF NOT EXISTS schema_migrations (version TEXT PRIMARY KEY, applied_at TIMESTAMPTZ NOT NULL DEFAULT now());
CREATE TABLE IF NOT EXISTS policies (tenant_id TEXT NOT NULL, id TEXT NOT NULL, version INTEGER NOT NULL CHECK (version > 0), definition JSONB NOT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT now(), PRIMARY KEY (tenant_id, id, version));
CREATE TABLE IF NOT EXISTS reservations (
  tenant_id TEXT NOT NULL, subject_id TEXT NOT NULL, policy_id TEXT NOT NULL, action_key TEXT NOT NULL, local_day DATE NOT NULL,
  policy_version INTEGER NOT NULL CHECK (policy_version > 0), state TEXT NOT NULL CHECK (state IN ('reserved','queued','claimed','delivered','suppressed','failed')),
  created_at TIMESTAMPTZ NOT NULL, updated_at TIMESTAMPTZ NOT NULL,
  PRIMARY KEY (tenant_id, subject_id, policy_id, action_key, local_day)
);
CREATE TABLE IF NOT EXISTS jobs (id UUID PRIMARY KEY, tenant_id TEXT NOT NULL, state TEXT NOT NULL CHECK (state IN ('queued','claimed','succeeded','failed','suppressed')), lease_until TIMESTAMPTZ, available_at TIMESTAMPTZ NOT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT now(), updated_at TIMESTAMPTZ NOT NULL DEFAULT now());
CREATE TABLE IF NOT EXISTS attempts (id UUID PRIMARY KEY, job_id UUID NOT NULL REFERENCES jobs(id), idempotency_key TEXT NOT NULL, outcome TEXT NOT NULL CHECK (outcome IN ('accepted','rejected','unknown','retryable_error','permanent_error')), provider_receipt JSONB, created_at TIMESTAMPTZ NOT NULL DEFAULT now(), UNIQUE (job_id, idempotency_key));
CREATE TABLE IF NOT EXISTS outbox (id BIGSERIAL PRIMARY KEY, topic TEXT NOT NULL, payload JSONB NOT NULL, published_at TIMESTAMPTZ, created_at TIMESTAMPTZ NOT NULL DEFAULT now());
CREATE TABLE IF NOT EXISTS audit_events (id BIGSERIAL PRIMARY KEY, tenant_id TEXT NOT NULL, event_type TEXT NOT NULL, entity_type TEXT NOT NULL, entity_key JSONB NOT NULL, payload JSONB NOT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT now());
CREATE INDEX IF NOT EXISTS jobs_claim_idx ON jobs (available_at) WHERE state = 'queued';
CREATE INDEX IF NOT EXISTS outbox_unpublished_idx ON outbox (id) WHERE published_at IS NULL;
