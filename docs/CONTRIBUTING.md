# Contributing

This is a small project with a clear shape. The rules below exist to keep it that way.

## Before you start

- Read [`docs/ARCHITECTURE.md`](ARCHITECTURE.md). It explains the package layout, dependency direction, and the non-obvious security choices. Changes that violate those boundaries will be asked to change.
- Run `make help` — the targets are the source of truth for common dev/ops tasks.

## Branching & commits

- Base your work on `main`.
- Use short, descriptive branch names: `feat/workouts-search`, `fix/pg-connection-leak`, `docs/readme-rewrite`.
- Commits follow **Conventional Commits**:
  - `feat: add workouts search endpoint`
  - `fix: guard against nil principal in handler`
  - `refactor: extract token hashing into auth package`
  - `docs: rewrite README`
  - `test: add integration test for update workout`
  - `chore: bump golangci-lint to v1.62.0`
- Keep commits focused. A reviewer should be able to reconstruct intent from the subject line. If a commit message needs a body to explain "why", write it.
- Do not mix refactors and behavioural changes in the same commit.

## Pull requests

A PR is ready to review when all of these hold:

- [ ] `make lint` passes.
- [ ] `make test` passes with `-race -count=1`.
- [ ] `make test-integration` passes if you touched the DB path.
- [ ] The change is focused — no drive-by reformatting, no unrelated refactors.
- [ ] Public API (HTTP routes, request/response shapes) changes are reflected in [`docs/API.md`](API.md).
- [ ] Architectural changes (new bounded context, new dependency direction) are reflected in [`docs/ARCHITECTURE.md`](ARCHITECTURE.md).
- [ ] New env vars land in [`.env.example`](../.env.example) **and** [`docs/OPERATIONS.md`](OPERATIONS.md#configuration).
- [ ] Migrations follow the existing naming (`NNNNN_description.sql`) and goose annotation conventions.

PR description should briefly cover: what changed, why, and what you tested. Don't restate the diff.

## Code style

- **`gofmt -s`** + **`goimports`** with local prefix `github.com/tsatsarisg/go-fit`. `make fmt` does both.
- **`golangci-lint`** is authoritative; the enabled linter set is in [`.golangci.yml`](../.golangci.yml).
- **No comments that restate the code.** Comments explain *why*, not *what*. If a comment removal would surprise a future reader, keep it. Otherwise delete it.
- **Errors**: wrap with `fmt.Errorf("context: %w", err)` when adding context. Define sentinels at the package that owns the semantic (e.g. `workout.ErrNotFound`). Cross-package error mapping belongs in `httpx.WriteStoreError`, not in handlers.
- **Context**: every function that does IO takes `context.Context` as the first argument. Every DB call uses the `*Context` variant.
- **Naming**: domain types are unprefixed (`Workout`, not `WorkoutDTO`). Commands end in `Command` (`RegisterCommand`, `UpdateWorkoutCommand`). IDs are named types (`UserID`, `WorkoutID`), not raw `int64`.
- **Public API**: only export what needs to be exported. Handlers are methods on feature `Handler` structs; Feature packages expose `NewService`, `NewHandler`, `NewPostgresStore`, and their types.
- **No package-level state** except the `httpx.prettyJSON` atomic bool. If you're tempted to add one, push it into the composition root in `internal/app/`.

## Adding a new feature package (new bounded context)

1. Create `internal/<feature>/` with the standard layout:
   - `model.go` — domain types + `Validate()` methods.
   - `service.go` — `Service` struct, commands, `Store` interface (consumer-side), sentinel errors.
   - `handler.go` — HTTP handler methods.
   - `postgres_store.go` — adapter implementing `Store`.
   - `*_test.go` — unit + integration tests.
2. Wire it up in `internal/app/app.go`:
   - Construct the store → service → handler chain.
   - Register routes on the chi router.
3. Respect the dependency direction: your new package can import `user` (for `UserID`), `httpx`, and `platform/postgres`. It should not be imported by `user`.
4. Document the new endpoints in [`docs/API.md`](API.md).

## Adding a migration

1. Create the next numbered file: `migrations/NNNNN_<description>.sql`.
2. Use the standard goose annotations:
   ```sql
   -- +goose Up
   -- +goose StatementBegin
   CREATE TABLE ...
   -- +goose StatementEnd

   -- +goose Down
   -- +goose StatementBegin
   DROP TABLE ...
   -- +goose StatementEnd
   ```
3. Both `Up` and `Down` are required. If a migration is truly irreversible, write the best-effort `Down` you can and say so in a comment.
4. Test against a fresh DB: `make compose-down-volumes && make compose-up`. Watch for the `database migrated` log line.
5. Don't edit an already-merged migration. Add a new one.

## Security-sensitive code

If your change touches any of these, flag it in the PR description and expect extra scrutiny:

- The login / token flow (`internal/auth/`).
- Password handling (`internal/user/hasher.go`, `internal/user/model.go`).
- The `VerifyPassword` constant-time path (critical for the enumeration fix — see `ARCHITECTURE.md`).
- The production SSL guard (`internal/config/config.go`).
- Ownership enforcement in workout SQL.

Do not "simplify" these without understanding the attack they prevent.

## Questions / disagreements

Open an issue before a large refactor. If a convention here feels wrong, challenge it in an issue with your reasoning — the conventions are load-bearing, but they're not sacred.
