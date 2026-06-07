package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/vermasarthak/cairn/internal/api"
	"github.com/vermasarthak/cairn/internal/policy"
	"github.com/vermasarthak/cairn/internal/reservation"
)

func main() {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}
	tokens, err := parseTokens(os.Getenv("CAIRN_API_KEYS"))
	if err != nil {
		log.Fatal(err)
	}
	pool, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()
	server := api.Server{Policies: policy.NewPostgresStore(pool), Reservations: reservation.NewPostgresStore(pool), TokenTenants: tokens}
	httpServer := &http.Server{Addr: ":8080", Handler: server.Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second}
	log.Printf("cairn API listening on %s", httpServer.Addr)
	log.Fatal(httpServer.ListenAndServe())
}

func parseTokens(value string) (map[string]string, error) {
	tokens := map[string]string{}
	for _, pair := range strings.Split(value, ",") {
		tenant, token, ok := strings.Cut(pair, "=")
		if !ok || tenant == "" || token == "" {
			return nil, fmt.Errorf("CAIRN_API_KEYS must contain tenant=secret entries")
		}
		if _, exists := tokens[token]; exists {
			return nil, fmt.Errorf("duplicate API key")
		}
		tokens[token] = tenant
	}
	return tokens, nil
}
