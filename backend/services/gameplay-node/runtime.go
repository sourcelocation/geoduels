package main

import (
	"errors"
	"sync"

	"geoduels/pkg/duel"
	"geoduels/pkg/gameplay"
	"geoduels/pkg/matchkind"
	"geoduels/pkg/singleplayer"
)

type roundPlanRegistry struct {
	mu    sync.RWMutex
	plans map[string][]gameplay.LocationPoint
}

func newRoundPlanRegistry() *roundPlanRegistry {
	return &roundPlanRegistry{plans: map[string][]gameplay.LocationPoint{}}
}

func (r *roundPlanRegistry) Set(matchID string, rounds []gameplay.LocationPoint) {
	r.mu.Lock()
	r.plans[matchID] = rounds
	r.mu.Unlock()
}

func (r *roundPlanRegistry) Delete(matchID string) {
	r.mu.Lock()
	delete(r.plans, matchID)
	r.mu.Unlock()
}

func (r *roundPlanRegistry) Get(matchID string, roundIndex int) (gameplay.LocationPoint, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	points := r.plans[matchID]
	if roundIndex < 0 || roundIndex >= len(points) {
		return gameplay.LocationPoint{}, errors.New("round plan exhausted")
	}
	return points[roundIndex], nil
}

// newMatch is what an engine needs to create a match.
type newMatch struct {
	MatchID  string
	Kind     matchkind.Kind
	Players  []string
	Profiles map[string]gameplay.PlayerProfile
	Teams    map[string]string
	SeasonID string
	Config   gameplay.MatchConfig
}

// gameplayRuntime is one engine, as the node drives it.
type gameplayRuntime interface {
	CreateMatch(m newMatch) error
	GetSnapshot(matchID string) (*gameplay.MatchSnapshot, error)
	SubmitGuess(g gameplay.Guess) (*gameplay.MatchSnapshot, error)
	AdvanceRound(matchID, userID string) (*gameplay.MatchSnapshot, error)
	Forfeit(matchID, userID string) (*gameplay.MatchSnapshot, error)
	// Abandon ends a match nobody plays any more.
	Abandon(matchID, userID string) (*gameplay.MatchSnapshot, error)
	MarkDisconnected(matchID, userID string) (*gameplay.MatchSnapshot, error)
	MarkResumed(matchID, userID string) (*gameplay.MatchSnapshot, error)
	Remove(matchID string)
	Tick() []string
}

// versusRuntime runs every kind with opponents: duels, team duels and free-for-alls.
type versusRuntime struct {
	engine *duel.Engine
}

func (r versusRuntime) CreateMatch(m newMatch) error {
	_, err := r.engine.CreateMatchWithOptions(m.MatchID, m.Players, m.Profiles, duel.MatchOptions{
		Kind: m.Kind, SeasonID: m.SeasonID, Config: gameplay.NormalizeMatchConfig(m.Config), Teams: m.Teams,
	})
	return err
}

func (r versusRuntime) GetSnapshot(matchID string) (*gameplay.MatchSnapshot, error) {
	return r.engine.GetSnapshot(matchID)
}

func (r versusRuntime) SubmitGuess(g gameplay.Guess) (*gameplay.MatchSnapshot, error) {
	return r.engine.SubmitGuess(g)
}

func (r versusRuntime) AdvanceRound(matchID, userID string) (*gameplay.MatchSnapshot, error) {
	return nil, errors.New("advance round is not supported for duel")
}

func (r versusRuntime) Forfeit(matchID, userID string) (*gameplay.MatchSnapshot, error) {
	return r.engine.Forfeit(matchID, userID)
}

func (r versusRuntime) Abandon(matchID, _ string) (*gameplay.MatchSnapshot, error) {
	return r.engine.Abandon(matchID)
}

func (r versusRuntime) MarkDisconnected(matchID, userID string) (*gameplay.MatchSnapshot, error) {
	return r.engine.MarkDisconnected(matchID, userID)
}

func (r versusRuntime) MarkResumed(matchID, userID string) (*gameplay.MatchSnapshot, error) {
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

func (r soloRuntime) GetSnapshot(matchID string) (*gameplay.MatchSnapshot, error) {
	return r.engine.GetSnapshot(matchID)
}

func (r soloRuntime) SubmitGuess(g gameplay.Guess) (*gameplay.MatchSnapshot, error) {
	return r.engine.SubmitGuess(g)
}

func (r soloRuntime) AdvanceRound(matchID, userID string) (*gameplay.MatchSnapshot, error) {
	return r.engine.AdvanceRound(matchID, userID)
}

func (r soloRuntime) Forfeit(matchID, userID string) (*gameplay.MatchSnapshot, error) {
	return r.engine.Forfeit(matchID, userID)
}

func (r soloRuntime) Abandon(matchID, userID string) (*gameplay.MatchSnapshot, error) {
	return r.engine.Forfeit(matchID, userID)
}

func (r soloRuntime) MarkDisconnected(matchID, userID string) (*gameplay.MatchSnapshot, error) {
	return r.engine.MarkDisconnected(matchID, userID)
}

func (r soloRuntime) MarkResumed(matchID, userID string) (*gameplay.MatchSnapshot, error) {
	return r.engine.MarkResumed(matchID, userID)
}

func (r soloRuntime) Remove(matchID string) { r.engine.Remove(matchID) }

func (r soloRuntime) Tick() []string { return nil }
