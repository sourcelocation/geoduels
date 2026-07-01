# Development on macOS

## Prerequisites

- Docker Desktop
- Go 1.26+
- Node 20+

## Prepare infrastructure

```bash
cp .env.example .env
docker compose up -d postgres redis
./scripts/migrate.sh up
```

Create and manage maps through the web map administration UI.

## Start the backend stack

```bash
docker compose up -d gameplay-node match-coordinator realtime-gateway api
```

This starts the core playable backend services defined in `docker-compose.yml`.

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

Run migrations with the repository helper, which uses the pinned migration container:

```bash
MIGRATIONS_DB_URL='postgres://geoduels:geoduels@127.0.0.1:5432/geoduels?sslmode=disable' \
./scripts/migrate.sh up
```

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
