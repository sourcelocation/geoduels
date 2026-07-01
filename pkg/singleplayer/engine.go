package singleplayer

import (
	"errors"
	"sync"
	"time"

	"geoduels/pkg/contracts"
	"geoduels/pkg/gameplay"
)

const maxRounds = 5

type Guess struct {
	Lat       float64
	Lng       float64
	Finalized bool
	Ts        time.Time
}

type Session struct {
	ID              string
	Config          contracts.MatchConfig
	Player          *contracts.PlayerState
	CurrentLocation contracts.LocationPoint
	CurrentIndex    int
	RoundID         string
	RoundStartedAt  time.Time
	Guess           *Guess
	LastRoundResult *contracts.RoundResult
	RoundResults    []*contracts.RoundResult
	AwaitingAdvance bool
	State           contracts.MatchState
	EventSeq        int64
	CreatedAt       time.Time
	LastActivity    time.Time
}

type RoundProvider func(matchID string, roundIndex int) (contracts.LocationPoint, error)

type Engine struct {
	mu            sync.RWMutex
	sessions      map[string]*Session
	roundProvider RoundProvider
}

func New(roundProvider RoundProvider) *Engine {
	return &Engine{
		sessions:      map[string]*Session{},
		roundProvider: roundProvider,
	}
}

func (e *Engine) CreateMatch(matchID string, playerIDs []string, profiles map[string]contracts.PlayerProfile) (*Session, error) {
	return e.CreateMatchWithConfig(matchID, playerIDs, profiles, contracts.MatchConfig{})
}

func (e *Engine) CreateMatchWithConfig(matchID string, playerIDs []string, profiles map[string]contracts.PlayerProfile, config contracts.MatchConfig) (*Session, error) {
	if len(playerIDs) != 1 {
		return nil, errors.New("singleplayer requires exactly one player")
	}
	if e.roundProvider == nil {
		return nil, errors.New("round provider required")
	}
	firstRound, err := e.roundProvider(matchID, 0)
	if err != nil {
		return nil, err
	}
	playerID := playerIDs[0]
	profile := profiles[playerID]
	name := profile.DisplayName
	if name == "" {
		name = playerID
	}
	now := time.Now()
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, ok := e.sessions[matchID]; ok {
		return nil, errors.New("match already exists")
	}
	config = contracts.NormalizeMatchConfig(config)
	session := &Session{
		ID:              matchID,
		Config:          config,
		Player:          &contracts.PlayerState{UserID: playerID, DisplayName: name, MMR: profile.MMR, RatingRD: profile.RatingRD, RankedGamesPlayed: profile.RankedGamesPlayed, AvatarURL: profile.AvatarURL, IsGuest: profile.IsGuest, IsAdmin: profile.IsAdmin, SelectedBadge: profile.SelectedBadge},
		CurrentLocation: firstRound,
		CurrentIndex:    0,
		RoundID:         roundID(matchID, 1),
		RoundStartedAt:  now,
		State:           contracts.MatchLive,
		EventSeq:        1,
		CreatedAt:       now,
		LastActivity:    now,
	}
	e.sessions[matchID] = session
	return session, nil
}

func (e *Engine) GetSnapshot(matchID string) (*contracts.MatchSnapshot, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	session, ok := e.sessions[matchID]
	if !ok {
		return nil, errors.New("match not found")
	}
	return session.snapshot(), nil
}

func (e *Engine) SubmitGuess(g contracts.GuessPayload) (*contracts.MatchSnapshot, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	session, ok := e.sessions[g.MatchID]
	if !ok {
		return nil, errors.New("match not found")
	}
	if session.State != contracts.MatchLive {
		return session.snapshot(), nil
	}
	if session.Player.UserID != g.UserID {
		return nil, errors.New("player not in match")
	}
	if session.RoundID != g.RoundID {
		return nil, errors.New("round mismatch")
	}
	if session.AwaitingAdvance {
		return session.snapshot(), nil
	}
	now := time.Now()
	session.Guess = &Guess{Lat: g.Lat, Lng: g.Lng, Finalized: g.Finalize, Ts: now}
	session.Player.LastGuessLat = g.Lat
	session.Player.LastGuessLng = g.Lng
	session.Player.HasGuess = true
	session.Player.Finalized = g.Finalize
	session.Player.Disconnected = false
	session.Player.DisconnectDue = 0
	session.LastActivity = now
	session.EventSeq++
	if g.Finalize {
		e.resolveRound(session)
	}
	return session.snapshot(), nil
}

func (e *Engine) AdvanceRound(matchID, userID string) (*contracts.MatchSnapshot, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	session, ok := e.sessions[matchID]
	if !ok {
		return nil, errors.New("match not found")
	}
	if session.Player.UserID != userID {
		return nil, errors.New("player not in match")
	}
	if session.State != contracts.MatchLive {
		return session.snapshot(), nil
	}
	if !session.AwaitingAdvance {
		return session.snapshot(), nil
	}
	if session.CurrentIndex+1 >= maxRounds {
		session.State = contracts.MatchEnded
		session.AwaitingAdvance = false
		session.LastActivity = time.Now()
		session.EventSeq++
		return session.snapshot(), nil
	}
	nextIndex := session.CurrentIndex + 1
	nextLoc, err := e.roundProvider(matchID, nextIndex)
	if err != nil {
		return nil, err
	}
	session.CurrentIndex = nextIndex
	session.CurrentLocation = nextLoc
	session.RoundID = roundID(matchID, nextIndex+1)
	session.RoundStartedAt = time.Now()
	session.Guess = nil
	session.LastRoundResult = nil
	session.AwaitingAdvance = false
	session.Player.Finalized = false
	session.Player.LastGuessLat = 0
	session.Player.LastGuessLng = 0
	session.Player.HasGuess = false
	session.LastActivity = time.Now()
	session.EventSeq++
	return session.snapshot(), nil
}

