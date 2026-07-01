package persistence

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"geoduels/pkg/contracts"
	"geoduels/pkg/entityid"
)

var ErrPartyMapUnavailable = errors.New("selected map is not accessible or ready")

const partyMapAccessiblePredicate = `id=$1
	and archived_at is null
	and status='ready'
	and (owner_user_id is null or owner_user_id=$2 or published_at is not null or visibility='unlisted')`

func (s *pgStore) CreateParty(ownerUserID string, mode contracts.MatchMode, mapScope string, ttl time.Duration) (contracts.PartySnapshot, error) {
	ownerUserID = strings.TrimSpace(ownerUserID)
	if ownerUserID == "" {
		return contracts.PartySnapshot{}, errors.New("owner required")
	}
	if mode == "" {
		mode = contracts.ModeDuel
	}
	if !contracts.IsPrivatePartyMode(mode) {
		return contracts.PartySnapshot{}, errors.New("unsupported party mode")
	}
	if strings.TrimSpace(mapScope) == "" {
		mapScope = "world"
	}
	if ttl <= 0 {
		ttl = 2 * time.Hour
	}
	mapID, err := s.ResolveGameplayMapID(contracts.ModeDuel, contracts.RulesetMoving, "")
	if err != nil {
		return contracts.PartySnapshot{}, fmt.Errorf("resolve party map: %w", err)
	}
	expiresAt := time.Now().Add(ttl)
	for attempt := 0; attempt < 5; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
		tx, err := s.pool.Begin(ctx)
		if err != nil {
			cancel()
			return contracts.PartySnapshot{}, fmt.Errorf("begin create party tx: %w", err)
		}
		inviteCode := newPartyCode()
		partyID := newPartyID()
		_, err = tx.Exec(ctx, `
			insert into parties(id, invite_code, owner_user_id, state, mode, map_scope, expires_at, map_id)
			values($1, $2, $3, 'open', $4, $5, $6, $7)
		`, partyID, inviteCode, ownerUserID, string(mode), mapScope, expiresAt, mapID)
		if err != nil {
			_ = tx.Rollback(ctx)
			cancel()
			if attempt == 4 {
				return contracts.PartySnapshot{}, fmt.Errorf("insert party: %w", err)
			}
			continue
		}
		if _, err := tx.Exec(ctx, `
		insert into party_members(party_id, user_id, role, ready, team_id)
		values($1, $2, 'owner', false, 'a')
		`, partyID, ownerUserID); err != nil {
			_ = tx.Rollback(ctx)
			cancel()
			return contracts.PartySnapshot{}, fmt.Errorf("insert party owner: %w", err)
		}
		if err := tx.Commit(ctx); err != nil {
			cancel()
			if snap, ok, readErr := s.GetPartyByID(partyID); readErr == nil && ok {
				return snap, nil
			}
			return contracts.PartySnapshot{}, fmt.Errorf("commit create party tx: %w", err)
		}
		cancel()
		snap, _, err := s.GetPartyByID(partyID)
		if err != nil {
			return contracts.PartySnapshot{}, fmt.Errorf("read created party: %w", err)
		}
		return snap, nil
	}
	return contracts.PartySnapshot{}, errors.New("could not allocate party invite code")
}

func (s *pgStore) GetPartyByID(partyID string) (contracts.PartySnapshot, bool, error) {
	return s.getParty("l.id = $1", strings.TrimSpace(partyID))
}

