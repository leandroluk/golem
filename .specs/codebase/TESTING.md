# Testing

**Stack:** Go 1.25 standard `testing` package. No test framework/assertion library added (matches zero-dependency philosophy — `t.Fatalf`/`t.Errorf` are enough).

**Test execution tooling:** a root `Makefile` (created in M1/T1) wraps every command below, so the user doesn't need to remember raw `go`/`docker` invocations — mirrors `package.json` scripts.

---

## Test Coverage Matrix

| Code Layer | Test Type | Parallel-Safe | Notes |
| --- | --- | --- | --- |
| `golem` root package (`DataSource`, `Conn`, `Dialect`/`Connector` interfaces, `ColumnType` stub, `Logger`/`LogLevel`) | unit | Yes | Pure Go, no I/O. Connector/Dialect are faked in-package for `DataSource` lifecycle tests (idempotent `Connect`/`Close`, error propagation) |
| `adapter/postgres` — `Options` + `resolveDSN` (DSN/discrete-field precedence) | unit | Yes | Pure function, table-driven, no network |
| `adapter/postgres` — `dialect` (Bind/Scan stub) | unit | Yes | Pure, no network — just verifies contract shape + "unrecognized type" error path |
| `adapter/postgres` — `connector` (real `pgxpool` Connect/Close) + `New` wiring | integration | No | Requires a live (dockerized) Postgres; shares one container/DSN per test run, so not run in parallel with itself |

**Rule of thumb for this repo:** anything that opens a real network connection is `integration`; anything else is `unit`.

---

## Gate Check Commands

| Gate | Command | When to use |
| --- | --- | --- |
| `quick` | `make gate-quick` → `go build ./... && go vet ./... && go test ./... -short` | Every task except the one that adds real Postgres I/O — fast, no Docker needed |
| `full` | `make gate-full` → `make gate-quick && make test-integration` | Tasks that touch `adapter/postgres`'s real connector; runs Postgres via Docker Compose |

**Makefile targets (created in M1/T1):**

```makefile
build:              go build ./...
vet:                go vet ./...
test:               go test ./... -short                 # unit only, -short skips integration-tagged tests
test-integration:   docker compose -f .docker/docker-compose.test.yml up -d --wait
                    go test -tags=integration ./... ; docker compose -f .docker/docker-compose.test.yml down
gate-quick:         build vet test
gate-full:          gate-quick test-integration
```

Integration tests live behind the `integration` build tag (`//go:build integration`) so `go test ./... -short` (the `quick` gate) never needs Docker. The DSN the integration tests connect to comes from `GOLEM_TEST_DSN` (env var), defaulting to the `.docker/docker-compose.test.yml` service's connection string when unset.

---

## Parallelism Assessment

| Test Type | Parallel-Safe | Reason |
| --- | --- | --- |
| unit | Yes | No shared state, no I/O, no fixed ports |
| integration | No | All integration tests in a milestone share one Postgres container/DSN — running them concurrently risks port/schema contention. Run integration-tagged tests sequentially within a single `go test -tags=integration ./...` invocation (Go itself parallelizes sub-tests only if a test opts into `t.Parallel()`, which we don't use here) |

Tasks whose `Tests` field is `integration` MUST NOT be marked `[P]` relative to other integration tasks. Unit-only tasks may be marked `[P]` freely as long as they don't touch the same file.
