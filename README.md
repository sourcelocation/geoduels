<img src="web/public/icon.v1.png" height="64" />

# GeoDuels

A free multiplayer geography game with community maps and competitive duels. Play at [geoduels.io](https://geoduels.io/).

Not a pretty readme, but does the job.

## Running GeoDuels yourself

### Development

You need Docker, Go 1.26, and Node 22 (matching CI). All commands run from the repository root.

```sh
cp .env.example .env

cd backend
go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.30.0 generate
cd ..

docker compose -f compose.yaml -f compose.dev.yaml up -d --build

npm --prefix web ci
npm --prefix web run dev
```

Open `http://localhost:24000`. The Go services run under Air, rebuilt and restarted as you edit; Next's dev server takes the front door's port and passes `/api` to the API (port 24001), as production's ingress does. On startup the gameplay node imports the bundled sample dataset for any required playable map that is not configured yet (`DEV_MAP_DATASET` in `compose.dev.yaml`; it never replaces existing maps). Start the optional workers by adding `workers` to `COMPOSE_PROFILES` in `.env`; stop everything with `docker compose down`.

To run it as production does, from images, leave out `compose.dev.yaml` and Next: `docker compose up -d --build` serves the same address.

### Self-hosted

`compose.yaml` runs the published images (amd64 and arm64) with every port bound to host loopback. On the server, copy `.env.example` to `.env` and change what it marks "Self-hosting": `NEXT_PUBLIC_SITE_URL`, new secrets, `GEODUELS_VERSION` (for example `v2.2.7`), and `COMPOSE_PROFILES=workers,tls`, with which Caddy gets the certificate and serves ports 80 and 443. Behind a reverse proxy of your own (nginx, say), leave `tls` out and send everything to port 24000. Then:

```sh
docker compose up -d
```

The `db-migrate` service brings the database to the release's schema before anything that uses it starts.

To run your own changes, leave `GEODUELS_VERSION` empty and add `--build`, or build and push the images with `docker buildx bake` and point `REGISTRY` and `GEODUELS_VERSION` at them (see [development notes](docs/development.md)).

## Development references

- [AGENTS.md](AGENTS.md): constraints and cross-cutting behavior to preserve.
- [Development notes](docs/development.md): generated code, meaningful verification, local infrastructure, releases, and map tools.
- [Extension notes](extension/README.md): local installation and production packaging.
- [Contributor agreement](CONTRIBUTOR_LICENSE_AGREEMENT.md) and [license](LICENSE): contribution and licensing terms.
