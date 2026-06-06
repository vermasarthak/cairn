package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

type Outcome string

const (
	Accepted       Outcome = "accepted"
	Rejected       Outcome = "rejected"
	Unknown        Outcome = "unknown"
	RetryableError Outcome = "retryable_error"
	PermanentError Outcome = "permanent_error"
)

type Attempt struct {
	ID             string
	JobID          string
	LeaseToken     string
	IdempotencyKey string
	Outcome        Outcome
	Receipt        json.RawMessage
}

// RecordAttempt persists a provider outcome and job transition atomically.
// Unknown outcomes move to reconcile and are never automatically retried.
func (s *Store) RecordAttempt(ctx context.Context, attempt Attempt, now, retryAt time.Time) (bool, error) {
	if attempt.ID == "" || attempt.JobID == "" || attempt.LeaseToken == "" || attempt.IdempotencyKey == "" {
		return false, fmt.Errorf("attempt id, job id, lease token, and idempotency key are required")
	}
	if attempt.Outcome != Accepted && attempt.Outcome != Rejected && attempt.Outcome != Unknown && attempt.Outcome != RetryableError && attempt.Outcome != PermanentError {
		return false, fmt.Errorf("unsupported outcome %q", attempt.Outcome)
	}
	if len(attempt.Receipt) == 0 {
		attempt.Receipt = json.RawMessage(`{}`)
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)
	// A provider idempotency key is intentionally stable across retries of one job.
	// Attempt identity is local and unique per execution, so every delivery try stays
	// inspectable without causing the provider to perform the action twice.
	tag, err := tx.Exec(ctx, `INSERT INTO attempts (id,job_id,idempotency_key,outcome,provider_receipt) VALUES ($1,$2,$3,$4,$5) ON CONFLICT (id) DO NOTHING`, attempt.ID, attempt.JobID, attempt.IdempotencyKey, attempt.Outcome, attempt.Receipt)
	if err != nil {
		return false, fmt.Errorf("insert attempt: %w", err)
	}
	if tag.RowsAffected() == 0 {
		if err := tx.Commit(ctx); err != nil {
			return false, err
		}
		return false, nil
	}
	next := "failed"
	availableAt := now.UTC()
	switch attempt.Outcome {
	case Accepted:
		next = "succeeded"
	case Unknown:
		next = "reconcile"
	case RetryableError:
		next = "queued"
		availableAt = retryAt.UTC()
	}
	tag, err = tx.Exec(ctx, `UPDATE jobs SET state=$2, available_at=$3, lease_until=NULL, lease_token=NULL, updated_at=$4 WHERE id=$1 AND state='claimed' AND lease_token=$5`, attempt.JobID, next, availableAt, now.UTC(), attempt.LeaseToken)
	if err != nil {
		return false, fmt.Errorf("transition job: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return false, fmt.Errorf("claimed job %s is not owned by this worker", attempt.JobID)
	}
	if err = tx.Commit(ctx); err != nil {
		return false, err
	}
	return true, nil
}
