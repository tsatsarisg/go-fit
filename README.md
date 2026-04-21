# go-fit

A small, opinionated Go HTTP service for managing users and workouts. Built as a portfolio / reference project that tries to show what a pragmatic, production-shaped Go backend looks like: bounded contexts, ports-and-adapters, typed IDs, a real auth model, graceful shutdown, embedded migrations, structured logging, a multi-stage container build, and CI that actually gates merges.

- **Module:** `github.com/tsatsarisg/go-fit`
- **Go:** 1.24.4 (pinned in `go.mod`)
- **Entrypoint:** `cmd/api/main.go`
- **Database:** PostgreSQL 16

---

## Quickstart

The whole stack — app with hot reload + Postgres 16 — comes up with one command:

```bash
make compose-up
```

Then:

```bash
curl -i http://localhost:8080/health
# HTTP/1.1 200 OK
# OK
```

Tear it down:

```bash
make compose-down               # preserve data
make compose-down-volumes       # nuke the pg volume too
```

If you prefer running the binary directly against a local Postgres:

```bash
cp .env.example .env            # tweak if needed
make run                        # equivalent to: go run ./cmd/api
```

See [`docs/OPERATIONS.md`](docs/OPERATIONS.md) for the full dev/prod workflow.

---

## What's in here

```
cmd/api/              HTTP server entrypoint (main package)
internal/
  app/                Wires config → stores → services → handlers → router
  config/             Env-driven config (APP_ENV, PORT, DATABASE_URL, PG*)
  auth/               Bounded context: tokens, middleware, login/logout
  user/               Bounded context: registration, password hashing
  workout/            Bounded context: workout aggregate + entries
  httpx/              Transport plumbing: JSON envelope, decode, error mapping, logger
  platform/postgres/  DB open/ping/migrate + pgx error classification
migrations/           Embedded goose SQL migrations (go:embed)
docs/                 Architecture, API, configuration, operations, contributing
```

Architecture is bounded-context + ports-and-adapters. Each feature package owns its domain model, service (application layer), store port (interface), and postgres adapter. Handlers are thin — they decode, call the service, and format the response. See [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md).

---

## API at a glance

| Method | Path | Auth | Purpose |
| --- | --- | --- | --- |
| GET | `/health` | no | Liveness probe — returns `200 OK` |
| POST | `/users` | no | Register a new user |
| POST | `/tokens/authentication` | no | Log in, receive a bearer token |
| POST | `/tokens/authentication/logout` | yes | Revoke all tokens for the caller |
| GET | `/workouts/{id}` | yes | Fetch a workout the caller owns |
| POST | `/workouts` | yes | Create a workout |
| PATCH | `/workouts/{id}` | yes | Partial-update (RFC 5789 merge patch) |
| DELETE | `/workouts/{id}` | yes | Delete a workout the caller owns |

Full request/response examples, error shapes, and status code semantics live in [`docs/API.md`](docs/API.md).

---

## Configuration

All runtime config is read from the environment. See [`.env.example`](.env.example) for a runnable template and [`docs/OPERATIONS.md`](docs/OPERATIONS.md#configuration) for the full table and production notes (SSL enforcement, pool sizing, secrets handling).

The two you almost always care about:

| Var | Default | Notes |
| --- | --- | --- |
| `APP_ENV` | `development` | `production` enables JSON-compact responses and enforces `sslmode=require` |
| `DATABASE_URL` | discrete `PG*` fallback | Wins over `PGHOST`/etc. when set |

---

## Development loop

```bash
make help                       # list every target
make run                        # go run ./cmd/api
make test                       # go test -race -count=1 ./...
make lint                       # golangci-lint run
make fmt vet tidy               # standard go hygiene
make cover                      # coverage.html
make compose-up                 # full dev stack with hot reload
```

The dev container uses [air](https://github.com/air-verse/air) for hot reload; source is bind-mounted, so saves trigger a rebuild in ~500ms.

More detail, including the integration-test flow that hits the `test_db` service on port 5433, is in [`docs/OPERATIONS.md`](docs/OPERATIONS.md#development).

---

## Tests

```bash
make test                       # unit tests with race detector
make test-integration           # runs against test_db (localhost:5433)
```

Integration tests expect `test_db` from `docker-compose.yml` to be healthy; CI spins up a Postgres 16 service container for the same purpose.

---

## Build & deploy

The production image is a two-stage build that ends in `gcr.io/distroless/static-debian12:nonroot` — no shell, no package manager, no libc, runs as UID 65532.

```bash
make docker-build               # build the final image locally
DATABASE_URL=postgres://... make compose-prod-up   # run the prod image locally
```

Release is tag-driven: push a `vX.Y.Z` tag and GitHub Actions builds a multi-arch (`linux/amd64`, `linux/arm64`) image, publishes to GHCR, attaches SBOM + provenance, and signs it with cosign (keyless OIDC).

Full pipeline details, image layer breakdown, and deployment notes are in [`docs/OPERATIONS.md`](docs/OPERATIONS.md#deployment).

---

## Docs

- [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) — why the code is shaped like this (bounded contexts, layering, dependency rules, common patterns)
- [`docs/API.md`](docs/API.md) — full HTTP reference with request/response examples and error taxonomy
- [`docs/OPERATIONS.md`](docs/OPERATIONS.md) — configuration, local dev, testing, Docker, CI/CD, deployment
- [`docs/CONTRIBUTING.md`](docs/CONTRIBUTING.md) — commit style, branching, PR expectations

---

## License

MIT — see [`LICENSE`](LICENSE).
