<img src="apps/web/public/icon.v1.png" height=64 />

# GeoDuels

GeoDuels is a free GeoGuessr-inspired multiplayer geography game with a focus on user-generated maps and real-time competitive dueling. The game is playable at

https://geoduels.io/

## Architecture

### Runtime topology

- `apps/web` (Next.js): browser UI and gameplay shell.
- `services/api` (Go): auth/session/profile, public player profiles, maps, parties, match history, moderation/admin, content, and support APIs (`/v1`).
- `services/match-coordinator` (Go): matchmaking over websocket (`/queue`), party coordination/chat, assignment, presence, and maintenance status. Resumable-session lookup is exposed by the API at `/v1/session/resumable`.
- `services/realtime-gateway` (Go): websocket gatewaying (`/ws/{node}`) to the assigned gameplay node.
- `services/gameplay-node` (Go): round engine and authoritative match state broadcast for assigned matches.
- `services/moderation-worker` (Go): background moderation signal notifications and risk-engine work.
- `services/discord-worker` (Go): Discord role synchronization and membership badge processing.
- `workers/storage-maintenance` (Go): replay compression, retention cleanup, and other bounded storage maintenance.

### Data and state

- PostgreSQL: source of truth for profiles, stats, user maps and their current location datasets, round plans, and match persistence.
- Redis: queue and distributed coordination state for matchmaking and gameplay node ownership.

### Network flow

1. Browser loads `web`.
2. Browser calls `api` for auth + app APIs (`/v1`).
3. Browser opens websocket matchmaking to `match-coordinator` (`/queue`) to enter duels.
4. `match-coordinator` assigns a match + gameplay route and issues ticket.
5. Browser upgrades to websocket through `realtime-gateway` (`/ws/{node}`), which proxies to the assigned `gameplay-node`.
6. `gameplay-node` runs duel engine and broadcasts authoritative snapshots.

Before step 4, the launching service locks the selected map while reading its current locations and persists the match's complete round plan. Gameplay pods receive that bounded plan and never preload map catalogs.

## Custom maps

- Signed-in non-guest accounts upload JSON through `/v1/maps`; uploads are validated and normalized directly into PostgreSQL, and the source file is discarded.
- Creator trust tiers enforce transactional quotas:
  - base: 10 maps, 200,000 active locations, 10 uploads/hour, and 30 uploads/day
  - trusted: 25 maps, 500,000 active locations, 10 uploads/hour, and 30 uploads/day
  - established: 100 maps, 1,000,000 active locations, 10 uploads/hour, and 30 uploads/day
- Trust advances from account age and qualified favorites/maps. Moderation restrictions force the base tier, and administrators can apply a tier override.
- A replacement upload atomically swaps the map's current locations. The current tier's active-location allowance is also the per-map location ceiling.
- Ranked duels always use the official server-selected map. Private lobbies may select an accessible ready map independently from movement rules.

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
- `geoduels-moderation-worker`
- `geoduels-discord-worker`
- `geoduels-web`

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
./scripts/dev-up.sh
cd apps/web
npm ci
npm run dev
```

Create and manage maps through the web map administration UI.

For bulk official country maps generated from a local Vali install, use the
dry-run-first pipeline in [`docs/map-imports.md`](docs/map-imports.md).

To remove stale or unavailable Street View panoramas from a Vali export, validate it before uploading through the web UI:

```bash
cd apps/web
npm ci
GOOGLE_MAPS_API_KEY='server-key' npm run validate:streetview -- \
  --input ../../datasets/world.json \
  --output ../../datasets/world.clean.json
```

The key must have Street View Static API enabled and should be restricted to that API and, where practical, the machine's IP address. The validator calls only Google's no-charge Street View metadata endpoint and never requests imagery. Deleted panorama IDs are refreshed from their saved coordinates against the nearest outdoor panorama within 50 meters. The run is resumable through an append-only checkpoint and writes refreshed IDs and rejected locations beside the clean output.

Endpoints:

- Web: `http://localhost:3000`
- API health: `http://localhost:8080/health`
- Queue health: `http://localhost:8090/health`
- Gameplay health: `http://localhost:8091/health`
- Realtime health: `http://localhost:8092/health`
- Moderation worker health, when started: `http://localhost:8093/health`
- Discord worker health, when started: `http://localhost:8094/health`

