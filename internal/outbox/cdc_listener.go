package outbox

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// SimulatedCDCEventHandler handles incoming change events in simulation / test harnesses.
type SimulatedCDCEventHandler interface {
	HandleCDCEvent(ctx context.Context, event Event) error
}

// SimulatedStreamListener provides a ticker-based event polling simulation for test harnesses.
// Note: This is an in-memory simulation component for test harnesses, not a Postgres logical replication connection.
type SimulatedStreamListener struct {
	SlotName     string
	Publication  string
	Handler      SimulatedCDCEventHandler
	PollInterval time.Duration

	mu      sync.Mutex
	running bool
	offset  uint64
	cancel  context.CancelFunc
}

// CDCEventHandler is an alias for SimulatedCDCEventHandler for test compatibility.
type CDCEventHandler = SimulatedCDCEventHandler

// CDCListener is an alias for SimulatedStreamListener for test compatibility.
type CDCListener = SimulatedStreamListener

// NewSimulatedStreamListener initializes a ticker-based test stream listener simulation.
func NewSimulatedStreamListener(slotName, publication string, handler SimulatedCDCEventHandler) *SimulatedStreamListener {
	return &SimulatedStreamListener{
		SlotName:     slotName,
		Publication:  publication,
		Handler:      handler,
		PollInterval: 20 * time.Millisecond,
	}
}

// NewCDCListener initializes a test stream listener (simulation support).
func NewCDCListener(slotName, publication string, handler CDCEventHandler) *CDCListener {
	return NewSimulatedStreamListener(slotName, publication, handler)
}

// Start begins listening for simulated events in background.
func (c *SimulatedStreamListener) Start(ctx context.Context) error {
	c.mu.Lock()
	if c.running {
		c.mu.Unlock()
		return fmt.Errorf("simulated stream listener is already running")
	}
	runCtx, cancel := context.WithCancel(ctx)
	c.cancel = cancel
	c.running = true
	c.mu.Unlock()

	go c.runLoop(runCtx)
	return nil
}

// Stop gracefully stops the simulated stream listener.
func (c *SimulatedStreamListener) Stop() {
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

// IsRunning returns whether the listener is active.
func (c *SimulatedStreamListener) IsRunning() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.running
}

// Offset returns the current simulated monotonic sequence offset (not a Postgres LSN).
func (c *SimulatedStreamListener) Offset() uint64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.offset
}

func (c *SimulatedStreamListener) runLoop(ctx context.Context) {
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
