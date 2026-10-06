package moderation

import (
	"context"
	"encoding/json"
	"errors"
	"geoduels/internal/storekit"
	"geoduels/pkg/contracts"
	db "geoduels/pkg/persistence/sqlc/db"
	"github.com/jackc/pgx/v5"
)

func (a *PGStore) ReportEligibility(ctx context.Context, matchID, reporterID, subjectID string) (ReportEligibility, error) {
	mid, err := storekit.ProfileUUID(matchID)
	if err != nil {
		return ReportEligibility{}, err
	}
	rid, err := storekit.ProfileUUID(reporterID)
	if err != nil {
		return ReportEligibility{}, err
	}
	sid, err := storekit.ProfileUUID(subjectID)
	if err != nil {
		return ReportEligibility{}, err
	}
	participated, err := a.q().PlayerParticipated(ctx, db.PlayerParticipatedParams{MatchID: mid, UserID: sid})
	if err != nil {
		return ReportEligibility{}, err
	}
	muted, err := a.q().ReporterMuted(ctx, rid)
	if err != nil {
		return ReportEligibility{}, err
	}
	isMuted, _ := muted.(bool)
	return ReportEligibility{TargetParticipated: participated, ReporterMuted: isMuted}, nil
}

// SaveReport persists and deduplicates the signal together with its notification job.
func (a *PGStore) SaveReport(ctx context.Context, report PlayerReport) (contracts.ModerationSignalCreated, error) {
	tx, err := a.requireTx()
	if err != nil {
		return contracts.ModerationSignalCreated{}, err
	}
	payload := map[string]any{"category": report.Category}
	if report.Reason != "" {
		payload["reason"] = report.Reason
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return contracts.ModerationSignalCreated{}, err
	}
	id, err := a.q().CreatePlayerReportSignal(ctx, db.CreatePlayerReportSignalParams{
		SubjectUserID: storekit.MustUUID(report.SubjectID), SignalType: "player_report:" + report.Category,
		Severity: db.GdModerationSeverity(report.Severity), ReasonCode: report.Category, Score: report.Score,
		ReporterUserID: storekit.MustUUID(report.ReporterID), MatchID: storekit.MustUUID(report.MatchID), PayloadJson: body,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return contracts.ModerationSignalCreated{Status: "duplicate"}, nil
	}
	if err != nil {
		return contracts.ModerationSignalCreated{}, err
	}
	if a.jobs != nil {
		if err := a.jobs.EnqueueModerationSignal(ctx, tx, id); err != nil {
			return contracts.ModerationSignalCreated{}, err
		}
	}
	return contracts.ModerationSignalCreated{SignalID: id, Status: "created"}, nil
}
