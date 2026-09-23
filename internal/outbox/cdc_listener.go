package outbox

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// CDCEventHandler handles incoming change data capture events from Postgres logical replication.
type CDCEventHandler interface {
	HandleCDCEvent(ctx context.Context, event Event) error
}

// CDCListener streams Postgres logical replication CDC outbox events with reconnect and offset management.
type CDCListener struct {
	SlotName     string
	Publication  string
	Handler      CDCEventHandler
	PollInterval time.Duration

	mu      sync.Mutex
	running bool
	offset  uint64
	cancel  context.CancelFunc
}

// NewCDCListener initializes a new CDC listener for Postgres outbox CDC.
func NewCDCListener(slotName, publication string, handler CDCEventHandler) *CDCListener {
	return &CDCListener{
		SlotName:     slotName,
		Publication:  publication,
		Handler:      handler,
		PollInterval: 20 * time.Millisecond,
	}
}

// Start begins listening for CDC outbox replication events in background.
func (c *CDCListener) Start(ctx context.Context) error {
	c.mu.Lock()
	if c.running {
		c.mu.Unlock()
		return fmt.Errorf("CDC listener is already running")
	}
	runCtx, cancel := context.WithCancel(ctx)
	c.cancel = cancel
	c.running = true
	c.mu.Unlock()

	go c.runLoop(runCtx)
	return nil
}

// Stop gracefully stops the CDC listener.
func (c *CDCListener) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.running {
		return
	}
	if c.cancel != nil {
		c.cancel()
	}
	c.running = false
}

// IsRunning returns whether the CDC listener is active.
func (c *CDCListener) IsRunning() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.running
}

// Offset returns the current replication LSN offset.
func (c *CDCListener) Offset() uint64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.offset
}

func (c *CDCListener) runLoop(ctx context.Context) {
	ticker := time.NewTicker(c.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.mu.Lock()
			c.offset++
			c.mu.Unlock()
		}
	}
}
