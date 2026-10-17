package duel

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"geoduels/internal/rating"
	"geoduels/pkg/gameplay"
	"geoduels/pkg/matchkind"
)

const (
	startingHP      = 6000
	roundDuration   = 45 * time.Second
	roundIdleCap    = 8 * time.Minute
	roundIntro      = 3 * time.Second
	resultDuration  = 6 * time.Second
	disconnectGrace = 30 * time.Second
	staleGrace      = 3 * time.Minute
	maxRounds       = 20
	ffaRounds       = 5
	maxDistanceKm   = math.Pi * 6371.0
	maxScore        = gameplay.MaxScore
	perfectGuessKm  = 0.15
)

type Guess struct {
	Lat       float64
	Lng       float64
	Finalized bool
	Ts        time.Time
}

type Match struct {
	ID       string
	Mode     matchkind.Mode
	Kind     matchkind.Kind
	SeasonID string
	Config   gameplay.MatchConfig
	State    gameplay.MatchState
	// Rated matches preview rating changes and forfeit a player who stays disconnected.
	Rated              bool
	Players            map[string]*gameplay.PlayerState
	Teams              map[string]*gameplay.TeamState
	CurrentLocation    gameplay.LocationPoint
	CurrentIndex       int
	RoundStartedAt     time.Time
	RoundDeadline      time.Time
	RoundID            string
	Guesses            map[string]Guess
	LastRoundResult    *gameplay.RoundResult
	RoundResults       []*gameplay.RoundResult
	RatingPreview      map[string]gameplay.RatingDeltaPreview
	IntermissionUntil  time.Time
	PendingAdvance     bool
	EventSeq           int64
	RoundLiveAnnounced bool
	CreatedAt          time.Time
	LastActivity       time.Time
	now                func() time.Time
}

type RoundProvider func(matchID string, roundIndex int) (gameplay.LocationPoint, error)

type Engine struct {
	mu            sync.RWMutex
	matches       map[string]*Match
	roundProvider RoundProvider
	now           func() time.Time
}

func New(roundProvider RoundProvider) *Engine {
	return NewWithClock(roundProvider, time.Now)
}

// NewWithClock makes all authoritative timestamps deterministic in tests and
// replay tools. Production callers should use New.
func NewWithClock(roundProvider RoundProvider, now func() time.Time) *Engine {
	if now == nil {
		now = time.Now
	}
	return &Engine{matches: map[string]*Match{}, roundProvider: roundProvider, now: now}
}

// CreateMatch creates a ranked duel.
func (e *Engine) CreateMatch(matchID string, playerIDs []string, profiles map[string]gameplay.PlayerProfile) (*Match, error) {
	return e.CreateMatchWithOptions(matchID, playerIDs, profiles, MatchOptions{Kind: matchkind.RankedDuel})
}

type MatchOptions struct {
	Kind     matchkind.Kind
	SeasonID string
	Config   gameplay.MatchConfig
	Teams    map[string]string
}