The core playable stack is `gameplay-node`, `match-coordinator`, `realtime-gateway`, and `api`. Start the background workers when exercising their features:

```bash
docker compose up -d moderation-worker discord-worker
```

`discord-worker` requires the Discord bot and guild environment variables from `.env`.

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
3. Apply DB migrations with `./scripts/migrate.sh up`. Fresh databases use the
   GeoDuels v2 version-2000 schema in `db/migrations`; existing databases below
   version 2000 must first be upgraded with `./scripts/migrate.sh --legacy up`.
4. Configure the release workflow variables/secrets, especially `OPS_REPO_TOKEN`.
5. Push a release tag (for example `v1.2.3`) to build images and open the Flux release PR.
6. Merge the generated release PR to trigger production rollout through Flux.
7. Run post-deploy health checks for `/health`, queue flow, and websocket gameplay.

### Storage optimization migration

Migration 42 removes redundant match guesses/indexes, converts map locations to compact fixed-width values, and stores new replays as Zstandard-compressed PostgreSQL blobs with 30-day retention. Apply it during a write maintenance window.

The compaction helper intentionally requires the database to be exactly at schema version 42. Migration 42 is part of the historical path, so when the legacy database is at version 41, use `./scripts/migrate.sh --legacy up 1` to apply only migration 42, start the migration-42-compatible application, run smoke tests, stop writes, compact, and only then continue the legacy migrations through the version-2000 v2 cutover:

```bash
CONFIRM_STORAGE_COMPACTION=yes \
MIGRATIONS_DB_URL='postgres://user:pass@host:5432/geoduels?sslmode=disable' \
./scripts/compact-storage.sh
```

Do not run the helper after migrations 43 or later have been applied; it will refuse to proceed. For a database already beyond version 42, plan any `VACUUM FULL` work separately with a PostgreSQL administrator instead of changing the script's version guard.

To accelerate legacy replay compression and retention cleanup before compaction:

```bash
POSTGRES_URL='postgres://user:pass@host:5432/geoduels?sslmode=disable' \
go run ./workers/storage-maintenance -batch-size 1000 -max-batches 0
```

For production PostgreSQL, enable `wal_compression=on` and `track_io_timing=on` in server configuration. Keep at least 15 GiB free before running the rewrite.

## Documentation

- [`AGENTS.md`](AGENTS.md) - repository boundaries and essential context for coding agents.
- [`docs/architecture.md`](docs/architecture.md) - service ownership, data flow, routing, reconnects, maintenance, and persistence.
- [`docs/development.md`](docs/development.md) - local macOS setup, service startup, tests, and the k3d development environment.
- [`docs/deployment.md`](docs/deployment.md) - production release flow, database migration policy, and post-deploy checks.
- [`docs/map-imports.md`](docs/map-imports.md) - Vali country-map generation, dry-run validation, and admin production import.
- [`apps/web/docs/frontend-architecture.md`](apps/web/docs/frontend-architecture.md) - frontend ownership, shared UI and styling rules, and file-size budgets.
- [`infra/k3s/README.md`](infra/k3s/README.md) - reusable Kubernetes manifests and local k3d scaling tests.
- [`apps/web/assets/source-map-thumbnails/README.md`](apps/web/assets/source-map-thumbnails/README.md) - source requirements and attribution for generated map thumbnails.
- [`CONTRIBUTOR_LICENSE_AGREEMENT.md`](CONTRIBUTOR_LICENSE_AGREEMENT.md) - contributor licensing terms.

## Repo pointers

- `docker-compose.yml` - local stack
- `infra/k3s/base` - base k8s manifests
- `infra/k3s/overlays/k3d` - local 3-node k3d overlay for routing/scaling tests
- production overlays and Flux cluster state live in the private ops repository
- `services/*/Dockerfile`, `apps/web/Dockerfile` - production image definitions
