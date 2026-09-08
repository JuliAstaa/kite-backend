# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

`kite-financial-tracker` backend — Kite, a **single-user** personal finance REST API in Bahasa Indonesia. Go 1.26 with `net/http` stdlib only: no web framework, no ORM, no migration library. PostgreSQL via `database/sql` + `pgx/v5` stdlib driver. Comments, error messages, and docs are in Indonesian — match that when editing.

This directory is its own git repo; the sibling `../frontend` (Next.js) is a separate repo with its own `CLAUDE.md`.

Specs, in order of authority:
- `prd-backend.md` — the contract. Business rules are numbered there and referenced from code comments (e.g. `// aturan 4` in `transaction/service.go`). Don't invent fields or endpoints.
- `docs/API.md` — the frontend-facing API doc. Update it when request/response shapes change.
- `README.md` — running, structure, and the "decisions to remember" section.

## Commands

```bash
cp .env.example .env    # set DB_URL (and DB_URL_TEST for repository tests)
make db-up              # Postgres 16 via docker compose
make run                # migrations run automatically at startup
make test               # all tests (-p 1)
make test-unit          # -short, no database needed
make vet                # go vet ./...
make fmt                # gofmt -w ./cmd ./db ./internal
make backup             # pg_dump to backups/
```

`make help` lists everything. There is no lint step beyond `go vet`.

## Architecture

Feature-first (vertical slice). Each feature owns every layer in one folder:

```
cmd/api/main.go            constructs every feature repo→service→handler, starts server
db/migrations/             numbered .up.sql / .down.sql, embedded via db/embed.go
internal/features/<name>/  model, dto, repository, service, handler, routes (+ scheduler for recurring)
internal/platform/         config, database, middleware, router, wiring
internal/shared/           apperror, response, validator, queryparam, timeutil, httpx, testutil
```

Features: analytics, backup, budget, category, health, quickadd, recurring, saving, transaction, wallet, wishlist.

**Features must never import another feature package.** When A needs data from B, A declares a small interface *in its own package* (`transaction.CategoriesReader`, `wishlist.SavingsReader`), and `internal/platform/wiring` adapts B's service to it. Only `wiring` and `main.go` know about two features at once. Consequence for tests: a fake struct satisfies the interface, so service tests need no database.

Aggregate reads (analytics, savings, budget status, wallet balance) query the `transactions` table directly in that feature's repository — no cross-package adapter for summing numbers.

**Layering per feature.** `repository.go` defines a `<Name>Repositorer` interface + the `*sql.DB` implementation; `service.go` defines `<Name>Servicer` and holds all business rules; `handler.go` only parses/validates request shape and maps to status codes; `routes.go` registers paths on a `*http.ServeMux` **without** the `/api/v1` prefix — `platform.NewRouter` applies it once with `http.StripPrefix`.

**Routing.** Go 1.22+ `ServeMux` patterns. Some routes use method-prefixed patterns (`"POST /transactions/{id}/restore"`); collection/item routes register one handler that switches on `r.Method` and returns `405 METHOD_NOT_ALLOWED` in the default branch. Middleware chain, outermost first: `Recover`, `Logging`, `CORS`, `APIToken` (`internal/platform/router.go`).

**Errors.** Services return typed errors from `internal/shared/apperror`; handlers call `response.WriteServiceError`, which maps them via `response.StatusFromError`: `ValidationError`→400 `VALIDATION_ERROR` (field goes into `details`), `NotFoundError`→404, `AlreadyExistsErr`/`ConflictError`→409, `UnprocessableError`→422, anything else→500 `INTERNAL_ERROR`. Never write status codes ad hoc — add an apperror type instead.

**Response envelope** (`internal/shared/response`): `{ "data": ... }`, list responses add `"meta": {total, limit, offset}`, errors are `{ "error": { code, message, details? } }`.

## Non-obvious rules

- **Money is `BIGINT`, whole rupiah.** No `NUMERIC`, no float, no cents. Never divide or multiply by 100.
- **Wallet balance is never stored.** It is computed from `initial_balance` plus transaction aggregation on every read. Do not add a balance column.
- **Soft delete everywhere.** Every base query filters `deleted_at IS NULL`; unique constraints are partial indexes `WHERE deleted_at IS NULL`; foreign keys deliberately have no `ON DELETE CASCADE`. Joins that resolve historical names (`transaction.detailColumns`) intentionally include soft-deleted wallets/categories and expose `is_deleted`.
- **Time is app-local, not UTC.** `timeutil.Init(cfg.TZ)` (default `Asia/Makassar`) is called once in `main` — and again in every `TestMain`. Use `timeutil.Now/ParseDate/StartOfDay/EndOfDay`, never `time.Now()` directly. `from`/`to` query params are `YYYY-MM-DD` and **inclusive**: `httpx.DateRange` pushes `to` to end of day.
- **UUIDs are validated in the handler** (`validator.IsValidUUID`) so a malformed id returns 400 instead of reaching Postgres and coming back as a 500.
- **Migrations run at startup**, by a hand-rolled runner in `internal/platform/database/migrate.go`: `<version>_<name>.up.sql` files embedded from `db/`, applied one per transaction, recorded in `schema_migrations_applied`. `.down.sql` files exist but this runner never executes them. It backfills once from a pre-existing golang-migrate `schema_migrations` table. New migration = next number, both directions, no renumbering.
- **`API_TOKEN` empty makes the token middleware a no-op** — that's the local-dev default. `OPTIONS` preflight skips the check.
- Multi-statement writes go through `database.WithTx`.

## Testing

Every feature has three test files: `service_test.go` (business rules, fake repository), `repository_test.go` (real SQL against Postgres), `handler_test.go` (parsing + status codes, fake service).

- Repository tests need `DB_URL_TEST`. If unset or unreachable, `testutil.TryConnectTestDB` returns nil and those tests **skip** (5s connect timeout) while service and handler tests still run — read the skip lines in output before calling a run green.
- Repository tests share one database and `TRUNCATE` each other's tables. Correctness comes from a Postgres advisory lock held per package for its whole run (`testutil.LockTestDB` in `TestMain`), not from remembering to pass a flag — plain `go test ./...` is safe. `make test` still uses `-p 1` because it's faster than queueing on the lock.
- `TestMain` also calls `timeutil.Init("Asia/Makassar")` and `testutil.MigrateTestDB`. Copy the existing `main_test.go` when adding a feature that touches the database.
- Test helpers for fixtures live in `internal/shared/testutil` (`InsertWallet`, `InsertCategory`, `InsertTransaction`, `InsertTransfer`, `TruncateAll`).

## Adding a feature

1. `internal/features/<name>/` with model, dto, repository (+ interface), service (+ `Servicer` interface and any `XReader` interfaces it needs), handler, routes, and the three test files (+ `main_test.go` if it hits the DB).
2. Migration in `db/migrations/` if new tables are involved.
3. Adapters in `internal/platform/wiring` for any cross-feature dependency.
4. Add the handler to `platform.Handlers` and register its routes in `internal/platform/router.go`.
5. Construct it in `cmd/api/main.go`.
6. Document the endpoints in `docs/API.md` and update `README.md`'s endpoint table.
