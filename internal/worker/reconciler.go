package worker

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/vermasarthak/cairn/internal/jobs"
	"github.com/vermasarthak/cairn/internal/provider"
)

type Reconciler struct {
	Jobs     *jobs.Store
	Provider interface {
		Lookup(context.Context, provider.LookupRequest) jobs.ReconciliationResult
	}
	Lease time.Duration
}

// RunOnce handles a single ambiguous provider result. It is intentionally
// separate from Runner so an ordinary sender can never claim this work.
func (r Reconciler) RunOnce(ctx context.Context, now time.Time) (bool, error) {
	job, claimed, err := r.Jobs.ClaimNextReconciliation(ctx, now, r.Lease)
	if err != nil || !claimed {
		return claimed, err
	}
	result := r.Provider.Lookup(ctx, provider.LookupRequest{JobID: job.ID, IdempotencyKey: job.ID})
	_, err = r.Jobs.RecordReconciliation(ctx, jobs.Reconciliation{ID: uuid.NewString(), JobID: job.ID, Result: result}, now)
	return true, err
}
