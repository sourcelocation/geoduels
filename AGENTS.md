# GeoDuels

GeoDuels is a GeoGuessr-style multiplayer game: Next.js web client, account/API service, matchmaking/coordinator, realtime gateway, and authoritative gameplay nodes.

## Service Boundaries

- `services/api`: browser sessions, OAuth, self/public profiles, leaderboard, maps, content/support, match history/session lookup, moderation/admin APIs.
- `services/match-coordinator`: pre-game multiplayer coordination, parties, party presence/chat, matchmaking, gameplay-node assignment, gameplay ticket minting.
- `services/realtime-gateway`: gameplay websocket routing to assigned nodes.
- `services/gameplay-node`: authoritative live match simulation.
- `services/moderation-worker`: background moderation projection and enforcement processing.
- `services/discord-worker`: Discord role/member synchronization and badge processing.
- `workers/storage-maintenance`: bounded storage maintenance.

## Auth And Tickets

- Browser auth is cookie-first: `services/api` owns the `HttpOnly` refresh/session cookie, OAuth, and session rotation
- App access JWTs are for API/queue/coordination auth. Gameplay tickets are per-match gameplay admission.

## State Stores

- PostgreSQL is the durable source of truth for users, identities, auth sessions, profiles, stats, ranks, maps/revisions, parties, chat, moderation, and persisted match data.
- Redis is for ephemeral coordination: queue/coordinator state, node assignment/liveness, pubsub/presence
- Shared wire/domain types live in `pkg/contracts`

## Route Semantics

- `/` is the main lobby/session launcher.
- `/match/[id]` is the canonical route for live reconnects and saved match/history views.
- `/players/[id]` is the public player profile route.
- API match bootstrap/session endpoints resolve existing match routes

## Browser Testing

If Chrome/Chromium is not found, use either Edge or Brave Browser.

## Documentation Map

Read the smallest relevant document before changing a subsystem:

- [`README.md`](README.md): repository overview, local quick start, production checklist, and documentation index.
- [`docs/architecture.md`](docs/architecture.md): runtime responsibilities, data ownership, end-to-end flows, scaling, draining, and retention.
- [`docs/development.md`](docs/development.md): local infrastructure, backend and web startup, tests, and k3d development.
- [`docs/deployment.md`](docs/deployment.md): Flux release flow, database migration procedure, and production verification. `geoduels-prod` repo is located in ../geoduels-prod, unless specified otherwise.
- [`apps/web/docs/frontend-architecture.md`](apps/web/docs/frontend-architecture.md): frontend module ownership, styling constraints, shared primitives, and complexity budgets.
- [`infra/k3s/README.md`](infra/k3s/README.md): Kubernetes base manifests and local cluster testing.
- [`apps/web/assets/source-map-thumbnails/README.md`](apps/web/assets/source-map-thumbnails/README.md): map thumbnail source and attribution requirements.
- [`CONTRIBUTOR_LICENSE_AGREEMENT.md`](CONTRIBUTOR_LICENSE_AGREEMENT.md): contributor licensing requirements.
