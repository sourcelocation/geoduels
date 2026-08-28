# Backend refactor: preferences slice

The preferences vertical slice establishes the target boundaries for the backend migration:

`handler -> preferences application service -> consumer-owned store adapter -> sqlc queries`.

PostgreSQL statements for `user_preferences` live in `db/queries/preferences`; generated code is checked in under `pkg/persistence/sqlc/preferences` and is only used by the persistence adapter. The application service owns the supported schema version and revision-conflict policy, while the HTTP handler owns decoding and status mapping.

This is an incremental migration. Other files under `pkg/persistence` still contain legacy SQL and are not covered by this slice. Each subsequent domain should add named sqlc queries, a focused store capability, an application service, and focused tests before removing its legacy methods. The migration is complete only when an architecture check can forbid direct database calls outside generated sqlc adapters and the transaction infrastructure.

## Notification slice

Notification producers retain caller-owned transactions; persistence-internal helpers bind generated notification queries with `Queries.WithTx(tx)`, preserving atomic moderation, badge, and social workflows. Standalone inbox and outbox operations use generated notification queries behind `pkg/notifications.Service`.

## Auth-session foundation

Auth-session SQL is now named sqlc queries under `db/queries/auth_sessions`, with a persistence-owned `withTx` runner for create-plus-registration-IP updates. The API routes session creation, lookup, rotation, and revocation through `pkg/authsession.Service`; identity and account SQL remain outside this slice.

## Leaderboard slice

The leaderboard follows the same boundaries. Ranking, filtering, pagination, and active-season selection are named sqlc queries in `db/queries/leaderboard`; generated queries are initialized once on `pgStore`. `pkg/leaderboard` owns the consumer-facing application capability, and the API uses its adapter rather than persistence or generated types directly. Remaining persistence domains are intentionally still legacy and are not implied to be migrated by this slice.
