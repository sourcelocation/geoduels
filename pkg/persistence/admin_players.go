package persistence

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"geoduels/pkg/contracts"
	db "geoduels/pkg/persistence/sqlc/db"
)

func (s *DB) ListUserRoles() ([]UserRoleGrant, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	rows, err := s.db.ListUserRoles(ctx)
	if err != nil {
		return nil, err
	}
	out := []UserRoleGrant{}
	for _, row := range rows {
		var item UserRoleGrant
		item.UserID = row.ID.String()
		item.DisplayName, _ = row.DisplayName.(string)
		item.Email, item.Role, item.GrantedBy, item.Reason = row.Email, row.Role, "", row.LastReason
		item.GrantedBy = uuidVal(row.ActorUserID)
		if row.GrantedAt.Valid {
			item.GrantedAt = row.GrantedAt.Time
		}
		out = append(out, item)
	}
	return out, nil
}

func normalizeAdminRole(role string) (string, error) {
	role = strings.ToLower(strings.TrimSpace(role))
	switch role {
	case "admin", "moderator":
		return role, nil
	default:
		return "", errors.New("unsupported role")
	}
}

func (s *DB) GrantUserRole(userID, role, grantedBy, reason string) error {
	userID = strings.TrimSpace(userID)
	role, err := normalizeAdminRole(role)
	if err != nil {
		return err
	}
	if userID == "" {
		return errors.New("user id required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	uid, err := profileUUID(userID)
	if err != nil {
		return err
	}
	q := s.db.WithTx(tx)
	tag, err := q.GrantUserRole(ctx, db.GrantUserRoleParams{UserID: uid, Role: role})
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errors.New("user not found")
	}
	if err := q.GrantRoleLog(ctx, db.GrantRoleLogParams{SubjectUserID: uid, ActorUserID: strings.TrimSpace(grantedBy), Reason: strings.TrimSpace(reason), Role: role}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *DB) RevokeUserRole(userID, role, revokedBy, reason string) error {
	userID = strings.TrimSpace(userID)
	role, err := normalizeAdminRole(role)
	if err != nil {
		return err
	}
	if userID == "" {
		return errors.New("user id required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	q := s.db.WithTx(tx)
	uid, err := profileUUID(userID)
	if err != nil {
		return err
	}
	tag, err := q.RevokeUserRole(ctx, db.RevokeUserRoleParams{UserID: uid, Role: role})
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errors.New("user not found")
	}
	hasTeamRoleValue, err := q.HasTeamRole(ctx, uid)
	if err != nil {
		return err
	}
	hasTeamRole := hasTeamRoleValue.Bool
	if !hasTeamRole {
		if err := removeGeoDuelsTeamBadgeTx(ctx, tx, userID); err != nil {
			return err
		}
	}
	if err := q.RevokeRoleLog(ctx, db.RevokeRoleLogParams{SubjectUserID: uid, ActorUserID: strings.TrimSpace(revokedBy), Reason: strings.TrimSpace(reason), Role: role}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *DB) SearchPlayers(query string, limit int) ([]AdminPlayerSummary, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	pattern := "%"
	trimmed := strings.TrimSpace(query)
	if trimmed != "" {
		pattern = "%" + strings.ToLower(trimmed) + "%"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	seasonID, err := s.activeSeasonID(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := s.db.SearchAdminPlayers(ctx, db.SearchAdminPlayersParams{
		Mode:       db.GdMatchMode(modeDuel),
		SeasonID:   seasonID,
		DefaultMmr: int32(initialMMR),
		Search:     pattern,
		RowLimit:   int32(limit),
		CreatorID:  pgtype.UUID{},
	})
	if err != nil {
		return nil, err
	}
	result := make([]AdminPlayerSummary, 0, len(rows))
	for _, row := range rows {
		result = append(result, adminPlayerSummaryFromRow(row))
	}
	if err := s.populateAdminPlayerIdentities(ctx, result); err != nil {
		return nil, err
	}
	return result, nil
}

func adminPlayerSummaryFromRow(row db.SearchAdminPlayersRow) AdminPlayerSummary {
	var item AdminPlayerSummary
	item.UserID = uuidVal(row.UserID)
	item.Email = row.Email
	if row.DisplayName.Valid {
		item.DisplayName = row.DisplayName.String
	}
	item.AvatarURL = row.AvatarUrl
	item.MMR = int(row.Mmr)
	item.GamesPlayed = int(row.GamesPlayed)
	item.Wins = int(row.Wins)
	item.RankedGamesPlayed = int(row.RankedGamesPlayed)
	item.IsGuest, _ = row.IsGuest.(bool)
	item.IsAdmin = row.IsAdmin
	item.IsModerator = row.IsModerator
	item.IsBanned, _ = row.IsBanned.(bool)
	item.BanReason = row.BanReason
	item.BannedAt = row.BannedAt.Time
	item.BanExpiresAt = row.BanExpiresAt.Time
	item.ChatMutedAt = row.ChatMutedAt.Time
	item.ChatMuteReason = row.ChatMuteReason
	item.ChatMutedUntil = row.ChatMuteExpiresAt.Time
	item.ReportMutedAt = row.ReportMutedAt.Time
	item.ReportMuteReason = row.ReportMuteReason
	item.ReportMutedUntil = row.ReportMuteExpiresAt.Time
	item.LastIPAddress = row.LastIpAddress
	return item
}

func (s *DB) getAdminPlayerSummary(ctx context.Context, userID string) (AdminPlayerSummary, error) {
	seasonID, err := s.activeSeasonID(ctx)
	if err != nil {
		return AdminPlayerSummary{}, err
	}
	uid, err := profileUUID(userID)
	if err != nil {
		return AdminPlayerSummary{}, err
	}
	rows, err := s.db.SearchAdminPlayers(ctx, db.SearchAdminPlayersParams{
		Mode:       db.GdMatchMode(modeDuel),
		SeasonID:   seasonID,
		DefaultMmr: int32(initialMMR),
		Search:     "%",
		RowLimit:   1,
		CreatorID:  uid,
	})
	if err != nil {
		return AdminPlayerSummary{}, err
	}
	if len(rows) == 0 {
		return AdminPlayerSummary{}, pgx.ErrNoRows
	}
	item := adminPlayerSummaryFromRow(rows[0])
	items := []AdminPlayerSummary{item}
	if err := s.populateAdminPlayerIdentities(ctx, items); err != nil {
		return AdminPlayerSummary{}, err
	}
	return items[0], nil
}

func (s *DB) GetAdminPlayerDetail(userID string) (AdminPlayerDetail, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return AdminPlayerDetail{}, errors.New("userID required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	player, err := s.getAdminPlayerSummary(ctx, userID)
	if err != nil {
		return AdminPlayerDetail{}, err
	}
	stats, err := s.adminPlayerStats(ctx, userID)
	if err != nil {
		return AdminPlayerDetail{}, err
	}
	applyAdminPlayerStats(&player, stats)
	return AdminPlayerDetail{Player: player}, nil
}

func applyAdminPlayerStats(player *AdminPlayerSummary, stats AdminPlayerStats) {
	player.TrackedMatches = stats.TotalMatches
	player.RankedMatches = stats.RankedMatches
	player.DuelMatches = stats.DuelMatches
	player.SingleplayerRuns = stats.SingleplayerRuns
	player.Losses = stats.Losses
	if stats.Wins > player.Wins {
		player.Wins = stats.Wins
	}
}

func (s *DB) adminPlayerStats(ctx context.Context, userID string) (AdminPlayerStats, error) {
	var stats AdminPlayerStats
	u, err := profileUUID(userID)
	if err != nil {
		return stats, err
	}
	row, err := s.db.AdminPlayerStats(ctx, db.AdminPlayerStatsParams{WinnerUserID: u, Mode: db.GdMatchMode(modeDuel)})
	if err == nil {
		stats.TotalMatches, stats.RankedMatches, stats.DuelMatches, stats.SingleplayerRuns, stats.Wins, stats.Losses = int(row.TotalMatches), int(row.RankedMatches), int(row.DuelMatches), int(row.SingleplayerRuns), int(row.Wins), int(row.Losses)
	}
	return stats, err
}

func (s *DB) populateAdminPlayerIdentities(ctx context.Context, players []AdminPlayerSummary) error {
	if len(players) == 0 {
		return nil
	}
	userIDs := make([]pgtype.UUID, 0, len(players))
	byUserID := make(map[string]int, len(players))
	for i := range players {
		uid := chatUUID(players[i].UserID)
		userIDs = append(userIDs, uid)
		byUserID[uid.String()] = i
	}
	rows, err := s.db.AdminPlayerIdentities(ctx, userIDs)
	if err != nil {
		return err
	}
	for _, row := range rows {
		var identity contracts.AdminUserIdentity
		identity.Provider = string(row.Provider)
		identity.ProviderUserID = row.ProviderUserID
		identity.Email = row.Email
		identity.ProviderName = row.ProviderName
		identity.LastSeenAt = row.LastSeenAt.Time
		if row.DeletedAt.Valid {
			identity.DeletedAt = row.DeletedAt.Time
		}
		if idx, ok := byUserID[row.UserID.String()]; ok {
			players[idx].Identities = append(players[idx].Identities, identity)
		}
	}
	return nil
}

func (s *DB) SetPlayerBan(userID, reason, actorUserID string, banned bool) error {
	if userID == "" {
		return errors.New("user id required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	q := db.New(tx)
	uid, err := profileUUID(userID)
	if err != nil {
		return err
	}
	bannedAt := pgtype.Timestamptz{}
	banReason := pgtype.Text{}
	if banned {
		bannedAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
		if strings.TrimSpace(reason) != "" {
			banReason = pgtype.Text{String: strings.TrimSpace(reason), Valid: true}
		}
	}
	tag, err := q.BanUser(ctx, db.BanUserParams{UserID: uid, BannedAt: bannedAt, BanReason: banReason})
	if err != nil {
		return err
	}
	if tag == 0 {
		return errors.New("user not found")
	}
	if banned {
		if err := q.BanUserOAuthIdentities(ctx, db.BanUserOAuthIdentitiesParams{BannedUserID: uid, Reason: strings.TrimSpace(reason), CreatedBy: strings.TrimSpace(actorUserID)}); err != nil {
			return err
		}
	} else {
		if _, err := q.RevokeOAuthIdentityBans(ctx, uid); err != nil {
			return err
		}
	}
	action := "unban"
	if banned {
		action = "permanent_ban"
	}
	logID, err := q.InsertModerationLog(ctx, db.InsertModerationLogParams{
		SubjectUserID: uid,
		ActorUserID:   strings.TrimSpace(actorUserID),
		Action:        db.GdModerationLogAction(action),
		Reason:        strings.TrimSpace(reason),
	})
	if err != nil {
		return err
	}
	if err := notifyAccountEnforcement(ctx, tx, userID, action, reason, logID, nil); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *DB) PreviewCommunityPardon(olderThan time.Duration) (CommunityPardonSummary, error) {
	if olderThan <= 0 {
		olderThan = 7 * 24 * time.Hour
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	cutoff := time.Now().Add(-olderThan)
	candidates, err := s.db.ListCommunityPardonCandidates(ctx, pgtype.Timestamptz{Time: cutoff, Valid: true})
	if err != nil {
		return CommunityPardonSummary{}, err
	}
	return CommunityPardonSummary{Eligible: len(candidates), Cutoff: cutoff}, nil
}

func (s *DB) PardonBannedPlayers(olderThan time.Duration, actorUserID string) (CommunityPardonSummary, error) {
	if olderThan <= 0 {
		olderThan = 7 * 24 * time.Hour
	}
	actorUserID = strings.TrimSpace(actorUserID)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return CommunityPardonSummary{}, err
	}
	defer tx.Rollback(ctx)
	q := db.New(tx)

	cutoff := time.Now().Add(-olderThan)
	userIDs, err := q.PardonBannedPlayers(ctx, pgtype.Timestamptz{Time: cutoff, Valid: true})
	if err != nil {
		return CommunityPardonSummary{}, err
	}
	metadata, _ := json.Marshal(map[string]any{"release": "v2", "policy": "active ban older than 7 days"})
	for _, uid := range userIDs {
		userID := uuidVal(uid)
		if _, err := q.RevokeOAuthIdentityBans(ctx, uid); err != nil {
			return CommunityPardonSummary{}, err
		}
		logID, err := q.InsertModerationLog(ctx, db.InsertModerationLogParams{
			SubjectUserID: uid,
			ActorUserID:   actorUserID,
			Action:        db.GdModerationLogActionUnban,
			Reason:        "v2 community pardon",
			Metadata:      metadata,
		})
		if err != nil {
			return CommunityPardonSummary{}, err
		}
		if err := notifyAccountEnforcement(ctx, tx, userID, "unban", "v2 community pardon", logID, nil); err != nil {
			return CommunityPardonSummary{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return CommunityPardonSummary{}, err
	}
	return CommunityPardonSummary{Eligible: len(userIDs), Pardoned: len(userIDs), Cutoff: cutoff}, nil
}

func (s *DB) ClearReporterMute(userID string) error {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return errors.New("user id required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	q := db.New(tx)
	uid, err := profileUUID(userID)
	if err != nil {
		return err
	}
	tag, err := q.ClearReporterMute(ctx, uid)
	if err != nil {
		return err
	}
	if tag == 0 {
		return errors.New("user not found")
	}
	if _, err := q.InsertModerationLog(ctx, db.InsertModerationLogParams{SubjectUserID: uid, Action: db.GdModerationLogActionReportUnmute}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *DB) SetPlayerMute(userID, kind, reason, actorUserID string, until time.Time, muted bool) error {
	userID = strings.TrimSpace(userID)
	kind = strings.ToLower(strings.TrimSpace(kind))
	if userID == "" {
		return errors.New("user id required")
	}
	if kind != "chat" && kind != "report" {
		return errors.New("unsupported mute kind")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	q := db.New(tx)
	uid, err := profileUUID(userID)
	if err != nil {
		return err
	}
	var tag int64
	if muted {
		if until.IsZero() {
			until = time.Now().Add(7 * 24 * time.Hour)
		}
		if kind == "chat" {
			tag, err = q.SetChatMute(ctx, db.SetChatMuteParams{UserID: uid, Reason: strings.TrimSpace(reason), ChatMuteExpiresAt: pgtype.Timestamptz{Time: until, Valid: true}})
		} else {
			tag, err = q.SetReportMute(ctx, db.SetReportMuteParams{UserID: uid, Reason: strings.TrimSpace(reason), ReportMuteExpiresAt: pgtype.Timestamptz{Time: until, Valid: true}})
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
		return errors.New("user not found")
	}
	action := kind + "_unmute"
	expiresAt := pgtype.Timestamptz{}
	if muted {
		action = kind + "_mute"
		expiresAt = pgtype.Timestamptz{Time: until, Valid: true}
	}
	if _, err := q.InsertModerationLog(ctx, db.InsertModerationLogParams{
		SubjectUserID: uid,
		ActorUserID:   strings.TrimSpace(actorUserID),
		Action:        db.GdModerationLogAction(action),
		Reason:        strings.TrimSpace(reason),
		ExpiresAt:     expiresAt,
	}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
