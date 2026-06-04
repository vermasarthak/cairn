# ADR 0002: Reservation is not delivery

## Context

A successful reservation prevents competing planners from choosing the same action. It does not prove a provider received, accepted, or displayed the action.

## Decision

Cairn models reservation, queueing, claiming, provider attempt, delivery receipt, suppression, and failure as separate states. The eventual API will report an unknown outcome when a timeout occurs after a provider may have accepted the request.

## Consequences

The system makes narrower, honest guarantees. Provider idempotency keys and reconciliation become first-class requirements instead of an implementation detail.
