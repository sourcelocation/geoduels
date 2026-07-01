package persistence

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"geoduels/pkg/contracts"
)

type badgeDefinition struct {
	ID           string
	Code         int16
	Kind         string
	Label        string
	Description  string
	ImageURL     string
	Rarity       string
	Unobtainable bool
}

var badgeDefinitions = []badgeDefinition{
	{
		ID:           "discord-member",
		Code:         badgeCodeDiscordMember,
		Kind:         "community",
		Label:        "Discord Member",
		Description:  "Retired badge previously awarded for linking Discord to your GeoDuels account.",
		ImageURL:     "/medals/discord-medal.v1.png",
		Rarity:       "common",
		Unobtainable: true,
	},
	{
		ID:          "geoduels-team",
		Code:        badgeCodeGeoDuelsTeam,
		Kind:        "special",
		Label:       "GeoDuels Team",
		Description: "An exclusive medal for GeoDuels moderators and team members.",
		ImageURL:    "/medals/team-badge.v1.png",
		Rarity:      "special",
	},
	{
		ID:          "discord-server-member",
		Code:        badgeCodeDiscordServerMember,
		Kind:        "community",
		Label:       "Discord Server Member",
		Description: "Awarded for joining the official GeoDuels Discord server.",
		ImageURL:    "/medals/discord-new-badge.v1.png",
		Rarity:      "common",
	},
	{
		ID:          "supporter",
		Code:        badgeCodeSupporter,
		Kind:        "supporter",
		Label:       "Supporter",
		Description: "Awarded for supporting GeoDuels.",
		ImageURL:    "/medals/supporter-badge.v2.png",
		Rarity:      "rare",
	},
	{
		ID:          "speedrunner",
		Code:        badgeCodeSpeedrunner,
		Kind:        "achievement",
		Label:       "Speedrunner",
		Description: "Awarded for scoring 5000 points in under 30 seconds in ranked.",
		ImageURL:    "/medals/speedrunner-badge.v2.png",
		Rarity:      "epic",
	},
	{
		ID:          "elo-1000",
		Code:        badgeCodeElo1000,
		Kind:        "ranked",
		Label:       "1000 Elo",
		Description: "Awarded for reaching 1000 Elo.",
		ImageURL:    "/medals/1k-medal.v1.png",
		Rarity:      "common",
	},
	{
		ID:          "elo-1500",
		Code:        badgeCodeElo1500,
		Kind:        "ranked",
		Label:       "1500 Elo",
		Description: "Awarded for reaching 1500 Elo.",
		ImageURL:    "/medals/1.5k-medal.v1.png",
		Rarity:      "rare",
	},
	{
		ID:          "elo-2000",
		Code:        badgeCodeElo2000,
		Kind:        "ranked",
		Label:       "2000 Elo",
		Description: "Awarded for reaching 2000 Elo.",
		ImageURL:    "/medals/2k-medal.v1.png",
		Rarity:      "legendary",
	},
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
		if def.Unobtainable {
			continue
		}
		out = append(out, badgeFromDefinition(def, false))
	}
	return out
}

func seasonRankBadgeTemplate(seasonID string) contracts.PlayerBadge {
	displaySeason := seasonBadgeDisplayName(seasonID)
	return contracts.PlayerBadge{
		ID:          seasonRankBadgeID(seasonID),
		Kind:        "season_rank",
		Label:       displaySeason + " Top 100",
		Description: "Awarded to players who finish in the top 100 when " + displaySeason + " ends.",
		ImageURL:    "/medals/platinum-medal.v1.png",
		Rarity:      "legendary",
		SeasonID:    seasonID,
		Owned:       false,
	}
}

func seasonRankBadgeID(seasonID string) string {
	return "season-" + strings.TrimSpace(seasonID) + "-top-100"
}

type badgeRef struct {
	Code     int16
	SeasonID string
}

func badgeRefFromID(id string) (badgeRef, bool) {
	id = strings.TrimSpace(id)
	switch id {
	case "":
		return badgeRef{}, true
	default:
		if def, ok := badgeDefinitionByID(id); ok {
			return badgeRef{Code: def.Code}, true
		}
		if strings.HasPrefix(id, "season-") && strings.HasSuffix(id, "-top-100") {
			seasonID := strings.TrimSuffix(strings.TrimPrefix(id, "season-"), "-top-100")
			if strings.TrimSpace(seasonID) == "" {
				return badgeRef{}, false
			}
			return badgeRef{Code: badgeCodeSeasonRank, SeasonID: seasonID}, true
		}
		return badgeRef{}, false
	}
}

func badgeIDFromParts(code int16, seasonID string) string {
	if code == badgeCodeSeasonRank {
		if strings.TrimSpace(seasonID) != "" {
			return seasonRankBadgeID(seasonID)
		}
		return ""
	}
	if def, ok := badgeDefinitionByCode(code); ok {
		return def.ID
	}
	return ""
}

