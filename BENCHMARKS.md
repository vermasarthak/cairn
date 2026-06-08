# Benchmarks

## Method

`BenchmarkReserveAndEnqueueParallel` exercises the Postgres transaction that writes a reservation, queued job, outbox event, and audit event. It uses unique subjects/actions so the measurement is throughput rather than duplicate-key handling.

Run it against a disposable, migrated Postgres database:

```bash
CAIRN_BENCH_DATABASE_URL='postgres://cairn:cairn_dev_only@localhost:54321/cairn_bench?sslmode=disable' \
  go test -tags=integration -bench=BenchmarkReserveAndEnqueueParallel -benchtime=1s -count=1 ./benchmarks
```

Results are environment-specific and are recorded below only after a measured run.

## Latest measured result

Run on 2026-09-19 on an Apple M1 (arm64), Go 1.27.1, and the repository's local Docker Postgres 17 configuration:

```text
BenchmarkReserveAndEnqueueParallel-8    2107    583148 ns/op
```

That is approximately 2,107 completed atomic transactions in the one-second benchmark window. This is a local development baseline, not a latency SLO, cloud comparison, or capacity claim; it does not report P95/P99 latency or include a real broker/provider.
