package provider

import (
	"context"
	"sync"

	"github.com/vermasarthak/cairn/internal/jobs"
)

type Request struct{ JobID, IdempotencyKey string }
type LookupRequest struct{ JobID, IdempotencyKey string }
type Fake struct {
	mu                    sync.Mutex
	Outcomes              []jobs.Outcome
	ReconciliationResults []jobs.ReconciliationResult
	Requests              []Request
	Lookups               []LookupRequest
}

func (f *Fake) Lookup(_ context.Context, request LookupRequest) jobs.ReconciliationResult {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Lookups = append(f.Lookups, request)
	if len(f.ReconciliationResults) == 0 {
		return jobs.StillUnknown
	}
	result := f.ReconciliationResults[0]
	f.ReconciliationResults = f.ReconciliationResults[1:]
	return result
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
