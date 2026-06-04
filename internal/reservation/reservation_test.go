package reservation

import (
	"sync"
	"testing"
	"time"

	"github.com/vermasarthak/cairn/internal/policy"
)

func allowedDecision(now time.Time) policy.Decision {
	return policy.Decision{Allowed: true, Reason: policy.Allowed, PolicyID: "daily", PolicyVersion: 1, EvaluatedAt: now, LocalDay: "2026-09-19"}
}

func TestReserveAllowsExactlyOneConcurrentPlanner(t *testing.T) {
	store := NewStore()
	now := time.Date(2026, 9, 19, 10, 0, 0, 0, time.UTC)
	subject := policy.Subject{ID: "s1", TenantID: "t1"}
	action := policy.Action{Key: "check-in"}
	var wg sync.WaitGroup
	created := make(chan bool, 32)
	for range 32 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, wasCreated, err := store.Reserve(allowedDecision(now), subject, action)
			if err != nil {
				t.Errorf("reserve: %v", err)
				return
			}
			created <- wasCreated
		}()
	}
	wg.Wait()
	close(created)
	count := 0
	for wasCreated := range created {
		if wasCreated {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("created %d reservations, want 1", count)
	}
}

func TestReservationCannotReturnFromDelivered(t *testing.T) {
	store := NewStore()
	now := time.Date(2026, 9, 19, 10, 0, 0, 0, time.UTC)
	reservation, _, err := store.Reserve(allowedDecision(now), policy.Subject{ID: "s1", TenantID: "t1"}, policy.Action{Key: "check-in"})
	if err != nil {
		t.Fatal(err)
	}
	for _, state := range []State{Queued, Claimed, Delivered} {
		if _, err := store.Transition(reservation.Key, state, now); err != nil {
			t.Fatalf("transition to %s: %v", state, err)
		}
	}
	if _, err := store.Transition(reservation.Key, Queued, now); err == nil {
		t.Fatal("expected terminal delivered state to reject transition")
	}
}
