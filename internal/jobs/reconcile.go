package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ReconciliationResult reflects what a provider's status endpoint can prove.
// Only Absent permits a job to return to the ordinary delivery queue.
type ReconciliationResult string

const (
	Confirmed              ReconciliationResult = "confirmed"
	ReconciliationRejected ReconciliationResult = "rejected"
	Absent                 ReconciliationResult = "absent"
	StillUnknown           ReconciliationResult = "unknown"
)

type Reconciliation struct {
	ID         string
	JobID      string
	LeaseToken string
	Result     ReconciliationResult
	Receipt    json.RawMessage
}

// ClaimNextReconciliation leases an ambiguous job without making it eligible
// for the normal sender. Expired reconciliation leases are safe to reclaim.
func (s *Store) ClaimNextReconciliation(ctx context.Context, now time.Time, lease time.Duration) (Job, bool, error) {
	var job Job
	leaseToken := uuid.NewString()
	err := s.pool.QueryRow(ctx, `
WITH candidate AS (
  SELECT id FROM jobs
  WHERE state = 'reconcile' OR (state = 'reconciling' AND lease_until <= $1)
  ORDER BY updated_at, id
  FOR UPDATE SKIP LOCKED
  LIMIT 1
)
UPDATE jobs SET state = 'reconciling', lease_until = $2, lease_token = $3, updated_at = $1
WHERE id = (SELECT id FROM candidate)
RETURNING id::text, tenant_id, lease_token::text, lease_until`, now.UTC(), now.UTC().Add(lease), leaseToken).Scan(&job.ID, &job.TenantID, &job.LeaseToken, &job.LeaseUntil)
	if err == nil {
		return job, true, nil
	}
	if err == pgx.ErrNoRows {
		return Job{}, false, nil
	}
	return Job{}, false, err
}

// RecordReconciliation persists an immutable provider-status lookup and the
// resulting job transition atomically. Unknown status remains reconciliation
// work; it cannot accidentally return to ordinary delivery.
func (s *Store) RecordReconciliation(ctx context.Context, reconciliation Reconciliation, now time.Time) (bool, error) {
	if reconciliation.ID == "" || reconciliation.JobID == "" || reconciliation.LeaseToken == "" {
		return false, fmt.Errorf("reconciliation id, job id, and lease token are required")
	}
	if reconciliation.Result != Confirmed && reconciliation.Result != ReconciliationRejected && reconciliation.Result != Absent && reconciliation.Result != StillUnknown {
		return false, fmt.Errorf("unsupported reconciliation result %q", reconciliation.Result)
	}
	if len(reconciliation.Receipt) == 0 {
		reconciliation.Receipt = json.RawMessage(`{}`)
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `INSERT INTO reconciliations (id, job_id, result, provider_receipt) VALUES ($1,$2,$3,$4) ON CONFLICT (id) DO NOTHING`, reconciliation.ID, reconciliation.JobID, reconciliation.Result, reconciliation.Receipt)
	if err != nil {
		return false, fmt.Errorf("insert reconciliation: %w", err)
	}
	if tag.RowsAffected() == 0 {
		if err := tx.Commit(ctx); err != nil {
			return false, err
		}
		return false, nil
	}
	next := "reconcile"
	availableAt := now.UTC()
	switch reconciliation.Result {
	case Confirmed:
		next = "succeeded"
	case ReconciliationRejected:
		next = "failed"
	case Absent:
		next = "queued"
	}
	tag, err = tx.Exec(ctx, `UPDATE jobs SET state=$2, available_at=$3, lease_until=NULL, lease_token=NULL, updated_at=$4 WHERE id=$1 AND state='reconciling' AND lease_token=$5`, reconciliation.JobID, next, availableAt, now.UTC(), reconciliation.LeaseToken)
	if err != nil {
		return false, fmt.Errorf("transition reconciled job: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return false, fmt.Errorf("reconciling job %s is not owned by this worker", reconciliation.JobID)
	}
	if err = tx.Commit(ctx); err != nil {
		return false, err
	}
	return true, nil
}
