package outbox

import (
	"testing"
)

func TestLSNReconciler(t *testing.T) {
	rec := NewLSNReconciler(100)

	hasGap, err := rec.Reconcile(101)
	if err != nil || hasGap {
		t.Fatalf("expected no gap for sequential LSN, got gap=%v err=%v", hasGap, err)
	}

	hasGap, err = rec.Reconcile(105)
	if err != nil || !hasGap {
		t.Fatalf("expected gap for jump from 101 to 105, got gap=%v err=%v", hasGap, err)
	}

	gaps := rec.Gaps()
	if len(gaps) != 1 || gaps[0].StartOffset != 102 || gaps[0].EndOffset != 104 {
		t.Fatalf("unexpected gaps: %+v", gaps)
	}

	resolved := rec.ResolveGap(102, 104)
	if !resolved || len(rec.Gaps()) != 0 {
		t.Fatalf("expected gap 102-104 to be resolved, remaining gaps: %+v", rec.Gaps())
	}
}

func TestW3CTraceContext(t *testing.T) {
	raw := "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"
	tc, err := ParseTraceparent(raw)
	if err != nil {
		t.Fatalf("failed to parse traceparent: %v", err)
	}

	if tc.Format() != raw {
		t.Fatalf("formatted mismatch: %s vs %s", tc.Format(), raw)
	}

	invalidLengths := []string{
		"0-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01",
		"00-4bf92f3577b34da6a3ce929d0e0e473-00f067aa0ba902b7-01",
		"00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b-01",
		"00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-1",
	}
	for _, inv := range invalidLengths {
		if _, err := ParseTraceparent(inv); err == nil {
			t.Fatalf("expected error for invalid traceparent length: %s", inv)
		}
	}
}
