package outbox_test

import (
	"context"
	"testing"
	"time"

	"github.com/vermasarthak/cairn/internal/outbox"
)

type mockCDCHandler struct{}

func (m *mockCDCHandler) HandleCDCEvent(ctx context.Context, event outbox.Event) error {
	return nil
}

func TestCDCListenerLifecycle(t *testing.T) {
	handler := &mockCDCHandler{}
	listener := outbox.NewCDCListener("cairn_slot", "cairn_pub", handler)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	if err := listener.Start(ctx); err != nil {
		t.Fatalf("failed to start CDC listener: %v", err)
	}

	if !listener.IsRunning() {
		t.Errorf("expected listener to be running")
	}

	time.Sleep(50 * time.Millisecond)

	if listener.Offset() == 0 {
		t.Errorf("expected offset to advance")
	}

	listener.Stop()

	if listener.IsRunning() {
		t.Errorf("expected listener to be stopped")
	}
}
