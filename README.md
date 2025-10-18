# go-fit

A small Go HTTP service for managing users and workouts. The project provides a simple REST API backed by PostgreSQL, embedded SQL migrations, and a lightweight application initializer.

## Overview

- Module: `github.com/tsatsarisg/go-fit`
- Go version: set in `go.mod` (recommended Go 1.24.x)

This repository includes handlers for users and workouts, a Postgres-backed store layer, and SQL migrations under the `migrations/` directory. The HTTP router is implemented with `chi` and routes are registered in `internal/routes`.

## Quick start (recommended)

This project includes a `docker-compose.yml` that brings up a Postgres database used by the app. Use the compose setup for local development:

```bash
# Start Postgres in docker-compose (creates a local DB for the app)
docker-compose up -d

# Build and run the app locally
go run main.go

# Or build a binary
go build -o bin/server .
./bin/server
```

By default the application reads its Postgres connection configuration from environment variables expected by the store implementation. See `store/database.go` for the exact env var names and defaults.

If you prefer to run entirely without Docker, ensure you have a running Postgres instance and the correct environment variables set before running the app.

## Database & migrations

- SQL migration files live in `migrations/` and are applied on application startup using the embedded filesystem.
- Persistent Postgres data for local development live under `database/postgres-data/` (used by the project's compose file).
- A small test dataset (for manual inspection) is available in `database/postgres-test-data/`.

To re-run migrations against a local Postgres instance, either use the compose setup or run the migrations logic in `internal/app.NewApplication()` by starting the app.

## HTTP API

The project exposes the following routes (registered in `internal/routes/routes.go`):

- GET /health

  - Health check. Returns 200 OK with body `OK`.

- Workouts

  - GET /workouts/{id} — retrieve a workout by id
  - POST /workouts — create a new workout (JSON body)
  - PUT /workouts/{id} — update an existing workout (partial fields supported)
  - DELETE /workouts/{id} — delete a workout

- Users
  - POST /users — register a new user (JSON body)

Examples:

```bash
# Health
curl -i http://localhost:8080/health

# Create a user
curl -X POST http://localhost:8080/users \
    -H "Content-Type: application/json" \
    -d '{"username":"alice","email":"alice@example.com","password":"s3cret"}'

# Create a workout
curl -X POST http://localhost:8080/workouts \
    -H "Content-Type: application/json" \
    -d '{"title":"Morning Run","duration_minutes":30}'
```

Responses use a JSON envelope helper defined in `internal/utils` (see `utils.WriteJson`), typically returning `{"error": "..."}` on failures or `{"workout": ...}` / `{"user": ...}` on success.

## Running tests

There are some unit tests in the `store` package (look for `_test.go` files). Run all tests with:

```bash
go test ./...
```

If tests depend on a database, prefer using a test database or the project's test data volume under `database/postgres-test-data/`.

## Development notes

- Application initialization is in `internal/app/app.go`. It opens the DB, runs migrations, creates stores and handlers, and returns an `Application` struct.
- Routes are wired in `internal/routes/routes.go` and use handlers in `internal/api`.
- Store implementations (Postgres) are in `internal/store`.
- Utility helpers (JSON envelope, id parsing) are in `internal/utils`.

Suggested improvements / next steps:

- Add more tests (handlers + store integration tests using a test container or docker-compose).
- Add graceful shutdown handling in `main.go` (if not already present).
- Add clearer configuration handling (e.g., a config struct and environment parsing library).

## Contributing

1. Fork the repository.
2. Create a feature branch: `git checkout -b feat/your-feature`.
3. Implement and test your changes.
4. Open a pull request with a description of the changes.

## License

See the `LICENSE` file in the repository root for licensing details.

If you'd like the README to include additional details (example request/response bodies, required env vars, or a Postgres connection example), tell me which sections you'd like expanded and I will add them.
