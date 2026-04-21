# API reference

All endpoints are served from the base URL the binary listens on — `http://localhost:8080` by default. Every response is JSON except `GET /health`, which returns the plaintext body `OK`.

Base conventions:

- **Content type**: requests and responses are `application/json` unless noted.
- **Envelope**: successful responses wrap the resource under a named key (`{"workout": ...}`, `{"user": ...}`). Errors use `{"error": "..."}`.
- **Auth**: protected endpoints require `Authorization: Bearer <token>`. Tokens come from `POST /tokens/authentication` and live for 24 hours.
- **Unknown fields**: request bodies are decoded with `DisallowUnknownFields`. Typos return `400`.
- **IDs**: all resource IDs are `int64` (encoded as JSON numbers).

---

## Health

### `GET /health`

Liveness probe. No auth. Not logged.

```bash
curl -i http://localhost:8080/health
```

```
HTTP/1.1 200 OK
Content-Length: 2
Content-Type: text/plain; charset=utf-8

OK
```

---

## Users

### `POST /users`

Register a new user.

**Request body**

| Field | Type | Required | Notes |
| --- | --- | --- | --- |
| `username` | string | yes | Unique. |
| `email` | string | yes | Unique, validated via `net/mail`, stored lowercased. |
| `password` | string | yes | Minimum 12 characters. |
| `bio` | string | no | Free text. |

```bash
curl -X POST http://localhost:8080/users \
  -H 'Content-Type: application/json' \
  -d '{
    "username": "alice",
    "email": "alice@example.com",
    "password": "correct-horse-battery-staple",
    "bio": "lifts things up and puts them down"
  }'
```

**Response** — `201 Created`

```json
{
  "user": {
    "id": 1,
    "username": "alice",
    "email": "alice@example.com",
    "bio": "lifts things up and puts them down",
    "created_at": "2026-04-21T19:00:00Z",
    "updated_at": "2026-04-21T19:00:00Z"
  }
}
```

**Errors**

| Status | Condition |
| --- | --- |
| `400` | Missing field, invalid email, password under 12 chars |
| `409` | `username` or `email` already taken (generic body — no enumeration) |
| `500` | DB error |

The `409` body is intentionally generic (`"resource already exists"`) rather than `"email already taken"` so registration can't be used to probe whether an address has an account.

---

## Authentication

### `POST /tokens/authentication`

Log in with username + password, receive a bearer token.

**Request body**

| Field | Type | Required |
| --- | --- | --- |
| `username` | string | yes |
| `password` | string | yes |

```bash
curl -X POST http://localhost:8080/tokens/authentication \
  -H 'Content-Type: application/json' \
  -d '{"username":"alice","password":"correct-horse-battery-staple"}'
```

**Response** — `200 OK`

```json
{
  "token": "OQMYYSKXMK22JZTOJIHL3MI7MI",
  "expiry": "2026-04-22T19:00:00Z"
}
```

The token lives 24 hours. Store it; the plaintext is only returned here once.

**Errors**

| Status | Condition |
| --- | --- |
| `400` | Malformed body |
| `401` | Invalid credentials — **same body for "unknown user" and "wrong password"**, by design (no enumeration, constant-time bcrypt either way) |
| `500` | DB error |

### `POST /tokens/authentication/logout`

Revoke every auth-scoped token for the calling principal. Requires a valid bearer token (so you need one token to revoke all of them).

```bash
curl -X POST http://localhost:8080/tokens/authentication/logout \
  -H 'Authorization: Bearer OQMYYSKXMK22JZTOJIHL3MI7MI'
```

**Response** — `204 No Content`

**Errors**

| Status | Condition |
| --- | --- |
| `401` | Missing / malformed / expired / unknown token |
| `500` | DB error |

---

## Workouts

All workout endpoints require `Authorization: Bearer <token>`. A request with no header, a malformed header, or an invalid token is rejected before reaching the handler.

### Resource shape

```json
{
  "id": 42,
  "user_id": 1,
  "title": "Morning Run",
  "description": "easy pace, zone 2",
  "duration_minutes": 30,
  "calories_burned": 250,
  "entries": [
    {
      "id": 101,
      "exercise_name": "run",
      "sets": 1,
      "reps": null,
      "duration_seconds": 1800,
      "weight": null,
      "notes": "",
      "order_index": 0
    }
  ]
}
```

**Entry invariants (enforced at domain and DB level):**

