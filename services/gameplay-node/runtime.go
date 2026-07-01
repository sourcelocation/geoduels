package main

import (
	"errors"
	"sync"

	"geoduels/pkg/contracts"
	"geoduels/pkg/duel"
	"geoduels/pkg/singleplayer"
)

type matchConfigRegistry struct {
	mu      sync.RWMutex
	configs map[string]contracts.MatchConfig
}

type roundPlanRegistry struct {
	mu    sync.RWMutex
	plans map[string][]contracts.LocationPoint
}

func newRoundPlanRegistry() *roundPlanRegistry {
	return &roundPlanRegistry{plans: map[string][]contracts.LocationPoint{}}
}

func (r *roundPlanRegistry) Set(matchID string, rounds []contracts.PlannedRound) {
	points := make([]contracts.LocationPoint, len(rounds))
	for _, round := range rounds {
		if round.RoundIndex >= 0 && round.RoundIndex < len(points) {
			points[round.RoundIndex] = round.Location
		}
	}
	r.mu.Lock()
	r.plans[matchID] = points
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

func newMatchConfigRegistry() *matchConfigRegistry {
	return &matchConfigRegistry{configs: map[string]contracts.MatchConfig{}}
}

func (r *matchConfigRegistry) Set(matchID string, cfg contracts.MatchConfig) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.configs[matchID] = contracts.NormalizeMatchConfig(cfg)
}

func (r *matchConfigRegistry) Get(matchID string) contracts.MatchConfig {
	if r == nil {
		return contracts.NormalizeMatchConfig(contracts.MatchConfig{})
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return contracts.NormalizeMatchConfig(r.configs[matchID])
}

type gameplayRuntime interface {
	Mode() contracts.MatchMode
	CreateMatch(matchID string, playerIDs []string, profiles map[string]contracts.PlayerProfile, unranked bool, seasonID string, config contracts.MatchConfig, teams map[string]string) error
	GetSnapshot(matchID string) (*contracts.MatchSnapshot, error)
	SubmitGuess(g contracts.GuessPayload) (*contracts.MatchSnapshot, error)
	AdvanceRound(matchID, userID string) (*contracts.MatchSnapshot, error)
	Forfeit(matchID, userID string) (*contracts.MatchSnapshot, error)
	MarkDisconnected(matchID, userID string) (*contracts.MatchSnapshot, error)
	MarkResumed(matchID, userID string) (*contracts.MatchSnapshot, error)
	Tick() []string
}

type duelRuntime struct {
	mode    contracts.MatchMode
	engine  *duel.Engine
	configs *matchConfigRegistry
}

func (r duelRuntime) Mode() contracts.MatchMode {
	if r.mode == "" {
		return contracts.ModeDuel
	}
	return r.mode
}

func (r duelRuntime) CreateMatch(matchID string, playerIDs []string, profiles map[string]contracts.PlayerProfile, unranked bool, seasonID string, config contracts.MatchConfig, teams map[string]string) error {
	config = contracts.NormalizeMatchConfig(config)
	r.configs.Set(matchID, config)
	_, err := r.engine.CreateMatchWithOptions(matchID, playerIDs, profiles, duel.MatchOptions{Unranked: unranked, SeasonID: seasonID, Config: config, Mode: r.Mode(), Teams: teams})
	return err
}

func (r duelRuntime) GetSnapshot(matchID string) (*contracts.MatchSnapshot, error) {
	return r.engine.GetSnapshot(matchID)
}

func (r duelRuntime) SubmitGuess(g contracts.GuessPayload) (*contracts.MatchSnapshot, error) {
	return r.engine.SubmitGuess(g)
}

func (r duelRuntime) AdvanceRound(matchID, userID string) (*contracts.MatchSnapshot, error) {
	return nil, errors.New("advance round is not supported for duel")
}

func (r duelRuntime) Forfeit(matchID, userID string) (*contracts.MatchSnapshot, error) {
	return r.engine.Forfeit(matchID, userID)
}

func (r duelRuntime) MarkDisconnected(matchID, userID string) (*contracts.MatchSnapshot, error) {
	return r.engine.MarkDisconnected(matchID, userID)
}

func (r duelRuntime) MarkResumed(matchID, userID string) (*contracts.MatchSnapshot, error) {
	return r.engine.MarkResumed(matchID, userID)
}

func (r duelRuntime) Tick() []string {
	return r.engine.Tick()
}

type singleplayerRuntime struct {
	engine *singleplayer.Engine
}

func (r singleplayerRuntime) Mode() contracts.MatchMode { return contracts.ModeSingleplayer }

func (r singleplayerRuntime) CreateMatch(matchID string, playerIDs []string, profiles map[string]contracts.PlayerProfile, unranked bool, seasonID string, config contracts.MatchConfig, teams map[string]string) error {
	_, err := r.engine.CreateMatchWithConfig(matchID, playerIDs, profiles, config)
	return err
}

func (r singleplayerRuntime) GetSnapshot(matchID string) (*contracts.MatchSnapshot, error) {
	return r.engine.GetSnapshot(matchID)
}

func (r singleplayerRuntime) SubmitGuess(g contracts.GuessPayload) (*contracts.MatchSnapshot, error) {
	return r.engine.SubmitGuess(g)
}

func (r singleplayerRuntime) AdvanceRound(matchID, userID string) (*contracts.MatchSnapshot, error) {
	return r.engine.AdvanceRound(matchID, userID)
}

func (r singleplayerRuntime) Forfeit(matchID, userID string) (*contracts.MatchSnapshot, error) {
	return r.engine.Forfeit(matchID, userID)
}

func (r singleplayerRuntime) MarkDisconnected(matchID, userID string) (*contracts.MatchSnapshot, error) {
	return r.engine.MarkDisconnected(matchID, userID)
}

func (r singleplayerRuntime) MarkResumed(matchID, userID string) (*contracts.MatchSnapshot, error) {
	return r.engine.MarkResumed(matchID, userID)
}

func (r singleplayerRuntime) Tick() []string { return nil }
