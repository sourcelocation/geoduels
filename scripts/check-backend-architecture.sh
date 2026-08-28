#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

failed=0

report() {
  local title="$1"
  local matches="$2"
  if [[ -n "$matches" ]]; then
    echo "$title"
    echo "$matches"
    failed=1
  fi
}

production_go_files=()
while IFS= read -r file; do
  production_go_files+=("$file")
done < <(find services workers pkg tools -type f -name '*.go' \
  ! -name '*_test.go' \
  ! -path 'pkg/persistence/sqlc/*' | sort)

if ((${#production_go_files[@]})); then
  report "raw database execution is forbidden outside generated sqlc code:" \
    "$(rg -n '(\.pool|\btx|\bq|\bdb)\.(Query|QueryRow|Exec|Prepare)(Context)?\(' "${production_go_files[@]}" || true)"

  report "SQL statement literals are forbidden in production Go:" \
    "$(rg -ni '`[[:space:]]*(select|insert[[:space:]]+into|update[[:space:]]+[a-z_]|delete[[:space:]]+from|with[[:space:]]+[a-z_]+[[:space:]]+as)' "${production_go_files[@]}" || true)"
fi

domain_files=()
while IFS= read -r file; do
  domain_files+=("$file")
done < <(find pkg -mindepth 2 -maxdepth 2 -type f -name '*.go' \
  ! -name '*_test.go' \
  ! -path 'pkg/persistence/*' \
  ! -path 'pkg/controlplane/*' | sort)

if ((${#domain_files[@]})); then
  report "domain packages may not import concrete persistence or database drivers:" \
    "$(rg -n '"geoduels/pkg/persistence|"github.com/jackc/pgx|"database/sql' "${domain_files[@]}" || true)"
fi

handler_files=()
while IFS= read -r file; do
  handler_files+=("$file")
done < <(find services -type f \( -name '*handler*.go' -o -name '*handlers*.go' \) ! -name '*_test.go' | sort)

if ((${#handler_files[@]})); then
  report "handlers may not access a database store directly:" \
    "$(rg -n '\.(store|Store)\.|persistence\.|pgx\.' "${handler_files[@]}" || true)"
fi

report "the aggregate persistence.Store application interface must be removed:" \
  "$( { rg -n 'persistence\.Store' services workers pkg --glob '*.go' --glob '!**/*_test.go' --glob '!pkg/persistence/sqlc/**' || true; rg -n 'type Store interface[[:space:]]*\{' pkg/persistence --glob '*.go' --glob '!**/*_test.go' --glob '!pkg/persistence/sqlc/**' || true; } )"

if ((failed)); then
  exit 1
fi

echo "backend architecture checks passed"
