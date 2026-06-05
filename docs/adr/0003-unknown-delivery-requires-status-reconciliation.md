# ADR 0003: Unknown delivery requires status reconciliation

## Context

A network timeout after sending a provider request cannot prove whether the provider accepted it. Retrying as ordinary work can duplicate a user-facing action.

## Decision

An `unknown` provider outcome moves a job to `reconcile`. A dedicated reconciler leases it separately, records every status lookup, and only returns it to `queued` when the provider explicitly reports the action absent. Confirmed delivery succeeds; an unknown status remains in reconciliation.

## Consequences

This sacrifices automatic progress during a provider outage for a safer duplicate-delivery posture. Providers without a reliable status lookup require a manual, auditable resolution path in a future release.