func (e *Engine) CreateMatchWithOptions(matchID string, playerIDs []string, profiles map[string]gameplay.PlayerProfile, opts MatchOptions) (*Match, error) {
	spec := matchkind.Of(opts.Kind)
	mode := spec.Mode
	switch mode {
	case matchkind.ModeDuel:
		if len(playerIDs) != 2 {
			return nil, errors.New("duel requires exactly two players")
		}
	case matchkind.ModeTeamDuel, matchkind.ModeFreeForAll:
		if len(playerIDs) < matchkind.MinPartyMembers || len(playerIDs) > matchkind.MaxPartyMembers {
			return nil, fmt.Errorf("party match requires %d to %d players", matchkind.MinPartyMembers, matchkind.MaxPartyMembers)
		}
	default:
		return nil, errors.New("unsupported duel mode")
	}
	for _, id := range playerIDs {
		if strings.TrimSpace(id) == "" {
			return nil, errors.New("player id required")
		}
	}
	if e.roundProvider == nil {
		return nil, errors.New("round provider required")
	}
	firstRound, err := e.roundProvider(matchID, 0)
	if err != nil {
		return nil, err
	}
	cfg := gameplay.NormalizeMatchConfig(opts.Config)
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, ok := e.matches[matchID]; ok {
		return nil, errors.New("match already exists")
	}
	players := map[string]*gameplay.PlayerState{}
	for _, id := range playerIDs {
		p := profiles[id]
		name := p.DisplayName
		if name == "" {
			name = id
		}
		teamID := ""
		if mode == matchkind.ModeTeamDuel {
			teamID = normalizeTeamID(opts.Teams[id])
		}
		players[id] = &gameplay.PlayerState{
			UserID:            id,
			DisplayName:       name,
			MMR:               p.MMR,
			RatingRD:          p.RatingRD,
			RankedGamesPlayed: p.RankedGamesPlayed,
			AvatarURL:         p.AvatarURL,
			IsGuest:           p.IsGuest,
			IsAdmin:           p.IsAdmin,
			SelectedBadge:     p.SelectedBadge,
			TeamID:            teamID,
			HP:                startingHP,
			DamageMultiplier:  1,
		}
	}
	teams := buildTeams(mode, playerIDs, players)
	if mode == matchkind.ModeTeamDuel && len(teams) != 2 {
		return nil, errors.New("team duel requires both teams")
	}
	m := &Match{
		ID:              matchID,
		Mode:            mode,
		Kind:            spec.Kind,
		SeasonID:        opts.SeasonID,
		Config:          cfg,
		State:           gameplay.MatchLive,
		Rated:           spec.Rated,
		Players:         players,
		Teams:           teams,
		CurrentLocation: firstRound,
		CurrentIndex:    0,
		RoundStartedAt:  e.now(),
		RoundID:         roundID(matchID, 1),
		Guesses:         map[string]Guess{},
		EventSeq:        1,
		CreatedAt:       e.now(),
		LastActivity:    e.now(),
		now:             e.now,
	}
	e.startRoundTimer(m)
	if m.Rated {
		m.RatingPreview = ratingPreview(playerIDs, players, e.now())
	}
	e.matches[matchID] = m
	return m, nil
}

func (e *Engine) GetSnapshot(matchID string) (*gameplay.MatchSnapshot, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	m, ok := e.matches[matchID]
	if !ok {
		return nil, errors.New("match not found")
	}
	return m.snapshot(), nil
}

func (e *Engine) SubmitGuess(g gameplay.Guess) (*gameplay.MatchSnapshot, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	m, ok := e.matches[g.MatchID]
	if !ok {
		return nil, errors.New("match not found")
	}
	if m.State != gameplay.MatchLive {
		return nil, errors.New("match is not live")
	}
	if g.RoundID != m.RoundID {
		return nil, errors.New("round mismatch")
	}
	if m.PendingAdvance {
		// The round is already resolved; ignore guesses between resolution and
		// the next round actually advancing.
		return m.snapshot(), nil
	}
	p, exists := m.Players[g.UserID]
	if !exists {
		return nil, errors.New("player not in match")
	}
	now := e.now()
	if e.roundExpired(m, now) {
		e.resolveRound(m)
		return m.snapshot(), nil
	}
	if now.Before(m.RoundStartedAt.Add(roundIntro)) {
		return m.snapshot(), nil
	}
	if existing, ok := m.Guesses[g.UserID]; ok && existing.Finalized {
		return m.snapshot(), nil
	}
	guess := Guess{Lat: g.Lat, Lng: g.Lng, Finalized: g.Finalize, Ts: now}
	m.Guesses[g.UserID] = guess
	p.LastGuessLat = g.Lat
	p.LastGuessLng = g.Lng
	p.HasGuess = true
	p.Finalized = g.Finalize
	p.Disconnected = false
	p.DisconnectDue = 0
	m.LastActivity = now
	m.EventSeq++
	if g.Finalize {
		pressureMS := m.Config.PressureTimeLimitMS
		if m.Config.RoundTimerMode == gameplay.RoundTimerPressure && pressureMS <= 0 {
			pressureMS = gameplay.DefaultPressureTimeMS
		}
		if pressureMS > 0 {
			pressureDeadline := now.Add(time.Duration(pressureMS) * time.Millisecond)
			if m.RoundDeadline.IsZero() || pressureDeadline.Before(m.RoundDeadline) {
				m.RoundDeadline = pressureDeadline
			}
		}
	}
	allFinal := true
	for id := range m.Players {
		gu, ok := m.Guesses[id]
		if !ok || !gu.Finalized {
			allFinal = false
			break
		}
	}
	if allFinal {
		e.resolveRound(m)
	}
	return m.snapshot(), nil
}

