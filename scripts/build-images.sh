#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

REGISTRY="${REGISTRY:-ghcr.io/sourcelocation}"
TAG="${TAG:-latest}"
PLATFORMS="${PLATFORMS:-linux/arm64}"
BUILDER="${BUILDER:-}"
PARALLEL="${PARALLEL:-2}"
OUTPUT_MODE="${OUTPUT_MODE:-load}"
IMPORT_TO_K3D="${IMPORT_TO_K3D:-0}"
K3D_CLUSTER="${K3D_CLUSTER:-geoduels}"
APP_VERSION="${APP_VERSION:-$TAG}"
GIT_SHA="${GIT_SHA:-$(git rev-parse --short HEAD)}"

if ! docker buildx version >/dev/null 2>&1; then
  echo "docker buildx is required"
  exit 1
fi

if ! [[ "$PARALLEL" =~ ^[0-9]+$ ]] || [ "$PARALLEL" -lt 1 ]; then
  echo "PARALLEL must be a positive integer"
  exit 1
fi

if [[ "$PLATFORMS" == *,* ]] && [[ "$OUTPUT_MODE" == "load" ]]; then
  echo "OUTPUT_MODE=load only supports a single platform; got PLATFORMS=$PLATFORMS"
  exit 1
fi

case "$OUTPUT_MODE" in
  load)
    output_args=(--load)
    ;;
  push)
    output_args=(--push)
    ;;
  *)
    echo "OUTPUT_MODE must be 'load' or 'push'"
    exit 1
    ;;
esac

build_image() {
  local dockerfile="$1"
  local image="$2"
  local context="$3"
  local kind="${4:-generic}"
  local ref="$REGISTRY/$image:$TAG"
  local -a extra_args=()
  local -a cmd

  echo "[$image] building ($PLATFORMS)"

  if [ "$kind" = "web" ]; then
    extra_args+=(
      --build-arg "NEXT_PUBLIC_APP_VERSION=$APP_VERSION"
      --build-arg "NEXT_PUBLIC_GIT_SHA=$GIT_SHA"
    )
  fi

  cmd=(docker buildx build)
  if [ -n "$BUILDER" ]; then
    cmd+=(--builder "$BUILDER")
  fi
  cmd+=(--platform "$PLATFORMS")
  cmd+=("${output_args[@]}")
  if [ "${#extra_args[@]}" -gt 0 ]; then
    cmd+=("${extra_args[@]}")
  fi
  cmd+=(-f "$dockerfile" -t "$ref" "$context")

  "${cmd[@]}"

  echo "[$image] done"
}

launch_build() {
  local dockerfile="$1"
  local image="$2"
  local context="$3"
  local kind="${4:-generic}"
  build_image "$dockerfile" "$image" "$context" "$kind" &
  pids+=("$!")
  labels+=("$image")
}

wait_first_job() {
  local pid="${pids[0]}"
  local label="${labels[0]}"

  if ! wait "$pid"; then
    echo "[$label] failed"
    failed=1
  fi

  pids=("${pids[@]:1}")
  labels=("${labels[@]:1}")
}

# dockerfile|image|context
builds=(
  "services/api/Dockerfile|geoduels-api|.|generic"
  "services/discord-worker/Dockerfile|geoduels-discord-worker|.|generic"
  "services/match-coordinator/Dockerfile|geoduels-match-coordinator|.|generic"
  "services/moderation-worker/Dockerfile|geoduels-moderation-worker|.|generic"
  "services/realtime-gateway/Dockerfile|geoduels-realtime-gateway|.|generic"
  "services/gameplay-node/Dockerfile|geoduels-gameplay-node|.|generic"
  "apps/web/Dockerfile|geoduels-web|apps/web|web"
)

pids=()
labels=()
failed=0

for spec in "${builds[@]}"; do
  IFS='|' read -r dockerfile image context kind <<<"$spec"

  if [ "$failed" -ne 0 ]; then
    break
  fi

  launch_build "$dockerfile" "$image" "$context" "$kind"

  if [ "${#pids[@]}" -ge "$PARALLEL" ]; then
    wait_first_job
  fi
done

while [ "${#pids[@]}" -gt 0 ]; do
  wait_first_job
done

if [ "$failed" -ne 0 ]; then
  exit 1
fi

if [ "$IMPORT_TO_K3D" = "1" ]; then
  if ! command -v k3d >/dev/null 2>&1; then
    echo "k3d is required when IMPORT_TO_K3D=1"
    exit 1
  fi

  refs=()
  for spec in "${builds[@]}"; do
    IFS='|' read -r _ image _ _ <<<"$spec"
    refs+=("$REGISTRY/$image:$TAG")
  done

  echo "[k3d] importing images into cluster '$K3D_CLUSTER'"
  k3d image import "${refs[@]}" -c "$K3D_CLUSTER"
  echo "[k3d] import complete"
fi
