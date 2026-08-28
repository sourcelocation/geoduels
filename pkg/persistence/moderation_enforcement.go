package persistence

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	db "geoduels/pkg/persistence/sqlc/db"
)

func (s *DB) IssueEloRefundsForCheater(userID string, lookback time.Duration) (EloRefundSummary, error) {
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

func (s *DB) BanPlayerForCheating(userID, reason, actorUserID string) (CheatingBanSummary, error) {
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
	q := db.New(tx)
	uid, err := profileUUID(userID)
	if err != nil {
		return CheatingBanSummary{}, err
	}

	tag, err := q.BanUserForCheating(ctx, db.BanUserForCheatingParams{ID: uid, BanReason: pgtype.Text{String: reason, Valid: true}})
	if err != nil {
		return CheatingBanSummary{}, err
	}
	if tag == 0 {
		return CheatingBanSummary{}, errors.New("user not found")
	}
	if err := q.BanUserOAuthIdentities(ctx, db.BanUserOAuthIdentitiesParams{BannedUserID: uid, Reason: reason, CreatedBy: actorUserID}); err != nil {
		return CheatingBanSummary{}, err
	}
	registrationIP, err := q.GetUserRegistrationIP(ctx, uid)
	if err != nil {
		return CheatingBanSummary{}, err
	}

	summary := CheatingBanSummary{UserID: userID, Reason: reason}
	refunds, err := issueCurrentMMRRefundsForCheater(ctx, tx, userID, reason, time.Time{})
	if err != nil {
		return CheatingBanSummary{}, err
	}
	summary.Refunds = refunds

	metadata, _ := json.Marshal(map[string]any{"refundsIssued": refunds.RefundsIssued, "totalRefunded": refunds.TotalRefunded})
	logID, err := q.InsertModerationLog(ctx, db.InsertModerationLogParams{
		SubjectUserID: uid,
		ActorUserID:   actorUserID,
		Action:        db.GdModerationLogActionPermanentBan,
		Reason:        reason,
		Metadata:      metadata,
	})
	if err != nil {
		return CheatingBanSummary{}, err
	}
	if err := notifyAccountEnforcement(ctx, tx, userID, "permanent_ban", reason, logID, nil); err != nil {
		return CheatingBanSummary{}, err
	}
	if err := notifyReportersOfBan(ctx, tx, userID, "permanent_ban", logID); err != nil {
		return CheatingBanSummary{}, err
	}

	if registrationIP != "" {
		relatedCheater, err := q.HasRelatedCheaterFromIP(ctx, db.HasRelatedCheaterFromIPParams{ID: uid, RegistrationIpAddress: pgtype.Text{String: registrationIP, Valid: true}})
		if err != nil {
			return CheatingBanSummary{}, err
		}
		if relatedCheater {
			if err := q.InsertIPSignupBan(ctx, db.InsertIPSignupBanParams{
				IpAddress: registrationIP,
				Reason:    "Automatic signup ban: repeated cheating bans from registration IP",
				CreatedBy: actorUserID,
			}); err != nil {
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
	q := db.New(tx)
	cheaterUUID, err := profileUUID(cheaterUserID)
	if err != nil {
		return EloRefundSummary{}, err
	}
	sinceArg := pgtype.Timestamptz{}
	if !since.IsZero() {
		sinceArg = pgtype.Timestamptz{Time: since, Valid: true}
	}
	seasonID, err := activeSeasonIDTx(ctx, tx)
	if err != nil {
		return EloRefundSummary{}, err
	}
	rows, err := q.ListCheaterRefundCandidates(ctx, db.ListCheaterRefundCandidatesParams{
		UserID:          cheaterUUID,
		Mode:            db.GdMatchMode(modeDuel),
		DefaultRatingRd: initialRatingRD,
		Since:           sinceArg,
	})
	if err != nil {
		return EloRefundSummary{}, err
	}
	type refundCandidate struct {
		matchID       string
		opponentID    string
		cheaterMMR    int
		cheaterRD     float64
		originalDelta int
	}
	candidates := []refundCandidate{}
	for _, row := range rows {
		item := refundCandidate{
			matchID:       row.MatchID,
			opponentID:    row.OpponentUserID,
			cheaterMMR:    int(row.CheaterMmr),
			cheaterRD:     row.CheaterRd.Float64,
			originalDelta: int(row.OriginalDelta),
		}
		if item.originalDelta < 0 {
			candidates = append(candidates, item)
		}
	}

	var summary EloRefundSummary
	for _, item := range candidates {
		opponentUUID, err := profileUUID(item.opponentID)
		if err != nil {
			return EloRefundSummary{}, err
		}
		matchUUID, err := profileUUID(item.matchID)
		if err != nil {
			return EloRefundSummary{}, err
		}
		current, err := q.LockOpponentRating(ctx, db.LockOpponentRatingParams{
			UserID:   opponentUUID,
			Mode:     db.GdMatchMode(modeDuel),
			SeasonID: seasonID,
		})
		if errors.Is(err, pgx.ErrNoRows) {
			continue
		}
		if err != nil {
			return EloRefundSummary{}, err
		}
		now := time.Now()
		currentState := RatingState{MMR: int(current.Mmr), RD: current.Rd, UpdatedAt: current.UpdatedAt.Time}
		victimWin, _ := CalculateDuelRatingUpdates(currentState, RatingState{MMR: item.cheaterMMR, RD: item.cheaterRD, UpdatedAt: now}, "p1", now)
		refundDelta := victimWin.Delta
		if refundDelta <= 0 {
			continue
		}
		originalLoss := -item.originalDelta
		if refundDelta > originalLoss {
			refundDelta = originalLoss
		}
		before := currentState.MMR
		after := clampRankedMMR(before + refundDelta)
		refundDelta = after - before
		if refundDelta <= 0 {
			continue
		}
		tag, err := q.InsertEloRefund(ctx, db.InsertEloRefundParams{
			UserID:          opponentUUID,
			MatchID:         matchUUID,
			CheaterUserID:   cheaterUUID,
			OriginalDelta:   int32(item.originalDelta),
			RefundDelta:     int32(refundDelta),
			VictimMmrBefore: pgtype.Int4{Int32: int32(before), Valid: true},
			VictimMmrAfter:  pgtype.Int4{Int32: int32(after), Valid: true},
			CreatedByReason: pgtype.Text{String: reason, Valid: true},
		})
		if err != nil {
			return EloRefundSummary{}, err
		}
		if tag == 0 {
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
		if err := q.SetEloRefundNotification(ctx, db.SetEloRefundNotificationParams{
			UserID:         opponentUUID,
			MatchID:        matchUUID,
			CheaterUserID:  cheaterUUID,
			NotificationID: pgtype.Int8{Int64: notificationID, Valid: true},
		}); err != nil {
			return EloRefundSummary{}, err
		}
		if err := q.ApplyEloRefund(ctx, db.ApplyEloRefundParams{
			UserID:   opponentUUID,
			Mode:     db.GdMatchMode(modeDuel),
			SeasonID: seasonID,
			Mmr:      int32(after),
		}); err != nil {
			return EloRefundSummary{}, err
		}
		summary.RefundsIssued++
		summary.TotalRefunded += refundDelta
	}
	return summary, nil
}

func (s *DB) EvaluateAutoCheatBansForMatch(matchID string) error {
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

func (s *DB) riskEngineAnalyzeRequest(ctx context.Context, matchID string) (riskEngineAnalyzeRequest, error) {
	seasonID, err := s.activeSeasonID(ctx)
	if err != nil {
		return riskEngineAnalyzeRequest{}, err
	}
	matchUUID, err := profileUUID(matchID)
	if err != nil {
		return riskEngineAnalyzeRequest{}, err
	}
	rows, err := s.db.ListMatchGuessPlayers(ctx, matchUUID)
	if err != nil {
		return riskEngineAnalyzeRequest{}, err
	}
	userIDs := []string{}
	for _, row := range rows {
		userIDs = append(userIDs, row)
	}
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

func (s *DB) riskEngineRecentGuessEvents(ctx context.Context, userID string) ([]riskEngineGuessEvent, error) {
	rows, err := s.db.ListRecentGuessEvents(ctx, chatUUID(userID))
	if err != nil {
		return nil, err
	}
	events := []riskEngineGuessEvent{}
	for _, row := range rows {
		events = append(events, riskEngineGuessEvent{
			MatchID:     row.MatchID,
			RoundNumber: int(row.RoundNumber),
			Score:       int(row.Score),
			GuessMS:     int(row.GuessMs),
			Evidence:    float64(row.Evidence),
			OccurredAt:  row.OccurredAt.Time,
		})
	}
	return events, nil
}

func (s *DB) riskEnginePlayerContext(ctx context.Context, userID, seasonID string) (int, int, error) {
	row, err := s.db.GetPlayerRiskContext(ctx, db.GetPlayerRiskContextParams{
		UserID:        chatUUID(userID),
		Mode:          db.GdMatchMode(modeDuel),
		SeasonID:      seasonID,
		DefaultRating: int32(initialMMR),
	})
	if err != nil {
		return 0, 0, err
	}
	return int(row.Rating), int(row.RankedGames), nil
}

func (s *DB) recordRiskEngineSignal(ctx context.Context, matchID string, signal riskEngineSignal) error {
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
	signalID, err := db.New(tx).UpsertRiskEngineSignal(ctx, db.UpsertRiskEngineSignalParams{
		SubjectUserID:    chatUUID(signal.SubjectUserID),
		SignalType:       normalizeRiskSignalType(signal.SignalType),
		Severity:         db.GdModerationSeverity(normalizeRiskSeverity(signal.Severity)),
		EvidenceStrength: db.GdEvidenceStrength(normalizeRiskEvidenceStrength(signal.EvidenceStrength)),
		DetectorKey:      signal.DetectorKey,
		DetectorVersion:  signal.DetectorVersion,
		ReasonCode:       nonempty(signal.ReasonCode, "risk_engine_signal"),
		Score:            signal.Score,
		RecommendedQueue: signal.RecommendedQueue,
		MatchID:          chatUUID(matchID),
		PayloadJson:      payload,
		OccurredAt:       pgtype.Timestamptz{Time: occurredAt, Valid: true},
	})
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

func (s *DB) AddSignupIPBan(ipAddress, reason, createdBy string) error {
	ipAddress = strings.TrimSpace(ipAddress)
	if ipAddress == "" {
		return errors.New("ip address required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	return s.db.InsertIPSignupBan(ctx, db.InsertIPSignupBanParams{
		IpAddress: ipAddress,
		Reason:    strings.TrimSpace(reason),
		CreatedBy: strings.TrimSpace(createdBy),
	})
}

func (s *DB) RemoveSignupIPBan(ipAddress string) error {
	ipAddress = strings.TrimSpace(ipAddress)
	if ipAddress == "" {
		return errors.New("ip address required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	return s.db.RevokeIPSignupBan(ctx, ipAddress)
}

func (s *DB) ListSignupIPBans(limit int) ([]SignupIPBan, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	rows, err := s.db.ListActiveIPSignupBans(ctx, int32(limit))
	if err != nil {
		return nil, err
	}
	out := make([]SignupIPBan, 0, len(rows))
	for _, row := range rows {
		out = append(out, SignupIPBan{ID: row.ID, IPAddress: row.IpAddress, Reason: row.Reason, CreatedBy: anyText(row.CreatedBy), CreatedAt: row.CreatedAt.Time})
	}
	return out, nil
}

func (s *DB) IsSignupIPBanned(ipAddress string) (bool, error) {
	ipAddress = strings.TrimSpace(ipAddress)
	if ipAddress == "" {
		return false, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	return s.db.IsIPSignupBanned(ctx, ipAddress)
}
