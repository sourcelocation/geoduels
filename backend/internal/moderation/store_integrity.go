package moderation

import (
	"context"
	"encoding/json"
	"time"

	"geoduels/internal/seasons"
	"geoduels/internal/storekit"
	db "geoduels/pkg/persistence/sqlc/db"
)

func (a *PGStore) ActiveSeasonID(ctx context.Context) (string, error) {
	if a.tx != nil {
		return seasons.ActiveSeasonIDTx(ctx, a.tx)
	}
	return storekit.ActiveSeasonID(ctx, a.pool)
}

func (a *PGStore) MatchPlayerIDs(ctx context.Context, matchID string) ([]string, error) {
	matchUUID, err := storekit.ProfileUUID(matchID)
	if err != nil {
		return nil, err
	}
	rows, err := a.q().ListMatchGuessPlayers(ctx, matchUUID)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, storekit.UUIDVal(row))
	}
	return ids, nil
}

func (a *PGStore) RecentGuessEvents(ctx context.Context, userID string) ([]RiskGuessEvent, error) {
	rows, err := a.q().ListRecentGuessEvents(ctx, storekit.MustUUID(userID))
	if err != nil {
		return nil, err
	}
	events := make([]RiskGuessEvent, 0, len(rows))
	for _, row := range rows {
		events = append(events, RiskGuessEvent{
			MatchID: storekit.UUIDVal(row.MatchID), RoundNumber: int(row.RoundNumber), Score: int(row.Score),
			GuessMS: int(row.GuessMs), Evidence: float64(row.Evidence), OccurredAt: row.OccurredAt.Time,
		})
	}
	return events, nil
}

func (a *PGStore) PlayerContext(ctx context.Context, userID, seasonID string) (int, int, error) {
	row, err := a.q().GetPlayerRiskContext(ctx, db.GetPlayerRiskContextParams{
		UserID: storekit.MustUUID(userID), Mode: db.GdMatchMode(modeDuel), SeasonID: seasonID, DefaultRating: initialMMR,
	})
	if err != nil {
		return 0, 0, err
	}
	return int(row.Rating), int(row.RankedGames), nil
}

func (a *PGStore) RecordSignal(ctx context.Context, matchID string, signal RiskSignal, occurredAt time.Time) error {
	q := a.q()
	payload, err := json.Marshal(signal.Payload)
	if err != nil {
		payload = []byte("{}")
	}
	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}
	signalID, err := q.UpsertRiskEngineSignal(ctx, db.UpsertRiskEngineSignalParams{
		SubjectUserID:    storekit.MustUUID(signal.SubjectUserID),
		SignalType:       signal.SignalType,
		Severity:         db.GdModerationSeverity(signal.Severity),
		EvidenceStrength: db.GdEvidenceStrength(signal.EvidenceStrength),
		DetectorKey:      signal.DetectorKey,
		DetectorVersion:  signal.DetectorVersion,
		ReasonCode:       signal.ReasonCode,
		Score:            signal.Score,
		RecommendedQueue: signal.RecommendedQueue,
		MatchID:          storekit.MustUUID(matchID),
		PayloadJson:      payload,
		OccurredAt:       storekit.Timestamptz(occurredAt),
	})
	if err != nil {
		return err
	}
	if riskSignalQueues(signal.Severity) || signal.RecommendedQueue {
		if a.jobs != nil {
			tx, txErr := a.requireTx()
			if txErr != nil {
				return txErr
			}
			return a.jobs.EnqueueModerationSignal(ctx, tx, signalID)
		}
	}
	return nil
}
