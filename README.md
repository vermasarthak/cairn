# Cairn

**A durable, policy-aware delivery runtime for user-facing actions.**

Cairn is being built for a narrow, difficult problem: deciding whether a user-facing action is still allowed, reserving it exactly once for a defined policy window, and making every later delivery attempt inspectable.

## Status

`v0.0.1` is a local, deterministic vertical slice. It includes an authenticated local ingress endpoint, but is not publicly deployed or production-ready.

Implemented today:

- explicit policy evaluation for consent, quiet hours, and per-local-day caps;
- injectable/virtual time for deterministic testing;
- concurrency-safe in-memory reservation uniqueness;
- legal reservation state transitions; and
- Postgres-backed reservation, outbox, token-fenced leases, and attempt state;
- deterministic retry scheduling, provider-status reconciliation, and immutable receipt history; and
- API-key-authenticated reservation ingress using server-owned policy definitions; and
- integration proofs for concurrent planners, lease expiry, retries, and ambiguous provider outcomes.

## Non-goals for this increment

- no real provider integration;
- no public deployment or queue broker;
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

Postgres is the durable source of truth. Start it with `docker compose up -d postgres`, then run the checksum-verified migrator:

```bash
DATABASE_URL='postgres://cairn:cairn_dev_only@localhost:54321/cairn?sslmode=disable' go run ./cmd/cairn-migrate
```

The migrator refuses to guess about a database that has application tables but no recorded migration history. For a pre-migrator local development database, recreate the disposable Compose volume rather than baselining it blindly.

A local worker can claim one job, call the deterministic provider fake, write an attempt receipt, and either succeed, schedule a retry, or move an ambiguous result to reconciliation.

After applying the migration, run the database proof with `CAIRN_TEST_DATABASE_URL=postgres://cairn:cairn_dev_only@localhost:54321/cairn?sslmode=disable go test -tags=integration -race ./...`.
