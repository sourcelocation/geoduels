package main

import (
	"errors"
	"sync"

	"geoduels/pkg/contracts"
	"geoduels/pkg/duel"
	"geoduels/pkg/singleplayer"
)

type roundPlanRegistry struct {
	mu    sync.RWMutex
	plans map[string][]contracts.LocationPoint
}

func newRoundPlanRegistry() *roundPlanRegistry {
	return &roundPlanRegistry{plans: map[string][]contracts.LocationPoint{}}
}

func (r *roundPlanRegistry) Set(matchID string, rounds []contracts.LocationPoint) {
	r.mu.Lock()
	r.plans[matchID] = rounds
	r.mu.Unlock()
}

func (r *roundPlanRegistry) Delete(matchID string) {
	r.mu.Lock()
	delete(r.plans, matchID)
	r.mu.Unlock()
}

func (r *roundPlanRegistry) Get(matchID string, roundIndex int) (contracts.LocationPoint, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	points := r.plans[matchID]
	if roundIndex < 0 || roundIndex >= len(points) {
		return contracts.LocationPoint{}, errors.New("round plan exhausted")
	}
	return points[roundIndex], nil
}

// newMatch is what an engine needs to create a match.
type newMatch struct {
	MatchID  string
	Kind     contracts.MatchKind
	Players  []string
	Profiles map[string]contracts.PlayerProfile
	Teams    map[string]string
	SeasonID string
	Config   contracts.MatchConfig
}

// gameplayRuntime is one engine, as the node drives it.
type gameplayRuntime interface {
	CreateMatch(m newMatch) error
	GetSnapshot(matchID string) (*contracts.MatchSnapshot, error)
	SubmitGuess(g contracts.GuessPayload) (*contracts.MatchSnapshot, error)
	AdvanceRound(matchID, userID string) (*contracts.MatchSnapshot, error)
	Forfeit(matchID, userID string) (*contracts.MatchSnapshot, error)
	// Abandon ends a match nobody plays any more.
	Abandon(matchID, userID string) (*contracts.MatchSnapshot, error)
	MarkDisconnected(matchID, userID string) (*contracts.MatchSnapshot, error)
	MarkResumed(matchID, userID string) (*contracts.MatchSnapshot, error)
	Remove(matchID string)
	Tick() []string
}

// versusRuntime runs every kind with opponents: duels, team duels and free-for-alls.
type versusRuntime struct {
	engine *duel.Engine
}

func (r versusRuntime) CreateMatch(m newMatch) error {
	_, err := r.engine.CreateMatchWithOptions(m.MatchID, m.Players, m.Profiles, duel.MatchOptions{
		Kind: m.Kind, SeasonID: m.SeasonID, Config: contracts.NormalizeMatchConfig(m.Config), Teams: m.Teams,
	})
	return err
}

func (r versusRuntime) GetSnapshot(matchID string) (*contracts.MatchSnapshot, error) {
	return r.engine.GetSnapshot(matchID)
}

func (r versusRuntime) SubmitGuess(g contracts.GuessPayload) (*contracts.MatchSnapshot, error) {
	return r.engine.SubmitGuess(g)
}

func (r versusRuntime) AdvanceRound(matchID, userID string) (*contracts.MatchSnapshot, error) {
	return nil, errors.New("advance round is not supported for duel")
}

func (r versusRuntime) Forfeit(matchID, userID string) (*contracts.MatchSnapshot, error) {
	return r.engine.Forfeit(matchID, userID)
}

func (r versusRuntime) Abandon(matchID, _ string) (*contracts.MatchSnapshot, error) {
	return r.engine.Abandon(matchID)
}

func (r versusRuntime) MarkDisconnected(matchID, userID string) (*contracts.MatchSnapshot, error) {
	return r.engine.MarkDisconnected(matchID, userID)
}

func (r versusRuntime) MarkResumed(matchID, userID string) (*contracts.MatchSnapshot, error) {
	return r.engine.MarkResumed(matchID, userID)
}

func (r versusRuntime) Remove(matchID string) { r.engine.Remove(matchID) }

func (r versusRuntime) Tick() []string {
	return r.engine.Tick()
}

type soloRuntime struct {
	engine *singleplayer.Engine
}

func (r soloRuntime) CreateMatch(m newMatch) error {
	_, err := r.engine.CreateMatchWithConfig(m.MatchID, m.Players, m.Profiles, m.Config)
	return err
}

func (r soloRuntime) GetSnapshot(matchID string) (*contracts.MatchSnapshot, error) {
	return r.engine.GetSnapshot(matchID)
}

func (r soloRuntime) SubmitGuess(g contracts.GuessPayload) (*contracts.MatchSnapshot, error) {
	return r.engine.SubmitGuess(g)
}

func (r soloRuntime) AdvanceRound(matchID, userID string) (*contracts.MatchSnapshot, error) {
	return r.engine.AdvanceRound(matchID, userID)
}

func (r soloRuntime) Forfeit(matchID, userID string) (*contracts.MatchSnapshot, error) {
	return r.engine.Forfeit(matchID, userID)
}

func (r soloRuntime) Abandon(matchID, userID string) (*contracts.MatchSnapshot, error) {
	return r.engine.Forfeit(matchID, userID)
}

func (r soloRuntime) MarkDisconnected(matchID, userID string) (*contracts.MatchSnapshot, error) {
	return r.engine.MarkDisconnected(matchID, userID)
}

func (r soloRuntime) MarkResumed(matchID, userID string) (*contracts.MatchSnapshot, error) {
	return r.engine.MarkResumed(matchID, userID)
}

func (r soloRuntime) Remove(matchID string) { r.engine.Remove(matchID) }

func (r soloRuntime) Tick() []string { return nil }