func (e *Engine) Tick() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	now := e.now()
	changed := []string{}
	for _, m := range e.matches {
		if m.State != gameplay.MatchLive {
			continue
		}
		beforeSeq := m.EventSeq
		if m.PendingAdvance {
			if now.After(m.IntermissionUntil) {
				e.advanceRound(m)
			}
			if m.EventSeq != beforeSeq {
				changed = append(changed, m.ID)
			}
			continue
		}
		if !m.RoundLiveAnnounced && !now.Before(m.RoundStartedAt.Add(roundIntro)) {
			m.RoundLiveAnnounced = true
			m.EventSeq++
		}
		if e.roundExpired(m, now) {
			e.resolveRound(m)
		}
		if m.Rated {
			allDisconnected := true
			maxDue := int64(0)
			for _, p := range m.Players {
				if !p.Disconnected {
					allDisconnected = false
				}
				if p.DisconnectDue > maxDue {
					maxDue = p.DisconnectDue
				}
				if p.Disconnected && p.DisconnectDue > 0 && now.UnixMilli() > p.DisconnectDue {
					p.HP = 0
					m.State = gameplay.MatchEnded
					m.LastActivity = now
					m.EventSeq++
				}
			}
			if m.State != gameplay.MatchLive {
				if m.EventSeq != beforeSeq {
					changed = append(changed, m.ID)
				}
				continue
			}
			if allDisconnected && maxDue > 0 && now.UnixMilli() > maxDue {
				for _, p := range m.Players {
					p.HP = 0
				}
				m.State = gameplay.MatchEnded
				m.LastActivity = now
				m.EventSeq++
			} else if allDisconnected && !m.LastActivity.IsZero() && now.Sub(m.LastActivity) > staleGrace {
				for _, p := range m.Players {
					p.HP = 0
				}
				m.State = gameplay.MatchEnded
				m.EventSeq++
			}
		}
		if m.EventSeq != beforeSeq {
			changed = append(changed, m.ID)
		}
	}
	return changed
}

func (e *Engine) MarkDisconnected(matchID, userID string) (*gameplay.MatchSnapshot, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	m, ok := e.matches[matchID]
	if !ok {
		return nil, errors.New("match not found")
	}
	p, ok := m.Players[userID]
	if !ok {
		return nil, errors.New("player not in match")
	}
	p.Disconnected = true
	if !m.Rated {
		p.DisconnectDue = 0
	} else {
		p.DisconnectDue = e.now().Add(disconnectGrace).UnixMilli()
	}
	m.LastActivity = e.now()
	m.EventSeq++
	return m.snapshot(), nil
}

func (e *Engine) MarkResumed(matchID, userID string) (*gameplay.MatchSnapshot, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	m, ok := e.matches[matchID]
	if !ok {
		return nil, errors.New("match not found")
	}
	p, ok := m.Players[userID]
	if !ok {
		return nil, errors.New("player not in match")
	}
	p.Disconnected = false
	p.DisconnectDue = 0
	m.LastActivity = e.now()
	m.EventSeq++
	return m.snapshot(), nil
}

func (e *Engine) Forfeit(matchID, userID string) (*gameplay.MatchSnapshot, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	m, ok := e.matches[matchID]
	if !ok {
		return nil, errors.New("match not found")
	}
	if m.State != gameplay.MatchLive {
		return m.snapshot(), nil
	}
	p, ok := m.Players[userID]
	if !ok {
		return nil, errors.New("player not in match")
	}
	p.HP = 0
	p.Finalized = false
	p.Disconnected = false
	p.DisconnectDue = 0
	m.PendingAdvance = false
	m.IntermissionUntil = time.Time{}
	m.State = gameplay.MatchEnded
	m.LastActivity = e.now()
	m.EventSeq++
	return m.snapshot(), nil
}

// Abandon ends a match nobody plays any more as a draw: every player's HP goes to zero.
func (e *Engine) Abandon(matchID string) (*gameplay.MatchSnapshot, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	m, ok := e.matches[matchID]
	if !ok {
		return nil, errors.New("match not found")
	}
	if m.State == gameplay.MatchLive {
		for _, p := range m.Players {
			p.HP = 0
			p.Finalized = false
		}
		m.PendingAdvance = false
		m.IntermissionUntil = time.Time{}
		m.State = gameplay.MatchEnded
		m.LastActivity = e.now()
		m.EventSeq++
	}
	return m.snapshot(), nil
}

