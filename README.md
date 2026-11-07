<img src="web/public/icon.v1.png" height="64" />

# GeoDuels

A free multiplayer geography game with community maps and competitive duels. Play at [geoduels.io](https://geoduels.io/).

Not a pretty readme, but does the job.

## Running GeoDuels yourself

### Development

You need Docker, Go 1.26, Node 22 (matching CI), and [Overmind](https://github.com/DarthSim/overmind) (`brew install overmind`). All commands run from the repository root.

```sh
cp .env.example .env

cd backend
go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.30.0 generate
cd ..

docker compose up -d
npm --prefix web ci
overmind start
```

Open `http://localhost:24000`. Docker runs only Postgres and NATS; the `Procfile` runs everything else on your machine, with the Go services under Air, rebuilt and restarted as you edit. Next's dev server takes the front door's port and routes `/api` as production's ingress does. See [development notes](docs/development.md#local-infrastructure).

To run it as production does, from images, stop Overmind and run `COMPOSE_PROFILES=app docker compose up -d --build`, which serves the same address.

### Self-hosted

`compose.yaml` runs the published images (amd64 and arm64) with every port bound to host loopback. On the server, copy `.env.example` to `.env` and change what it marks "Self-hosting": `NEXT_PUBLIC_SITE_URL`, new secrets, `GEODUELS_VERSION` (for example `v2.2.7`), and `COMPOSE_PROFILES=app,workers,tls`, with which Caddy gets the certificate and serves ports 80 and 443. Behind a reverse proxy of your own (nginx, say), leave `tls` out and send everything to port 24000. Then:

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
