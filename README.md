# Cairn

**A durable, policy-aware delivery runtime for user-facing actions.**

Cairn is being built for a narrow, difficult problem: deciding whether a user-facing action is still allowed, reserving it exactly once for a defined policy window, and making every later delivery attempt inspectable.

## Status

`v0.0.1` is the deterministic domain core. It is not a server, scheduler, queue, or production-ready delivery system yet.

Implemented today:

- explicit policy evaluation for consent, quiet hours, and per-local-day caps;
- injectable/virtual time for deterministic testing;
- concurrency-safe in-memory reservation uniqueness;
- legal reservation state transitions; and
- tests for time-zone boundaries, duplicate reservations, and concurrent planners.

## Non-goals for this increment

- no provider integration;
- no database or queue;
- no claim of exactly-once external delivery;
- no AI-generated action authority; and
- no public deployment.

## Guarantees under development

The eventual service will offer:

1. idempotent event ingress;
2. at-most-one active reservation per tenant, subject, policy, action, and local-day window;
3. at-least-once worker execution; and
4. effective-once provider delivery only where a provider supports an idempotency key.

See [ARCHITECTURE.md](ARCHITECTURE.md) and the ADRs in `docs/adr/` for the boundaries and trade-offs.

## Run the core tests

```bash
go test ./...
```

The repository pins Go 1.25. A containerized test command will be added before the first public release.

## Local database

The next increment adds Postgres as the durable source of truth. Start it with `docker compose up -d postgres`; the schema lives in `internal/database/migrations/0001_init.sql`. The database is not yet connected to a public API or worker process.

After applying the migration, run the database proof with `CAIRN_TEST_DATABASE_URL=postgres://cairn:cairn_dev_only@localhost:54321/cairn?sslmode=disable go test -tags=integration -race ./...`.
