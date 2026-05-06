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
	UserID            string  `json:"userId"`
	DisplayName       string  `json:"displayName"`
	MMR               int     `json:"mmr"`
	RatingRD          float64 `json:"ratingRd,omitempty"`
	RankedGamesPlayed int     `json:"rankedGamesPlayed,omitempty"`
	AvatarURL         string  `json:"avatarUrl,omitempty"`
	IsGuest           bool    `json:"isGuest,omitempty"`
	IsAdmin           bool    `json:"isAdmin,omitempty"`
}

type MatchMode string

const (
	ModeDuel         MatchMode = "duel"
	ModeSingleplayer MatchMode = "singleplayer"
)

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
	UserID            string  `json:"userId"`
	DisplayName       string  `json:"displayName"`
	MMR               int     `json:"mmr"`
	RatingRD          float64 `json:"ratingRd,omitempty"`
	RankedGamesPlayed int     `json:"rankedGamesPlayed,omitempty"`
	AvatarURL         string  `json:"avatarUrl,omitempty"`
	IsGuest           bool    `json:"isGuest,omitempty"`
	IsAdmin           bool    `json:"isAdmin,omitempty"`
	HP                int     `json:"hp"`
	TotalScore        int     `json:"totalScore,omitempty"`
	Finalized         bool    `json:"finalized"`
	LastGuessLat      float64 `json:"lastGuessLat"`
	LastGuessLng      float64 `json:"lastGuessLng"`
	HasGuess          bool    `json:"-"`
	Disconnected      bool    `json:"disconnected"`
	DisconnectDue     int64   `json:"disconnectDue"`
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

type RoundResult struct {
	RoundID        string                       `json:"roundId"`
	RoundNumber    int                          `json:"roundNumber"`
	ActualLocation LocationPoint                `json:"actualLocation"`
	Players        map[string]RoundPlayerResult `json:"players"`
}

type MatchSnapshot struct {
	MatchID         string                        `json:"matchId"`
	Mode            MatchMode                     `json:"mode"`
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
	UserID            string  `json:"userId"`
	DisplayName       string  `json:"displayName"`
	MMR               int     `json:"mmr"`
	RatingRD          float64 `json:"ratingRd,omitempty"`
	RankedGamesPlayed int     `json:"rankedGamesPlayed,omitempty"`
	AvatarURL         string  `json:"avatarUrl,omitempty"`
	IsGuest           bool    `json:"isGuest,omitempty"`
	IsAdmin           bool    `json:"isAdmin,omitempty"`
	HP                int     `json:"hp"`
	TotalScore        int     `json:"totalScore,omitempty"`
	Finalized         bool    `json:"finalized"`
	Disconnected      bool    `json:"disconnected"`
	DisconnectDue     int64   `json:"disconnectDue"`
}

type ClientSelfState struct {
	UserID       string            `json:"userId"`
	CurrentGuess *ClientGuessPoint `json:"currentGuess,omitempty"`
}

