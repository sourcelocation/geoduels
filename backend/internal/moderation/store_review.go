package moderation

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"geoduels/internal/audit"
	"geoduels/internal/storekit"
	"geoduels/pkg/contracts"
	db "geoduels/pkg/persistence/sqlc/db"
	pkgstaff "geoduels/pkg/staff"
)

func (a *PGStore) SearchSubjects(ctx context.Context, viewer pkgstaff.Actor, query string, limit int) ([]contracts.AdminPlayerSummary, error) {
	if limit <= 0 {
		limit = 30
	}
	if limit > 100 {
		limit = 100
	}
	pattern := "%"
	if trimmed := strings.TrimSpace(query); trimmed != "" {
		pattern = "%" + strings.ToLower(trimmed) + "%"
	}
	ctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	seasonID, err := storekit.ActiveSeasonID(ctx, a.pool)
	if err != nil {
		return nil, err
	}
	rows, err := a.q().SearchAdminPlayers(ctx, db.SearchAdminPlayersParams{
		Mode:             db.GdMatchMode(modeDuel),
		SeasonID:         seasonID,
		DefaultMmr:       int32(initialMMR),
		Search:           pattern,
		RowLimit:         int32(limit),
		CreatorID:        pgtype.UUID{},
		IncludeSensitive: viewer.Can(pkgstaff.CapManageAccess),
	})
	if err != nil {
		return nil, err
	}
	result := make([]contracts.AdminPlayerSummary, 0, len(rows))
	for _, row := range rows {
		result = append(result, playerSummaryFromRow(row))
	}
	if viewer.Can(pkgstaff.CapManageAccess) {
		if err := a.populateIdentities(ctx, result); err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (a *PGStore) GetSubject(ctx context.Context, viewer pkgstaff.Actor, userID string) (SubjectDetail, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	summary, err := a.getSubjectSummary(ctx, viewer.Can(pkgstaff.CapManageAccess), userID)
	if err != nil {
		return SubjectDetail{}, err
	}
	stats, err := a.subjectStats(ctx, userID)
	if err != nil {
		return SubjectDetail{}, err
	}
	applyStats(&summary, stats)
	return SubjectDetail{Player: summary}, nil
}

func (a *PGStore) getSubjectSummary(ctx context.Context, includeSensitive bool, userID string) (contracts.AdminPlayerSummary, error) {
	uid, err := storekit.ProfileUUID(userID)
	if err != nil {
		return contracts.AdminPlayerSummary{}, ErrNotFound
	}
	seasonID, err := storekit.ActiveSeasonID(ctx, a.pool)
	if err != nil {
		return contracts.AdminPlayerSummary{}, err
	}
	rows, err := a.q().SearchAdminPlayers(ctx, db.SearchAdminPlayersParams{
		Mode:             db.GdMatchMode(modeDuel),
		SeasonID:         seasonID,
		DefaultMmr:       int32(initialMMR),
		Search:           "%",
		RowLimit:         1,
		CreatorID:        uid,
		IncludeSensitive: includeSensitive,
	})
	if err != nil {
		return contracts.AdminPlayerSummary{}, err
	}
	if len(rows) == 0 {
		return contracts.AdminPlayerSummary{}, ErrNotFound
	}
	item := playerSummaryFromRow(rows[0])
	if includeSensitive {
		items := []contracts.AdminPlayerSummary{item}
		if err := a.populateIdentities(ctx, items); err != nil {
			return contracts.AdminPlayerSummary{}, err
		}
		item = items[0]
	}
	return item, nil
}

type adminStats struct {
	total, ranked, duel, singleplayer, wins, losses int
}

func (a *PGStore) subjectStats(ctx context.Context, userID string) (adminStats, error) {
	u, err := storekit.ProfileUUID(userID)
	if err != nil {
		return adminStats{}, err
	}
	row, err := a.q().AdminPlayerStats(ctx, db.AdminPlayerStatsParams{WinnerUserID: u, Mode: db.GdMatchMode(modeDuel)})
	if err != nil {
		return adminStats{}, err
	}
	return adminStats{
		total: int(row.TotalMatches), ranked: int(row.RankedMatches), duel: int(row.DuelMatches),
		singleplayer: int(row.SingleplayerRuns), wins: int(row.Wins), losses: int(row.Losses),
	}, nil
}

func applyStats(player *contracts.AdminPlayerSummary, stats adminStats) {
	player.TrackedMatches = stats.total
	player.RankedMatches = stats.ranked
	player.DuelMatches = stats.duel
	player.SingleplayerRuns = stats.singleplayer
	player.Losses = stats.losses
	if stats.wins > player.Wins {
		player.Wins = stats.wins
	}
}

func playerSummaryFromRow(row db.SearchAdminPlayersRow) contracts.AdminPlayerSummary {
	var item contracts.AdminPlayerSummary
	item.UserID = storekit.UUIDVal(row.UserID)
	item.Email = row.Email
	item.DisplayName = row.DisplayName
	item.AvatarURL = row.AvatarUrl
	item.MMR = int(row.Mmr)
	item.GamesPlayed = int(row.GamesPlayed)
	item.Wins = int(row.Wins)
	item.RankedGamesPlayed = int(row.RankedGamesPlayed)
	item.IsGuest = row.IsGuest
	item.IsAdmin = row.IsAdmin
	item.IsModerator = row.IsModerator
	item.IsBanned = row.IsBanned
	item.BanReason = row.BanReason
	if row.BannedAt.Valid {
		item.BannedAt = row.BannedAt.Time
	}
	if row.BanExpiresAt.Valid {
		item.BanExpiresAt = row.BanExpiresAt.Time
	}
	if row.ChatMutedAt.Valid {
		item.ChatMutedAt = row.ChatMutedAt.Time
	}
	item.ChatMuteReason = row.ChatMuteReason
	if row.ChatMuteExpiresAt.Valid {
		item.ChatMutedUntil = row.ChatMuteExpiresAt.Time
	}
	if row.ReportMutedAt.Valid {
		item.ReportMutedAt = row.ReportMutedAt.Time
	}
	item.ReportMuteReason = row.ReportMuteReason
	if row.ReportMuteExpiresAt.Valid {
		item.ReportMutedUntil = row.ReportMuteExpiresAt.Time
	}
	item.LastIPAddress = row.LastIpAddress
	return item
}

func (a *PGStore) populateIdentities(ctx context.Context, players []contracts.AdminPlayerSummary) error {
	if len(players) == 0 {
		return nil
	}
	userIDs := make([]pgtype.UUID, 0, len(players))
	byUserID := make(map[string]int, len(players))
	for i := range players {
		uid := storekit.MustUUID(players[i].UserID)
		userIDs = append(userIDs, uid)
		byUserID[uid.String()] = i
	}
	rows, err := a.q().AdminPlayerIdentities(ctx, userIDs)
	if err != nil {
		return err
	}
	for _, row := range rows {
		var identity contracts.AdminUserIdentity
		identity.Provider = string(row.Provider)
		identity.ProviderUserID = row.ProviderUserID
		identity.Email = row.Email
		identity.ProviderName = row.ProviderName
		if row.LastSeenAt.Valid {
			identity.LastSeenAt = row.LastSeenAt.Time
		}
		if row.DeletedAt.Valid {
			identity.DeletedAt = row.DeletedAt.Time
		}
		if idx, ok := byUserID[row.UserID.String()]; ok {
			players[idx].Identities = append(players[idx].Identities, identity)
		}
	}
	return nil
}

func (a *PGStore) ListSignals(ctx context.Context, subjectID string, limit int) ([]contracts.ModerationSignalSummary, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 200 {
		limit = 200
	}
	ctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	subject := pgtype.UUID{}
	if strings.TrimSpace(subjectID) != "" {
		u, err := storekit.ProfileUUID(subjectID)
		if err != nil {
			return nil, err
		}
		subject = u
	}
	rows, err := a.q().ListModerationSignals(ctx, db.ListModerationSignalsParams{SubjectUserID: subject, RowLimit: int32(limit)})
	if err != nil {
		return nil, err
	}
	out := make([]contracts.ModerationSignalSummary, 0, len(rows))
	for _, x := range rows {
		item := contracts.ModerationSignalSummary{
			ID: x.SignalID, SubjectUserID: storekit.UUIDVal(x.SubjectUserID), SubjectName: storekit.TextVal(x.SubjectDisplayName),
			SignalType: x.SignalType, Source: string(x.Source), Severity: string(x.Severity), EvidenceStrength: string(x.EvidenceStrength),
			DetectorKey: x.DetectorKey, DetectorVersion: x.DetectorVersion, ReasonCode: x.ReasonCode, Score: x.Score,
			RecommendedQueue: x.RecommendedQueue, ReporterUserID: storekit.UUIDVal(x.ReporterUserID), ReporterName: storekit.TextVal(x.ReporterDisplayName),
			MatchID: storekit.UUIDVal(x.MatchID), Payload: json.RawMessage(x.PayloadJson), OccurredAt: x.OccurredAt.Time,
			CreatedAt: x.CreatedAt.Time, ReviewedAt: x.ReviewedAt.Time, ReviewedBy: storekit.UUIDVal(x.ReviewedBy),
		}
		if x.Outcome.Valid {
			item.Outcome = string(x.Outcome.GdModerationOutcome)
		}
		out = append(out, item)
	}
	return out, nil
}

func (a *PGStore) ListAudit(ctx context.Context, subjectID string, limit int) ([]contracts.ModerationAuditLogEntry, error) {
	ctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	return audit.List(ctx, a.conn(), subjectID, limit)
}
