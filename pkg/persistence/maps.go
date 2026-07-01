package persistence

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"geoduels/pkg/contracts"
)

const (
	absoluteMaxMapLocations   = 1_000_000
	minMapLocations           = 5
	plannedRoundCount         = 20
	mapTrendingWindowDays     = 7
	communityMapListPredicate = "m.owner_user_id is not null and m.visibility='public' and m.status='ready'"
)

type MapCatalog interface {
	ListMaps(userID string, opts contracts.MapListOptions) ([]contracts.CustomMap, error)
	GetMap(userID, mapID string) (contracts.MapDetails, bool, error)
	GetMapUploadQuota(userID string) (contracts.MapUploadQuota, error)
	CreateCustomMap(userID, displayName, description, visibility, difficulty, thumbnailKey string, thumbnailVariant int, source io.Reader) (contracts.CustomMap, error)
	ImportOfficialMap(adminUserID string, input OfficialMapImportInput, source io.Reader) (contracts.CustomMap, error)
	ReplaceCustomMapLocations(userID, mapID string, source io.Reader) (contracts.CustomMap, error)
	UpdateCustomMap(userID, mapID string, update contracts.CustomMapUpdate) (contracts.CustomMap, error)
	PublishCustomMap(userID, mapID string) (contracts.CustomMap, error)
	SetMapFavorite(userID, mapID string, favorite bool) (contracts.CustomMap, error)
	SetMapOfficial(adminUserID, mapID string, official bool) (contracts.CustomMap, error)
	SetGameplayMapRole(adminUserID, mapID, role string) (contracts.CustomMap, error)
	ListMapComments(userID, mapID string) ([]contracts.MapComment, error)
	CreateMapComment(userID, mapID string, input contracts.MapCommentCreate) (contracts.MapComment, error)
	DeleteMapComment(userID, mapID, commentID string, moderator bool) error
	SetMapCommentLike(userID, mapID, commentID string, liked bool) (contracts.MapComment, error)
	ArchiveCustomMap(userID, mapID string, allowAnyMap bool) error
	PrepareMatchPlan(ctx context.Context, found *contracts.MatchFound) error
}

type OfficialMapImportInput struct {
	MapKey             string
	DisplayName        string
	Description        string
	Visibility         string
	Difficulty         string
	ThumbnailKey       string
	ThumbnailVariant   int
	OfficialRegionType string
	OfficialRegionCode string
}

