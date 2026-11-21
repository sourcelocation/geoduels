<p align="center">
  <a href="https://geoduels.io"><img src="web/public/icon.v1.png" alt="GeoDuels" width="96"></a>
</p>

<h1 align="center">GeoDuels</h1>

<p align="center">
  <b>A free, open-source geography game built for head-to-head play.</b><br>
  Drop into Street View, pin where you think you are, and outguess a real opponent.
</p>

<p align="center">
  <a href="https://geoduels.io"><b>Play now</b></a>
  &nbsp;·&nbsp;
  <a href="https://discord.gg/xxz8V9UU7Z">Discord</a>
  &nbsp;·&nbsp;
  <a href="#how-it-works">How it works</a>
  &nbsp;·&nbsp;
  <a href="#run-your-own">Run your own</a>
</p>

<p align="center">
  <a href="https://github.com/sourcelocation/geoduels/releases/latest"><img alt="Release" src="https://img.shields.io/github/v/release/sourcelocation/geoduels?style=flat-square&label=release"></a>
  <a href="https://github.com/sourcelocation/geoduels/actions/workflows/ci.yml"><img alt="CI" src="https://img.shields.io/github/actions/workflow/status/sourcelocation/geoduels/ci.yml?branch=master&style=flat-square&label=ci"></a>
  <a href="LICENSE"><img alt="License: AGPL-3.0" src="https://img.shields.io/badge/license-AGPL--3.0-blue?style=flat-square"></a>
</p>

<p align="center">
  <a href="https://geoduels.io"><img src=".github/assets/duel.gif" alt="A GeoDuels duel: both players guess on the map, the round reveals the answer, and the loser's health drops" width="100%"></a>
</p>

## Features

- **Ranked duels** with seasons and a leaderboard
- **Team duels and free-for-alls** in custom lobbies with friends
- **Community maps**, curated and validated against Street View
- **Parties, friends, chat and an inbox**, live across devices
- **Cheater mitigations**, with staff review and an audit log of every staff action
- **Sign in with Google or Discord**, or play as a guest
- **[Browser extension](extension/)** for extra Street View controls, on [Chrome](https://chromewebstore.google.com/detail/geoduels-enhancer/ecdjkhpicnccgbkdimbbjbnppnkeelmd) and [Firefox](https://addons.mozilla.org/en-US/firefox/addon/geoduels-enhancer/)

## How it works

A duel is two people racing a clock. Every guess has to land instantly, a deploy can't end a match, and a cheater can't keep anyone's rating for long. Here's how GeoDuels manages all three.

```
 browser ──► front (Caddy) ──► web (Next.js)
                │
                ├── /api ──────► api (Go) ────────────────────────────┐
                │                                                     │
                └── /api/live ─► gateway (Go) ──► gameplay node (Go)  ├──► PostgreSQL
                                   ▲            one match, one node   │    (state, jobs)
                                   │                                  │
                                   └──── NATS ◄── api, workers ───────┘
                                    live events to whoever holds the socket
```

- **Everything is stateless** except the gameplay nodes, which keep matches in RAM so guesses never wait on the database. That lets GeoDuels scale about as far as you need.
- **No maintenance breaks required,** since every service rolls out on its own. Gameplay nodes are replaced one at a time: each stops taking new matches and finishes its current ones, while the other keeps hosting.
- **No unnecessary cross-server communication.** NATS only carries what truly has to cross servers *right now*. Postgres stores the rest: accounts, Glicko ratings, matches, chat history and the job queue.
- **Bold and strict architecture.** No inheritance, no type casts. This alone puts constraints on code, incentivising everyone working on project to write better code - catches more bugs than you'd expect.
- **Cheaters face an in-house private anticheat.** Stored in a private repo, it scores the speed and accuracy of every guess, and victims are refunded as if they'd beaten the cheater.
- **Shipping is one merge.** release-please tags it, CI builds six images for amd64 and arm64, and Flux deploys them to k3s. Every pod migrates the database before it starts, so the new version goes live on its own.

## Run your own

[`compose.yaml`](compose.yaml) runs the released images (amd64 and arm64), with Caddy getting the certificate.

```sh
cp .env.example .env   # then set the values marked "Self-hosting"
docker compose up -d
```

Set `COMPOSE_PROFILES=app,workers,tls` and a `GEODUELS_VERSION` such as `v2.3.0`. Behind your own reverse proxy, leave out `tls` and send everything to port 24000. The `db-migrate` service brings the schema up to date before anything else starts.

<details>
<summary><b>Develop locally</b></summary>

<br>

You need Docker, Go 1.26, Node 22 and [Overmind](https://github.com/DarthSim/overmind).

```sh
cp .env.example .env
(cd backend && go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.30.0 generate)
docker compose up -d     # Postgres and NATS
npm --prefix web ci
overmind start           # Go services under Air, Next's dev server
```

Open http://localhost:24000. Every service reloads as you edit, and Next routes `/api` the way production's ingress does. To run the images instead, stop Overmind and run `COMPOSE_PROFILES=app docker compose up -d --build`.

More in the [development notes](docs/development.md): generated code, tests, releases and map tools.

</details>

## Contributing

Issues and pull requests are welcome. Read [CONTRIBUTING.md](CONTRIBUTING.md) and [AGENTS.md](AGENTS.md) first: commits follow Conventional Commits, and contributions fall under the [CLA](CONTRIBUTOR_LICENSE_AGREEMENT.md).

## License

[AGPL-3.0](LICENSE).
