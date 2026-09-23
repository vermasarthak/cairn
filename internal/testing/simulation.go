package testing

import (
	"math/rand"
	"sync"
	"time"
)

// SimulationConfig configures deterministic simulation parameters.
type SimulationConfig struct {
	Seed           int64
	NumNodes       int
	MaxDelay       time.Duration
	FaultInjection bool
}

// EventType represents simulated distributed event types.
type EventType string

const (
	EventMessageSent      EventType = "MESSAGE_SENT"
	EventMessageDelivered EventType = "MESSAGE_DELIVERED"
	EventNodeCrash        EventType = "NODE_CRASH"
	EventNodeRecover      EventType = "NODE_RECOVER"
)

// SimEvent represents a scheduled event in the simulation.
type SimEvent struct {
	ID        int64
	Type      EventType
	NodeID    int
	Scheduled time.Time
	Payload   interface{}
}

// Simulator executes deterministic simulation testing across simulated nodes.
type Simulator struct {
	config  SimulationConfig
	rnd     *rand.Rand
	mu      sync.Mutex
	now     time.Time
	events  []SimEvent
	nextID  int64
	crashed map[int]bool
}

// NewSimulator creates a new deterministic simulator initialized with a seed.
func NewSimulator(cfg SimulationConfig) *Simulator {
	if cfg.MaxDelay == 0 {
		cfg.MaxDelay = 100 * time.Millisecond
	}
	return &Simulator{
		config:  cfg,
		rnd:     rand.New(rand.NewSource(cfg.Seed)),
		now:     time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		crashed: make(map[int]bool),
	}
}

// Step advances the simulation by processing the next scheduled event.
func (s *Simulator) Step() (SimEvent, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.events) == 0 {
		return SimEvent{}, false
	}

	minIdx := 0
	for i := 1; i < len(s.events); i++ {
		if s.events[i].Scheduled.Before(s.events[minIdx].Scheduled) {
			minIdx = i
		}
	}

	ev := s.events[minIdx]
	s.events = append(s.events[:minIdx], s.events[minIdx+1:]...)
	s.now = ev.Scheduled

	if ev.Type == EventNodeCrash {
		s.crashed[ev.NodeID] = true
	} else if ev.Type == EventNodeRecover {
		delete(s.crashed, ev.NodeID)
	}

	return ev, true
}

// Schedule adds a new event to the simulation queue with deterministic delay.
func (s *Simulator) Schedule(nodeID int, eventType EventType, payload interface{}) SimEvent {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.nextID++
	delayMs := s.rnd.Int63n(int64(s.config.MaxDelay / time.Millisecond))
	if delayMs <= 0 {
		delayMs = 1
	}
	scheduledTime := s.now.Add(time.Duration(delayMs) * time.Millisecond)

	ev := SimEvent{
		ID:        s.nextID,
		Type:      eventType,
		NodeID:    nodeID,
		Scheduled: scheduledTime,
		Payload:   payload,
	}
	s.events = append(s.events, ev)
	return ev
}

// IsNodeAlive returns whether a node is currently alive.
func (s *Simulator) IsNodeAlive(nodeID int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return !s.crashed[nodeID]
}
