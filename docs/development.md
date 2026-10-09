# Development notes

For local setup, see [Running GeoDuels yourself](../README.md#running-geoduels-yourself).
Commands below run from the repository root unless shown otherwise.

## Backend

Generated sqlc output is ignored. Generate it before building or testing a clean
checkout and after changing SQL queries or migrations; do not edit generated files.

```sh
cd backend
go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.30.0 generate
go test ./...
go vet ./...
```

### Module boundaries

Each directory under `backend/internal` is a module, drawn around the data it
changes. Staff operations live in the module that owns the data and check
`pkg/staff` capabilities themselves; `internal/staff` only manages role grants.
A module calls only the queries in its own files under `db/queries`. When
another module must write in the same transaction, the owner exports a function
that takes the caller's transaction, such as `badges.AwardBadgeTx` or
`accounts.ResetNicknameTx`; staff actions are audited through `audit.Record`.
`internal/architecture` enforces query ownership and the allowed module imports
as part of `go test ./...`.

## Frontend

```sh
npm --prefix web run lint:architecture:strict
npm --prefix web test
(cd web && npx tsc --noEmit)
npm --prefix web run build
```

## Local infrastructure

Migrations are goose files in `backend/db/migrations`. `docker compose -f backend/dev.yaml up` applies
them before the services start; after adding one, run `docker compose -f backend/dev.yaml run --rm db-migrate`.
`go run ./cmd/migrate status` (from `backend/`, with `POSTGRES_URL` set) lists them, and `go run ./cmd/migrate down`
rolls back the latest.
After changing Compose environment variables, recreate containers:

```sh
docker compose -f backend/dev.yaml up -d --force-recreate
```