func (s *pgStore) SetPartyMode(partyID string, mode contracts.MatchMode) error {
	partyID = strings.TrimSpace(partyID)
	if partyID == "" || !contracts.IsPrivatePartyMode(mode) {
		return errors.New("invalid party mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	tag, err := s.pool.Exec(ctx, `
		update parties
		set mode = $2, updated_at = now()
		where id = $1 and state = 'open'
	`, partyID, string(mode))
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errors.New("party is not open")
	}
	return nil
}

func (s *pgStore) SetPartyConfig(partyID string, cfg contracts.MatchConfig) (contracts.PartySnapshot, error) {
	partyID = strings.TrimSpace(partyID)
	cfg = contracts.NormalizeMatchConfig(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return contracts.PartySnapshot{}, err
	}
	defer tx.Rollback(ctx)
	var owner string
	if err := tx.QueryRow(ctx, `select owner_user_id from parties where id=$1 and state='open' for update`, partyID).Scan(&owner); err != nil {
		return contracts.PartySnapshot{}, err
	}
	canonicalMapID, _, err := resolveMapIdentity(ctx, tx, cfg.MapID)
	if err != nil {
		return contracts.PartySnapshot{}, ErrPartyMapUnavailable
	}
	cfg.MapID = canonicalMapID
	var accessible bool
	if err := tx.QueryRow(ctx, `select exists(select 1 from maps where `+partyMapAccessiblePredicate+`)`, cfg.MapID, owner).Scan(&accessible); err != nil {
		return contracts.PartySnapshot{}, err
	}
	if !accessible {
		return contracts.PartySnapshot{}, ErrPartyMapUnavailable
	}
	body, _ := json.Marshal(cfg)
	if _, err := tx.Exec(ctx, `update parties set config_json=$2::jsonb, map_id=$3, updated_at=now() where id=$1`, partyID, string(body), cfg.MapID); err != nil {
		return contracts.PartySnapshot{}, err
	}
	if _, err := tx.Exec(ctx, `update party_members set ready=false where party_id=$1`, partyID); err != nil {
		return contracts.PartySnapshot{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return contracts.PartySnapshot{}, err
	}
	snap, _, err := s.GetPartyByID(partyID)
	return snap, err
}

func (s *pgStore) GetPartyByInviteCode(inviteCode string) (contracts.PartySnapshot, bool, error) {
	return s.getParty("l.invite_code = $1", strings.ToUpper(strings.TrimSpace(inviteCode)))
}

func (s *pgStore) GetPartyByMatchID(matchID string) (contracts.PartySnapshot, bool, error) {
	matchID = strings.TrimSpace(matchID)
	if matchID == "" {
		return contracts.PartySnapshot{}, false, nil
	}
	return s.getParty("(l.active_match_id = $1 or l.last_match_id = $1 or l.started_match_id = $1)", matchID)
}

func (s *pgStore) JoinParty(partyID, userID string) (contracts.PartySnapshot, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	if err := s.ensurePartyJoinable(ctx, partyID); err != nil {
		return contracts.PartySnapshot{}, err
	}
	var activeMembers int
	if err := s.pool.QueryRow(ctx, `
		select count(*) from party_members
		where party_id = $1 and left_at is null and user_id <> $2
	`, partyID, userID).Scan(&activeMembers); err != nil {
		return contracts.PartySnapshot{}, err
	}
	if activeMembers >= contracts.MaxPartyMembers {
		return contracts.PartySnapshot{}, errors.New("party is full")
	}
	role := "member"
	if snap, ok, err := s.GetPartyByID(partyID); err != nil {
		return contracts.PartySnapshot{}, err
	} else if !ok {
		return contracts.PartySnapshot{}, pgx.ErrNoRows
	} else if snap.OwnerUserID == userID {
		role = "owner"
	}
	_, err := s.pool.Exec(ctx, `
		insert into party_members(party_id, user_id, role, ready, team_id, left_at)
		values($1, $2, $3, false, (
			select case
				when count(*) filter (where team_id = 'a') <= count(*) filter (where team_id = 'b') then 'a'
				else 'b'
			end
			from party_members
			where party_id = $1 and left_at is null
		), null)
		on conflict (party_id, user_id) do update set
			role = case when party_members.role = 'owner' then 'owner' else excluded.role end,
			team_id = coalesce(party_members.team_id, excluded.team_id),
			left_at = null,
			joined_at = case when party_members.left_at is null then party_members.joined_at else now() end
	`, partyID, userID, role)
	if err != nil {
		return contracts.PartySnapshot{}, err
	}
	if _, err := s.pool.Exec(ctx, `
		update parties set updated_at = now()
		where id = $1 and state in ('open', 'in_match', 'started')
	`, partyID); err != nil {
		return contracts.PartySnapshot{}, err
	}
	snap, _, err := s.GetPartyByID(partyID)
	return snap, err
}

func (s *pgStore) LeaveParty(partyID, userID string) (contracts.PartySnapshot, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return contracts.PartySnapshot{}, err
	}
	defer tx.Rollback(ctx)

	var ownerUserID string
	var state string
	if err := tx.QueryRow(ctx, `
		select owner_user_id, state from parties where id = $1
	`, partyID).Scan(&ownerUserID, &state); err != nil {
		return contracts.PartySnapshot{}, err
	}
	if state != string(contracts.PartyOpen) && state != string(contracts.PartyInMatch) && state != string(contracts.PartyStarted) {
		return contracts.PartySnapshot{}, errors.New("party is not joinable")
	}
	if ownerUserID == userID && state != string(contracts.PartyOpen) {
		return contracts.PartySnapshot{}, errors.New("leader cannot leave the party while a game is in progress")
	}
	tag, err := tx.Exec(ctx, `
		update party_members set left_at = now(), ready = false
		where party_id = $1 and user_id = $2 and left_at is null
	`, partyID, userID)
	if err != nil {
		return contracts.PartySnapshot{}, err
	}
	if tag.RowsAffected() == 0 {
		return contracts.PartySnapshot{}, pgx.ErrNoRows
	}
	if ownerUserID == userID {
		var nextOwner string
		err = tx.QueryRow(ctx, `
			select user_id
			from party_members
			where party_id = $1 and left_at is null
			order by joined_at asc
			limit 1
		`, partyID).Scan(&nextOwner)
		if errors.Is(err, pgx.ErrNoRows) {
			if _, err := tx.Exec(ctx, `
				update parties set state = 'closed', updated_at = now()
				where id = $1 and state = 'open'
			`, partyID); err != nil {
				return contracts.PartySnapshot{}, err
			}
		} else if err != nil {
			return contracts.PartySnapshot{}, err
		} else {
			if err := transferPartyOwnerTx(ctx, tx, partyID, userID, nextOwner); err != nil {
				return contracts.PartySnapshot{}, err
			}
		}
	}
	if _, err := tx.Exec(ctx, `
		update parties set updated_at = now()
		where id = $1 and state in ('open', 'in_match', 'started')
	`, partyID); err != nil {
		return contracts.PartySnapshot{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return contracts.PartySnapshot{}, err
	}
	next, _, err := s.GetPartyByID(partyID)
	return next, err
}

func (s *pgStore) SetPartyMemberTeam(partyID, userID, teamID string) (contracts.PartySnapshot, error) {
	partyID = strings.TrimSpace(partyID)
	userID = strings.TrimSpace(userID)
	teamID = strings.ToLower(strings.TrimSpace(teamID))
	if partyID == "" || userID == "" || (teamID != "a" && teamID != "b") {
		return contracts.PartySnapshot{}, errors.New("invalid party team")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	if err := s.ensurePartyOpen(ctx, partyID); err != nil {
		return contracts.PartySnapshot{}, err
	}
	tag, err := s.pool.Exec(ctx, `
		update party_members
		set team_id = $3
		where party_id = $1 and user_id = $2 and left_at is null
	`, partyID, userID, teamID)
	if err != nil {
		return contracts.PartySnapshot{}, err
	}
	if tag.RowsAffected() == 0 {
		return contracts.PartySnapshot{}, pgx.ErrNoRows
	}
	if _, err := s.pool.Exec(ctx, `
		update parties set updated_at = now()
		where id = $1 and state = 'open'
	`, partyID); err != nil {
		return contracts.PartySnapshot{}, err
	}
	snap, _, err := s.GetPartyByID(partyID)
	return snap, err
}

func (s *pgStore) ExpireOpenParties() error {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	_, err := s.pool.Exec(ctx, `
		update parties
		set state = 'expired',
			updated_at = now()
		where state = 'open'
		  and expires_at < now()
	`)
	return err
}

func (s *pgStore) ListOpenPartyIDs() ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	rows, err := s.pool.Query(ctx, `
		select id
		from parties
		where state = 'open'
		order by updated_at asc
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (s *pgStore) CloseInactiveOpenParties(partyIDs []string, inactiveFor time.Duration) (int64, error) {
	if len(partyIDs) == 0 || inactiveFor <= 0 {
		return 0, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	tag, err := s.pool.Exec(ctx, `
		update parties
		set state = 'closed',
			updated_at = now()
		where state = 'open'
		  and id = any($1)
		  and updated_at < now() - ($2::double precision * interval '1 second')
	`, partyIDs, inactiveFor.Seconds())
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

func (s *pgStore) KickPartyMember(partyID, ownerUserID, targetUserID string) (contracts.PartySnapshot, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	ownerUserID = strings.TrimSpace(ownerUserID)
	targetUserID = strings.TrimSpace(targetUserID)
	if ownerUserID == "" || targetUserID == "" || ownerUserID == targetUserID {
		return contracts.PartySnapshot{}, errors.New("invalid party member")
	}
	if err := s.ensurePartyOwner(ctx, partyID, ownerUserID); err != nil {
		return contracts.PartySnapshot{}, err
	}
	tag, err := s.pool.Exec(ctx, `
		update party_members set left_at = now(), ready = false
		where party_id = $1 and user_id = $2 and role <> 'owner' and left_at is null
	`, partyID, targetUserID)
	if err != nil {
		return contracts.PartySnapshot{}, err
	}
	if tag.RowsAffected() == 0 {
		return contracts.PartySnapshot{}, pgx.ErrNoRows
	}
	snap, _, err := s.GetPartyByID(partyID)
	return snap, err
}

func (s *pgStore) TransferPartyOwner(partyID, ownerUserID, targetUserID string) (contracts.PartySnapshot, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return contracts.PartySnapshot{}, err
	}
	defer tx.Rollback(ctx)
	if err := ensurePartyOwnerTx(ctx, tx, partyID, ownerUserID); err != nil {
		return contracts.PartySnapshot{}, err
	}
	if err := transferPartyOwnerTx(ctx, tx, partyID, ownerUserID, targetUserID); err != nil {
		return contracts.PartySnapshot{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return contracts.PartySnapshot{}, err
	}
	snap, _, err := s.GetPartyByID(partyID)
	return snap, err
}

func (s *pgStore) MarkPartyInMatch(partyID, matchID string) (contracts.PartySnapshot, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	tag, err := s.pool.Exec(ctx, `
		update parties
		set state = 'in_match',
			active_match_id = $2,
			started_match_id = $2,
			updated_at = now()
		where id = $1 and state = 'open'
	`, partyID, matchID)
	if err != nil {
		return contracts.PartySnapshot{}, err
	}
	if tag.RowsAffected() == 0 {
		return contracts.PartySnapshot{}, pgx.ErrNoRows
	}
	snap, _, err := s.GetPartyByID(partyID)
	return snap, err
}

func (s *pgStore) ReopenEndedParties() (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	tag, err := s.pool.Exec(ctx, `
		with ended as (
			select l.id, l.active_match_id
			from parties l
			join runtime_matches rm on rm.id = l.active_match_id
			where l.state in ('in_match', 'started')
			  and rm.state = $1
		),
		reopened as (
			update parties l
			set state = 'open',
				last_match_id = ended.active_match_id,
				active_match_id = null,
				started_match_id = null,
				updated_at = now()
			from ended
			where l.id = ended.id
			returning l.id
		)
		update party_members m
		set ready = false
		from reopened
		where m.party_id = reopened.id
	`, string(contracts.MatchEnded))
	if err != nil {
		return 0, err
	}
	if _, err := s.pool.Exec(ctx, `
		update match_sessions ms
		set state = 'ended',
			ended_at = coalesce(ms.ended_at, now()),
			updated_at = now()
		from runtime_matches rm
		where rm.id = ms.match_id
		  and rm.state = $1
		  and ms.state <> 'ended'
	`, string(contracts.MatchEnded)); err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

func (s *pgStore) ensurePartyOpen(ctx context.Context, partyID string) error {
	var state string
	var expiresAt time.Time
	err := s.pool.QueryRow(ctx, `select state, expires_at from parties where id = $1`, partyID).Scan(&state, &expiresAt)
	if err != nil {
		return err
	}
	if state != string(contracts.PartyOpen) {
		return errors.New("party is not open")
	}
	if time.Now().After(expiresAt) {
		_, _ = s.pool.Exec(ctx, `update parties set state = 'expired', updated_at = now() where id = $1 and state = 'open'`, partyID)
		return errors.New("party expired")
	}
	return nil
}

func (s *pgStore) ensurePartyJoinable(ctx context.Context, partyID string) error {
	var state string
	var expiresAt time.Time
	err := s.pool.QueryRow(ctx, `select state, expires_at from parties where id = $1`, partyID).Scan(&state, &expiresAt)
	if err != nil {
		return err
	}
	if state != string(contracts.PartyOpen) && state != string(contracts.PartyInMatch) && state != string(contracts.PartyStarted) {
		return errors.New("party is not joinable")
	}
	if time.Now().After(expiresAt) {
		return errors.New("party expired")
	}
	return nil
}

func (s *pgStore) ensurePartyOwner(ctx context.Context, partyID, ownerUserID string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	return ensurePartyOwnerTx(ctx, tx, partyID, ownerUserID)
}

type partyTx interface {
	QueryRow(context.Context, string, ...any) pgx.Row
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}

func ensurePartyOwnerTx(ctx context.Context, tx partyTx, partyID, ownerUserID string) error {
	var state string
	var actualOwner string
	if err := tx.QueryRow(ctx, `
		select state, owner_user_id from parties where id = $1
	`, partyID).Scan(&state, &actualOwner); err != nil {
		return err
	}
	if state != string(contracts.PartyOpen) {
		return errors.New("party is not open")
	}
	if actualOwner != ownerUserID {
		return errors.New("forbidden")
	}
	return nil
}

func transferPartyOwnerTx(ctx context.Context, tx partyTx, partyID, ownerUserID, targetUserID string) error {
	ownerUserID = strings.TrimSpace(ownerUserID)
	targetUserID = strings.TrimSpace(targetUserID)
	if ownerUserID == "" || targetUserID == "" || ownerUserID == targetUserID {
		return errors.New("invalid party member")
	}
	var targetActive bool
	if err := tx.QueryRow(ctx, `
		select exists(
			select 1 from party_members
			where party_id = $1 and user_id = $2 and left_at is null
		)
	`, partyID, targetUserID).Scan(&targetActive); err != nil {
		return err
	}
	if !targetActive {
		return pgx.ErrNoRows
	}
	if _, err := tx.Exec(ctx, `
		update parties set owner_user_id = $2, updated_at = now()
		where id = $1
	`, partyID, targetUserID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		update party_members
		set role = case when user_id = $2 then 'owner' else 'member' end
		where party_id = $1 and left_at is null
	`, partyID, targetUserID); err != nil {
		return err
	}
	return nil
}

func (s *pgStore) getParty(whereClause, value string) (contracts.PartySnapshot, bool, error) {
	if strings.TrimSpace(value) == "" {
		return contracts.PartySnapshot{}, false, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	var snap contracts.PartySnapshot
	row := s.pool.QueryRow(ctx, partyReadQuery(whereClause), value)
	var configJSON, mapID string
	if err := row.Scan(&snap.ID, &snap.InviteCode, &snap.OwnerUserID, &snap.State, &snap.Mode, &snap.MapScope, &snap.ActiveMatchID, &snap.LastMatchID, &snap.StartedMatchID, &snap.CreatedAt, &snap.ExpiresAt, &configJSON, &mapID, &snap.MapName, &snap.MapLocationCount); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return contracts.PartySnapshot{}, false, nil
		}
		return contracts.PartySnapshot{}, false, err
	}
	_ = json.Unmarshal([]byte(configJSON), &snap.Config)
	snap.Config = contracts.NormalizeMatchConfig(snap.Config)
	if mapID != "" {
		snap.Config.MapID = mapID
		snap.Config.MapName = snap.MapName
	}
	if snap.StartedMatchID == "" {
		snap.StartedMatchID = snap.ActiveMatchID
	}
	members, err := s.listPartyMembers(ctx, snap.ID)
	if err != nil {
		return contracts.PartySnapshot{}, false, err
	}
	snap.Members = members
	return snap, true, nil
}

func partyReadQuery(whereClause string) string {
	return `
		select l.id, l.invite_code, l.owner_user_id, l.state, l.mode, l.map_scope,
		       coalesce(l.active_match_id::text, l.started_match_id::text, ''),
		       coalesce(l.last_match_id::text, ''),
		       coalesce(l.started_match_id::text, ''),
		       l.created_at, l.expires_at, l.config_json::text, coalesce(l.map_id::text, ''),
		       coalesce(mp.display_name, ''), coalesce(mp.location_count, 0)
		from parties l
		left join maps mp on mp.id = l.map_id
		where ` + whereClause
}

func (s *pgStore) listPartyMembers(ctx context.Context, partyID string) ([]contracts.PartyMember, error) {
	rows, err := s.pool.Query(ctx, `
		select m.user_id, u.display_name, coalesce(u.avatar_url, ''),
			u.account_type = 'guest',
			coalesce(u.is_admin, false),
			coalesce(u.selected_badge_code, 0),
			coalesce(u.selected_badge_season_id, ''),
			coalesce(m.team_id, ''),
			m.role, m.ready, m.joined_at
		from party_members m
		join users u on u.id = m.user_id
		where m.party_id = $1 and m.left_at is null
		order by case when m.role = 'owner' then 0 else 1 end, m.joined_at asc
	`, partyID)
	if err != nil {
		return nil, err
	}
	out := []contracts.PartyMember{}
	selected := map[string]string{}
	for rows.Next() {
		var member contracts.PartyMember
		var selectedBadgeCode int16
		var selectedBadgeSeasonID string
		if err := rows.Scan(&member.UserID, &member.DisplayName, &member.AvatarURL, &member.IsGuest, &member.IsAdmin, &selectedBadgeCode, &selectedBadgeSeasonID, &member.TeamID, &member.Role, &member.Ready, &member.JoinedAt); err != nil {
			rows.Close()
			return nil, err
		}
		selected[member.UserID] = badgeIDFromParts(selectedBadgeCode, selectedBadgeSeasonID)
		out = append(out, member)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	badges, err := s.selectedPartyBadges(ctx, selected)
	if err != nil {
		return nil, err
	}
	for i := range out {
		out[i].SelectedBadge = badges[out[i].UserID]
	}
	return out, nil
}

func (s *pgStore) selectedPartyBadges(ctx context.Context, selected map[string]string) (map[string]*contracts.PlayerBadge, error) {
	if len(selected) == 0 {
		return map[string]*contracts.PlayerBadge{}, nil
	}
	userIDs := make([]string, 0, len(selected))
	for userID := range selected {
		if strings.TrimSpace(userID) != "" {
			userIDs = append(userIDs, userID)
		}
	}
	if len(userIDs) == 0 {
		return map[string]*contracts.PlayerBadge{}, nil
	}
	rows, err := s.pool.Query(ctx, `
		select ub.user_id, ub.badge_code, coalesce(ub.badge_season_id, ''), coalesce(ub.rank, 0)
		from user_badges ub
		where ub.user_id = any($1)
		order by ub.user_id asc, ub.awarded_at desc, ub.badge_code asc, ub.badge_season_id asc
	`, userIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[string]*contracts.PlayerBadge{}
	fallback := map[string]*contracts.PlayerBadge{}
	for rows.Next() {
		var userID string
		var code int16
		var seasonID string
		var rank int
		if err := rows.Scan(&userID, &code, &seasonID, &rank); err != nil {
			return nil, err
		}
		if code == badgeCodeSeasonRank {
			continue
		}
		badge, ok := badgeFromParts(code, seasonID, rank, true)
		if !ok {
			continue
		}
		if fallback[userID] == nil {
			b := badge
			fallback[userID] = &b
		}
		if out[userID] == nil && badge.ID == selected[userID] {
			b := badge
			out[userID] = &b
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	seasonBadges, err := s.earnedSeasonRankBadges(ctx, userIDs)
	if err != nil {
		return nil, err
	}
	for userID, badges := range seasonBadges {
		for _, badge := range badges {
			if fallback[userID] == nil {
				b := badge
				fallback[userID] = &b
			}
			if out[userID] == nil && badge.ID == selected[userID] {
				b := badge
				out[userID] = &b
			}
		}
	}
	for userID, badge := range fallback {
		if out[userID] == nil && selected[userID] != "" {
			out[userID] = badge
		}
	}
	return out, nil
}

func newPartyID() string {
	return entityid.New()
}

func newPartyCode() string {
	const alphabet = "23456789ABCDEFGHJKMNPQRSTUVWXYZ"
	buf := make([]byte, 6)
	random := make([]byte, 6)
	if _, err := rand.Read(random); err != nil {
		fallback := strings.ToUpper(time.Now().Format("150405"))
		return fallback[:6]
	}
	for i, b := range random {
		buf[i] = alphabet[int(b)%len(alphabet)]
	}
	return string(buf)
}
