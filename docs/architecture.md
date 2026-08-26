# Current Architecture

This document describes the current runtime implemented in this repository. Older "runtime-core" / NATS notes are historical and should not be treated as the source of truth for the running system.

## Runtime roles

- `apps/web`: Next.js browser client and route shell.
- `services/api`: browser auth/session rotation, self and public profiles, leaderboard, maps, singleplayer launch, match-route lookup/history, moderation/admin, content, and support APIs.
- `services/match-coordinator`: duel queue websocket endpoint, parties, party presence/chat coordination, gameplay-node assignment, online count, and maintenance exposure.
- `services/realtime-gateway`: public websocket gateway for `/ws/{node}`.
- `services/gameplay-node`: authoritative in-memory duel and singleplayer execution for assigned matches.
- `services/moderation-worker`: background moderation signal notifications and risk-engine work.
- private `risk-engine`, when deployed: opaque ranked-integrity signal generation for moderation.
- `services/discord-worker`: Discord membership/role reconciliation and badge synchronization.
- `workers/storage-maintenance`: bounded replay compression and retention cleanup.

## Data ownership

- PostgreSQL is the durable source of truth for users, identities, sessions, profiles, stats, ranks, maps and their current location datasets, parties, chat, moderation, runtime match metadata, compact match summaries, and retained replays.
- Current bans and chat/report mutes are projected directly on users. `moderation_signals` stores reports and detector observations; the append-only `moderation_log` stores moderator and system actions.
- PostgreSQL also owns friendships, friend requests and blocks, expiring friend codes, targeted party invitations, notification history, last-seen projections, and sequenced user events.
- Redis is used for queue state, gameplay-node registration, route assignment, ephemeral party/global presence, user-event fanout, and maintenance status.
- `pkg/persistence` owns Postgres persistence behavior.
- `pkg/coordinator` owns Redis-backed node registration, assignment, and presence.
- `pkg/duel` and `pkg/singleplayer` own match rules and round progression.
- `pkg/gameticket` issues short-lived gameplay admission tickets.

## Entity identifiers

- Durable users, auth sessions, matches, maps, parties, comments, chat conversations, and chat messages use PostgreSQL `uuid` keys.
- New keys are UUIDv7 for index locality. Chat conversation keys are deterministic UUIDs derived from their canonical UUID-backed scope.
- Service contracts, JWT subjects, Redis values, and websocket payloads carry canonical UUID strings.
- Browser routes render UUIDs as reversible 26-character Crockford Base32 values. Human map slugs and party invite codes remain text.
- Legacy entity aliases are no longer resolved by application or database code.
- Match mode is explicit state; identifier prefixes are not used to determine gameplay behavior.
- Text fields ending in `_id` are reserved for external identifiers or semantic keys; internal entity and foreign keys use `uuid`.

## End-to-end flows

### Auth

1. Browser loads `apps/web`.
2. Web restores auth through a single `GET /v1/auth/session` bootstrap. No refresh cookie is an anonymous viewer; it does not create an account.
3. Playable actions (singleplayer, queue, party) mint a guest through `POST /v1/auth/guest` when no session exists, after Turnstile when configured.
4. `services/api` validates the `HttpOnly` refresh-session cookie and returns a short-lived app access JWT without rotating the refresh token.
5. Explicit refresh through `POST /v1/auth/refresh` rotates the refresh token and returns a new app access JWT.
6. The browser keeps the access JWT in memory only.

### Duel

1. Browser opens websocket matchmaking to `services/match-coordinator` at `/queue`.
2. `match-coordinator` authenticates the app JWT, manages queue state, and selects a gameplay node.
3. `match-coordinator` reads the selected map's current locations under a shared lock, persists a deterministic round plan, and creates the match on the chosen `gameplay-node`.
4. `match-coordinator` returns match assignment plus a short-lived gameplay ticket.
5. Browser connects to `/ws/{node}` through `services/realtime-gateway`.
6. `realtime-gateway` resolves the registered gameplay pod for that route and proxies the websocket to the exact `gameplay-node`.
7. `gameplay-node` runs the authoritative match loop and broadcasts snapshots.

Live match sessions also carry a durable PostgreSQL lease. The owning gameplay
node renews that lease in a single bounded batch. Expired leases are reconciled
by the API maintenance loop and become `ended`. Redis assignment and presence
keys expire through their existing TTLs and are never the durable match-state
authority.

### Singleplayer

1. Browser asks `services/api` to start a singleplayer session.
2. `api` selects a non-draining gameplay node and creates the match there.
3. `api` returns assignment plus gameplay ticket.
4. Browser connects through `realtime-gateway` to the assigned node.

### Parties

1. Authenticated browsers create or join parties through `services/match-coordinator`.
2. The coordinator persists durable party membership/configuration in PostgreSQL and uses websocket presence for live updates.
3. Party chat uses the durable `party` conversation scope and remains available while a party-sourced match is active.
4. The party owner selects a permitted ready map and starts the configured duel, team duel, or free-for-all match.
5. Match session/history responses retain the source party identifiers so players can return to the party.

### Friends, presence, and notifications

