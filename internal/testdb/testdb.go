//go:build integration

package testdb

import (
	"context"
	"fmt"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/vermasarthak/cairn/internal/database/migrator"
)

var schemaSeq uint64

// New creates an isolated PostgreSQL schema for the test, runs all database
// migrations, sets the connection search_path to the schema, and automatically
// drops the schema when the test completes. This guarantees total test isolation
// under parallel test execution across packages.
func New(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("CAIRN_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("CAIRN_TEST_DATABASE_URL is required")
	}

	seq := atomic.AddUint64(&schemaSeq, 1)
	schemaName := fmt.Sprintf("test_schema_%d_%d_%d", os.Getpid(), time.Now().UnixNano()%100000, seq)

	adminConfig, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatalf("parse test database config: %v", err)
	}

	adminPool, err := pgxpool.NewWithConfig(context.Background(), adminConfig)
	if err != nil {
		t.Fatalf("connect to admin test database: %v", err)
	}

	ctx := context.Background()
	if _, err := adminPool.Exec(ctx, fmt.Sprintf("CREATE SCHEMA %q", schemaName)); err != nil {
		adminPool.Close()
		t.Fatalf("create test schema %s: %v", schemaName, err)
	}

	t.Cleanup(func() {
		adminPool.Exec(context.Background(), fmt.Sprintf("DROP SCHEMA %q CASCADE", schemaName))
		adminPool.Close()
	})

	testConfig, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatalf("parse test database config: %v", err)
	}
	testConfig.ConnConfig.RuntimeParams["search_path"] = schemaName

	pool, err := pgxpool.NewWithConfig(context.Background(), testConfig)
	if err != nil {
		t.Fatalf("connect to test schema %s: %v", schemaName, err)
	}

	t.Cleanup(func() {
		pool.Close()
	})

	if err := migrator.ApplyAll(context.Background(), pool); err != nil {
		t.Fatalf("apply migrations to test schema %s: %v", schemaName, err)
	}

	return pool
}
