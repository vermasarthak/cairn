package reservation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/vermasarthak/cairn/internal/policy"
)

type EnqueueResult struct {
	Reservation Reservation
	JobID       string
	Created     bool
}

// PostgresStore makes the reservation uniqueness invariant durable.
type PostgresStore struct{ pool *pgxpool.Pool }

func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore { return &PostgresStore{pool: pool} }

// ReserveAndEnqueue makes reservation, queued work, audit evidence, and the
// outbox event atomic. A duplicate planner receives the existing reservation.
func (s *PostgresStore) ReserveAndEnqueue(ctx context.Context, decision policy.Decision, subject policy.Subject, action policy.Action) (EnqueueResult, error) {
	if !decision.Allowed {
		return EnqueueResult{}, fmt.Errorf("cannot reserve disallowed decision: %s", decision.Reason)
	}
	day, err := time.Parse("2006-01-02", decision.LocalDay)
	if err != nil {
		return EnqueueResult{}, fmt.Errorf("parse local day: %w", err)
	}
	key := Key{TenantID: subject.TenantID, SubjectID: subject.ID, PolicyID: decision.PolicyID, ActionKey: action.Key, LocalDay: decision.LocalDay}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return EnqueueResult{}, fmt.Errorf("begin reservation transaction: %w", err)
	}
	defer tx.Rollback(ctx)
	reservation := Reservation{Key: key}
	err = tx.QueryRow(ctx, `INSERT INTO reservations (tenant_id, subject_id, policy_id, action_key, local_day, policy_version, state, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$8) ON CONFLICT (tenant_id, subject_id, policy_id, action_key, local_day) DO NOTHING RETURNING policy_version,state,created_at,updated_at`, key.TenantID, key.SubjectID, key.PolicyID, key.ActionKey, day, decision.PolicyVersion, Reserved, decision.EvaluatedAt.UTC()).Scan(&reservation.PolicyVersion, &reservation.State, &reservation.CreatedAt, &reservation.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		err = tx.QueryRow(ctx, `SELECT policy_version,state,created_at,updated_at FROM reservations WHERE tenant_id=$1 AND subject_id=$2 AND policy_id=$3 AND action_key=$4 AND local_day=$5`, key.TenantID, key.SubjectID, key.PolicyID, key.ActionKey, day).Scan(&reservation.PolicyVersion, &reservation.State, &reservation.CreatedAt, &reservation.UpdatedAt)
		if err != nil {
			return EnqueueResult{}, err
		}
		var jobID string
		if err = tx.QueryRow(ctx, `SELECT id::text FROM jobs WHERE reservation_tenant_id=$1 AND reservation_subject_id=$2 AND reservation_policy_id=$3 AND reservation_action_key=$4 AND reservation_local_day=$5`, key.TenantID, key.SubjectID, key.PolicyID, key.ActionKey, day).Scan(&jobID); err != nil {
			return EnqueueResult{}, fmt.Errorf("load existing job: %w", err)
		}
		if err = tx.Commit(ctx); err != nil {
			return EnqueueResult{}, err
		}
		return EnqueueResult{Reservation: reservation, JobID: jobID}, nil
	}
	if err != nil {
		return EnqueueResult{}, fmt.Errorf("insert reservation: %w", err)
	}
	var jobID string
	err = tx.QueryRow(ctx, `INSERT INTO jobs (id,tenant_id,reservation_tenant_id,reservation_subject_id,reservation_policy_id,reservation_action_key,reservation_local_day,state,available_at) VALUES (gen_random_uuid(),$1,$1,$2,$3,$4,$5,'queued',$6) RETURNING id::text`, key.TenantID, key.SubjectID, key.PolicyID, key.ActionKey, day, decision.EvaluatedAt.UTC()).Scan(&jobID)
	if err != nil {
		return EnqueueResult{}, fmt.Errorf("create job: %w", err)
	}
	payload, _ := json.Marshal(map[string]string{"job_id": jobID, "tenant_id": key.TenantID})
	if _, err = tx.Exec(ctx, `INSERT INTO outbox (topic,payload) VALUES ('job.queued',$1)`, payload); err != nil {
		return EnqueueResult{}, fmt.Errorf("write outbox: %w", err)
	}
	entity, _ := json.Marshal(key)
	if _, err = tx.Exec(ctx, `INSERT INTO audit_events (tenant_id,event_type,entity_type,entity_key,payload) VALUES ($1,'reservation.created','reservation',$2,$3)`, key.TenantID, entity, payload); err != nil {
		return EnqueueResult{}, fmt.Errorf("write audit event: %w", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return EnqueueResult{}, fmt.Errorf("commit reservation transaction: %w", err)
	}
	return EnqueueResult{Reservation: reservation, JobID: jobID, Created: true}, nil
}

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
