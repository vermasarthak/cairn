//go:build integration

package jobs

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/vermasarthak/cairn/internal/policy"
	"github.com/vermasarthak/cairn/internal/reservation"
)

func claimedJob(t *testing.T, pool *pgxpool.Pool) Job {
	t.Helper()
	ctx := context.Background()
	_, err := pool.Exec(ctx, "TRUNCATE audit_events, outbox, attempts, jobs, reservations CASCADE")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	_, err = reservation.NewPostgresStore(pool).ReserveAndEnqueue(ctx, policy.Decision{Allowed: true, Reason: policy.Allowed, PolicyID: "daily", PolicyVersion: 1, LocalDay: "2026-09-22", EvaluatedAt: now}, policy.Subject{ID: "subject", TenantID: "tenant"}, policy.Action{Key: "check-in"})
	if err != nil {
		t.Fatal(err)
	}
	job, ok, err := NewStore(pool).ClaimNext(ctx, now, time.Minute)
	if err != nil || !ok {
		t.Fatalf("claim: ok=%t err=%v", ok, err)
	}
	return job
}

func TestUnknownAttemptRequiresReconciliation(t *testing.T) {
	url := os.Getenv("CAIRN_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("CAIRN_TEST_DATABASE_URL is required")
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	job := claimedJob(t, pool)
	now := time.Now().UTC()
	stored, err := NewStore(pool).RecordAttempt(context.Background(), Attempt{ID: uuid.NewString(), JobID: job.ID, IdempotencyKey: "provider-key", Outcome: Unknown}, now, now)
	if err != nil || !stored {
		t.Fatalf("record: stored=%t err=%v", stored, err)
	}
	var state string
	if err := pool.QueryRow(context.Background(), "SELECT state FROM jobs WHERE id=$1", job.ID).Scan(&state); err != nil {
		t.Fatal(err)
	}
	if state != "reconcile" {
		t.Fatalf("state=%s want reconcile", state)
	}
}

func TestDuplicateAttemptDoesNotCreateSecondReceipt(t *testing.T) {
	url := os.Getenv("CAIRN_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("CAIRN_TEST_DATABASE_URL is required")
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	job := claimedJob(t, pool)
	store := NewStore(pool)
	now := time.Now().UTC()
	attempt := Attempt{ID: uuid.NewString(), JobID: job.ID, IdempotencyKey: "provider-key", Outcome: Accepted}
	if ok, err := store.RecordAttempt(context.Background(), attempt, now, now); err != nil || !ok {
		t.Fatalf("first record: %t %v", ok, err)
	}
	if ok, err := store.RecordAttempt(context.Background(), attempt, now, now); err != nil || ok {
		t.Fatalf("duplicate record: %t %v", ok, err)
	}
	var count int
	if err := pool.QueryRow(context.Background(), "SELECT count(*) FROM attempts WHERE job_id=$1", job.ID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("attempts=%d want 1", count)
	}
}
