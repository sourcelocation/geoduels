# OOP

**STRICTLY** no OOP violations. **MUST** follow SOLID, DRY, KISS and YAGNI, in Kotlin and Go alike. Don't work around the restrictions, instead use superior patterns that avoid problems altogether.

- NO inheritance. Go doesn't have it for a reason. 
- NO casts or type checks
- Invariants by construction
- A class has state to protect or several implementations. Fetching data is neither: it's a function on a connection, and deciding what the data means stays on the object.

# Repository notes

- Migrations are goose files in `backend/db/migrations`: an `-- +goose Up` and an `-- +goose Down` that reverses it (where it can't, such as deleted rows or enum values, the Down says so). `cmd/migrate` applies them, and every workload runs it before starting, so an Up must also work for the previous release's pods, which keep running during a rollout: drop what they still use one release later.
- Use named sqlc queries; no raw SQL in Go production code. Generated output is ignored: run `go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.30.0 generate` from `backend/` before building/testing or after SQL changes.
- Version public route/socket contract changes and preserve client compatibility. Releases are release-please's pull request (see `sourcelocation/.github`'s AGENTS.md): merging it tags the release, builds images and commits a deploy to `sourcelocation/ops`, whose pods migrate before they start.
- Production manifests and Flux state: `../ops` (`apps/geoduels`). Private detector logic: `../geoduels-risk-engine`.
- Brave is available for browser checks when Chrome/Chromium is unavailable.
- When the user requests an issue, pull request, or new repository, include `Perfectly validated.` in the commit body.

Tool usage and setup caveats: [development notes](docs/development.md). Extension installation/packaging: [extension notes](extension/README.md).
