package outbox

import (
	"fmt"
	"sync"
)

// LSNGap represents a missing sequence range during failover.
type LSNGap struct {
	StartLSN uint64
	EndLSN   uint64
}

// LSNReconciler tracks LSN sequence offsets and detects gaps during failover.
type LSNReconciler struct {
	mu         sync.Mutex
	lastAckLSN uint64
	gaps       []LSNGap
}

// NewLSNReconciler initializes a new LSN reconciler.
func NewLSNReconciler(initialLSN uint64) *LSNReconciler {
	return &LSNReconciler{
		lastAckLSN: initialLSN,
		gaps:       make([]LSNGap, 0),
	}
}

// Reconcile checks an incoming LSN offset against lastAckLSN and records any gap.
func (r *LSNReconciler) Reconcile(incomingLSN uint64) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if incomingLSN <= r.lastAckLSN {
		return false, fmt.Errorf("duplicate or stale LSN %d (last ack: %d)", incomingLSN, r.lastAckLSN)
	}

	hasGap := false
	if incomingLSN > r.lastAckLSN+1 {
		gap := LSNGap{
			StartLSN: r.lastAckLSN + 1,
			EndLSN:   incomingLSN - 1,
		}
		r.gaps = append(r.gaps, gap)
		hasGap = true
	}

	r.lastAckLSN = incomingLSN
	return hasGap, nil
}

// Gaps returns list of detected sequence gaps.
func (r *LSNReconciler) Gaps() []LSNGap {
	r.mu.Lock()
	defer r.mu.Unlock()
	copied := make([]LSNGap, len(r.gaps))
	copy(copied, r.gaps)
	return copied
}

// ResolveGap resolves and removes a gap once out-of-order LSN messages are replayed.
func (r *LSNReconciler) ResolveGap(startLSN, endLSN uint64) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	for i, gap := range r.gaps {
		if gap.StartLSN == startLSN && gap.EndLSN == endLSN {
			r.gaps = append(r.gaps[:i], r.gaps[i+1:]...)
			return true
		}
	}
	return false
}
