//go:build integration

package outbox

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/vermasarthak/cairn/internal/policy"
	"github.com/vermasarthak/cairn/internal/reservation"
	"github.com/vermasarthak/cairn/internal/testdb"
)

func TestDispatcherReclaimsFailedPublishAfterLeaseExpiry(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
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
func TestOutboxDispatchIdempotency(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	now := time.Now().UTC()
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

	ran, err := dispatcher.RunOnce(ctx, now)
	if err != nil {
		t.Fatalf("first RunOnce error: %v", err)
	}
	if !ran {
		t.Fatal("first RunOnce: expected ran=true, got false")
	}

	ran2, err := dispatcher.RunOnce(ctx, now.Add(time.Second))
	if err != nil {
		t.Fatalf("second RunOnce error: %v", err)
	}
	if ran2 {
		t.Fatal("second RunOnce: expected ran=false (no claimable event), got true")
	}

	if len(publisher.Events) != 1 {
		t.Fatalf("publisher called %d times, want exactly 1", len(publisher.Events))
	}

	var publishedCount int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM outbox WHERE published_at IS NOT NULL").Scan(&publishedCount); err != nil {
		t.Fatal(err)
	}
	if publishedCount != 1 {
		t.Fatalf("published_at rows = %d, want 1", publishedCount)
	}

	var unpublishedCount int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM outbox WHERE published_at IS NULL").Scan(&unpublishedCount); err != nil {
		t.Fatal(err)
	}
	if unpublishedCount != 0 {
		t.Fatalf("unpublished outbox events = %d, want 0", unpublishedCount)
	}
}
