package reservation

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/vermasarthak/cairn/internal/policy"
)

// PostgresStore makes the reservation uniqueness invariant durable.
type PostgresStore struct{ pool *pgxpool.Pool }

func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore { return &PostgresStore{pool: pool} }

func (s *PostgresStore) Reserve(ctx context.Context, decision policy.Decision, subject policy.Subject, action policy.Action) (Reservation, bool, error) {
	if !decision.Allowed {
		return Reservation{}, false, fmt.Errorf("cannot reserve disallowed decision: %s", decision.Reason)
	}
	day, err := time.Parse("2006-01-02", decision.LocalDay)
	if err != nil {
		return Reservation{}, false, fmt.Errorf("parse local day: %w", err)
	}
	key := Key{TenantID: subject.TenantID, SubjectID: subject.ID, PolicyID: decision.PolicyID, ActionKey: action.Key, LocalDay: decision.LocalDay}
	row := s.pool.QueryRow(ctx, `INSERT INTO reservations (tenant_id, subject_id, policy_id, action_key, local_day, policy_version, state, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$8) ON CONFLICT (tenant_id, subject_id, policy_id, action_key, local_day) DO NOTHING RETURNING policy_version, state, created_at, updated_at`, key.TenantID, key.SubjectID, key.PolicyID, key.ActionKey, day, decision.PolicyVersion, Reserved, decision.EvaluatedAt.UTC())
	reservation := Reservation{Key: key}
	err = row.Scan(&reservation.PolicyVersion, &reservation.State, &reservation.CreatedAt, &reservation.UpdatedAt)
	if err == nil {
		return reservation, true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Reservation{}, false, fmt.Errorf("insert reservation: %w", err)
	}
	err = s.pool.QueryRow(ctx, `SELECT policy_version, state, created_at, updated_at FROM reservations WHERE tenant_id=$1 AND subject_id=$2 AND policy_id=$3 AND action_key=$4 AND local_day=$5`, key.TenantID, key.SubjectID, key.PolicyID, key.ActionKey, day).Scan(&reservation.PolicyVersion, &reservation.State, &reservation.CreatedAt, &reservation.UpdatedAt)
	if err != nil {
		return Reservation{}, false, fmt.Errorf("load existing reservation: %w", err)
	}
	return reservation, false, nil
}
