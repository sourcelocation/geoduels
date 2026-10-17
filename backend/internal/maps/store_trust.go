package maps

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	db "geoduels/pkg/persistence/sqlc/db"
)

func (s *PGStore) GetMapUploadQuota(userID string) (UploadQuota, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return UploadQuota{}, errors.New("user required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return UploadQuota{}, err
	}
	defer tx.Rollback(ctx)
	if err := db.New(tx).LockMapUpload(ctx, "map-upload:"+userID); err != nil {
		return UploadQuota{}, err
	}
	quota, err := refreshMapCreatorTrust(ctx, tx, userID)
	if err != nil {
		return UploadQuota{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return UploadQuota{}, err
	}
	return quota, nil
}

func (s *PGStore) SetMapCreatorTierOverride(userID string, tier *int) (UploadQuota, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return UploadQuota{}, errors.New("user required")
	}
	if tier != nil && (*tier < mapCreatorTierBase || *tier > mapCreatorTierEstablished) {
		return UploadQuota{}, errors.New("invalid map creator tier")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return UploadQuota{}, err
	}
	defer tx.Rollback(ctx)
	if err := db.New(tx).LockMapUpload(ctx, "map-upload:"+userID); err != nil {
		return UploadQuota{}, err
	}
	var tierArg pgtype.Int2
	if tier != nil {
		tierArg = pgtype.Int2{Int16: int16(*tier), Valid: true}
	}
	tag, err := db.New(tx).SetMapCreatorTierOverride(ctx, db.SetMapCreatorTierOverrideParams{ID: mustMapUUID(userID), MapCreatorTierOverride: tierArg})
	if err != nil {
		return UploadQuota{}, err
	}
	if tag == 0 {
		return UploadQuota{}, pgx.ErrNoRows
	}
	quota, err := refreshMapCreatorTrust(ctx, tx, userID)
	if err != nil {
		return UploadQuota{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return UploadQuota{}, err
	}
	return quota, nil
}

func refreshMapCreatorTrust(ctx context.Context, tx pgx.Tx, userID string) (UploadQuota, error) {
	q := db.New(tx)
	var (
		createdAt    time.Time
		bannedAt     *time.Time
		banExpiresAt *time.Time
		deletedAt    *time.Time
		override     *int
	)
	u, err := q.GetMapTrustUser(ctx, mustMapUUID(userID))
	if err != nil {
		return UploadQuota{}, err
	}
	createdAt = u.CreatedAt.Time
	if u.BannedAt.Valid {
		bannedAt = &u.BannedAt.Time
	}
	if u.BanExpiresAt.Valid {
		banExpiresAt = &u.BanExpiresAt.Time
	}
	if u.DeletedAt.Valid {
		deletedAt = &u.DeletedAt.Time
	}
	if u.MapCreatorTierOverride.Valid {
		v := int(u.MapCreatorTierOverride.Int16)
		override = &v
	}
	activeSanction := u.ReportMutedAt.Valid && (!u.ReportMuteExpiresAt.Valid || u.ReportMuteExpiresAt.Time.After(time.Now()))
	if false {
		return UploadQuota{}, err
	}

	var qualifiedFavorites, qualifiedMaps int
	f, err := q.GetQualifiedMapFavorites(ctx, mustMapUUID(userID))
	if err != nil {
		return UploadQuota{}, err
	}
	qualifiedFavorites = int(f.QualifiedFavorites)
	qualifiedMaps = int(f.QualifiedMaps)

	accountAgeDays := max(0, int(time.Since(createdAt).Hours()/24))
	activeBan := bannedAt != nil && (banExpiresAt == nil || banExpiresAt.After(time.Now()))
	restricted := !u.HasIdentity || activeBan || deletedAt != nil || activeSanction
	tier := automaticMapCreatorTier(accountAgeDays, qualifiedFavorites, restricted)
	if override != nil && !restricted {
		tier = *override
	}
	spec := specForMapCreatorTier(tier)

	var currentMaps, currentLocations int
	c, err := q.GetActiveMapCounts(ctx, mustMapUUID(userID))
	if err != nil {
		return UploadQuota{}, err
	}
	currentMaps = int(c.CurrentMaps)
	currentLocations = int(c.CurrentLocations)
	if err := q.UpdateMapCreatorTrust(ctx, db.UpdateMapCreatorTrustParams{ID: mustMapUUID(userID), MapCreatorTier: int16(tier), MapCreatorQualifiedFavorites: int32(qualifiedFavorites), MapCreatorQualifiedMaps: int32(qualifiedMaps)}); err != nil {
		return UploadQuota{}, err
	}

	quota := UploadQuota{
		Tier:                     spec.name,
		QualifiedFavorites:       qualifiedFavorites,
		QualifiedMaps:            qualifiedMaps,
		AccountAgeDays:           accountAgeDays,
		MaxMaps:                  spec.maxMaps,
		MaxActiveLocations:       spec.maxActiveLocations,
		MaxMapLocations:          spec.maxLocationsPerUpload,
		MaxUploadsPerHour:        spec.maxUploadsPerHour,
		MaxUploadsPerDay:         spec.maxUploadsPerDay,
		MaxUploadedLocationsHour: spec.maxUploadedLocationsHour,
		CurrentMaps:              currentMaps,
		CurrentActiveLocations:   currentLocations,
		RestrictedByModeration:   restricted,
	}
	if override != nil {
		quota.TierOverride = mapCreatorTierName(*override)
	}
	switch tier {
	case mapCreatorTierBase:
		applyNextTierProgress(&quota, specForMapCreatorTier(mapCreatorTierTrusted), qualifiedFavorites, accountAgeDays)
	case mapCreatorTierTrusted:
		applyNextTierProgress(&quota, specForMapCreatorTier(mapCreatorTierEstablished), qualifiedFavorites, accountAgeDays)
	}
	return quota, nil
}

func applyNextTierProgress(quota *UploadQuota, next mapCreatorSpec, qualifiedFavorites, accountAgeDays int) {
	quota.NextTier = next.name
	quota.FavoritesNeeded = max(0, next.favoritesNeeded-qualifiedFavorites)
	quota.DaysNeeded = max(0, next.accountAgeDays-accountAgeDays)
}

func enforceMapUploadQuota(ctx context.Context, tx pgx.Tx, userID, mapID string, incoming int, create bool) error {
	quota, err := refreshMapCreatorTrust(ctx, tx, userID)
	if err != nil {
		return err
	}
	if incoming > quota.MaxMapLocations {
		return fmt.Errorf("%s tier map limit is %d locations", quota.Tier, quota.MaxMapLocations)
	}
	if create && quota.CurrentMaps >= quota.MaxMaps {
		return fmt.Errorf("%s tier account limit is %d custom maps", quota.Tier, quota.MaxMaps)
	}

	currentLocations := quota.CurrentActiveLocations
	if !create {
		existing, err := db.New(tx).GetOwnedActiveMapLocationCount(ctx, db.GetOwnedActiveMapLocationCountParams{ID: mustMapUUID(mapID), OwnerUserID: mustMapUUID(userID)})
		if err != nil {
			return err
		}
		currentLocations -= int(existing)
	}
	if currentLocations+incoming > quota.MaxActiveLocations {
		return fmt.Errorf("%s tier account limit is %d active map locations", quota.Tier, quota.MaxActiveLocations)
	}

	stats, err := db.New(tx).GetMapUploadStats(ctx, mustMapUUID(userID))
	if err != nil {
		return err
	}
	hourlyUploads, dailyUploads, hourlyLocations := int(stats.HourlyUploads), int(stats.DailyUploads), int(stats.HourlyLocations)
	if hourlyUploads >= quota.MaxUploadsPerHour || dailyUploads >= quota.MaxUploadsPerDay {
		return errors.New("map upload rate limit exceeded")
	}
	if hourlyLocations+incoming > quota.MaxUploadedLocationsHour {
		return fmt.Errorf("%s tier upload throughput is %d locations per hour", quota.Tier, quota.MaxUploadedLocationsHour)
	}
	return nil
}
