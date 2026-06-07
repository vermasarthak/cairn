package outbox

import (
	"context"
	"sync"
)

type FakePublisher struct {
	mu     sync.Mutex
	Events []Event
	Errors []error
}

func (f *FakePublisher) Publish(_ context.Context, event Event) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Events = append(f.Events, event)
	if len(f.Errors) == 0 {
		return nil
	}
	err := f.Errors[0]
	f.Errors = f.Errors[1:]
	return err
}
