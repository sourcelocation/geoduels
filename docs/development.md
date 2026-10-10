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

### Matches

A match's kind (`pkg/matchkind`) is its one description: rated or not, which maps it plays, who may
be in it, and whether starting another match replaces it. Its row in `match_sessions` is the only
record of it while it runs; nothing about matches lives anywhere else.

- `services/api` starts every match in one transaction (`match_start.go`): the queue's matchmaker,
  a party's start command and `POST /api/v2/matches` all go through it. A player holds at most one
  unfinished seat (`match_participants_one_active`).
- A gameplay node holds a lease named `gameplay-node:<id>` in `control_plane_leases`, picks up
  waiting matches itself and stamps them with the lease's fencing token. A match is live while that
  lease holds (`gd_match_status`); a node that dies takes its matches with it within the lease's
  ten seconds. A new match frees its players' seats in such matches itself; the storage job only
  records their end so the rows can be cleaned up.
- Who is online is the unlogged `presence` table, refreshed by any open socket every half minute.
  Friends read it when their friends list refreshes; nothing pushes it.
- Clients read `GET /api/v2/matches/{id}` and play over `/api/v2/matches/{id}/ws`, which the API
  proxies to the node running the match.
- Events between processes (notifications, parties, chat, match starts) go through Postgres `NOTIFY` (`pkg/pgnotify`). A
  LISTEN needs a session, so behind a transaction-pooling PgBouncer set `POSTGRES_LISTEN_URL` to
  reach Postgres directly.

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