// Remove forgets a match, once it is recorded or no longer this engine's.
func (e *Engine) Remove(matchID string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.matches, matchID)
}

func (e *Engine) resolveRound(m *Match) {
	if m.State != gameplay.MatchLive {
		return
	}
	loc := m.CurrentLocation
	result := &gameplay.RoundResult{
		RoundID:        m.RoundID,
		RoundNumber:    m.CurrentIndex + 1,
		ActualLocation: loc,
		Players:        map[string]gameplay.RoundPlayerResult{},
	}
	userIDs := make([]string, 0, len(m.Players))
	for userID, p := range m.Players {
		userIDs = append(userIDs, userID)
		g, ok := m.Guesses[userID]
		if !ok {
			g = Guess{Lat: 0, Lng: 0}
		}
		dist := maxDistanceKm
		if ok {
			dist = gameplay.HaversineKm(loc.Lat, loc.Lng, g.Lat, g.Lng)
		}
		if dist > maxDistanceKm {
			dist = maxDistanceKm
		}
		result.Players[userID] = gameplay.RoundPlayerResult{
			UserID:       userID,
			Lat:          g.Lat,
			Lng:          g.Lng,
			DistanceKm:   dist,
			Score:        gameplay.RoundScore(dist),
			HPAfterRound: p.HP,
			GuessUnixMS:  guessUnixMS(g),
			GuessMS:      guessMS(g, m.RoundStartedAt.Add(roundIntro)),
		}
		p.Finalized = false
	}
	switch m.Mode {
	case matchkind.ModeDuel:
		resolveDuelDamage(m, result, userIDs)
	case matchkind.ModeTeamDuel:
		resolveTeamDuelDamage(m, result)
	case matchkind.ModeFreeForAll:
		for userID, playerResult := range result.Players {
			p := m.Players[userID]
			p.TotalScore += playerResult.Score
			playerResult.HPAfterRound = 0
			result.Players[userID] = playerResult
		}
	}
	if m.Mode != matchkind.ModeFreeForAll {
		for _, p := range m.Players {
			if p.HP <= 0 {
				p.HP = 0
				m.State = gameplay.MatchEnded
			}
		}
	}
	if m.Mode == matchkind.ModeFreeForAll && result.RoundNumber >= ffaRounds {
		m.State = gameplay.MatchEnded
	}
	if result.RoundNumber >= maxRounds {
		m.State = gameplay.MatchEnded
	}
	m.LastRoundResult = result
	m.RoundResults = append(m.RoundResults, result)
	m.Guesses = map[string]Guess{}
	m.LastActivity = e.now()
	m.EventSeq++
	if m.State == gameplay.MatchEnded {
		return
	}
	m.PendingAdvance = true
	m.IntermissionUntil = e.now().Add(resultDuration)
	m.LastActivity = e.now()
	m.EventSeq++
}

func resolveDuelDamage(m *Match, result *gameplay.RoundResult, userIDs []string) {
	if len(userIDs) == 2 {
		a := result.Players[userIDs[0]]
		b := result.Players[userIDs[1]]
		multiplier := roundDamageMultiplier(result.RoundNumber)
		if m.Config.MultiplierMode == gameplay.MultiplierIndividual {
			if a.Score > b.Score {
				multiplier = playerDamageMultiplier(m.Players[userIDs[0]])
			} else if b.Score > a.Score {
				multiplier = playerDamageMultiplier(m.Players[userIDs[1]])
			}
		}
		result.DamageMultiplier = multiplier
		damage := int(math.Round(float64(absInt(a.Score-b.Score)) * multiplier))
		switch {
		case a.Score > b.Score:
			a.DamageDealt = damage
			b.DamageTaken = damage
			p := m.Players[userIDs[1]]
			p.HP -= damage
			if p.HP < 0 {
				p.HP = 0
			}
			raiseIndividualMultiplier(m, userIDs[0])
		case b.Score > a.Score:
			b.DamageDealt = damage
			a.DamageTaken = damage
			p := m.Players[userIDs[0]]
			p.HP -= damage
			if p.HP < 0 {
				p.HP = 0
			}
			raiseIndividualMultiplier(m, userIDs[1])
		}
		a.HPAfterRound = m.Players[userIDs[0]].HP
		b.HPAfterRound = m.Players[userIDs[1]].HP
		result.Players[userIDs[0]] = a
		result.Players[userIDs[1]] = b
	}
}

