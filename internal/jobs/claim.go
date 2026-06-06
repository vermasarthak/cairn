package jobs

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Job struct {
	ID         string
	TenantID   string
	LeaseToken string
	LeaseUntil time.Time
}

type Store struct{ pool *pgxpool.Pool }

func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// ClaimNext atomically claims one available or expired job. It offers
// at-least-once execution: an expired lease may be claimed again.
func (s *Store) ClaimNext(ctx context.Context, now time.Time, lease time.Duration) (Job, bool, error) {
	var job Job
	leaseToken := uuid.NewString()
	err := s.pool.QueryRow(ctx, `
WITH candidate AS (
  SELECT id FROM jobs
  WHERE (state = 'queued' AND available_at <= $1) OR (state = 'claimed' AND lease_until <= $1)
  ORDER BY available_at, id
  FOR UPDATE SKIP LOCKED
  LIMIT 1
)
UPDATE jobs SET state = 'claimed', lease_until = $2, lease_token = $3, updated_at = $1
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
