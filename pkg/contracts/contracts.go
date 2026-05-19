package contracts

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type LocationPoint struct {
	Lat     float64  `json:"lat"`
	Lng     float64  `json:"lng"`
	Country string   `json:"country"`
	PanoID  *string  `json:"panoId,omitempty"`
	Heading *float64 `json:"heading,omitempty"`
	Pitch   *float64 `json:"pitch,omitempty"`
}

type GuessPayload struct {
	UserID         string  `json:"userId"`
	MatchID        string  `json:"matchId"`
	RoundID        string  `json:"roundId"`
	Lat            float64 `json:"lat"`
	Lng            float64 `json:"lng"`
	IdempotencyKey string  `json:"idempotencyKey"`
	Finalize       bool    `json:"finalize"`
}

type PlayerProfile struct {
	UserID            string       `json:"userId"`
	DisplayName       string       `json:"displayName"`
	MMR               int          `json:"mmr"`
	RatingRD          float64      `json:"ratingRd,omitempty"`
	RankedGamesPlayed int          `json:"rankedGamesPlayed,omitempty"`
	AvatarURL         string       `json:"avatarUrl,omitempty"`
	IsGuest           bool         `json:"isGuest,omitempty"`
	IsAdmin           bool         `json:"isAdmin,omitempty"`
	SelectedBadge     *PlayerBadge `json:"selectedBadge,omitempty"`
}

type PlayerBadge struct {
	ID          string `json:"id"`
	Kind        string `json:"kind"`
	Label       string `json:"label"`
	Description string `json:"description"`
	ImageURL    string `json:"imageUrl"`
	SeasonID    string `json:"seasonId,omitempty"`
	Rank        int    `json:"rank,omitempty"`
	Owned       bool   `json:"owned"`
}

type MatchMode string

const (
	ModeDuel         MatchMode = "duel"
	ModeSingleplayer MatchMode = "singleplayer"
	ModeTeamDuel     MatchMode = "team_duel"
	ModeFreeForAll   MatchMode = "free_for_all"
)

func IsPrivatePartyMode(mode MatchMode) bool {
	return mode == ModeDuel || mode == ModeTeamDuel || mode == ModeFreeForAll
}

type GameRuleset string

const (
	RulesetMoving GameRuleset = "moving"
	RulesetNoMove GameRuleset = "no_move"
	RulesetNMPZ   GameRuleset = "nmpz"
)

type RoundTimerMode string

const (
	RoundTimerNone     RoundTimerMode = "none"
	RoundTimerPressure RoundTimerMode = "pressure"
	RoundTimerFixed    RoundTimerMode = "fixed"
)

const (
	MapKeyMoving            = "a-source-world"
	MapKeyNMPZ              = "a-location-world"
	DefaultFixedRoundTimeMS = int64(45_000)
	MinimumFixedRoundTimeMS = int64(10_000)
	MaximumFixedRoundTimeMS = int64(120_000)
	DefaultPressureTimeMS   = int64(15_000)
)

type MatchConfig struct {
	Ruleset             GameRuleset    `json:"ruleset,omitempty"`
	MapKey              string         `json:"mapKey,omitempty"`
	RoundTimerMode      RoundTimerMode `json:"roundTimerMode,omitempty"`
	RoundTimeLimitMS    int64          `json:"roundTimeLimitMs,omitempty"`
	PressureTimeLimitMS int64          `json:"pressureTimeLimitMs,omitempty"`
}

func NormalizeRuleset(v GameRuleset) GameRuleset {
	switch v {
	case RulesetNoMove:
		return RulesetNoMove
	case RulesetNMPZ:
		return RulesetNMPZ
	default:
		return RulesetMoving
	}
}

func MapKeyForRuleset(ruleset GameRuleset) string {
	if NormalizeRuleset(ruleset) == RulesetNMPZ {
		return MapKeyNMPZ
	}
	return MapKeyMoving
}

