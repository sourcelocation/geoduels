# Backend rework and cutover rules

The backend is being simplified through vertical, compatible cutovers rather
than a parallel rewrite. PostgreSQL remains durable authority; Redis remains
limited to ephemeral presence, routing, and queue transport. The realtime
gateway and gameplay node remain independent scaling/failure boundaries.

## Control plane roles

The control-plane composition has three roles:

- `serve`: browser HTTP and websocket endpoints.
- `matchmaker`: the one active queue consumer and launch coordinator.
- `jobs`: bounded asynchronous work such as cleanup and outbox processing.

Roles can temporarily be co-located, but their startup ownership must remain
explicit. External calls and retrying jobs must never run in the request
process by accident. `pkg/controlplane` is the shared composition boundary;
domain packages must not import service `main` packages.

`match-coordinator` now obtains the durable `matchmaker` lease in
`control_plane_leases` before accepting queue websocket upgrades or executing
queue ticks. Standby coordinators remain alive but return 503 for queue work.
The lease has a fencing token; future launch writes must persist that token in
the same transaction as their terminal/idempotency transition before Redis Lua
launch code can be removed.

Apply legacy migration 59 (via `./scripts/migrate.sh --legacy`) before deploying this change. Use a lease TTL comfortably
larger than normal PostgreSQL failover latency (`MATCHMAKER_LEASE_TTL`, default
15 seconds). Verify failover by stopping the active coordinator, waiting one
TTL, then checking that exactly one standby begins accepting `/queue`.

## Persistence rules

`sqlc.yaml` is the generated-query entry point. Each query module owns a small
current-schema projection under `db/schema/`, because historical migrations
can use PostgreSQL catalog operations that sqlc intentionally does not parse.
Generated files are committed and CI should regenerate/check them whenever a
query module changes.

New use cases define the narrow repository interfaces they consume. They take
a caller-provided `context.Context`, use typed nullable values, and perform a
single transaction at the use-case boundary. Do not extend
`persistence.Store`; it is a legacy adapter to be reduced by moving one
vertical slice at a time.

## Match persistence cutover

The target durable model is `matches`, `match_participants`, and separately
retained `match_replays`. This is not an in-place rename of historical tables:

1. expand with new tables and dual-read-compatible writers;
2. backfill compact history and verify counts/checksums;
3. switch readers, then writers, while retaining the prior tables;
4. contract only after a release window and replay/moderation retention have
   elapsed.

`match_history` remains available during the compatibility window. Never put
large replay payloads or high-frequency leases on the same hot lifecycle row.

## Contracts and gameplay

Public route and websocket payload changes require a versioned contract change
and compatibility test. Additive fields are preferred; removals require an
advertised deprecation period. Gameplay commands must be idempotent and match
actors must use injected clocks and RNGs so recorded command streams can be
replayed in deterministic parity tests. Competitive and solo state machines
may differ while sharing only the genuinely common scoring/round primitives.
