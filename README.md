# Cairn

**A durable, policy-aware delivery runtime for user-facing actions.**

Cairn is being built for a narrow, difficult problem: deciding whether a user-facing action is still allowed, reserving it exactly once for a defined policy window, and making every later delivery attempt inspectable.

## Status

`v0.0.1` is a local, deterministic vertical slice. It is not a public server, scheduler, or production-ready delivery system yet.

Implemented today:

- explicit policy evaluation for consent, quiet hours, and per-local-day caps;
- injectable/virtual time for deterministic testing;
- concurrency-safe in-memory reservation uniqueness;
- legal reservation state transitions; and
- Postgres-backed reservation, outbox, lease, and attempt state;
- deterministic retry scheduling and reconciliation transitions; and
- integration proofs for concurrent planners, lease expiry, retries, and ambiguous provider outcomes.

## Non-goals for this increment

- no real provider integration;
- no public API or queue broker;
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

Postgres is the durable source of truth. Start it with `docker compose up -d postgres`, then apply the migrations in `internal/database/migrations/` in lexical order. A local worker can claim one job, call the deterministic provider fake, write an attempt receipt, and either succeed, schedule a retry, or move an ambiguous result to reconciliation.

After applying the migration, run the database proof with `CAIRN_TEST_DATABASE_URL=postgres://cairn:cairn_dev_only@localhost:54321/cairn?sslmode=disable go test -tags=integration -race ./...`.
