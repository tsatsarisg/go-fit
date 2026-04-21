# Architecture

This document describes the shape of the code: how packages are organised, what depends on what, and the reasoning behind the non-obvious choices. If you're reading the code for the first time, start here.

## Design goals

1. **Bounded contexts over layered monolith.** Each feature (`user`, `workout`, `auth`) owns its model, service, store port, and adapter. A feature package is a vertical slice, not a horizontal layer.
2. **Ports and adapters.** Services depend on `Store` interfaces defined in the same package as the service (consumer-side interfaces). The postgres implementation satisfies them. Swapping to an in-memory fake or a different DB is a local change.
3. **Handlers stay thin.** Transport code decodes, delegates to a service, formats the response. All orchestration, validation, and invariants live in the service or domain layer.
4. **Typed IDs.** `UserID` and `WorkoutID` are named `int64` types so the compiler refuses to accept one where the other is expected. Catches a whole class of id-mix-up bugs at compile time.
5. **Security-first defaults.** Password hashes are opaque values, not strings. Tokens are stored as SHA-256 hashes. Login has constant-time comparison against a dummy hash when the user is missing (no user-enumeration side channel). Production refuses `sslmode=disable`.

## Package layout

```
cmd/api/                  Binary entrypoint: parses flags, loads config, runs app.
internal/app/             Composition root. Builds the dependency graph in one place.
internal/config/          Env loading + production guards (SSL enforcement).
internal/auth/            Bounded context: tokens, middleware, login/logout service.
internal/user/            Bounded context: user aggregate, registration, hasher port.
internal/workout/         Bounded context: workout aggregate, entries, CRUD service.
internal/httpx/           Shared transport plumbing (JSON envelope, decode, error mapping, logger, middleware).
internal/platform/postgres/  DB lifecycle + pgx error classification (ErrDuplicate, ErrConstraintViolation).
migrations/               Embedded SQL migrations (go:embed FS).
```

### Why `internal/`?

Everything non-entrypoint is under `internal/` so no external module can import it. This is a deliberate API surface of zero — the only public contract is the HTTP one.

### Why a separate `httpx` package?

The JSON envelope, decode helpers, store-error mapping, and request logger are used by every bounded context. Putting them in a feature package would invert the dependency. `httpx` has no bounded-context imports (other than `platform/postgres` for the `ErrDuplicate`/`ErrConstraintViolation` sentinels).

## Dependency direction

```
   cmd/api
      |
      v
   app  (composition root — allowed to import everything)
    |------------------------.
    v                        v
  auth --> user           workout --> user
    \      /                 \       /
     v    v                   v     v
      httpx <-----------------.
        |
        v
      platform/postgres
```

Rules:

- Feature packages (`user`, `workout`, `auth`) may depend on `httpx` and `platform/postgres`.
- `workout` and `auth` depend on `user` for `user.UserID` (the shared identity type). `user` must not depend back.
- No feature package imports another feature's handler or store; cross-context orchestration lives in services that take narrow collaborators (e.g. `auth.Service` takes `*user.Service`).
- Nothing under `internal/` imports `cmd/` or `app/`.

## Layers inside a feature package

Each feature package follows the same shape. Example: `internal/workout/`:

| File | Role |
| --- | --- |
| `model.go` | Domain types, invariants, `Validate()` methods. No IO. |
| `service.go` | Application service + commands + `Store` port + sentinel errors. |
| `handler.go` | HTTP handler — decode, call service, format response. No domain logic. |
| `postgres_store.go` | Adapter implementing `Store` against `*sql.DB`. |
| `*_test.go` | Package-local tests (unit + store integration). |

The `Store` interface is defined **in the same file as the service** (consumer-side), not in the adapter. The service decides what it needs; the adapter conforms.

## Request flow

A typical authenticated request flows like this:

```
HTTP request
  │
  ├─ chi Router
  │     RequestID middleware  (attaches request_id)
  │     httpx.RequestLogger   (structured slog line per request)
  │     auth.Authenticate     (resolves bearer token → Principal or AnonymousPrincipal)
  │
  ├─ auth.RequireAuthenticatedUser  (rejects Anonymous with 401)
  │
  ├─ workout.Handler.HandleUpdateWorkout
  │     ├─ httpx.ReadIdParam         (parse :id to int64 → WorkoutID)
  │     ├─ auth.GetPrincipal          (extract UserID from context)
  │     ├─ httpx.DecodeJSONBody       (unknown fields rejected, size-limited)
  │     ├─ workout.Service.Update
  │     │     ├─ WorkoutPatch.Validate()
  │     │     └─ Store.UpdateWorkout(ctx, id, userID, patch)
  │     │           └─ single UPDATE ... COALESCE ... WHERE id AND user_id RETURNING
  │     │                   (ownership enforced in SQL, not in Go)
  │     │
  │     └─ httpx.WriteJson(200, {"workout": ...})
  │                 OR
  │        httpx.WriteStoreError → maps domain/pg sentinels to HTTP status
  │
  └─ HTTP response
```

