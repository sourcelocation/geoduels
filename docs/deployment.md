# Deployment Model

Production deployment state is intentionally kept outside this public source repository.

This repository owns:

- application source
- reusable Kubernetes base manifests in `infra/k3s/base`
- local k3d manifests in `infra/k3s/overlays/k3d`
- release image builds

The private ops repository owns:

- Flux bootstrap and cluster sync state
- production overlays
- production runtime config
- encrypted or externally managed secret references
- image tag pins consumed by Flux

## Release Flow

1. Merge application changes to `main`.
2. Push a version tag such as `v1.2.3`.
3. GitHub Actions compiles the backend services with one shared Go cache while
   a dedicated runner builds the web image, then pushes the versioned images.
4. The release workflow opens a PR in the private ops repository.
5. Merge the ops PR.
6. Flux reconciles production from the private ops repository.

The public release workflow needs `OPS_REPO_TOKEN` with access to the private ops repository. Optional repository variables:

- `OPS_REPOSITORY`, default `sourcelocation/geoduels-prod`
- `REGISTRY`, default `ghcr.io/<repository-owner>`
- `PRODUCTION_SITE_URL`, default `https://geoduels.io`

## Database Migrations

The release workflow does not apply PostgreSQL migrations. Migration execution is an explicit production operation owned by the private ops process.

For each release:

1. Review every migration added since the currently deployed schema version and its application compatibility window.
2. Back up PostgreSQL and confirm sufficient free disk space before table rewrites or `VACUUM FULL`.
3. Pause writes or enter maintenance mode when a migration requires it.
4. Apply migrations with the release's `db/migrations` directory before rolling out code that requires the new schema. This release requires the database to already be at v2 schema version 2000 or later. For an older database, check out the `v2.0.1` tag and complete its migration path first.
5. Run database and API smoke tests, then allow Flux to roll out the new images.
6. Roll back application images only when the previous application is compatible with the migrated schema. Correct schema problems with a new forward migration.

Migration 42 is a historical staged operation. `scripts/compact-storage.sh` only runs while the schema version is exactly `42`; use the `v2.0.1` tag for its historical instructions. A database already beyond version 42 requires a separately planned PostgreSQL maintenance operation.

Migration 47 rewrites entity primary keys, foreign keys, and their indexes from
`text` to `uuid`. Put the application in maintenance mode, stop all writers,
take a verified backup, apply the migration, and deploy UUID-compatible
application images before reopening traffic. The migration preserves old route
identifiers in `legacy_id_aliases` for the original compatibility window.
Migration 61 removes that expired compatibility table. Drain active matches first and clear stale queue,
assignment, and presence data from Redis before traffic resumes.
When migrations 42 and 47 are performed in the same maintenance window, keep
all writers stopped from storage compaction through the UUID migration and
deploy the UUID-compatible application before reopening traffic.

Migration 55 adds match-session leases. Apply it before deploying the API and
gameplay-node changes that renew and reconcile those leases. Existing live
sessions receive a 30-minute initial lease so a rolling deployment does not
immediately classify them as stale. Drain active matches where practical,
apply the migration, then roll out the gameplay nodes and API within that
window. Monitor database write failures during the rollout. Do not shorten
`MATCH_SESSION_STALE_GRACE` below the expected
deployment and transient database-recovery time without reviewing the
abandonment behavior.

Migration 59 adds the durable matchmaker fencing lease. Apply it before an
image that uses the updated coordinator. A standby coordinator will report
queue unavailability until it acquires the lease; this is expected and avoids
two active queue consumers during rollout. See
[`docs/backend-rework.md`](backend-rework.md) for the failover verification
procedure.

## Post-Deploy Checks

- `/health`, `/health/live`, and `/health/ready` for API and all deployed workers/services
- risk-engine health, when enabled in the private production overlay: `/health/ready`
- browser auth bootstrap and refresh
- queue join, assignment, realtime websocket connection, and match completion
- public match history and player profile routes
- map browse/upload flow when map-related migrations changed

## Private Risk Engine

Live moderation detector thresholds and scoring are deployed as a private
`geoduels-risk-engine` service. This public repository only owns the narrow
client integration and persisted opaque moderation signals.

Production overlays in `geoduels-prod` should provide:

- `RISK_ENGINE_URL`, usually `http://risk-engine:8096`
- `RISK_ENGINE_TOKEN`
- `RISK_ENGINE_TIMEOUT`, default-compatible value `750ms`

Keep the risk engine off the live gameplay path. The moderation worker calls it
asynchronously after ranked match facts have been persisted; player reports and
manual moderation continue to work when the risk engine is disabled or down.