type ClientMatchSnapshot struct {
	MatchID         string                        `json:"matchId"`
	Mode            MatchMode                     `json:"mode"`
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
	Players         map[string]ClientPlayerState  `json:"players"`
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
		Unranked:        snap.Unranked,
		State:           snap.State,
		Phase:           snap.Phase,
		RoundPhase:      snap.RoundPhase,
		PhaseStartedAt:  snap.PhaseStartedAt,
		PhaseEndsAt:     snap.PhaseEndsAt,
		CurrentRound:    clientCurrentRound(snap),
		LastRoundResult: snap.LastRoundResult,
		RoundResults:    snap.RoundResults,
		RoundMSLeft:     snap.RoundMSLeft,
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

func clientCurrentRound(snap *MatchSnapshot) *RoundState {
	if snap == nil || snap.CurrentRound == nil {
		return nil
	}
	round := *snap.CurrentRound
	if shouldRedactLiveLocation(snap) {
		round.Location.Lat = 0
		round.Location.Lng = 0
		round.Location.Country = ""
	}
	return &round
}

func shouldRedactLiveLocation(snap *MatchSnapshot) bool {
	if snap == nil || snap.Phase != PhaseLive {
		return false
	}
	switch snap.RoundPhase {
	case RoundPhaseIntro, RoundPhaseLive, RoundPhaseTransition:
		return true
	default:
		return false
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
	UserID            string  `json:"userId"`
	DisplayName       string  `json:"displayName"`
	AvatarURL         string  `json:"avatarUrl,omitempty"`
	MMR               int     `json:"mmr"`
	RatingRD          float64 `json:"ratingRd,omitempty"`
	RankedGamesPlayed int     `json:"rankedGamesPlayed,omitempty"`
	IsGuest           bool    `json:"isGuest,omitempty"`
	IsAdmin           bool    `json:"isAdmin,omitempty"`
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
	MatchID               string `json:"matchId"`
	Mode                  string `json:"mode,omitempty"`
	Node                  string `json:"node"`
	Ticket                string `json:"ticket"`
	WSPath                string `json:"wsPath"`
	SourceLobbyID         string `json:"sourceLobbyId,omitempty"`
	SourceLobbyInviteCode string `json:"sourceLobbyInviteCode,omitempty"`
}

type SessionStartRequest struct {
	Mode MatchMode `json:"mode"`
}

type MatchSessionResponse struct {
	Status                string                `json:"status"`
	MatchID               string                `json:"matchId"`
	Mode                  string                `json:"mode,omitempty"`
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
	AccessToken        string   `json:"accessToken"`
	OnboardingRequired bool     `json:"onboardingRequired"`
	SuggestedNickname  string   `json:"suggestedNickname,omitempty"`
	User               AuthUser `json:"user"`
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

type LobbyMember struct {
	UserID      string    `json:"userId"`
	DisplayName string    `json:"displayName"`
	AvatarURL   string    `json:"avatarUrl,omitempty"`
	IsGuest     bool      `json:"isGuest,omitempty"`
	IsAdmin     bool      `json:"isAdmin,omitempty"`
	Role        string    `json:"role"`
	Ready       bool      `json:"ready"`
	Connected   bool      `json:"connected,omitempty"`
	JoinedAt    time.Time `json:"joinedAt"`
}

type LobbySnapshot struct {
	ID             string        `json:"id"`
	InviteCode     string        `json:"inviteCode"`
	OwnerUserID    string        `json:"ownerUserId"`
	State          LobbyState    `json:"state"`
	Mode           MatchMode     `json:"mode"`
	MapScope       string        `json:"mapScope"`
	ActiveMatchID  string        `json:"activeMatchId,omitempty"`
	LastMatchID    string        `json:"lastMatchId,omitempty"`
	StartedMatchID string        `json:"startedMatchId,omitempty"`
	CreatedAt      time.Time     `json:"createdAt"`
	ExpiresAt      time.Time     `json:"expiresAt"`
	Members        []LobbyMember `json:"members"`
}

type LobbyCreateRequest struct {
	Mode     MatchMode `json:"mode,omitempty"`
	MapScope string    `json:"mapScope,omitempty"`
}

type LobbyMemberRequest struct {
	UserID string `json:"userId"`
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
	Unranked              bool                     `json:"unranked,omitempty"`
	Players               []string                 `json:"players"`
	Profiles              map[string]PlayerProfile `json:"profiles,omitempty"`
	MapScope              string                   `json:"mapScope"`
	SourceLobbyID         string                   `json:"sourceLobbyId,omitempty"`
	SourceLobbyInviteCode string                   `json:"sourceLobbyInviteCode,omitempty"`
}

type AdminPlayerSummary struct {
	UserID            string    `json:"userId"`
	Email             string    `json:"email,omitempty"`
	DisplayName       string    `json:"displayName"`
	AvatarURL         string    `json:"avatarUrl,omitempty"`
	MMR               int       `json:"mmr"`
	GamesPlayed       int       `json:"gamesPlayed"`
	Wins              int       `json:"wins"`
	RankedGamesPlayed int       `json:"rankedGamesPlayed"`
	IsGuest           bool      `json:"isGuest"`
	IsAdmin           bool      `json:"isAdmin"`
	IsModerator       bool      `json:"isModerator"`
	IsBanned          bool      `json:"isBanned"`
	BanReason         string    `json:"banReason,omitempty"`
	BannedAt          time.Time `json:"bannedAt,omitempty"`
	LastIPAddress     string    `json:"lastIpAddress,omitempty"`
	ReportMutedUntil  time.Time `json:"reportMutedUntil,omitempty"`
}

type ModerationCaseSummary struct {
	ID                  int64          `json:"id"`
	TargetUserID        string         `json:"targetUserId"`
	TargetDisplayName   string         `json:"targetDisplayName"`
	Status              string         `json:"status"`
	Priority            string         `json:"priority"`
	Score               float64        `json:"score"`
	ReportCount         int            `json:"reportCount"`
	UniqueReporterCount int            `json:"uniqueReporterCount"`
	Categories          map[string]int `json:"categories"`
	Summary             string         `json:"summary,omitempty"`
	AssignedTo          string         `json:"assignedTo,omitempty"`
	LatestActivityAt    time.Time      `json:"latestActivityAt"`
	CreatedAt           time.Time      `json:"createdAt"`
	NotificationSentAt  time.Time      `json:"notificationSentAt,omitempty"`
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
	CaseID              int64          `json:"caseId"`
	TargetUserID        string         `json:"targetUserId"`
	TargetDisplayName   string         `json:"targetDisplayName"`
	Priority            string         `json:"priority"`
	Score               float64        `json:"score"`
	ReportCount         int            `json:"reportCount"`
	UniqueReporterCount int            `json:"uniqueReporterCount"`
	Categories          map[string]int `json:"categories"`
	LatestActivityAt    time.Time      `json:"latestActivityAt"`
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

const (
	EventMatchSnapshot  = "match.snapshot"
	EventMatchState     = "match.state"
	EventLegacySnapshot = "match.lifecycle.v2.snapshot"
)

const (
	ErrAuthInvalid      = "ERR_AUTH_INVALID"
	ErrAuthSubMismatch  = "ERR_AUTH_SUB_MISMATCH"
	ErrMatchInProgress  = "ERR_MATCH_IN_PROGRESS"
	ErrMatchNotOwner    = "ERR_MATCH_NOT_OWNER"
	ErrMatchNotFound    = "ERR_MATCH_NOT_FOUND"
	ErrCommandDuplicate = "ERR_COMMAND_DUPLICATE"
	ErrResumeInvalid    = "ERR_RESUME_INVALID"
)
