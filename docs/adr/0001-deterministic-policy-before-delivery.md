# ADR 0001: Evaluate deterministic policy before creating delivery work

## Context

User-facing actions can be harmful when they are stale, unwanted, duplicated, or sent at an inappropriate time. A generated suggestion or a product event is not sufficient authority to deliver an action.

## Decision

Cairn evaluates a versioned deterministic policy before creating a reservation. The initial policy includes consent, local quiet hours, and a per-local-day cap. Every decision includes a machine-readable reason and policy version.

## Alternatives considered

- Let a worker decide immediately before provider delivery: rejected because it leaves no stable planning record and makes duplicate planners difficult to reason about.
- Let an LLM decide eligibility: rejected because authorization must be reproducible and auditable.
- Use only provider-side limits: rejected because providers cannot express product-specific consent and action policy.

## Consequences

The policy surface must remain small and testable. New policy predicates require explicit semantics, tests, and an ADR when they change guarantees.
