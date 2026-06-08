# Security

## Boundary

Cairn accepts tenant-scoped API keys via `X-Cairn-API-Key`. The server maps the key to a tenant and ignores any tenant identity supplied by the caller. Policy definitions are loaded from the server-owned policy store; event callers cannot provide a replacement policy.

## Deployment requirements

- Store `DATABASE_URL` and `CAIRN_API_KEYS` in a secret manager, never source control or images.
- Terminate TLS before the API and restrict `/metrics` to trusted operators.
- Rotate API keys by deploying an overlap set, then removing old keys.
- Run database migrations as a separately audited release step.
- Use a distinct database role with only Cairn's required schema privileges.

## Threats handled

- Tenant spoofing: key-to-tenant mapping is server controlled.
- Duplicate ingress: the reservation key and transaction prevent duplicate jobs for a policy window.
- Stale workers: lease tokens fence old owners from writing outcomes.
- Ambiguous provider delivery: unknown outcomes enter reconciliation instead of automatic resend.

## Known limits

API keys are intentionally the small v1 authentication mechanism; there is no key hashing, per-key role model, rate limiter, OAuth flow, or external secret manager integration in this repository. Do not expose it publicly until those choices are appropriate for the deployment.
