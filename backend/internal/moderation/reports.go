package moderation

import (
	"context"
	"errors"
	"strings"
	"time"

	"geoduels/pkg/contracts"
)

// ReportEligibility contains the stored facts used to authorize a player report.
type ReportEligibility struct{ TargetParticipated, ReporterMuted bool }

type PlayerReport struct {
	MatchID, ReporterID, SubjectID, Category, Reason, Severity string
	Score                                                      float64
}

// CreateReport records a player-initiated report.
func (s *Service) CreateReport(ctx context.Context, matchID, reporter, reported, category, reason string) (contracts.ModerationSignalCreated, error) {
	report := PlayerReport{MatchID: strings.TrimSpace(matchID), ReporterID: strings.TrimSpace(reporter), SubjectID: strings.TrimSpace(reported), Category: normalizeReportCategory(category), Reason: strings.TrimSpace(reason)}
	if report.MatchID == "" || report.ReporterID == "" || report.SubjectID == "" {
		return contracts.ModerationSignalCreated{}, errors.New("matchID, reporter, and reported user are required")
	}
	if report.ReporterID == report.SubjectID {
		return contracts.ModerationSignalCreated{}, errors.New("self reports are not allowed")
	}
	report.Severity, report.Score = reportSeverity(report.Category), reportScore(report.Category)
	ctx, cancel := context.WithTimeout(ctx, 6*time.Second)
	defer cancel()
	var out contracts.ModerationSignalCreated
	err := s.store.WithinTx(ctx, func(store Store) error {
		eligibility, err := store.ReportEligibility(ctx, report.MatchID, report.ReporterID, report.SubjectID)
		if err != nil {
			return err
		}
		if !eligibility.TargetParticipated {
			return errors.New("report target not found")
		}
		if eligibility.ReporterMuted {
			return errors.New("reporting is temporarily muted")
		}
		out, err = store.SaveReport(ctx, report)
		return err
	})
	if err != nil {
		return contracts.ModerationSignalCreated{}, err
	}
	return out, nil
}

func normalizeReportCategory(category string) string {
	switch strings.ToLower(strings.TrimSpace(category)) {
	case "cheating", "profile", "harassment", "boosting":
		return strings.ToLower(strings.TrimSpace(category))
	default:
		return "other"
	}
}

func reportSeverity(category string) string {
	switch category {
	case "cheating", "boosting", "harassment":
		return "medium"
	default:
		return "low"
	}
}

func reportScore(category string) float64 {
	switch category {
	case "cheating", "boosting":
		return 2
	case "harassment":
		return 1.5
	default:
		return 1
	}
}