func NormalizeMatchConfig(cfg MatchConfig) MatchConfig {
	cfg.Ruleset = NormalizeRuleset(cfg.Ruleset)
	if cfg.MapKey == "" {
		cfg.MapKey = MapKeyForRuleset(cfg.Ruleset)
	}
	switch cfg.RoundTimerMode {
	case "":
		cfg.RoundTimerMode = RoundTimerNone
		cfg.RoundTimeLimitMS = 0
		if cfg.PressureTimeLimitMS <= 0 {
			cfg.PressureTimeLimitMS = DefaultPressureTimeMS
		}
	case RoundTimerFixed:
		cfg.RoundTimerMode = RoundTimerFixed
		if cfg.RoundTimeLimitMS <= 0 {
			cfg.RoundTimeLimitMS = DefaultFixedRoundTimeMS
		}
		if cfg.RoundTimeLimitMS < MinimumFixedRoundTimeMS {
			cfg.RoundTimeLimitMS = MinimumFixedRoundTimeMS
		}
		if cfg.RoundTimeLimitMS > MaximumFixedRoundTimeMS {
			cfg.RoundTimeLimitMS = MaximumFixedRoundTimeMS
		}
	case RoundTimerPressure:
		cfg.RoundTimerMode = RoundTimerNone
		cfg.RoundTimeLimitMS = 0
		if cfg.PressureTimeLimitMS <= 0 {
			cfg.PressureTimeLimitMS = DefaultPressureTimeMS
		}
	case RoundTimerNone:
		cfg.RoundTimerMode = RoundTimerNone
		cfg.RoundTimeLimitMS = 0
	default:
		cfg.RoundTimerMode = RoundTimerNone
		cfg.RoundTimeLimitMS = 0
	}
	if cfg.PressureTimeLimitMS != DefaultPressureTimeMS {
		cfg.PressureTimeLimitMS = 0
	}
	return cfg
}

type MatchState string

const (
	MatchWaiting MatchState = "waiting"
	MatchLive    MatchState = "live"
	MatchEnded   MatchState = "ended"
)

type MatchPhase string

const (
	PhaseLive        MatchPhase = "live"
	PhaseRoundResult MatchPhase = "round_result"
	PhaseEnded       MatchPhase = "ended"
)

type RoundPhase string

const (
	RoundPhaseIntro      RoundPhase = "round_intro"
	RoundPhaseLive       RoundPhase = "round_live"
	RoundPhaseResult     RoundPhase = "round_result"
	RoundPhaseTransition RoundPhase = "round_transition"
	RoundPhaseEnded      RoundPhase = "ended"
)

type RoundState struct {
	RoundID       string        `json:"roundId"`
	RoundNumber   int           `json:"roundNumber"`
	RoundDeadline time.Time     `json:"roundDeadline"`
	TimerStarted  bool          `json:"timerStarted"`
	Location      LocationPoint `json:"location"`
}

type PlayerState struct {
	UserID            string       `json:"userId"`
	DisplayName       string       `json:"displayName"`
	MMR               int          `json:"mmr"`
	RatingRD          float64      `json:"ratingRd,omitempty"`
	RankedGamesPlayed int          `json:"rankedGamesPlayed,omitempty"`
	AvatarURL         string       `json:"avatarUrl,omitempty"`
	IsGuest           bool         `json:"isGuest,omitempty"`
	IsAdmin           bool         `json:"isAdmin,omitempty"`
	SelectedBadge     *PlayerBadge `json:"selectedBadge,omitempty"`
	TeamID            string       `json:"teamId,omitempty"`
	HP                int          `json:"hp"`
	TotalScore        int          `json:"totalScore,omitempty"`
	Finalized         bool         `json:"finalized"`
	LastGuessLat      float64      `json:"lastGuessLat"`
	LastGuessLng      float64      `json:"lastGuessLng"`
	HasGuess          bool         `json:"-"`
	Disconnected      bool         `json:"disconnected"`
	DisconnectDue     int64        `json:"disconnectDue"`
}

type TeamState struct {
	TeamID  string   `json:"teamId"`
	Name    string   `json:"name,omitempty"`
	HP      int      `json:"hp,omitempty"`
	Players []string `json:"players"`
}

type RatingDeltaPreview struct {
	Win  int `json:"win"`
	Lose int `json:"lose"`
	Draw int `json:"draw"`
}

