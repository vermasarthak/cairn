package reservation

import (
	"fmt"
	"sync"
	"time"

	"github.com/vermasarthak/cairn/internal/policy"
)

type State string

const (
	Reserved   State = "reserved"
	Queued     State = "queued"
	Claimed    State = "claimed"
	Delivered  State = "delivered"
	Suppressed State = "suppressed"
	Failed     State = "failed"
)

type Key struct {
	TenantID  string
	SubjectID string
	PolicyID  string
	ActionKey string
	LocalDay  string
}

type Reservation struct {
	Key           Key
	PolicyVersion int
	State         State
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type Store struct {
	mu    sync.Mutex
	items map[Key]Reservation
}

func NewStore() *Store {
	return &Store{items: make(map[Key]Reservation)}
}

// Reserve creates one reservation per policy window. A duplicate caller receives
// the existing record rather than a second delivery opportunity.
func (s *Store) Reserve(decision policy.Decision, subject policy.Subject, action policy.Action) (Reservation, bool, error) {
	if !decision.Allowed {
		return Reservation{}, false, fmt.Errorf("cannot reserve disallowed decision: %s", decision.Reason)
	}
	key := Key{TenantID: subject.TenantID, SubjectID: subject.ID, PolicyID: decision.PolicyID, ActionKey: action.Key, LocalDay: decision.LocalDay}
	s.mu.Lock()
	defer s.mu.Unlock()
	if existing, ok := s.items[key]; ok {
		return existing, false, nil
	}
	reservation := Reservation{Key: key, PolicyVersion: decision.PolicyVersion, State: Reserved, CreatedAt: decision.EvaluatedAt, UpdatedAt: decision.EvaluatedAt}
	s.items[key] = reservation
	return reservation, true, nil
}

func (s *Store) Transition(key Key, next State, now time.Time) (Reservation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	current, ok := s.items[key]
	if !ok {
		return Reservation{}, fmt.Errorf("reservation not found")
	}
	if !allowedTransition(current.State, next) {
		return Reservation{}, fmt.Errorf("invalid transition %s -> %s", current.State, next)
	}
	current.State = next
	current.UpdatedAt = now.UTC()
	s.items[key] = current
	return current, nil
}

func allowedTransition(current, next State) bool {
	return (current == Reserved && (next == Queued || next == Suppressed)) ||
		(current == Queued && (next == Claimed || next == Suppressed || next == Failed)) ||
		(current == Claimed && (next == Delivered || next == Queued || next == Suppressed || next == Failed))
}
