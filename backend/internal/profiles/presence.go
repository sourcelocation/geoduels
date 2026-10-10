package profiles

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"geoduels/internal/storekit"
	db "geoduels/pkg/persistence/sqlc/db"
)

// PresenceWindow is how recently a player must have been seen to count as online. Every open socket
// refreshes it well within the window.
const PresenceWindow = 90 * time.Second

// TouchPresence records that the player is online now.
func (s *PGStore) TouchPresence(ctx context.Context, userID string) error {
	id, err := storekit.ProfileUUID(userID)
	if err != nil {
		return err
	}
	return s.db.TouchPresence(ctx, id)
}

// TouchLastSeen records when the player was last seen, as their profile shows it.
func (s *PGStore) TouchLastSeen(ctx context.Context, userID string, seenAt time.Time) error {
	id, err := storekit.ProfileUUID(userID)
	if err != nil {
		return err
	}
	return s.db.TouchLastSeen(ctx, db.TouchLastSeenParams{ID: id, LastSeenAt: pgtype.Timestamptz{Time: seenAt, Valid: true}})
}

// CountOnline counts the players seen within the presence window.
func (s *PGStore) CountOnline(ctx context.Context) (int, error) {
	n, err := s.db.CountOnlineUsers(ctx, PresenceWindow.Seconds())
	return int(n), err
}

// Online says which of the players were seen within the presence window.
func (s *PGStore) Online(ctx context.Context, userIDs []string) (map[string]bool, error) {
	out := map[string]bool{}
	ids := make([]string, 0, len(userIDs))
	for _, id := range userIDs {
		if _, err := storekit.ProfileUUID(id); err == nil {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := s.db.ListOnlineUsers(ctx, db.ListOnlineUsersParams{UserIds: storekit.UUIDList(ids), WindowSeconds: PresenceWindow.Seconds()})
	if err != nil {
		return nil, err
	}
	for _, id := range rows {
		out[storekit.UUIDVal(id)] = true
	}
	return out, nil
}
