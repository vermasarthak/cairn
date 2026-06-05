CREATE TABLE reconciliations (
  id UUID PRIMARY KEY,
  job_id UUID NOT NULL REFERENCES jobs(id),
  result TEXT NOT NULL CHECK (result IN ('confirmed','rejected','absent','unknown')),
  provider_receipt JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE jobs DROP CONSTRAINT jobs_state_check;
ALTER TABLE jobs ADD CONSTRAINT jobs_state_check CHECK (state IN ('queued','claimed','succeeded','failed','suppressed','reconcile','reconciling'));
CREATE INDEX jobs_reconcile_idx ON jobs (updated_at) WHERE state = 'reconcile';