func resolveTeamDuelDamage(m *Match, result *gameplay.RoundResult) {
	if len(m.Teams) != 2 {
		m.State = gameplay.MatchEnded
		return
	}
	result.Teams = map[string]gameplay.RoundTeamResult{}
	for teamID, team := range m.Teams {
		best := gameplay.RoundTeamResult{TeamID: teamID, DistanceKm: maxDistanceKm, HPAfterRound: team.HP}
		for _, userID := range team.Players {
			playerResult, ok := result.Players[userID]
			if !ok {
				continue
			}
			if best.RepresentativeUserID == "" || playerResult.Score > best.Score {
				best.RepresentativeUserID = userID
				best.Lat = playerResult.Lat
				best.Lng = playerResult.Lng
				best.DistanceKm = playerResult.DistanceKm
				best.Score = playerResult.Score
			}
		}
		result.Teams[teamID] = best
	}
	a := result.Teams["a"]
	b := result.Teams["b"]
	multiplier := roundDamageMultiplier(result.RoundNumber)
	if m.Config.MultiplierMode == gameplay.MultiplierIndividual {
		if a.Score > b.Score {
			multiplier = playerDamageMultiplier(m.Players[a.RepresentativeUserID])
		} else if b.Score > a.Score {
			multiplier = playerDamageMultiplier(m.Players[b.RepresentativeUserID])
		}
	}
	result.DamageMultiplier = multiplier
	damage := int(math.Round(float64(absInt(a.Score-b.Score)) * multiplier))
	switch {
	case a.Score > b.Score:
		a.DamageDealt = damage
		b.DamageTaken = damage
		m.Teams["b"].HP -= damage
		raiseIndividualMultiplier(m, a.RepresentativeUserID)
	case b.Score > a.Score:
		b.DamageDealt = damage
		a.DamageTaken = damage
		m.Teams["a"].HP -= damage
		raiseIndividualMultiplier(m, b.RepresentativeUserID)
	}
	for teamID, team := range m.Teams {
		if team.HP < 0 {
			team.HP = 0
		}
		for _, userID := range team.Players {
			if p := m.Players[userID]; p != nil {
				p.HP = team.HP
			}
		}
		teamResult := result.Teams[teamID]
		teamResult.HPAfterRound = team.HP
		result.Teams[teamID] = teamResult
		if team.HP <= 0 {
			m.State = gameplay.MatchEnded
		}
	}
	a.HPAfterRound = m.Teams["a"].HP
	b.HPAfterRound = m.Teams["b"].HP
	result.Teams["a"] = a
	result.Teams["b"] = b
}

func playerDamageMultiplier(player *gameplay.PlayerState) float64 {
	if player == nil || player.DamageMultiplier < 1 {
		return 1
	}
	return player.DamageMultiplier
}

func raiseIndividualMultiplier(m *Match, userID string) {
	if m.Config.MultiplierMode != gameplay.MultiplierIndividual {
		return
	}
	if player := m.Players[userID]; player != nil {
		player.DamageMultiplier = playerDamageMultiplier(player) + 0.5
	}
}

func guessUnixMS(g Guess) int64 {
	if g.Ts.IsZero() {
		return 0
	}
	return g.Ts.UnixMilli()
}

func guessMS(g Guess, roundLiveAt time.Time) int64 {
	if g.Ts.IsZero() || roundLiveAt.IsZero() {
		return 0
	}
	ms := g.Ts.Sub(roundLiveAt).Milliseconds()
	if ms < 0 {
		return 0
	}
	return ms
}

func (e *Engine) advanceRound(m *Match) {
	if !m.PendingAdvance {
		return
	}
	nextIndex := m.CurrentIndex + 1
	nextLoc, err := e.roundProvider(m.ID, nextIndex)
	if err != nil {
		m.State = gameplay.MatchEnded
		m.PendingAdvance = false
		m.IntermissionUntil = time.Time{}
		m.LastActivity = e.now()
		m.EventSeq++
		return
	}
	m.CurrentIndex = nextIndex
	m.CurrentLocation = nextLoc
	nextRound := m.CurrentIndex + 1
	m.RoundID = roundID(m.ID, nextRound)
	m.RoundStartedAt = e.now()
	m.RoundDeadline = time.Time{}
	m.RoundLiveAnnounced = false
	e.startRoundTimer(m)
	for _, p := range m.Players {
		p.Finalized = false
		p.LastGuessLat = 0
		p.LastGuessLng = 0
		p.HasGuess = false
	}
	m.PendingAdvance = false
	m.IntermissionUntil = time.Time{}
	m.LastActivity = e.now()
	m.EventSeq++
}

