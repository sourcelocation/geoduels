package matches

import (
	"time"

	"geoduels/pkg/contracts"
)

type MatchHistorySummary struct {
	MatchID             string    `json:"matchId"`
	Mode                string    `json:"mode"`
	StartedAt           time.Time `json:"startedAt"`
	EndedAt             time.Time `json:"endedAt"`
	WinnerUserID        string    `json:"winnerUserId,omitempty"`
	Outcome             string    `json:"outcome"`
	Ranked              bool      `json:"ranked"`
	RatingDelta         int       `json:"ratingDelta,omitempty"`
	TotalScore          int       `json:"totalScore,omitempty"`
	OpponentUserID      string    `json:"opponentUserId,omitempty"`
	OpponentDisplayName string    `json:"opponentDisplayName,omitempty"`
}

type MatchHistoryPage struct {
	Matches     []MatchHistorySummary
	HasMore     bool
	NextEndedAt time.Time
	NextMatchID string
}

type MatchSessionUpsert = contracts.MatchSessionUpsert
