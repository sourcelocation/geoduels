package main

import (
	"time"

	"geoduels/pkg/observability"
)

const rankedSeasonResetCheckInterval = time.Minute

func (a *api) runRankedSeasonResetLoop() {
	a.runDueRankedSeasonReset()
	ticker := time.NewTicker(rankedSeasonResetCheckInterval)
	defer ticker.Stop()
	for range ticker.C {
		a.runDueRankedSeasonReset()
	}
}

func (a *api) runDueRankedSeasonReset() {
	result, reset, err := a.store.RunDueRankedSeasonReset(time.Now().UTC())
	if err != nil {
		observability.Log("warn", "ranked season reset check failed", map[string]any{"error": err.Error()})
		return
	}
	if !reset {
		return
	}
	observability.Log("info", "ranked season reset completed", map[string]any{
		"previousSeasonId": result.PreviousSeasonID,
		"activeSeasonId":   result.ActiveSeasonID,
		"playersSeeded":    result.PlayersSeeded,
		"resetAt":          result.ResetAt,
	})
}
