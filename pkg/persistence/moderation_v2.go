package persistence

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	db "geoduels/pkg/persistence/sqlc/db"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"strings"
	"time"
)

func (s *DB) CreatePlayerReportSignal(p CreatePlayerReportSignalParams) (ModerationSignalCreated, error) {
	p.MatchID = strings.TrimSpace(p.MatchID)
	p.ReporterUserID = strings.TrimSpace(p.ReporterUserID)
	p.ReportedUserID = strings.TrimSpace(p.ReportedUserID)
	p.Category = normalizeReportCategory(p.Category)
	p.Reason = strings.TrimSpace(p.Reason)
	if p.MatchID == "" || p.ReporterUserID == "" || p.ReportedUserID == "" {
		return ModerationSignalCreated{}, errors.New("matchID, reporter, and reported user are required")
	}
	if p.ReporterUserID == p.ReportedUserID {
		return ModerationSignalCreated{}, errors.New("self reports are not allowed")
	}
	ctx, c := context.WithTimeout(context.Background(), 6*time.Second)
	defer c()
	tx, e := s.pool.Begin(ctx)
	if e != nil {
		return ModerationSignalCreated{}, e
	}
	defer tx.Rollback(ctx)
	mid, e := profileUUID(p.MatchID)
	if e != nil {
		return ModerationSignalCreated{}, e
	}
	rid, e := profileUUID(p.ReporterUserID)
	if e != nil {
		return ModerationSignalCreated{}, e
	}
	sid, e := profileUUID(p.ReportedUserID)
	if e != nil {
		return ModerationSignalCreated{}, e
	}
	q := s.db.WithTx(tx)
	ok, e := q.PlayerParticipated(ctx, db.PlayerParticipatedParams{MatchID: mid, UserID: sid})
	if e != nil {
		return ModerationSignalCreated{}, e
	}
	if !ok {
		return ModerationSignalCreated{}, errors.New("report target not found")
	}
	mv, e := q.ReporterMuted(ctx, rid)
	if e != nil {
		return ModerationSignalCreated{}, e
	}
	if v, ok := mv.(bool); ok && v {
		return ModerationSignalCreated{}, errors.New("reporting is temporarily muted")
	}
	b, _ := json.Marshal(map[string]any{"category": p.Category})
	if p.Reason != "" {
		b, _ = json.Marshal(map[string]any{"category": p.Category, "reason": p.Reason})
	}
	id, e := q.CreatePlayerReportSignal(ctx, db.CreatePlayerReportSignalParams{SubjectUserID: sid, SignalType: "player_report:" + p.Category, Severity: db.GdModerationSeverity(reportSeverity(p.Category)), ReasonCode: p.Category, Score: reportScore(p.Category), ReporterUserID: rid, MatchID: mid, PayloadJson: b})
	if errors.Is(e, pgx.ErrNoRows) {
		return ModerationSignalCreated{Status: "duplicate"}, tx.Commit(ctx)
	}
	if e != nil {
		return ModerationSignalCreated{}, e
	}
	r, e := q.GetSignalNotificationPayload(ctx, int64(id))
	if e != nil {
		return ModerationSignalCreated{}, e
	}
	np, _ := json.Marshal(ModerationSignalNotificationPayload{SignalID: id, SubjectUserID: r.SSubjectUserID, SubjectName: textVal(r.Coalesce), Severity: string(r.Severity), EvidenceStrength: string(r.EvidenceStrength), ReasonCode: string(r.ReasonCode), OccurredAt: r.OccurredAt.Time})
	if e = s.db.WithTx(tx).EnqueueNotificationOutbox(ctx, db.EnqueueNotificationOutboxParams{NotificationType: "moderation_signal_queued", DedupeKey: fmt.Sprintf("moderation_signal:%d:queued", id), Payload: np}); e != nil {
		return ModerationSignalCreated{}, e
	}
	if e = tx.Commit(ctx); e != nil {
		return ModerationSignalCreated{}, e
	}
	return ModerationSignalCreated{SignalID: id, Status: "created"}, nil
}
func (s *DB) ListSubjectModerationProfile(id string) (ModerationSubjectProfile, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return ModerationSubjectProfile{}, errors.New("userID required")
	}
	ctx, c := context.WithTimeout(context.Background(), 5*time.Second)
	defer c()
	p, e := s.getAdminPlayerSummary(ctx, id)
	if e != nil {
		return ModerationSubjectProfile{}, e
	}
	st, e := s.adminPlayerStats(ctx, id)
	if e != nil {
		return ModerationSubjectProfile{}, e
	}
	u, e := profileUUID(id)
	if e != nil {
		return ModerationSubjectProfile{}, e
	}
	sg, e := s.listSignals(ctx, u, 100)
	if e != nil {
		return ModerationSubjectProfile{}, e
	}
	lg, e := s.listModerationLog(ctx, u, 100)
	if e != nil {
		return ModerationSubjectProfile{}, e
	}
	applyAdminPlayerStats(&p, st)
	return ModerationSubjectProfile{Player: p, Signals: sg, Log: lg}, nil
}
func (s *DB) ListModerationSignals(n int) ([]ModerationSignalSummary, error) {
	if n <= 0 {
		n = 100
	}
	if n > 200 {
		n = 200
	}
	ctx, c := context.WithTimeout(context.Background(), 4*time.Second)
	defer c()
	return s.listSignals(ctx, pgtype.UUID{}, int32(n))
}
func (s *DB) ListModerationLog(n int) ([]ModerationAuditLogEntry, error) {
	if n <= 0 {
		n = 100
	}
	if n > 200 {
		n = 200
	}
	ctx, c := context.WithTimeout(context.Background(), 4*time.Second)
	defer c()
	return s.listModerationLog(ctx, pgtype.UUID{}, int32(n))
}
func (s *DB) listSignals(ctx context.Context, u pgtype.UUID, n int32) ([]ModerationSignalSummary, error) {
	r, e := s.db.ListModerationSignals(ctx, db.ListModerationSignalsParams{SubjectUserID: u, LimitCount: n})
	if e != nil {
		return nil, e
	}
	o := make([]ModerationSignalSummary, 0, len(r))
	for _, x := range r {
		o = append(o, ModerationSignalSummary{ID: x.ID, SubjectUserID: x.SSubjectUserID, SubjectName: textVal(x.Coalesce), SignalType: x.SignalType, Source: string(x.Source), Severity: string(x.Severity), EvidenceStrength: string(x.EvidenceStrength), DetectorKey: x.DetectorKey, DetectorVersion: x.DetectorVersion, ReasonCode: x.ReasonCode, Score: x.Score, RecommendedQueue: x.RecommendedQueue, ReporterUserID: textVal(x.Coalesce_2), ReporterName: textVal(x.Coalesce_3), MatchID: textVal(x.Coalesce_4), Payload: json.RawMessage(x.SPayloadJson), OccurredAt: x.OccurredAt.Time, CreatedAt: x.CreatedAt.Time})
	}
	return o, nil
}
func (s *DB) listModerationLog(ctx context.Context, u pgtype.UUID, n int32) ([]ModerationAuditLogEntry, error) {
	r, e := s.db.ListModerationLog(ctx, db.ListModerationLogParams{SubjectUserID: u, LimitCount: n})
	if e != nil {
		return nil, e
	}
	o := make([]ModerationAuditLogEntry, 0, len(r))
	for _, x := range r {
		o = append(o, ModerationAuditLogEntry{ID: x.ID, SubjectUserID: textVal(x.Coalesce), SubjectName: textVal(x.Coalesce_2), ActorUserID: textVal(x.Coalesce_3), ActorName: textVal(x.Coalesce_4), Action: string(x.Action), Reason: x.Reason, SignalIDs: x.SignalIds, Metadata: json.RawMessage(x.LMetadata), CreatedAt: x.CreatedAt.Time})
	}
	return o, nil
}
func textVal(v interface{}) string {
	if v == nil {
		return ""
	}
	return fmt.Sprint(v)
}
func normalizeReportCategory(c string) string {
	switch strings.ToLower(strings.TrimSpace(c)) {
	case "cheating", "profile", "harassment", "boosting":
		return strings.ToLower(strings.TrimSpace(c))
	default:
		return "other"
	}
}
func reportSeverity(c string) string {
	switch c {
	case "cheating", "boosting", "harassment":
		return "medium"
	default:
		return "low"
	}
}
func reportScore(c string) float64 {
	switch c {
	case "cheating", "boosting":
		return 2
	case "harassment":
		return 1.5
	default:
		return 1
	}
}

func enqueueSignalNotification(ctx context.Context, tx pgx.Tx, signalID int64) error {
	r, err := db.New(tx).GetSignalNotificationPayload(ctx, signalID)
	if err != nil {
		return err
	}
	p, _ := json.Marshal(ModerationSignalNotificationPayload{SignalID: signalID, SubjectUserID: r.SSubjectUserID, SubjectName: textVal(r.Coalesce), Severity: string(r.Severity), EvidenceStrength: string(r.EvidenceStrength), ReasonCode: r.ReasonCode, OccurredAt: r.OccurredAt.Time})
	return db.New(tx).EnqueueNotificationOutbox(ctx, db.EnqueueNotificationOutboxParams{NotificationType: "moderation_signal_queued", DedupeKey: fmt.Sprintf("moderation_signal:%d:queued", signalID), Payload: p})
}
