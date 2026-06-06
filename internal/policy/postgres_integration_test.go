//go:build integration

package policy

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPostgresPolicyVersionsAreImmutable(t *testing.T) {
	url := os.Getenv("CAIRN_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("CAIRN_TEST_DATABASE_URL is required")
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if _, err := pool.Exec(context.Background(), "TRUNCATE policies"); err != nil {
		t.Fatal(err)
	}
	store := NewPostgresStore(pool)
	v1 := Policy{ID: "daily-check-in", Version: 1, Active: true, QuietStartHour: 22, QuietEndHour: 8, MaxPerLocalDay: 1}
	if created, err := store.Put(context.Background(), "tenant", v1); err != nil || !created {
		t.Fatalf("first put: created=%t err=%v", created, err)
	}
	if created, err := store.Put(context.Background(), "tenant", v1); err != nil || created {
		t.Fatalf("idempotent put: created=%t err=%v", created, err)
	}
	changed := v1
	changed.MaxPerLocalDay = 2
	if _, err := store.Put(context.Background(), "tenant", changed); err == nil {
		t.Fatal("mutated policy version was accepted")
	}
	v2 := v1
	v2.Version = 2
	v2.MaxPerLocalDay = 2
	if created, err := store.Put(context.Background(), "tenant", v2); err != nil || !created {
		t.Fatalf("version two: created=%t err=%v", created, err)
	}
	latest, err := store.Latest(context.Background(), "tenant", v1.ID)
	if err != nil || latest.Version != 2 || latest.MaxPerLocalDay != 2 {
		t.Fatalf("latest=%+v err=%v", latest, err)
	}
}
