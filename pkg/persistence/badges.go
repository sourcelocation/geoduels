package persistence

import (
	"context"
	"errors"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"geoduels/pkg/contracts"
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

func badgeTemplates() []contracts.PlayerBadge {
	out := []contracts.PlayerBadge{}
	for _, def := range badgeDefinitions {
		if !def.Unobtainable {
			out = append(out, badgeFromParts(def.Code, 1, 0, false))
		}
	}
	return out
}

func (s *pgStore) ListAdminGrantableBadges() []AdminBadgeDefinition {
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

func badgeRefFromID(id string) (int16, bool) {
	if strings.TrimSpace(id) == "" {
		return 0, true
	}
	def, ok := badgeDefinitionByID(id)
	if !ok {
		return 0, false
	}
	return def.Code, true
}

func badgeIDFromCode(code int16) string {
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

func badgeFromParts(code, level, extra int16, owned bool) contracts.PlayerBadge {
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
	if err := upsertUserNotification(ctx, tx, userID, "badge_unlocked", key, map[string]any{"badge": badge}, &notificationID); err != nil {
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
	err := tx.QueryRow(ctx, `
		select level, coalesce(extra, 0)
		from user_badges
		where user_id = $1 and badge_code = $2
		for update
	`, userID, code).Scan(&oldLevel, &oldExtra)
	if errors.Is(err, pgx.ErrNoRows) {
		if _, err := tx.Exec(ctx, `
			insert into user_badges(user_id, badge_code, level, extra, awarded_at, updated_at)
			values($1, $2, $3, nullif($4, 0), now(), now())
		`, userID, code, level, extra); err != nil {
			return false, err
		}
		if notify {
			return notifyBadgeUnlock(ctx, tx, userID, badgeFromParts(code, level, extra, true))
		}
		return true, nil
	}
	if err != nil {
		return false, err
	}
	if level <= oldLevel && extra <= oldExtra {
		return false, nil
	}
	if level < oldLevel {
		level = oldLevel
	}
	if extra < oldExtra {
		extra = oldExtra
	}
	if _, err := tx.Exec(ctx, `
		update user_badges
		set level = $3, extra = nullif($4, 0), updated_at = now()
		where user_id = $1 and badge_code = $2
	`, userID, code, level, extra); err != nil {
		return false, err
	}
	if notify && level > oldLevel {
		return notifyBadgeUnlock(ctx, tx, userID, badgeFromParts(code, level, extra, true))
	}
	return true, nil
}

func awardBadgeTx(ctx context.Context, tx pgx.Tx, userID, badgeID string) (bool, error) {
	code, ok := badgeRefFromID(badgeID)
	if !ok || code == 0 || code == badgeCodeLegacyTopFinish || code == badgeCodeTopFinish {
		return false, errors.New("badge unavailable")
	}
	return upsertBadgeTx(ctx, tx, userID, code, 1, 0, true)
}

func (s *pgStore) GrantBadgeToUser(nickname, badgeID, actorUserID string) (contracts.PlayerBadge, bool, error) {
	nickname = strings.TrimSpace(nickname)
	actorUserID = strings.TrimSpace(actorUserID)
	def, ok := badgeDefinitionByID(badgeID)
	if nickname == "" {
		return contracts.PlayerBadge{}, false, ErrBadgeNicknameRequired
	}
	if !ok || !def.AdminGrantable {
		return contracts.PlayerBadge{}, false, ErrBadgeUnavailable
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return contracts.PlayerBadge{}, false, err
	}
	defer tx.Rollback(ctx)
	var userID string
	if err := tx.QueryRow(ctx, `
		select id
		from users
		where nickname_claimed_at is not null
		  and lower(display_name) = lower($1)
	`, nickname).Scan(&userID); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return contracts.PlayerBadge{}, false, err
	}
	if userID == "" {
		return contracts.PlayerBadge{}, false, ErrBadgeUserNotFound
	}
	changed, err := upsertBadgeTx(ctx, tx, userID, def.Code, 1, 0, true)
	if err != nil {
		return contracts.PlayerBadge{}, false, err
	}
	var level, extra int16
	if err := tx.QueryRow(ctx, `
		select level, coalesce(extra, 0) from user_badges where user_id = $1 and badge_code = $2
	`, userID, def.Code).Scan(&level, &extra); err != nil {
		return contracts.PlayerBadge{}, false, err
	}
	if _, err := tx.Exec(ctx, `
		insert into moderation_log(subject_user_id, actor_user_id, action, reason, metadata)
		values($1, nullif($2, '')::uuid, 'badge_granted', null, jsonb_build_object('badgeId', $3::text, 'source', 'admin'))
	`, userID, actorUserID, def.ID); err != nil {
		return contracts.PlayerBadge{}, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return contracts.PlayerBadge{}, false, err
	}
	return badgeFromParts(def.Code, level, extra, true), changed, nil
}

func awardTopFinishTx(ctx context.Context, tx pgx.Tx, userID string) (bool, error) {
	var count int16
	err := tx.QueryRow(ctx, `
		select coalesce(extra, 0) + 1
		from user_badges
		where user_id = $1 and badge_code = $2
		for update
	`, userID, badgeCodeTopFinish).Scan(&count)
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

func removeGeoDuelsTeamBadgeTx(ctx context.Context, tx pgx.Tx, userID string) error {
	if _, err := tx.Exec(ctx, `
		update users
		set selected_badge_code = null
		where id = $1 and selected_badge_code = $2
	`, userID, badgeCodeGeoDuelsTeam); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `
		delete from user_badges
		where user_id = $1 and badge_code = $2
	`, userID, badgeCodeGeoDuelsTeam)
	return err
}

func (s *pgStore) SyncLoginBadges(userID string) error {
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
	seasonID, err := activeSeasonIDTx(ctx, tx)
	if err != nil {
		return err
	}
	var isGuest, hasTeamRole bool
	var mmr int
	if err := tx.QueryRow(ctx, `
		select
			coalesce(u.account_type = 'guest', false),
			coalesce(u.is_admin, false)
				or coalesce(u.is_moderator, false),
			coalesce(r.mmr, $3)
		from users u
		left join ranks r on r.user_id = u.id and r.mode = $2 and r.season_id = $4
		where u.id = $1
	`, userID, modeDuel, initialMMR, seasonID).Scan(&isGuest, &hasTeamRole, &mmr); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errors.New("user not found")
		}
		return err
	}
	if hasTeamRole {
		if _, err := awardBadgeTx(ctx, tx, userID, "geoduels-team"); err != nil {
			return err
		}
	}
	if !isGuest {
		if err := awardEloBadgesTx(ctx, tx, userID, mmr); err != nil {
			return err
		}
	}
	if err := enqueueDiscordSyncForUserTx(ctx, tx, userID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func awardEloBadgesTx(ctx context.Context, tx pgx.Tx, userID string, mmr int) error {
	thresholds := []struct {
		mmr     int
		badgeID string
	}{
		{1000, "elo-1000"},
		{1500, "elo-1500"},
		{2000, "elo-2000"},
	}
	awardedAny := false
	for _, threshold := range thresholds {
		if mmr >= threshold.mmr {
			awarded, err := awardBadgeTx(ctx, tx, userID, threshold.badgeID)
			if err != nil {
				return err
			}
			awardedAny = awardedAny || awarded
		}
	}
	if awardedAny {
		if err := enqueueDiscordSyncForUserTx(ctx, tx, userID); err != nil {
			return err
		}
	}
	return nil
}

func (s *pgStore) AwardDiscordServerMemberByDiscordID(discordUserID string) (bool, error) {
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
	var userID string
	err = tx.QueryRow(ctx, `
		select user_id
		from user_identities
		where provider = $1 and provider_user_id = $2
		limit 1
	`, IdentityProviderDiscord, discordUserID).Scan(&userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	awarded, err := awardBadgeTx(ctx, tx, userID, "discord-server-member")
	if err != nil {
		return false, err
	}
	return awarded, tx.Commit(ctx)
}

func (s *pgStore) ClaimPendingDiscordSync(now time.Time) (DiscordSyncOutboxItem, bool, error) {
	if now.IsZero() {
		now = time.Now()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return DiscordSyncOutboxItem{}, false, err
	}
	defer tx.Rollback(ctx)
	row := tx.QueryRow(ctx, `
		with candidate as (
			select id
			from discord_sync_outbox
			where processed_at is null
			  and next_attempt_at <= $1
			order by next_attempt_at asc, id asc
			limit 1
			for update skip locked
		)
		update discord_sync_outbox o
		set attempts = o.attempts + 1,
			next_attempt_at = $2,
			last_error = null
		from candidate
		where o.id = candidate.id
		returning o.id, o.action, o.discord_user_id, o.attempts
	`, now, now.Add(5*time.Minute))
	var item DiscordSyncOutboxItem
	if err := row.Scan(&item.ID, &item.Action, &item.DiscordUserID, &item.Attempts); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return DiscordSyncOutboxItem{}, false, nil
		}
		return DiscordSyncOutboxItem{}, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return DiscordSyncOutboxItem{}, false, err
	}
	return item, true, nil
}

func (s *pgStore) MarkDiscordSyncProcessed(id int64) error {
	if id <= 0 {
		return errors.New("discord sync id required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	_, err := s.pool.Exec(ctx, `
		update discord_sync_outbox
		set processed_at = now(),
			last_error = null
		where id = $1
	`, id)
	return err
}

func (s *pgStore) MarkDiscordSyncFailed(id int64, nextAttemptAt time.Time, lastError string) error {
	if id <= 0 {
		return errors.New("discord sync id required")
	}
	lastError = strings.TrimSpace(lastError)
	if len(lastError) > 1000 {
		lastError = lastError[:1000]
	}
	if nextAttemptAt.IsZero() {
		nextAttemptAt = time.Now().Add(time.Minute)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	_, err := s.pool.Exec(ctx, `
		update discord_sync_outbox
		set next_attempt_at = $2,
			last_error = nullif($3, '')
		where id = $1
		  and processed_at is null
	`, id, nextAttemptAt, lastError)
	return err
}

func (s *pgStore) GetDiscordLinkedUser(discordUserID string) (DiscordLinkedUser, bool, error) {
	discordUserID = strings.TrimSpace(discordUserID)
	if discordUserID == "" {
		return DiscordLinkedUser{}, false, errors.New("discord user id required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	row := s.pool.QueryRow(ctx, `
		select
			ui.user_id,
			ui.provider_user_id,
			coalesce(max(case ub.badge_code
				when $3 then 2000
				when $4 then 1500
				when $5 then 1000
				else 0
			end), 0)::int as highest_elo_badge_mmr
		from user_identities ui
		join users u on u.id = ui.user_id
		left join user_badges ub on ub.user_id = ui.user_id
			and ub.badge_code in ($3, $4, $5)
		where ui.provider = $1
		  and ui.provider_user_id = $2
		  and coalesce(u.account_type, 'registered') <> 'guest'
		  and u.deleted_at is null
		group by ui.user_id, ui.provider_user_id
	`, IdentityProviderDiscord, discordUserID, badgeCodeElo2000, badgeCodeElo1500, badgeCodeElo1000)
	var user DiscordLinkedUser
	if err := row.Scan(&user.UserID, &user.DiscordUserID, &user.HighestEloBadgeMMR); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return DiscordLinkedUser{}, false, nil
		}
		return DiscordLinkedUser{}, false, err
	}
	return user, true, nil
}

func (s *pgStore) CreateDonationRef(userID string) (string, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return "", errors.New("user id required")
	}
	ref := "don_" + strings.TrimPrefix(newUserID(), "u_")
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	if _, err := s.pool.Exec(ctx, `
		insert into support_donation_refs(ref, user_id)
		values($1, $2)
	`, ref, userID); err != nil {
		return "", err
	}
	return ref, nil
}

func (s *pgStore) AwardSupporterByDonationRef(ref string) (bool, error) {
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
	var userID string
	err = tx.QueryRow(ctx, `
		update support_donation_refs
		set completed_at = coalesce(completed_at, now())
		where ref = $1
		returning user_id
	`, ref).Scan(&userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	awarded, err := awardBadgeTx(ctx, tx, userID, "supporter")
	if err != nil {
		return false, err
	}
	return awarded, tx.Commit(ctx)
}
