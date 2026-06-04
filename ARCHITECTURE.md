# Cairn architecture

## Problem boundary

A product event is not authority to contact a user. Cairn separates candidate selection from permission, reservation, execution, and recording.

```text
event -> candidate -> evaluate policy -> reserve -> durable job -> claim lease
      -> revalidate -> provider attempt -> receipt/reconcile -> audit record
```

The current repository implements only the first four deterministic concepts. Postgres-backed durability, jobs, workers, provider adapters, and observability will be added in that order.

## Core invariants

1. A disallowed decision cannot create a reservation.
2. A reservation key is unique for `(tenant, subject, policy, action, local-day)`.
3. A policy decision records its policy version and evaluation instant.
4. State transitions are explicit; an already delivered action cannot return to a runnable state.
5. Local-day caps use the subject's declared IANA time zone, never the worker's time zone.

## Planned persistence model

Postgres will hold canonical events, policy versions, candidates, reservations, jobs, attempts, provider receipts, and immutable audit events. A transactional outbox will bridge the database commit and worker scheduling boundary.

The system will not claim exactly-once delivery. A worker may retry after an ambiguous provider timeout. Provider idempotency keys and reconciliation determine whether that retry can be made effectively once.

## Planned concurrency model

Planning creates a reservation in the same transaction that enforces the unique reservation key. Worker claiming will use time-bounded leases. A reaper will make expired leases eligible again. Every provider attempt will have an independent, immutable record.

## Security boundary

Policy evaluation is deterministic. Future model output may create an untrusted candidate, but can never bypass consent, quiet hours, caps, authorization, or kill switches.
