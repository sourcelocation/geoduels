package badges

import (
	"context"
	"errors"
	"geoduels/internal/accounts"
	"geoduels/internal/audit"
	"geoduels/internal/storekit"
	"geoduels/pkg/entityid"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"geoduels/pkg/contracts"
	db "geoduels/pkg/persistence/sqlc/db"
)

var (
	ErrBadgeNicknameRequired = errors.New("nickname required")
	ErrBadgeUserNotFound     = errors.New("user not found")
	ErrBadgeUnavailable      = errors.New("badge unavailable for manual grant")
)

type badgeDefinition struct {
	ID           string
	Code         int16
	Kind         string
	Label        string
	Description  string
	ImageURL     string
	Rarity       string
	MaxLevel     int16
	Unobtainable bool
	// AdminGrantable is the sole policy for the admin catalog. It is reserved
	// for manual/event awards whose ownership is not synchronized from a role,
	// rank, or historical result.
	AdminGrantable bool
}

var badgeDefinitions = []badgeDefinition{
	{ID: "map-of-the-week", Code: 13, Kind: "achievement", Label: "Map of the Week", Description: "Created a map selected as GeoDuels Map of the Week.", ImageURL: "/badges/map-winner-badge.v1.webp", Rarity: "epic", MaxLevel: 1},
	{
		ID: "discord-member", Code: badgeCodeDiscordMember, Kind: "community",
		Label: "Discord Member", Description: "Retired badge previously awarded for linking Discord to your GeoDuels account.",
		ImageURL: "/badges/discord-badge.v1.png", Rarity: "common", MaxLevel: 1, Unobtainable: true,
	},
	{
		ID: "geoduels-team", Code: badgeCodeGeoDuelsTeam, Kind: "special",
		Label: "GeoDuels Team", Description: "An exclusive medal for GeoDuels moderators and team members.",
		ImageURL: "/badges/team-badge.v1.png", Rarity: "special", MaxLevel: 1,
	},
	{
		ID: "discord-server-member", Code: badgeCodeDiscordServerMember, Kind: "community",
		Label: "Discord Server Member", Description: "Awarded for joining the official GeoDuels Discord server.",
		ImageURL: "/badges/discord-new-badge.v1.png", Rarity: "common", MaxLevel: 1,
	},
	{
		ID: "supporter", Code: badgeCodeSupporter, Kind: "supporter",
		Label: "Supporter", Description: "Awarded for supporting GeoDuels.",
		ImageURL: "/badges/supporter-badge.v1.png", Rarity: "rare", MaxLevel: 1, AdminGrantable: true,
	},
	{
		ID: "speedrunner", Code: badgeCodeSpeedrunner, Kind: "achievement",
		Label: "Speedrunner", Description: "Awarded for scoring 5000 points in under 30 seconds in ranked.",
		ImageURL: "/badges/speedrunner-badge.v1.png", Rarity: "epic", MaxLevel: 1, AdminGrantable: true,
	},
	{
		ID: "elo-1000", Code: badgeCodeElo1000, Kind: "ranked",
		Label: "1000 Elo", Description: "Awarded for reaching 1000 Elo.",
		ImageURL: "/badges/1k-badge.v1.png", Rarity: "common", MaxLevel: 1,
	},
	{
		ID: "elo-1500", Code: badgeCodeElo1500, Kind: "ranked",
		Label: "1500 Elo", Description: "Awarded for reaching 1500 Elo.",
		ImageURL: "/badges/1.5k-badge.v1.png", Rarity: "rare", MaxLevel: 1,
	},
	{
		ID: "elo-2000", Code: badgeCodeElo2000, Kind: "ranked",
		Label: "2000 Elo", Description: "Awarded for reaching 2000 Elo.",
		ImageURL: "/badges/2k-badge.v1.png", Rarity: "legendary", MaxLevel: 1,
	},
	{
		ID: "geoduels-v1-top-finish", Code: badgeCodeLegacyTopFinish, Kind: "legacy_top_finish",
		Label: "GeoDuels V1 Top Finish", Description: "Finished in the global top 100 during GeoDuels V1.",
		ImageURL: "/badges/geoduels-v1-top-finish-badge.v1.png", Rarity: "legendary", MaxLevel: 1, Unobtainable: true,
	},
	{
		ID: "top-finish", Code: badgeCodeTopFinish, Kind: "top_finish",
		Label: "Top Finisher", Description: "Finished in the global top 100.",
		ImageURL: "/badges/top-finish-1-badge.v1.png", Rarity: "legendary", MaxLevel: 3,
	},
	{
		ID: "event-winner-2026", Code: badgeCodeEventWinner2026, Kind: "event",
		Label: "2026 Event Winner", Description: "Awarded for winning the official GeoDuels 2026 Tournament.",
		ImageURL: "/badges/event-winner-2026-badge.v1.png", Rarity: "legendary", MaxLevel: 1, AdminGrantable: true,
	},
}

