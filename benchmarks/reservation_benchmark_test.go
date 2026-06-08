//go:build integration

package benchmarks

import (
	"context"
	"fmt"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/vermasarthak/cairn/internal/policy"
	"github.com/vermasarthak/cairn/internal/reservation"
)

func BenchmarkReserveAndEnqueueParallel(b *testing.B) {
	url := os.Getenv("CAIRN_BENCH_DATABASE_URL")
	if url == "" {
		b.Skip("CAIRN_BENCH_DATABASE_URL is required")
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		b.Fatal(err)
	}
	defer pool.Close()
	if _, err := pool.Exec(context.Background(), "TRUNCATE audit_events, outbox, reconciliations, attempts, jobs, reservations CASCADE"); err != nil {
		b.Fatal(err)
	}
	store := reservation.NewPostgresStore(pool)
	now := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)
	var sequence atomic.Uint64
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			i := sequence.Add(1)
			if _, err := store.ReserveAndEnqueue(context.Background(), policy.Decision{Allowed: true, Reason: policy.Allowed, PolicyID: "benchmark", PolicyVersion: 1, LocalDay: "2026-09-25", EvaluatedAt: now}, policy.Subject{ID: fmt.Sprintf("subject-%d", i), TenantID: "benchmark"}, policy.Action{Key: fmt.Sprintf("action-%d", i)}); err != nil {
				b.Error(err)
			}
		}
	})
}
