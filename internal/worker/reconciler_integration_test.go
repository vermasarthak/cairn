//go:build integration

package worker

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/vermasarthak/cairn/internal/jobs"
	"github.com/vermasarthak/cairn/internal/policy"
	"github.com/vermasarthak/cairn/internal/provider"
	"github.com/vermasarthak/cairn/internal/reservation"
)

func reconciliationJob(t *testing.T, pool *pgxpool.Pool) (jobs.Job, time.Time) {
	t.Helper()
	ctx := context.Background()
	if _, err := pool.Exec(ctx, "TRUNCATE audit_events, outbox, reconciliations, attempts, jobs, reservations CASCADE"); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	result, err := reservation.NewPostgresStore(pool).ReserveAndEnqueue(ctx, policy.Decision{Allowed: true, Reason: policy.Allowed, PolicyID: "daily", PolicyVersion: 1, LocalDay: "2026-09-24", EvaluatedAt: now}, policy.Subject{ID: "subject", TenantID: "tenant"}, policy.Action{Key: "check-in"})
	if err != nil {
		t.Fatal(err)
	}
	job, claimed, err := jobs.NewStore(pool).ClaimNext(ctx, now, time.Minute)
	if err != nil || !claimed || job.ID != result.JobID {
		t.Fatalf("claim: job=%s claimed=%t err=%v", job.ID, claimed, err)
	}
	if stored, err := jobs.NewStore(pool).RecordAttempt(ctx, jobs.Attempt{ID: uuid.NewString(), JobID: job.ID, LeaseToken: job.LeaseToken, IdempotencyKey: job.ID, Outcome: jobs.Unknown}, now, now); err != nil || !stored {
		t.Fatalf("unknown attempt: stored=%t err=%v", stored, err)
	}
	return job, now
}

func TestReconcilerConfirmsAmbiguousDeliveryWithoutResending(t *testing.T) {
	url := os.Getenv("CAIRN_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("CAIRN_TEST_DATABASE_URL is required")
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	job, now := reconciliationJob(t, pool)
	fake := &provider.Fake{ReconciliationResults: []jobs.ReconciliationResult{jobs.Confirmed}}
	r := Reconciler{Jobs: jobs.NewStore(pool), Provider: fake, Lease: time.Minute}
	if ran, err := r.RunOnce(context.Background(), now.Add(time.Second)); err != nil || !ran {
		t.Fatalf("reconcile: ran=%t err=%v", ran, err)
	}
	var state string
	var count int
	if err := pool.QueryRow(context.Background(), "SELECT state FROM jobs WHERE id=$1", job.ID).Scan(&state); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(context.Background(), "SELECT count(*) FROM reconciliations WHERE job_id=$1", job.ID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if state != "succeeded" || count != 1 || len(fake.Requests) != 0 || len(fake.Lookups) != 1 {
		t.Fatalf("state=%s reconciliations=%d sends=%d lookups=%d", state, count, len(fake.Requests), len(fake.Lookups))
	}
}

func TestReconcilerLeavesUnknownStatusOutOfDeliveryQueue(t *testing.T) {
	url := os.Getenv("CAIRN_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("CAIRN_TEST_DATABASE_URL is required")
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	_, now := reconciliationJob(t, pool)
	r := Reconciler{Jobs: jobs.NewStore(pool), Provider: &provider.Fake{}, Lease: time.Minute}
	if ran, err := r.RunOnce(context.Background(), now.Add(time.Second)); err != nil || !ran {
		t.Fatalf("reconcile: ran=%t err=%v", ran, err)
	}
	if _, claimed, err := jobs.NewStore(pool).ClaimNext(context.Background(), now.Add(2*time.Second), time.Minute); err != nil || claimed {
		t.Fatalf("unknown reconciliation re-entered delivery: claimed=%t err=%v", claimed, err)
	}
}
