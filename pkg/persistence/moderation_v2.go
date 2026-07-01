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
	var participated bool
	if err := tx.QueryRow(ctx, `
		select exists(select 1 from match_players where match_id = $1 and user_id = $2)
	`, params.MatchID, params.ReportedUserID).Scan(&participated); err != nil {
		return ModerationSignalCreated{}, err
	}
	if !participated {
		return ModerationSignalCreated{}, errors.New("report target not found")
	}
	var muted bool
	if err := tx.QueryRow(ctx, `
		select exists(
			select 1 from moderation_reporter_state
			where user_id = $1 and muted_until is not null and muted_until > now()
		)
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
		insert into moderation_signals(
			subject_user_id, signal_type, source, severity, evidence_strength,
			reason_code, score, recommended_queue, reporter_user_id, match_id,
			payload_json, occurred_at
		)
		values($1, $2, 'player_report', $3, $4, $5, $6, true, $7, $8, $9::jsonb, now())
		on conflict (match_id, reporter_user_id, subject_user_id)
			where source = 'player_report' and reporter_user_id is not null and match_id is not null
		do nothing
		returning id
	`, params.ReportedUserID, "player_report:"+params.Category, reportSeverity(params.Category), "limited", params.Category, reportScore(params.Category), params.ReporterUserID, params.MatchID, mustJSON(payload)).Scan(&signalID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ModerationSignalCreated{Status: "duplicate"}, tx.Commit(ctx)
	}
	if err != nil {
		return ModerationSignalCreated{}, err
	}
	if _, err := tx.Exec(ctx, `
		insert into moderation_reporter_state(user_id, reports_submitted, updated_at)
		values($1, 1, now())
		on conflict (user_id) do update set
			reports_submitted = moderation_reporter_state.reports_submitted + 1,
			updated_at = now()
	`, params.ReporterUserID); err != nil {
		return ModerationSignalCreated{}, err
	}
	incidentID, taskID, err := upsertIncidentForSignal(ctx, tx, signalID)
	if err != nil {
		return ModerationSignalCreated{}, err
	}
	if err := enqueueIncidentNotification(ctx, tx, incidentID, taskID); err != nil {
		return ModerationSignalCreated{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return ModerationSignalCreated{}, err
	}
	return ModerationSignalCreated{SignalID: signalID, IncidentID: incidentID, Status: "created"}, nil
}

