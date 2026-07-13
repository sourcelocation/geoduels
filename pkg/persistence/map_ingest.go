package persistence

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"geoduels/pkg/contracts"
	"geoduels/pkg/entityid"
)

// fullCircleE7 is 360 degrees expressed in the e7 fixed-point scale.
const fullCircleE7 = int64(3_600_000_000)

// computePlayRegionBoundsE7 derives the play-region bounding box for a set of
// parsed map locations. Latitude uses a plain min/max; longitude uses the
// shortest circular interval so that antimeridian-crossing maps stay narrow.
func computePlayRegionBoundsE7(rows []mapRow) (minLat, maxLat, minLng, maxLng int32) {
	minLat, maxLat = rows[0].LatE7, rows[0].LatE7
	lngs := make([]int32, 0, len(rows))
	for _, row := range rows {
		if row.LatE7 < minLat {
			minLat = row.LatE7
		}
		if row.LatE7 > maxLat {
			maxLat = row.LatE7
		}
		lngs = append(lngs, row.LngE7)
	}
	minLng, maxLng = wrappedLngBoundsE7(lngs)
	return
}

// wrappedLngBoundsE7 returns the shortest circular longitude interval covering
// all points, expressed as [start, end] traversed eastward. The interval is the
// complement of the largest empty gap between adjacent longitudes. When it
// crosses the antimeridian, start is numerically greater than end.
func wrappedLngBoundsE7(lngs []int32) (start, end int32) {
	sorted := append([]int32(nil), lngs...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	uniq := sorted[:0]
	for i, v := range sorted {
		if i == 0 || v != uniq[len(uniq)-1] {
			uniq = append(uniq, v)
		}
	}
	if len(uniq) == 1 {
		return uniq[0], uniq[0]
	}
	// Default to the non-crossing interval min..max, whose empty gap is the
	// wrap-around segment across the antimeridian.
	start, end = uniq[0], uniq[len(uniq)-1]
	largestGap := (int64(uniq[0]) + fullCircleE7) - int64(uniq[len(uniq)-1])
	for i := 0; i < len(uniq)-1; i++ {
		gap := int64(uniq[i+1]) - int64(uniq[i])
		if gap > largestGap {
			largestGap = gap
			// The empty region is (uniq[i], uniq[i+1]); the populated region
			// wraps from uniq[i+1] eastward back to uniq[i].
			start, end = uniq[i+1], uniq[i]
		}
	}
	return start, end
}

func (s *pgStore) CreateCustomMap(userID, displayName, description, visibility, difficulty, thumbnailKey string, thumbnailVariant int, source io.Reader) (contracts.CustomMap, error) {
	mapID := entityid.New()
	return s.ingestCustomMap(userID, mapID, displayName, description, visibility, difficulty, thumbnailKey, thumbnailVariant, source, true)
}

func (s *pgStore) ImportOfficialMap(adminUserID string, input OfficialMapImportInput, source io.Reader) (contracts.CustomMap, error) {
	mapKey := strings.TrimSpace(input.MapKey)
	displayName := strings.TrimSpace(input.DisplayName)
	if mapKey == "" {
		return contracts.CustomMap{}, errors.New("map key required")
	}
	if displayName == "" || len(displayName) > 80 {
		return contracts.CustomMap{}, errors.New("map name must be 1 to 80 characters")
	}
	if len(input.Description) > 500 {
		return contracts.CustomMap{}, errors.New("description must be at most 500 characters")
	}
	visibility := normalizeMapVisibility(input.Visibility)
	difficulty := normalizeMapDifficulty(input.Difficulty)
	thumbnailVariant := normalizeThumbnailVariant(input.ThumbnailVariant)
	thumbnailKey := normalizeThumbnailKey(input.ThumbnailKey, thumbnailVariant)
	regionType := strings.ToLower(strings.TrimSpace(input.OfficialRegionType))
	regionCode := strings.ToUpper(strings.TrimSpace(input.OfficialRegionCode))
	if regionType == "" && regionCode != "" {
		regionType = "country"
	}
	if regionType != "" && regionType != "country" && regionType != "continent" {
		return contracts.CustomMap{}, errors.New("unsupported official region type")
	}
	if len(regionCode) > 32 {
		return contracts.CustomMap{}, errors.New("official region code must be at most 32 characters")
	}

	parsed, digest, rejected, err := decodeMapRows(source, absoluteMaxMapLocations)
	if err != nil {
		return contracts.CustomMap{}, err
	}
	if len(parsed) < minMapLocations {
		return contracts.CustomMap{}, fmt.Errorf("map requires at least %d valid locations", minMapLocations)
	}
	digestBytes, err := hex.DecodeString(digest)
	if err != nil {
		return contracts.CustomMap{}, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return contracts.CustomMap{}, err
	}
	defer tx.Rollback(ctx)

	mapID := entityid.Derive("map", mapKey)
	if _, err := tx.Exec(ctx, `
		insert into maps(
			id,map_key,owner_user_id,display_name,description,visibility,status,difficulty,
			thumbnail_variant,thumbnail_key,location_count,content_hash,rejected_location_count,
			published_at,official_at,official_by,official_region_type,official_region_code,created_at,updated_at
		)
		values($1,$2,null,$3,$4,$5,'processing',$6,$7,$8,0,$9,$10,case when $5='public' then now() else null end,now(),nullif($11,'')::uuid,$12,$13,now(),now())
		on conflict(map_key) do update set
			owner_user_id=null,
			display_name=excluded.display_name,
			description=excluded.description,
			visibility=excluded.visibility,
			status='processing',
			difficulty=excluded.difficulty,
			thumbnail_variant=excluded.thumbnail_variant,
			thumbnail_key=excluded.thumbnail_key,
			content_hash=excluded.content_hash,
			rejected_location_count=excluded.rejected_location_count,
			published_at=case when excluded.visibility='public' then coalesce(maps.published_at, now()) else null end,
			official_at=coalesce(maps.official_at, now()),
			official_by=excluded.official_by,
			official_region_type=excluded.official_region_type,
			official_region_code=excluded.official_region_code,
			archived_at=null,
			updated_at=now()
	`, mapID, mapKey, displayName, strings.TrimSpace(input.Description), visibility, difficulty, thumbnailVariant, thumbnailKey, digestBytes, rejected, strings.TrimSpace(adminUserID), regionType, regionCode); err != nil {
		return contracts.CustomMap{}, err
	}

	if err := tx.QueryRow(ctx, `select id::text from maps where map_key=$1 for update`, mapKey).Scan(&mapID); err != nil {
		return contracts.CustomMap{}, err
	}
	var mapStorageID int32
	if err := tx.QueryRow(ctx, `select storage_id from maps where id=$1`, mapID).Scan(&mapStorageID); err != nil {
		return contracts.CustomMap{}, err
	}
	if _, err := tx.Exec(ctx, `delete from locations where map_storage_id=$1`, mapStorageID); err != nil {
		return contracts.CustomMap{}, err
	}
	block := make([][]any, 0, len(parsed))
	for _, row := range parsed {
		block = append(block, []any{mapStorageID, row.LatE7, row.LngE7, row.Country, row.PanoID, row.HeadingCDeg, row.PitchCDeg, row.RandKey})
	}
	if _, err := tx.CopyFrom(ctx, pgx.Identifier{"locations"}, []string{"map_storage_id", "lat_e7", "lng_e7", "country", "pano_id", "heading_cdeg", "pitch_cdeg", "rand_key_i"}, pgx.CopyFromRows(block)); err != nil {
		return contracts.CustomMap{}, err
	}
	if _, err := tx.Exec(ctx, `delete from map_country_stats where map_id=$1`, mapID); err != nil {
		return contracts.CustomMap{}, err
	}
	if _, err := tx.Exec(ctx, `
		insert into map_country_stats(map_id,country,location_count)
		select $1,coalesce(nullif(country,''),'Unknown'),count(*)::int
		from locations where map_storage_id=$2
		group by coalesce(nullif(country,''), 'Unknown')
	`, mapID, mapStorageID); err != nil {
		return contracts.CustomMap{}, err
	}
	boundsMinLat, boundsMaxLat, boundsMinLng, boundsMaxLng := computePlayRegionBoundsE7(parsed)
	if _, err := tx.Exec(ctx, `
		update maps set status='ready',location_count=$2,
			bounds_min_lat_e7=$3,bounds_max_lat_e7=$4,bounds_min_lng_e7=$5,bounds_max_lng_e7=$6,
			updated_at=now()
		where id=$1
	`, mapID, len(parsed), boundsMinLat, boundsMaxLat, boundsMinLng, boundsMaxLng); err != nil {
		return contracts.CustomMap{}, err
	}
	if _, err := tx.Exec(ctx, `insert into map_aliases(alias,map_id) values($1,$2) on conflict(alias) do update set map_id=excluded.map_id`, mapKey, mapID); err != nil {
		return contracts.CustomMap{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return contracts.CustomMap{}, err
	}
	details, ok, err := s.GetMap(adminUserID, mapID)
	if err != nil || !ok {
		return contracts.CustomMap{}, err
	}
	return details.Map, nil
}

func (s *pgStore) ReplaceCustomMapLocations(userID, mapID string, source io.Reader) (contracts.CustomMap, error) {
	return s.ingestCustomMap(userID, strings.TrimSpace(mapID), "", "", "", "", "", 0, source, false)
}

func (s *pgStore) ingestCustomMap(userID, mapID, displayName, description, visibility, difficulty, thumbnailKey string, thumbnailVariant int, source io.Reader, create bool) (contracts.CustomMap, error) {
	userID, mapID = strings.TrimSpace(userID), strings.TrimSpace(mapID)
	if userID == "" || mapID == "" {
		return contracts.CustomMap{}, errors.New("user and map required")
	}
	if create {
		displayName = strings.TrimSpace(displayName)
		if displayName == "" || len(displayName) > 80 {
			return contracts.CustomMap{}, errors.New("map name must be 1 to 80 characters")
		}
		if len(description) > 500 {
			return contracts.CustomMap{}, errors.New("description must be at most 500 characters")
		}
		difficulty = normalizeMapDifficulty(difficulty)
		thumbnailVariant = normalizeThumbnailVariant(thumbnailVariant)
		visibility = normalizeMapVisibility(visibility)
		thumbnailKey = normalizeThumbnailKey(thumbnailKey, thumbnailVariant)
	}
	quota, err := s.GetMapUploadQuota(userID)
	if err != nil {
		return contracts.CustomMap{}, err
	}
	parsed, digest, rejected, err := decodeMapRows(source, quota.MaxMapLocations)
	if err != nil {
		return contracts.CustomMap{}, err
	}
	if len(parsed) < minMapLocations {
		return contracts.CustomMap{}, fmt.Errorf("map requires at least %d valid locations", minMapLocations)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return contracts.CustomMap{}, err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `select pg_advisory_xact_lock(hashtext($1))`, "map-upload:"+userID); err != nil {
		return contracts.CustomMap{}, err
	}
	if err := enforceMapUploadQuota(ctx, tx, userID, mapID, len(parsed), create); err != nil {
		return contracts.CustomMap{}, err
	}
	if create {
		_, err = tx.Exec(ctx, `
			insert into maps(id,map_key,owner_user_id,display_name,description,visibility,status,difficulty,thumbnail_variant,thumbnail_key,location_count,published_at,created_at,updated_at)
			values($1,$1,$2,$3,$4,$5,'processing',$6,$7,$8,0,case when $5='public' then now() else null end,now(),now())
		`, mapID, userID, displayName, strings.TrimSpace(description), visibility, difficulty, thumbnailVariant, thumbnailKey)
	} else {
		var owner string
		var canonicalID string
		if canonicalID, _, err = resolveMapIdentity(ctx, tx, mapID); err == nil {
			mapID = canonicalID
		}
		if err == nil {
			err = tx.QueryRow(ctx, `select coalesce(owner_user_id::text,'') from maps where id=$1 and archived_at is null for update`, mapID).Scan(&owner)
		}
		if err == nil && owner != userID {
			err = errors.New("map is not owned by this account")
		}
	}
	if err != nil {
		return contracts.CustomMap{}, err
	}
	var mapStorageID int32
	if err := tx.QueryRow(ctx, `select storage_id from maps where id=$1`, mapID).Scan(&mapStorageID); err != nil {
		return contracts.CustomMap{}, err
	}

	if _, err := tx.Exec(ctx, `delete from locations where map_storage_id=$1`, mapStorageID); err != nil {
		return contracts.CustomMap{}, err
	}
	block := make([][]any, 0, len(parsed))
	for _, row := range parsed {
		block = append(block, []any{mapStorageID, row.LatE7, row.LngE7, row.Country, row.PanoID, row.HeadingCDeg, row.PitchCDeg, row.RandKey})
	}
	if _, err := tx.CopyFrom(ctx, pgx.Identifier{"locations"}, []string{"map_storage_id", "lat_e7", "lng_e7", "country", "pano_id", "heading_cdeg", "pitch_cdeg", "rand_key_i"}, pgx.CopyFromRows(block)); err != nil {
		return contracts.CustomMap{}, err
	}
	if _, err := tx.Exec(ctx, `delete from map_country_stats where map_id=$1`, mapID); err != nil {
		return contracts.CustomMap{}, err
	}
	if _, err := tx.Exec(ctx, `
		insert into map_country_stats(map_id,country,location_count)
		select $1,coalesce(nullif(country,''),'Unknown'),count(*)::int
		from locations where map_storage_id=$2
		group by coalesce(nullif(country,''), 'Unknown')
	`, mapID, mapStorageID); err != nil {
		return contracts.CustomMap{}, err
	}
	digestBytes, err := hex.DecodeString(digest)
	if err != nil {
		return contracts.CustomMap{}, err
	}
	boundsMinLat, boundsMaxLat, boundsMinLng, boundsMaxLng := computePlayRegionBoundsE7(parsed)
	if _, err := tx.Exec(ctx, `
		update maps set status='ready',location_count=$2,content_hash=$3,rejected_location_count=$4,
			bounds_min_lat_e7=$5,bounds_max_lat_e7=$6,bounds_min_lng_e7=$7,bounds_max_lng_e7=$8,
			updated_at=now()
		where id=$1
	`, mapID, len(parsed), digestBytes, rejected, boundsMinLat, boundsMaxLat, boundsMinLng, boundsMaxLng); err != nil {
		return contracts.CustomMap{}, err
	}
	if _, err := tx.Exec(ctx, `insert into map_aliases(alias,map_id) select map_key,id from maps where id=$1 on conflict(alias) do update set map_id=excluded.map_id`, mapID); err != nil {
		return contracts.CustomMap{}, err
	}
	if _, err := tx.Exec(ctx, `insert into map_upload_events(user_id,map_id,location_count) values($1,$2,$3)`, userID, mapID, len(parsed)); err != nil {
		return contracts.CustomMap{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return contracts.CustomMap{}, err
	}
	details, ok, err := s.GetMap(userID, mapID)
	if err != nil || !ok {
		return contracts.CustomMap{}, err
	}
	return details.Map, nil
}

func (s *pgStore) UpdateCustomMap(userID, mapID string, update contracts.CustomMapUpdate) (contracts.CustomMap, error) {
	name := strings.TrimSpace(update.DisplayName)
	if name == "" || len(name) > 80 || len(update.Description) > 500 {
		return contracts.CustomMap{}, errors.New("invalid map details")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	canonicalID, _, err := resolveMapIdentity(ctx, s.pool, mapID)
	if err != nil {
		return contracts.CustomMap{}, err
	}
	mapID = canonicalID
	tag, err := s.pool.Exec(ctx, `
		update maps
		set display_name=$3, description=$4, visibility=$5, difficulty=$6, thumbnail_variant=$7, thumbnail_key=$8, auto_zoom_play_region=$9, updated_at=now()
		where id=$1 and owner_user_id=$2 and archived_at is null
	`, strings.TrimSpace(mapID), strings.TrimSpace(userID), name, strings.TrimSpace(update.Description), normalizeMapVisibility(update.Visibility), normalizeMapDifficulty(update.Difficulty), normalizeThumbnailVariant(update.ThumbnailVariant), normalizeThumbnailKey(update.ThumbnailKey, update.ThumbnailVariant), update.AutoZoomPlayRegion)
	if err != nil {
		return contracts.CustomMap{}, err
	}
	if tag.RowsAffected() == 0 {
		return contracts.CustomMap{}, pgx.ErrNoRows
	}
	details, _, err := s.GetMap(userID, mapID)
	return details.Map, err
}

func (s *pgStore) PublishCustomMap(userID, mapID string) (contracts.CustomMap, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	canonicalID, _, err := resolveMapIdentity(ctx, s.pool, mapID)
	if err != nil {
		return contracts.CustomMap{}, err
	}
	mapID = canonicalID
	tag, err := s.pool.Exec(ctx, `
		update maps
		set visibility='public', published_at=coalesce(published_at, now()), updated_at=now()
		where id=$1 and owner_user_id=$2 and archived_at is null and status='ready'
	`, strings.TrimSpace(mapID), strings.TrimSpace(userID))
	if err != nil {
		return contracts.CustomMap{}, err
	}
	if tag.RowsAffected() == 0 {
		return contracts.CustomMap{}, pgx.ErrNoRows
	}
	details, _, err := s.GetMap(userID, mapID)
	return details.Map, err
}

func (s *pgStore) SetMapFavorite(userID, mapID string, favorite bool) (contracts.CustomMap, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return contracts.CustomMap{}, err
	}
	defer tx.Rollback(ctx)
	canonicalID, _, err := resolveMapIdentity(ctx, tx, mapID)
	if err != nil {
		return contracts.CustomMap{}, err
	}
	mapID = canonicalID
	var visible bool
	var ownerUserID string
	if err := tx.QueryRow(ctx, `
		select
			exists(select 1 from maps where id=$1 and archived_at is null and `+mapVisibleToUserSQL("maps", 2, true)+`),
			coalesce((select owner_user_id::text from maps where id=$1 and archived_at is null), '')
	`, strings.TrimSpace(mapID), strings.TrimSpace(userID)).Scan(&visible, &ownerUserID); err != nil {
		return contracts.CustomMap{}, err
	}
	if !visible {
		return contracts.CustomMap{}, pgx.ErrNoRows
	}
	changed := false
	if favorite {
		tag, err := tx.Exec(ctx, `insert into map_favorites(map_id,user_id) values($1,$2) on conflict do nothing`, strings.TrimSpace(mapID), strings.TrimSpace(userID))
		if err != nil {
			return contracts.CustomMap{}, err
		}
		if tag.RowsAffected() > 0 {
			changed = true
			if err := incrementMapFavoriteStats(ctx, tx, strings.TrimSpace(mapID), strings.TrimSpace(userID)); err != nil {
				return contracts.CustomMap{}, err
			}
		}
	} else {
		tag, err := tx.Exec(ctx, `delete from map_favorites where map_id=$1 and user_id=$2`, strings.TrimSpace(mapID), strings.TrimSpace(userID))
		if err != nil {
			return contracts.CustomMap{}, err
		}
		if tag.RowsAffected() > 0 {
			changed = true
			if _, err := tx.Exec(ctx, `update maps set favorite_count=greatest(favorite_count-1,0), updated_at=now() where id=$1`, strings.TrimSpace(mapID)); err != nil {
				return contracts.CustomMap{}, err
			}
			if err := refreshMapTrendingScore(ctx, tx, strings.TrimSpace(mapID)); err != nil {
				return contracts.CustomMap{}, err
			}
		}
	}
	if changed && ownerUserID != "" {
		if _, err := refreshMapCreatorTrust(ctx, tx, ownerUserID); err != nil {
			return contracts.CustomMap{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return contracts.CustomMap{}, err
	}
	details, ok, err := s.GetMap(userID, mapID)
	if err != nil || !ok {
		return contracts.CustomMap{}, err
	}
	return details.Map, nil
}

func (s *pgStore) ArchiveCustomMap(userID, mapID string, allowAnyMap bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	canonicalID, _, err := resolveMapIdentity(ctx, tx, mapID)
	if err != nil {
		return err
	}
	mapID = canonicalID
	tag, err := tx.Exec(ctx, `
		delete from maps m
		where m.id=$1
		  and (m.owner_user_id=$2 or $3)
		  and m.archived_at is null
		  and not exists(select 1 from match_round_plans p where p.map_id=m.id)
		  and not exists(select 1 from parties p where p.map_id=m.id)
	`, strings.TrimSpace(mapID), strings.TrimSpace(userID), allowAnyMap)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		tag, err = tx.Exec(ctx, `
			update maps
			set status='archived', archived_at=now(), updated_at=now()
			where id=$1
			  and (owner_user_id=$2 or $3)
			  and archived_at is null
		`, strings.TrimSpace(mapID), strings.TrimSpace(userID), allowAnyMap)
	}
	if err == nil && tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}
