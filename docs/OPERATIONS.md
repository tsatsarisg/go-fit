# Operations

Everything you need to run, test, package, and ship go-fit. If you're here to write code, skim [Configuration](#configuration) and [Development](#development); if you're here to deploy, skip to [Deployment](#deployment).

---

## Configuration

All runtime config is read from environment variables. [`.env.example`](../.env.example) is the runnable template; copy it to `.env` for local work.

### Variables

| Var | Default | Required | Notes |
| --- | --- | --- | --- |
| `APP_ENV` | `development` | no | One of `development` \| `production`. Controls log format, JSON pretty-printing, and SSL enforcement. |
| `PORT` | `8080` | no | HTTP listen port. Overridable by `--port` CLI flag. |
| `DATABASE_URL` | (built from `PG*`) | see below | Full Postgres DSN. If set, wins over the discrete `PG*` vars. |
| `PGHOST` | `localhost` | no | Ignored when `DATABASE_URL` is set. |
| `PGPORT` | `5432` | no | |
| `PGUSER` | `postgres` | no | |
| `PGPASSWORD` | `postgres` | no | |
| `PGDATABASE` | `postgres` | no | |
| `PGSSLMODE` | `disable` (dev) / `require` (prod) | no | |

Either `DATABASE_URL` or the `PG*` set must resolve to a reachable Postgres.

### Production guards (`internal/config/config.go`)

When `APP_ENV=production`, the config layer:

1. Appends `sslmode=require` to `DATABASE_URL` if no `sslmode` is present.
2. **Rejects** `sslmode=disable` with an explicit error — no "oops, we deployed with plaintext TLS".
3. Logs are JSON (not text).
4. JSON responses are compact (not indented).

Do not attempt to "relax" these for convenience; production SSL rejection is load-bearing.

### Connection pool

Hardcoded in `internal/platform/postgres/db.go`:

- `SetMaxOpenConns(25)`
- `SetMaxIdleConns(25)`
- `SetConnMaxLifetime(5 * time.Minute)`
- `PingContext` with a 5-second timeout on startup — slow DB fails fast rather than hanging on readiness.

### Secrets

- **Local**: `.env` file, gitignored.
- **Docker dev**: values hardcoded in `docker-compose.yml` (all throwaway).
- **Docker prod-like**: `DATABASE_URL` **must** come from the host env; `docker-compose.prod.yml` uses `:?` to fail fast if it's missing.
- **Real prod**: whatever secret manager your platform provides (AWS Secrets Manager, Vault, Doppler, Fly secrets, etc.). Do not bake values into the image.

---

## Development

### Prerequisites

- Go 1.24.4 (matches `go.mod`)
- Docker + Docker Compose v2 (`docker compose`, not the legacy `docker-compose`)
- `make`

Optional but recommended:

- `golangci-lint` ≥ v1.61.0 for local linting (CI pins this version)
- `air` is installed inside the dev container, not on the host

### Common targets

```bash
make help                       # list every target with its description
make run                        # go run ./cmd/api  (assumes a local pg)
make build                      # bin/api — static binary with -trimpath -ldflags
make test                       # unit tests, race detector, -count=1
make test-integration           # integration tests against test_db on :5433
make lint                       # golangci-lint run ./...
make fmt                        # gofmt -s -w .
make vet                        # go vet ./...
make tidy                       # go mod tidy
make cover                      # coverage.out + coverage.html
make clean                      # wipe bin/, tmp/, coverage.*
```

Docker/compose targets:

```bash
make docker-build               # build local prod image (tag: go-fit:local)
make docker-run                 # run go-fit:local locally (requires DATABASE_URL)
make compose-up                 # dev stack: app + pg16 + test_db (hot reload)
make compose-down               # tear down (preserves named volumes)
make compose-down-volumes       # tear down AND delete pg volumes (destructive)
make compose-logs               # tail last 200 lines, follow
make compose-prod-up            # prod-like stack with distroless image
make compose-prod-down          # tear down prod-like stack
make migrate-up / migrate-down  # goose CLI via `go run` (optional; the app runs
                                # migrations on boot, so these are rarely needed)
```

### Dev stack (`docker-compose.yml`)

| Service | Image | Host port | Role |
| --- | --- | --- | --- |
| `db` | `postgres:16-alpine` | `5432` | Primary dev DB |
| `test_db` | `postgres:16-alpine` | `5433` | Integration test DB |
| `app` | built from `Dockerfile` target `dev` | `8080` | API with air hot-reload |

Source is bind-mounted at `/app`. Saving a `.go` or `.sql` file triggers an air rebuild in ~500ms. Named volumes (`pgdata`, `pgdata_test`, `gomodcache`, `gobuildcache`) persist Postgres data and Go caches across restarts.

### Hot reload

`air` is configured in [`.air.toml`](../.air.toml):

- Build command: `go build -o ./tmp/api ./cmd/api`
- Watches: `.go`, `.sql`, `.tpl`, `.tmpl`, `.html`
- Ignores: `tmp/`, `bin/`, `vendor/`, `database/`, `.git/`, `.github/`
- Ignores test files (`_test.go`)
- `send_interrupt = true` + `kill_delay = "2s"` — graceful shutdown on reload

### Migrations

- SQL files live in `migrations/`, embedded via `go:embed` in `migrations/fs.go`.
- Applied on application startup by `postgres.MigrateFS` (see `internal/app/app.go`).
- Use `goose` conventions: `-- +goose Up` / `-- +goose Down`, `-- +goose StatementBegin` / `-- +goose StatementEnd` around each statement.
- File naming: `NNNNN_description.sql` (5-digit zero-padded).
- To add a migration: create the next numbered file, restart the app, watch it apply on boot. For ad-hoc use, `make migrate-up` runs the goose CLI against the DSN in `$DATABASE_URL`.

### Testing

```bash
make test                       # unit + anything that doesn't need a DB
make test-integration           # uses test_db (localhost:5433)
make cover                      # coverage report
```

For integration tests, `test_db` must be healthy:

```bash
docker compose up -d test_db
make test-integration
```

CI spins up its own Postgres 16 service container, so `go test -race -count=1 ./...` in CI is the authoritative gate.

---

## Build

### Multi-stage `Dockerfile`

Four stages:

1. **`deps`** — `golang:1.24.4-alpine`, `go mod download` with a buildx mount cache.
2. **`builder`** — inherits `deps`, compiles a static binary: `CGO_ENABLED=0`, `-trimpath`, `-ldflags='-s -w -buildid=' + version/commit/build-date injections`, target `./cmd/api`, output `/out/api`.
3. **`dev`** — `golang:1.24.4-alpine` + `air`, used by `docker-compose.yml`.
4. **`final`** — `gcr.io/distroless/static-debian12:nonroot`, copies `/out/api` in, sets `ENV PORT=8080 APP_ENV=production`, runs as `nonroot:nonroot`. No shell, no package manager.

Build args: `GIT_SHA`, `BUILD_DATE`, `VERSION` (surfaced in OCI labels and — if you add `var (version, commit, buildDate string)` to `main.go` — linker-injected at build time).

### Build locally

```bash
make docker-build                           # tag: go-fit:local
docker images go-fit                         # ~20-25 MB image
```

### `.dockerignore`

Keeps the build context lean. Excludes VCS metadata, `bin/`, `tmp/`, compose files, markdown (except `README.md`), and dev dirs. Deliberately **does not** exclude `migrations/` (embedded at compile time) or `go.sum` (needed for reproducibility).

---

## Deployment

### Local prod-like run

```bash
export DATABASE_URL=postgres://user:pass@host:5432/dbname?sslmode=require
make compose-prod-up
```

This builds the `final` distroless image and runs it via `docker-compose.prod.yml` with:

- `APP_ENV=production`
- No source mount, no hot reload
- `read_only: true` rootfs
- `security_opt: no-new-privileges:true`
- `cap_drop: [ALL]`
- CPU/memory limits: 1 CPU, 256 MiB

### Health probes

Distroless has no shell, so there is **no** container-level `HEALTHCHECK`. Probe HTTP `/health` from the orchestrator:

- **Compose locally**: swap the final stage to `gcr.io/distroless/base-debian12:nonroot` (ships `wget`) and add:
  ```yaml
  healthcheck:
    test: ["CMD", "wget", "-qO-", "http://localhost:8080/health"]
  ```
- **Kubernetes**: `livenessProbe` / `readinessProbe` with `httpGet: path: /health, port: 8080`.
- **ECS / Fly / Render**: configure the platform's HTTP health check to `/health`.

### Running behind a load balancer

- Set `APP_ENV=production`.
- `/health` is the cheapest probe endpoint (un-logged, no DB round trip).
- Graceful shutdown budget is 10s — give the LB at least that long between SIGTERM and SIGKILL (`terminationGracePeriodSeconds: 30` on k8s is a safe default).
- The server has `IdleTimeout: 60s`, `ReadTimeout: 10s`, `WriteTimeout: 30s` — LB-side timeouts should respect these.

---

## CI/CD

### CI — `.github/workflows/ci.yml`

Triggers: `push` to `main`, all `pull_request`. Jobs run in parallel where possible.

| Job | What it does |
| --- | --- |
| `lint` | `golangci-lint` pinned to `v1.61.0`, config in `.golangci.yml` |
| `test` | Spins up Postgres 16 as a service container, runs `go test -race -count=1 -covermode=atomic -coverprofile`, uploads coverage artifact |
| `build` | `go build ./...` smoke + `docker buildx build --target final` (no push) — catches Dockerfile regressions |
| `govulncheck` | Runs `govulncheck ./...` against the module graph |

Additional: `.github/workflows/codeql.yml` runs on push, PR, and weekly cron for language-level static analysis.

Go version is read from `go.mod` via `actions/setup-go`'s `go-version-file` input — no drift between CI and the module.

### Release — `.github/workflows/release.yml`

Trigger: `push` of a tag matching `v*.*.*`.

Steps:

1. `docker/setup-qemu-action` + `setup-buildx-action` for multi-arch.
2. `docker/login-action` to GHCR using `GITHUB_TOKEN`.
3. `docker/metadata-action` produces tags: `vX.Y.Z`, `X.Y`, `latest` (on default branch), `sha-<short>`.
4. `docker/build-push-action` builds `linux/amd64,linux/arm64`, pushes, attaches SBOM + SLSA provenance.
5. `sigstore/cosign-installer` + `cosign sign --yes` signs each tag keyless via OIDC.

To cut a release:

```bash
git tag -a v0.1.0 -m "v0.1.0"
git push origin v0.1.0
```

Image ends up at `ghcr.io/tsatsarisg/go-fit:v0.1.0` (+ the extra tags). Pull + verify:

```bash
docker pull ghcr.io/tsatsarisg/go-fit:v0.1.0
cosign verify \
  --certificate-identity-regexp 'https://github.com/tsatsarisg/go-fit/.github/workflows/release.yml' \
  --certificate-oidc-issuer 'https://token.actions.githubusercontent.com' \
  ghcr.io/tsatsarisg/go-fit:v0.1.0
```

---

## Troubleshooting

| Symptom | Likely cause |
| --- | --- |
| `sslmode=disable is not allowed when APP_ENV=production` | You're starting the binary with `APP_ENV=production` but a dev DSN. Fix the DSN or unset `APP_ENV`. |
| `failed to ping database: ...` on startup | DB unreachable or creds wrong. Check `DATABASE_URL`, network, pg is up. 5-second ping timeout is deliberate. |
| `500` with `"internal server error"` body | Logged via `slog.ErrorContext` with the request id. Grep server logs for the id. |
| Login always returns `401` | Expected behaviour for both "unknown user" and "wrong password" — the response is identical by design. Check the password length (min 12). |
| `go-fit:local` image is huge | You built the `dev` or `builder` target. `make docker-build` targets `final` (distroless); use that. |
| Hot reload isn't firing | Check `.air.toml` include/exclude globs; ensure you edited a watched extension and not under an `exclude_dir`. |
| Migrations stuck / dirty | `goose` keeps state in `goose_db_version`. Inspect/fix there before rerunning. For dev, `make compose-down-volumes` is the sledgehammer. |

---

## Planned improvements (out of scope right now)

- **OpenTelemetry**: `otelhttp` middleware + OTLP exporter behind `OTEL_EXPORTER_OTLP_ENDPOINT`.
- **Image scanning**: Trivy or Grype as a release-gate step.
- **Dependabot**: enable for both Go modules and GitHub Actions.
- **Preview environments per PR**: Neon branch + Fly/Render app per PR, torn down on merge.
- **Branch protection**: require `lint`, `test`, `build`, `govulncheck`, `codeql` checks before merging to `main` (repo settings change).
- **Deploy workflow**: `deploy.yml` that promotes a signed image to a real target after `release.yml` succeeds.
