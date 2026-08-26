#!/usr/bin/env bash
set -euo pipefail

usage() {
  echo "usage: $0 [--legacy] <migrate-command> [args...]"
  echo "example: MIGRATIONS_DB_URL='postgres://user:pass@127.0.0.1:5544/geoduels?sslmode=disable' $0 up"
  echo "legacy example: MIGRATIONS_DB_URL='postgres://user:pass@127.0.0.1:5544/geoduels?sslmode=disable' $0 --legacy up"
}

if [ $# -lt 1 ]; then
  usage
  exit 1
fi

REPO_ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
DEFAULT_MIGRATIONS_DB_URL="postgres://geoduels:geoduels@127.0.0.1:5432/geoduels?sslmode=disable"
MIGRATIONS_DB_URL="${MIGRATIONS_DB_URL:-$DEFAULT_MIGRATIONS_DB_URL}"
MIGRATION_PATH="$REPO_ROOT/db/migrations"
LEGACY=0

if [ "${1:-}" = "--legacy" ]; then
  LEGACY=1
  MIGRATION_PATH="$REPO_ROOT/db/migrations-legacy"
  shift
fi

if [ $# -lt 1 ]; then
  usage
  exit 1
fi

DB_URL_FOR_CONTAINER="$MIGRATIONS_DB_URL"
DB_URL_FOR_CONTAINER="${DB_URL_FOR_CONTAINER//@127.0.0.1/@host.docker.internal}"
DB_URL_FOR_CONTAINER="${DB_URL_FOR_CONTAINER//@localhost/@host.docker.internal}"

# The v2 path contains only the clean version-2000 schema and future v2
# migrations. Never let it run that baseline against a pre-v2 installation.
# Use a disposable psql container so this helper does not require psql on the
# host and does not print or otherwise expose the database URL.
if [ "$LEGACY" -eq 0 ]; then
  schema_migrations_table="$(docker run --rm --add-host=host.docker.internal:host-gateway postgres:16-alpine \
    psql "$DB_URL_FOR_CONTAINER" -X -Atqc "select to_regclass('public.schema_migrations')" 2>/dev/null)"

  public_object_count() {
    local query
    if [ -n "$schema_migrations_table" ]; then
      query="
        with objects as (
          select c.oid
          from pg_catalog.pg_class c
          join pg_catalog.pg_namespace n on n.oid = c.relnamespace
          where n.nspname = 'public'
            and c.relkind in ('r', 'p', 'v', 'm', 'f', 'S')
            and c.oid <> 'public.schema_migrations'::regclass
            and not exists (
              select 1
              from pg_catalog.pg_depend d
              where d.objid = c.oid
                and d.refobjid = 'public.schema_migrations'::regclass
            )
          union all
          select t.oid
          from pg_catalog.pg_type t
          join pg_catalog.pg_namespace n on n.oid = t.typnamespace
          where n.nspname = 'public'
            and t.typtype in ('b', 'c', 'd', 'e', 'r', 'm')
            and t.typname not in ('schema_migrations', '_schema_migrations')
          union all
          select p.oid
          from pg_catalog.pg_proc p
          join pg_catalog.pg_namespace n on n.oid = p.pronamespace
          where n.nspname = 'public'
        )
        select count(*) from objects
      "
    else
      query="
        with objects as (
          select c.oid
          from pg_catalog.pg_class c
          join pg_catalog.pg_namespace n on n.oid = c.relnamespace
          where n.nspname = 'public'
            and c.relkind in ('r', 'p', 'v', 'm', 'f', 'S')
          union all
          select t.oid
          from pg_catalog.pg_type t
          join pg_catalog.pg_namespace n on n.oid = t.typnamespace
          where n.nspname = 'public'
            and t.typtype in ('b', 'c', 'd', 'e', 'r', 'm')
          union all
          select p.oid
          from pg_catalog.pg_proc p
          join pg_catalog.pg_namespace n on n.oid = p.pronamespace
          where n.nspname = 'public'
        )
        select count(*) from objects
      "
    fi
    docker run --rm --add-host=host.docker.internal:host-gateway postgres:16-alpine \
      psql "$DB_URL_FOR_CONTAINER" -X -Atqc "$query" 2>/dev/null
  }

  if [ -n "$schema_migrations_table" ]; then
    current_version="$(docker run --rm --add-host=host.docker.internal:host-gateway postgres:16-alpine \
      psql "$DB_URL_FOR_CONTAINER" -X -Atqc 'select coalesce(max(version), 0) from public.schema_migrations' 2>/dev/null)"
    if [ -n "$current_version" ] && [ "$current_version" -gt 0 ] && [ "$current_version" -lt 2000 ]; then
      echo "database is at legacy schema version $current_version; rerun with --legacy to continue the historical migrations" >&2
      exit 2
    fi
    if [ "${current_version:-0}" = "0" ]; then
      if [ "$(public_object_count)" -gt 0 ]; then
        echo "database has public schema objects but migration state is version 0; refusing the v2 baseline (inspect it and use --legacy if appropriate)" >&2
        exit 2
      fi
    fi
  else
    if [ "$(public_object_count)" -gt 0 ]; then
      echo "database has public schema objects but no migration state; refusing the v2 baseline (inspect it and use --legacy if appropriate)" >&2
      exit 2
    fi
  fi
fi

exec docker run --rm \
  --add-host=host.docker.internal:host-gateway \
  -v "$MIGRATION_PATH:/migrations:ro" \
  migrate/migrate:v4.18.3 \
  -path=/migrations \
  -database "$DB_URL_FOR_CONTAINER" \
  "$@"
