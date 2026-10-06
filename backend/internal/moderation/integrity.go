package moderation

import (
	"context"
	"time"
)

// RiskSignal is one detector finding.
type RiskSignal struct {
	SubjectUserID    string         `json:"subjectUserId"`
	SignalType       string         `json:"signalType"`
	DetectorKey      string         `json:"detectorKey"`
	DetectorVersion  string         `json:"detectorVersion"`
	Severity         string         `json:"severity"`
	EvidenceStrength string         `json:"evidenceStrength"`
	ReasonCode       string         `json:"reasonCode"`
	RecommendedQueue bool           `json:"recommendedQueue"`
	Score            float64        `json:"score"`
	Payload          map[string]any `json:"payload,omitempty"`
	OccurredAt       time.Time      `json:"occurredAt"`
}

// RiskGuessEvent is one guess observation sent to the detector.
type RiskGuessEvent struct {
	MatchID     string    `json:"matchId"`
	RoundNumber int       `json:"roundNumber"`
	Score       int       `json:"score"`
	GuessMS     int       `json:"guessMs"`
	Evidence    float64   `json:"evidence"`
	OccurredAt  time.Time `json:"occurredAt"`
}

// RiskPlayer is per-player history sent to the detector.
type RiskPlayer struct {
	UserID        string           `json:"userId"`
	CurrentRating int              `json:"currentRating,omitempty"`
	RankedGames   int              `json:"rankedGames,omitempty"`
	Events        []RiskGuessEvent `json:"events"`
}

// RiskRequest is the detector analyze request.
type RiskRequest struct {
	RequestID    string       `json:"requestId,omitempty"`
	MatchID      string       `json:"matchId"`
	FactsVersion string       `json:"factsVersion"`
	GeneratedAt  time.Time    `json:"generatedAt"`
	Players      []RiskPlayer `json:"players"`
}

// RiskResponse is the detector analyze response.
type RiskResponse struct {
	DetectorVersion string       `json:"detectorVersion"`
	FactsVersion    string       `json:"factsVersion"`
	Signals         []RiskSignal `json:"signals"`
}

// RiskEngine analyzes match facts for integrity signals.
type RiskEngine interface {
	Analyze(ctx context.Context, req RiskRequest) (RiskResponse, error)
	Enabled() bool
}

// EvaluateMatch runs the integrity detector for a finished match and records
// its signals. It is dispatched by the job worker, not by HTTP.
func (s *Service) EvaluateMatch(ctx context.Context, matchID string) error {
	if s.risk == nil || !s.risk.Enabled() {
		return nil
	}
	store := s.store
	playerIDs, err := store.MatchPlayerIDs(ctx, matchID)
	if err != nil {
		return err
	}
	if len(playerIDs) == 0 {
		return nil
	}
	seasonID, err := store.ActiveSeasonID(ctx)
	if err != nil {
		return err
	}
	req := RiskRequest{MatchID: matchID, FactsVersion: "match-facts/2026-01", GeneratedAt: s.now(), Players: make([]RiskPlayer, 0, len(playerIDs))}
	for _, userID := range playerIDs {
		events, err := store.RecentGuessEvents(ctx, userID)
		if err != nil {
			return err
		}
		rating, rankedGames, err := store.PlayerContext(ctx, userID, seasonID)
		if err != nil {
			return err
		}
		req.Players = append(req.Players, RiskPlayer{UserID: userID, CurrentRating: rating, RankedGames: rankedGames, Events: events})
	}
	resp, err := s.risk.Analyze(ctx, req)
	if err != nil {
		return err
	}
	return s.store.WithinTx(ctx, func(store Store) error {
		for _, signal := range resp.Signals {
			if signal.SubjectUserID == "" {
				continue
			}
			if err := store.RecordSignal(ctx, matchID, normalizeRiskSignal(signal), s.now()); err != nil {
				return err
			}
		}
		return nil
	})
}
