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
	if _, err := pool.Exec(context.Background(), "TRUNCATE audit_events, outbox, jobs, reservations CASCADE"); err != nil {
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
			result, err := store.ReserveAndEnqueue(context.Background(), decision, subject, policy.Action{Key: "check-in"})
			if err != nil {
				t.Error(err)
				return
			}
			created <- result.Created
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
	for _, table := range []string{"reservations", "jobs", "outbox", "audit_events"} {
		var rows int
		if err := pool.QueryRow(context.Background(), "SELECT count(*) FROM "+table).Scan(&rows); err != nil {
			t.Fatal(err)
		}
		if rows != 1 {
			t.Fatalf("%s has %d rows, want 1", table, rows)
		}
	}
}

func TestPostgresReservationRollsBackWhenOutboxWriteFails(t *testing.T) {
	url := os.Getenv("CAIRN_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("CAIRN_TEST_DATABASE_URL is required")
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if _, err := pool.Exec(context.Background(), "TRUNCATE audit_events, outbox, jobs, reservations CASCADE"); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(context.Background(), "ALTER TABLE outbox ADD CONSTRAINT fail_outbox_write CHECK (false)"); err != nil {
		t.Fatal(err)
	}
	defer pool.Exec(context.Background(), "ALTER TABLE outbox DROP CONSTRAINT fail_outbox_write")
	store := NewPostgresStore(pool)
	decision := policy.Decision{Allowed: true, Reason: policy.Allowed, PolicyID: "daily", PolicyVersion: 1, LocalDay: "2026-09-20", EvaluatedAt: time.Now().UTC()}
	_, err = store.ReserveAndEnqueue(context.Background(), decision, policy.Subject{ID: "subject", TenantID: "tenant"}, policy.Action{Key: "check-in"})
	if err == nil {
		t.Fatal("expected outbox constraint failure")
	}
	for _, table := range []string{"reservations", "jobs", "audit_events"} {
		var rows int
		if err := pool.QueryRow(context.Background(), "SELECT count(*) FROM "+table).Scan(&rows); err != nil {
			t.Fatal(err)
		}
		if rows != 0 {
			t.Fatalf("%s has %d rows after rollback, want 0", table, rows)
		}
	}
}

// TestConcurrentReservationUniqueness races exactly two goroutines for the
// same (tenant, subject, policy, action, local-day) tuple and asserts that
// the database uniqueness constraint means only one caller sees Created=true.
// This is the canonical proof that ReserveAndEnqueue is safe under concurrent
// planners even when both callers evaluate the policy simultaneously.
func TestConcurrentReservationUniqueness(t *testing.T) {
	url := os.Getenv("CAIRN_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("CAIRN_TEST_DATABASE_URL is required")
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if _, err := pool.Exec(context.Background(), "TRUNCATE audit_events, outbox, jobs, reservations CASCADE"); err != nil {
		t.Fatal(err)
	}

	store := NewPostgresStore(pool)
	decision := policy.Decision{
		Allowed:       true,
		Reason:        policy.Allowed,
		PolicyID:      "daily-cap",
		PolicyVersion: 1,
		LocalDay:      "2026-09-21",
		EvaluatedAt:   time.Now().UTC(),
	}
	subject := policy.Subject{ID: "user-race", TenantID: "tenant-race"}
	action := policy.Action{Key: "send-notification"}

	// Use a ready channel so both goroutines start as simultaneously as possible.
	ready := make(chan struct{})
	type result struct {
		created bool
		err     error
	}
	results := make(chan result, 2)

	for i := 0; i < 2; i++ {
		go func() {
			<-ready
			r, err := store.ReserveAndEnqueue(context.Background(), decision, subject, action)
			results <- result{r.Created, err}
		}()
	}
	close(ready) // release both goroutines simultaneously

	var createdCount int
	for i := 0; i < 2; i++ {
		r := <-results
		if r.err != nil {
			t.Errorf("ReserveAndEnqueue error: %v", r.err)
		}
		if r.created {
			createdCount++
		}
	}

	if createdCount != 1 {
		t.Fatalf("got %d Created=true, want exactly 1 — uniqueness invariant violated", createdCount)
	}

	// Verify exactly one reservation row exists.
	var reservationRows int
	if err := pool.QueryRow(context.Background(), "SELECT count(*) FROM reservations").Scan(&reservationRows); err != nil {
		t.Fatal(err)
	}
	if reservationRows != 1 {
		t.Fatalf("reservations table has %d rows, want 1", reservationRows)
	}
}
