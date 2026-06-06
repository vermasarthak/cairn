//go:build integration

package migrator

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/vermasarthak/cairn/internal/database/migrations"
)

func TestApplyAllIsRepeatableAndRecordsChecksums(t *testing.T) {
	url := os.Getenv("CAIRN_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("CAIRN_TEST_DATABASE_URL is required")
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
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
	url := os.Getenv("CAIRN_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("CAIRN_TEST_DATABASE_URL is required")
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if _, err := pool.Exec(context.Background(), "CREATE TABLE IF NOT EXISTS migration_checksum_test (id integer)"); err != nil {
		t.Fatal(err)
	}
	first := []migrations.Migration{{Version: "9998_checksum_test.sql", SQL: "INSERT INTO migration_checksum_test (id) VALUES (1);"}}
	if err := Apply(context.Background(), pool, first); err != nil {
		t.Fatal(err)
	}
	edited := []migrations.Migration{{Version: "9998_checksum_test.sql", SQL: "INSERT INTO migration_checksum_test (id) VALUES (2);"}}
	if err := Apply(context.Background(), pool, edited); err == nil {
		t.Fatal("edited migration was accepted")
	}
}