type RoundPlayerResult struct {
	UserID       string  `json:"userId"`
	Lat          float64 `json:"lat"`
	Lng          float64 `json:"lng"`
	DistanceKm   float64 `json:"distanceKm"`
	Score        int     `json:"score"`
	DamageDealt  int     `json:"damageDealt"`
	DamageTaken  int     `json:"damageTaken"`
	HPAfterRound int     `json:"hpAfterRound"`
	GuessUnixMS  int64   `json:"guessUnixMs,omitempty"`
	GuessMS      int64   `json:"guessMs,omitempty"`
}

type RoundTeamResult struct {
	TeamID               string  `json:"teamId"`
	RepresentativeUserID string  `json:"representativeUserId,omitempty"`
	Lat                  float64 `json:"lat"`
	Lng                  float64 `json:"lng"`
	DistanceKm           float64 `json:"distanceKm"`
	Score                int     `json:"score"`
	DamageDealt          int     `json:"damageDealt"`
	DamageTaken          int     `json:"damageTaken"`
	HPAfterRound         int     `json:"hpAfterRound"`
}

type RoundResult struct {
	RoundID        string                       `json:"roundId"`
	RoundNumber    int                          `json:"roundNumber"`
	ActualLocation LocationPoint                `json:"actualLocation"`
	Players        map[string]RoundPlayerResult `json:"players"`
	Teams          map[string]RoundTeamResult   `json:"teams,omitempty"`
}

type MatchSnapshot struct {
	MatchID         string                        `json:"matchId"`
	Mode            MatchMode                     `json:"mode"`
	SeasonID        string                        `json:"seasonId,omitempty"`
	Config          MatchConfig                   `json:"config,omitempty"`
	Unranked        bool                          `json:"unranked,omitempty"`
	State           MatchState                    `json:"state"`
	Phase           MatchPhase                    `json:"phase"`
	RoundPhase      RoundPhase                    `json:"roundPhase"`
	PhaseStartedAt  int64                         `json:"phaseStartedAt"`
	PhaseEndsAt     int64                         `json:"phaseEndsAt"`
	CurrentRound    *RoundState                   `json:"currentRound,omitempty"`
	LastRoundResult *RoundResult                  `json:"lastRoundResult,omitempty"`
	RoundResults    []*RoundResult                `json:"roundResults,omitempty"`
	RoundMSLeft     int64                         `json:"roundMsLeft"`
	Players         map[string]PlayerState        `json:"players"`
	Teams           map[string]TeamState          `json:"teams,omitempty"`
	RatingPreview   map[string]RatingDeltaPreview `json:"ratingPreview,omitempty"`
	EventSequence   int64                         `json:"eventSequence"`
	ServerUnixMS    int64                         `json:"serverUnixMs"`
	GraceWindowSec  int                           `json:"graceWindowSec"`
}

type ClientGuessPoint struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

type ClientPlayerState struct {
	UserID            string       `json:"userId"`
	DisplayName       string       `json:"displayName"`
	MMR               int          `json:"mmr"`
	RatingRD          float64      `json:"ratingRd,omitempty"`
	RankedGamesPlayed int          `json:"rankedGamesPlayed,omitempty"`
	AvatarURL         string       `json:"avatarUrl,omitempty"`
	IsGuest           bool         `json:"isGuest,omitempty"`
	IsAdmin           bool         `json:"isAdmin,omitempty"`
	SelectedBadge     *PlayerBadge `json:"selectedBadge,omitempty"`
	TeamID            string       `json:"teamId,omitempty"`
	HP                int          `json:"hp"`
	TotalScore        int          `json:"totalScore,omitempty"`
	Finalized         bool         `json:"finalized"`
	Disconnected      bool         `json:"disconnected"`
	DisconnectDue     int64        `json:"disconnectDue"`
}

type ClientSelfState struct {
	UserID       string            `json:"userId"`
	CurrentGuess *ClientGuessPoint `json:"currentGuess,omitempty"`
}

type ClientRoundLocation struct {
	PanoID  *string  `json:"panoId,omitempty"`
	Heading *float64 `json:"heading,omitempty"`
	Pitch   *float64 `json:"pitch,omitempty"`
}

type ClientRoundState struct {
	RoundID       string              `json:"roundId"`
	RoundNumber   int                 `json:"roundNumber"`
	RoundDeadline time.Time           `json:"roundDeadline"`
	TimerStarted  bool                `json:"timerStarted"`
	Location      ClientRoundLocation `json:"location"`
}

