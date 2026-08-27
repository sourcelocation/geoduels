#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
container_name="geoduels-migrations-test-${RANDOM}-$$"
work_dir="$(mktemp -d)"

cleanup() {
  docker stop "$container_name" >/dev/null 2>&1 || true
  rm -rf "$work_dir"
}
trap cleanup EXIT

docker run --detach --rm \
  --name "$container_name" \
  --env POSTGRES_PASSWORD=geoduels \
  --env POSTGRES_USER=geoduels \
  --env POSTGRES_DB=v2 \
  postgres:16 >/dev/null

docker exec "$container_name" sh -c \
  'until pg_isready -U geoduels -d v2 >/dev/null 2>&1; do sleep 1; done'

run_migrations() {
  local database="$1"
  local migration_path="$2"
  shift 2

  docker run --rm \
    --network "container:$container_name" \
    --volume "$migration_path:/migrations:ro" \
    migrate/migrate:v4.18.3 \
    -path=/migrations \
    -database "postgres://geoduels:geoduels@127.0.0.1:5432/$database?sslmode=disable" \
    "$@"
}

run_migrations v2 "$repo_root/db/migrations" goto 2000

run_migrations v2 "$repo_root/db/migrations" up

version="$(docker exec "$container_name" psql -X -Atqc 'select version from schema_migrations' -U geoduels -d v2)"
if [ "$version" != "2001" ]; then
  echo "v2 forward migration ended at '$version', expected '2001'" >&2
  exit 1
fi

echo "v2 forward migrations apply cleanly from schema version 2000"
