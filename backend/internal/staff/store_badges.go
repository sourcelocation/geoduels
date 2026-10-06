package staff

import (
	"context"

	"geoduels/internal/badges"
)

// AwardTeamBadge gives staff the GeoDuels team badge in the role transaction.
func (a *PGStore) AwardTeamBadge(ctx context.Context, userID string) error {
	tx, err := a.requireTx()
	if err != nil {
		return err
	}
	_, err = badges.AwardBadgeTx(ctx, tx, userID, "geoduels-team")
	return err
}

// RemoveTeamBadge removes the badge when a user's last role is revoked.
func (a *PGStore) RemoveTeamBadge(ctx context.Context, userID string) error {
	tx, err := a.requireTx()
	if err != nil {
		return err
	}
	return badges.RemoveGeoDuelsTeamBadgeTx(ctx, tx, userID)
}
