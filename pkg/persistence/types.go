package persistence

import (
	"time"

	"geoduels/pkg/contracts"
)

type Profile struct {
	UserID            string                  `json:"userId"`
	DisplayName       string                  `json:"displayName"`
	AvatarURL         string                  `json:"avatarUrl,omitempty"`
	MMR               int                     `json:"mmr"`
	RatingRD          float64                 `json:"ratingRd,omitempty"`
	SeasonID          string                  `json:"seasonId,omitempty"`
	GamesPlayed       int                     `json:"gamesPlayed"`
	Wins              int                     `json:"wins"`
	RankedGamesPlayed int                     `json:"rankedGamesPlayed"`
	RankedWins        int                     `json:"rankedWins"`
	IsGuest           bool                    `json:"isGuest"`
	IsAdmin           bool                    `json:"isAdmin"`
	IsModerator       bool                    `json:"isModerator"`
	IsBanned          bool                    `json:"isBanned"`
	BanReason         string                  `json:"banReason,omitempty"`
	Badges            []contracts.PlayerBadge `json:"badges,omitempty"`
	SelectedBadge     *contracts.PlayerBadge  `json:"selectedBadge,omitempty"`
}

type PublicPlayerProfile struct {
	UserID            string                  `json:"userId"`
	DisplayName       string                  `json:"displayName"`
	AvatarURL         string                  `json:"avatarUrl,omitempty"`
	MMR               int                     `json:"mmr"`
	LeaderboardRank   int                     `json:"leaderboardRank"`
	LeaderboardTotal  int                     `json:"leaderboardTotal"`
	RatingRD          float64                 `json:"ratingRd,omitempty"`
	SeasonID          string                  `json:"seasonId,omitempty"`
	GamesPlayed       int                     `json:"gamesPlayed"`
	Wins              int                     `json:"wins"`
	RankedGamesPlayed int                     `json:"rankedGamesPlayed"`
	RankedWins        int                     `json:"rankedWins"`
	BestWinStreak     int                     `json:"bestWinStreak"`
	PerfectGuesses    int                     `json:"perfectGuesses"`
	FlawlessWins      int                     `json:"flawlessWins"`
	Badges            []contracts.PlayerBadge `json:"badges,omitempty"`
	SelectedBadge     *contracts.PlayerBadge  `json:"selectedBadge,omitempty"`
}

type LeaderboardEntry struct {
	Rank        int    `json:"rank"`
	UserID      string `json:"userId"`
	DisplayName string `json:"displayName"`
	AvatarURL   string `json:"avatarUrl,omitempty"`
	MMR         int    `json:"mmr"`
	GamesPlayed int    `json:"gamesPlayed"`
	Wins        int    `json:"wins"`
}

type LeaderboardOverview struct {
	Mode         string             `json:"mode"`
	SeasonID     string             `json:"season"`
	SelfRank     int                `json:"selfRank"`
	TotalPlayers int                `json:"totalPlayers"`
	Entries      []LeaderboardEntry `json:"entries"`
}

type Identity struct {
	Sub                   string
	Email                 string
	GoogleName            string
	ProviderName          string
	AvatarURL             string
	NicknameRequired      bool
	DisplayName           string
	AccountType           string
	LinkedProviders       []string
	AuthMigrationRequired bool
	RecoveryAvailable     bool
	IsAdmin               bool
	IsModerator           bool
	IsBanned              bool
	BanReason             string
}

type AdminPlayerSummary = contracts.AdminPlayerSummary

// AdminBadgeDefinition is the server-authoritative catalog entry for badges
// that an administrator may grant manually.
type AdminBadgeDefinition struct {
	ID          string `json:"id"`
	Kind        string `json:"kind"`
	Label       string `json:"label"`
	Description string `json:"description"`
	ImageURL    string `json:"imageUrl"`
	Rarity      string `json:"rarity,omitempty"`
	MaxLevel    int    `json:"maxLevel"`
}

type ModerationSignalSummary = contracts.ModerationSignalSummary
type ModerationAuditLogEntry = contracts.ModerationAuditLogEntry
type ModerationSubjectProfile = contracts.ModerationSubjectProfile
type ModerationSignalCreated = contracts.ModerationSignalCreated
type ModerationSignalNotificationPayload = contracts.ModerationSignalNotificationPayload
type UserRoleGrant = contracts.UserRoleGrant

type MapImportSummary = contracts.MapImportSummary

