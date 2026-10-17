package gameplay

import (
	"time"

	"geoduels/pkg/matchkind"
)

// What clients see of a match. These are the wire shapes sent to players and viewers; the engine
// state in snapshot.go never goes out as is.

// ClientMatchConfig is a match's config as clients send and receive it.
type ClientMatchConfig struct {
	Ruleset             Ruleset        `json:"ruleset,omitempty"`
	StreetNames         StreetNames    `json:"streetNames,omitempty"`
	MapID               string         `json:"mapId,omitempty"`
	MapName             string         `json:"mapName,omitempty"`
	MapKey              string         `json:"mapKey,omitempty"` // Legacy read compatibility.
	RoundTimerMode      RoundTimerMode `json:"roundTimerMode,omitempty"`
	RoundTimeLimitMS    int64          `json:"roundTimeLimitMs,omitempty"`
	PressureTimeLimitMS int64          `json:"pressureTimeLimitMs,omitempty"`
	MultiplierMode      MultiplierMode `json:"multiplierMode,omitempty"`
}

func NewClientMatchConfig(cfg MatchConfig) ClientMatchConfig {
	return ClientMatchConfig{
		Ruleset: cfg.Ruleset, StreetNames: cfg.StreetNames, MapID: cfg.MapID, MapName: cfg.MapName, MapKey: cfg.MapKey,
		RoundTimerMode: cfg.RoundTimerMode, RoundTimeLimitMS: cfg.RoundTimeLimitMS,
		PressureTimeLimitMS: cfg.PressureTimeLimitMS, MultiplierMode: cfg.MultiplierMode,
	}
}

// MatchConfig is the config a client asked for, not yet normalized.
func (c ClientMatchConfig) MatchConfig() MatchConfig {
	return MatchConfig{
		Ruleset: c.Ruleset, StreetNames: c.StreetNames, MapID: c.MapID, MapName: c.MapName, MapKey: c.MapKey,
		RoundTimerMode: c.RoundTimerMode, RoundTimeLimitMS: c.RoundTimeLimitMS,
		PressureTimeLimitMS: c.PressureTimeLimitMS, MultiplierMode: c.MultiplierMode,
	}
}

