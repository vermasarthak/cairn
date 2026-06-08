# Cairn

**A durable, policy-aware delivery runtime for user-facing actions.**

Cairn is being built for a narrow, difficult problem: deciding whether a user-facing action is still allowed, reserving it exactly once for a defined policy window, and making every later delivery attempt inspectable.

## Status

`v0.1.0` is a local, deterministic delivery-runtime reference. It includes authenticated ingress, durable state, retries, reconciliation, and operational endpoints; it is not a hosted service.

Implemented today:

- explicit policy evaluation for consent, quiet hours, and per-local-day caps;
- injectable/virtual time for deterministic testing;
- concurrency-safe in-memory reservation uniqueness;
- legal reservation state transitions; and
- Postgres-backed reservation, outbox, token-fenced leases, and attempt state;
- deterministic retry scheduling, provider-status reconciliation, and immutable receipt history; and
- API-key-authenticated reservation ingress using server-owned policy definitions; and
- a leased transactional outbox dispatcher plus health, readiness, metrics, and JSON logs.
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

The local API exposes `/healthz`, database-backed `/readyz`, and `/metrics`. Logs are structured JSON and intentionally omit API keys, request bodies, and subject IDs.

## Local demo

In separate terminals, after starting Postgres and running migrations:

```bash
DATABASE_URL='postgres://cairn:cairn_dev_only@localhost:54321/cairn?sslmode=disable' go run ./cmd/cairn-seed-demo
DATABASE_URL='postgres://cairn:cairn_dev_only@localhost:54321/cairn?sslmode=disable' CAIRN_API_KEYS='demo=demo-local-only-secret' go run ./cmd/cairn-api
```

Then create a policy-governed action:

```bash
curl -i http://localhost:8080/v1/reservations \
  -H 'Content-Type: application/json' \
  -H 'X-Cairn-API-Key: demo-local-only-secret' \
  --data '{"policy_id":"daily-check-in","subject":{"id":"user-1","time_zone":"UTC","consented":true},"action":{"key":"check-in"}}'
```

See [ARCHITECTURE.md](ARCHITECTURE.md), [BENCHMARKS.md](BENCHMARKS.md), [SECURITY.md](SECURITY.md), and [LIMITATIONS.md](LIMITATIONS.md) before deploying any adaptation.

After applying the migration, run the database proof with `CAIRN_TEST_DATABASE_URL=postgres://cairn:cairn_dev_only@localhost:54321/cairn?sslmode=disable go test -tags=integration -race ./...`.