type ClientMatchSnapshot struct {
	MatchID         string                        `json:"matchId"`
	Mode            MatchMode                     `json:"mode"`
	Config          MatchConfig                   `json:"config,omitempty"`
	Unranked        bool                          `json:"unranked,omitempty"`
	State           MatchState                    `json:"state"`
	Phase           MatchPhase                    `json:"phase"`
	RoundPhase      RoundPhase                    `json:"roundPhase"`
	PhaseStartedAt  int64                         `json:"phaseStartedAt"`
	PhaseEndsAt     int64                         `json:"phaseEndsAt"`
	CurrentRound    *ClientRoundState             `json:"currentRound,omitempty"`
	LastRoundResult *RoundResult                  `json:"lastRoundResult,omitempty"`
	RoundResults    []*RoundResult                `json:"roundResults,omitempty"`
	RoundMSLeft     int64                         `json:"roundMsLeft"`
	Players         map[string]ClientPlayerState  `json:"players"`
	Teams           map[string]TeamState          `json:"teams,omitempty"`
	Self            *ClientSelfState              `json:"self,omitempty"`
	RatingPreview   map[string]RatingDeltaPreview `json:"ratingPreview,omitempty"`
	EventSequence   int64                         `json:"eventSequence"`
	ServerUnixMS    int64                         `json:"serverUnixMs"`
	GraceWindowSec  int                           `json:"graceWindowSec"`
}

func ClientSnapshotForPlayer(snap *MatchSnapshot, userID string) *ClientMatchSnapshot {
	if snap == nil {
		return nil
	}
	client := &ClientMatchSnapshot{
		MatchID:         snap.MatchID,
		Mode:            snap.Mode,
		Config:          NormalizeMatchConfig(snap.Config),
		Unranked:        snap.Unranked,
		State:           snap.State,
		Phase:           snap.Phase,
		RoundPhase:      snap.RoundPhase,
		PhaseStartedAt:  snap.PhaseStartedAt,
		PhaseEndsAt:     snap.PhaseEndsAt,
		CurrentRound:    clientRoundState(snap.CurrentRound),
		LastRoundResult: snap.LastRoundResult,
		RoundResults:    snap.RoundResults,
		RoundMSLeft:     snap.RoundMSLeft,
		Teams:           snap.Teams,
		RatingPreview:   snap.RatingPreview,
		EventSequence:   snap.EventSequence,
		ServerUnixMS:    snap.ServerUnixMS,
		GraceWindowSec:  snap.GraceWindowSec,
	}
	if snap.Players != nil {
		client.Players = make(map[string]ClientPlayerState, len(snap.Players))
		for id, player := range snap.Players {
			client.Players[id] = ClientPlayerState{
				UserID:            player.UserID,
				DisplayName:       player.DisplayName,
				MMR:               player.MMR,
				RatingRD:          player.RatingRD,
				RankedGamesPlayed: player.RankedGamesPlayed,
				AvatarURL:         player.AvatarURL,
				IsGuest:           player.IsGuest,
				IsAdmin:           player.IsAdmin,
				SelectedBadge:     player.SelectedBadge,
				TeamID:            player.TeamID,
				HP:                player.HP,
				TotalScore:        player.TotalScore,
				Finalized:         player.Finalized,
				Disconnected:      player.Disconnected,
				DisconnectDue:     player.DisconnectDue,
			}
			if id == userID {
				client.Self = clientSelfState(snap, player)
			}
		}
	}
	return client
}

func clientRoundState(round *RoundState) *ClientRoundState {
	if round == nil {
		return nil
	}
	return &ClientRoundState{
		RoundID:       round.RoundID,
		RoundNumber:   round.RoundNumber,
		RoundDeadline: round.RoundDeadline,
		TimerStarted:  round.TimerStarted,
		Location: ClientRoundLocation{
			PanoID:  round.Location.PanoID,
			Heading: round.Location.Heading,
			Pitch:   round.Location.Pitch,
		},
	}
}

func clientSelfState(snap *MatchSnapshot, player PlayerState) *ClientSelfState {
	self := &ClientSelfState{UserID: player.UserID}
	if snap.Phase == PhaseLive && snap.RoundPhase == RoundPhaseLive && player.HasGuess {
		self.CurrentGuess = &ClientGuessPoint{Lat: player.LastGuessLat, Lng: player.LastGuessLng}
	}
	return self
}