- Exactly one of `reps` or `duration_seconds` must be present.
- `sets`, `reps`, `duration_seconds`, `weight` must all be non-negative when set.
- `exercise_name` is required.

---

### `POST /workouts`

Create a workout owned by the calling user. `user_id` is taken from the authenticated principal; sending one in the body is ignored (no ownership forgery).

**Request body**

| Field | Type | Required | Notes |
| --- | --- | --- | --- |
| `title` | string | yes | |
| `description` | string | no | |
| `duration_minutes` | int | no | Non-negative. |
| `calories_burned` | int | no | Non-negative. |
| `entries` | array | no | See entry shape above. |

```bash
curl -X POST http://localhost:8080/workouts \
  -H 'Authorization: Bearer <TOKEN>' \
  -H 'Content-Type: application/json' \
  -d '{
    "title": "Morning Run",
    "duration_minutes": 30,
    "calories_burned": 250,
    "entries": [
      {"exercise_name": "run", "sets": 1, "duration_seconds": 1800, "order_index": 0}
    ]
  }'
```

**Response** — `201 Created` with the resource envelope.

**Errors**

| Status | Condition |
| --- | --- |
| `400` | Validation failure (empty title, negative values, entry with both `reps` and `duration_seconds`, etc.) |
| `401` | Missing / invalid token |
| `500` | DB error |

---

### `GET /workouts/{id}`

Fetch a workout by id. Returns the workout regardless of owner (read path); ownership scoping on reads can be added to the service if needed.

```bash
curl -H 'Authorization: Bearer <TOKEN>' http://localhost:8080/workouts/42
```

**Response** — `200 OK` with the resource envelope.

**Errors**

| Status | Condition |
| --- | --- |
| `400` | `{id}` is not an int64 |
| `401` | Missing / invalid token |
| `404` | Workout not found |

---

### `PATCH /workouts/{id}`

Partial update (RFC 5789 merge patch). Fields omitted from the body are left untouched. Ownership is enforced in SQL in the same statement as the update, closing the TOCTOU window that a separate "fetch then update" pattern would have.

**Request body** — all fields optional, all are nullable pointers on the server:

| Field | Type | Notes |
| --- | --- | --- |
| `title` | string | Non-empty if supplied. |
| `description` | string | |
| `duration_minutes` | int | Non-negative. |
| `calories_burned` | int | Non-negative. |
| `entries` | array | **Full replace** of the entry collection when supplied. Pass `[]` to clear, omit to leave alone. |

```bash
curl -X PATCH http://localhost:8080/workouts/42 \
  -H 'Authorization: Bearer <TOKEN>' \
  -H 'Content-Type: application/json' \
  -d '{"title": "Evening Run", "calories_burned": 275}'
```

**Response** — `200 OK` with the resource envelope (updated row).

**Errors**

| Status | Condition |
| --- | --- |
| `400` | Validation failure |
| `401` | Missing / invalid token |
| `403` | Workout exists but belongs to another user |
| `404` | Workout does not exist |
| `500` | DB error |

---

### `DELETE /workouts/{id}`

Delete a workout the caller owns. Ownership is enforced in the SQL `WHERE` clause.

```bash
curl -X DELETE http://localhost:8080/workouts/42 \
  -H 'Authorization: Bearer <TOKEN>'
```

**Response** — `204 No Content`

**Errors**

| Status | Condition |
| --- | --- |
| `400` | `{id}` is not an int64 |
| `401` | Missing / invalid token |
| `403` | Workout exists but belongs to another user |
| `404` | Workout does not exist |
| `500` | DB error |

---

## Status code cheatsheet

| Status | Meaning here |
| --- | --- |
| `200 OK` | Success with a body |
| `201 Created` | Resource created, body returned |
| `204 No Content` | Success, no body |
| `400 Bad Request` | Validation or decode failure (unknown field, wrong type, domain invariant) |
| `401 Unauthorized` | No token, bad token, or login failure |
| `403 Forbidden` | Authenticated, but you don't own the resource |
| `404 Not Found` | Unknown resource id |
| `409 Conflict` | Uniqueness violation (registration) |
| `500 Internal Server Error` | Bug or infra failure — body is always generic, details are in the server logs keyed by `request_id` |

Every `500` is logged via `slog.ErrorContext` with the request id; grep the logs with the id from your client's response tracing to correlate.

---

## Versioning

The HTTP API is currently **unversioned** (no `/v1/` prefix). This is a portfolio project, not a public API. If it ever needs versioning, a `/v1/` prefix is the planned approach — routing lives in `internal/app/app.go`.
