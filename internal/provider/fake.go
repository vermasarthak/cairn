package provider

import (
	"context"
	"sync"

	"github.com/vermasarthak/cairn/internal/jobs"
)

type Request struct{ JobID, IdempotencyKey string }
type Fake struct {
	mu       sync.Mutex
	Outcomes []jobs.Outcome
	Requests []Request
}

func (f *Fake) Send(_ context.Context, request Request) jobs.Outcome {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Requests = append(f.Requests, request)
	if len(f.Outcomes) == 0 {
		return jobs.Accepted
	}
	outcome := f.Outcomes[0]
	f.Outcomes = f.Outcomes[1:]
	return outcome
}