type QueueJoinRequest struct {
	UserID            string       `json:"userId"`
	DisplayName       string       `json:"displayName"`
	AvatarURL         string       `json:"avatarUrl,omitempty"`
	MMR               int          `json:"mmr"`
	RatingRD          float64      `json:"ratingRd,omitempty"`
	SeasonID          string       `json:"seasonId,omitempty"`
	RankedGamesPlayed int          `json:"rankedGamesPlayed,omitempty"`
	IsGuest           bool         `json:"isGuest,omitempty"`
	IsAdmin           bool         `json:"isAdmin,omitempty"`
	SelectedBadge     *PlayerBadge `json:"selectedBadge,omitempty"`
}

type QueueJoinResponse struct {
	TicketID string `json:"ticketId"`
	Status   string `json:"status"`
}

type QueueStatusEvent struct {
	Status   string `json:"status"`
	QueuedAt int64  `json:"queuedAt"`
}

type MatchAssignedPayload struct {
	MatchID               string      `json:"matchId"`
	Mode                  string      `json:"mode,omitempty"`
	Config                MatchConfig `json:"config,omitempty"`
	Node                  string      `json:"node"`
	Ticket                string      `json:"ticket"`
	WSPath                string      `json:"wsPath"`
	SourceLobbyID         string      `json:"sourceLobbyId,omitempty"`
	SourceLobbyInviteCode string      `json:"sourceLobbyInviteCode,omitempty"`
}

type SessionStartRequest struct {
	Mode    MatchMode   `json:"mode"`
	Ruleset GameRuleset `json:"ruleset,omitempty"`
	Config  MatchConfig `json:"config,omitempty"`
}

type MatchSessionResponse struct {
	Status                string                `json:"status"`
	MatchID               string                `json:"matchId"`
	Mode                  string                `json:"mode,omitempty"`
	Config                MatchConfig           `json:"config,omitempty"`
	Node                  string                `json:"node,omitempty"`
	Ticket                string                `json:"ticket,omitempty"`
	WSPath                string                `json:"wsPath,omitempty"`
	Reason                string                `json:"reason,omitempty"`
	Snapshot              *MatchSnapshot        `json:"snapshot,omitempty"`
	ReplacementMatchID    string                `json:"replacementMatchId,omitempty"`
	Replacement           *MatchAssignedPayload `json:"replacement,omitempty"`
	SourceLobbyID         string                `json:"sourceLobbyId,omitempty"`
	SourceLobbyInviteCode string                `json:"sourceLobbyInviteCode,omitempty"`
}

type AuthUser struct {
	ID          string `json:"id"`
	Email       string `json:"email,omitempty"`
	DisplayName string `json:"display_name,omitempty"`
	AvatarURL   string `json:"avatar_url,omitempty"`
	IsGuest     bool   `json:"isGuest"`
	IsAdmin     bool   `json:"isAdmin,omitempty"`
	IsModerator bool   `json:"isModerator,omitempty"`
}

type LeaderboardEntrySummary struct {
	Rank        int    `json:"rank"`
	UserID      string `json:"userId"`
	DisplayName string `json:"displayName"`
	AvatarURL   string `json:"avatarUrl,omitempty"`
	MMR         int    `json:"mmr"`
	GamesPlayed int    `json:"gamesPlayed"`
	Wins        int    `json:"wins"`
}

type LeaderboardSummary struct {
	Mode         string                    `json:"mode"`
	Season       string                    `json:"season"`
	SelfRank     int                       `json:"selfRank"`
	TotalPlayers int                       `json:"totalPlayers"`
	Entries      []LeaderboardEntrySummary `json:"entries"`
}

type AuthSessionPayload struct {
	AccessToken           string   `json:"accessToken"`
	OnboardingRequired    bool     `json:"onboardingRequired"`
	SuggestedNickname     string   `json:"suggestedNickname,omitempty"`
	LinkedProviders       []string `json:"linkedProviders,omitempty"`
	AuthMigrationRequired bool     `json:"authMigrationRequired,omitempty"`
	RecoveryAvailable     bool     `json:"recoveryAvailable,omitempty"`
	CanPlay               bool     `json:"canPlay"`
	User                  AuthUser `json:"user"`
}

