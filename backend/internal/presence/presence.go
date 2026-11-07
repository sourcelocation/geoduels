// Package presence knows who is connected: one row per connection, kept current by the gateways that
// hold them. Fetching it is a function on a connection, and nothing about it lives in a process.
package presence

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"geoduels/internal/storekit"
	db "geoduels/pkg/persistence/sqlc/db"
)

// HeartbeatEvery is how often a gateway reports what it holds; StaleAfter is how long until a gateway
// that stopped reporting counts as gone.
const (
	HeartbeatEvery = 15 * time.Second
	StaleAfter     = 45 * time.Second
)

// Connection is one open socket. UserID is empty for a visitor without a session.
type Connection struct {
	ID     string `json:"id"`
	UserID string `json:"userId"`
}

func userParam(userID string) (pgtype.UUID, error) {
	if userID == "" {
		return pgtype.UUID{}, nil
	}
	return storekit.ProfileUUID(userID)
}

// Connect records a connection. It reports whether this is the person's first, so they just came online.
func Connect(ctx context.Context, q db.DBTX, gatewayID string, connection Connection) (bool, error) {
	id, err := storekit.ProfileUUID(connection.ID)
	if err != nil {
		return false, err
	}
	gateway, err := storekit.ProfileUUID(gatewayID)
	if err != nil {
		return false, err
	}
	user, err := userParam(connection.UserID)
	if err != nil {
		return false, err
	}
	queries := db.New(q)
	if err := queries.TouchGateway(ctx, gateway); err != nil {
		return false, err
	}
	if err := queries.InsertLiveConnection(ctx, db.InsertLiveConnectionParams{ID: id, GatewayID: gateway, UserID: user}); err != nil {
		return false, err
	}
	if !user.Valid {
		return false, nil
	}
	count, err := queries.CountUserConnections(ctx, user)
	return count == 1, err
}

// Disconnect removes a connection. It reports whether the person has none left, so they went offline.
func Disconnect(ctx context.Context, q db.DBTX, connection Connection) (bool, error) {
	id, err := storekit.ProfileUUID(connection.ID)
	if err != nil {
		return false, err
	}
	user, err := userParam(connection.UserID)
	if err != nil {
		return false, err
	}
	queries := db.New(q)
	if err := queries.DeleteLiveConnection(ctx, id); err != nil {
		return false, err
	}
	if !user.Valid {
		return false, nil
	}
	count, err := queries.CountUserConnections(ctx, user)
	return count == 0, err
}

// Reconcile makes the rows match what a gateway reports holding, which also repairs any connect or
// disconnect that never arrived. It returns who has no connection left.
func Reconcile(ctx context.Context, q db.DBTX, gatewayID string, held []Connection) ([]string, error) {
	gateway, err := storekit.ProfileUUID(gatewayID)
	if err != nil {
		return nil, err
	}
	queries := db.New(q)
	if err := queries.TouchGateway(ctx, gateway); err != nil {
		return nil, err
	}
	ids := make([]pgtype.UUID, 0, len(held))
	for _, connection := range held {
		id, err := storekit.ProfileUUID(connection.ID)
		if err != nil {
			return nil, err
		}
		user, err := userParam(connection.UserID)
		if err != nil {
			return nil, err
		}
		if err := queries.InsertLiveConnection(ctx, db.InsertLiveConnectionParams{ID: id, GatewayID: gateway, UserID: user}); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	gone, err := queries.DeleteGatewayConnectionsExcept(ctx, db.DeleteGatewayConnectionsExceptParams{GatewayID: gateway, Held: ids})
	if err != nil {
		return nil, err
	}
	return disconnected(ctx, queries, gone)
}

// Sweep removes gateways that stopped reporting and their connections, returning who has no
// connection left. Any number of replicas can run it: only one removes a given row.
func Sweep(ctx context.Context, q db.DBTX) ([]string, error) {
	queries := db.New(q)
	gateways, err := queries.DeleteStaleGateways(ctx, StaleAfter.Seconds())
	if err != nil || len(gateways) == 0 {
		return nil, err
	}
	gone, err := queries.DeleteGatewaysConnections(ctx, gateways)
	if err != nil {
		return nil, err
	}
	return disconnected(ctx, queries, gone)
}

func disconnected(ctx context.Context, queries *db.Queries, gone []pgtype.UUID) ([]string, error) {
	candidates := make([]pgtype.UUID, 0, len(gone))
	for _, user := range gone {
		if user.Valid {
			candidates = append(candidates, user)
		}
	}
	if len(candidates) == 0 {
		return nil, nil
	}
	stillConnected, err := queries.ListConnectedUsers(ctx, candidates)
	if err != nil {
		return nil, err
	}
	connected := make(map[string]bool, len(stillConnected))
	for _, user := range stillConnected {
		connected[storekit.UUIDVal(user)] = true
	}
	seen := make(map[string]bool, len(candidates))
	out := make([]string, 0, len(candidates))
	for _, user := range candidates {
		id := storekit.UUIDVal(user)
		if !connected[id] && !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out, nil
}

// Count is how many people are connected.
func Count(ctx context.Context, q db.DBTX) (int, error) {
	count, err := db.New(q).CountConnected(ctx)
	return int(count), err
}

// Online says which of the people are connected.
func Online(ctx context.Context, q db.DBTX, userIDs []string) (map[string]bool, error) {
	out := make(map[string]bool, len(userIDs))
	if len(userIDs) == 0 {
		return out, nil
	}
	rows, err := db.New(q).ListConnectedUsers(ctx, storekit.UUIDList(userIDs))
	if err != nil {
		return nil, err
	}
	for _, user := range rows {
		out[storekit.UUIDVal(user)] = true
	}
	return out, nil
}