var topFinishImageByLevel = map[int16]string{
	1: "/badges/top-finish-1-badge.v1.png",
	2: "/badges/top-finish-2-badge.v1.png",
	3: "/badges/top-finish-3-badge.v1.png",
}

func badgeDefinitionByID(id string) (badgeDefinition, bool) {
	id = strings.TrimSpace(id)
	for _, def := range badgeDefinitions {
		if def.ID == id {
			return def, true
		}
	}
	return badgeDefinition{}, false
}

func badgeDefinitionByCode(code int16) (badgeDefinition, bool) {
	for _, def := range badgeDefinitions {
		if def.Code == code {
			return def, true
		}
	}
	return badgeDefinition{}, false
}

func Templates() []contracts.PlayerBadge {
	out := []contracts.PlayerBadge{}
	for _, def := range badgeDefinitions {
		if !def.Unobtainable {
			out = append(out, FromParts(def.Code, 1, 0, false))
		}
	}
	return out
}

func (s *PGStore) ListAdminGrantableBadges() []AdminBadgeDefinition {
	out := make([]AdminBadgeDefinition, 0)
	for _, def := range badgeDefinitions {
		if !def.AdminGrantable {
			continue
		}
		out = append(out, AdminBadgeDefinition{
			ID: def.ID, Kind: def.Kind, Label: def.Label, Description: def.Description,
			ImageURL: def.ImageURL, Rarity: def.Rarity, MaxLevel: int(def.MaxLevel),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func RefFromID(id string) (int16, bool) {
	if strings.TrimSpace(id) == "" {
		return 0, true
	}
	def, ok := badgeDefinitionByID(id)
	if !ok {
		return 0, false
	}
	return def.Code, true
}

func IDFromCode(code int16) string {
	def, ok := badgeDefinitionByCode(code)
	if !ok {
		return ""
	}
	return def.ID
}

func badgeImageURL(def badgeDefinition, level int16) string {
	if def.Code != badgeCodeTopFinish {
		return def.ImageURL
	}
	if level < 1 {
		level = 1
	}
	if level > def.MaxLevel {
		level = def.MaxLevel
	}
	return topFinishImageByLevel[level]
}

func FromParts(code, level, extra int16, owned bool) contracts.PlayerBadge {
	def, ok := badgeDefinitionByCode(code)
	if !ok {
		return contracts.PlayerBadge{}
	}
	if level < 1 {
		level = 1
	}
	if def.MaxLevel > 0 && level > def.MaxLevel {
		level = def.MaxLevel
	}
	badge := contracts.PlayerBadge{
		ID:           def.ID,
		Kind:         def.Kind,
		Label:        def.Label,
		Description:  def.Description,
		ImageURL:     badgeImageURL(def, level),
		Rarity:       def.Rarity,
		Level:        int(level),
		MaxLevel:     int(def.MaxLevel),
		Owned:        owned,
		Unobtainable: def.Unobtainable,
	}
	if extra > 0 {
		badge.Extra = int(extra)
	}
	if code == badgeCodeTopFinish && level > 1 {
		badge.Label = def.Label + " (" + strconv.Itoa(int(level)) + ")"
	}
	return badge
}

func notifyBadgeUnlock(ctx context.Context, tx pgx.Tx, userID string, badge contracts.PlayerBadge) (bool, error) {
	var notificationID int64
	key := "badge_unlocked:" + userID + ":" + badge.ID + ":" + strconv.Itoa(badge.Level)
	if err := storekit.UpsertUserNotificationTx(ctx, tx, userID, "badge_unlocked", key, map[string]any{"badge": badge}, "", &notificationID); err != nil {
		return false, err
	}
	return true, nil
}

func upsertBadgeTx(ctx context.Context, tx pgx.Tx, userID string, code, level, extra int16, notify bool) (bool, error) {
	def, ok := badgeDefinitionByCode(code)
	if !ok || code == 0 {
		return false, errors.New("badge unavailable")
	}
	if level < 1 {
		level = 1
	}
	if def.MaxLevel > 0 && level > def.MaxLevel {
		level = def.MaxLevel
	}
	var oldLevel, oldExtra int16
	row, err := db.New(tx).LockBadge(ctx, db.LockBadgeParams{UserID: chatUUID(userID), BadgeCode: code})
	if errors.Is(err, pgx.ErrNoRows) {
		if err := db.New(tx).InsertBadge(ctx, db.InsertBadgeParams{UserID: chatUUID(userID), BadgeCode: code, Level: level, Extra: extra}); err != nil {
			return false, err
		}
		if notify {
			return notifyBadgeUnlock(ctx, tx, userID, FromParts(code, level, extra, true))
		}
		return true, nil
	}
	if err != nil {
		return false, err
	}
	oldLevel, oldExtra = row.Level, row.Extra
	if level <= oldLevel && extra <= oldExtra {
		return false, nil
	}
	if level < oldLevel {
		level = oldLevel
	}
	if extra < oldExtra {
		extra = oldExtra
	}
	if err := db.New(tx).UpdateBadge(ctx, db.UpdateBadgeParams{UserID: chatUUID(userID), BadgeCode: code, Level: level, Extra: extra}); err != nil {
		return false, err
	}
	if notify && level > oldLevel {
		return notifyBadgeUnlock(ctx, tx, userID, FromParts(code, level, extra, true))
	}
	return true, nil
}

func AwardBadgeTx(ctx context.Context, tx pgx.Tx, userID, badgeID string) (bool, error) {
	code, ok := RefFromID(badgeID)
	if !ok || code == 0 || code == badgeCodeLegacyTopFinish || code == badgeCodeTopFinish {
		return false, errors.New("badge unavailable")
	}
	return upsertBadgeTx(ctx, tx, userID, code, 1, 0, true)
}

func (s *PGStore) GrantBadgeToUser(nickname, badgeID, actorUserID string) (contracts.PlayerBadge, bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	var badge contracts.PlayerBadge
	var changed bool
	err := storekit.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		var err error
		badge, changed, err = GrantBadgeToUserTx(ctx, tx, nickname, badgeID, actorUserID)
		return err
	})
	if err != nil {
		return contracts.PlayerBadge{}, false, err
	}
	return badge, changed, nil
}

// GrantBadgeToUserTx writes the badge, notification and audit in the caller's transaction.
func GrantBadgeToUserTx(ctx context.Context, tx pgx.Tx, nickname, badgeID, actorUserID string) (contracts.PlayerBadge, bool, error) {
	nickname = strings.TrimSpace(nickname)
	actorUserID = strings.TrimSpace(actorUserID)
	def, ok := badgeDefinitionByID(badgeID)
	if nickname == "" {
		return contracts.PlayerBadge{}, false, ErrBadgeNicknameRequired
	}
	if !ok || !def.AdminGrantable {
		return contracts.PlayerBadge{}, false, ErrBadgeUnavailable
	}
	userUUID, err := db.New(tx).FindClaimedUser(ctx, nickname)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return contracts.PlayerBadge{}, false, err
	}
	if errors.Is(err, pgx.ErrNoRows) || !userUUID.Valid {
		return contracts.PlayerBadge{}, false, ErrBadgeUserNotFound
	}
	userID := storekit.UUIDVal(userUUID)
	changed, err := upsertBadgeTx(ctx, tx, userID, def.Code, 1, 0, true)
	if err != nil {
		return contracts.PlayerBadge{}, false, err
	}
	var level, extra int16
	badge, err := db.New(tx).GetBadge(ctx, db.GetBadgeParams{UserID: chatUUID(userID), BadgeCode: def.Code})
	if err != nil {
		return contracts.PlayerBadge{}, false, err
	}
	level, extra = badge.Level, badge.Extra
	if _, err := audit.Record(ctx, tx, audit.Entry{SubjectID: userID, ActorID: actorUserID, Action: audit.ActionBadgeGranted, Metadata: map[string]any{"badgeId": def.ID, "source": "admin"}}); err != nil {
		return contracts.PlayerBadge{}, false, err
	}
	return FromParts(def.Code, level, extra, true), changed, nil
}

func AwardTopFinishTx(ctx context.Context, tx pgx.Tx, userID string) (bool, error) {
	var count int16
	value, err := db.New(tx).LockTopFinish(ctx, db.LockTopFinishParams{UserID: chatUUID(userID), BadgeCode: badgeCodeTopFinish})
	count = int16(value)
	if errors.Is(err, pgx.ErrNoRows) {
		return upsertBadgeTx(ctx, tx, userID, badgeCodeTopFinish, 1, 1, true)
	}
	if err != nil {
		return false, err
	}
	level := topFinishLevel(count)
	return upsertBadgeTx(ctx, tx, userID, badgeCodeTopFinish, level, count, true)
}

func topFinishLevel(count int16) int16 {
	if count < 1 {
		return 1
	}
	if count > 3 {
		return 3
	}
	return count
}

func RemoveGeoDuelsTeamBadgeTx(ctx context.Context, tx pgx.Tx, userID string) error {
	if err := db.New(tx).ClearTeamBadgeSelection(ctx, db.ClearTeamBadgeSelectionParams{UserID: chatUUID(userID), BadgeCode: pgtype.Int2{Int16: badgeCodeGeoDuelsTeam, Valid: true}}); err != nil {
		return err
	}
	return db.New(tx).DeleteTeamBadge(ctx, db.DeleteTeamBadgeParams{UserID: chatUUID(userID), BadgeCode: badgeCodeGeoDuelsTeam})
}

func (s *PGStore) SyncLoginBadges(userID string) error {
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
	seasonID, err := storekit.ActiveSeasonID(ctx, tx)
	if err != nil {
		return err
	}
	var isGuest, hasTeamRole bool
	var mmr int
	info, err := db.New(tx).LoginBadgeInfo(ctx, db.LoginBadgeInfoParams{UserID: chatUUID(userID), Mode: modeDuel, DefaultMmr: int32(initialMMR), SeasonID: seasonID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errors.New("user not found")
		}
		return err
	}
	if info.IsGuest {
		isGuest = true
	}
	hasTeamRole = info.IsStaff
	mmr = int(info.Mmr)
	if hasTeamRole {
		if _, err := AwardBadgeTx(ctx, tx, userID, "geoduels-team"); err != nil {
			return err
		}
	}
	if !isGuest {
		if err := AwardEloBadgesTx(ctx, tx, userID, mmr); err != nil {
			return err
		}
	}
	if s.jobs != nil {
		if err := s.jobs.EnqueueDiscordSyncForUser(ctx, tx, userID); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func AwardEloBadgesTx(ctx context.Context, tx pgx.Tx, userID string, mmr int) error {
	thresholds := []struct {
		mmr     int
		badgeID string
	}{
		{1000, "elo-1000"},
		{1500, "elo-1500"},
		{2000, "elo-2000"},
	}
	for _, threshold := range thresholds {
		if mmr >= threshold.mmr {
			if _, err := AwardBadgeTx(ctx, tx, userID, threshold.badgeID); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *PGStore) AwardDiscordServerMemberByDiscordID(discordUserID string) (bool, error) {
	discordUserID = strings.TrimSpace(discordUserID)
	if discordUserID == "" {
		return false, errors.New("discord user id required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)
	userID, err := db.New(tx).FindDiscordIdentity(ctx, db.FindDiscordIdentityParams{Provider: accounts.IdentityProviderDiscord, ProviderUserID: discordUserID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	awarded, err := AwardBadgeTx(ctx, tx, storekit.UUIDVal(userID), "discord-server-member")
	if err != nil {
		return false, err
	}
	return awarded, tx.Commit(ctx)
}

func (s *PGStore) GetDiscordLinkedUser(discordUserID string) (DiscordLinkedUser, bool, error) {
	discordUserID = strings.TrimSpace(discordUserID)
	if discordUserID == "" {
		return DiscordLinkedUser{}, false, errors.New("discord user id required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	row, err := s.db.LoginDiscordSyncInfo(ctx, db.LoginDiscordSyncInfoParams{Provider: accounts.IdentityProviderDiscord, ProviderUserID: discordUserID, Elo2000Code: badgeCodeElo2000, Elo1500Code: badgeCodeElo1500, Elo1000Code: badgeCodeElo1000})
	var user DiscordLinkedUser
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return DiscordLinkedUser{}, false, nil
		}
		return DiscordLinkedUser{}, false, err
	}
	user.UserID, user.DiscordUserID, user.HighestEloBadgeMMR = storekit.UUIDVal(row.UserID), row.ProviderUserID, int(row.HighestEloBadgeMmr)
	return user, true, nil
}

func (s *PGStore) CreateDonationRef(userID string) (string, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return "", errors.New("user id required")
	}
	ref := "don_" + strings.TrimPrefix(entityid.New(), "u_")
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	if err := s.db.InsertDonationRef(ctx, db.InsertDonationRefParams{DonationRef: ref, UserID: chatUUID(userID)}); err != nil {
		return "", err
	}
	return ref, nil
}

func (s *PGStore) AwardSupporterByDonationRef(ref string) (bool, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return false, errors.New("donation ref required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)
	userID, err := db.New(tx).ClaimDonation(ctx, ref)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	awarded, err := AwardBadgeTx(ctx, tx, storekit.UUIDVal(userID), "supporter")
	if err != nil {
		return false, err
	}
	return awarded, tx.Commit(ctx)
}
