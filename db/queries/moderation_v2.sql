-- name: CreatePlayerReportSignal :one
INSERT INTO moderation_signals(subject_user_id,signal_type,source,severity,evidence_strength,reason_code,score,recommended_queue,reporter_user_id,match_id,payload_json,occurred_at) VALUES(sqlc.arg(subject_user_id)::uuid,sqlc.arg(signal_type),'player_report',sqlc.arg(severity),'limited',sqlc.arg(reason_code),sqlc.arg(score),true,sqlc.arg(reporter_user_id)::uuid,sqlc.arg(match_id)::uuid,sqlc.arg(payload_json)::jsonb,now()) ON CONFLICT (match_id,reporter_user_id,subject_user_id) WHERE source='player_report' AND reporter_user_id IS NOT NULL AND match_id IS NOT NULL DO NOTHING RETURNING id;

-- name: GetSignalNotificationPayload :one
SELECT s.id AS signal_id,s.subject_user_id::text AS subject_user_id,coalesce(nullif(u.display_name,''),s.subject_user_id::text) AS subject_display_name,s.severity,s.evidence_strength,s.reason_code,s.occurred_at FROM moderation_signals s LEFT JOIN users u ON u.id=s.subject_user_id WHERE s.id=sqlc.arg(signal_id);

-- name: ListModerationLog :many
SELECT l.id AS log_id, coalesce(l.subject_user_id::text, '') AS subject_user_id, coalesce(nullif(subject.display_name, ''), l.subject_user_id::text, '') AS subject_display_name, coalesce(l.actor_user_id::text, '') AS actor_user_id, coalesce(nullif(actor.display_name, ''), l.actor_user_id::text, '') AS actor_display_name, l.action, coalesce(l.reason, '') AS reason, l.expires_at, l.signal_ids, l.metadata::text AS metadata, l.created_at
FROM moderation_log l LEFT JOIN users subject ON subject.id=l.subject_user_id LEFT JOIN users actor ON actor.id=l.actor_user_id
WHERE (sqlc.narg(subject_user_id)::uuid IS NULL OR l.subject_user_id=sqlc.narg(subject_user_id)::uuid) ORDER BY l.created_at DESC,l.id DESC LIMIT sqlc.arg(row_limit);

-- name: ListModerationSignals :many
SELECT s.id AS signal_id, s.subject_user_id::text AS subject_user_id, coalesce(nullif(subject.display_name, ''), s.subject_user_id::text) AS subject_display_name, s.signal_type, s.source, s.severity, s.evidence_strength, coalesce(s.detector_key, '') AS detector_key, coalesce(s.detector_version, '') AS detector_version, s.reason_code, s.score, s.recommended_queue, coalesce(s.reporter_user_id::text, '') AS reporter_user_id, coalesce(nullif(reporter.display_name, ''), s.reporter_user_id::text, '') AS reporter_display_name, coalesce(s.match_id::text, '') AS match_id, s.payload_json::text AS payload_json, s.occurred_at, s.created_at, s.reviewed_at, coalesce(s.reviewed_by::text, '') AS reviewed_by, coalesce(s.outcome, '') AS outcome
FROM moderation_signals s LEFT JOIN users subject ON subject.id=s.subject_user_id LEFT JOIN users reporter ON reporter.id=s.reporter_user_id
WHERE (sqlc.narg(subject_user_id)::uuid IS NULL OR s.subject_user_id=sqlc.narg(subject_user_id)::uuid) ORDER BY s.occurred_at DESC,s.id DESC LIMIT sqlc.arg(row_limit);

-- name: PlayerParticipated :one
SELECT EXISTS(SELECT 1 FROM match_players WHERE match_id=sqlc.arg(match_id)::uuid AND user_id=sqlc.arg(user_id)::uuid);

-- name: ReporterMuted :one
SELECT coalesce(report_muted_at is not null and (report_mute_expires_at is null or report_mute_expires_at > now()),false) FROM users WHERE id=$1::uuid;

-- name: UpsertRiskEngineSignal :one
INSERT INTO moderation_signals(
    subject_user_id, signal_type, source, severity, evidence_strength,
    detector_key, detector_version, reason_code, score, recommended_queue,
    match_id, payload_json, occurred_at
)
VALUES(
    sqlc.arg(subject_user_id)::uuid, sqlc.arg(signal_type), 'risk_engine', sqlc.arg(severity), sqlc.arg(evidence_strength),
    NULLIF(sqlc.arg(detector_key)::text, ''), NULLIF(sqlc.arg(detector_version)::text, ''), sqlc.arg(reason_code), sqlc.arg(score), sqlc.arg(recommended_queue),
    sqlc.arg(match_id)::uuid, sqlc.arg(payload_json)::jsonb, sqlc.arg(occurred_at)
)
ON CONFLICT (subject_user_id, coalesce(match_id, '00000000-0000-0000-0000-000000000000'::uuid), coalesce(detector_key, ''), coalesce(detector_version, ''), reason_code)
    WHERE source = 'risk_engine'
DO UPDATE SET
    severity = CASE WHEN array_position(ARRAY['low','medium','high','critical'], excluded.severity) > array_position(ARRAY['low','medium','high','critical'], moderation_signals.severity) THEN excluded.severity ELSE moderation_signals.severity END,
    evidence_strength = CASE WHEN array_position(ARRAY['weak','limited','substantial','strong'], excluded.evidence_strength) > array_position(ARRAY['weak','limited','substantial','strong'], moderation_signals.evidence_strength) THEN excluded.evidence_strength ELSE moderation_signals.evidence_strength END,
    score = greatest(moderation_signals.score, excluded.score),
    recommended_queue = moderation_signals.recommended_queue OR excluded.recommended_queue,
    payload_json = excluded.payload_json,
    occurred_at = greatest(moderation_signals.occurred_at, excluded.occurred_at)
RETURNING id;
