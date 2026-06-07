package outbox

import (
	"context"
	"time"

	"github.com/vermasarthak/cairn/internal/observability"
)

type Publisher interface {
	Publish(context.Context, Event) error
}

type Dispatcher struct {
	Store     *Store
	Publisher Publisher
	Lease     time.Duration
	Metrics   *observability.Metrics
}

func (d Dispatcher) RunOnce(ctx context.Context, now time.Time) (bool, error) {
	event, claimed, err := d.Store.ClaimNext(ctx, now, d.Lease)
	if err != nil || !claimed {
		return claimed, err
	}
	if err := d.Publisher.Publish(ctx, event); err != nil {
		d.Metrics.Inc("cairn_outbox_publish_failures_total")
		return true, err
	}
	_, err = d.Store.MarkPublished(ctx, event, now)
	if err == nil {
		d.Metrics.Inc("cairn_outbox_published_total")
	}
	return true, err
}
