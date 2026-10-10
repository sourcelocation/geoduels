package moderation

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"geoduels/internal/audit"
	"geoduels/internal/rating"
	"geoduels/internal/storekit"
	db "geoduels/pkg/persistence/sqlc/db"
)

// ApplyCheatingBan persists the account and linked-identity ban together.
func (a *PGStore) ApplyCheatingBan(ctx context.Context, userID, reason, actorID string) (string, error) {
	q := a.q()
	uid, err := storekit.ProfileUUID(userID)
	if err != nil {
		return "", ErrNotFound
	}
	tag, err := q.BanUserForCheating(ctx, db.BanUserForCheatingParams{ID: uid, BanReason: pgtype.Text{String: reason, Valid: true}})
	if err != nil {
		return "", err
	}
	if tag == 0 {
		return "", ErrNotFound
	}
	if err := q.BanUserOAuthIdentities(ctx, db.BanUserOAuthIdentitiesParams{BannedUserID: uid, Reason: reason, CreatedBy: actorID}); err != nil {
		return "", err
	}
	return q.GetUserRegistrationIP(ctx, uid)
}

func (a *PGStore) HasRelatedCheater(ctx context.Context, userID, registrationIP string) (bool, error) {
	uid, err := storekit.ProfileUUID(userID)
	if err != nil {
		return false, err
	}
	return a.q().HasRelatedCheaterFromIP(ctx, db.HasRelatedCheaterFromIPParams{ID: uid, RegistrationIpAddress: pgtype.Text{String: registrationIP, Valid: true}})
}

func (a *PGStore) NotifyCheatingBan(ctx context.Context, userID, reason string, logID int64) error {
	if err := a.notifyAccountEnforcement(ctx, userID, "permanent_ban", reason, logID); err != nil {
		return err
	}
	return a.notifyReportersOfBan(ctx, userID, "permanent_ban", logID)
}

func (a *PGStore) SetBan(ctx context.Context, userID, reason, actorID string, banned bool) error {
	if strings.TrimSpace(userID) == "" {
		return errors.New("user id required")
	}
	q := a.q()
	uid, err := storekit.ProfileUUID(userID)
	if err != nil {
		return ErrNotFound
	}
	bannedAt := pgtype.Timestamptz{}
	banReason := pgtype.Text{}
	if banned {
		bannedAt = storekit.Timestamptz(time.Now())
		if strings.TrimSpace(reason) != "" {
			banReason = pgtype.Text{String: strings.TrimSpace(reason), Valid: true}
		}
	}
	tag, err := q.BanUser(ctx, db.BanUserParams{UserID: uid, BannedAt: bannedAt, BanReason: banReason})
	if err != nil {
		return err
	}
	if tag == 0 {
		return ErrNotFound
	}
	if banned {
		if err := q.BanUserOAuthIdentities(ctx, db.BanUserOAuthIdentitiesParams{BannedUserID: uid, Reason: strings.TrimSpace(reason), CreatedBy: strings.TrimSpace(actorID)}); err != nil {
			return err
		}
	} else {
		if _, err := q.RevokeOAuthIdentityBans(ctx, uid); err != nil {
			return err
		}
	}
	action := audit.ActionUnban
	if banned {
		action = audit.ActionPermanentBan
	}
	logID, err := a.RecordAudit(ctx, audit.Entry{SubjectID: userID, ActorID: actorID, Action: action, Reason: reason})
	if err != nil {
		return err
	}
	return a.notifyAccountEnforcement(ctx, userID, string(action), reason, logID)
}

func (a *PGStore) SetMute(ctx context.Context, userID, kind, reason, actorID string, until time.Time, muted bool) error {
	userID = strings.TrimSpace(userID)
	kind = strings.ToLower(strings.TrimSpace(kind))
	if userID == "" {
		return errors.New("user id required")
	}
	if kind != "chat" && kind != "report" {
		return errors.New("unsupported mute kind")
	}
	q := a.q()
	uid, err := storekit.ProfileUUID(userID)
	if err != nil {
		return ErrNotFound
	}
	var tag int64
	if muted {
		if kind == "chat" {
			tag, err = q.SetChatMute(ctx, db.SetChatMuteParams{UserID: uid, Reason: strings.TrimSpace(reason), ChatMuteExpiresAt: storekit.Timestamptz(until)})
		} else {
			tag, err = q.SetReportMute(ctx, db.SetReportMuteParams{UserID: uid, Reason: strings.TrimSpace(reason), ReportMuteExpiresAt: storekit.Timestamptz(until)})
		}
	} else if kind == "chat" {
		tag, err = q.ClearChatMute(ctx, uid)
	} else {
		tag, err = q.ClearReportMute(ctx, uid)
	}
	if err != nil {
		return err
	}
	if tag == 0 {
		return ErrNotFound
	}
	entry := audit.Entry{SubjectID: userID, ActorID: actorID, Action: muteAction(kind, muted), Reason: reason}
	if muted {
		entry.ExpiresAt = &until
	}
	_, err = a.RecordAudit(ctx, entry)
	return err
}

