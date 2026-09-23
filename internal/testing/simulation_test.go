package testing_test

import (
	"testing"
	"time"

	sim "github.com/vermasarthak/cairn/internal/testing"
)

func TestDeterministicSimulation(t *testing.T) {
	cfg := sim.SimulationConfig{
		Seed:           42,
		NumNodes:       3,
		MaxDelay:       50 * time.Millisecond,
		FaultInjection: true,
	}

	simulator1 := sim.NewSimulator(cfg)
	simulator1.Schedule(0, sim.EventMessageSent, "hello")
	simulator1.Schedule(1, sim.EventNodeCrash, nil)
	simulator1.Schedule(1, sim.EventNodeRecover, nil)

	var sequence1 []sim.EventType
	for {
		ev, ok := simulator1.Step()
		if !ok {
			break
		}
		sequence1 = append(sequence1, ev.Type)
	}

	// Re-run with SAME seed 42 -> must yield IDENTICAL event order
	simulator2 := sim.NewSimulator(cfg)
	simulator2.Schedule(0, sim.EventMessageSent, "hello")
	simulator2.Schedule(1, sim.EventNodeCrash, nil)
	simulator2.Schedule(1, sim.EventNodeRecover, nil)

	var sequence2 []sim.EventType
	for {
		ev, ok := simulator2.Step()
		if !ok {
			break
		}
		sequence2 = append(sequence2, ev.Type)
	}

	if len(sequence1) != len(sequence2) {
		t.Fatalf("sequence length mismatch: %d vs %d", len(sequence1), len(sequence2))
	}

	for i := range sequence1 {
		if sequence1[i] != sequence2[i] {
			t.Errorf("step %d mismatch: %v vs %v", i, sequence1[i], sequence2[i])
		}
	}
}
