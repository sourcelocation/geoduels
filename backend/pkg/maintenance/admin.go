package maintenance

import (
	"context"
	"encoding/json"

	"github.com/redis/go-redis/v9"

	pkgstaff "geoduels/pkg/staff"
)

// StaffRead returns the current status for the staff configuration page.
func StaffRead(ctx context.Context, rdb *redis.Client, actor pkgstaff.Actor) (Status, error) {
	if err := actor.RequireCap(pkgstaff.CapManageConfig); err != nil {
		return Status{}, err
	}
	return Read(ctx, rdb)
}

// Save publishes status to every service that reads maintenance state.
func Save(ctx context.Context, rdb *redis.Client, actor pkgstaff.Actor, status Status) (Status, error) {
	if err := actor.RequireCap(pkgstaff.CapManageConfig); err != nil {
		return Status{}, err
	}
	status = status.Normalized()
	body, err := json.Marshal(status)
	if err != nil {
		return Status{}, err
	}
	if err := rdb.Set(ctx, RedisKey, body, 0).Err(); err != nil {
		return Status{}, err
	}
	return status, nil
}

// Clear returns every service to normal operation.
func Clear(ctx context.Context, rdb *redis.Client, actor pkgstaff.Actor) error {
	if err := actor.RequireCap(pkgstaff.CapManageConfig); err != nil {
		return err
	}
	return rdb.Del(ctx, RedisKey).Err()
}