type MatchHistorySummary struct {
	MatchID             string    `json:"matchId"`
	Mode                string    `json:"mode"`
	StartedAt           time.Time `json:"startedAt"`
	EndedAt             time.Time `json:"endedAt"`
	WinnerUserID        string    `json:"winnerUserId,omitempty"`
	Outcome             string    `json:"outcome"`
	Ranked              bool      `json:"ranked"`
	RatingDelta         int       `json:"ratingDelta,omitempty"`
	TotalScore          int       `json:"totalScore,omitempty"`
	OpponentUserID      string    `json:"opponentUserId,omitempty"`
	OpponentDisplayName string    `json:"opponentDisplayName,omitempty"`
}

type MatchHistoryPage struct {
	Matches     []MatchHistorySummary
	HasMore     bool
	NextEndedAt time.Time
	NextMatchID string
}

type CreatePlayerReportSignalParams struct {
	MatchID        string
	ReporterUserID string
	ReportedUserID string
	Category       string
	Reason         string
}

type NotificationOutboxItem = contracts.NotificationOutboxItem

type ChatRestriction struct {
	ActionType string
	ReasonCode string
	ReasonNote string
	EndsAt     time.Time
}

type EloRefundSummary struct {
	RefundsIssued int `json:"refundsIssued"`
	TotalRefunded int `json:"totalRefunded"`
}

type CheatingBanSummary struct {
	UserID         string           `json:"userId"`
	Reason         string           `json:"reason,omitempty"`
	Refunds        EloRefundSummary `json:"refunds"`
	IPSignupBanned bool             `json:"ipSignupBanned"`
}

type CommunityPardonSummary struct {
	Eligible int       `json:"eligible"`
	Pardoned int       `json:"pardoned"`
	Cutoff   time.Time `json:"cutoff"`
}

type AdminPlayerStats struct {
	TotalMatches     int `json:"totalMatches"`
	RankedMatches    int `json:"rankedMatches"`
	DuelMatches      int `json:"duelMatches"`
	SingleplayerRuns int `json:"singleplayerRuns"`
	Wins             int `json:"wins"`
	Losses           int `json:"losses"`
}

type AdminPlayerDetail struct {
	Player AdminPlayerSummary `json:"player"`
}

type UserNotification = contracts.UserNotification

type SignupIPBan struct {
	ID        int64     `json:"id"`
	IPAddress string    `json:"ipAddress"`
	Reason    string    `json:"reason,omitempty"`
	CreatedBy string    `json:"createdBy,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}

type LobbyChangelogContent struct {
	Eyebrow   string    `json:"eyebrow"`
	Title     string    `json:"title"`
	Markdown  string    `json:"markdown"`
	Slug      string    `json:"slug,omitempty"`
	UpdatedAt time.Time `json:"updatedAt,omitempty"`
}

type ChangelogPost struct {
	ID        int64     `json:"id"`
	Slug      string    `json:"slug"`
	Title     string    `json:"title"`
	Markdown  string    `json:"markdown"`
	Published bool      `json:"published"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type ChangelogPostInput struct {
	Slug      string `json:"slug"`
	Title     string `json:"title"`
	Markdown  string `json:"markdown"`
	Published bool   `json:"published"`
}

type ModerationSettings struct {
	DiscordWebhookURL string `json:"discordWebhookUrl"`
}

type DiscordIntegrationSettings struct {
	GuildID                  string   `json:"guildId"`
	JoinsChannelID           string   `json:"joinsChannelId"`
	Elo1000RoleID            string   `json:"elo1000RoleId"`
	Elo1500RoleID            string   `json:"elo1500RoleId"`
	Elo2000RoleID            string   `json:"elo2000RoleId"`
	ManagedRoleIDs           []string `json:"managedRoleIds,omitempty"`
	ReconcileIntervalMinutes int      `json:"reconcileIntervalMinutes"`
}

type RankedSeasonSettings struct {
	ActiveSeasonID  string     `json:"activeSeasonId"`
	MonthlyResetDay int        `json:"monthlyResetDay"`
	NextResetAt     *time.Time `json:"nextResetAt,omitempty"`
	LastResetAt     *time.Time `json:"lastResetAt,omitempty"`
}

type RankedSeasonResetResult struct {
	PreviousSeasonID string `json:"previousSeasonId"`
	ActiveSeasonID   string `json:"activeSeasonId"`
	PlayersSeeded    int    `json:"playersSeeded"`
	ResetAt          string `json:"resetAt"`
}

type RefreshTokenRecord = contracts.RefreshTokenRecord
type DiscordSyncOutboxItem struct {
	ID            int64
	Action        string
	DiscordUserID string
	Attempts      int
}

type DiscordLinkedUser struct {
	UserID             string
	DiscordUserID      string
	HighestEloBadgeMMR int
}

type AuthSessionParams = contracts.AuthSessionParams

type RuntimeMatch = contracts.RuntimeMatch

type MatchSessionUpsert = contracts.MatchSessionUpsert

type ChatMessage = contracts.ChatMessage
