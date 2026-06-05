package retry

import (
	"testing"
	"time"
)

func TestBackoffCapsDeterministically(t *testing.T) {
	now := time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC)
	p := Policy{Base: time.Second, Max: 4 * time.Second}
	if got := p.Next(now, 1); !got.Equal(now.Add(time.Second)) {
		t.Fatal(got)
	}
	if got := p.Next(now, 4); !got.Equal(now.Add(4 * time.Second)) {
		t.Fatal(got)
	}
}
