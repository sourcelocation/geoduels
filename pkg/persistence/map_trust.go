package persistence

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"geoduels/pkg/contracts"
)

const (
	mapCreatorTierBase = iota
	mapCreatorTierTrusted
	mapCreatorTierEstablished
)

type mapCreatorLimits struct {
	name                     string
	maxMaps                  int
	maxActiveLocations       int
	maxUploadsPerHour        int
	maxUploadsPerDay         int
	maxUploadedLocationsHour int
}

func limitsForMapCreatorTier(tier int) mapCreatorLimits {
	switch tier {
	case mapCreatorTierEstablished:
		return mapCreatorLimits{
			name:                     "established",
			maxMaps:                  100,
			maxActiveLocations:       1_000_000,
			maxUploadsPerHour:        10,
			maxUploadsPerDay:         30,
			maxUploadedLocationsHour: 1_000_000,
		}
	case mapCreatorTierTrusted:
		return mapCreatorLimits{
			name:                     "trusted",
			maxMaps:                  25,
			maxActiveLocations:       500_000,
			maxUploadsPerHour:        10,
			maxUploadsPerDay:         30,
			maxUploadedLocationsHour: 600_000,
		}
	default:
		return mapCreatorLimits{
			name:                     "base",
			maxMaps:                  10,
			maxActiveLocations:       200_000,
			maxUploadsPerHour:        10,
			maxUploadsPerDay:         30,
			maxUploadedLocationsHour: 300_000,
		}
	}
}

func automaticMapCreatorTier(accountAgeDays, qualifiedFavorites, qualifiedMaps int, restricted bool) int {
	if restricted {
		return mapCreatorTierBase
	}
	if accountAgeDays >= 30 && qualifiedFavorites >= 100 && qualifiedMaps >= 2 {
		return mapCreatorTierEstablished
	}
	if accountAgeDays >= 14 && qualifiedFavorites >= 25 {
		return mapCreatorTierTrusted
	}
	return mapCreatorTierBase
}

func mapCreatorTierName(tier int) string {
	return limitsForMapCreatorTier(tier).name
}

