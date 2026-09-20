//go:build integration

package policy

import (
	"context"
	"testing"

	"github.com/vermasarthak/cairn/internal/testdb"
)

func TestPostgresPolicyVersionsAreImmutable(t *testing.T) {
	pool := testdb.New(t)
	store := NewPostgresStore(pool)
	created1, err := store.Put(context.Background(), "tenant-a", Policy{ID: "daily", Version: 1, Active: true, QuietStartHour: 22, QuietEndHour: 8, MaxPerLocalDay: 1})
	if err != nil || !created1 {
		t.Fatalf("v1 put: created=%t err=%v", created1, err)
	}
	created2, err := store.Put(context.Background(), "tenant-a", Policy{ID: "daily", Version: 2, Active: true, QuietStartHour: 22, QuietEndHour: 8, MaxPerLocalDay: 2})
	if err != nil || !created2 {
		t.Fatalf("v2 put: created=%t err=%v", created2, err)
	}
	v1Read, err := store.Get(context.Background(), "tenant-a", "daily", 1)
	if err != nil || v1Read.MaxPerLocalDay != 1 {
		t.Fatalf("v1: p=%+v err=%v", v1Read, err)
	}
	v2Read, err := store.Get(context.Background(), "tenant-a", "daily", 2)
	if err != nil || v2Read.MaxPerLocalDay != 2 {
		t.Fatalf("v2: p=%+v err=%v", v2Read, err)
	}
}
