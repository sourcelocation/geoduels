package persistence

import (
	"encoding/json"
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
	RatingRD          float64                 `json:"ratingRd,omitempty"`
	SeasonID          string                  `json:"seasonId,omitempty"`
	GamesPlayed       int                     `json:"gamesPlayed"`
	Wins              int                     `json:"wins"`
	RankedGamesPlayed int                     `json:"rankedGamesPlayed"`
	RankedWins        int                     `json:"rankedWins"`
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

type ModerationSignalSummary = contracts.ModerationSignalSummary
type ModerationIncidentSummary = contracts.ModerationIncidentSummary
type ModerationReviewTaskSummary = contracts.ModerationReviewTaskSummary
type ModerationVerdictSummary = contracts.ModerationVerdictSummary
type ModerationAuditLogEntry = contracts.ModerationAuditLogEntry
type ModerationReporterState = contracts.ModerationReporterState
type ModerationMatchSummary = contracts.ModerationMatchSummary
type ModerationMatchPlayerSummary = contracts.ModerationMatchPlayerSummary
type ModerationIncidentDetail = contracts.ModerationIncidentDetail
type ModerationSubjectProfile = contracts.ModerationSubjectProfile
type ModerationSignalCreated = contracts.ModerationSignalCreated
type ModerationVerdictInput = contracts.ModerationVerdictInput
type ModerationIncidentNotificationPayload = contracts.ModerationIncidentNotificationPayload
type EnforcementActionSummary = contracts.EnforcementActionSummary
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

type NotificationOutboxItem struct {
	ID          int64
	Type        string
	PayloadJSON []byte
	Attempts    int
}

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
	IncidentIDs    []int64          `json:"incidentIds,omitempty"`
}

type AdminPlayerStats struct {
	TotalMatches     int `json:"totalMatches"`
	RankedMatches    int `json:"rankedMatches"`
	DuelMatches      int `json:"duelMatches"`
	SingleplayerRuns int `json:"singleplayerRuns"`
	Wins             int `json:"wins"`
	Losses           int `json:"losses"`
}

type AdminPlayerEloPoint struct {
	Date   time.Time `json:"date"`
	MMR    int       `json:"mmr"`
	Delta  int       `json:"delta"`
	Played int       `json:"played"`
}

type AdminPlayerDetail struct {
	Player     AdminPlayerSummary    `json:"player"`
	Stats      AdminPlayerStats      `json:"stats"`
	EloHistory []AdminPlayerEloPoint `json:"eloHistory"`
}

type UserNotification struct {
	ID        int64           `json:"id"`
	Type      string          `json:"type"`
	Payload   json.RawMessage `json:"payload"`
	CreatedAt time.Time       `json:"createdAt"`
}

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

type RefreshTokenRecord struct {
	ID               string
	UserID           string
	RefreshTokenHash string
	ExpiresAt        time.Time
	CreatedAt        time.Time
	LastUsedAt       time.Time
	RevokedAt        *time.Time
	UserAgent        string
	IPAddress        string
}
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

type AuthSessionParams struct {
	UserAgent string
	IPAddress string
}

type RuntimeMatch struct {
	MatchID    string
	State      string
	OwnerEpoch int64
	StartedAt  time.Time
	EndedAt    time.Time
}

type MatchSessionUpsert struct {
	Found       contracts.MatchFound
	NodeID      string
	NodeEpoch   int64
	PublicRoute string
}

type ChatMessage = contracts.ChatMessage
