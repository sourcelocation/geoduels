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

A type lives in the slice that owns it; there is no shared package of types.
Entities stay in their module (`internal/*`, or `pkg/gameplay` and `pkg/matchkind`
for matches) and carry JSON tags only when that JSON is how they are stored,
such as a replay or a match config. What clients send and receive are DTOs
defined beside the handler that speaks them in `services/api` or
`services/gameplay-node`, mapped from entities explicitly; match state reaches
clients only through the `Client*` views in `pkg/gameplay`.

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
- Clients read `GET /api/v2/matches/{id}` and play over `/api/live/matches/{id}`, which the gateway
  forwards to the node running the match.

### Live

Two services handle what people see as it happens, and both can run at any number of replicas:

- `services/api` answers HTTP. Everything anyone does is an HTTP call, and anything that happens is
  published after it commits, to the person's NATS subject (`pkg/live`), as the JSON the browser gets.
  A transaction elsewhere that needs people told asks for a River job on the `live` queue, which the
  API works once it commits (`jobs.Teller`).
- `services/gateway` holds the browser's sockets and has no database. It forwards what's published
  without reading it, and asks the API about everything else over `/internal/live/*`, which isn't routed
  publicly.

Nothing is kept along the way, and nothing has to be: every connection starts with a `session` event
holding everything the client needs, read after subscribing, so losing an event only delays it until the
next reconnect. A client too far behind is disconnected, and a gateway that loses NATS asks all its
clients to reconnect. Each event has an example in `tests/shared/live-events.json`, which the server's
and the web client's tests both check; `UPDATE_FIXTURES=1 go test ./services/api` rewrites it. A party
has a `revision`, and clients keep the highest they've seen.

Everything a person is told is one of two things:

- A **conversation item** (chat): a message, or a friend request or party invitation, which stay in their
  own tables and appear as items. The server words each one and says what can be done about it, as
  `actions` of `{label, post, open}`; the same item with `notify` is a toast. No client code knows what
  any of them are for.
- An **inbox message**: something to read, with no actions, like a badge, a ban or a warning
  (`internal/inbox`). Reading a warning acknowledges it.

Who is online is the unlogged `gateways` and `live_connections` tables (`internal/presence`). Each gateway
reports what it holds every 15 seconds, which repairs anything missed, and a gateway that stops reporting
is swept by any API replica. Anyone connected counts as online, signed in or not. A person's last
connection closing, after a few seconds' grace for reloads, takes them out of the queue.

## Frontend

```sh
npm --prefix web run lint:architecture:strict
npm --prefix web test
(cd web && npx tsc --noEmit)
npm --prefix web run build
```

### How pages are served

The server renders the same interface the browser runs, from public data only, and keeps the result
(`web/lib/cached-page.ts`): a page is served from memory and rebuilt in the background at most once a
minute. If rebuilding fails, the last good page keeps being served. Everything personal or live is
the browser's: it reads `/api/bootstrap`, fetches what the player needs and opens its sockets.

- Whatever a page's `load` puts in `queries` is what its own `useQuery` calls find, so crawlers get
  real content from the normal components. Use the same query keys, under the anonymous user.
- Nothing about a player reaches the server's render: effects don't run there, so the one
  `authGateway` and `homeRuntime` in the process stay empty. Their constructors acquire nothing; browser
  resources open in `start()`, called from an effect.
- `router.isReady` is false on the server for these pages. Don't read it while rendering.
- A map a visitor can't see might be private to someone signed in, so it is `noindex` rather than a 404.
- Links to pages built on request (`/maps/[id]`, `/players/[id]`, `/match/[id]`) use `prefetch={false}`:
  a list of them would otherwise build every page it shows.

`SITE_URL` and `API_URL` are read by the web server only. The browser calls `/api` on its own origin
and gets its public keys from `/api/bootstrap` (the API's `GOOGLE_CLIENT_ID`, `DISCORD_CLIENT_ID`,
`TURNSTILE_SITE_KEY`, `GOOGLE_EMBED_KEY`).

## Local infrastructure

`docker compose up -d` runs Postgres and NATS, and nothing else: `overmind start` runs the rest on this
machine from the `Procfile`. The Go services run under Air, pinned in `backend/tools`, which rebuilds
and restarts each one when its code changes. They read `.env`, then `backend/dev.env` for what compose
would give their containers. The web app reads `.env` and takes port 24000.

`overmind connect api` attaches to one process, `overmind restart api` restarts it, and `overmind quit`
stops them all. The moderation worker and Discord bot aren't started by default (`.overmind.env`), since
playing doesn't need them and the bot needs a token; `overmind start -l api,gateway,gameplay,web,moderation`
picks exactly which processes run.

Migrations are goose files in `backend/db/migrations`. Each service that uses the database applies them
when Air rebuilds it, so saving a new one is enough. `go run ./cmd/migrate status` (from `backend/`, with
`POSTGRES_URL` set) lists them, and `go run ./cmd/migrate down` rolls back the latest. Postgres is at
`postgres://geoduels:geoduels@localhost:24003/geoduels?sslmode=disable`. After changing `.env`, run
`overmind restart`.