type MatchBootstrapResponse struct {
	Auth  AuthSessionPayload   `json:"auth"`
	Match MatchSessionResponse `json:"match"`
}

type ResumableSessionResponse struct {
	Status  string `json:"status"`
	MatchID string `json:"matchId,omitempty"`
	Mode    string `json:"mode,omitempty"`
}

type LobbyState string

const (
	LobbyOpen    LobbyState = "open"
	LobbyInMatch LobbyState = "in_match"
	LobbyStarted LobbyState = "started"
	LobbyClosed  LobbyState = "closed"
	LobbyExpired LobbyState = "expired"
)

type LobbyPresenceStatus string

const (
	LobbyPresenceOnline  LobbyPresenceStatus = "online"
	LobbyPresenceAway    LobbyPresenceStatus = "away"
	LobbyPresenceOffline LobbyPresenceStatus = "offline"
)

type LobbyMember struct {
	UserID         string              `json:"userId"`
	DisplayName    string              `json:"displayName"`
	AvatarURL      string              `json:"avatarUrl,omitempty"`
	IsGuest        bool                `json:"isGuest,omitempty"`
	IsAdmin        bool                `json:"isAdmin,omitempty"`
	SelectedBadge  *PlayerBadge        `json:"selectedBadge,omitempty"`
	TeamID         string              `json:"teamId,omitempty"`
	Role           string              `json:"role"`
	Ready          bool                `json:"ready"`
	Connected      bool                `json:"connected,omitempty"`
	PresenceStatus LobbyPresenceStatus `json:"presenceStatus,omitempty"`
	JoinedAt       time.Time           `json:"joinedAt"`
}

type LobbySnapshot struct {
	ID             string        `json:"id"`
	InviteCode     string        `json:"inviteCode"`
	OwnerUserID    string        `json:"ownerUserId"`
	State          LobbyState    `json:"state"`
	Mode           MatchMode     `json:"mode"`
	MapScope       string        `json:"mapScope"`
	Config         MatchConfig   `json:"config,omitempty"`
	ActiveMatchID  string        `json:"activeMatchId,omitempty"`
	LastMatchID    string        `json:"lastMatchId,omitempty"`
	StartedMatchID string        `json:"startedMatchId,omitempty"`
	CreatedAt      time.Time     `json:"createdAt"`
	ExpiresAt      time.Time     `json:"expiresAt"`
	Members        []LobbyMember `json:"members"`
}

type LobbyPatch struct {
	Revision        int64         `json:"revision"`
	State           *LobbyState   `json:"state,omitempty"`
	OwnerUserID     *string       `json:"ownerUserId,omitempty"`
	Mode            *MatchMode    `json:"mode,omitempty"`
	Config          *MatchConfig  `json:"config,omitempty"`
	ActiveMatchID   *string       `json:"activeMatchId,omitempty"`
	LastMatchID     *string       `json:"lastMatchId,omitempty"`
	StartedMatchID  *string       `json:"startedMatchId,omitempty"`
	UpsertMembers   []LobbyMember `json:"upsertMembers,omitempty"`
	RemoveMemberIDs []string      `json:"removeMemberIds,omitempty"`
}

type LobbyCreateRequest struct {
	Mode     MatchMode   `json:"mode,omitempty"`
	MapScope string      `json:"mapScope,omitempty"`
	Config   MatchConfig `json:"config,omitempty"`
}

type LobbyMemberRequest struct {
	UserID string `json:"userId"`
}

type LobbyTeamRequest struct {
	TeamID string `json:"teamId"`
}

type LobbyStartResponse struct {
	Assignment MatchAssignedPayload `json:"assignment"`
}

type GameplayTicketClaims struct {
	Node    string `json:"node"`
	MatchID string `json:"matchId"`
	jwt.RegisteredClaims
}