func muteAction(kind string, muted bool) audit.Action {
	switch {
	case kind == "chat" && muted:
		return audit.ActionChatMute
	case kind == "chat":
		return audit.ActionChatUnmute
	case muted:
		return audit.ActionReportMute
	default:
		return audit.ActionReportUnmute
	}
}

func (a *PGStore) ClearReporterMute(ctx context.Context, userID string) error {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return errors.New("user id required")
	}
	q := a.q()
	uid, err := storekit.ProfileUUID(userID)
	if err != nil {
		return ErrNotFound
	}
	tag, err := q.ClearReporterMute(ctx, uid)
	if err != nil {
		return err
	}
	if tag == 0 {
		return ErrNotFound
	}
	_, err = a.RecordAudit(ctx, audit.Entry{SubjectID: userID, Action: audit.ActionReportUnmute})
	return err
}

func (a *PGStore) PreviewPardon(ctx context.Context, cutoff time.Time) (CommunityPardonSummary, error) {
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	candidates, err := a.q().ListCommunityPardonCandidates(ctx, storekit.Timestamptz(cutoff))
	if err != nil {
		return CommunityPardonSummary{}, err
	}
	return CommunityPardonSummary{Eligible: len(candidates), Cutoff: cutoff}, nil
}

func (a *PGStore) Pardon(ctx context.Context, cutoff time.Time, actorID string) (CommunityPardonSummary, error) {
	actorID = strings.TrimSpace(actorID)
	q := a.q()
	userIDs, err := q.PardonBannedPlayers(ctx, storekit.Timestamptz(cutoff))
	if err != nil {
		return CommunityPardonSummary{}, err
	}
	metadata := map[string]any{"release": "v2", "policy": "active ban older than 7 days"}
	for _, uid := range userIDs {
		userID := storekit.UUIDVal(uid)
		if _, err := q.RevokeOAuthIdentityBans(ctx, uid); err != nil {
			return CommunityPardonSummary{}, err
		}
		logID, err := a.RecordAudit(ctx, audit.Entry{SubjectID: userID, ActorID: actorID, Action: audit.ActionUnban, Reason: "v2 community pardon", Metadata: metadata})
		if err != nil {
			return CommunityPardonSummary{}, err
		}
		if err := a.notifyAccountEnforcement(ctx, userID, "unban", "v2 community pardon", logID); err != nil {
			return CommunityPardonSummary{}, err
		}
	}
	return CommunityPardonSummary{Eligible: len(userIDs), Pardoned: len(userIDs), Cutoff: cutoff}, nil
}

func (a *PGStore) AddIPBan(ctx context.Context, ipAddress, reason, actorID string) error {
	ipAddress = strings.TrimSpace(ipAddress)
	if ipAddress == "" {
		return errors.New("ip address required")
	}
	return a.q().InsertIPSignupBan(ctx, db.InsertIPSignupBanParams{
		IpAddress: ipAddress,
		Reason:    strings.TrimSpace(reason),
		CreatedBy: strings.TrimSpace(actorID),
	})
}

func (a *PGStore) RemoveIPBan(ctx context.Context, ipAddress string) error {
	ipAddress = strings.TrimSpace(ipAddress)
	if ipAddress == "" {
		return errors.New("ip address required")
	}
	return a.q().RevokeIPSignupBan(ctx, ipAddress)
}

func (a *PGStore) IsSignupIPBanned(ctx context.Context, ipAddress string) (bool, error) {
	ipAddress = strings.TrimSpace(ipAddress)
	if ipAddress == "" {
		return false, nil
	}
	ctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	return a.q().IsIPSignupBanned(ctx, ipAddress)
}

func (a *PGStore) ListIPBans(ctx context.Context, limit int) ([]SignupIPBan, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	ctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	rows, err := a.q().ListActiveIPSignupBans(ctx, int32(limit))
	if err != nil {
		return nil, err
	}
	out := make([]SignupIPBan, 0, len(rows))
	for _, row := range rows {
		out = append(out, SignupIPBan{ID: row.ID, IPAddress: row.IpAddress, Reason: row.Reason, CreatedBy: storekit.UUIDVal(row.CreatedBy), CreatedAt: row.CreatedAt.Time})
	}
	return out, nil
}

func (a *PGStore) RefundCandidates(ctx context.Context, cheaterID string) ([]RefundCandidate, error) {
	uid, err := storekit.ProfileUUID(cheaterID)
	if err != nil {
		return nil, err
	}
	rows, err := a.q().ListCheaterRefundCandidates(ctx, db.ListCheaterRefundCandidatesParams{
		UserID: uid, Mode: db.GdMatchMode(modeDuel), DefaultRatingRd: rating.InitialRatingRD,
	})
	if err != nil {
		return nil, err
	}
	out := make([]RefundCandidate, 0, len(rows))
	for _, row := range rows {
		out = append(out, RefundCandidate{MatchID: storekit.UUIDVal(row.MatchID), UserID: storekit.UUIDVal(row.OpponentUserID), CheaterMMR: int(row.CheaterMmr), CheaterRD: row.CheaterRd.Float64, OriginalDelta: int(row.OriginalDelta)})
	}
	return out, nil
}

