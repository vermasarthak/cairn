# Cairn architecture

## Problem boundary

A product event is not authority to contact a user. Cairn separates candidate selection from permission, reservation, execution, and recording.

```text
event -> candidate -> evaluate policy -> reserve -> durable job -> claim lease
      -> revalidate -> provider attempt -> receipt/reconcile -> audit record
```

The current repository implements policy evaluation, atomic reservation-to-job creation, lease-based claiming, retry scheduling, and an isolated reconciliation path using a deterministic provider fake. Public ingress, real provider adapters, and observability remain future work.

## Core invariants

1. A disallowed decision cannot create a reservation.
2. A reservation key is unique for `(tenant, subject, policy, action, local-day)`.
3. A policy decision records its policy version and evaluation instant.
4. State transitions are explicit; an already delivered action cannot return to a runnable state.
5. A claimed or reconciling worker must present its unique lease token to transition the job; a stale worker cannot commit after its lease has been reclaimed.
5. Local-day caps use the subject's declared IANA time zone, never the worker's time zone.

## Planned persistence model

Postgres holds reservations, jobs, attempts, reconciliation lookups, provider receipt placeholders, and immutable audit events. A transactional outbox bridges the reservation commit and worker scheduling boundary. An embedded migration runner serializes schema changes with a Postgres advisory lock and stores a checksum for each applied migration.

The system will not claim exactly-once delivery. A retryable pre-acceptance failure is retried with the same provider idempotency key. An ambiguous timeout enters dedicated reconciliation and cannot be resent unless the provider explicitly reports the action absent.

## Planned concurrency model

Planning creates a reservation in the same transaction that enforces the unique reservation key. Senders and reconcilers use separate time-bounded, token-fenced leases; an expired lease can be claimed again only by its own execution class, and the previous owner can no longer transition it. Every provider attempt and lookup has an independent, immutable record.

## Security boundary

Policy evaluation is deterministic. Future model output may create an untrusted candidate, but can never bypass consent, quiet hours, caps, authorization, or kill switches.
