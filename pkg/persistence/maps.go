package persistence

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"geoduels/pkg/contracts"
	db "geoduels/pkg/persistence/sqlc/db"
)

func mapFromQueryRow(r db.ListMapsRow) contracts.CustomMap {
	return mapFromParts(r.ID, r.MapKey, r.OwnerUserID, r.AuthorDisplayName, r.DisplayName, r.Description, r.Visibility, r.Status, r.Difficulty, r.ThumbnailVariant, r.ThumbnailKey, r.LocationCount, r.IsSystem, r.IsOfficial, r.PublishedAt, r.PlayCount, r.FavoriteCount, r.CommentCount, r.TrendingScore, r.Favorited, r.OfficialRegion, r.ModeMoving, r.ModeNoMove, r.ModeNmpz, r.CreatedAt, r.UpdatedAt, r.BestScore, r.BestMatchID, r.AchievedAt)
}
func mapFromGetRow(r db.GetMapRow) contracts.CustomMap {
	return mapFromParts(r.ID, r.MapKey, r.OwnerUserID, r.AuthorDisplayName, r.DisplayName, r.Description, r.Visibility, r.Status, r.Difficulty, r.ThumbnailVariant, r.ThumbnailKey, r.LocationCount, r.IsSystem, r.IsOfficial, r.PublishedAt, r.PlayCount, r.FavoriteCount, r.CommentCount, r.TrendingScore, r.Favorited, r.OfficialRegion, r.ModeMoving, r.ModeNoMove, r.ModeNmpz, r.CreatedAt, r.UpdatedAt, r.BestScore, r.BestMatchID, r.AchievedAt)
}
func mapFromParts(id string, key, owner, author any, name, desc string, vis db.GdMapVisibility, status db.GdMapStatus, diff db.GdMapDifficulty, thumbVariant int32, thumbKey string, count int32, system pgtype.Bool, official any, published pgtype.Timestamptz, plays, favs, comments int32, trend float64, favorited bool, region []byte, moving, noMove, nmpz bool, created, updated pgtype.Timestamptz, best pgtype.Int2, match any, achieved pgtype.Timestamptz) contracts.CustomMap {
	r := contracts.CustomMap{ID: id, MapKey: fmt.Sprint(key), OwnerUserID: fmt.Sprint(owner), AuthorName: fmt.Sprint(author), DisplayName: name, Description: desc, Visibility: string(vis), Status: string(status), Difficulty: string(diff), ThumbnailVariant: int(thumbVariant), ThumbnailKey: thumbKey, LocationCount: int(count), System: system.Bool, Official: fmt.Sprint(official) == "true", PlayCount: int(plays), FavoriteCount: int(favs), CommentCount: int(comments), TrendingScore: trend, Favorited: favorited, OfficialRegion: string(region), ModeMoving: moving, ModeNoMove: noMove, ModeNMPZ: nmpz, CreatedAt: created.Time, UpdatedAt: updated.Time}
	if published.Valid {
		t := published.Time
		r.PublishedAt = &t
	}
	if best.Valid && achieved.Valid {
		r.PersonalBest = &contracts.MapPersonalBest{Score: int(best.Int16), MatchID: fmt.Sprint(match), AchievedAt: achieved.Time}
	}
	return r
}

const (
	absoluteMaxMapLocations   = 1_000_000
	minMapLocations           = 5
	plannedRoundCount         = 20
	mapTrendingWindowDays     = 7
	communityMapListPredicate = "m.owner_user_id is not null and m.visibility='public' and m.status='ready'"
)

func gameplayMapSettingMatchSQL(jsonKey, defaultAlias string) string {
	return fmt.Sprintf(`exists(select 1 from map_aliases a where a.map_id=m.id and a.alias=coalesce((select value_json->>'%[1]s' from site_settings where key='gameplay_map_settings'),'%[2]s')) or coalesce((select value_json->>'%[1]s' from site_settings where key='gameplay_map_settings'),'%[2]s')=m.id::text`, jsonKey, defaultAlias)
}

func gameplayMapRoleFlagsSQL() string {
	return strings.Join([]string{
		gameplayMapSettingMatchSQL("movingMapId", contracts.MapKeyMoving),
		gameplayMapSettingMatchSQL("noMoveMapId", contracts.MapKeyNMPZ),
		gameplayMapSettingMatchSQL("nmpzMapId", contracts.MapKeyNMPZ),
	}, ",\n\t\t       ")
}

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

func (s *DB) ListMaps(userID string, opts contracts.MapListOptions) ([]contracts.CustomMap, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	scope := normalizeMapScope(opts.Scope)
	sortMode := normalizeMapSort(opts.Sort)
	searchPattern := mapSearchPattern(opts.Search)
	rows, err := s.db.ListMaps(ctx, db.ListMapsParams{ViewerUserID: strings.TrimSpace(userID), Scope: scope, Search: searchPattern, Sort: sortMode})
	if err != nil {
		return nil, err
	}
	out := make([]contracts.CustomMap, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapFromQueryRow(row))
	}
	return out, nil
}

func (s *DB) GetMap(userID, mapID string) (contracts.MapDetails, bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	parsedID, parseErr := profileUUID(strings.TrimSpace(mapID))
	if parseErr != nil {
		return contracts.MapDetails{}, false, nil
	}
	row, err := s.db.GetMap(ctx, db.GetMapParams{ViewerUserID: strings.TrimSpace(userID), MapKey: parsedID})
	if errors.Is(err, pgx.ErrNoRows) {
		return contracts.MapDetails{}, false, nil
	}
	if err != nil {
		return contracts.MapDetails{}, false, err
	}
	item := mapFromGetRow(row)
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
