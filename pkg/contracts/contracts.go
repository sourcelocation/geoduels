package contracts

import (
	"encoding/json"
	"strings"
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

type MatchMode string

const (
	ModeDuel         MatchMode = "duel"
	ModeSingleplayer MatchMode = "singleplayer"
	ModeTeamDuel     MatchMode = "team_duel"
	ModeFreeForAll   MatchMode = "free_for_all"
	MinPartyMembers            = 2
	MaxPartyMembers            = 64
)

func IsPrivatePartyMode(mode MatchMode) bool {
	return mode == ModeDuel || mode == ModeTeamDuel || mode == ModeFreeForAll
}

type GameRuleset = string

const (
	RulesetMoving GameRuleset = "moving"
	RulesetNoMove GameRuleset = "no_move"
	RulesetNMPZ   GameRuleset = "nmpz"
)

type StreetNamesVisibility = string

const (
	StreetNamesShown  StreetNamesVisibility = "shown"
	StreetNamesHidden StreetNamesVisibility = "hidden"
)

type RoundTimerMode string

const (
	RoundTimerNone     RoundTimerMode = "none"
	RoundTimerPressure RoundTimerMode = "pressure"
	RoundTimerFixed    RoundTimerMode = "fixed"
)

type MultiplierMode string

const (
	MultiplierShared     MultiplierMode = "shared"
	MultiplierIndividual MultiplierMode = "individual"
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
	Ruleset             GameRuleset           `json:"ruleset,omitempty"`
	StreetNames         StreetNamesVisibility `json:"streetNames,omitempty"`
	MapID               string                `json:"mapId,omitempty"`
	MapName             string                `json:"mapName,omitempty"`
	MapKey              string                `json:"mapKey,omitempty"` // Legacy read compatibility.
	RoundTimerMode      RoundTimerMode        `json:"roundTimerMode,omitempty"`
	RoundTimeLimitMS    int64                 `json:"roundTimeLimitMs,omitempty"`
	PressureTimeLimitMS int64                 `json:"pressureTimeLimitMs,omitempty"`
	MultiplierMode      MultiplierMode        `json:"multiplierMode,omitempty"`
}

// MatchReturnTarget is the server-owned navigation intent for a match. It is
// deliberately separate from MatchConfig: the map that was played does not
// determine where the player should return after leaving the match.
type MatchReturnTargetKind string

const (
	MatchReturnHome  MatchReturnTargetKind = "home"
	MatchReturnMap   MatchReturnTargetKind = "map"
	MatchReturnParty MatchReturnTargetKind = "party"
)

type MatchReturnTarget struct {
	Kind            MatchReturnTargetKind `json:"kind"`
	MapID           string                `json:"mapId,omitempty"`
	PartyID         string                `json:"partyId,omitempty"`
	PartyInviteCode string                `json:"partyInviteCode,omitempty"`
}

func NormalizeMatchReturnTarget(target *MatchReturnTarget) *MatchReturnTarget {
	if target == nil {
		return &MatchReturnTarget{Kind: MatchReturnHome}
	}
	normalized := *target
	switch normalized.Kind {
	case MatchReturnMap:
		normalized.MapID = strings.TrimSpace(normalized.MapID)
		if normalized.MapID == "" {
			return &MatchReturnTarget{Kind: MatchReturnHome}
		}
		normalized.PartyID = ""
		normalized.PartyInviteCode = ""
	case MatchReturnParty:
		normalized.PartyID = strings.TrimSpace(normalized.PartyID)
		if normalized.PartyID == "" {
			return &MatchReturnTarget{Kind: MatchReturnHome}
		}
		normalized.MapID = ""
	case MatchReturnHome:
		normalized = MatchReturnTarget{Kind: MatchReturnHome}
	default:
		return &MatchReturnTarget{Kind: MatchReturnHome}
	}
	return &normalized
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

func NormalizeStreetNames(v StreetNamesVisibility) StreetNamesVisibility {
	if v == StreetNamesHidden {
		return StreetNamesHidden
	}
	return StreetNamesShown
}

func NormalizeMatchConfig(cfg MatchConfig) MatchConfig {
	cfg.Ruleset = NormalizeRuleset(cfg.Ruleset)
	cfg.StreetNames = NormalizeStreetNames(cfg.StreetNames)
	if cfg.MultiplierMode != MultiplierIndividual {
		cfg.MultiplierMode = MultiplierShared
	}
	if cfg.MapID == "" {
		cfg.MapID = cfg.MapKey
	}
	cfg.MapKey = ""
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
	switch cfg.PressureTimeLimitMS {
	case 0, DefaultPressureTimeMS, 30_000, 60_000, 90_000:
	default:
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
	DamageMultiplier  float64      `json:"damageMultiplier"`
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
	RoundID          string                       `json:"roundId"`
	RoundNumber      int                          `json:"roundNumber"`
	ActualLocation   LocationPoint                `json:"actualLocation"`
	Players          map[string]RoundPlayerResult `json:"players"`
	Teams            map[string]RoundTeamResult   `json:"teams,omitempty"`
	DamageMultiplier float64                      `json:"damageMultiplier,omitempty"`
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
	DamageMultiplier  float64      `json:"damageMultiplier"`
}

type ClientSelfState struct {
	UserID       string            `json:"userId"`
	CurrentGuess *ClientGuessPoint `json:"currentGuess,omitempty"`
}

type ClientTeamState struct {
	Guesses map[string]ClientGuessPoint `json:"guesses,omitempty"`
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
	Team            *ClientTeamState              `json:"team,omitempty"`
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
				DamageMultiplier:  player.DamageMultiplier,
			}
			if id == userID {
				client.Self = clientSelfState(snap, player)
			}
		}
	}
	if snap.Mode == ModeTeamDuel && snap.Phase == PhaseLive && snap.RoundPhase == RoundPhaseLive {
		if self, ok := snap.Players[userID]; ok && self.TeamID != "" {
			guesses := map[string]ClientGuessPoint{}
			for id, player := range snap.Players {
				if id != userID && player.TeamID == self.TeamID && player.HasGuess {
					guesses[id] = ClientGuessPoint{Lat: player.LastGuessLat, Lng: player.LastGuessLng}
				}
			}
			if len(guesses) > 0 {
				client.Team = &ClientTeamState{Guesses: guesses}
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
	MatchID               string             `json:"matchId"`
	Mode                  string             `json:"mode,omitempty"`
	Config                MatchConfig        `json:"config,omitempty"`
	Node                  string             `json:"node"`
	Ticket                string             `json:"ticket"`
	WSPath                string             `json:"wsPath"`
	SourcePartyID         string             `json:"sourcePartyId,omitempty"`
	SourcePartyInviteCode string             `json:"sourcePartyInviteCode,omitempty"`
	ReturnTarget          *MatchReturnTarget `json:"returnTarget,omitempty"`
}

type SessionStartRequest struct {
	Mode MatchMode `json:"mode"`
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
	SourcePartyID         string                `json:"sourcePartyId,omitempty"`
	SourcePartyInviteCode string                `json:"sourcePartyInviteCode,omitempty"`
	ReturnTarget          *MatchReturnTarget    `json:"returnTarget,omitempty"`
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
	NicknameRequired      bool     `json:"nicknameRequired"`
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

type PartyState string

const (
	PartyOpen    PartyState = "open"
	PartyInMatch PartyState = "in_match"
	PartyStarted PartyState = "started"
	PartyClosed  PartyState = "closed"
	PartyExpired PartyState = "expired"
)

type PartyPresenceStatus string

const (
	PartyPresenceOnline  PartyPresenceStatus = "online"
	PartyPresenceAway    PartyPresenceStatus = "away"
	PartyPresenceOffline PartyPresenceStatus = "offline"
)

type PartyMember struct {
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
	PresenceStatus PartyPresenceStatus `json:"presenceStatus,omitempty"`
	InActiveMatch  bool                `json:"inActiveMatch,omitempty"`
	JoinedAt       time.Time           `json:"joinedAt"`
}

type PartySnapshot struct {
	ID               string        `json:"id"`
	InviteCode       string        `json:"inviteCode"`
	OwnerUserID      string        `json:"ownerUserId"`
	State            PartyState    `json:"state"`
	Mode             MatchMode     `json:"mode"`
	MapScope         string        `json:"mapScope"`
	MapName          string        `json:"mapName,omitempty"`
	MapLocationCount int           `json:"mapLocationCount,omitempty"`
	Config           MatchConfig   `json:"config,omitempty"`
	ActiveMatchID    string        `json:"activeMatchId,omitempty"`
	LastMatchID      string        `json:"lastMatchId,omitempty"`
	StartedMatchID   string        `json:"startedMatchId,omitempty"`
	CreatedAt        time.Time     `json:"createdAt"`
	ExpiresAt        time.Time     `json:"expiresAt"`
	Members          []PartyMember `json:"members"`
}

type PartyPatch struct {
	Revision        int64         `json:"revision"`
	State           *PartyState   `json:"state,omitempty"`
	OwnerUserID     *string       `json:"ownerUserId,omitempty"`
	Mode            *MatchMode    `json:"mode,omitempty"`
	Config          *MatchConfig  `json:"config,omitempty"`
	ActiveMatchID   *string       `json:"activeMatchId,omitempty"`
	LastMatchID     *string       `json:"lastMatchId,omitempty"`
	StartedMatchID  *string       `json:"startedMatchId,omitempty"`
	UpsertMembers   []PartyMember `json:"upsertMembers,omitempty"`
	RemoveMemberIDs []string      `json:"removeMemberIds,omitempty"`
}

type PartyCreateRequest struct {
	Mode     MatchMode   `json:"mode,omitempty"`
	MapScope string      `json:"mapScope,omitempty"`
	Config   MatchConfig `json:"config,omitempty"`
}

type PartyMemberRequest struct {
	UserID string `json:"userId"`
}

type PartyTeamRequest struct {
	TeamID string `json:"teamId"`
}

type PartyStartResponse struct {
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
	ResolvedMap           ResolvedMap              `json:"resolvedMap"`
	PlannedRounds         []PlannedRound           `json:"plannedRounds"`
	MapAccessUserID       string                   `json:"mapAccessUserId,omitempty"`
	MapScope              string                   `json:"mapScope,omitempty"` // Legacy read compatibility.
	SourcePartyID         string                   `json:"sourcePartyId,omitempty"`
	SourcePartyInviteCode string                   `json:"sourcePartyInviteCode,omitempty"`
	ReturnTarget          *MatchReturnTarget       `json:"returnTarget,omitempty"`
}

type MatchPresetID string

const (
	MatchPresetRankedDuel  MatchPresetID = "ranked_duel"
	MatchPresetPrivateDuel MatchPresetID = "private_duel"
	MatchPresetTeamDuel    MatchPresetID = "team_duel"
	MatchPresetFreeForAll  MatchPresetID = "free_for_all"
	MatchPresetSolo        MatchPresetID = "solo"
)

type MatchParticipant struct {
	UserID        string    `json:"userId"`
	TeamID        string    `json:"teamId,omitempty"`
	JoinedPartyAt time.Time `json:"joinedPartyAt,omitempty"`
	InGame        bool      `json:"inGame"`
}

type ResolvedMap struct {
	MapID       string `json:"mapId"`
	DisplayName string `json:"displayName"`
}

type PlannedRound struct {
	RoundIndex int           `json:"roundIndex"`
	Location   LocationPoint `json:"location"`
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
	TrackedMatches    int                 `json:"trackedMatches"`
	RankedMatches     int                 `json:"rankedMatches"`
	DuelMatches       int                 `json:"duelMatches"`
	SingleplayerRuns  int                 `json:"singleplayerRuns"`
	Losses            int                 `json:"losses"`
	IsGuest           bool                `json:"isGuest"`
	IsAdmin           bool                `json:"isAdmin"`
	IsModerator       bool                `json:"isModerator"`
	IsBanned          bool                `json:"isBanned"`
	BanReason         string              `json:"banReason,omitempty"`
	BannedAt          time.Time           `json:"bannedAt,omitempty"`
	BanExpiresAt      time.Time           `json:"banExpiresAt,omitempty"`
	ChatMuteReason    string              `json:"chatMuteReason,omitempty"`
	ChatMutedAt       time.Time           `json:"chatMutedAt,omitempty"`
	ChatMutedUntil    time.Time           `json:"chatMutedUntil,omitempty"`
	ReportMuteReason  string              `json:"reportMuteReason,omitempty"`
	ReportMutedAt     time.Time           `json:"reportMutedAt,omitempty"`
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

type ModerationSignalSummary struct {
	ID               int64           `json:"id"`
	SubjectUserID    string          `json:"subjectUserId"`
	SubjectName      string          `json:"subjectName,omitempty"`
	SignalType       string          `json:"signalType"`
	Source           string          `json:"source"`
	Severity         string          `json:"severity"`
	EvidenceStrength string          `json:"evidenceStrength"`
	DetectorKey      string          `json:"detectorKey,omitempty"`
	DetectorVersion  string          `json:"detectorVersion,omitempty"`
	ReasonCode       string          `json:"reasonCode"`
	Score            float64         `json:"score"`
	RecommendedQueue bool            `json:"recommendedQueue"`
	ReporterUserID   string          `json:"reporterUserId,omitempty"`
	ReporterName     string          `json:"reporterName,omitempty"`
	MatchID          string          `json:"matchId,omitempty"`
	Payload          json.RawMessage `json:"payload,omitempty"`
	OccurredAt       time.Time       `json:"occurredAt"`
	CreatedAt        time.Time       `json:"createdAt"`
	ReviewedAt       time.Time       `json:"reviewedAt,omitempty"`
	ReviewedBy       string          `json:"reviewedBy,omitempty"`
	Outcome          string          `json:"outcome,omitempty"`
}

type ModerationAuditLogEntry struct {
	ID            int64           `json:"id"`
	SubjectUserID string          `json:"subjectUserId,omitempty"`
	SubjectName   string          `json:"subjectName,omitempty"`
	ActorUserID   string          `json:"actorUserId,omitempty"`
	ActorName     string          `json:"actorName,omitempty"`
	Action        string          `json:"action"`
	Reason        string          `json:"reason,omitempty"`
	ExpiresAt     time.Time       `json:"expiresAt,omitempty"`
	SignalIDs     []int64         `json:"signalIds,omitempty"`
	Metadata      json.RawMessage `json:"metadata,omitempty"`
	CreatedAt     time.Time       `json:"createdAt"`
}

type ModerationSubjectProfile struct {
	Player  AdminPlayerSummary        `json:"player"`
	Signals []ModerationSignalSummary `json:"signals"`
	Log     []ModerationAuditLogEntry `json:"log"`
}

type UserRoleGrant struct {
	UserID      string    `json:"userId"`
	DisplayName string    `json:"displayName,omitempty"`
	Email       string    `json:"email,omitempty"`
	Role        string    `json:"role"`
	GrantedBy   string    `json:"grantedBy,omitempty"`
	GrantedAt   time.Time `json:"grantedAt"`
	RevokedAt   time.Time `json:"revokedAt,omitempty"`
	Reason      string    `json:"reason,omitempty"`
}

type ModerationSignalCreated struct {
	SignalID int64  `json:"signalId"`
	Status   string `json:"status"`
}

type ModerationSignalNotificationPayload struct {
	SignalID         int64     `json:"signalId"`
	SubjectUserID    string    `json:"subjectUserId"`
	SubjectName      string    `json:"subjectName"`
	Severity         string    `json:"severity"`
	EvidenceStrength string    `json:"evidenceStrength"`
	ReasonCode       string    `json:"reasonCode"`
	OccurredAt       time.Time `json:"occurredAt"`
}

type MapImportSummary struct {
	MapID         string `json:"mapId"`
	MapKey        string `json:"mapKey"`
	LocationCount int    `json:"locationCount"`
	DisplayName   string `json:"displayName"`
}

type MapPersonalBest struct {
	Score      int       `json:"score"`
	MatchID    string    `json:"matchId"`
	AchievedAt time.Time `json:"achievedAt"`
}

type CustomMap struct {
	ID               string           `json:"id"`
	MapKey           string           `json:"mapKey"`
	OwnerUserID      string           `json:"ownerUserId,omitempty"`
	AuthorName       string           `json:"authorName,omitempty"`
	DisplayName      string           `json:"displayName"`
	Description      string           `json:"description,omitempty"`
	Visibility       string           `json:"visibility"`
	Status           string           `json:"status"`
	Difficulty       string           `json:"difficulty"`
	ThumbnailVariant int              `json:"thumbnailVariant"`
	ThumbnailKey     string           `json:"thumbnailKey"`
	LocationCount    int              `json:"locationCount"`
	PersonalBest     *MapPersonalBest `json:"personalBest,omitempty"`
	System           bool             `json:"system"`
	Official         bool             `json:"official,omitempty"`
	PublishedAt      *time.Time       `json:"publishedAt,omitempty"`
	PlayCount        int              `json:"playCount"`
	FavoriteCount    int              `json:"favoriteCount"`
	CommentCount     int              `json:"commentCount"`
	TrendingScore    float64          `json:"trendingScore"`
	Favorited        bool             `json:"favorited,omitempty"`
	OfficialRegion   string           `json:"officialRegion,omitempty"`
	ModeMoving       bool             `json:"modeMoving,omitempty"`
	ModeNoMove       bool             `json:"modeNoMove,omitempty"`
	ModeNMPZ         bool             `json:"modeNmpz,omitempty"`
	CreatedAt        time.Time        `json:"createdAt"`
	UpdatedAt        time.Time        `json:"updatedAt"`
}

type CustomMapUpdate struct {
	DisplayName      string `json:"displayName"`
	Description      string `json:"description"`
	Visibility       string `json:"visibility"`
	Difficulty       string `json:"difficulty"`
	ThumbnailVariant int    `json:"thumbnailVariant"`
	ThumbnailKey     string `json:"thumbnailKey"`
}

type MapListOptions struct {
	Scope  string `json:"scope"`
	Sort   string `json:"sort"`
	Search string `json:"search"`
}

type MapUploadQuota struct {
	Tier                     string `json:"tier"`
	TierOverride             string `json:"tierOverride,omitempty"`
	QualifiedFavorites       int    `json:"qualifiedFavorites"`
	QualifiedMaps            int    `json:"qualifiedMaps"`
	AccountAgeDays           int    `json:"accountAgeDays"`
	NextTier                 string `json:"nextTier,omitempty"`
	FavoritesNeeded          int    `json:"favoritesNeeded,omitempty"`
	MapsNeeded               int    `json:"mapsNeeded,omitempty"`
	DaysNeeded               int    `json:"daysNeeded,omitempty"`
	MaxMaps                  int    `json:"maxMaps"`
	MaxActiveLocations       int    `json:"maxActiveLocations"`
	MaxMapLocations          int    `json:"maxMapLocations"`
	MaxUploadsPerHour        int    `json:"maxUploadsPerHour"`
	MaxUploadsPerDay         int    `json:"maxUploadsPerDay"`
	MaxUploadedLocationsHour int    `json:"maxUploadedLocationsPerHour"`
	CurrentMaps              int    `json:"currentMaps"`
	CurrentActiveLocations   int    `json:"currentActiveLocations"`
	RestrictedByModeration   bool   `json:"restrictedByModeration,omitempty"`
}

type GameplayMapSettings struct {
	MovingMapID string `json:"movingMapId"`
	NoMoveMapID string `json:"noMoveMapId"`
	NMPZMapID   string `json:"nmpzMapId"`
}

type MapCountryStat struct {
	Country       string `json:"country"`
	LocationCount int    `json:"locationCount"`
}

type MapComment struct {
	ID              string       `json:"id"`
	MapID           string       `json:"mapId"`
	ParentID        string       `json:"parentId,omitempty"`
	UserID          string       `json:"userId"`
	UserDisplayName string       `json:"userDisplayName"`
	AvatarURL       string       `json:"avatarUrl,omitempty"`
	Body            string       `json:"body"`
	Status          string       `json:"status"`
	CanDelete       bool         `json:"canDelete,omitempty"`
	LikeCount       int          `json:"likeCount"`
	Liked           bool         `json:"liked,omitempty"`
	CreatedAt       time.Time    `json:"createdAt"`
	UpdatedAt       time.Time    `json:"updatedAt"`
	Replies         []MapComment `json:"replies,omitempty"`
}

type MapCommentCreate struct {
	Body     string `json:"body"`
	ParentID string `json:"parentId,omitempty"`
}

type MapDetails struct {
	Map          CustomMap        `json:"map"`
	CountryStats []MapCountryStat `json:"countryStats"`
	Comments     []MapComment     `json:"comments"`
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

type ChatAudience string

const (
	ChatAudienceAll  ChatAudience = "all"
	ChatAudienceTeam ChatAudience = "team"
)

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
	ChatEmoteWave       ChatEmote = "wave"
)

type ChatMessage struct {
	ID                string          `json:"id"`
	ConversationID    string          `json:"conversationId,omitempty"`
	MatchID           string          `json:"matchId"`
	SenderUserID      string          `json:"senderUserId"`
	SenderDisplayName string          `json:"senderDisplayName"`
	Kind              ChatMessageKind `json:"kind"`
	Audience          ChatAudience    `json:"audience"`
	TeamID            string          `json:"teamId,omitempty"`
	Body              string          `json:"body,omitempty"`
	Emote             ChatEmote       `json:"emote,omitempty"`
	CreatedAt         time.Time       `json:"createdAt"`
}

const (
	EventMatchSnapshot  = "match.snapshot"
	EventMatchState     = "match.state"
	EventTeamPing       = "team.ping"
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

// Session refresh records and creation parameters shared between the API
// service, the authsession domain service, and the persistence store.
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

type AuthSessionParams struct {
	UserAgent string
	IPAddress string
}

// Notification DTOs shared between the notifications domain service and the
// persistence store.
type UserNotification struct {
	ID        int64           `json:"id"`
	Type      string          `json:"type"`
	Category  string          `json:"category,omitempty"`
	Payload   json.RawMessage `json:"payload"`
	ReadAt    *time.Time      `json:"readAt,omitempty"`
	CreatedAt time.Time       `json:"createdAt"`
}

type NotificationOutboxItem struct {
	ID          int64
	Type        string
	PayloadJSON []byte
	Attempts    int
}

// Runtime match bookkeeping shared between the match-launch planner and the
// persistence store.
type RuntimeMatch struct {
	MatchID    string
	State      string
	OwnerEpoch int64
	StartedAt  time.Time
	EndedAt    time.Time
}

type MatchSessionUpsert struct {
	Found       MatchFound
	NodeID      string
	NodeEpoch   int64
	PublicRoute string
}
