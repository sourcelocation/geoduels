package staff

import (
	"context"
	"time"

	"geoduels/internal/storekit"
	"geoduels/pkg/contracts"
	db "geoduels/pkg/persistence/sqlc/db"
)

func (a *PGStore) LockUser(ctx context.Context, userID string) error {
	id, err := storekit.ProfileUUID(userID)
	if err != nil {
		return ErrNotFound
	}
	_, err = a.q().LockStaffUser(ctx, id)
	return mapNoRows(err)
}

func (a *PGStore) UserRoles(ctx context.Context, userID string) ([]string, error) {
	id, err := storekit.ProfileUUID(userID)
	if err != nil {
		return nil, ErrNotFound
	}
	return a.q().GetStaffRoles(ctx, id)
}

func (a *PGStore) GrantRole(ctx context.Context, userID, role, actorID, reason string) error {
	id, err := storekit.ProfileUUID(userID)
	if err != nil {
		return ErrNotFound
	}
	return a.q().InsertStaffRole(ctx, db.InsertStaffRoleParams{
		UserID:      id,
		Role:        db.StaffRole(role),
		ActorUserID: actorID,
		Reason:      reason,
	})
}

func (a *PGStore) RevokeRole(ctx context.Context, userID, role string) error {
	id, err := storekit.ProfileUUID(userID)
	if err != nil {
		return ErrNotFound
	}
	return a.q().DeleteStaffRole(ctx, db.DeleteStaffRoleParams{UserID: id, Role: db.StaffRole(role)})
}

func (a *PGStore) ListRoleGrants(ctx context.Context) ([]contracts.UserRoleGrant, error) {
	ctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	rows, err := a.q().ListUserRoles(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]contracts.UserRoleGrant, 0, len(rows))
	for _, row := range rows {
		out = append(out, contracts.UserRoleGrant{
			UserID: row.ID.String(), DisplayName: storekit.TextVal(row.DisplayName), Email: row.Email,
			Role: row.Role, GrantedBy: storekit.UUIDVal(row.ActorUserID), GrantedAt: row.GrantedAt.Time, Reason: row.LastReason,
		})
	}
	return out, nil
}
