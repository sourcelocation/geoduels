# Development on macOS

## Prerequisites

- Docker Desktop
- Go 1.26+
- Node 20+

## Prepare infrastructure

```bash
cp .env.example .env
./scripts/dev-up.sh
```

`dev-up.sh` starts PostgreSQL and Redis, applies migrations, bootstraps sample
maps when needed, and starts the playable backend stack. The development-data
bootstrap is idempotent: it imports the lightweight sample dataset only when
the configured Moving or NMPZ map is missing. Run
`./scripts/bootstrap-dev-data.sh` at any time to repair an empty local database
without replacing existing maps.

Create and manage maps through the web map administration UI.

## Start the backend stack

The core playable backend services are defined in `docker-compose.yml`.

To rebuild/recreate containers before starting them:

```bash
docker compose up -d --force-recreate gameplay-node match-coordinator realtime-gateway api
```

Start background workers when exercising moderation or Discord integration:

```bash
docker compose up -d moderation-worker
docker compose up -d discord-worker
```

`discord-worker` requires the Discord bot, guild, channel, and role IDs in `.env`.
`moderation-worker` can run without the private risk engine. To exercise private
detector integration locally, run the sibling `../geoduels-risk-engine` service
and set `RISK_ENGINE_URL=http://host.docker.internal:8096` plus
`RISK_ENGINE_TOKEN` before starting Docker Compose.

## Start the web app

Run the Next.js app separately:

```bash
cd apps/web
npm ci
cp .env.local.example .env.local
npm run dev
```

The browser connects directly to the local backend services. Next.js does not
proxy API, coordinator, or realtime traffic in development.

## Endpoints

- Web: `http://localhost:3000`
- API health: `http://localhost:8080/health`
- Queue health: `http://localhost:8090/health`
- Gameplay health: `http://localhost:8091/health`
- Realtime websocket base: `ws://localhost:8092/ws/{node}`
- Moderation worker health, when started: `http://localhost:8093/health`
- Discord worker health, when started: `http://localhost:8094/health`

## PostgreSQL Local (macOS)

Start local PostgreSQL container:

```bash
docker compose up -d postgres
```

Run migrations with the repository helper, which uses the pinned migration container. This release applies only forward migrations to an existing GeoDuels v2 database at schema version 2000 or later:

```bash
MIGRATIONS_DB_URL='postgres://geoduels:geoduels@127.0.0.1:5432/geoduels?sslmode=disable' \
./scripts/migrate.sh up
```

For a database below migration 2000, check out the `v2.0.1` tag and complete
its migration path before returning to this release. Post-v2 migrations belong
in `db/migrations` at version 2001 or later.

To verify v2 forward migrations from schema version 2000 using a disposable
PostgreSQL container, run:

```bash
./scripts/test-migrations-local.sh
```

The check confirms that forward migrations apply cleanly from the published
version-2000 baseline.

Set backend DB URL in `.env`:

```bash
POSTGRES_URL=postgres://geoduels:geoduels@localhost:5432/geoduels?sslmode=disable
```

Restart backend services after changing `.env`:

```bash
docker compose up -d --force-recreate gameplay-node match-coordinator realtime-gateway api
```

Stop local PostgreSQL:

```bash
docker compose stop postgres
```

## Stop stack

```bash
docker compose down
```

## Running Go tests locally

```bash
go test ./...
```

## Local k3d routing test

For a local multi-node Kubernetes test of websocket routing and `gameplay-node` scaling, use `infra/k3s/overlays/k3d` with a k3d cluster that has 3 agent nodes. The overlay expects PostgreSQL and Redis to stay on the host and be reachable from the cluster through `host.k3d.internal`.
