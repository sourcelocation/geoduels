# k3s Manifests

This directory contains reusable and local-development Kubernetes manifests.

## Layout

- `base/`: reusable manifests for app workloads and services
- `overlays/k3d/`: local multi-node k3d test overlay

Production cluster state, Flux bootstrap manifests, real ingress hosts, TLS issuers, and production runtime config live in the private ops repository.

## Local k3d Scaling Test

Use `infra/k3s/overlays/k3d` to exercise horizontal `gameplay-node` routing on a local 3-node cluster.

Requirements:

- k3d installed locally
- Docker running
- local PostgreSQL and Redis reachable from the cluster through `host.k3d.internal`
- locally built images imported into the k3d cluster, or equivalent registry access
- all database migrations applied before application workloads start

Recommended cluster shape:

```bash
k3d cluster create geoduels \
  --servers 1 \
  --agents 3 \
  --port "80:80@loadbalancer"
```

Create a local secret from `infra/k3s/overlays/k3d/secrets.env.example` after filling in local values:

```bash
kubectl create namespace geoduels
kubectl -n geoduels create secret generic geoduels-secrets \
  --from-env-file=infra/k3s/overlays/k3d/secrets.env.example
```

The overlay adds the `ghcr-creds` image pull secret to workloads. When pulling private GHCR images, create it before applying the overlay:

```bash
kubectl -n geoduels create secret docker-registry ghcr-creds \
  --docker-server=ghcr.io \
  --docker-username='<github-user>' \
  --docker-password='<github-token>'
```

For a fully local image test, build/tag images with the exact names referenced by the manifests, import them with `k3d image import -c geoduels ...`, and remove the `imagePullSecrets` patches from a local copy of the overlay. The base also deploys `discord-worker`; either import/tag `ghcr.io/sourcelocation/geoduels-discord-worker:latest`, provide registry access, or remove that deployment for a test that does not exercise Discord integration.

Apply migrations to the host PostgreSQL before starting the Kubernetes workloads:

```bash
MIGRATIONS_DB_URL='postgres://geoduels:geoduels@127.0.0.1:5432/geoduels?sslmode=disable' \
./scripts/migrate.sh up
```

For a pre-v2 database, check out the `v2.0.1` tag and complete its migration
path before returning to this release.

Apply the overlay:

```bash
kubectl apply -k infra/k3s/overlays/k3d
```

The base manifests run PgBouncer inside the cluster and point DB-using
workloads at `pgbouncer:6432`. The `geoduels-secrets` secret must include
`PGBOUNCER_POSTGRES_HOST`, `PGBOUNCER_POSTGRES_PORT`, `PGBOUNCER_POSTGRES_DB`,
`PGBOUNCER_POSTGRES_USER`, and `PGBOUNCER_POSTGRES_PASSWORD` for PgBouncer's
upstream direct Postgres connection.

After applying, verify the API, coordinator, realtime gateway, gameplay nodes, moderation worker, and Discord worker with their `/health/ready` endpoints or Kubernetes readiness status.