func (s *pgStore) ListReviewTasks(view, actorUserID string, limit int) ([]ModerationReviewTaskSummary, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}
	view = strings.TrimSpace(view)
	if view == "" {
		view = "queue"
	}
	where := "t.status = 'open'"
	args := []any{limit}
	switch view {
	case "mine":
		where = "t.status in ('claimed', 'blocked') and t.assigned_to = $2"
		args = append(args, strings.TrimSpace(actorUserID))
	case "watchlist":
		where = "i.status = 'watching'"
	case "closed":
		where = "t.status in ('done', 'expired')"
	case "queue":
	default:
		where = "t.status = 'open'"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	rows, err := s.pool.Query(ctx, reviewTaskSelectQuery(where), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ModerationReviewTaskSummary{}
	for rows.Next() {
		item, err := scanReviewTask(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *pgStore) GetIncidentDetail(incidentID int64) (ModerationIncidentDetail, error) {
	if incidentID <= 0 {
		return ModerationIncidentDetail{}, errors.New("incidentID required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return s.getIncidentDetail(ctx, incidentID)
}

func (s *pgStore) ClaimReviewTask(taskID int64, actorUserID string) (ModerationIncidentDetail, error) {
	return s.assignReviewTask(taskID, actorUserID, actorUserID)
}

func (s *pgStore) ReleaseReviewTask(taskID int64, actorUserID string) (ModerationIncidentDetail, error) {
	return s.assignReviewTask(taskID, actorUserID, "")
}

func (s *pgStore) assignReviewTask(taskID int64, actorUserID, assignedTo string) (ModerationIncidentDetail, error) {
	if taskID <= 0 {
		return ModerationIncidentDetail{}, errors.New("taskID required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return ModerationIncidentDetail{}, err
	}
	defer tx.Rollback(ctx)
	var incidentID int64
	status := "open"
	if strings.TrimSpace(assignedTo) != "" {
		status = "claimed"
	}
	if err := tx.QueryRow(ctx, `
		update moderation_review_tasks
		set assigned_to = nullif($2, '')::uuid,
			status = $3,
			claimed_at = case when nullif($2, '') is null then null else now() end,
			claim_expires_at = case when nullif($2, '') is null then null else now() + interval '2 hours' end,
			updated_at = now()
		where id = $1
		returning incident_id
	`, taskID, strings.TrimSpace(assignedTo), status).Scan(&incidentID); err != nil {
		return ModerationIncidentDetail{}, err
	}
	if _, err := tx.Exec(ctx, `
		update moderation_incidents
		set assigned_to = nullif($2, '')::uuid, updated_at = now()
		where id = $1
	`, incidentID, strings.TrimSpace(assignedTo)); err != nil {
		return ModerationIncidentDetail{}, err
	}
	event := "task_released"
	if strings.TrimSpace(assignedTo) != "" {
		event = "task_claimed"
	}
	if _, err := tx.Exec(ctx, `
		insert into moderation_audit_log(incident_id, task_id, actor_user_id, event_type)
		values($1, $2, nullif($3, '')::uuid, $4)
	`, incidentID, taskID, strings.TrimSpace(actorUserID), event); err != nil {
		return ModerationIncidentDetail{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return ModerationIncidentDetail{}, err
	}
	return s.GetIncidentDetail(incidentID)
}

func (s *pgStore) SubmitVerdict(incidentID int64, actorUserID string, input ModerationVerdictInput) (ModerationIncidentDetail, error) {
	if incidentID <= 0 {
		return ModerationIncidentDetail{}, errors.New("incidentID required")
	}
	input.Verdict = strings.TrimSpace(input.Verdict)
	input.ReasonCode = strings.TrimSpace(input.ReasonCode)
	input.EnforcementAction = strings.TrimSpace(input.EnforcementAction)
	input.Note = strings.TrimSpace(input.Note)
	if input.Verdict == "" || input.ReasonCode == "" {
		return ModerationIncidentDetail{}, errors.New("verdict and reasonCode are required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return ModerationIncidentDetail{}, err
	}
	defer tx.Rollback(ctx)
	var subjectUserID string
	if err := tx.QueryRow(ctx, `select subject_user_id from moderation_incidents where id = $1 for update`, incidentID).Scan(&subjectUserID); err != nil {
		return ModerationIncidentDetail{}, err
	}
	var verdictID int64
	if err := tx.QueryRow(ctx, `
		insert into moderation_verdicts(
			incident_id, task_id, actor_user_id, verdict, reason_code, note,
			enforcement_action, metadata
		)
		values($1, nullif($2, 0), nullif($3, '')::uuid, $4, $5, nullif($6, ''), nullif($7, ''), $8::jsonb)
		returning id
	`, incidentID, input.TaskID, strings.TrimSpace(actorUserID), input.Verdict, input.ReasonCode, input.Note, input.EnforcementAction, mustJSON(map[string]any{"durationHours": input.DurationHours})).Scan(&verdictID); err != nil {
		return ModerationIncidentDetail{}, err
	}
	status := incidentStatusForVerdict(input.Verdict)
	if _, err := tx.Exec(ctx, `
		update moderation_incidents
		set status = $2,
			resolved_at = case when $2 in ('actioned', 'dismissed', 'inconclusive', 'duplicate') then now() else resolved_at end,
			resolved_by = case when $2 in ('actioned', 'dismissed', 'inconclusive', 'duplicate') then nullif($3, '')::uuid else resolved_by end,
			resolution_note = nullif($4, ''),
			updated_at = now()
		where id = $1
	`, incidentID, status, strings.TrimSpace(actorUserID), input.Note); err != nil {
		return ModerationIncidentDetail{}, err
	}
	if input.TaskID > 0 {
		if _, err := tx.Exec(ctx, `
			update moderation_review_tasks
			set status = 'done', completed_at = now(), updated_at = now()
			where id = $1 and incident_id = $2
		`, input.TaskID, incidentID); err != nil {
			return ModerationIncidentDetail{}, err
		}
	}
	if err := updateReporterStateForVerdict(ctx, tx, incidentID, input.Verdict); err != nil {
		return ModerationIncidentDetail{}, err
	}
	if input.EnforcementAction != "" {
		if err := applyModerationEnforcement(ctx, tx, subjectUserID, strings.TrimSpace(actorUserID), incidentID, verdictID, input); err != nil {
			return ModerationIncidentDetail{}, err
		}
	}
	if _, err := tx.Exec(ctx, `
		insert into moderation_audit_log(incident_id, task_id, actor_user_id, event_type, reason_code, body)
		values($1, nullif($2, 0), nullif($3, '')::uuid, 'verdict_submitted', $4, nullif($5, ''))
	`, incidentID, input.TaskID, strings.TrimSpace(actorUserID), input.ReasonCode, input.Note); err != nil {
		return ModerationIncidentDetail{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return ModerationIncidentDetail{}, err
	}
	return s.GetIncidentDetail(incidentID)
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
	incidents, err := s.listIncidentsForSubject(ctx, userID, 20)
	if err != nil {
		return ModerationSubjectProfile{}, err
	}
	signals, err := s.listSignals(ctx, "subject_user_id = $2", []any{20, userID})
	if err != nil {
		return ModerationSubjectProfile{}, err
	}
	enforcement, err := s.listEnforcementActions(ctx, "e.target_user_id = $2", []any{20, userID})
	if err != nil {
		return ModerationSubjectProfile{}, err
	}
	return ModerationSubjectProfile{Player: player, Incidents: incidents, Signals: signals, Enforcement: enforcement}, nil
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

func (s *pgStore) getIncidentDetail(ctx context.Context, incidentID int64) (ModerationIncidentDetail, error) {
	var detail ModerationIncidentDetail
	row := s.pool.QueryRow(ctx, incidentSelectQuery("i.id = $1"), incidentID)
	incident, err := scanIncident(row)
	if err != nil {
		return ModerationIncidentDetail{}, err
	}
	detail.Incident = incident
	player, err := s.getAdminPlayerSummary(ctx, incident.SubjectUserID)
	if err == nil {
		detail.SubjectPlayer = &player
	}
	tasks, err := s.listTasksForIncident(ctx, incidentID)
	if err != nil {
		return ModerationIncidentDetail{}, err
	}
	signals, err := s.listSignalsForIncident(ctx, incidentID)
	if err != nil {
		return ModerationIncidentDetail{}, err
	}
	matches, err := s.listIncidentMatches(ctx, incidentID)
	if err != nil {
		return ModerationIncidentDetail{}, err
	}
	verdicts, err := s.listVerdicts(ctx, incidentID)
	if err != nil {
		return ModerationIncidentDetail{}, err
	}
	audit, err := s.listAuditLog(ctx, incidentID)
	if err != nil {
		return ModerationIncidentDetail{}, err
	}
	reporters, err := s.listReporterStateForIncident(ctx, incidentID)
	if err != nil {
		return ModerationIncidentDetail{}, err
	}
	detail.Tasks = tasks
	detail.Signals = signals
	detail.Matches = matches
	detail.Verdicts = verdicts
	detail.AuditLog = audit
	detail.ReporterState = reporters
	return detail, nil
}

func upsertIncidentForSignal(ctx context.Context, tx pgx.Tx, signalID int64) (int64, int64, error) {
	var subjectUserID, severity, strength, reasonCode string
	var recommended bool
	var occurredAt time.Time
	if err := tx.QueryRow(ctx, `
		select subject_user_id, severity, evidence_strength, reason_code, recommended_queue, occurred_at
		from moderation_signals
		where id = $1
	`, signalID).Scan(&subjectUserID, &severity, &strength, &reasonCode, &recommended, &occurredAt); err != nil {
		return 0, 0, err
	}
	var incidentID int64
	if err := tx.QueryRow(ctx, `
		insert into moderation_incidents(
			subject_user_id, status, severity, evidence_strength, reason_code,
			summary, signal_count, unique_reporter_count, latest_signal_at
		)
		values($1, 'open', $2, $3, $4, $5, 0, 0, $6)
		on conflict (subject_user_id) where status in ('open', 'watching')
		do update set
			severity = greatest_severity(moderation_incidents.severity, excluded.severity),
			evidence_strength = greatest_evidence_strength(moderation_incidents.evidence_strength, excluded.evidence_strength),
			reason_code = case
				when severity_rank(excluded.severity) >= severity_rank(moderation_incidents.severity) then excluded.reason_code
				else moderation_incidents.reason_code
			end,
			latest_signal_at = greatest(moderation_incidents.latest_signal_at, excluded.latest_signal_at),
			updated_at = now()
		returning id
	`, subjectUserID, severity, strength, reasonCode, incidentSummary(reasonCode), occurredAt).Scan(&incidentID); err != nil {
		return 0, 0, err
	}
	if _, err := tx.Exec(ctx, `
		insert into moderation_incident_signals(incident_id, signal_id)
		values($1, $2)
		on conflict do nothing
	`, incidentID, signalID); err != nil {
		return 0, 0, err
	}
	if err := refreshIncidentAggregates(ctx, tx, incidentID); err != nil {
		return 0, 0, err
	}
	taskID, err := upsertReviewTask(ctx, tx, incidentID, recommended, severity)
	if err != nil {
		return 0, 0, err
	}
	if _, err := tx.Exec(ctx, `
		insert into moderation_audit_log(incident_id, event_type, reason_code, metadata)
		values($1, 'signal_attached', $2, jsonb_build_object('signalId', $3::bigint))
	`, incidentID, reasonCode, signalID); err != nil {
		return 0, 0, err
	}
	return incidentID, taskID, nil
}

func refreshIncidentAggregates(ctx context.Context, tx pgx.Tx, incidentID int64) error {
	_, err := tx.Exec(ctx, `
		update moderation_incidents i
		set
			signal_count = agg.signal_count,
			unique_reporter_count = agg.unique_reporter_count,
			severity = agg.severity,
			evidence_strength = agg.evidence_strength,
			latest_signal_at = agg.latest_signal_at,
			updated_at = now()
		from (
			select
				count(*)::int as signal_count,
				count(distinct reporter_user_id) filter (where reporter_user_id is not null)::int as unique_reporter_count,
				(select s2.severity from moderation_incident_signals x join moderation_signals s2 on s2.id = x.signal_id where x.incident_id = $1 order by severity_rank(s2.severity) desc, s2.occurred_at desc limit 1) as severity,
				(select s2.evidence_strength from moderation_incident_signals x join moderation_signals s2 on s2.id = x.signal_id where x.incident_id = $1 order by evidence_strength_rank(s2.evidence_strength) desc, s2.occurred_at desc limit 1) as evidence_strength,
				max(s.occurred_at) as latest_signal_at
			from moderation_incident_signals link
			join moderation_signals s on s.id = link.signal_id
			where link.incident_id = $1
		) agg
		where i.id = $1
	`, incidentID)
	return err
}

func upsertReviewTask(ctx context.Context, tx pgx.Tx, incidentID int64, recommended bool, severity string) (int64, error) {
	priority := priorityForSeverity(severity)
	queue := "standard"
	if severity == "critical" || severity == "high" {
		queue = "high_risk"
	}
	if !recommended && severity == "low" {
		queue = "watchlist"
	}
	var taskID int64
	err := tx.QueryRow(ctx, `
		insert into moderation_review_tasks(incident_id, status, queue, priority)
		values($1, 'open', $2, $3)
		on conflict (incident_id) where status in ('open', 'claimed', 'blocked')
		do update set
			queue = case when moderation_review_tasks.queue = 'high_risk' then 'high_risk' else excluded.queue end,
			priority = greatest_priority(moderation_review_tasks.priority, excluded.priority),
			updated_at = now()
		returning id
	`, incidentID, queue, priority).Scan(&taskID)
	return taskID, err
}

func enqueueIncidentNotification(ctx context.Context, tx pgx.Tx, incidentID, taskID int64) error {
	var payload ModerationIncidentNotificationPayload
	var strongest []string
	if err := tx.QueryRow(ctx, `
		select
			i.id, $2::bigint, i.subject_user_id::text,
			coalesce(nullif(u.display_name, ''), i.subject_user_id::text),
			i.severity, i.evidence_strength, i.reason_code, i.signal_count, i.latest_signal_at
		from moderation_incidents i
		left join users u on u.id = i.subject_user_id
		where i.id = $1
	`, incidentID, taskID).Scan(&payload.IncidentID, &payload.TaskID, &payload.SubjectUserID, &payload.SubjectName, &payload.Severity, &payload.EvidenceStrength, &payload.ReasonCode, &payload.SignalCount, &payload.LatestSignalAt); err != nil {
		return err
	}
	rows, err := tx.Query(ctx, `
		select reason_code
		from moderation_signals s
		join moderation_incident_signals link on link.signal_id = s.id
		where link.incident_id = $1
		order by severity_rank(s.severity) desc, evidence_strength_rank(s.evidence_strength) desc, s.occurred_at desc
		limit 3
	`, incidentID)
	if err != nil {
		return err
	}
	for rows.Next() {
		var reason string
		if err := rows.Scan(&reason); err != nil {
			rows.Close()
			return err
		}
		strongest = append(strongest, reason)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	payload.StrongestSignals = strongest
	if payload.Severity != "high" && payload.Severity != "critical" {
		return nil
	}
	return enqueueNotificationOutbox(ctx, tx, "moderation_incident_queued", fmt.Sprintf("moderation_incident:%d:queued", incidentID), payload)
}

func applyModerationEnforcement(ctx context.Context, tx pgx.Tx, subjectUserID, actorUserID string, incidentID, verdictID int64, input ModerationVerdictInput) error {
	action := input.EnforcementAction
	var endsAt any
	if input.DurationHours > 0 {
		endsAt = time.Now().Add(time.Duration(input.DurationHours) * time.Hour)
	}
	switch action {
	case "temporary_ban", "permanent_ban":
		if action == "temporary_ban" && endsAt == nil {
			endsAt = time.Now().Add(30 * 24 * time.Hour)
		}
		if action == "permanent_ban" {
			endsAt = nil
		}
		if _, err := tx.Exec(ctx, `
			update users
			set banned_at = coalesce(banned_at, now()),
				ban_reason = $2,
				ban_expires_at = $3
			where id = $1
		`, subjectUserID, nonempty(input.Note, input.ReasonCode), endsAt); err != nil {
			return err
		}
		if action == "permanent_ban" {
			if err := banUserOAuthIdentities(ctx, tx, subjectUserID, nonempty(input.Note, input.ReasonCode), actorUserID); err != nil {
				return err
			}
		}
	case "unban":
		if _, err := tx.Exec(ctx, `
			update users set banned_at = null, ban_reason = null, ban_expires_at = null where id = $1
		`, subjectUserID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			update oauth_identity_bans
			set revoked_at = coalesce(revoked_at, now())
			where banned_user_id = $1
				and revoked_at is null
		`, subjectUserID); err != nil {
			return err
		}
	case "chat_mute":
		if endsAt == nil {
			endsAt = time.Now().Add(30 * 24 * time.Hour)
		}
	case "report_mute":
		if _, err := tx.Exec(ctx, `
			insert into moderation_reporter_state(user_id, muted_until, updated_at)
			values($1, coalesce($2::timestamptz, now() + interval '7 days'), now())
			on conflict (user_id) do update set muted_until = excluded.muted_until, updated_at = now()
		`, subjectUserID, endsAt); err != nil {
			return err
		}
	case "refund":
		refunds, err := issueCurrentMMRRefundsForCheater(ctx, tx, subjectUserID, input.ReasonCode, time.Time{})
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			insert into enforcement_actions(target_user_id, actor_user_id, source_incident_id, source_verdict_id, action_type, reason_code, reason_note, metadata)
			values($1, nullif($2, '')::uuid, $3, $4, 'refund', $5, nullif($6, ''), $7::jsonb)
		`, subjectUserID, actorUserID, incidentID, verdictID, input.ReasonCode, input.Note, mustJSON(refunds)); err != nil {
			return err
		}
		return nil
	}
	_, err := tx.Exec(ctx, `
		insert into enforcement_actions(
			target_user_id, actor_user_id, source_incident_id, source_verdict_id,
			action_type, reason_code, reason_note, ends_at
		)
		values($1, nullif($2, '')::uuid, $3, $4, $5, $6, nullif($7, ''), $8)
	`, subjectUserID, actorUserID, incidentID, verdictID, action, input.ReasonCode, input.Note, endsAt)
	return err
}

func updateReporterStateForVerdict(ctx context.Context, tx pgx.Tx, incidentID int64, verdict string) error {
	column := ""
	switch verdict {
	case "confirmed":
		column = "reports_useful"
	case "dismissed":
		column = "reports_dismissed"
	case "inconclusive", "watch":
		column = "reports_inconclusive"
	case "abusive_report":
		column = "reports_abusive"
	default:
		return nil
	}
	_, err := tx.Exec(ctx, fmt.Sprintf(`
		insert into moderation_reporter_state(user_id, %s, reports_submitted, report_weight, muted_until, updated_at)
		select reporter_user_id, 1, 0, reporter_weight_for_outcome($1), case when $1 = 'abusive_report' then now() + interval '7 days' else null end, now()
		from (
			select distinct s.reporter_user_id
			from moderation_signals s
			join moderation_incident_signals link on link.signal_id = s.id
			where link.incident_id = $2 and s.reporter_user_id is not null
		) reporters
		on conflict (user_id) do update set
			%s = moderation_reporter_state.%s + 1,
			report_weight = reporter_weight_for_outcome($1),
			muted_until = case when $1 = 'abusive_report' then greatest(coalesce(moderation_reporter_state.muted_until, now()), now() + interval '7 days') else moderation_reporter_state.muted_until end,
			updated_at = now()
	`, column, column, column), verdict, incidentID)
	return err
}

func incidentSelectQuery(where string) string {
	return `
		select
			i.id, i.subject_user_id::text, coalesce(nullif(u.display_name, ''), i.subject_user_id::text),
			i.status, i.severity, i.evidence_strength, i.reason_code, i.summary,
			i.signal_count, i.unique_reporter_count, coalesce(i.assigned_to::text, ''),
			i.watch_until, i.latest_signal_at, i.resolved_at, coalesce(i.resolved_by::text, ''),
			coalesce(i.resolution_note, ''), i.created_at, i.updated_at
		from moderation_incidents i
		left join users u on u.id = i.subject_user_id
		where ` + where
}

func reviewTaskSelectQuery(where string) string {
	return `
		select
			t.id, t.incident_id, t.status, t.queue, t.priority, coalesce(t.assigned_to::text, ''),
			t.claimed_at, t.claim_expires_at, t.completed_at, t.created_at, t.updated_at,
			i.id, i.subject_user_id::text, coalesce(nullif(u.display_name, ''), i.subject_user_id::text),
			i.status, i.severity, i.evidence_strength, i.reason_code, i.summary,
			i.signal_count, i.unique_reporter_count, coalesce(i.assigned_to::text, ''),
			i.watch_until, i.latest_signal_at, i.resolved_at, coalesce(i.resolved_by::text, ''),
			coalesce(i.resolution_note, ''), i.created_at, i.updated_at
		from moderation_review_tasks t
		join moderation_incidents i on i.id = t.incident_id
		left join users u on u.id = i.subject_user_id
		where ` + where + `
		order by
			case t.priority when 'urgent' then 0 when 'high' then 1 when 'medium' then 2 else 3 end,
			t.created_at desc
		limit $1
	`
}

func scanReviewTask(row interface{ Scan(...any) error }) (ModerationReviewTaskSummary, error) {
	var item ModerationReviewTaskSummary
	var claimedAt, claimExpiresAt, completedAt *time.Time
	var incidentWatchUntil, incidentResolvedAt *time.Time
	err := row.Scan(
		&item.ID, &item.IncidentID, &item.Status, &item.Queue, &item.Priority, &item.AssignedTo,
		&claimedAt, &claimExpiresAt, &completedAt, &item.CreatedAt, &item.UpdatedAt,
		&item.Incident.ID, &item.Incident.SubjectUserID, &item.Incident.SubjectName,
		&item.Incident.Status, &item.Incident.Severity, &item.Incident.EvidenceStrength, &item.Incident.ReasonCode, &item.Incident.Summary,
		&item.Incident.SignalCount, &item.Incident.UniqueReporterCount, &item.Incident.AssignedTo,
		&incidentWatchUntil, &item.Incident.LatestSignalAt, &incidentResolvedAt, &item.Incident.ResolvedBy,
		&item.Incident.ResolutionNote, &item.Incident.CreatedAt, &item.Incident.UpdatedAt,
	)
	if claimedAt != nil {
		item.ClaimedAt = *claimedAt
	}
	if claimExpiresAt != nil {
		item.ClaimExpiresAt = *claimExpiresAt
	}
	if completedAt != nil {
		item.CompletedAt = *completedAt
	}
	if incidentWatchUntil != nil {
		item.Incident.WatchUntil = *incidentWatchUntil
	}
	if incidentResolvedAt != nil {
		item.Incident.ResolvedAt = *incidentResolvedAt
	}
	return item, err
}

func scanIncident(row interface{ Scan(...any) error }) (ModerationIncidentSummary, error) {
	var item ModerationIncidentSummary
	var watchUntil, resolvedAt *time.Time
	err := row.Scan(
		&item.ID, &item.SubjectUserID, &item.SubjectName, &item.Status, &item.Severity,
		&item.EvidenceStrength, &item.ReasonCode, &item.Summary, &item.SignalCount,
		&item.UniqueReporterCount, &item.AssignedTo, &watchUntil, &item.LatestSignalAt,
		&resolvedAt, &item.ResolvedBy, &item.ResolutionNote, &item.CreatedAt, &item.UpdatedAt,
	)
	if watchUntil != nil {
		item.WatchUntil = *watchUntil
	}
	if resolvedAt != nil {
		item.ResolvedAt = *resolvedAt
	}
	return item, err
}

func (s *pgStore) listTasksForIncident(ctx context.Context, incidentID int64) ([]ModerationReviewTaskSummary, error) {
	rows, err := s.pool.Query(ctx, reviewTaskSelectQuery("t.incident_id = $2"), 20, incidentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ModerationReviewTaskSummary{}
	for rows.Next() {
		item, err := scanReviewTask(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *pgStore) listSignalsForIncident(ctx context.Context, incidentID int64) ([]ModerationSignalSummary, error) {
	return s.listSignals(ctx, "exists(select 1 from moderation_incident_signals link where link.signal_id = s.id and link.incident_id = $2)", []any{100, incidentID})
}

func (s *pgStore) listSignals(ctx context.Context, where string, args []any) ([]ModerationSignalSummary, error) {
	rows, err := s.pool.Query(ctx, `
		select
			s.id, s.subject_user_id::text, coalesce(nullif(subject.display_name, ''), s.subject_user_id::text),
			s.signal_type, s.source, s.severity, s.evidence_strength, coalesce(s.detector_key, ''),
			coalesce(s.detector_version, ''), s.reason_code, s.score, s.recommended_queue,
			coalesce(s.reporter_user_id::text, ''), coalesce(nullif(reporter.display_name, ''), s.reporter_user_id::text, ''),
			coalesce(s.match_id::text, ''), s.payload_json::text, s.occurred_at, s.created_at
		from moderation_signals s
		left join users subject on subject.id = s.subject_user_id
		left join users reporter on reporter.id = s.reporter_user_id
		where `+where+`
		order by s.occurred_at desc, s.id desc
		limit $1
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ModerationSignalSummary{}
	for rows.Next() {
		var item ModerationSignalSummary
		var payload string
		if err := rows.Scan(
			&item.ID, &item.SubjectUserID, &item.SubjectName, &item.SignalType, &item.Source,
			&item.Severity, &item.EvidenceStrength, &item.DetectorKey, &item.DetectorVersion,
			&item.ReasonCode, &item.Score, &item.RecommendedQueue, &item.ReporterUserID,
			&item.ReporterName, &item.MatchID, &payload, &item.OccurredAt, &item.CreatedAt,
		); err != nil {
			return nil, err
		}
		item.Payload = json.RawMessage(payload)
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *pgStore) listIncidentMatches(ctx context.Context, incidentID int64) ([]ModerationMatchSummary, error) {
	rows, err := s.pool.Query(ctx, `
		select distinct s.match_id::text
		from moderation_signals s
		join moderation_incident_signals link on link.signal_id = s.id
		where link.incident_id = $1 and s.match_id is not null
		order by s.match_id::text
		limit 20
	`, incidentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ModerationMatchSummary{}
	for rows.Next() {
		var matchID string
		if err := rows.Scan(&matchID); err != nil {
			return nil, err
		}
		out = append(out, ModerationMatchSummary{MatchID: matchID})
	}
	return out, rows.Err()
}

func (s *pgStore) listVerdicts(ctx context.Context, incidentID int64) ([]ModerationVerdictSummary, error) {
	rows, err := s.pool.Query(ctx, `
		select
			v.id, v.incident_id, coalesce(v.task_id, 0), coalesce(v.actor_user_id::text, ''),
			coalesce(nullif(u.display_name, ''), v.actor_user_id::text, ''), v.verdict,
			v.reason_code, coalesce(v.note, ''), coalesce(v.enforcement_action, ''),
			v.metadata::text, v.created_at
		from moderation_verdicts v
		left join users u on u.id = v.actor_user_id
		where v.incident_id = $1
		order by v.created_at desc
	`, incidentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ModerationVerdictSummary{}
	for rows.Next() {
		var item ModerationVerdictSummary
		var metadata string
		if err := rows.Scan(&item.ID, &item.IncidentID, &item.TaskID, &item.ActorUserID, &item.ActorName, &item.Verdict, &item.ReasonCode, &item.Note, &item.EnforcementAction, &metadata, &item.CreatedAt); err != nil {
			return nil, err
		}
		item.Metadata = json.RawMessage(metadata)
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *pgStore) listAuditLog(ctx context.Context, incidentID int64) ([]ModerationAuditLogEntry, error) {
	rows, err := s.pool.Query(ctx, `
		select id, coalesce(incident_id, 0), coalesce(task_id, 0), coalesce(actor_user_id::text, ''),
			event_type, coalesce(reason_code, ''), coalesce(body, ''), metadata::text, created_at
		from moderation_audit_log
		where incident_id = $1
		order by created_at desc, id desc
		limit 100
	`, incidentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ModerationAuditLogEntry{}
	for rows.Next() {
		var item ModerationAuditLogEntry
		var metadata string
		if err := rows.Scan(&item.ID, &item.IncidentID, &item.TaskID, &item.ActorUserID, &item.EventType, &item.ReasonCode, &item.Body, &metadata, &item.CreatedAt); err != nil {
			return nil, err
		}
		item.Metadata = json.RawMessage(metadata)
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *pgStore) listReporterStateForIncident(ctx context.Context, incidentID int64) ([]ModerationReporterState, error) {
	rows, err := s.pool.Query(ctx, `
		select r.user_id::text, r.reports_submitted, r.reports_useful, r.reports_dismissed,
			r.reports_inconclusive, r.reports_abusive, r.report_weight, r.muted_until, r.updated_at
		from moderation_reporter_state r
		where exists(
			select 1
			from moderation_signals s
			join moderation_incident_signals link on link.signal_id = s.id
			where link.incident_id = $1 and s.reporter_user_id = r.user_id
		)
		order by r.updated_at desc
	`, incidentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ModerationReporterState{}
	for rows.Next() {
		var item ModerationReporterState
		var mutedUntil *time.Time
		if err := rows.Scan(&item.UserID, &item.ReportsSubmitted, &item.ReportsUseful, &item.ReportsDismissed, &item.ReportsInconclusive, &item.ReportsAbusive, &item.ReportWeight, &mutedUntil, &item.UpdatedAt); err != nil {
			return nil, err
		}
		if mutedUntil != nil {
			item.MutedUntil = *mutedUntil
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *pgStore) listIncidentsForSubject(ctx context.Context, userID string, limit int) ([]ModerationIncidentSummary, error) {
	rows, err := s.pool.Query(ctx, incidentSelectQuery("i.subject_user_id = $1")+` order by i.latest_signal_at desc limit $2`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ModerationIncidentSummary{}
	for rows.Next() {
		item, err := scanIncident(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
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
	case "cheating", "boosting":
		return "medium"
	case "harassment":
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

func incidentSummary(reason string) string {
	reason = strings.ReplaceAll(strings.TrimSpace(reason), "_", " ")
	if reason == "" {
		return "Moderation review needed."
	}
	return "Review " + reason + "."
}

func priorityForSeverity(severity string) string {
	switch severity {
	case "critical":
		return "urgent"
	case "high":
		return "high"
	case "medium":
		return "medium"
	default:
		return "low"
	}
}

func incidentStatusForVerdict(verdict string) string {
	switch verdict {
	case "confirmed":
		return "actioned"
	case "dismissed", "abusive_report":
		return "dismissed"
	case "inconclusive":
		return "inconclusive"
	case "duplicate":
		return "duplicate"
	case "watch":
		return "watching"
	default:
		return "open"
	}
}
