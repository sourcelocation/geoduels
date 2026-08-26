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

func (s *pgStore) IssueEloRefundsForCheater(userID string, lookback time.Duration) (EloRefundSummary, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return EloRefundSummary{}, errors.New("userID required")
	}
	if lookback <= 0 {
		lookback = 24 * time.Hour
	}
	since := time.Now().Add(-lookback)
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return EloRefundSummary{}, err
	}
	defer tx.Rollback(ctx)
	summary, err := issueCurrentMMRRefundsForCheater(ctx, tx, userID, "cheating_verdict", since)
	if err != nil {
		return EloRefundSummary{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return EloRefundSummary{}, err
	}
	return summary, nil
}

func (s *pgStore) BanPlayerForCheating(userID, reason, actorUserID string) (CheatingBanSummary, error) {
	userID = strings.TrimSpace(userID)
	reason = strings.TrimSpace(reason)
	actorUserID = strings.TrimSpace(actorUserID)
	if userID == "" {
		return CheatingBanSummary{}, errors.New("userID required")
	}
	if reason == "" {
		reason = "cheating"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return CheatingBanSummary{}, err
	}
	defer tx.Rollback(ctx)

	var registrationIP string
	tag, err := tx.Exec(ctx, `
		update users
		set banned_at = coalesce(banned_at, now()),
			ban_reason = $2,
			ban_expires_at = null
		where id = $1
	`, userID, reason)
	if err != nil {
		return CheatingBanSummary{}, err
	}
	if tag.RowsAffected() == 0 {
		return CheatingBanSummary{}, errors.New("user not found")
	}
	if err := banUserOAuthIdentities(ctx, tx, userID, reason, actorUserID); err != nil {
		return CheatingBanSummary{}, err
	}
	if err := tx.QueryRow(ctx, `
		select coalesce(registration_ip_address, '')
		from users
		where id = $1
	`, userID).Scan(&registrationIP); err != nil {
		return CheatingBanSummary{}, err
	}

	summary := CheatingBanSummary{UserID: userID, Reason: reason}
	refunds, err := issueCurrentMMRRefundsForCheater(ctx, tx, userID, reason, time.Time{})
	if err != nil {
		return CheatingBanSummary{}, err
	}
	summary.Refunds = refunds

	var logID int64
	if err := tx.QueryRow(ctx, `
		insert into moderation_log(subject_user_id, actor_user_id, action, reason, metadata)
		values($1, nullif($2, '')::uuid, 'permanent_ban', nullif($3, ''), jsonb_build_object('refundsIssued', $4::int, 'totalRefunded', $5::int))
		returning id
	`, userID, actorUserID, reason, summary.Refunds.RefundsIssued, summary.Refunds.TotalRefunded).Scan(&logID); err != nil {
		return CheatingBanSummary{}, err
	}
	if err := notifyAccountEnforcement(ctx, tx, userID, "permanent_ban", reason, logID, nil); err != nil {
		return CheatingBanSummary{}, err
	}
	if err := notifyReportersOfBan(ctx, tx, userID, "permanent_ban", logID); err != nil {
		return CheatingBanSummary{}, err
	}

	if registrationIP != "" {
		var relatedCheater bool
		if err := tx.QueryRow(ctx, `
			select exists(
				select 1
				from users
				where id <> $1
					and registration_ip_address = $2
					and banned_at >= now() - interval '7 days'
					and (
						lower(coalesce(ban_reason, '')) like '%cheat%'
						or lower(coalesce(ban_reason, '')) like 'auto_%'
					)
			)
		`, userID, registrationIP).Scan(&relatedCheater); err != nil {
			return CheatingBanSummary{}, err
		}
		if relatedCheater {
			if _, err := tx.Exec(ctx, `
				insert into ip_signup_bans(ip_address, reason, created_by, created_at, revoked_at)
				values($1, $2, nullif($3, '')::uuid, now(), null)
				on conflict (ip_address) do update set
					reason = excluded.reason,
					created_by = excluded.created_by,
					created_at = now(),
					revoked_at = null
			`, registrationIP, "Automatic signup ban: repeated cheating bans from registration IP", actorUserID); err != nil {
				return CheatingBanSummary{}, err
			}
			summary.IPSignupBanned = true
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return CheatingBanSummary{}, err
	}
	return summary, nil
}

func issueCurrentMMRRefundsForCheater(ctx context.Context, tx pgx.Tx, cheaterUserID, reason string, since time.Time) (EloRefundSummary, error) {
	var sinceArg any
	if !since.IsZero() {
		sinceArg = since
	}
	seasonID, err := activeSeasonIDTx(ctx, tx)
	if err != nil {
		return EloRefundSummary{}, err
	}
	rows, err := tx.Query(ctx, `
		with candidate_matches as (
			select
				h.match_id,
				h.ended_at,
				h.winner_user_id,
				opponent.user_id as opponent_user_id,
				cheater.mmr as cheater_mmr,
				coalesce(cheater.rating_rd, $3) as cheater_rd,
				opponent.final_ranked_delta as original_delta
			from match_history h
			join match_players cheater on cheater.match_id = h.match_id and cheater.user_id = $1
			join match_players opponent on opponent.match_id = h.match_id and opponent.user_id <> $1
			left join parties l on l.active_match_id = h.match_id
				or l.started_match_id = h.match_id
				or l.last_match_id = h.match_id
			where h.mode = $2
				and h.winner_user_id = $1
				and ($4::timestamptz is null or h.ended_at >= $4)
				and h.ranked
				and l.id is null
		)
		select
			match_id,
			opponent_user_id,
			cheater_mmr,
			cheater_rd,
			coalesce(original_delta, 0)
		from candidate_matches
		where original_delta is not null
		order by ended_at asc, match_id asc
	`, cheaterUserID, modeDuel, initialRatingRD, sinceArg)
	if err != nil {
		return EloRefundSummary{}, err
	}
	defer rows.Close()
	type refundCandidate struct {
		matchID       string
		opponentID    string
		cheaterMMR    int
		cheaterRD     float64
		originalDelta int
	}
	candidates := []refundCandidate{}
	for rows.Next() {
		var item refundCandidate
		if err := rows.Scan(&item.matchID, &item.opponentID, &item.cheaterMMR, &item.cheaterRD, &item.originalDelta); err != nil {
			return EloRefundSummary{}, err
		}
		if item.originalDelta < 0 {
			candidates = append(candidates, item)
		}
	}
	if err := rows.Err(); err != nil {
		return EloRefundSummary{}, err
	}
	rows.Close()

	var summary EloRefundSummary
	for _, item := range candidates {
		var current RatingState
		if err := tx.QueryRow(ctx, `
			select mmr, rd, updated_at
			from ranks
			where user_id = $1 and mode = $2 and season_id = $3
			for update
			`, item.opponentID, modeDuel, seasonID).Scan(&current.MMR, &current.RD, &current.UpdatedAt); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				continue
			}
			return EloRefundSummary{}, err
		}
		now := time.Now()
		victimWin, _ := CalculateDuelRatingUpdates(current, RatingState{MMR: item.cheaterMMR, RD: item.cheaterRD, UpdatedAt: now}, "p1", now)
		refundDelta := victimWin.Delta
		if refundDelta <= 0 {
			continue
		}
		originalLoss := -item.originalDelta
		if refundDelta > originalLoss {
			refundDelta = originalLoss
		}
		before := current.MMR
		after := clampRankedMMR(before + refundDelta)
		refundDelta = after - before
		if refundDelta <= 0 {
			continue
		}
		tag, err := tx.Exec(ctx, `
			insert into elo_refunds(
				user_id, match_id, cheater_user_id, original_delta, refund_delta,
				victim_mmr_before, victim_mmr_after, computed_refund_delta, reason, created_by_reason
			)
			values($1, $2, $3, $4, $5, $6, $7, $5, 'cheating_verdict', $8)
			on conflict (user_id, match_id, cheater_user_id) do nothing
		`, item.opponentID, item.matchID, cheaterUserID, item.originalDelta, refundDelta, before, after, reason)
		if err != nil {
			return EloRefundSummary{}, err
		}
		if tag.RowsAffected() == 0 {
			continue
		}
		var notificationID int64
		payload := map[string]any{
			"refundDelta":   refundDelta,
			"matchId":       item.matchID,
			"cheaterUserId": cheaterUserID,
			"reason":        reason,
			"mmrBefore":     before,
			"mmrAfter":      after,
		}
		if err := upsertUserNotification(ctx, tx, item.opponentID, "mmr_refund", fmt.Sprintf("mmr_refund:%s:%s:%s", item.opponentID, item.matchID, cheaterUserID), payload, &notificationID); err != nil {
			return EloRefundSummary{}, err
		}
		if _, err := tx.Exec(ctx, `
			update elo_refunds
			set notification_id = $4
			where user_id = $1 and match_id = $2 and cheater_user_id = $3
		`, item.opponentID, item.matchID, cheaterUserID, notificationID); err != nil {
			return EloRefundSummary{}, err
		}
		if _, err := tx.Exec(ctx, `
			update ranks
			set mmr = $4,
				updated_at = now()
			where user_id = $1
				and mode = $2
				and season_id = $3
			`, item.opponentID, modeDuel, seasonID, after); err != nil {
			return EloRefundSummary{}, err
		}
		summary.RefundsIssued++
		summary.TotalRefunded += refundDelta
	}
	return summary, nil
}

