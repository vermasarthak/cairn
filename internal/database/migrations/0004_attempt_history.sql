-- Provider idempotency is scoped to a job, not to a single local execution.
-- Preserve a full receipt history when a retryable failure is attempted again.
ALTER TABLE attempts DROP CONSTRAINT attempts_job_id_idempotency_key_key;
