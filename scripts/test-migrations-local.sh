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
  --env POSTGRES_DB=fresh \
  postgres:16 >/dev/null

docker exec "$container_name" sh -c \
  'until pg_isready -U geoduels -d fresh >/dev/null 2>&1; do sleep 1; done'
docker exec "$container_name" createdb -U geoduels upgraded

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

run_migrations fresh "$repo_root/db/migrations" up
run_migrations upgraded "$repo_root/db/migrations-legacy" goto 62

# Exercise the version-2000 legacy cutover, not only its empty-schema shape.
docker exec --interactive "$container_name" psql -X -v ON_ERROR_STOP=1 -U geoduels -d upgraded <<'SQL'
-- Some pre-v2 installations have empty-string defaults from an older schema.
-- The enum migration must treat these as nullable team assignments.
alter table party_members alter column team_id set default '';
alter table chat_messages alter column team_id set default '';

insert into users (id, display_name, selected_badge_code, selected_badge_season_id)
values ('00000000-0000-0000-0000-000000000064', 'MigrationFixture', 10, 's1');

insert into user_badges (user_id, badge_code, badge_season_id, rank)
values ('00000000-0000-0000-0000-000000000064', 10, 's1', 7);
SQL

run_migrations upgraded "$repo_root/db/migrations-legacy" up
run_migrations upgraded "$repo_root/db/migrations" up

converted_badge="$(docker exec "$container_name" psql -X -Atqc \
  "select level || ':' || extra from user_badges where user_id = '00000000-0000-0000-0000-000000000064'" \
  -U geoduels -d upgraded)"
if [ "$converted_badge" != "1:7" ]; then
  echo "legacy badge conversion produced '$converted_badge', expected '1:7'" >&2
  exit 1
fi

for database in fresh upgraded; do
  docker exec "$container_name" pg_dump \
    -U geoduels \
    -d "$database" \
    --schema-only \
    --no-owner \
    --no-privileges \
    --exclude-table=schema_migrations \
    >"$work_dir/$database.sql"

  sed -E \
    -e '/^\\restrict /d' \
    -e '/^\\unrestrict /d' \
    "$work_dir/$database.sql" >"$work_dir/$database.normalized.sql"
done

diff --unified \
  "$work_dir/fresh.normalized.sql" \
  "$work_dir/upgraded.normalized.sql"

echo "fresh and legacy-upgrade migration paths produce equivalent schemas"
