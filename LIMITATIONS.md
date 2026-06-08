# Limitations

- The provider is a deterministic fake. Cairn does not yet include a real email, push, or SMS adapter.
- Policy evaluation currently receives no historical delivery feed, so per-local-day caps beyond the reservation window require a durable delivery-history query before production use.
- The outbox publisher is an in-process interface; a real broker adapter and consumer deduplication store are deployment work.
- API keys live in process configuration and lack rate limiting or an administrative control plane.
- Metrics are process-local counters. Multi-instance aggregation requires a Prometheus scraper or equivalent.
- No deployment is operated by this repository. Docker packaging is a reproducible artifact, not evidence of a live service.
