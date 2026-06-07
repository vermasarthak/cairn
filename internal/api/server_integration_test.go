//go:build integration

package api

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/vermasarthak/cairn/internal/policy"
	"github.com/vermasarthak/cairn/internal/reservation"
)

func TestAuthenticatedIngressCreatesReservationJobAndOutbox(t *testing.T) {
	url := os.Getenv("CAIRN_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("CAIRN_TEST_DATABASE_URL is required")
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if _, err := pool.Exec(context.Background(), "TRUNCATE audit_events, outbox, reconciliations, attempts, jobs, reservations, policies CASCADE"); err != nil {
		t.Fatal(err)
	}
	policies := policy.NewPostgresStore(pool)
	if _, err := policies.Put(context.Background(), "tenant-a", policy.Policy{ID: "daily", Version: 1, Active: true, QuietStartHour: 22, QuietEndHour: 8, MaxPerLocalDay: 1}); err != nil {
		t.Fatal(err)
	}
	server := Server{Policies: policies, Reservations: reservation.NewPostgresStore(pool), TokenTenants: map[string]string{"test-secret": "tenant-a"}, Now: func() time.Time { return time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC) }}
	request := httptest.NewRequest(http.MethodPost, "/v1/reservations", bytes.NewBufferString(`{"policy_id":"daily","subject":{"id":"subject-a","time_zone":"UTC","consented":true},"action":{"key":"check-in"}}`))
	request.Header.Set("X-Cairn-API-Key", "test-secret")
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var reservations, jobs, outbox int
	if err := pool.QueryRow(context.Background(), "SELECT count(*) FROM reservations").Scan(&reservations); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(context.Background(), "SELECT count(*) FROM jobs").Scan(&jobs); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(context.Background(), "SELECT count(*) FROM outbox WHERE topic='job.queued'").Scan(&outbox); err != nil {
		t.Fatal(err)
	}
	if reservations != 1 || jobs != 1 || outbox != 1 {
		t.Fatalf("reservations=%d jobs=%d outbox=%d", reservations, jobs, outbox)
	}
}
