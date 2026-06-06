package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/vermasarthak/cairn/internal/database/migrator"
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
	if err := migrator.ApplyAll(context.Background(), pool); err != nil {
		log.Fatal(err)
	}
	fmt.Println("cairn migrations are current")
}