type MatchFound struct {
	MatchID               string                   `json:"matchId"`
	Mode                  MatchMode                `json:"mode,omitempty"`
	SeasonID              string                   `json:"seasonId,omitempty"`
	Config                MatchConfig              `json:"config,omitempty"`
	Unranked              bool                     `json:"unranked,omitempty"`
	Players               []string                 `json:"players"`
	Profiles              map[string]PlayerProfile `json:"profiles,omitempty"`
	Teams                 map[string]string        `json:"teams,omitempty"`
	MapScope              string                   `json:"mapScope"`
	SourceLobbyID         string                   `json:"sourceLobbyId,omitempty"`
	SourceLobbyInviteCode string                   `json:"sourceLobbyInviteCode,omitempty"`
}

type AdminPlayerSummary struct {
	UserID            string              `json:"userId"`
	Email             string              `json:"email,omitempty"`
	DisplayName       string              `json:"displayName"`
	AvatarURL         string              `json:"avatarUrl,omitempty"`
	MMR               int                 `json:"mmr"`
	GamesPlayed       int                 `json:"gamesPlayed"`
	Wins              int                 `json:"wins"`
	RankedGamesPlayed int                 `json:"rankedGamesPlayed"`
	IsGuest           bool                `json:"isGuest"`
	IsAdmin           bool                `json:"isAdmin"`
	IsModerator       bool                `json:"isModerator"`
	IsBanned          bool                `json:"isBanned"`
	BanReason         string              `json:"banReason,omitempty"`
	BannedAt          time.Time           `json:"bannedAt,omitempty"`
	LastIPAddress     string              `json:"lastIpAddress,omitempty"`
	ReportMutedUntil  time.Time           `json:"reportMutedUntil,omitempty"`
	Identities        []AdminUserIdentity `json:"identities,omitempty"`
}

type AdminUserIdentity struct {
	Provider       string    `json:"provider"`
	ProviderUserID string    `json:"providerUserId"`
	Email          string    `json:"email,omitempty"`
	ProviderName   string    `json:"providerName,omitempty"`
	LastSeenAt     time.Time `json:"lastSeenAt,omitempty"`
	DeletedAt      time.Time `json:"deletedAt,omitempty"`
}

type ModerationCaseSummary struct {
	ID                   int64          `json:"id"`
	TargetUserID         string         `json:"targetUserId"`
	TargetDisplayName    string         `json:"targetDisplayName"`
	Status               string         `json:"status"`
	Priority             string         `json:"priority"`
	Score                float64        `json:"score"`
	ReporterScore        float64        `json:"reporterScore,omitempty"`
	RecentReportPressure float64        `json:"recentReportPressure,omitempty"`
	GameplayEvidence     float64        `json:"gameplayEvidence,omitempty"`
	ReportCount          int            `json:"reportCount"`
	UniqueReporterCount  int            `json:"uniqueReporterCount"`
	Categories           map[string]int `json:"categories"`
	Summary              string         `json:"summary,omitempty"`
	AssignedTo           string         `json:"assignedTo,omitempty"`
	LatestActivityAt     time.Time      `json:"latestActivityAt"`
	CreatedAt            time.Time      `json:"createdAt"`
	NotificationSentAt   time.Time      `json:"notificationSentAt,omitempty"`
}

type ModerationReportSummary struct {
	ID             int64     `json:"id"`
	CaseID         int64     `json:"caseId"`
	MatchID        string    `json:"matchId"`
	ReporterUserID string    `json:"reporterUserId"`
	ReporterName   string    `json:"reporterName"`
	ReportedUserID string    `json:"reportedUserId"`
	ReportedName   string    `json:"reportedName"`
	Category       string    `json:"category"`
	Reason         string    `json:"reason,omitempty"`
	ReporterWeight float64   `json:"reporterWeight"`
	CreatedAt      time.Time `json:"createdAt"`
}

