-- name: InsertLiveConnection :exec
INSERT INTO live_connections (id, gateway_id, user_id) VALUES ($1, $2, sqlc.narg(user_id))
ON CONFLICT (id) DO NOTHING;

-- name: DeleteLiveConnection :exec
DELETE FROM live_connections WHERE id = $1;

-- name: CountUserConnections :one
SELECT count(*)::int FROM live_connections WHERE user_id = $1;

-- name: TouchGateway :exec
INSERT INTO gateways (id, seen_at) VALUES ($1, now())
ON CONFLICT (id) DO UPDATE SET seen_at = now();

-- name: DeleteGatewayConnectionsExcept :many
-- The connections a gateway no longer holds. Returns whose they were.
DELETE FROM live_connections
WHERE gateway_id = $1 AND NOT (id = ANY(sqlc.arg(held)::uuid[]))
RETURNING user_id;

-- name: DeleteStaleGateways :many
-- Only one replica's delete removes a given row, so no leader is needed.
DELETE FROM gateways WHERE seen_at < now() - (sqlc.arg(stale_seconds)::double precision * interval '1 second')
RETURNING id;

-- name: DeleteGatewaysConnections :many
DELETE FROM live_connections WHERE gateway_id = ANY(sqlc.arg(gateways)::uuid[]) RETURNING user_id;

-- name: CountConnected :one
-- Everyone connected: each player once, however many tabs, and each visitor without a session.
SELECT (count(DISTINCT user_id) + count(*) FILTER (WHERE user_id IS NULL))::int FROM live_connections;

-- name: ListConnectedUsers :many
SELECT DISTINCT user_id::uuid FROM live_connections WHERE user_id = ANY(sqlc.arg(user_ids)::uuid[]);