func (s *pgStore) EvaluateAutoCheatBansForMatch(matchID string) error {
	matchID = strings.TrimSpace(matchID)
	if matchID == "" {
		return nil
	}
	client, ok := riskEngineClientFromEnv()
	if !ok {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	req, err := s.riskEngineAnalyzeRequest(ctx, matchID)
	if err != nil {
		return err
	}
	if len(req.Players) == 0 {
		return nil
	}
	resp, err := client.analyze(ctx, req)
	if err != nil {
		return err
	}
	for _, signal := range resp.Signals {
		if strings.TrimSpace(signal.SubjectUserID) == "" {
			continue
		}
		if err := s.recordRiskEngineSignal(ctx, matchID, signal); err != nil {
			return err
		}
	}
	return nil
}

func (s *pgStore) riskEngineAnalyzeRequest(ctx context.Context, matchID string) (riskEngineAnalyzeRequest, error) {
	seasonID, err := s.activeSeasonID(ctx)
	if err != nil {
		return riskEngineAnalyzeRequest{}, err
	}
	rows, err := s.pool.Query(ctx, `
		select distinct user_id
		from ranked_guess_events
		where match_id = $1
	`, matchID)
	if err != nil {
		return riskEngineAnalyzeRequest{}, err
	}
	userIDs := []string{}
	for rows.Next() {
		var userID string
		if err := rows.Scan(&userID); err != nil {
			rows.Close()
			return riskEngineAnalyzeRequest{}, err
		}
		userIDs = append(userIDs, userID)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return riskEngineAnalyzeRequest{}, err
	}
	rows.Close()
	req := riskEngineAnalyzeRequest{
		MatchID:      matchID,
		FactsVersion: "match-facts/2026-01",
		GeneratedAt:  time.Now().UTC(),
		Players:      []riskEnginePlayerHistory{},
	}
	for _, userID := range userIDs {
		events, err := s.riskEngineRecentGuessEvents(ctx, userID)
		if err != nil {
			return riskEngineAnalyzeRequest{}, err
		}
		currentRating, rankedGames, err := s.riskEnginePlayerContext(ctx, userID, seasonID)
		if err != nil {
			return riskEngineAnalyzeRequest{}, err
		}
		req.Players = append(req.Players, riskEnginePlayerHistory{
			UserID:        userID,
			CurrentRating: currentRating,
			RankedGames:   rankedGames,
			Events:        events,
		})
	}
	return req, nil
}

func (s *pgStore) riskEngineRecentGuessEvents(ctx context.Context, userID string) ([]riskEngineGuessEvent, error) {
	rows, err := s.pool.Query(ctx, `
		select match_id, round_number, score, guess_ms, evidence, occurred_at
		from ranked_guess_events
		where user_id = $1
		order by occurred_at desc, round_number desc
		limit 50
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	events := []riskEngineGuessEvent{}
	for rows.Next() {
		var event riskEngineGuessEvent
		if err := rows.Scan(&event.MatchID, &event.RoundNumber, &event.Score, &event.GuessMS, &event.Evidence, &event.OccurredAt); err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, rows.Err()
}

func (s *pgStore) riskEnginePlayerContext(ctx context.Context, userID, seasonID string) (int, int, error) {
	var rating, games int
	err := s.pool.QueryRow(ctx, `
		select
			coalesce(r.mmr, $4)::int,
			coalesce(rs.games_played, 0)::int
		from users u
		left join ranks r on r.user_id = u.id and r.mode = $2 and r.season_id = $3
		left join ranked_stats rs on rs.user_id = u.id and rs.mode = $2 and rs.season_id = $3
		where u.id = $1
	`, userID, modeDuel, seasonID, initialMMR).Scan(&rating, &games)
	if err != nil {
		return 0, 0, err
	}
	return rating, games, nil
}

func (s *pgStore) recordRiskEngineSignal(ctx context.Context, matchID string, signal riskEngineSignal) error {
	payload, err := json.Marshal(signal.Payload)
	if err != nil {
		return err
	}
	occurredAt := signal.OccurredAt
	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var signalID int64
	err = tx.QueryRow(ctx, `
		insert into moderation_signals(
			subject_user_id, signal_type, source, severity, evidence_strength,
			detector_key, detector_version, reason_code, score, recommended_queue,
			match_id, payload_json, occurred_at
		)
		values(
			$1, $2, 'risk_engine', $3, $4,
			nullif($5, ''), nullif($6, ''), $7, $8, $9,
			$10, $11::jsonb, $12
		)
		on conflict (subject_user_id, coalesce(match_id, '00000000-0000-0000-0000-000000000000'::uuid), coalesce(detector_key, ''), coalesce(detector_version, ''), reason_code)
			where source = 'risk_engine'
		do update set
			severity = case when array_position(array['low','medium','high','critical'], excluded.severity) > array_position(array['low','medium','high','critical'], moderation_signals.severity) then excluded.severity else moderation_signals.severity end,
			evidence_strength = case when array_position(array['weak','limited','substantial','strong'], excluded.evidence_strength) > array_position(array['weak','limited','substantial','strong'], moderation_signals.evidence_strength) then excluded.evidence_strength else moderation_signals.evidence_strength end,
			score = greatest(moderation_signals.score, excluded.score),
			recommended_queue = moderation_signals.recommended_queue or excluded.recommended_queue,
			payload_json = excluded.payload_json,
			occurred_at = greatest(moderation_signals.occurred_at, excluded.occurred_at)
		returning id
	`, signal.SubjectUserID, normalizeRiskSignalType(signal.SignalType), normalizeRiskSeverity(signal.Severity), normalizeRiskEvidenceStrength(signal.EvidenceStrength), signal.DetectorKey, signal.DetectorVersion, nonempty(signal.ReasonCode, "risk_engine_signal"), signal.Score, signal.RecommendedQueue, matchID, string(payload), occurredAt).Scan(&signalID)
	if err != nil {
		return err
	}
	if riskSignalQueues(signal.Severity) || signal.RecommendedQueue {
		if err := enqueueSignalNotification(ctx, tx, signalID); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (s *pgStore) AddSignupIPBan(ipAddress, reason, createdBy string) error {
	ipAddress = strings.TrimSpace(ipAddress)
	if ipAddress == "" {
		return errors.New("ip address required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	_, err := s.pool.Exec(ctx, `
		insert into ip_signup_bans(ip_address, reason, created_by, created_at, revoked_at)
		values($1, nullif($2, ''), nullif($3, '')::uuid, now(), null)
		on conflict (ip_address) do update set
			reason = excluded.reason,
			created_by = excluded.created_by,
			created_at = now(),
			revoked_at = null
	`, ipAddress, strings.TrimSpace(reason), strings.TrimSpace(createdBy))
	return err
}

func (s *pgStore) RemoveSignupIPBan(ipAddress string) error {
	ipAddress = strings.TrimSpace(ipAddress)
	if ipAddress == "" {
		return errors.New("ip address required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	_, err := s.pool.Exec(ctx, `
		update ip_signup_bans
		set revoked_at = coalesce(revoked_at, now())
		where ip_address = $1
	`, ipAddress)
	return err
}

func (s *pgStore) ListSignupIPBans(limit int) ([]SignupIPBan, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	rows, err := s.pool.Query(ctx, `
		select id, ip_address, coalesce(reason, ''), coalesce(created_by::text, ''), created_at
		from ip_signup_bans
		where revoked_at is null
		order by created_at desc
		limit $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]SignupIPBan, 0, limit)
	for rows.Next() {
		var item SignupIPBan
		if err := rows.Scan(&item.ID, &item.IPAddress, &item.Reason, &item.CreatedBy, &item.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *pgStore) IsSignupIPBanned(ipAddress string) (bool, error) {
	ipAddress = strings.TrimSpace(ipAddress)
	if ipAddress == "" {
		return false, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	var exists bool
	if err := s.pool.QueryRow(ctx, `
		select exists(
			select 1 from ip_signup_bans
			where ip_address = $1 and revoked_at is null
		)
	`, ipAddress).Scan(&exists); err != nil {
		return false, err
	}
	return exists, nil
}
