package outbox

import (
	"context"
	"time"
)

type Publisher interface {
	Publish(context.Context, Event) error
}

type Dispatcher struct {
	Store     *Store
	Publisher Publisher
	Lease     time.Duration
}

func (d Dispatcher) RunOnce(ctx context.Context, now time.Time) (bool, error) {
	event, claimed, err := d.Store.ClaimNext(ctx, now, d.Lease)
	if err != nil || !claimed {
		return claimed, err
	}
	if err := d.Publisher.Publish(ctx, event); err != nil {
		return true, err
	}
	_, err = d.Store.MarkPublished(ctx, event, now)
	return true, err
}
