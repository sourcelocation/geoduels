package maps

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"geoduels/internal/rating"
	"geoduels/internal/storekit"
	"geoduels/pkg/entityid"
	db "geoduels/pkg/persistence/sqlc/db"
)

// PGStore owns PostgreSQL access for the maps feature: the catalog, uploads,
// comments, creator trust, gameplay roles, official map revisions, and the
// gameplay map settings used at match launch.
type PGStore struct {
	pool *pgxpool.Pool
	db   *db.Queries
}

func NewPGStore(pool *pgxpool.Pool) *PGStore {
	return &PGStore{pool: pool, db: db.New(pool)}
}

func (s *PGStore) withTx(ctx context.Context, fn func(pgx.Tx) error) error {
	return storekit.WithTx(ctx, s.pool, fn)
}

func (s *PGStore) q() *db.Queries { return db.New(s.pool) }

const modeDuel = "duel"

func mapUUID(value string) (pgtype.UUID, error) {
	var u pgtype.UUID
	return u, u.Scan(strings.TrimSpace(value))
}

func mustMapUUID(value string) pgtype.UUID {
	id, err := mapUUID(value)
	if err != nil {
		return pgtype.UUID{}
	}
	return id
}

func anyText(v any) string {
	switch value := v.(type) {
	case string:
		return value
	case pgtype.UUID:
		return storekit.UUIDVal(value)
	}
	return ""
}

func textVal(value any) string {
	if value == nil {
		return ""
	}
	return fmt.Sprint(value)
}

// resolveMapIdentity resolves a public map ID or alias to its canonical ID and
// key. Persistence keeps its own copy for match plans and parties so the
// persistence package never depends on this feature package.
func resolveMapIdentity(ctx context.Context, q db.DBTX, identifier string) (string, string, error) {
	return ResolveMapIdentity(ctx, q, identifier)
}

func ResolveMapIdentity(ctx context.Context, q db.DBTX, identifier string) (string, string, error) {
	identifier = strings.TrimSpace(identifier)
	params := db.ResolveMapIdentityParams{}
	if canonicalID, err := entityid.Parse(identifier); err == nil {
		if err := params.MapID.Scan(canonicalID); err != nil {
			return "", "", err
		}
	} else {
		params.Alias = pgtype.Text{String: identifier, Valid: identifier != ""}
	}
	row, err := db.New(q).ResolveMapIdentity(ctx, params)
	return storekit.UUIDVal(row.ID), textVal(row.Key), err
}

func mapFromQueryRow(r db.ListMapsRow) Map {
	item := mapFromParts(r.ID, r.MapKey, r.OwnerUserID, r.AuthorDisplayName, r.DisplayName, r.Description, r.Visibility, r.Status, r.Difficulty, r.ThumbnailVariant, r.ThumbnailKey, r.LocationCount, r.IsSystem, r.IsOfficial, r.PublishedAt, r.PlayCount, r.FavoriteCount, r.CommentCount, r.TrendingScore, r.Favorited, r.OfficialRegion, r.ModeMoving, r.ModeNoMove, r.ModeNmpz, r.CreatedAt, r.UpdatedAt, r.BestScore, r.BestMatchID, r.AchievedAt)
	item.MOTWCount = int(r.MotwCount)
	item.MOTWCurrent = r.MotwCurrent
	if r.MotwLastAt.Valid {
		v := r.MotwLastAt.Time
		item.MOTWLastAt = &v
	}
	return item
}

func mapFromGetRow(r db.GetMapRow) Map {
	item := mapFromParts(r.ID, r.MapKey, r.OwnerUserID, r.AuthorDisplayName, r.DisplayName, r.Description, r.Visibility, r.Status, r.Difficulty, r.ThumbnailVariant, r.ThumbnailKey, r.LocationCount, r.IsSystem, r.IsOfficial, r.PublishedAt, r.PlayCount, r.FavoriteCount, r.CommentCount, r.TrendingScore, r.Favorited, r.OfficialRegion, r.ModeMoving, r.ModeNoMove, r.ModeNmpz, r.CreatedAt, r.UpdatedAt, r.BestScore, r.BestMatchID, r.AchievedAt)
	item.MOTWCount = int(r.MotwCount)
	item.MOTWCurrent = r.MotwCurrent
	if r.MotwLastAt.Valid {
		v := r.MotwLastAt.Time
		item.MOTWLastAt = &v
	}
	return item
}

