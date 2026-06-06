ALTER TABLE jobs ADD COLUMN lease_token UUID;
ALTER TABLE jobs ADD CONSTRAINT jobs_lease_token_check CHECK (
  (state IN ('claimed', 'reconciling')) = (lease_token IS NOT NULL)
);
