<img src="apps/web/public/icon.v1.png" height=64 />

# GeoDuels

GeoDuels is a production-ready + dev-ready GeoGuessr-style platform built for horizontal scaling.

https://geoduels.io/

## Architecture

### Runtime topology

- `apps/web` (Next.js): browser UI and gameplay shell.
- `services/api` (Go): auth/session/profile endpoints and backend API surface (`/v1`).
- `services/match-coordinator` (Go): matchmaking over websocket (`/queue`), assignment, maintenance status, and recovery (`/v1/session/recover`).
- `services/realtime-gateway` (Go): websocket gatewaying (`/ws/{node}`) to the assigned gameplay node.
- `services/gameplay-node` (Go): round engine and authoritative match state broadcast for assigned matches.
- `workers/location-ingest` (Go): one-off/cron worker that validates and ingests location datasets into PostgreSQL.

### Data and state

- PostgreSQL: source of truth for persistent data (profiles, stats, location catalog, match persistence).
- Redis: queue and distributed coordination state for matchmaking and gameplay node ownership.
- Dataset JSON files (`datasets/*.json`): seed source for location ingest.

### Network flow

1. Browser loads `web`.
2. Browser calls `api` for auth + app APIs (`/v1`).
3. Browser opens websocket matchmaking to `match-coordinator` (`/queue`) to enter duels.
4. `match-coordinator` assigns a match + gameplay route and issues ticket.
5. Browser upgrades to websocket through `realtime-gateway` (`/ws/{node}`), which proxies to the assigned `gameplay-node`.
6. `gameplay-node` runs duel engine and broadcasts authoritative snapshots.

### Match route flow

- `/` is the lobby and launcher.
- `/match/[id]` is the canonical route for a specific match.
- Cold loads resolve through `GET /v1/matches/{id}/bootstrap`.
- Already-authenticated route refreshes can resolve through `GET /v1/matches/{id}/session`.
- A match route can resolve to:
  - live reconnect with a minted gameplay ticket
  - saved history / end-of-match snapshot
  - replaced, forbidden, or missing state

### Kubernetes ingress routing (prod)

- `/` -> `web`
- `/v1` -> `api`
- `/queue` and `/queue/online` -> `match-coordinator`
- `/ws` -> `realtime-gateway`

## Container images

Development (`docker-compose.yml`) uses language runtime images for fast iteration on the backend:

- `golang:1.26` for `api`, `match-coordinator`, `gameplay-node`
- `postgres:16`
- `redis:7`

The web app is typically run directly from `apps/web` with Node during local development.

Production images are built from service Dockerfiles and pushed to registry:

- `geoduels-api`
- `geoduels-match-coordinator`
- `geoduels-realtime-gateway`
- `geoduels-gameplay-node`
- `geoduels-web`
- `geoduels-location-ingest`

## Maintenance and draining

- `gameplay-node` marks itself draining on shutdown, refuses new match creation, and waits for active matches to finish before exit.
- `match-coordinator` excludes draining gameplay nodes from new duel assignment.
- `realtime-gateway` stops accepting new websocket upgrades during shutdown and waits for active proxied sockets to close.
- `api`, `match-coordinator`, `realtime-gateway`, and `gameplay-node` all fail readiness while draining so Kubernetes can stop routing new traffic.
- Redis key `system:maintenance` can publish lobby maintenance state:
  - `queuePaused`: pause duel queueing
  - `playPaused`: pause all new play sessions
  - `phase: warning|active`: drive lobby warning banner / blocking maintenance overlay

## Local development

Prerequisites:

- Docker Desktop
- Go 1.26+
- Node 20+

Start:

```bash
cp .env.example .env
cp apps/web/.env.local.example apps/web/.env.local
docker compose up -d postgres redis
./scripts/migrate.sh up
POSTGRES_URL='postgres://geoduels:geoduels@127.0.0.1:5432/geoduels?sslmode=disable' \
  go run ./workers/location-ingest \
  -dataset datasets/a-source-world.sample.json \
  -map-key a-source-world
docker compose up -d gameplay-node match-coordinator realtime-gateway api
cd apps/web && npm ci && npm run dev
```

The tracked sample map at `datasets/a-source-world.sample.json` contains 10 public landmark locations so contributors can launch a playable local stack without private location data. To use a larger local dataset, keep it in ignored `datasets/*.json` and pass that path to `workers/location-ingest`.

Endpoints:

- Web: `http://localhost:3000`
- API health: `http://localhost:8080/health`
- Queue health: `http://localhost:8090/health`
- Gameplay health: `http://localhost:8091/health`

Stop:

```bash
docker compose down
```

## CI/CD

### Production release (`.github/workflows/release-prod.yml`)

Triggered by git tag push.

- Run Go, web, and manifest checks
- Build and push versioned production images
- Open a PR against the private ops repository configured by `OPS_REPOSITORY`
- Update production image tags and `NEXT_PUBLIC_APP_VERSION` in that ops repository
- Deploy after that release PR is merged and Flux reconciles production

## Production checklist

1. Provision k3s cluster, ingress, DNS, and TLS.
2. Create namespace and required secrets (`geoduels-secrets`, `ghcr-creds`) in the private ops flow.
3. Apply DB migrations in `db/migrations`.
4. Configure the release workflow variables/secrets, especially `OPS_REPO_TOKEN`.
5. Push a release tag (for example `v1.2.3`) to build images and open the Flux release PR.
6. Merge the generated release PR to trigger production rollout through Flux.
7. Run post-deploy health checks for `/health`, queue flow, and websocket gameplay.

## Repo pointers

- `docker-compose.yml` - local stack
- `infra/k3s/base` - base k8s manifests
- `infra/k3s/overlays/k3d` - local 3-node k3d overlay for routing/scaling tests
- production overlays and Flux cluster state live in the private ops repository
- `services/*/Dockerfile`, `apps/web/Dockerfile`, `workers/location-ingest/Dockerfile` - production image definitions
- `docs/architecture.md` - current runtime architecture