type ModerationCaseEvent struct {
	ID          int64     `json:"id"`
	CaseID      int64     `json:"caseId"`
	ActorUserID string    `json:"actorUserId,omitempty"`
	EventType   string    `json:"eventType"`
	Body        string    `json:"body,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
}

type ModerationActionSummary struct {
	ID           int64     `json:"id"`
	CaseID       int64     `json:"caseId"`
	ActorUserID  string    `json:"actorUserId,omitempty"`
	TargetUserID string    `json:"targetUserId"`
	ActionType   string    `json:"actionType"`
	Reason       string    `json:"reason,omitempty"`
	CreatedAt    time.Time `json:"createdAt"`
}

type ModerationCaseDetail struct {
	Case         ModerationCaseSummary     `json:"case"`
	TargetPlayer *AdminPlayerSummary       `json:"targetPlayer,omitempty"`
	Reports      []ModerationReportSummary `json:"reports"`
	Events       []ModerationCaseEvent     `json:"events"`
	Actions      []ModerationActionSummary `json:"actions"`
}

type ModerationReportCreated struct {
	CaseID int64  `json:"caseId"`
	Status string `json:"status"`
}

type ModerationCaseNotificationPayload struct {
	CaseID               int64          `json:"caseId"`
	TargetUserID         string         `json:"targetUserId"`
	TargetDisplayName    string         `json:"targetDisplayName"`
	Priority             string         `json:"priority"`
	Score                float64        `json:"score"`
	ReporterScore        float64        `json:"reporterScore,omitempty"`
	RecentReportPressure float64        `json:"recentReportPressure,omitempty"`
	GameplayEvidence     float64        `json:"gameplayEvidence,omitempty"`
	ReportCount          int            `json:"reportCount"`
	UniqueReporterCount  int            `json:"uniqueReporterCount"`
	Categories           map[string]int `json:"categories"`
	LatestActivityAt     time.Time      `json:"latestActivityAt"`
}

type MapRevisionSummary struct {
	MapKey      string `json:"mapKey"`
	RevisionID  string `json:"revisionId"`
	RowCount    int    `json:"rowCount"`
	Inserted    bool   `json:"inserted"`
	DisplayName string `json:"displayName"`
}

type CommandEnvelope struct {
	CommandID string         `json:"commandId"`
	Type      string         `json:"type"`
	Payload   map[string]any `json:"payload"`
	SentAt    int64          `json:"sentAt"`
}

type CommandAck struct {
	Kind      string `json:"kind"`
	CommandID string `json:"commandId"`
	Status    string `json:"status"`
	ErrorCode string `json:"errorCode,omitempty"`
	Message   string `json:"message,omitempty"`
	ServerTS  int64  `json:"serverTs"`
}

type EventEnvelope struct {
	Kind     string `json:"kind"`
	EventID  string `json:"eventId"`
	Type     string `json:"type"`
	MatchID  string `json:"matchId,omitempty"`
	Seq      int64  `json:"seq,omitempty"`
	ServerTS int64  `json:"serverTs"`
	Payload  any    `json:"payload,omitempty"`
}

type ChatMessageKind string

const (
	ChatMessageText  ChatMessageKind = "text"
	ChatMessageEmote ChatMessageKind = "emote"
)

type ChatEmote string

const (
	ChatEmoteSkull      ChatEmote = "skull"
	ChatEmoteSob        ChatEmote = "sob"
	ChatEmoteThinking   ChatEmote = "thinking"
	ChatEmoteSunglasses ChatEmote = "sunglasses"
)

type ChatMessage struct {
	ID                string          `json:"id"`
	ConversationID    string          `json:"conversationId,omitempty"`
	MatchID           string          `json:"matchId"`
	SenderUserID      string          `json:"senderUserId"`
	SenderDisplayName string          `json:"senderDisplayName"`
	Kind              ChatMessageKind `json:"kind"`
	Body              string          `json:"body,omitempty"`
	Emote             ChatEmote       `json:"emote,omitempty"`
	CreatedAt         time.Time       `json:"createdAt"`
}

const (
	EventMatchSnapshot  = "match.snapshot"
	EventMatchState     = "match.state"
	EventLegacySnapshot = "match.lifecycle.v2.snapshot"
	EventChatMessage    = "chat.message"
)

const (
	ErrAuthInvalid      = "ERR_AUTH_INVALID"
	ErrAuthSubMismatch  = "ERR_AUTH_SUB_MISMATCH"
	ErrMatchInProgress  = "ERR_MATCH_IN_PROGRESS"
	ErrMatchNotOwner    = "ERR_MATCH_NOT_OWNER"
	ErrMatchNotFound    = "ERR_MATCH_NOT_FOUND"
	ErrCommandDuplicate = "ERR_COMMAND_DUPLICATE"
	ErrResumeInvalid    = "ERR_RESUME_INVALID"
	ErrRateLimited      = "ERR_RATE_LIMITED"
)
