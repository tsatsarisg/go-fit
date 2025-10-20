# go-fit

A small Go HTTP service for managing users and workouts. The project provides a simple REST API backed by PostgreSQL, embedded SQL migrations, and a lightweight application initializer.

## Overview

- Module: `github.com/tsatsarisg/go-fit`
- Go version: set in `go.mod` (this repo uses Go 1.24.x)

This repository includes handlers for users and workouts, a Postgres-backed store layer, and SQL migrations embedded under the `migrations/` package. The HTTP router is implemented with `chi` and routes are registered in `internal/routes`.

## Quick start (recommended)

The easiest way to run the app for local development is with Docker Compose. The compose file starts a Postgres instance and mounts a local data directory.

```bash
# Start Postgres in docker-compose (creates a local DB for the app)
docker-compose up -d

# Build and run the app (defaults to port 8080)
go run main.go

# Or build a binary
go build -o bin/server .
./bin/server
```

By default the application connects to Postgres using the connection string in `internal/store.Open()`:

host=localhost port=5432 user=postgres password=postgres dbname=postgres sslmode=disable

If you run the database via Docker Compose the project already exposes Postgres on localhost:5432 and uses a persistent volume at `./database/postgres-data`.

If you prefer to run without Docker, ensure you have a running Postgres instance and either update `internal/store.Open()` or set the appropriate environment/connection values before running the app.

## Database & migrations

- SQL migration files live in `migrations/` and are embedded in the binary. Migrations are applied on application startup by `internal/app.NewApplication()` using `pressly/goose` and the embedded FS.
- Persistent Postgres data for local development live under `database/postgres-data/` (used by the project's compose file).

To re-run migrations against a local Postgres instance, start the app (it will run the migrations on startup) or call `store.Migrate` with an appropriate DB handle.

## HTTP API

The project exposes the following routes (registered in `internal/routes/routes.go`):

- GET /health

  - Health check. Returns 200 OK with body `OK`.

- Workouts (authenticated)

  - GET /workouts/{id} — retrieve a workout by id
  - POST /workouts — create a new workout (JSON body)
  - PUT /workouts/{id} — update an existing workout
  - DELETE /workouts/{id} — delete a workout

- Users

  - POST /users — register a new user (JSON body)

- Tokens
  - POST /tokens/authentication — create an authentication token (used to obtain JWTs / session tokens)

Note: workout endpoints are registered inside an authenticated group and require a valid token. Authentication and middleware are implemented in `internal/middleware` and `internal/api/token_handler.go`.

Examples:

```bash
# Health
curl -i http://localhost:8080/health

# Create a user
curl -X POST http://localhost:8080/users \
    -H "Content-Type: application/json" \
    -d '{"username":"alice","email":"alice@example.com","password":"s3cret"}'

# Create a token (authentication)
curl -X POST http://localhost:8080/tokens/authentication \
    -H "Content-Type: application/json" \
    -d '{"username":"alice","password":"s3cret"}'

# Create a workout (authenticated — replace <TOKEN> with the token returned above)
curl -X POST http://localhost:8080/workouts \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer <TOKEN>" \
    -d '{"title":"Morning Run","duration_minutes":30}'
```

Responses use a JSON envelope helper defined in `internal/utils` (see `utils.WriteJson`), typically returning `{"error": "..."}` on failures or `{"workout": ...}` / `{"user": ...}` on success.

## Running tests

There are unit tests in the `store` package and elsewhere (look for `_test.go` files). Run all tests with:

```bash
go test ./...
```

If tests depend on a database, prefer using the `test_db` service from `docker-compose.yml` (it mounts `database/postgres-test-data`) or run a separate test Postgres instance.

## Development notes

- Application initialization is in `internal/app/app.go`. It opens the DB (using `internal/store.Open()`), runs migrations, creates stores and handlers, and returns an `Application` struct.
- Routes are wired in `internal/routes/routes.go` and use handlers in `internal/api`.
- Store implementations (Postgres) are in `internal/store`.
- Utility helpers (JSON envelope, id parsing) are in `internal/utils`.

Suggested improvements / next steps:

- Add more tests (handlers + store integration tests using a test container or docker-compose).
- Add graceful shutdown handling in `main.go` (the current implementation starts the server and logs fatal on error).
- Improve configuration handling (e.g., a config struct, environment parsing, or use of `github.com/joho/godotenv`).

## License

See the `LICENSE` file in the repository root for licensing details.

If you'd like the README to include additional details (example request/response bodies, required env vars, or a Postgres connection example), tell me which sections you'd like expanded and I will add them.
