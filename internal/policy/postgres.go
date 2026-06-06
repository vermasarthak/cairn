package policy

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresStore persists immutable, tenant-scoped policy versions. Event
// ingress later references an ID; it never supplies the policy definition.
type PostgresStore struct{ pool *pgxpool.Pool }

func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore { return &PostgresStore{pool: pool} }

// Put stores one immutable policy version. Repeating an identical write is
// idempotent; changing an existing version is rejected.
func (s *PostgresStore) Put(ctx context.Context, tenantID string, policy Policy) (bool, error) {
	if tenantID == "" {
		return false, fmt.Errorf("tenant id is required")
	}
	if err := policy.validate(); err != nil {
		return false, err
	}
	definition, err := json.Marshal(policy)
	if err != nil {
		return false, fmt.Errorf("encode policy: %w", err)
	}
	var created bool
	err = s.pool.QueryRow(ctx, `
INSERT INTO policies (tenant_id, id, version, definition)
VALUES ($1, $2, $3, $4)
ON CONFLICT (tenant_id, id, version) DO NOTHING
RETURNING true`, tenantID, policy.ID, policy.Version, definition).Scan(&created)
	if err == nil {
		return created, nil
	}
	if err != pgx.ErrNoRows {
		return false, fmt.Errorf("store policy: %w", err)
	}
	existing, err := s.Get(ctx, tenantID, policy.ID, policy.Version)
	if err != nil {
		return false, err
	}
	if !reflect.DeepEqual(existing, policy) {
		return false, fmt.Errorf("policy %s version %d already exists with a different definition", policy.ID, policy.Version)
	}
	return false, nil
}

func (s *PostgresStore) Get(ctx context.Context, tenantID, id string, version int) (Policy, error) {
	var raw []byte
	err := s.pool.QueryRow(ctx, `SELECT definition FROM policies WHERE tenant_id=$1 AND id=$2 AND version=$3`, tenantID, id, version).Scan(&raw)
	if err != nil {
		if err == pgx.ErrNoRows {
			return Policy{}, fmt.Errorf("policy %s version %d not found", id, version)
		}
		return Policy{}, fmt.Errorf("load policy: %w", err)
	}
	var policy Policy
	if err := json.Unmarshal(raw, &policy); err != nil {
		return Policy{}, fmt.Errorf("decode policy: %w", err)
	}
	if policy.ID != id || policy.Version != version {
		return Policy{}, fmt.Errorf("stored policy identity does not match row")
	}
	return policy, nil
}

func (s *PostgresStore) Latest(ctx context.Context, tenantID, id string) (Policy, error) {
	var version int
	err := s.pool.QueryRow(ctx, `SELECT version FROM policies WHERE tenant_id=$1 AND id=$2 ORDER BY version DESC LIMIT 1`, tenantID, id).Scan(&version)
	if err != nil {
		if err == pgx.ErrNoRows {
			return Policy{}, fmt.Errorf("policy %s not found", id)
		}
		return Policy{}, fmt.Errorf("load latest policy: %w", err)
	}
	return s.Get(ctx, tenantID, id, version)
}