func (e *Engine) startRoundTimer(m *Match) {
	m.Config = gameplay.NormalizeMatchConfig(m.Config)
	if m.Config.RoundTimerMode != gameplay.RoundTimerFixed {
		return
	}
	m.RoundDeadline = m.RoundStartedAt.Add(roundIntro).Add(time.Duration(m.Config.RoundTimeLimitMS) * time.Millisecond)
}

func (m *Match) snapshot() *gameplay.MatchSnapshot {
	now := m.now()
	phase := gameplay.PhaseLive
	roundPhase := gameplay.RoundPhaseLive
	phaseStartedAt := m.RoundStartedAt
	phaseEndsAt := m.RoundDeadline
	if m.State == gameplay.MatchEnded {
		phase = gameplay.PhaseEnded
		roundPhase = gameplay.RoundPhaseEnded
		phaseStartedAt = now
		phaseEndsAt = now
	} else if m.PendingAdvance && now.Before(m.IntermissionUntil) {
		phase = gameplay.PhaseRoundResult
		roundPhase = gameplay.RoundPhaseResult
		phaseStartedAt = m.IntermissionUntil.Add(-resultDuration)
		phaseEndsAt = m.IntermissionUntil
	} else if m.PendingAdvance {
		roundPhase = gameplay.RoundPhaseTransition
		phaseStartedAt = m.IntermissionUntil
		phaseEndsAt = now
	} else if now.Before(m.RoundStartedAt.Add(roundIntro)) {
		roundPhase = gameplay.RoundPhaseIntro
		phaseStartedAt = m.RoundStartedAt
		phaseEndsAt = m.RoundStartedAt.Add(roundIntro)
	} else if m.RoundDeadline.IsZero() {
		roundPhase = gameplay.RoundPhaseLive
		phaseStartedAt = m.RoundStartedAt.Add(roundIntro)
		phaseEndsAt = time.Time{}
	} else {
		roundPhase = gameplay.RoundPhaseLive
		phaseStartedAt = m.RoundStartedAt.Add(roundIntro)
		phaseEndsAt = m.RoundDeadline
	}
	var current *gameplay.RoundState
	if phase == gameplay.PhaseLive && m.State == gameplay.MatchLive {
		current = &gameplay.RoundState{
			RoundID:       m.RoundID,
			RoundNumber:   m.CurrentIndex + 1,
			RoundDeadline: m.RoundDeadline,
			TimerStarted:  !m.RoundDeadline.IsZero(),
			Location:      m.CurrentLocation,
		}
	}
	players := map[string]gameplay.PlayerState{}
	for id, p := range m.Players {
		if p.DamageMultiplier < 1 {
			p.DamageMultiplier = 1
		}
		players[id] = *p
	}
	teams := map[string]gameplay.TeamState{}
	for id, team := range m.Teams {
		teams[id] = *team
		teams[id] = gameplay.TeamState{
			TeamID:  team.TeamID,
			Name:    team.Name,
			HP:      team.HP,
			Players: append([]string(nil), team.Players...),
		}
	}
	if len(teams) == 0 {
		teams = nil
	}
	msLeft := int64(0)
	if phase == gameplay.PhaseRoundResult {
		msLeft = maxInt64(0, m.IntermissionUntil.Sub(now).Milliseconds())
	} else if phase == gameplay.PhaseLive && roundPhase == gameplay.RoundPhaseIntro {
		msLeft = maxInt64(0, m.RoundStartedAt.Add(roundIntro).Sub(now).Milliseconds())
	} else if phase == gameplay.PhaseLive && !m.RoundDeadline.IsZero() {
		msLeft = maxInt64(0, m.RoundDeadline.Sub(now).Milliseconds())
	}
	return &gameplay.MatchSnapshot{
		MatchID:         m.ID,
		Mode:            m.Mode,
		Kind:            m.Kind,
		SeasonID:        m.SeasonID,
		Config:          gameplay.NormalizeMatchConfig(m.Config),
		State:           m.State,
		Phase:           phase,
		RoundPhase:      roundPhase,
		PhaseStartedAt:  phaseStartedAt.UnixMilli(),
		PhaseEndsAt:     unixMilliOrZero(phaseEndsAt),
		CurrentRound:    current,
		LastRoundResult: m.LastRoundResult,
		RoundResults:    append([]*gameplay.RoundResult(nil), m.RoundResults...),
		RoundMSLeft:     msLeft,
		Players:         players,
		Teams:           teams,
		RatingPreview:   copyRatingPreview(m.RatingPreview),
		EventSequence:   m.EventSeq,
		ServerUnixMS:    m.now().UnixMilli(),
		GraceWindowSec:  int(disconnectGrace.Seconds()),
	}
}

