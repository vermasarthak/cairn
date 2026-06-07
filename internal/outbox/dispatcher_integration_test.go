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