1. Registered users manage durable friend requests, friendships, blocks, friend codes, party invitations, and notification history through `services/api`.
2. Each signed-in browser keeps one global user-events websocket separate from queue, party, and gameplay sockets.
3. Redis stores per-connection presence with TTLs so multiple tabs and devices collapse into one online/away state.
4. PostgreSQL stores coarse `last_seen_at` transitions rather than heartbeat writes.
5. Durable user-scoped event sequences let reconnecting clients detect gaps and reconcile authoritative HTTP queries.
6. `match-coordinator` remains the authority for accepting party membership; social party invitations resolve to the existing party join flow.
7. Authenticated guest sockets receive only public global-status events. Global sockets refresh the coordinator presence set, while each API instance reads the aggregate online count and maintenance state once every ten seconds and fans changes out to its local sockets.
8. Signed-out pages use the API's slow `/v1/status` fallback; lobby pages do not run their former per-browser `/queue/online` polling loop.

### Match route bootstrap

- `/match/[id]` is the canonical route for a specific match.
- `/players/[id]` is the public profile route for public-safe rating, badge, and recent match-history data.
- `/settings/account` owns private nickname, badge-selection, sign-in-provider, logout, and account-deletion controls.
- Cold loads resolve through `GET /v1/matches/{id}/bootstrap`.
- Already-authenticated refreshes can resolve through `GET /v1/matches/{id}/session`.
- The route can resolve to:
  - live reconnect with a fresh gameplay ticket
  - final match history
  - replaced
  - forbidden
  - missing

## Routing and scaling boundaries

- `match-coordinator` is responsible for duel creation and assignment.
- `api` is responsible for browser identity/session ownership, singleplayer creation, durable content APIs, and route bootstrap/session endpoints.
- `realtime-gateway` is only a routing/proxy layer.
- `gameplay-node` is the in-memory authority for the matches assigned to it.
- `moderation-worker` and `discord-worker` process background work independently from latency-sensitive request and match paths.
- `moderation-worker` calls the private risk engine asynchronously after ranked match facts are persisted. Risk-engine outages must not block live gameplay, reports, or manual moderation.
- Queueing and realtime simulation are intentionally separate scaling boundaries.

## Reconnect and session model

- Browser auth and gameplay admission are intentionally separate:
  - app JWT for API and queue access
  - gameplay ticket for a specific match/node websocket connection
- Disconnects are handled by the match runtime on the gameplay node.
- Reconnect flows recover through assignment lookup plus a newly minted gameplay ticket.

## Maintenance and drain behavior

- Gameplay nodes register themselves in Redis with:
  - public websocket route
  - internal pod URL
  - active match count
  - draining flag
- During shutdown, a gameplay node marks itself draining, stops accepting new match creation, and waits for active matches to finish before exit.
- `match-coordinator` and `api` both exclude draining gameplay nodes from new assignment decisions.
- `realtime-gateway` stops accepting new websocket upgrades during shutdown and waits for active proxied sockets to drain.
- `api`, `match-coordinator`, `realtime-gateway`, and `gameplay-node` all fail readiness while draining so Kubernetes can remove them from rotation.

## Maintenance status

- Redis key `system:maintenance` stores optional maintenance status.
- `match-coordinator` exposes that status through `/queue/online`.
- `queuePaused` blocks new duel queueing.
- `playPaused` blocks all new play session creation.
- The web lobby renders:
  - a warning banner when `phase=warning`
  - a blocking overlay when `phase=active`

## Location pipeline

- Custom-map JSON is streamed through the API, validated, and atomically replaces the map's current normalized PostgreSQL location dataset.
- Map upload quotas are derived from the creator's base/trusted/established trust tier. Trust uses account age and qualified community favorites/maps; moderation restrictions lower the effective tier.
- Match launch locks the selected map while reading its current dataset, then persists the full bounded round plan before assigning a gameplay node. Existing matches therefore remain independent of later map uploads.
- `gameplay-node` consumes the supplied in-memory plan and does not query or preload map catalogs.
- Redis is not used for map contents, lobby map settings, or per-match location deduplication.

## Match persistence and retention

- A terminal gameplay snapshot is not broadcast as `ended` until PostgreSQL atomically commits match history, participant projections, rating/stat updates, match-session completion, runtime completion, and party reopening.
- Duel finalization computes one explicit, stable-slot result (`player 1 win`,
  `player 2 win`, or `draw`) before persistence. UUIDs are parsed at that
  boundary, and stat writes consume per-player boolean win facts rather than
  empty-string winner sentinels.
- Finalization is attempted once as one idempotent transaction. Gameplay nodes
  do not own retry timers or repeat failed writes.
- The committed snapshot returned to the gameplay node contains authoritative post-match ratings. Redis assignment cleanup happens only after that snapshot is broadcast.
- Compact match summaries and participant projections are retained durably for profiles, rankings, moderation, and match lists.
- Full replay snapshots are Zstandard-compressed when written and retained for 30 days by default.
- Replays referenced by moderation signals are pinned by clearing their expiration.
- `workers/storage-maintenance` compresses legacy JSON replays and clears expired replay payloads without deleting compact match history.