func ratingPreview(playerIDs []string, players map[string]*gameplay.PlayerState, now time.Time) map[string]gameplay.RatingDeltaPreview {
	if len(playerIDs) != 2 {
		return nil
	}
	p1 := players[playerIDs[0]]
	p2 := players[playerIDs[1]]
	if p1 == nil || p2 == nil || (p1.IsGuest && p2.IsGuest) {
		return nil
	}
	p1State := rating.State{MMR: p1.MMR, RD: p1.RatingRD, UpdatedAt: now}
	p2State := rating.State{MMR: p2.MMR, RD: p2.RatingRD, UpdatedAt: now}
	p1Win, p2Lose := rating.CalculateDuelUpdates(p1State, p2State, "p1", now)
	p1Lose, p2Win := rating.CalculateDuelUpdates(p1State, p2State, "p2", now)
	p1Draw, p2Draw := rating.CalculateDuelUpdates(p1State, p2State, "", now)

	preview := map[string]gameplay.RatingDeltaPreview{}
	if !p1.IsGuest {
		preview[p1.UserID] = gameplay.RatingDeltaPreview{Win: p1Win.Delta, Lose: p1Lose.Delta, Draw: p1Draw.Delta}
	}
	if !p2.IsGuest {
		preview[p2.UserID] = gameplay.RatingDeltaPreview{Win: p2Win.Delta, Lose: p2Lose.Delta, Draw: p2Draw.Delta}
	}
	return preview
}

func copyRatingPreview(in map[string]gameplay.RatingDeltaPreview) map[string]gameplay.RatingDeltaPreview {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]gameplay.RatingDeltaPreview, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func buildTeams(mode matchkind.Mode, playerIDs []string, players map[string]*gameplay.PlayerState) map[string]*gameplay.TeamState {
	if mode != matchkind.ModeTeamDuel {
		return nil
	}
	teams := map[string]*gameplay.TeamState{
		"a": {TeamID: "a", Name: "Team Red", HP: startingHP, Players: []string{}},
		"b": {TeamID: "b", Name: "Team Blue", HP: startingHP, Players: []string{}},
	}
	for index, userID := range playerIDs {
		player := players[userID]
		if player == nil {
			continue
		}
		teamID := normalizeTeamID(player.TeamID)
		if player.TeamID == "" {
			if index%2 == 1 {
				teamID = "b"
			}
			player.TeamID = teamID
		}
		teams[teamID].Players = append(teams[teamID].Players, userID)
		player.HP = teams[teamID].HP
	}
	for teamID, team := range teams {
		if len(team.Players) == 0 {
			delete(teams, teamID)
		}
	}
	return teams
}

func normalizeTeamID(teamID string) string {
	if teamID == "b" {
		return "b"
	}
	return "a"
}

func (e *Engine) roundExpired(m *Match, now time.Time) bool {
	if !m.RoundDeadline.IsZero() {
		return now.After(m.RoundDeadline)
	}
	// No fixed deadline (time limit "none", or pressure mode before the first
	// finalize). Fall back to an idle watchdog so genuinely abandoned rounds
	// don't pin a gameplay node forever. Measure the idle window from the last
	// player activity (pin placement/move, (re)connect) rather than the round
	// start, so an actively-played unlimited round is never force-resolved.
	liveAt := m.RoundStartedAt.Add(roundIntro)
	idleSince := m.LastActivity
	if idleSince.Before(liveAt) {
		idleSince = liveAt
	}
	return now.After(idleSince.Add(roundIdleCap))
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

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func unixMilliOrZero(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.UnixMilli()
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func roundScore(distanceKm float64) int {
	return gameplay.RoundScore(distanceKm)
}

func roundDamageMultiplier(roundNumber int) float64 {
	if roundNumber <= 2 {
		return 1.0
	}
	return 1.0 + (0.5 * float64(roundNumber-2))
}
