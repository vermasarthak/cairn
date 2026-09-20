package migrator

import (
	"context"
	"crypto/sha256"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/vermasarthak/cairn/internal/database/migrations"
)

const advisoryLockID int64 = 541187201

// ApplyAll serializes schema changes with a database advisory lock and records
// a content checksum for every migration. Historical migrations are immutable:
// changing one after it has been applied fails closed.
func ApplyAll(ctx context.Context, pool *pgxpool.Pool) error {
	all, err := migrations.All()
	if err != nil {
		return err
	}
	return Apply(ctx, pool, all)
}

func Apply(ctx context.Context, pool *pgxpool.Pool, all []migrations.Migration) error {
	connection, err := pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire migration connection: %w", err)
	}
	defer connection.Release()
	if _, err := connection.Exec(ctx, "SELECT pg_advisory_lock($1)", advisoryLockID); err != nil {
		return fmt.Errorf("acquire migration lock: %w", err)
	}
	defer connection.Exec(context.Background(), "SELECT pg_advisory_unlock($1)", advisoryLockID)
	if _, err := connection.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (version TEXT PRIMARY KEY, applied_at TIMESTAMPTZ NOT NULL DEFAULT now()); ALTER TABLE schema_migrations ADD COLUMN IF NOT EXISTS checksum TEXT`); err != nil {
		return fmt.Errorf("bootstrap migration metadata: %w", err)
	}
	applied := map[string]string{}
	rows, err := connection.Query(ctx, "SELECT version, checksum FROM schema_migrations")
	if err != nil {
		return fmt.Errorf("load migration metadata: %w", err)
	}
	for rows.Next() {
		var version, checksum string
		if err := rows.Scan(&version, &checksum); err != nil {
			rows.Close()
			return fmt.Errorf("scan migration metadata: %w", err)
		}
		applied[version] = checksum
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("iterate migration metadata: %w", err)
	}
	rows.Close()
	if len(applied) == 0 {
		var existingTables int
		if err := connection.QueryRow(ctx, `SELECT count(*) FROM information_schema.tables WHERE table_schema = current_schema() AND table_name <> 'schema_migrations'`).Scan(&existingTables); err != nil {
			return fmt.Errorf("inspect existing schema: %w", err)
		}
		if existingTables != 0 {
			return fmt.Errorf("schema_migrations is empty but the database already has application tables; use a fresh database or explicitly baseline it after verifying its schema")
		}
	}
	for _, migration := range all {
		checksum := fmt.Sprintf("%x", sha256.Sum256([]byte(migration.SQL)))
		if recorded, ok := applied[migration.Version]; ok {
			if recorded != checksum {
				return fmt.Errorf("migration checksum mismatch for %s", migration.Version)
			}
			continue
		}
		tx, err := connection.Begin(ctx)
		if err != nil {
			return fmt.Errorf("begin migration %s: %w", migration.Version, err)
		}
		if _, err = tx.Exec(ctx, migration.SQL); err == nil {
			_, err = tx.Exec(ctx, "INSERT INTO schema_migrations (version, checksum) VALUES ($1, $2)", migration.Version, checksum)
		}
		if err != nil {
			tx.Rollback(ctx)
			return fmt.Errorf("apply migration %s: %w", migration.Version, err)
		}
		if err = tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit migration %s: %w", migration.Version, err)
		}
	}
	return nil
}