func badgeFromDefinition(def badgeDefinition, owned bool) contracts.PlayerBadge {
	return contracts.PlayerBadge{
		ID:           def.ID,
		Kind:         def.Kind,
		Label:        def.Label,
		Description:  def.Description,
		ImageURL:     def.ImageURL,
		Rarity:       def.Rarity,
		Owned:        owned,
		Unobtainable: def.Unobtainable,
	}
}

func badgeFromParts(code int16, seasonID string, rank int, owned bool) (contracts.PlayerBadge, bool) {
	if code == badgeCodeSeasonRank {
		badge := seasonRankBadgeTemplate(seasonID)
		badge.Rank = rank
		badge.Owned = owned
		if owned && rank > 0 {
			displaySeason := seasonBadgeDisplayName(seasonID)
			badge.Label = displaySeason + " #" + fmt.Sprint(rank)
			badge.Description = "Finished #" + fmt.Sprint(rank) + " in " + displaySeason + "."
		}
		return badge, strings.TrimSpace(seasonID) != ""
	}
	if def, ok := badgeDefinitionByCode(code); ok {
		return badgeFromDefinition(def, owned), true
	}
	return contracts.PlayerBadge{}, false
}

func seasonBadgeDisplayName(seasonID string) string {
	switch strings.TrimSpace(seasonID) {
	case "s2":
		return "Season 1"
	case "s2.5":
		return "Season 2"
	default:
		value := strings.TrimPrefix(strings.TrimSpace(seasonID), "s")
		if value == "" {
			return "Season"
		}
		return "Season " + strings.ToUpper(value)
	}
}

func (s *pgStore) earnedSeasonRankBadges(ctx context.Context, userIDs []string) (map[string][]contracts.PlayerBadge, error) {
	if len(userIDs) == 0 {
		return map[string][]contracts.PlayerBadge{}, nil
	}
	activeSeasonID, err := activeSeasonIDTx(ctx, s.pool)
	if err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `
		with ranked as (
			select
				r.user_id,
				r.season_id,
				row_number() over (
					partition by r.season_id
					order by r.mmr desc, r.updated_at asc, r.user_id asc
				)::int as rank
			from ranks r
			join users u on u.id = r.user_id
			where r.mode = $1
				and r.season_id <> $2
				and coalesce(u.account_type, 'registered') <> 'guest'
				and not coalesce(u.banned_at is not null and (u.ban_expires_at is null or u.ban_expires_at > now()), false)
		)
		select user_id, season_id, rank
		from ranked
		where user_id = any($3)
			and rank between 1 and 100
		order by user_id asc, season_id desc
	`, modeDuel, activeSeasonID, userIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string][]contracts.PlayerBadge{}
	for rows.Next() {
		var userID, seasonID string
		var rank int
		if err := rows.Scan(&userID, &seasonID, &rank); err != nil {
			return nil, err
		}
		badge, ok := badgeFromParts(badgeCodeSeasonRank, seasonID, rank, true)
		if !ok {
			continue
		}
		out[userID] = append(out[userID], badge)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func awardBadgeTx(ctx context.Context, tx pgx.Tx, userID, badgeID string) (bool, error) {
	ref, ok := badgeRefFromID(badgeID)
	if !ok || ref.Code == 0 || ref.Code == badgeCodeSeasonRank {
		return false, errors.New("badge unavailable")
	}
	tag, err := tx.Exec(ctx, `
		insert into user_badges(user_id, badge_code)
		values(
			$1,
			$2
		)
		on conflict (user_id, badge_code, badge_season_id) do nothing
	`, userID, ref.Code)
	if err != nil {
		return false, err
	}
	awarded := tag.RowsAffected() > 0
	if !awarded {
		return false, nil
	}
	badge, ok := badgeFromParts(ref.Code, ref.SeasonID, 0, true)
	if !ok {
		return false, nil
	}
	var notificationID int64
	if err := upsertUserNotification(ctx, tx, userID, "badge_unlocked", "badge_unlocked:"+userID+":"+badge.ID, map[string]any{
		"badge": badge,
	}, &notificationID); err != nil {
		return false, err
	}
	return true, nil
}

func removeGeoDuelsTeamBadgeTx(ctx context.Context, tx pgx.Tx, userID string) error {
	if _, err := tx.Exec(ctx, `
		update users
		set selected_badge_code = null,
			selected_badge_season_id = ''
		where id = $1
			and selected_badge_code = $2
	`, userID, badgeCodeGeoDuelsTeam); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `
		delete from user_badges
		where user_id = $1
			and badge_code = $2
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
				or coalesce(u.is_moderator, false)
				or exists (
					select 1
					from user_roles ur
					where ur.user_id = u.id
					  and ur.role in ('admin', 'moderator')
					  and ur.revoked_at is null
				),
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
