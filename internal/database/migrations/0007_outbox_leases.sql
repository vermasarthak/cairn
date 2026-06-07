ALTER TABLE outbox ADD COLUMN lease_until TIMESTAMPTZ;
ALTER TABLE outbox ADD COLUMN lease_token UUID;
ALTER TABLE outbox ADD CONSTRAINT outbox_lease_token_check CHECK (
  (published_at IS NULL AND lease_until IS NOT NULL) = (lease_token IS NOT NULL)
);
CREATE INDEX outbox_claim_idx ON outbox (id) WHERE published_at IS NULL;
