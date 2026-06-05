//go:build integration

package jobs

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/vermasarthak/cairn/internal/policy"
	"github.com/vermasarthak/cairn/internal/reservation"
)

func TestClaimNextClaimsOnceAndReclaimsExpiredLease(t *testing.T) {
	url := os.Getenv("CAIRN_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("CAIRN_TEST_DATABASE_URL is required")
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if _, err := pool.Exec(context.Background(), "TRUNCATE audit_events, outbox, attempts, jobs, reservations CASCADE"); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	_, err = reservation.NewPostgresStore(pool).ReserveAndEnqueue(context.Background(), policy.Decision{Allowed: true, Reason: policy.Allowed, PolicyID: "daily", PolicyVersion: 1, LocalDay: "2026-09-21", EvaluatedAt: now}, policy.Subject{ID: "subject", TenantID: "tenant"}, policy.Action{Key: "check-in"})
	if err != nil {
		t.Fatal(err)
	}
	store := NewStore(pool)
	claimed := make(chan bool, 12)
	var wg sync.WaitGroup
	for range 12 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, ok, err := store.ClaimNext(context.Background(), now, time.Second)
			if err != nil {
				t.Error(err)
				return
			}
			claimed <- ok
		}()
	}
	wg.Wait()
	close(claimed)
	count := 0
	for ok := range claimed {
		if ok {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("claimed %d jobs, want 1", count)
	}
	if _, ok, err := store.ClaimNext(context.Background(), now.Add(2*time.Second), time.Second); err != nil || !ok {
		t.Fatalf("expired lease was not reclaimed: ok=%t err=%v", ok, err)
	}
}
