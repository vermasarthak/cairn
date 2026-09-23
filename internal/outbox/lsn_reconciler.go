package outbox

import (
	"fmt"
	"sync"
)

// SequenceGap represents a missing monotonic sequence offset range.
type SequenceGap struct {
	StartOffset uint64
	EndOffset   uint64
}

// LSNGap is an alias for SequenceGap for compatibility.
type LSNGap = SequenceGap

// SequenceGapReconciler tracks monotonic sequence offsets and detects missing gaps in simulation/stream testing.
// Note: This is a sequence reconciliation helper for simulated streams, not a direct Postgres WAL LSN parser.
type SequenceGapReconciler struct {
	mu            sync.Mutex
	lastAckOffset uint64
	gaps          []SequenceGap
}

// LSNReconciler is an alias for SequenceGapReconciler.
type LSNReconciler = SequenceGapReconciler

// NewSequenceGapReconciler initializes a sequence gap reconciler starting from an initial offset.
func NewSequenceGapReconciler(initialOffset uint64) *SequenceGapReconciler {
	return &SequenceGapReconciler{
		lastAckOffset: initialOffset,
		gaps:          make([]SequenceGap, 0),
	}
}

// NewLSNReconciler initializes a sequence gap reconciler (simulation support).
func NewLSNReconciler(initialLSN uint64) *LSNReconciler {
	return NewSequenceGapReconciler(initialLSN)
}

// Reconcile checks an incoming offset against lastAckOffset and records any missing gap.
func (r *SequenceGapReconciler) Reconcile(incomingOffset uint64) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if incomingOffset <= r.lastAckOffset {
		return false, fmt.Errorf("duplicate or stale sequence offset %d (last ack: %d)", incomingOffset, r.lastAckOffset)
	}

	hasGap := false
	if incomingOffset > r.lastAckOffset+1 {
		gap := SequenceGap{
			StartOffset: r.lastAckOffset + 1,
			EndOffset:   incomingOffset - 1,
		}
		r.gaps = append(r.gaps, gap)
		hasGap = true
	}

	r.lastAckOffset = incomingOffset
	return hasGap, nil
}

// Gaps returns the list of detected sequence gaps.
func (r *SequenceGapReconciler) Gaps() []SequenceGap {
	r.mu.Lock()
	defer r.mu.Unlock()
	copied := make([]SequenceGap, len(r.gaps))
	copy(copied, r.gaps)
	return copied
}

// ResolveGap resolves and removes a sequence gap once out-of-order messages are processed.
func (r *SequenceGapReconciler) ResolveGap(startOffset, endOffset uint64) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	for i, gap := range r.gaps {
		if gap.StartOffset == startOffset && gap.EndOffset == endOffset {
			r.gaps = append(r.gaps[:i], r.gaps[i+1:]...)
			return true
		}
	}
	return false
}
