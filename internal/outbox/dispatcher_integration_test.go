//go:build integration

package outbox

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/vermasarthak/cairn/internal/policy"
	"github.com/vermasarthak/cairn/internal/reservation"
)

func TestDispatcherReclaimsFailedPublishAfterLeaseExpiry(t *testing.T) {
	url := os.Getenv("CAIRN_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("CAIRN_TEST_DATABASE_URL is required")
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	ctx := context.Background()
	if _, err := pool.Exec(ctx, "TRUNCATE audit_events, outbox, reconciliations, attempts, jobs, reservations CASCADE"); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if _, err := reservation.NewPostgresStore(pool).ReserveAndEnqueue(ctx, policy.Decision{Allowed: true, Reason: policy.Allowed, PolicyID: "daily", PolicyVersion: 1, LocalDay: "2026-09-25", EvaluatedAt: now}, policy.Subject{ID: "subject", TenantID: "tenant"}, policy.Action{Key: "check-in"}); err != nil {
		t.Fatal(err)
	}
	first := &FakePublisher{Errors: []error{errors.New("broker unavailable")}}
	if _, err := (Dispatcher{Store: NewStore(pool), Publisher: first, Lease: time.Minute}).RunOnce(ctx, now); err == nil {
		t.Fatal("publish failure was swallowed")
	}
	second := &FakePublisher{}
	if ran, err := (Dispatcher{Store: NewStore(pool), Publisher: second, Lease: time.Minute}).RunOnce(ctx, now.Add(2*time.Minute)); err != nil || !ran {
		t.Fatalf("reclaim: ran=%t err=%v", ran, err)
	}
	var published int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM outbox WHERE published_at IS NOT NULL").Scan(&published); err != nil {
		t.Fatal(err)
	}
	if len(first.Events) != 1 || len(second.Events) != 1 || first.Events[0].ID != second.Events[0].ID || published != 1 {
		t.Fatalf("first=%d second=%d published=%d", len(first.Events), len(second.Events), published)
	}
}

// TestOutboxDispatchIdempotency verifies that dispatching the same outbox event
// twice does not result in duplicate publishes or duplicate published_at marks.
//
// The first Dispatcher.RunOnce call claims the event, publishes it successfully,
// and marks it as published (published_at IS NOT NULL, lease_token cleared).
// A second RunOnce call on the same store must find no claimable events
// (the event is already published) and therefore must not call Publish again.
func TestOutboxDispatchIdempotency(t *testing.T) {
	url := os.Getenv("CAIRN_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("CAIRN_TEST_DATABASE_URL is required")
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	ctx := context.Background()
	if _, err := pool.Exec(ctx, "TRUNCATE audit_events, outbox, reconciliations, attempts, jobs, reservations CASCADE"); err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC()
	// Create exactly one outbox event via the reservation path.
	if _, err := reservation.NewPostgresStore(pool).ReserveAndEnqueue(
		ctx,
		policy.Decision{
			Allowed:       true,
			Reason:        policy.Allowed,
			PolicyID:      "daily",
			PolicyVersion: 1,
			LocalDay:      "2026-09-26",
			EvaluatedAt:   now,
		},
		policy.Subject{ID: "subject-idem", TenantID: "tenant-idem"},
		policy.Action{Key: "notification"},
	); err != nil {
		t.Fatal(err)
	}

	store := NewStore(pool)
	publisher := &FakePublisher{}
	dispatcher := Dispatcher{Store: store, Publisher: publisher, Lease: time.Minute}

	// First dispatch — should claim, publish, and mark published.
	ran, err := dispatcher.RunOnce(ctx, now)
	if err != nil {
		t.Fatalf("first RunOnce error: %v", err)
	}
	if !ran {
		t.Fatal("first RunOnce: expected ran=true, got false")
	}

	// Second dispatch — event is already published; ClaimNext should return nothing.
	ran2, err := dispatcher.RunOnce(ctx, now.Add(time.Second))
	if err != nil {
		t.Fatalf("second RunOnce error: %v", err)
	}
	if ran2 {
		t.Fatal("second RunOnce: expected ran=false (no claimable event), got true")
	}

	// Publisher must have been called exactly once.
	if len(publisher.Events) != 1 {
		t.Fatalf("publisher called %d times, want exactly 1", len(publisher.Events))
	}

	// Database must show exactly one published event.
	var publishedCount int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM outbox WHERE published_at IS NOT NULL").Scan(&publishedCount); err != nil {
		t.Fatal(err)
	}
	if publishedCount != 1 {
		t.Fatalf("published_at rows = %d, want 1", publishedCount)
	}

	// No event should remain claimable.
	var unpublishedCount int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM outbox WHERE published_at IS NULL").Scan(&unpublishedCount); err != nil {
		t.Fatal(err)
	}
	if unpublishedCount != 0 {
		t.Fatalf("unpublished outbox events = %d, want 0", unpublishedCount)
	}
}