type ClientBadge struct {
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

func newClientBadge(b *Badge) *ClientBadge {
	if b == nil {
		return nil
	}
	return &ClientBadge{
		ID: b.ID, Kind: b.Kind, Label: b.Label, Description: b.Description, ImageURL: b.ImageURL, Rarity: b.Rarity,
		Level: b.Level, MaxLevel: b.MaxLevel, Extra: b.Extra, Owned: b.Owned, Unobtainable: b.Unobtainable,
	}
}

type ClientLocation struct {
	Lat     float64  `json:"lat"`
	Lng     float64  `json:"lng"`
	Country string   `json:"country"`
	PanoID  *string  `json:"panoId,omitempty"`
	Heading *float64 `json:"heading,omitempty"`
	Pitch   *float64 `json:"pitch,omitempty"`
}

func newClientLocation(l LocationPoint) ClientLocation {
	return ClientLocation{Lat: l.Lat, Lng: l.Lng, Country: l.Country, PanoID: l.PanoID, Heading: l.Heading, Pitch: l.Pitch}
}

type ClientTeam struct {
	TeamID  string   `json:"teamId"`
	Name    string   `json:"name,omitempty"`
	HP      int      `json:"hp,omitempty"`
	Players []string `json:"players"`
}

type ClientRatingPreview struct {
	Win  int `json:"win"`
	Lose int `json:"lose"`
	Draw int `json:"draw"`
}

type ClientRoundPlayerResult struct {
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

type ClientRoundTeamResult struct {
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

type ClientRoundResult struct {
	RoundID          string                             `json:"roundId"`
	RoundNumber      int                                `json:"roundNumber"`
	ActualLocation   ClientLocation                     `json:"actualLocation"`
	Players          map[string]ClientRoundPlayerResult `json:"players"`
	Teams            map[string]ClientRoundTeamResult   `json:"teams,omitempty"`
	DamageMultiplier float64                            `json:"damageMultiplier,omitempty"`
}

func newClientRoundResult(r *RoundResult) *ClientRoundResult {
	if r == nil {
		return nil
	}
	out := &ClientRoundResult{
		RoundID: r.RoundID, RoundNumber: r.RoundNumber, ActualLocation: newClientLocation(r.ActualLocation),
		DamageMultiplier: r.DamageMultiplier,
	}
	if r.Players != nil {
		out.Players = make(map[string]ClientRoundPlayerResult, len(r.Players))
		for id, p := range r.Players {
			out.Players[id] = ClientRoundPlayerResult{
				UserID: p.UserID, Lat: p.Lat, Lng: p.Lng, DistanceKm: p.DistanceKm, Score: p.Score,
				DamageDealt: p.DamageDealt, DamageTaken: p.DamageTaken, HPAfterRound: p.HPAfterRound,
				GuessUnixMS: p.GuessUnixMS, GuessMS: p.GuessMS,
			}
		}
	}
	if r.Teams != nil {
		out.Teams = make(map[string]ClientRoundTeamResult, len(r.Teams))
		for id, t := range r.Teams {
			out.Teams[id] = ClientRoundTeamResult{
				TeamID: t.TeamID, RepresentativeUserID: t.RepresentativeUserID, Lat: t.Lat, Lng: t.Lng,
				DistanceKm: t.DistanceKm, Score: t.Score, DamageDealt: t.DamageDealt, DamageTaken: t.DamageTaken,
				HPAfterRound: t.HPAfterRound,
			}
		}
	}
	return out
}

// clientMatchParts are the parts of a match every viewer sees the same.
type clientMatchParts struct {
	lastRoundResult *ClientRoundResult
	roundResults    []*ClientRoundResult
	teams           map[string]ClientTeam
	ratingPreview   map[string]ClientRatingPreview
}

func newClientMatchParts(snap *MatchSnapshot) clientMatchParts {
	var parts clientMatchParts
	mapped := map[*RoundResult]*ClientRoundResult{}
	result := func(r *RoundResult) *ClientRoundResult {
		if r == nil {
			return nil
		}
		if out, ok := mapped[r]; ok {
			return out
		}
		out := newClientRoundResult(r)
		mapped[r] = out
		return out
	}
	if snap.RoundResults != nil {
		parts.roundResults = make([]*ClientRoundResult, len(snap.RoundResults))
		for i, r := range snap.RoundResults {
			parts.roundResults[i] = result(r)
		}
	}
	parts.lastRoundResult = result(snap.LastRoundResult)
	if snap.Teams != nil {
		parts.teams = make(map[string]ClientTeam, len(snap.Teams))
		for id, t := range snap.Teams {
			parts.teams[id] = ClientTeam{TeamID: t.TeamID, Name: t.Name, HP: t.HP, Players: t.Players}
		}
	}
	if snap.RatingPreview != nil {
		parts.ratingPreview = make(map[string]ClientRatingPreview, len(snap.RatingPreview))
		for id, p := range snap.RatingPreview {
			parts.ratingPreview[id] = ClientRatingPreview{Win: p.Win, Lose: p.Lose, Draw: p.Draw}
		}
	}
	return parts
}

// ClientMatchSnapshot is a live match as one of its players sees it: the round's location stays
// hidden, and the only guesses shown are the player's own and, in a team duel, their team's.
type ClientMatchSnapshot struct {
	MatchID         string                         `json:"matchId"`
	Mode            matchkind.Mode                 `json:"mode"`
	Kind            matchkind.Kind                 `json:"kind,omitempty"`
	Config          ClientMatchConfig              `json:"config,omitempty"`
	State           MatchState                     `json:"state"`
	Phase           MatchPhase                     `json:"phase"`
	RoundPhase      RoundPhase                     `json:"roundPhase"`
	PhaseStartedAt  int64                          `json:"phaseStartedAt"`
	PhaseEndsAt     int64                          `json:"phaseEndsAt"`
	CurrentRound    *ClientRoundState              `json:"currentRound,omitempty"`
	LastRoundResult *ClientRoundResult             `json:"lastRoundResult,omitempty"`
	RoundResults    []*ClientRoundResult           `json:"roundResults,omitempty"`
	RoundMSLeft     int64                          `json:"roundMsLeft"`
	Players         map[string]ClientPlayerState   `json:"players"`
	Teams           map[string]ClientTeam          `json:"teams,omitempty"`
	Self            *ClientSelfState               `json:"self,omitempty"`
	Team            *ClientTeamState               `json:"team,omitempty"`
	RatingPreview   map[string]ClientRatingPreview `json:"ratingPreview,omitempty"`
	EventSequence   int64                          `json:"eventSequence"`
	ServerUnixMS    int64                          `json:"serverUnixMs"`
	GraceWindowSec  int                            `json:"graceWindowSec"`
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
	SelectedBadge     *ClientBadge `json:"selectedBadge,omitempty"`
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

// ClientView builds each player's ClientMatchSnapshot of one snapshot, sharing what they all see.
type ClientView struct {
	snap *MatchSnapshot
	base *ClientMatchSnapshot
}

func NewClientView(snap *MatchSnapshot) *ClientView {
	if snap == nil {
		return &ClientView{}
	}
	parts := newClientMatchParts(snap)
	base := &ClientMatchSnapshot{
		MatchID:         snap.MatchID,
		Mode:            snap.Mode,
		Kind:            snap.Kind,
		Config:          NewClientMatchConfig(NormalizeMatchConfig(snap.Config)),
		State:           snap.State,
		Phase:           snap.Phase,
		RoundPhase:      snap.RoundPhase,
		PhaseStartedAt:  snap.PhaseStartedAt,
		PhaseEndsAt:     snap.PhaseEndsAt,
		CurrentRound:    clientRoundState(snap.CurrentRound),
		LastRoundResult: parts.lastRoundResult,
		RoundResults:    parts.roundResults,
		RoundMSLeft:     snap.RoundMSLeft,
		Teams:           parts.teams,
		RatingPreview:   parts.ratingPreview,
		EventSequence:   snap.EventSequence,
		ServerUnixMS:    snap.ServerUnixMS,
		GraceWindowSec:  snap.GraceWindowSec,
	}
	if snap.Players != nil {
		base.Players = make(map[string]ClientPlayerState, len(snap.Players))
		for id, player := range snap.Players {
			base.Players[id] = ClientPlayerState{
				UserID:            player.UserID,
				DisplayName:       player.DisplayName,
				MMR:               player.MMR,
				RatingRD:          player.RatingRD,
				RankedGamesPlayed: player.RankedGamesPlayed,
				AvatarURL:         player.AvatarURL,
				IsGuest:           player.IsGuest,
				IsAdmin:           player.IsAdmin,
				SelectedBadge:     newClientBadge(player.SelectedBadge),
				TeamID:            player.TeamID,
				HP:                player.HP,
				TotalScore:        player.TotalScore,
				Finalized:         player.Finalized,
				Disconnected:      player.Disconnected,
				DisconnectDue:     player.DisconnectDue,
				DamageMultiplier:  player.DamageMultiplier,
			}
		}
	}
	return &ClientView{snap: snap, base: base}
}

// For is the match as userID sees it.
func (v *ClientView) For(userID string) *ClientMatchSnapshot {
	if v.base == nil {
		return nil
	}
	snap := v.snap
	client := *v.base
	if player, ok := snap.Players[userID]; ok {
		client.Self = clientSelfState(snap, player)
	}
	if snap.Mode == matchkind.ModeTeamDuel && snap.Phase == PhaseLive && snap.RoundPhase == RoundPhaseLive {
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
	return &client
}

func ClientSnapshotForPlayer(snap *MatchSnapshot, userID string) *ClientMatchSnapshot {
	return NewClientView(snap).For(userID)
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

// ClientMatchResult is a recorded match that ended, as anyone may see it: everything is revealed.
type ClientMatchResult struct {
	MatchID         string                         `json:"matchId"`
	Mode            matchkind.Mode                 `json:"mode"`
	Kind            matchkind.Kind                 `json:"kind,omitempty"`
	SeasonID        string                         `json:"seasonId,omitempty"`
	Config          ClientMatchConfig              `json:"config,omitempty"`
	State           MatchState                     `json:"state"`
	Phase           MatchPhase                     `json:"phase"`
	RoundPhase      RoundPhase                     `json:"roundPhase"`
	PhaseStartedAt  int64                          `json:"phaseStartedAt"`
	PhaseEndsAt     int64                          `json:"phaseEndsAt"`
	CurrentRound    *ClientResultRound             `json:"currentRound,omitempty"`
	LastRoundResult *ClientRoundResult             `json:"lastRoundResult,omitempty"`
	RoundResults    []*ClientRoundResult           `json:"roundResults,omitempty"`
	RoundMSLeft     int64                          `json:"roundMsLeft"`
	Players         map[string]ClientResultPlayer  `json:"players"`
	Teams           map[string]ClientTeam          `json:"teams,omitempty"`
	RatingPreview   map[string]ClientRatingPreview `json:"ratingPreview,omitempty"`
	EventSequence   int64                          `json:"eventSequence"`
	ServerUnixMS    int64                          `json:"serverUnixMs"`
	GraceWindowSec  int                            `json:"graceWindowSec"`
}

type ClientResultRound struct {
	RoundID       string         `json:"roundId"`
	RoundNumber   int            `json:"roundNumber"`
	RoundDeadline time.Time      `json:"roundDeadline"`
	TimerStarted  bool           `json:"timerStarted"`
	Location      ClientLocation `json:"location"`
}

type ClientResultPlayer struct {
	UserID            string       `json:"userId"`
	DisplayName       string       `json:"displayName"`
	MMR               int          `json:"mmr"`
	RatingRD          float64      `json:"ratingRd,omitempty"`
	RankedGamesPlayed int          `json:"rankedGamesPlayed,omitempty"`
	AvatarURL         string       `json:"avatarUrl,omitempty"`
	IsGuest           bool         `json:"isGuest,omitempty"`
	IsAdmin           bool         `json:"isAdmin,omitempty"`
	SelectedBadge     *ClientBadge `json:"selectedBadge,omitempty"`
	TeamID            string       `json:"teamId,omitempty"`
	HP                int          `json:"hp"`
	TotalScore        int          `json:"totalScore,omitempty"`
	Finalized         bool         `json:"finalized"`
	LastGuessLat      float64      `json:"lastGuessLat"`
	LastGuessLng      float64      `json:"lastGuessLng"`
	Disconnected      bool         `json:"disconnected"`
	DisconnectDue     int64        `json:"disconnectDue"`
	DamageMultiplier  float64      `json:"damageMultiplier"`
}

func NewClientMatchResult(snap *MatchSnapshot) *ClientMatchResult {
	if snap == nil {
		return nil
	}
	parts := newClientMatchParts(snap)
	out := &ClientMatchResult{
		MatchID:         snap.MatchID,
		Mode:            snap.Mode,
		Kind:            snap.Kind,
		SeasonID:        snap.SeasonID,
		Config:          NewClientMatchConfig(snap.Config),
		State:           snap.State,
		Phase:           snap.Phase,
		RoundPhase:      snap.RoundPhase,
		PhaseStartedAt:  snap.PhaseStartedAt,
		PhaseEndsAt:     snap.PhaseEndsAt,
		LastRoundResult: parts.lastRoundResult,
		RoundResults:    parts.roundResults,
		RoundMSLeft:     snap.RoundMSLeft,
		Teams:           parts.teams,
		RatingPreview:   parts.ratingPreview,
		EventSequence:   snap.EventSequence,
		ServerUnixMS:    snap.ServerUnixMS,
		GraceWindowSec:  snap.GraceWindowSec,
	}
	if round := snap.CurrentRound; round != nil {
		out.CurrentRound = &ClientResultRound{
			RoundID: round.RoundID, RoundNumber: round.RoundNumber, RoundDeadline: round.RoundDeadline,
			TimerStarted: round.TimerStarted, Location: newClientLocation(round.Location),
		}
	}
	if snap.Players != nil {
		out.Players = make(map[string]ClientResultPlayer, len(snap.Players))
		for id, p := range snap.Players {
			out.Players[id] = ClientResultPlayer{
				UserID: p.UserID, DisplayName: p.DisplayName, MMR: p.MMR, RatingRD: p.RatingRD,
				RankedGamesPlayed: p.RankedGamesPlayed, AvatarURL: p.AvatarURL, IsGuest: p.IsGuest, IsAdmin: p.IsAdmin,
				SelectedBadge: newClientBadge(p.SelectedBadge), TeamID: p.TeamID, HP: p.HP, TotalScore: p.TotalScore,
				Finalized: p.Finalized, LastGuessLat: p.LastGuessLat, LastGuessLng: p.LastGuessLng,
				Disconnected: p.Disconnected, DisconnectDue: p.DisconnectDue, DamageMultiplier: p.DamageMultiplier,
			}
		}
	}
	return out
}
