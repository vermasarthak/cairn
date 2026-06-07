package outbox

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Event struct {
	ID         string
	Topic      string
	Payload    json.RawMessage
	LeaseToken string
}

type Store struct{ pool *pgxpool.Pool }

func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// ClaimNext leases one unpublished event. A lease expiration deliberately
// permits at-least-once publishing; consumers must deduplicate by event ID.
func (s *Store) ClaimNext(ctx context.Context, now time.Time, lease time.Duration) (Event, bool, error) {
	var event Event
	token := uuid.NewString()
	err := s.pool.QueryRow(ctx, `
WITH candidate AS (
  SELECT id FROM outbox
  WHERE published_at IS NULL AND (lease_until IS NULL OR lease_until <= $1)
  ORDER BY id
  FOR UPDATE SKIP LOCKED
  LIMIT 1
)
UPDATE outbox SET lease_until=$2, lease_token=$3
WHERE id = (SELECT id FROM candidate)
RETURNING id::text, topic, payload, lease_token::text`, now.UTC(), now.UTC().Add(lease), token).Scan(&event.ID, &event.Topic, &event.Payload, &event.LeaseToken)
	if err == nil {
		return event, true, nil
	}
	if err == pgx.ErrNoRows {
		return Event{}, false, nil
	}
	return Event{}, false, err
}

func (s *Store) MarkPublished(ctx context.Context, event Event, now time.Time) (bool, error) {
	tag, err := s.pool.Exec(ctx, `UPDATE outbox SET published_at=$3, lease_until=NULL, lease_token=NULL WHERE id=$1 AND published_at IS NULL AND lease_token=$2`, event.ID, event.LeaseToken, now.UTC())
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}