func (a *PGStore) LockRefundRating(ctx context.Context, userID, seasonID string) (RatingState, bool, error) {
	uid, err := storekit.ProfileUUID(userID)
	if err != nil {
		return RatingState{}, false, err
	}
	row, err := a.q().LockOpponentRating(ctx, db.LockOpponentRatingParams{UserID: uid, Mode: db.GdMatchMode(modeDuel), SeasonID: seasonID})
	if errors.Is(err, pgx.ErrNoRows) {
		return RatingState{}, false, nil
	}
	if err != nil {
		return RatingState{}, false, err
	}
	return RatingState{MMR: int(row.Mmr), RD: row.Rd, UpdatedAt: row.UpdatedAt.Time}, true, nil
}

// SaveRefund atomically deduplicates the refund, writes its notification and
// updates the locked rating. All writes use the caller's transaction.
func (a *PGStore) SaveRefund(ctx context.Context, award RefundAward) (bool, error) {
	if _, err := a.requireTx(); err != nil {
		return false, err
	}
	q := a.q()
	opponentUUID, err := storekit.ProfileUUID(award.UserID)
	if err != nil {
		return false, err
	}
	matchUUID, err := storekit.ProfileUUID(award.MatchID)
	if err != nil {
		return false, err
	}
	cheaterUUID, err := storekit.ProfileUUID(award.CheaterID)
	if err != nil {
		return false, err
	}
	tag, err := q.InsertEloRefund(ctx, db.InsertEloRefundParams{
		UserID:          opponentUUID,
		MatchID:         matchUUID,
		CheaterUserID:   cheaterUUID,
		OriginalDelta:   int32(award.OriginalDelta),
		RefundDelta:     int32(award.Delta),
		VictimMmrBefore: pgtype.Int4{Int32: int32(award.Before), Valid: true},
		VictimMmrAfter:  pgtype.Int4{Int32: int32(award.After), Valid: true},
		CreatedByReason: pgtype.Text{String: award.Reason, Valid: true},
	})
	if err != nil {
		return false, err
	}
	if tag == 0 {
		return false, nil
	}
	payload := map[string]any{
		"refundDelta": award.Delta, "matchId": award.MatchID, "cheaterUserId": award.CheaterID,
		"reason": award.Reason, "mmrBefore": award.Before, "mmrAfter": award.After,
	}
	notificationID, err := storekit.UpsertUserNotificationTx(ctx, a.tx, award.UserID, "mmr_refund", fmt.Sprintf("mmr_refund:%s:%s:%s", award.UserID, award.MatchID, award.CheaterID), payload, "", time.Time{})
	if err != nil {
		return false, err
	}
	if err := q.SetEloRefundNotification(ctx, db.SetEloRefundNotificationParams{UserID: opponentUUID, MatchID: matchUUID, CheaterUserID: cheaterUUID, NotificationID: pgtype.Int8{Int64: notificationID, Valid: true}}); err != nil {
		return false, err
	}
	if err := q.ApplyEloRefund(ctx, db.ApplyEloRefundParams{UserID: opponentUUID, Mode: db.GdMatchMode(modeDuel), SeasonID: award.SeasonID, Mmr: int32(award.After)}); err != nil {
		return false, err
	}
	return true, nil
}

func (a *PGStore) notifyAccountEnforcement(ctx context.Context, userID, action, reason string, moderationLogID int64) error {
	notificationType := "account_banned"
	if action == "unban" {
		notificationType = "account_unbanned"
	}
	_, err := storekit.UpsertUserNotificationTx(ctx, a.tx, userID, notificationType, fmt.Sprintf("%s:%d", notificationType, moderationLogID), map[string]any{
		"reason": strings.TrimSpace(reason), "action": action, "moderationLogId": moderationLogID, "endsAt": nil,
	}, "", time.Time{})
	return err
}

func (a *PGStore) notifyReportersOfBan(ctx context.Context, subjectUserID, action string, logID int64) error {
	rows, err := a.q().ListReporters(ctx, storekit.MustUUID(subjectUserID))
	if err != nil {
		return err
	}
	for _, row := range rows {
		reporterID := storekit.UUIDVal(row)
		if _, err := storekit.UpsertUserNotificationTx(ctx, a.tx, reporterID, "reported_player_banned", fmt.Sprintf("reported_player_banned:%d:%s", logID, reporterID), map[string]any{
			"action": action, "moderationLogId": logID,
		}, "", time.Time{}); err != nil {
			return err
		}
	}
	return nil
}
