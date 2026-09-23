package retry

import "time"

// Policy is deterministic so retry behavior is reproducible in tests and audits.
type Policy struct{ Base, Max time.Duration }

func (p Policy) Next(now time.Time, attempt int) time.Time {
	if attempt <= 0 {
		attempt = 1
	}
	if attempt > 62 {
		attempt = 62
	}
	if p.Base <= 0 {
		p.Base = time.Second
	}
	if p.Max <= 0 {
		p.Max = time.Minute
	}
	delay := p.Base
	for i := 1; i < attempt && delay < p.Max; i++ {
		delay *= 2
	}
	if delay > p.Max {
		delay = p.Max
	}
	return now.UTC().Add(delay)
}
