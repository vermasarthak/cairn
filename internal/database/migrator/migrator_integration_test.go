//go:build integration

package migrator

import (
	"context"
	"fmt"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/vermasarthak/cairn/internal/database/migrations"
)

var schemaSeq uint64

func setupMigratorPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("CAIRN_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("CAIRN_TEST_DATABASE_URL is required")
	}

	seq := atomic.AddUint64(&schemaSeq, 1)
	schemaName := fmt.Sprintf("migrator_schema_%d_%d_%d", os.Getpid(), time.Now().UnixNano()%100000, seq)

	adminConfig, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	adminPool, err := pgxpool.NewWithConfig(context.Background(), adminConfig)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	if _, err := adminPool.Exec(ctx, fmt.Sprintf("CREATE SCHEMA %q", schemaName)); err != nil {
		adminPool.Close()
		t.Fatal(err)
	}

	t.Cleanup(func() {
		adminPool.Exec(context.Background(), fmt.Sprintf("DROP SCHEMA %q CASCADE", schemaName))
		adminPool.Close()
	})

	testConfig, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	testConfig.ConnConfig.RuntimeParams["search_path"] = schemaName

	pool, err := pgxpool.NewWithConfig(context.Background(), testConfig)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		pool.Close()
	})

	return pool
}

func TestApplyAllIsRepeatableAndRecordsChecksums(t *testing.T) {
	pool := setupMigratorPool(t)
	if err := ApplyAll(context.Background(), pool); err != nil {
		t.Fatal(err)
	}
	if err := ApplyAll(context.Background(), pool); err != nil {
		t.Fatal(err)
	}
	all, err := migrations.All()
	if err != nil {
		t.Fatal(err)
	}
	var recorded int
	if err := pool.QueryRow(context.Background(), "SELECT count(*) FROM schema_migrations WHERE version LIKE '0%' AND checksum IS NOT NULL").Scan(&recorded); err != nil {
		t.Fatal(err)
	}
	if recorded != len(all) {
		t.Fatalf("recorded migrations=%d want %d", recorded, len(all))
	}
}

func TestApplyRejectsEditedHistory(t *testing.T) {
	pool := setupMigratorPool(t)
	first := []migrations.Migration{{Version: "9998_checksum_test.sql", SQL: "CREATE TABLE IF NOT EXISTS migration_checksum_test (id integer); INSERT INTO migration_checksum_test (id) VALUES (1);"}}
	if err := Apply(context.Background(), pool, first); err != nil {
		t.Fatal(err)
	}
	edited := []migrations.Migration{{Version: "9998_checksum_test.sql", SQL: "CREATE TABLE IF NOT EXISTS migration_checksum_test (id integer); INSERT INTO migration_checksum_test (id) VALUES (2);"}}
	if err := Apply(context.Background(), pool, edited); err == nil {
		t.Fatal("expected failure on edited migration checksum")
	}
}
