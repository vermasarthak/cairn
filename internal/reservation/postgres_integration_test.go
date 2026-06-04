//go:build integration

package reservation

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/vermasarthak/cairn/internal/policy"
)

func TestPostgresReserveAllowsOneConcurrentPlanner(t *testing.T) {
	url := os.Getenv("CAIRN_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("CAIRN_TEST_DATABASE_URL is required")
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if _, err := pool.Exec(context.Background(), "TRUNCATE reservations"); err != nil {
		t.Fatal(err)
	}
	store := NewPostgresStore(pool)
	decision := policy.Decision{Allowed: true, Reason: policy.Allowed, PolicyID: "daily", PolicyVersion: 1, LocalDay: "2026-09-19", EvaluatedAt: time.Now().UTC()}
	subject := policy.Subject{ID: "subject", TenantID: "tenant"}
	var wg sync.WaitGroup
	created := make(chan bool, 16)
	for range 16 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, ok, err := store.Reserve(context.Background(), decision, subject, policy.Action{Key: "check-in"})
			if err != nil {
				t.Error(err)
				return
			}
			created <- ok
		}()
	}
	wg.Wait()
	close(created)
	count := 0
	for ok := range created {
		if ok {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("created %d reservations, want 1", count)
	}
}
