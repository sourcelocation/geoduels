// Package maps owns the custom-map domain: the public catalog, uploads and
// revisions, comments, creator trust tiers, and official map administration.
package maps

import "time"

// The built-in maps every install has; gameplay falls back to them.
const (
	MapKeyMoving = "a-source-world"
	MapKeyNMPZ   = "a-location-world"
)

// Map is a map in the catalog, as one viewer sees it: PersonalBest and Favorited are theirs.
type Map struct {
	ID               string
	MapKey           string
	OwnerUserID      string
	AuthorName       string
	DisplayName      string
	Description      string
	Visibility       string
	Status           string
	Difficulty       string
	ThumbnailVariant int
	ThumbnailKey     string
	LocationCount    int
	PersonalBest     *PersonalBest
	System           bool
	Official         bool
	PublishedAt      *time.Time
	PlayCount        int
	FavoriteCount    int
	CommentCount     int
	TrendingScore    float64
	Favorited        bool
	OfficialRegion   string
	ModeMoving       bool
	ModeNoMove       bool
	ModeNMPZ         bool
	MOTWCount        int
	MOTWCurrent      bool
	MOTWLastAt       *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// PersonalBest is a viewer's best solo score on a map.
type PersonalBest struct {
	Score      int
	MatchID    string
	AchievedAt time.Time
}

// MapEdit is an owner's change to a map's details.
type MapEdit struct {
	DisplayName      string
	Description      string
	Visibility       string
	Difficulty       string
	ThumbnailVariant int
	ThumbnailKey     string
}

// ListOptions picks and orders the maps a catalog listing shows.
type ListOptions struct {
	Scope  string
	Sort   string
	Search string
}

// Details is a map's page: the map, where its locations are, and its comments.
type Details struct {
	Map          Map
	CountryStats []CountryStat
	Comments     []Comment
}

type CountryStat struct {
	Country       string
	LocationCount int
}

// Comment is a comment on a map, with its replies when it starts a thread.
type Comment struct {
	ID              string
	MapID           string
	ParentID        string
	UserID          string
	UserDisplayName string
	AvatarURL       string
	Body            string
	Status          string
	CanDelete       bool
	LikeCount       int
	Liked           bool
	CreatedAt       time.Time
	UpdatedAt       time.Time
	Replies         []Comment
}

// NewComment is a comment someone posts, or a reply when ParentID is set.
type NewComment struct {
	Body     string
	ParentID string
}

// UploadQuota is what a creator's trust tier lets them upload, and how much they use.
type UploadQuota struct {
	Tier                     string
	TierOverride             string
	QualifiedFavorites       int
	QualifiedMaps            int
	AccountAgeDays           int
	NextTier                 string
	FavoritesNeeded          int
	DaysNeeded               int
	MaxMaps                  int
	MaxActiveLocations       int
	MaxMapLocations          int
	MaxUploadsPerHour        int
	MaxUploadsPerDay         int
	MaxUploadedLocationsHour int
	CurrentMaps              int
	CurrentActiveLocations   int
	RestrictedByModeration   bool
}

// ImportSummary is an official map's dataset after an import or revision.
type ImportSummary struct {
	MapID         string
	MapKey        string
	LocationCount int
	DisplayName   string
}

// GameplaySettings says which map each ruleset plays when a match does not choose one. Its JSON is
// how the setting is stored.
type GameplaySettings struct {
	MovingMapID string `json:"movingMapId"`
	NoMoveMapID string `json:"noMoveMapId"`
	NMPZMapID   string `json:"nmpzMapId"`
}

// OfficialMapImportInput describes an administrator-driven import of an
// official map keyed by its canonical map key.
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