## Error taxonomy

Sentinel errors live in the feature package that owns them. `httpx.WriteStoreError` centralises sentinel → HTTP mapping.

| Sentinel | HTTP | Source |
| --- | --- | --- |
| `workout.ErrValidation`, `user.ErrValidation` | 400 | Aggregate/patch invariant violated |
| `workout.ErrNotFound` | 404 | Workout id doesn't exist |
| `workout.ErrForbidden` | 403 | Row exists but belongs to another user |
| `auth.ErrInvalidCredentials` | 401 | Login failed (wrong username *or* password — identical response by design) |
| `postgres.ErrDuplicate` | 409 | Unique-constraint violation (`23505`) |
| `postgres.ErrConstraintViolation` | 400 | Check-constraint violation (`23514`) |
| anything else | 500 | Logged via `slog.ErrorContext` with request_id |

Response bodies are intentionally generic (`"resource already exists"`, not `"email already taken"`) so registration can't be used as an enumeration oracle.

## Security choices worth knowing

### Login timing

`user.VerifyPassword` runs bcrypt against a package-level dummy hash when the user isn't found, so the response timing for "unknown username" matches "wrong password". Skipping bcrypt on the not-found path would leak which usernames exist. Do not "optimise" this.

### Token storage

Tokens are generated as 32 random bytes (base32-encoded for the plaintext) and stored as a SHA-256 hash. The plaintext is only returned to the client once, at login. DB compromise yields hashes, not usable bearer credentials.

### Ownership in SQL

`UpdateWorkout` and `DeleteWorkout` enforce ownership in the `WHERE` clause, in a single statement. The prior Go-side check had a TOCTOU window between "fetch to check owner" and "apply change". The single-statement form closes it.

### Password policy

Minimum 12 characters. Short enough to not be user-hostile, long enough to resist casual offline brute-forcing of a leaked hash.

### Context propagation

Every DB call uses the `*Context` variants of `database/sql`. Cancellation from the HTTP request propagates through the stack; a dropped client doesn't keep a DB transaction open.

## Observability

- **Structured logs**: `slog` with a production JSON handler and a development text handler (selected via `APP_ENV`). Request ID is propagated via `httpx.RequestLogger` so every log line attributable to a request carries it.
- **Health endpoint**: `/health` is deliberately un-logged (L2 fix). It gets hammered by load balancers; logging each hit would dominate log volume.
- **Metrics / tracing**: none yet — see `docs/OPERATIONS.md` for the OpenTelemetry follow-up plan.

## Graceful shutdown

`cmd/api/main.go` sets up a `signal.NotifyContext` for `SIGINT` / `SIGTERM`. `app.Application.Run` listens for that cancellation and calls `server.Shutdown` with a detached 10-second budget so in-flight requests can drain. If shutdown overruns, the server is force-closed. The DB pool is closed last, via `defer application.Close()` in `main.go`.

## Non-obvious choices that feel obvious in hindsight

- **`PATCH`, not `PUT`** for `/workouts/{id}` — the body is a partial-merge patch (nil fields left untouched), not a full replacement. RFC 5789.
- **Composition root in `internal/app`** — all wiring in one place, handlers/services/stores are unexported except where the router needs them.
- **Pretty-print JSON toggled via `httpx.SetPrettyJSON`** — readable in dev, compact in prod. One atomic bool, set once at startup.
- **`AnonymousPrincipal` is a sentinel pointer**, not a zero-valued struct. Pointer-identity comparison means a zero-valued Principal constructed elsewhere is not silently treated as anonymous.
- **`FindByUsername` returns `(nil, ErrNotFound)`** — callers in the login flow fold that back to `nil` before calling `VerifyPassword`, preserving the constant-time path.

## What's intentionally not here

- No ORM. `database/sql` + raw SQL. The data model is small enough that a query builder adds more friction than it removes.
- No request-validation framework. `Validate()` methods on the domain types are sufficient and keep invariants next to the data they protect.
- No dependency-injection container. The composition root is a function. Go is good at this.
- No middleware chain abstraction. `chi` already ships one.
- No global state except the `httpx.prettyJSON` atomic bool, which is set once at startup.