func (e *Engine) Forfeit(matchID, userID string) (*contracts.MatchSnapshot, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	session, ok := e.sessions[matchID]
	if !ok {
		return nil, errors.New("match not found")
	}
	if session.Player.UserID != userID {
		return nil, errors.New("player not in match")
	}
	session.State = contracts.MatchEnded
	session.AwaitingAdvance = false
	session.Player.Finalized = false
	session.LastActivity = time.Now()
	session.EventSeq++
	return session.snapshot(), nil
}

func (e *Engine) Tick() {}

func (e *Engine) MarkDisconnected(matchID, userID string) (*contracts.MatchSnapshot, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	session, ok := e.sessions[matchID]
	if !ok {
		return nil, errors.New("match not found")
	}
	if session.Player.UserID != userID {
		return nil, errors.New("player not in match")
	}
	session.Player.Disconnected = true
	session.LastActivity = time.Now()
	session.EventSeq++
	return session.snapshot(), nil
}

func (e *Engine) MarkResumed(matchID, userID string) (*contracts.MatchSnapshot, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	session, ok := e.sessions[matchID]
	if !ok {
		return nil, errors.New("match not found")
	}
	if session.Player.UserID != userID {
		return nil, errors.New("player not in match")
	}
	session.Player.Disconnected = false
	session.LastActivity = time.Now()
	session.EventSeq++
	return session.snapshot(), nil
}

func (e *Engine) resolveRound(session *Session) {
	if session.Guess == nil {
		return
	}
	guess := *session.Guess
	dist := gameplay.ClampDistanceKm(gameplay.HaversineKm(session.CurrentLocation.Lat, session.CurrentLocation.Lng, guess.Lat, guess.Lng))
	score := gameplay.RoundScore(dist)
	session.Player.TotalScore += score
	session.Player.Finalized = false
	session.LastRoundResult = &contracts.RoundResult{
		RoundID:        session.RoundID,
		RoundNumber:    session.CurrentIndex + 1,
		ActualLocation: session.CurrentLocation,
		Players: map[string]contracts.RoundPlayerResult{
			session.Player.UserID: {
				UserID:      session.Player.UserID,
				Lat:         guess.Lat,
				Lng:         guess.Lng,
				DistanceKm:  dist,
				Score:       score,
				GuessUnixMS: guessUnixMS(guess),
				GuessMS:     guessMS(guess, session.RoundStartedAt),
			},
		},
	}
	session.RoundResults = append(session.RoundResults, session.LastRoundResult)
	session.Guess = nil
	session.AwaitingAdvance = true
	session.LastActivity = time.Now()
	session.EventSeq++
}

func guessUnixMS(g Guess) int64 {
	if g.Ts.IsZero() {
		return 0
	}
	return g.Ts.UnixMilli()
}

func guessMS(g Guess, roundStartedAt time.Time) int64 {
	if g.Ts.IsZero() || roundStartedAt.IsZero() {
		return 0
	}
	ms := g.Ts.Sub(roundStartedAt).Milliseconds()
	if ms < 0 {
		return 0
	}
	return ms
}

func (s *Session) snapshot() *contracts.MatchSnapshot {
	players := map[string]contracts.PlayerState{
		s.Player.UserID: *s.Player,
	}
	now := time.Now().UnixMilli()
	phase := contracts.PhaseLive
	roundPhase := contracts.RoundPhaseLive
	currentRound := &contracts.RoundState{
		RoundID:       s.RoundID,
		RoundNumber:   s.CurrentIndex + 1,
		RoundDeadline: time.UnixMilli(0),
		Location:      s.CurrentLocation,
	}
	if s.State == contracts.MatchEnded {
		phase = contracts.PhaseEnded
		roundPhase = contracts.RoundPhaseEnded
		currentRound = nil
	} else if s.AwaitingAdvance {
		phase = contracts.PhaseRoundResult
		roundPhase = contracts.RoundPhaseResult
		currentRound = nil
	}
	return &contracts.MatchSnapshot{
		MatchID:         s.ID,
		Mode:            contracts.ModeSingleplayer,
		State:           s.State,
		Phase:           phase,
		RoundPhase:      roundPhase,
		PhaseStartedAt:  now,
		PhaseEndsAt:     now,
		CurrentRound:    currentRound,
		LastRoundResult: s.LastRoundResult,
		RoundResults:    append([]*contracts.RoundResult(nil), s.RoundResults...),
		RoundMSLeft:     0,
		Players:         players,
		Config:          contracts.NormalizeMatchConfig(s.Config),
		EventSequence:   s.EventSeq,
		ServerUnixMS:    now,
		GraceWindowSec:  0,
	}
}

func roundID(matchID string, round int) string {
	return matchID + ":r" + strconv(round)
}

func strconv(n int) string {
	if n == 0 {
		return "0"
	}
	out := ""
	for n > 0 {
		d := n % 10
		out = string(rune('0'+d)) + out
		n = n / 10
	}
	return out
}
