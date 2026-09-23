# Contributing to Cairn

Thank you for your interest in contributing to Cairn. Cairn is designed with strict verification and deterministic testing discipline.

## Prerequisites
- Go 1.22+ (tested up to Go 1.25)
- Docker & Docker Compose (for PostgreSQL integration tests)

## Local Development Workflow

1. **Clone and Branch**:
   ```bash
   git clone https://github.com/vermasarthak/cairn.git
   cd cairn
   git checkout -b feature/your-change
   ```

2. **Run Static Analysis & Unit Tests**:
   ```bash
   go vet ./...
   go test -v -race ./...
   ```

3. **Run PostgreSQL Integration Tests**:
   Start a local PostgreSQL instance:
   ```bash
   docker compose up -d postgres
   CAIRN_TEST_DATABASE_URL='postgres://cairn:cairn_dev_only@localhost:54321/cairn?sslmode=disable' go test -v -tags=integration -race ./...
   ```

4. **Coverage Profile**:
   ```bash
   go test -coverprofile=coverage.out ./...
   go tool cover -func=coverage.out
   ```

## Contribution Invariants
- **No Untracked State Transitions**: All reservation status updates must be legal transitions guarded by lease tokens.
- **Deterministic Time**: Never call `time.Now()` directly in core policy evaluation; use injectable `Clock` interfaces.
- **Defensible Documentation**: Do not claim production external publishing where only in-transaction outbox records are created.
