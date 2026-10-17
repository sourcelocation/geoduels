package gameplay

import (
	"time"

	"geoduels/pkg/matchkind"
)

// MatchSnapshot is an engine's whole state of a match. Its JSON is how a finished match's replay is
// stored; clients see it only through the views in client.go.
type MatchSnapshot struct {
	MatchID         string                        `json:"matchId"`
	Mode            matchkind.Mode                `json:"mode"`
	Kind            matchkind.Kind                `json:"kind,omitempty"`
	SeasonID        string                        `json:"seasonId,omitempty"`
	Config          MatchConfig                   `json:"config,omitempty"`
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

// LocationPoint is a round's location.
type LocationPoint struct {
	Lat     float64  `json:"lat"`
	Lng     float64  `json:"lng"`
	Country string   `json:"country"`
	PanoID  *string  `json:"panoId,omitempty"`
	Heading *float64 `json:"heading,omitempty"`
	Pitch   *float64 `json:"pitch,omitempty"`
}

// Guess is a player placing, or finalizing, their guess for a round.
type Guess struct {
	UserID         string
	MatchID        string
	RoundID        string
	Lat            float64
	Lng            float64
	IdempotencyKey string
	Finalize       bool
}

// PlayerProfile is who a player is in a match, fixed when the match starts. Its JSON is part of how
// a match session stores its seats.
type PlayerProfile struct {
	UserID            string  `json:"userId"`
	DisplayName       string  `json:"displayName"`
	MMR               int     `json:"mmr"`
	RatingRD          float64 `json:"ratingRd,omitempty"`
	RankedGamesPlayed int     `json:"rankedGamesPlayed,omitempty"`
	AvatarURL         string  `json:"avatarUrl,omitempty"`
	IsGuest           bool    `json:"isGuest,omitempty"`
	IsAdmin           bool    `json:"isAdmin,omitempty"`
	SelectedBadge     *Badge  `json:"selectedBadge,omitempty"`
}

// Badge is the badge a player shows in a match, as it was when the match started.
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
	SelectedBadge     *Badge  `json:"selectedBadge,omitempty"`
	TeamID            string  `json:"teamId,omitempty"`
	HP                int     `json:"hp"`
	TotalScore        int     `json:"totalScore,omitempty"`
	Finalized         bool    `json:"finalized"`
	LastGuessLat      float64 `json:"lastGuessLat"`
	LastGuessLng      float64 `json:"lastGuessLng"`
	HasGuess          bool    `json:"-"`
	Disconnected      bool    `json:"disconnected"`
	DisconnectDue     int64   `json:"disconnectDue"`
	DamageMultiplier  float64 `json:"damageMultiplier"`
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