func (s *pgStore) GetMapUploadQuota(userID string) (contracts.MapUploadQuota, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return contracts.MapUploadQuota{}, errors.New("user required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return contracts.MapUploadQuota{}, err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `select pg_advisory_xact_lock(hashtext($1))`, "map-upload:"+userID); err != nil {
		return contracts.MapUploadQuota{}, err
	}
	quota, err := refreshMapCreatorTrust(ctx, tx, userID)
	if err != nil {
		return contracts.MapUploadQuota{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return contracts.MapUploadQuota{}, err
	}
	return quota, nil
}

func (s *pgStore) SetMapCreatorTierOverride(userID string, tier *int) (contracts.MapUploadQuota, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return contracts.MapUploadQuota{}, errors.New("user required")
	}
	if tier != nil && (*tier < mapCreatorTierBase || *tier > mapCreatorTierEstablished) {
		return contracts.MapUploadQuota{}, errors.New("invalid map creator tier")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return contracts.MapUploadQuota{}, err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `select pg_advisory_xact_lock(hashtext($1))`, "map-upload:"+userID); err != nil {
		return contracts.MapUploadQuota{}, err
	}
	tag, err := tx.Exec(ctx, `update users set map_creator_tier_override=$2 where id=$1`, userID, tier)
	if err != nil {
		return contracts.MapUploadQuota{}, err
	}
	if tag.RowsAffected() == 0 {
		return contracts.MapUploadQuota{}, pgx.ErrNoRows
	}
	quota, err := refreshMapCreatorTrust(ctx, tx, userID)
	if err != nil {
		return contracts.MapUploadQuota{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return contracts.MapUploadQuota{}, err
	}
	return quota, nil
}

func refreshMapCreatorTrust(ctx context.Context, tx pgx.Tx, userID string) (contracts.MapUploadQuota, error) {
	var (
		accountType  string
		createdAt    time.Time
		bannedAt     *time.Time
		banExpiresAt *time.Time
		deletedAt    *time.Time
		override     *int
	)
	if err := tx.QueryRow(ctx, `
		select account_type, created_at, banned_at, ban_expires_at, deleted_at, map_creator_tier_override
		from users
		where id=$1
	`, userID).Scan(&accountType, &createdAt, &bannedAt, &banExpiresAt, &deletedAt, &override); err != nil {
		return contracts.MapUploadQuota{}, err
	}

	var qualifiedFavorites, qualifiedMaps int
	if err := tx.QueryRow(ctx, `
		select count(distinct mf.user_id)::int, count(distinct mf.map_id)::int
		from map_favorites mf
		join maps m on m.id=mf.map_id
		join users favoriter on favoriter.id=mf.user_id
		where m.owner_user_id=$1
		  and mf.user_id<>$1
		  and favoriter.account_type='registered'
		  and favoriter.created_at <= now()-interval '7 days'
		  and not coalesce(favoriter.banned_at is not null and (favoriter.ban_expires_at is null or favoriter.ban_expires_at > now()), false)
		  and favoriter.deleted_at is null
	`, userID).Scan(&qualifiedFavorites, &qualifiedMaps); err != nil {
		return contracts.MapUploadQuota{}, err
	}

	var activeSanction bool
	if err := tx.QueryRow(ctx, `
		select coalesce(
			report_muted_at is not null and (report_mute_expires_at is null or report_mute_expires_at > now()),
			false
		) from users where id=$1
	`, userID).Scan(&activeSanction); err != nil {
		return contracts.MapUploadQuota{}, err
	}

	accountAgeDays := max(0, int(time.Since(createdAt).Hours()/24))
	activeBan := bannedAt != nil && (banExpiresAt == nil || banExpiresAt.After(time.Now()))
	restricted := accountType != "registered" || activeBan || deletedAt != nil || activeSanction
	tier := automaticMapCreatorTier(accountAgeDays, qualifiedFavorites, qualifiedMaps, restricted)
	if override != nil && !restricted {
		tier = *override
	}
	limits := limitsForMapCreatorTier(tier)

	var currentMaps, currentLocations int
	if err := tx.QueryRow(ctx, `
		select count(*)::int, coalesce(sum(location_count),0)::int
		from maps
		where owner_user_id=$1 and archived_at is null
	`, userID).Scan(&currentMaps, &currentLocations); err != nil {
		return contracts.MapUploadQuota{}, err
	}
	if _, err := tx.Exec(ctx, `
		update users
		set map_creator_tier=$2,
		    map_creator_qualified_favorites=$3,
		    map_creator_qualified_maps=$4,
		    map_creator_trust_updated_at=now()
		where id=$1
	`, userID, tier, qualifiedFavorites, qualifiedMaps); err != nil {
		return contracts.MapUploadQuota{}, err
	}

	quota := contracts.MapUploadQuota{
		Tier:                     limits.name,
		QualifiedFavorites:       qualifiedFavorites,
		QualifiedMaps:            qualifiedMaps,
		AccountAgeDays:           accountAgeDays,
		MaxMaps:                  limits.maxMaps,
		MaxActiveLocations:       limits.maxActiveLocations,
		MaxMapLocations:          limits.maxActiveLocations,
		MaxUploadsPerHour:        limits.maxUploadsPerHour,
		MaxUploadsPerDay:         limits.maxUploadsPerDay,
		MaxUploadedLocationsHour: limits.maxUploadedLocationsHour,
		CurrentMaps:              currentMaps,
		CurrentActiveLocations:   currentLocations,
		RestrictedByModeration:   restricted,
	}
	if override != nil {
		quota.TierOverride = mapCreatorTierName(*override)
	}
	switch tier {
	case mapCreatorTierBase:
		quota.NextTier = "trusted"
		quota.FavoritesNeeded = max(0, 25-qualifiedFavorites)
		quota.DaysNeeded = max(0, 14-accountAgeDays)
	case mapCreatorTierTrusted:
		quota.NextTier = "established"
		quota.FavoritesNeeded = max(0, 100-qualifiedFavorites)
		quota.MapsNeeded = max(0, 2-qualifiedMaps)
		quota.DaysNeeded = max(0, 30-accountAgeDays)
	}
	return quota, nil
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
		var existingLocations int
		if err := tx.QueryRow(ctx, `
			select location_count
			from maps
			where id=$1 and owner_user_id=$2 and archived_at is null
		`, mapID, userID).Scan(&existingLocations); err != nil {
			return err
		}
		currentLocations -= existingLocations
	}
	if currentLocations+incoming > quota.MaxActiveLocations {
		return fmt.Errorf("%s tier account limit is %d active map locations", quota.Tier, quota.MaxActiveLocations)
	}

	var hourlyUploads, dailyUploads, hourlyLocations int
	if err := tx.QueryRow(ctx, `
		select
			count(*) filter(where created_at>now()-interval '1 hour')::int,
			count(*) filter(where created_at>now()-interval '1 day')::int,
			coalesce(sum(location_count) filter(where created_at>now()-interval '1 hour'),0)::int
		from map_upload_events
		where user_id=$1
	`, userID).Scan(&hourlyUploads, &dailyUploads, &hourlyLocations); err != nil {
		return err
	}
	if hourlyUploads >= quota.MaxUploadsPerHour || dailyUploads >= quota.MaxUploadsPerDay {
		return errors.New("map upload rate limit exceeded")
	}
	if hourlyLocations+incoming > quota.MaxUploadedLocationsHour {
		return fmt.Errorf("%s tier upload throughput is %d locations per hour", quota.Tier, quota.MaxUploadedLocationsHour)
	}
	return nil
}