func (s *pgStore) ListMaps(userID string, opts contracts.MapListOptions) ([]contracts.CustomMap, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	scope := normalizeMapScope(opts.Scope)
	sortMode := normalizeMapSort(opts.Sort)
	searchPattern := mapSearchPattern(opts.Search)
	query := `
		select m.id::text,m.map_key,coalesce(m.owner_user_id::text, ''), case when m.official_at is not null then 'GeoDuels' else coalesce(u.display_name, 'GeoDuels') end, m.display_name, m.description, m.visibility, m.status,
		       m.difficulty, m.thumbnail_variant, coalesce(m.thumbnail_key, 'generic/variant-' || greatest(1, least(5, m.thumbnail_variant))::text), m.location_count, (m.owner_user_id is null or m.official_at is not null),
		       m.official_at is not null,
		       coalesce(m.published_at, '0001-01-01'::timestamptz), m.play_count, m.favorite_count, m.comment_count, m.trending_score,
		       exists(select 1 from map_favorites mf where mf.map_id=m.id and mf.user_id=nullif($1,'')::uuid),
		       trim(both ':' from concat_ws(':', nullif(m.official_region_type,''), nullif(m.official_region_code,''))),
		       coalesce((select value_json->>'rankedMovingMapId' from site_settings where key='gameplay_map_settings'), '') in (m.id::text,m.map_key),
		       coalesce((select value_json->>'rankedNmpzMapId' from site_settings where key='gameplay_map_settings'), '') in (m.id::text,m.map_key),
		       coalesce((select value_json->>'singleplayerMovingMapId' from site_settings where key='gameplay_map_settings'), '') in (m.id::text,m.map_key),
		       coalesce((select value_json->>'singleplayerNmpzMapId' from site_settings where key='gameplay_map_settings'), '') in (m.id::text,m.map_key),
		       m.created_at,m.updated_at,pb.best_score,coalesce(pb.match_id::text,''),pb.achieved_at
		from maps m
		left join users u on u.id = m.owner_user_id
		left join player_map_bests pb on pb.map_id=m.id and pb.user_id=nullif($1,'')::uuid and pb.ruleset=0
		where m.archived_at is null
	`
	args := []any{strings.TrimSpace(userID)}
	switch scope {
	case "official":
		query += ` and (m.owner_user_id is null or m.official_at is not null)`
	case "community":
		query += ` and ` + communityMapListPredicate
	case "favorites":
		query += ` and exists(select 1 from map_favorites mf where mf.map_id=m.id and mf.user_id=nullif($1,'')::uuid)`
	case "mine":
		query += ` and m.owner_user_id = nullif($1,'')::uuid`
	default:
		query += ` and (m.owner_user_id is null or m.official_at is not null or m.owner_user_id = nullif($1,'')::uuid)`
	}
	if searchPattern != "" {
		args = append(args, searchPattern)
		searchArg := len(args)
		query += fmt.Sprintf(` and (
			m.display_name ilike $%[1]d escape '\'
			or m.description ilike $%[1]d escape '\'
			or m.map_key ilike $%[1]d escape '\'
			or coalesce(u.display_name, 'GeoDuels') ilike $%[1]d escape '\'
			or trim(both ':' from concat_ws(':', nullif(m.official_region_type,''), nullif(m.official_region_code,''))) ilike $%[1]d escape '\'
		)`, searchArg)
	}
	switch sortMode {
	case "popular":
		query += ` order by (m.play_count + m.favorite_count * 3) desc, m.published_at desc nulls last, m.updated_at desc`
	case "new":
		query += ` order by m.published_at desc nulls last, m.updated_at desc`
	default:
		query += ` order by m.trending_score desc, m.published_at desc nulls last, m.updated_at desc`
	}
	query += ` limit 72`
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []contracts.CustomMap{}
	for rows.Next() {
		item, err := scanCustomMap(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *pgStore) GetMap(userID, mapID string) (contracts.MapDetails, bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	row := s.pool.QueryRow(ctx, `
		select m.id::text,m.map_key,coalesce(m.owner_user_id::text, ''), case when m.official_at is not null then 'GeoDuels' else coalesce(u.display_name, 'GeoDuels') end, m.display_name, m.description, m.visibility, m.status,
		       m.difficulty, m.thumbnail_variant, coalesce(m.thumbnail_key, 'generic/variant-' || greatest(1, least(5, m.thumbnail_variant))::text), m.location_count, (m.owner_user_id is null or m.official_at is not null),
		       m.official_at is not null,
		       coalesce(m.published_at, '0001-01-01'::timestamptz), m.play_count, m.favorite_count, m.comment_count, m.trending_score,
		       exists(select 1 from map_favorites mf where mf.map_id=m.id and mf.user_id=nullif($2,'')::uuid),
		       trim(both ':' from concat_ws(':', nullif(m.official_region_type,''), nullif(m.official_region_code,''))),
		       coalesce((select value_json->>'rankedMovingMapId' from site_settings where key='gameplay_map_settings'), '') in (m.id::text,m.map_key),
		       coalesce((select value_json->>'rankedNmpzMapId' from site_settings where key='gameplay_map_settings'), '') in (m.id::text,m.map_key),
		       coalesce((select value_json->>'singleplayerMovingMapId' from site_settings where key='gameplay_map_settings'), '') in (m.id::text,m.map_key),
		       coalesce((select value_json->>'singleplayerNmpzMapId' from site_settings where key='gameplay_map_settings'), '') in (m.id::text,m.map_key),
		       m.created_at,m.updated_at,pb.best_score,coalesce(pb.match_id::text,''),pb.achieved_at
		from maps m
		left join users u on u.id = m.owner_user_id
		left join player_map_bests pb on pb.map_id=m.id and pb.user_id=nullif($2,'')::uuid and pb.ruleset=0
		where (m.id::text=$1 or m.map_key=$1 or exists(select 1 from map_aliases a where a.map_id=m.id and a.alias=$1)) and m.archived_at is null
		  and `+mapVisibleToUserSQL("m", 2, true)+`
	`, strings.TrimSpace(mapID), strings.TrimSpace(userID))
	item, err := scanCustomMap(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return contracts.MapDetails{}, false, nil
	}
	if err != nil {
		return contracts.MapDetails{}, false, err
	}
	stats, err := s.mapCountryStats(ctx, item.ID)
	if err != nil {
		return contracts.MapDetails{}, false, err
	}
	comments, err := s.listMapComments(ctx, strings.TrimSpace(userID), item.ID)
	if err != nil {
		return contracts.MapDetails{}, false, err
	}
	return contracts.MapDetails{Map: item, CountryStats: stats, Comments: comments}, true, nil
}
