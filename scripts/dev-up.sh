#!/usr/bin/env bash
set -euo pipefail

project_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$project_dir"

docker compose up -d postgres redis
./scripts/migrate.sh up
./scripts/bootstrap-dev-data.sh
docker compose up -d gameplay-node match-coordinator realtime-gateway api
