package main

import (
	"context"
	"log"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/vermasarthak/cairn/internal/policy"
)

func main() {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		log.Fatal("DATABASE_URL is required")
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()
	_, err = policy.NewPostgresStore(pool).Put(context.Background(), "demo", policy.Policy{ID: "daily-check-in", Version: 1, Active: true, QuietStartHour: 22, QuietEndHour: 8, MaxPerLocalDay: 1})
	if err != nil {
		log.Fatal(err)
	}
	log.Print("demo policy is ready for tenant demo")
}
