#!/usr/bin/env bash
set -euo pipefail

if [ "${CONFIRM_STORAGE_COMPACTION:-}" != "yes" ]; then
  echo "refusing to rewrite tables without CONFIRM_STORAGE_COMPACTION=yes"
  exit 1
fi

DB_URL="${MIGRATIONS_DB_URL:-postgres://geoduels:geoduels@127.0.0.1:5432/geoduels?sslmode=disable}"

# Migration 42 is intentionally available only through the historical
# migration path; the v2 fresh-install schema starts at version 2000.
version="$(psql "$DB_URL" -X -Atqc "select version from schema_migrations")"
if [ "$version" != "42" ]; then
  echo "storage compaction requires legacy schema migration 42; current version: $version"
  echo "on a version-41 database, apply it with: ./scripts/migrate.sh --legacy up 1"
  exit 1
fi

psql "$DB_URL" -X -v ON_ERROR_STOP=1 <<'SQL'
vacuum (full, analyze) match_history;
vacuum (full, analyze) locations;
vacuum (full, analyze) ranked_guess_events;
vacuum (full, analyze) match_players;
vacuum (full, analyze) chat_messages;
vacuum (full, analyze) runtime_matches;
vacuum (full, analyze) auth_sessions;
vacuum (full, analyze) user_identity_history;
SQL

echo "storage compaction complete"
