package badges

const (
	DiscordSyncActionSync         = "sync"
	DiscordSyncActionCleanupRoles = "cleanup_roles"
)

const (
	badgeCodeDiscordMember       = int16(1)
	badgeCodeGeoDuelsTeam        = int16(2)
	badgeCodeDiscordServerMember = int16(3)
	badgeCodeSupporter           = int16(4)
	badgeCodeSpeedrunner         = int16(5)
	badgeCodeElo1000             = int16(6)
	badgeCodeElo1500             = int16(7)
	badgeCodeElo2000             = int16(8)
	badgeCodeLegacyTopFinish     = int16(10)
	badgeCodeTopFinish           = int16(11)
	badgeCodeEventWinner2026     = int16(12)
)

// CatalogEntry is a badge staff may grant by hand.
type CatalogEntry struct {
	ID          string
	Kind        string
	Label       string
	Description string
	ImageURL    string
	Rarity      string
	MaxLevel    int
}

type DiscordLinkedUser struct {
	UserID             string
	DiscordUserID      string
	HighestEloBadgeMMR int
}

// Badge is a badge as a player has it, or could. Its JSON is how a badge_unlocked notification
// stores the badge it announces.
type Badge struct {
	ID           string `json:"id"`
	Kind         string `json:"kind"`
	Label        string `json:"label"`
	Description  string `json:"description"`
	ImageURL     string `json:"imageUrl"`
	Rarity       string `json:"rarity,omitempty"`
	Level        int    `json:"level,omitempty"`
	MaxLevel     int    `json:"maxLevel,omitempty"`
	Extra        int    `json:"extra,omitempty"`
	Owned        bool   `json:"owned"`
	Unobtainable bool   `json:"unobtainable,omitempty"`
}
