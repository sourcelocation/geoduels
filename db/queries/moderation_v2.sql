-- name: CreatePlayerReportSignal :one
INSERT INTO moderation_signals(subject_user_id,signal_type,source,severity,evidence_strength,reason_code,score,recommended_queue,reporter_user_id,match_id,payload_json,occurred_at) VALUES($1::uuid,$2,'player_report',$3,'limited',$4,$5,true,$6::uuid,$7::uuid,$8::jsonb,now()) ON CONFLICT (match_id,reporter_user_id,subject_user_id) WHERE source='player_report' AND reporter_user_id IS NOT NULL AND match_id IS NOT NULL DO NOTHING RETURNING id;

-- name: GetSignalNotificationPayload :one
SELECT s.id,s.subject_user_id::text,coalesce(nullif(u.display_name,''),s.subject_user_id::text),s.severity,s.evidence_strength,s.reason_code,s.occurred_at FROM moderation_signals s LEFT JOIN users u ON u.id=s.subject_user_id WHERE s.id=$1;

-- name: ListModerationLog :many
SELECT l.id, coalesce(l.subject_user_id::text, ''), coalesce(nullif(subject.display_name, ''), l.subject_user_id::text, ''), coalesce(l.actor_user_id::text, ''), coalesce(nullif(actor.display_name, ''), l.actor_user_id::text, ''), l.action, coalesce(l.reason, ''), l.expires_at, l.signal_ids, l.metadata::text, l.created_at
FROM moderation_log l LEFT JOIN users subject ON subject.id=l.subject_user_id LEFT JOIN users actor ON actor.id=l.actor_user_id
WHERE ($1::uuid IS NULL OR l.subject_user_id=$1::uuid) ORDER BY l.created_at DESC,l.id DESC LIMIT $2;

-- name: ListModerationSignals :many
SELECT s.id, s.subject_user_id::text, coalesce(nullif(subject.display_name, ''), s.subject_user_id::text), s.signal_type, s.source, s.severity, s.evidence_strength, coalesce(s.detector_key, ''), coalesce(s.detector_version, ''), s.reason_code, s.score, s.recommended_queue, coalesce(s.reporter_user_id::text, ''), coalesce(nullif(reporter.display_name, ''), s.reporter_user_id::text, ''), coalesce(s.match_id::text, ''), s.payload_json::text, s.occurred_at, s.created_at, s.reviewed_at, coalesce(s.reviewed_by::text, ''), coalesce(s.outcome, '')
FROM moderation_signals s LEFT JOIN users subject ON subject.id=s.subject_user_id LEFT JOIN users reporter ON reporter.id=s.reporter_user_id
WHERE ($1::uuid IS NULL OR s.subject_user_id=$1::uuid) ORDER BY s.occurred_at DESC,s.id DESC LIMIT $2;

-- name: PlayerParticipated :one
SELECT EXISTS(SELECT 1 FROM match_players WHERE match_id=$1::uuid AND user_id=$2::uuid);

-- name: ReporterMuted :one
SELECT coalesce(report_muted_at is not null and (report_mute_expires_at is null or report_mute_expires_at > now()),false) FROM users WHERE id=$1::uuid;

-- name: UpsertRiskEngineSignal :one
INSERT INTO moderation_signals(
    subject_user_id, signal_type, source, severity, evidence_strength,
    detector_key, detector_version, reason_code, score, recommended_queue,
    match_id, payload_json, occurred_at
)
VALUES(
    $1::uuid, $2, 'risk_engine', $3, $4,
    NULLIF($5, ''), NULLIF($6, ''), $7, $8, $9,
    $10::uuid, $11::jsonb, $12
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
