#!/usr/bin/env bash
set -euo pipefail

usage() {
  echo "usage: $0 <migrate-command> [args...]"
  echo "example: MIGRATIONS_DB_URL='postgres://user:pass@127.0.0.1:5544/geoduels?sslmode=disable' $0 up"
}

if [ $# -lt 1 ]; then
  usage
  exit 1
fi

REPO_ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
DEFAULT_MIGRATIONS_DB_URL="postgres://geoduels:geoduels@127.0.0.1:5432/geoduels?sslmode=disable"
MIGRATIONS_DB_URL="${MIGRATIONS_DB_URL:-$DEFAULT_MIGRATIONS_DB_URL}"
MIGRATION_PATH="$REPO_ROOT/db/migrations"

DB_URL_FOR_CONTAINER="$MIGRATIONS_DB_URL"
DB_URL_FOR_CONTAINER="${DB_URL_FOR_CONTAINER//@127.0.0.1/@host.docker.internal}"
DB_URL_FOR_CONTAINER="${DB_URL_FOR_CONTAINER//@localhost/@host.docker.internal}"

# v2.0.2 ships only forward migrations after the published v2 baseline.
# Existing databases must have completed the v2.0.1 migration path first.
# Use a disposable psql container so this helper does not require psql on the
# host and does not print or otherwise expose the database URL.
current_version="$(docker run --rm --add-host=host.docker.internal:host-gateway postgres:16-alpine \
  psql "$DB_URL_FOR_CONTAINER" -X -Atqc "select coalesce(max(version), 0) from public.schema_migrations" 2>/dev/null || true)"

if ! [[ "$current_version" =~ ^[0-9]+$ ]] || [ "$current_version" -lt 2000 ]; then
  echo "database must be migrated to GeoDuels v2 schema version 2000 before applying this release" >&2
  echo "check out the v2.0.1 tag and run ./scripts/migrate.sh up to complete the v2 migration first" >&2
  exit 2
fi

exec docker run --rm \
  --add-host=host.docker.internal:host-gateway \
  -v "$MIGRATION_PATH:/migrations:ro" \
  migrate/migrate:v4.18.3 \
  -path=/migrations \
  -database "$DB_URL_FOR_CONTAINER" \
  "$@"
