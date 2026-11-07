api: go -C backend tool -modfile=tools/go.mod air -c services/api/.air.toml
gateway: go -C backend tool -modfile=tools/go.mod air -c services/gateway/.air.toml
gameplay: go -C backend tool -modfile=tools/go.mod air -c services/gameplay-node/.air.toml
web: npm --prefix web run dev
moderation: go -C backend tool -modfile=tools/go.mod air -c services/moderation-worker/.air.toml
discord: go -C backend tool -modfile=tools/go.mod air -c services/discord-worker/.air.toml
