package persistence

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

func (s *pgStore) CreatePlayerReportSignal(params CreatePlayerReportSignalParams) (ModerationSignalCreated, error) {
	params.MatchID = strings.TrimSpace(params.MatchID)
	params.ReporterUserID = strings.TrimSpace(params.ReporterUserID)
	params.ReportedUserID = strings.TrimSpace(params.ReportedUserID)
	params.Category = normalizeReportCategory(params.Category)
	params.Reason = strings.TrimSpace(params.Reason)
	if params.MatchID == "" || params.ReporterUserID == "" || params.ReportedUserID == "" {
		return ModerationSignalCreated{}, errors.New("matchID, reporter, and reported user are required")
	}
	if params.ReporterUserID == params.ReportedUserID {
		return ModerationSignalCreated{}, errors.New("self reports are not allowed")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return ModerationSignalCreated{}, err
	}
	defer tx.Rollback(ctx)
	var participated, muted bool
	if err := tx.QueryRow(ctx, `select exists(select 1 from match_players where match_id = $1 and user_id = $2)`, params.MatchID, params.ReportedUserID).Scan(&participated); err != nil {
		return ModerationSignalCreated{}, err
	}
	if !participated {
		return ModerationSignalCreated{}, errors.New("report target not found")
	}
	if err := tx.QueryRow(ctx, `
		select coalesce(report_muted_at is not null and (report_mute_expires_at is null or report_mute_expires_at > now()), false)
		from users where id = $1
	`, params.ReporterUserID).Scan(&muted); err != nil {
		return ModerationSignalCreated{}, err
	}
	if muted {
		return ModerationSignalCreated{}, errors.New("reporting is temporarily muted")
	}
	payload := map[string]any{"category": params.Category}
	if params.Reason != "" {
		payload["reason"] = params.Reason
	}
	var signalID int64
	err = tx.QueryRow(ctx, `
		insert into moderation_signals(subject_user_id, signal_type, source, severity, evidence_strength,
			reason_code, score, recommended_queue, reporter_user_id, match_id, payload_json, occurred_at)
		values($1, $2, 'player_report', $3, 'limited', $4, $5, true, $6, $7, $8::jsonb, now())
		on conflict (match_id, reporter_user_id, subject_user_id)
			where source = 'player_report' and reporter_user_id is not null and match_id is not null
		do nothing returning id
	`, params.ReportedUserID, "player_report:"+params.Category, reportSeverity(params.Category), params.Category, reportScore(params.Category), params.ReporterUserID, params.MatchID, mustJSON(payload)).Scan(&signalID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ModerationSignalCreated{Status: "duplicate"}, tx.Commit(ctx)
	}
	if err != nil {
		return ModerationSignalCreated{}, err
	}
	if err := enqueueSignalNotification(ctx, tx, signalID); err != nil {
		return ModerationSignalCreated{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return ModerationSignalCreated{}, err
	}
	return ModerationSignalCreated{SignalID: signalID, Status: "created"}, nil
}

func (s *pgStore) ListSubjectModerationProfile(userID string) (ModerationSubjectProfile, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return ModerationSubjectProfile{}, errors.New("userID required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	player, err := s.getAdminPlayerSummary(ctx, userID)
	if err != nil {
		return ModerationSubjectProfile{}, err
	}
	stats, err := s.adminPlayerStats(ctx, userID)
	if err != nil {
		return ModerationSubjectProfile{}, err
	}
	signals, err := s.listSignals(ctx, "s.subject_user_id = $2", []any{100, userID})
	if err != nil {
		return ModerationSubjectProfile{}, err
	}
	log, err := s.listModerationLog(ctx, "l.subject_user_id = $2", []any{100, userID})
	if err != nil {
		return ModerationSubjectProfile{}, err
	}
	applyAdminPlayerStats(&player, stats)
	return ModerationSubjectProfile{Player: player, Signals: signals, Log: log}, nil
}

func (s *pgStore) ListModerationSignals(limit int) ([]ModerationSignalSummary, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 200 {
		limit = 200
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	return s.listSignals(ctx, "true", []any{limit})
}

func (s *pgStore) ListModerationLog(limit int) ([]ModerationAuditLogEntry, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 200 {
		limit = 200
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	return s.listModerationLog(ctx, "true", []any{limit})
}

func (s *pgStore) listSignals(ctx context.Context, where string, args []any) ([]ModerationSignalSummary, error) {
	rows, err := s.pool.Query(ctx, `
		select s.id, s.subject_user_id::text, coalesce(nullif(subject.display_name, ''), s.subject_user_id::text),
			s.signal_type, s.source, s.severity, s.evidence_strength, coalesce(s.detector_key, ''),
			coalesce(s.detector_version, ''), s.reason_code, s.score, s.recommended_queue,
			coalesce(s.reporter_user_id::text, ''), coalesce(nullif(reporter.display_name, ''), s.reporter_user_id::text, ''),
			coalesce(s.match_id::text, ''), s.payload_json::text, s.occurred_at, s.created_at,
			s.reviewed_at, coalesce(s.reviewed_by::text, ''), coalesce(s.outcome, '')
		from moderation_signals s
		left join users subject on subject.id = s.subject_user_id
		left join users reporter on reporter.id = s.reporter_user_id
		where `+where+` order by s.occurred_at desc, s.id desc limit $1
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ModerationSignalSummary{}
	for rows.Next() {
		var item ModerationSignalSummary
		var payload string
		var reviewedAt *time.Time
		if err := rows.Scan(&item.ID, &item.SubjectUserID, &item.SubjectName, &item.SignalType, &item.Source,
			&item.Severity, &item.EvidenceStrength, &item.DetectorKey, &item.DetectorVersion, &item.ReasonCode,
			&item.Score, &item.RecommendedQueue, &item.ReporterUserID, &item.ReporterName, &item.MatchID,
			&payload, &item.OccurredAt, &item.CreatedAt, &reviewedAt, &item.ReviewedBy, &item.Outcome); err != nil {
			return nil, err
		}
		item.Payload = json.RawMessage(payload)
		if reviewedAt != nil {
			item.ReviewedAt = *reviewedAt
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *pgStore) listModerationLog(ctx context.Context, where string, args []any) ([]ModerationAuditLogEntry, error) {
	rows, err := s.pool.Query(ctx, `
		select l.id, coalesce(l.subject_user_id::text, ''), coalesce(nullif(subject.display_name, ''), l.subject_user_id::text, ''),
			coalesce(l.actor_user_id::text, ''), coalesce(nullif(actor.display_name, ''), l.actor_user_id::text, ''),
			l.action, coalesce(l.reason, ''), l.expires_at, l.signal_ids, l.metadata::text, l.created_at
		from moderation_log l
		left join users subject on subject.id = l.subject_user_id
		left join users actor on actor.id = l.actor_user_id
		where `+where+` order by l.created_at desc, l.id desc limit $1
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ModerationAuditLogEntry{}
	for rows.Next() {
		var item ModerationAuditLogEntry
		var expiresAt *time.Time
		var metadata string
		if err := rows.Scan(&item.ID, &item.SubjectUserID, &item.SubjectName, &item.ActorUserID, &item.ActorName,
			&item.Action, &item.Reason, &expiresAt, &item.SignalIDs, &metadata, &item.CreatedAt); err != nil {
			return nil, err
		}
		if expiresAt != nil {
			item.ExpiresAt = *expiresAt
		}
		item.Metadata = json.RawMessage(metadata)
		out = append(out, item)
	}
	return out, rows.Err()
}

func enqueueSignalNotification(ctx context.Context, tx pgx.Tx, signalID int64) error {
	var payload ModerationSignalNotificationPayload
	if err := tx.QueryRow(ctx, `
		select s.id, s.subject_user_id::text, coalesce(nullif(u.display_name, ''), s.subject_user_id::text),
			s.severity, s.evidence_strength, s.reason_code, s.occurred_at
		from moderation_signals s left join users u on u.id = s.subject_user_id where s.id = $1
	`, signalID).Scan(&payload.SignalID, &payload.SubjectUserID, &payload.SubjectName, &payload.Severity,
		&payload.EvidenceStrength, &payload.ReasonCode, &payload.OccurredAt); err != nil {
		return err
	}
	return enqueueNotificationOutbox(ctx, tx, "moderation_signal_queued", fmt.Sprintf("moderation_signal:%d:queued", signalID), payload)
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
