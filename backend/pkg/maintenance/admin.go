package maintenance

import (
	"context"
	"encoding/json"

	db "geoduels/pkg/persistence/sqlc/db"
	pkgstaff "geoduels/pkg/staff"
)

// StaffRead returns the current status for the staff configuration page.
func StaffRead(ctx context.Context, q db.DBTX, actor pkgstaff.Actor) (Status, error) {
	if err := actor.RequireCap(pkgstaff.CapManageConfig); err != nil {
		return Status{}, err
	}
	return Read(ctx, q)
}

// Save sets the status every service follows.
func Save(ctx context.Context, q db.DBTX, actor pkgstaff.Actor, status Status) (Status, error) {
	if err := actor.RequireCap(pkgstaff.CapManageConfig); err != nil {
		return Status{}, err
	}
	status = status.Normalized()
	body, err := json.Marshal(status)
	if err != nil {
		return Status{}, err
	}
	if err := db.New(q).SetSetting(ctx, db.SetSettingParams{SettingKey: settingKey, ValueJson: body}); err != nil {
		return Status{}, err
	}
	return status, nil
}

// Clear returns every service to normal operation.
func Clear(ctx context.Context, q db.DBTX, actor pkgstaff.Actor) error {
	if err := actor.RequireCap(pkgstaff.CapManageConfig); err != nil {
		return err
	}
	return db.New(q).DeleteSetting(ctx, settingKey)
}
