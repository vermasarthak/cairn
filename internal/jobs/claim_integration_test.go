//go:build integration

package jobs

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/vermasarthak/cairn/internal/policy"
	"github.com/vermasarthak/cairn/internal/reservation"
	"github.com/vermasarthak/cairn/internal/testdb"
)

func TestClaimNextClaimsOnceAndReclaimsExpiredLease(t *testing.T) {
	pool := testdb.New(t)
	now := time.Now().UTC()
	_, err := reservation.NewPostgresStore(pool).ReserveAndEnqueue(context.Background(), policy.Decision{Allowed: true, Reason: policy.Allowed, PolicyID: "daily", PolicyVersion: 1, LocalDay: "2026-09-21", EvaluatedAt: now}, policy.Subject{ID: "subject", TenantID: "tenant"}, policy.Action{Key: "check-in"})
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
	reclaimed, ok, err := store.ClaimNext(context.Background(), now.Add(2*time.Second), time.Second)
	if err != nil || !ok || reclaimed.ID == "" {
		t.Fatalf("reclaim: ok=%t id=%s err=%v", ok, reclaimed.ID, err)
	}
}
