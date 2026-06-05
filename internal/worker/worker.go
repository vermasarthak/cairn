package worker

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/vermasarthak/cairn/internal/jobs"
	"github.com/vermasarthak/cairn/internal/provider"
	"github.com/vermasarthak/cairn/internal/retry"
)

type Runner struct {
	Jobs     *jobs.Store
	Provider interface {
		Send(context.Context, provider.Request) jobs.Outcome
	}
	Retry retry.Policy
	Lease time.Duration
}

func (r Runner) RunOnce(ctx context.Context, now time.Time) (bool, error) {
	job, claimed, err := r.Jobs.ClaimNext(ctx, now, r.Lease)
	if err != nil || !claimed {
		return claimed, err
	}
	outcome := r.Provider.Send(ctx, provider.Request{JobID: job.ID, IdempotencyKey: job.ID})
	_, err = r.Jobs.RecordAttempt(ctx, jobs.Attempt{ID: uuid.NewString(), JobID: job.ID, IdempotencyKey: job.ID, Outcome: outcome}, now, r.Retry.Next(now, 1))
	return true, err
}