func mapFromParts(id any, key, owner, author any, name, desc string, vis db.GdMapVisibility, status db.GdMapStatus, diff db.GdMapDifficulty, thumbVariant int32, thumbKey string, count int32, system pgtype.Bool, official any, published pgtype.Timestamptz, plays, favs, comments int32, trend float64, favorited bool, region []byte, moving, noMove, nmpz bool, created, updated pgtype.Timestamptz, best pgtype.Int2, match any, achieved pgtype.Timestamptz) Map {
	r := Map{ID: anyText(id), MapKey: fmt.Sprint(key), OwnerUserID: anyText(owner), AuthorName: fmt.Sprint(author), DisplayName: name, Description: desc, Visibility: string(vis), Status: string(status), Difficulty: string(diff), ThumbnailVariant: int(thumbVariant), ThumbnailKey: thumbKey, LocationCount: int(count), System: system.Bool, Official: fmt.Sprint(official) == "true", PlayCount: int(plays), FavoriteCount: int(favs), CommentCount: int(comments), TrendingScore: trend, Favorited: favorited, OfficialRegion: string(region), ModeMoving: moving, ModeNoMove: noMove, ModeNMPZ: nmpz, CreatedAt: created.Time, UpdatedAt: updated.Time}
	if published.Valid {
		t := published.Time
		r.PublishedAt = &t
	}
	if best.Valid && achieved.Valid {
		r.PersonalBest = &PersonalBest{Score: int(best.Int16), MatchID: anyText(match), AchievedAt: achieved.Time}
	}
	return r
}

func (s *PGStore) ListMaps(userID string, opts ListOptions) ([]Map, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	scope := normalizeMapScope(opts.Scope)
	sortMode := normalizeMapSort(opts.Sort)
	searchPattern := mapSearchPattern(opts.Search)
	rows, err := s.q().ListMaps(ctx, db.ListMapsParams{ViewerUserID: strings.TrimSpace(userID), Scope: scope, Search: searchPattern, Sort: sortMode})
	if err != nil {
		return nil, err
	}
	out := make([]Map, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapFromQueryRow(row))
	}
	return out, nil
}

func (s *PGStore) GetMap(userID, mapID string) (Details, bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	parsedID, parseErr := storekit.ProfileUUID(strings.TrimSpace(mapID))
	if parseErr != nil {
		return Details{}, false, nil
	}
	row, err := s.q().GetMap(ctx, db.GetMapParams{ViewerUserID: strings.TrimSpace(userID), MapID: parsedID})
	if errors.Is(err, pgx.ErrNoRows) {
		return Details{}, false, nil
	}
	if err != nil {
		return Details{}, false, err
	}
	item := mapFromGetRow(row)
	stats, err := s.mapCountryStats(ctx, item.ID)
	if err != nil {
		return Details{}, false, err
	}
	comments, err := s.listMapComments(ctx, strings.TrimSpace(userID), item.ID)
	if err != nil {
		return Details{}, false, err
	}
	return Details{Map: item, CountryStats: stats, Comments: comments}, true, nil
}

func (s *PGStore) mapCountryStats(ctx context.Context, mapID string) ([]CountryStat, error) {
	if strings.TrimSpace(mapID) == "" {
		return nil, nil
	}
	rows, err := s.q().ListMapCountryStats(ctx, mustMapUUID(mapID))
	if err != nil {
		return nil, err
	}
	out := make([]CountryStat, 0, len(rows))
	for _, row := range rows {
		out = append(out, CountryStat{Country: row.Country, LocationCount: int(row.LocationCount)})
	}
	return out, nil
}

// commentModerationRights reports whether the viewer may see (and delete)
// hidden comments. The former persistence path went through GetProfile; only
// the admin/moderator flags matter here.
func (s *PGStore) commentModerationRights(ctx context.Context, userID string) bool {
	seasonID, err := storekit.ActiveSeasonID(ctx, s.pool)
	if err != nil {
		return false
	}
	row, err := s.q().GetProfile(ctx, db.GetProfileParams{
		UserID: strings.TrimSpace(userID), Mode: db.GdMatchMode(modeDuel), SeasonID: seasonID,
		DefaultMmr: rating.InitialMMR, DefaultRd: rating.InitialRatingRD,
	})
	if err != nil {
		return false
	}
	return row.IsAdmin || row.IsModerator
}
