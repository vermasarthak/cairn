//go:build integration

package worker

import (
	"context"
	"testing"
	"time"

	"github.com/vermasarthak/cairn/internal/jobs"
	"github.com/vermasarthak/cairn/internal/policy"
	"github.com/vermasarthak/cairn/internal/provider"
	"github.com/vermasarthak/cairn/internal/reservation"
	"github.com/vermasarthak/cairn/internal/retry"
	"github.com/vermasarthak/cairn/internal/testdb"
)

func TestRetryableProviderFailureSchedulesLaterRetry(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	now := time.Now().UTC()
	_, err := reservation.NewPostgresStore(pool).ReserveAndEnqueue(ctx, policy.Decision{Allowed: true, Reason: policy.Allowed, PolicyID: "daily", PolicyVersion: 1, LocalDay: "2026-09-23", EvaluatedAt: now}, policy.Subject{ID: "subject", TenantID: "tenant"}, policy.Action{Key: "check-in"})
	if err != nil {
		t.Fatal(err)
	}
	fake := &provider.Fake{Outcomes: []jobs.Outcome{jobs.RetryableError, jobs.Accepted}}
	r := Runner{Jobs: jobs.NewStore(pool), Provider: fake, Retry: retry.Policy{Base: time.Minute, Max: time.Minute}, Lease: time.Minute}
	if ran, err := r.RunOnce(ctx, now); err != nil || !ran {
		t.Fatalf("first run: %t %v", ran, err)
	}
	if _, ok, err := jobs.NewStore(pool).ClaimNext(ctx, now, time.Minute); err != nil || ok {
		t.Fatalf("retried too early: %t %v", ok, err)
	}
	if ran, err := r.RunOnce(ctx, now.Add(time.Minute)); err != nil || !ran {
		t.Fatalf("second run: %t %v", ran, err)
	}
	var attempts int
	var state string
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM attempts").Scan(&attempts); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, "SELECT state FROM jobs").Scan(&state); err != nil {
		t.Fatal(err)
	}
	if attempts != 2 || state != "succeeded" {
		t.Fatalf("attempts=%d state=%s want 2 succeeded", attempts, state)
	}
	if len(fake.Requests) != 2 || fake.Requests[0].IdempotencyKey != fake.Requests[1].IdempotencyKey {
		t.Fatalf("provider calls=%d; expected stable idempotency key", len(fake.Requests))
	}
}
