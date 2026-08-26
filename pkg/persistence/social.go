package persistence

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"geoduels/pkg/entityid"
)

var (
	ErrSocialNotFound = errors.New("social resource not found")
	ErrSocialBlocked  = errors.New("social action unavailable")
	ErrSocialLimit    = errors.New("social limit reached")
)

const friendCodeAlphabet = "23456789ABCDEFGHJKMNPQRSTUVWXYZ"

type SocialRepository interface {
	GetSocialAccount(userID string) (isGuest, requestsEnabled, invitesEnabled bool, err error)
	GetSocialSettings(userID string) (SocialSettings, error)
	UpdateSocialSettings(userID string, settings SocialSettings) (SocialSettings, error)
	Relationship(userID, targetID string) (RelationshipState, string, error)
	ListFriends(userID string, limit int) ([]CompactPlayer, error)
	ListFriendRequests(userID string, direction string, limit int) ([]FriendRequest, error)
	SearchSocialPlayers(userID, query string, limit int) ([]CompactPlayer, error)
	ListRecentPlayers(userID string, limit int) ([]CompactPlayer, error)
	SendFriendRequest(userID, targetID string) (FriendRequest, error)
	RespondFriendRequest(userID, requestID, response string) error
	RemoveFriend(userID, targetID string) error
	SetUserBlock(userID, targetID string, blocked bool) error
	CreateFriendCode(userID string, ttl time.Duration) (FriendCode, error)
	ResolveFriendCode(userID, code string) (CompactPlayer, error)
	CreatePartyInvitation(partyID, inviterID, recipientID string, ttl time.Duration) (PartyInvitation, error)
	RespondPartyInvitation(userID, invitationID, response string) (PartyInvitation, error)
	ListPartyInvitations(userID string, limit int) ([]PartyInvitation, error)
	TouchLastSeen(userID string, seenAt time.Time) error
	AppendUserEvent(userID, eventType string, payload any) (int64, error)
	ListUserEvents(userID string, after int64, limit int) ([]UserEvent, error)
}

func (s *pgStore) GetSocialSettings(userID string) (SocialSettings, error) {
	var settings SocialSettings
	err := s.pool.QueryRow(context.Background(), `
		select social_discoverable,social_presence_visible,social_requests_enabled,social_party_invites_enabled
		from users where id=$1
	`, userID).Scan(&settings.Discoverable, &settings.PresenceVisible, &settings.RequestsEnabled, &settings.PartyInvitesEnabled)
	return settings, err
}

func (s *pgStore) UpdateSocialSettings(userID string, settings SocialSettings) (SocialSettings, error) {
	err := s.pool.QueryRow(context.Background(), `
		update users set social_discoverable=$2,social_presence_visible=$3,
		  social_requests_enabled=$4,social_party_invites_enabled=$5
		where id=$1 and account_type='registered'
		returning social_discoverable,social_presence_visible,social_requests_enabled,social_party_invites_enabled
	`, userID, settings.Discoverable, settings.PresenceVisible, settings.RequestsEnabled, settings.PartyInvitesEnabled).
		Scan(&settings.Discoverable, &settings.PresenceVisible, &settings.RequestsEnabled, &settings.PartyInvitesEnabled)
	return settings, err
}

func (s *pgStore) GetSocialAccount(userID string) (bool, bool, bool, error) {
	var accountType string
	var requestsEnabled, invitesEnabled bool
	err := s.pool.QueryRow(context.Background(), `
		select account_type, social_requests_enabled, social_party_invites_enabled
		from users where id=$1
	`, userID).Scan(&accountType, &requestsEnabled, &invitesEnabled)
	return accountType == "guest", requestsEnabled, invitesEnabled, err
}

func (s *pgStore) Relationship(userID, targetID string) (RelationshipState, string, error) {
	if userID == targetID {
		return RelationshipNone, "", nil
	}
	var blockedByViewer, blockedByTarget, friends bool
	var requestID, senderID string
	err := s.pool.QueryRow(context.Background(), `
		select
			exists(select 1 from user_blocks where blocker_user_id=$1 and blocked_user_id=$2),
			exists(select 1 from user_blocks where blocker_user_id=$2 and blocked_user_id=$1),
			exists(select 1 from friendships where user_id_low=least($1::uuid,$2::uuid) and user_id_high=greatest($1::uuid,$2::uuid)),
			coalesce((select id::text from friend_requests where status='pending'
				and least(sender_user_id,recipient_user_id)=least($1::uuid,$2::uuid)
				and greatest(sender_user_id,recipient_user_id)=greatest($1::uuid,$2::uuid) limit 1),''),
			coalesce((select sender_user_id::text from friend_requests where status='pending'
				and least(sender_user_id,recipient_user_id)=least($1::uuid,$2::uuid)
				and greatest(sender_user_id,recipient_user_id)=greatest($1::uuid,$2::uuid) limit 1),'')
	`, userID, targetID).Scan(&blockedByViewer, &blockedByTarget, &friends, &requestID, &senderID)
	if err != nil {
		return RelationshipNone, "", err
	}
	if blockedByViewer {
		return RelationshipBlocked, "", nil
	}
	if blockedByTarget {
		return RelationshipNone, "", nil
	}
	if friends {
		return RelationshipFriends, "", nil
	}
	if requestID != "" {
		if senderID == userID {
			return RelationshipOutgoing, requestID, nil
		}
		return RelationshipIncoming, requestID, nil
	}
	return RelationshipNone, "", nil
}

func (s *pgStore) ListFriends(userID string, limit int) ([]CompactPlayer, error) {
	limit = boundedSocialLimit(limit, 100, 500)
	rows, err := s.pool.Query(context.Background(), `
		with friend_ids as (
			select case when user_id_low=$1 then user_id_high else user_id_low end user_id, created_at
			from friendships where user_id_low=$1 or user_id_high=$1
		)
		select u.id::text, coalesce(nullif(u.display_name,''),u.id::text), coalesce(u.avatar_url,''),
			coalesce(r.mmr,1000), case when u.social_presence_visible then u.last_seen_at end
		from friend_ids f join users u on u.id=f.user_id
		left join ranks r on r.user_id=u.id and r.mode='duel'
		where u.account_type='registered'
		order by f.created_at desc limit $2
	`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []CompactPlayer{}
	for rows.Next() {
		var p CompactPlayer
		if err := rows.Scan(&p.UserID, &p.DisplayName, &p.AvatarURL, &p.MMR, &p.LastSeenAt); err != nil {
			return nil, err
		}
		p.Relationship = RelationshipFriends
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *pgStore) ListFriendRequests(userID, direction string, limit int) ([]FriendRequest, error) {
	limit = boundedSocialLimit(limit, 20, 100)
	column := "recipient_user_id"
	playerColumn := "sender_user_id"
	if direction == "outgoing" {
		column, playerColumn = "sender_user_id", "recipient_user_id"
	}
	rows, err := s.pool.Query(context.Background(), `
		select fr.id::text, u.id::text, coalesce(nullif(u.display_name,''),u.id::text),
			coalesce(u.avatar_url,''), coalesce(r.mmr,1000), u.last_seen_at, fr.created_at, fr.expires_at
		from friend_requests fr
		join users u on u.id=fr.`+playerColumn+`
		left join ranks r on r.user_id=u.id and r.mode='duel'
		where fr.`+column+`=$1 and fr.status='pending' and fr.expires_at>now()
		order by fr.created_at desc limit $2
	`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []FriendRequest{}
	for rows.Next() {
		var item FriendRequest
		item.Direction = direction
		if err := rows.Scan(&item.ID, &item.Player.UserID, &item.Player.DisplayName, &item.Player.AvatarURL,
			&item.Player.MMR, &item.Player.LastSeenAt, &item.CreatedAt, &item.ExpiresAt); err != nil {
			return nil, err
		}
		item.Player.RequestID = item.ID
		if direction == "outgoing" {
			item.Player.Relationship = RelationshipOutgoing
		} else {
			item.Player.Relationship = RelationshipIncoming
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *pgStore) SearchSocialPlayers(userID, query string, limit int) ([]CompactPlayer, error) {
	query = strings.TrimSpace(query)
	if len([]rune(query)) < 2 {
		return []CompactPlayer{}, nil
	}
	limit = boundedSocialLimit(limit, 10, 20)
	rows, err := s.pool.Query(context.Background(), `
		select u.id::text, u.display_name, coalesce(u.avatar_url,''), coalesce(r.mmr,1000), u.last_seen_at
		from users u left join ranks r on r.user_id=u.id and r.mode='duel'
		where u.id<>$1 and u.account_type='registered' and u.nickname_claimed_at is not null
		  and u.social_discoverable and lower(u.display_name) like lower($2)||'%'
		  and not exists(select 1 from user_blocks b where
		    (b.blocker_user_id=$1 and b.blocked_user_id=u.id) or
		    (b.blocker_user_id=u.id and b.blocked_user_id=$1))
		order by (lower(u.display_name)=lower($2)) desc, length(u.display_name), lower(u.display_name)
		limit $3
	`, userID, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return s.scanSocialPlayers(rows, userID)
}

func (s *pgStore) ListRecentPlayers(userID string, limit int) ([]CompactPlayer, error) {
	limit = boundedSocialLimit(limit, 3, 3)
	rows, err := s.pool.Query(context.Background(), `
		with recent as (
			select mp2.user_id, max(h.ended_at) shared_at
			from match_players mine
			join match_history h on h.match_id=mine.match_id
			join match_players mp2 on mp2.match_id=h.match_id and mp2.user_id<>$1
			where mine.user_id=$1 and h.ended_at is not null
			group by mp2.user_id
		)
		select u.id::text, u.display_name, coalesce(u.avatar_url,''), coalesce(r.mmr,1000), u.last_seen_at, recent.shared_at
		from recent join users u on u.id=recent.user_id
		left join ranks r on r.user_id=u.id and r.mode='duel'
		where u.account_type='registered' and u.nickname_claimed_at is not null and u.social_discoverable
		  and not exists(select 1 from friendships f where f.user_id_low=least($1::uuid,u.id) and f.user_id_high=greatest($1::uuid,u.id))
		  and not exists(select 1 from friend_requests fr where fr.status='pending'
		    and least(fr.sender_user_id,fr.recipient_user_id)=least($1::uuid,u.id)
		    and greatest(fr.sender_user_id,fr.recipient_user_id)=greatest($1::uuid,u.id))
		  and not exists(select 1 from user_blocks b where
		    (b.blocker_user_id=$1 and b.blocked_user_id=u.id) or (b.blocker_user_id=u.id and b.blocked_user_id=$1))
		order by recent.shared_at desc limit $2
	`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []CompactPlayer{}
	for rows.Next() {
		var p CompactPlayer
		if err := rows.Scan(&p.UserID, &p.DisplayName, &p.AvatarURL, &p.MMR, &p.LastSeenAt, &p.SharedMatchAt); err != nil {
			return nil, err
		}
		p.Relationship = RelationshipNone
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *pgStore) scanSocialPlayers(rows pgx.Rows, viewerID string) ([]CompactPlayer, error) {
	out := []CompactPlayer{}
	for rows.Next() {
		var p CompactPlayer
		if err := rows.Scan(&p.UserID, &p.DisplayName, &p.AvatarURL, &p.MMR, &p.LastSeenAt); err != nil {
			return nil, err
		}
		p.Relationship, p.RequestID, _ = s.Relationship(viewerID, p.UserID)
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *pgStore) SendFriendRequest(userID, targetID string) (FriendRequest, error) {
	if userID == targetID {
		return FriendRequest{}, ErrSocialBlocked
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return FriendRequest{}, err
	}
	defer tx.Rollback(ctx)
	var allowed bool
	if err := tx.QueryRow(ctx, `
		select exists(select 1 from users where id=$2 and account_type='registered' and social_requests_enabled)
		  and not exists(select 1 from user_blocks where
		    (blocker_user_id=$1 and blocked_user_id=$2) or (blocker_user_id=$2 and blocked_user_id=$1))
	`, userID, targetID).Scan(&allowed); err != nil || !allowed {
		return FriendRequest{}, ErrSocialBlocked
	}
	var friendCount int
	if err := tx.QueryRow(ctx, `select count(*) from friendships where user_id_low=$1 or user_id_high=$1`, userID).Scan(&friendCount); err != nil {
		return FriendRequest{}, err
	}
	if friendCount >= 500 {
		return FriendRequest{}, ErrSocialLimit
	}
	var crossedID string
	err = tx.QueryRow(ctx, `
		select id::text from friend_requests
		where sender_user_id=$2 and recipient_user_id=$1 and status='pending' and expires_at>now()
		for update
	`, userID, targetID).Scan(&crossedID)
	if err == nil {
		if err := acceptFriendRequestTx(ctx, tx, crossedID, userID); err != nil {
			return FriendRequest{}, err
		}
		_ = appendUserEventTx(ctx, tx, targetID, "friendship.created", map[string]any{"userId": userID})
		_ = appendUserEventTx(ctx, tx, userID, "friendship.created", map[string]any{"userId": targetID})
		return FriendRequest{ID: crossedID, Direction: "incoming"}, tx.Commit(ctx)
	}
	id := entityid.New()
	expiresAt := time.Now().Add(30 * 24 * time.Hour)
	var createdAt time.Time
	if err := tx.QueryRow(ctx, `
		insert into friend_requests(id,sender_user_id,recipient_user_id,expires_at)
		values($1,$2,$3,$4)
		on conflict (least(sender_user_id,recipient_user_id),greatest(sender_user_id,recipient_user_id))
		  where status='pending'
		do update set expires_at=greatest(friend_requests.expires_at,excluded.expires_at)
		returning id::text,created_at,expires_at
	`, id, userID, targetID, expiresAt).Scan(&id, &createdAt, &expiresAt); err != nil {
		return FriendRequest{}, err
	}
	var notificationID int64
	_ = upsertUserNotification(ctx, tx, targetID, "friend_request_received", "friend_request:"+id,
		map[string]any{"requestId": id, "actorUserId": userID}, &notificationID)
	_ = appendUserEventTx(ctx, tx, targetID, "friend_request.created", map[string]any{"requestId": id})
	if err := tx.Commit(ctx); err != nil {
		return FriendRequest{}, err
	}
	return FriendRequest{ID: id, Direction: "outgoing", CreatedAt: createdAt, ExpiresAt: expiresAt}, nil
}

func acceptFriendRequestTx(ctx context.Context, tx pgx.Tx, requestID, recipientID string) error {
	var senderID string
	if err := tx.QueryRow(ctx, `
		update friend_requests set status='accepted',responded_at=now()
		where id=$1 and recipient_user_id=$2 and status='pending' and expires_at>now()
		returning sender_user_id::text
	`, requestID, recipientID).Scan(&senderID); err != nil {
		return ErrSocialNotFound
	}
	_, err := tx.Exec(ctx, `
		insert into friendships(user_id_low,user_id_high,created_from_request_id)
		values(least($1::uuid,$2::uuid),greatest($1::uuid,$2::uuid),$3)
		on conflict do nothing
	`, senderID, recipientID, requestID)
	return err
}

func (s *pgStore) RespondFriendRequest(userID, requestID, response string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var otherID string
	if response == "accept" {
		if err := tx.QueryRow(ctx, `select sender_user_id::text from friend_requests where id=$1 and recipient_user_id=$2`, requestID, userID).Scan(&otherID); err != nil {
			return ErrSocialNotFound
		}
		if err := acceptFriendRequestTx(ctx, tx, requestID, userID); err != nil {
			return err
		}
		var notificationID int64
		_ = upsertUserNotification(ctx, tx, otherID, "friendship_accepted", "friendship_accepted:"+requestID,
			map[string]any{"actorUserId": userID}, &notificationID)
		_ = appendUserEventTx(ctx, tx, otherID, "friendship.created", map[string]any{"userId": userID})
		_ = appendUserEventTx(ctx, tx, userID, "friendship.created", map[string]any{"userId": otherID})
	} else {
		status := "declined"
		ownerColumn := "recipient_user_id"
		if response == "cancel" {
			status, ownerColumn = "cancelled", "sender_user_id"
		}
		tag, err := tx.Exec(ctx, `update friend_requests set status=$3,responded_at=now() where id=$1 and `+ownerColumn+`=$2 and status='pending'`, requestID, userID, status)
		if err != nil || tag.RowsAffected() == 0 {
			return ErrSocialNotFound
		}
	}
	_, _ = tx.Exec(ctx, `update user_notifications set read_at=coalesce(read_at,now()) where user_id=$1 and dedupe_key='friend_request:'||$2`, userID, requestID)
	return tx.Commit(ctx)
}

func (s *pgStore) RemoveFriend(userID, targetID string) error {
	_, err := s.pool.Exec(context.Background(), `
		delete from friendships where user_id_low=least($1::uuid,$2::uuid) and user_id_high=greatest($1::uuid,$2::uuid)
	`, userID, targetID)
	return err
}

func (s *pgStore) SetUserBlock(userID, targetID string, blocked bool) error {
	if userID == targetID {
		return ErrSocialBlocked
	}
	ctx := context.Background()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if blocked {
		if _, err := tx.Exec(ctx, `insert into user_blocks values($1,$2,now()) on conflict do nothing`, userID, targetID); err != nil {
			return err
		}
		_, _ = tx.Exec(ctx, `delete from friendships where user_id_low=least($1::uuid,$2::uuid) and user_id_high=greatest($1::uuid,$2::uuid)`, userID, targetID)
		_, _ = tx.Exec(ctx, `update friend_requests set status='cancelled',responded_at=now() where status='pending' and least(sender_user_id,recipient_user_id)=least($1::uuid,$2::uuid) and greatest(sender_user_id,recipient_user_id)=greatest($1::uuid,$2::uuid)`, userID, targetID)
	} else {
		_, err = tx.Exec(ctx, `delete from user_blocks where blocker_user_id=$1 and blocked_user_id=$2`, userID, targetID)
		if err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (s *pgStore) CreateFriendCode(userID string, ttl time.Duration) (FriendCode, error) {
	if ttl <= 0 {
		ttl = 7 * 24 * time.Hour
	}
	ctx := context.Background()
	for attempt := 0; attempt < 8; attempt++ {
		code, err := randomFriendCode()
		if err != nil {
			return FriendCode{}, err
		}
		expiresAt := time.Now().Add(ttl)
		tx, err := s.pool.Begin(ctx)
		if err != nil {
			return FriendCode{}, err
		}
		_, _ = tx.Exec(ctx, `update friend_codes set revoked_at=now() where user_id=$1 and revoked_at is null`, userID)
		_, err = tx.Exec(ctx, `insert into friend_codes(code,user_id,expires_at) values($1,$2,$3)`, code, userID, expiresAt)
		if err == nil {
			err = tx.Commit(ctx)
			return FriendCode{Code: code, ExpiresAt: expiresAt}, err
		}
		_ = tx.Rollback(ctx)
	}
	return FriendCode{}, errors.New("could not allocate friend code")
}

func (s *pgStore) ResolveFriendCode(userID, code string) (CompactPlayer, error) {
	code = strings.ToUpper(strings.TrimSpace(code))
	var p CompactPlayer
	err := s.pool.QueryRow(context.Background(), `
		select u.id::text,u.display_name,coalesce(u.avatar_url,''),coalesce(r.mmr,1000),u.last_seen_at
		from friend_codes fc join users u on u.id=fc.user_id
		left join ranks r on r.user_id=u.id and r.mode='duel'
		where fc.code=$1 and fc.revoked_at is null and fc.expires_at>now()
		  and u.id<>$2 and u.account_type='registered' and u.social_requests_enabled
		  and not exists(select 1 from user_blocks b where
		    (b.blocker_user_id=$2 and b.blocked_user_id=u.id) or (b.blocker_user_id=u.id and b.blocked_user_id=$2))
	`, code, userID).Scan(&p.UserID, &p.DisplayName, &p.AvatarURL, &p.MMR, &p.LastSeenAt)
	if err != nil {
		return CompactPlayer{}, ErrSocialNotFound
	}
	p.Relationship, p.RequestID, _ = s.Relationship(userID, p.UserID)
	return p, nil
}

func randomFriendCode() (string, error) {
	buf := make([]byte, 6)
	random := make([]byte, 6)
	if _, err := rand.Read(random); err != nil {
		return "", err
	}
	for i := range buf {
		buf[i] = friendCodeAlphabet[int(random[i])%len(friendCodeAlphabet)]
	}
	return string(buf), nil
}

func (s *pgStore) CreatePartyInvitation(partyID, inviterID, recipientID string, ttl time.Duration) (PartyInvitation, error) {
	if ttl <= 0 {
		ttl = 20 * time.Minute
	}
	ctx := context.Background()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return PartyInvitation{}, err
	}
	defer tx.Rollback(ctx)
	var inviteCode, mode string
	var memberCount int
	err = tx.QueryRow(ctx, `
		select p.invite_code,p.mode,count(pm.user_id)::int
		from parties p join party_members self on self.party_id=p.id and self.user_id=$2 and self.left_at is null
		left join party_members pm on pm.party_id=p.id and pm.left_at is null
		where p.id=$1 and p.state='open' and p.expires_at>now()
		  and exists(select 1 from friendships f where f.user_id_low=least($2::uuid,$3::uuid) and f.user_id_high=greatest($2::uuid,$3::uuid))
		  and exists(select 1 from users u where u.id=$3 and u.social_party_invites_enabled)
		  and not exists(select 1 from user_blocks b where
		    (b.blocker_user_id=$2 and b.blocked_user_id=$3) or (b.blocker_user_id=$3 and b.blocked_user_id=$2))
		group by p.id
	`, partyID, inviterID, recipientID).Scan(&inviteCode, &mode, &memberCount)
	if err != nil {
		return PartyInvitation{}, ErrSocialBlocked
	}
	id := entityid.New()
	expiresAt := time.Now().Add(ttl)
	if err := tx.QueryRow(ctx, `
		insert into party_invitations(id,party_id,inviter_user_id,recipient_user_id,expires_at)
		values($1,$2,$3,$4,$5)
		on conflict(party_id,recipient_user_id) where status='pending'
		do update set inviter_user_id=excluded.inviter_user_id,expires_at=excluded.expires_at
		returning id::text,created_at,expires_at
	`, id, partyID, inviterID, recipientID, expiresAt).Scan(&id, new(time.Time), &expiresAt); err != nil {
		return PartyInvitation{}, err
	}
	var notificationID int64
	_ = upsertUserNotification(ctx, tx, recipientID, "party_invitation_received", "party_invitation:"+id,
		map[string]any{"invitationId": id, "actorUserId": inviterID}, &notificationID)
	_ = appendUserEventTx(ctx, tx, recipientID, "party_invitation.created", map[string]any{"invitationId": id})
	if err := tx.Commit(ctx); err != nil {
		return PartyInvitation{}, err
	}
	return PartyInvitation{ID: id, PartyID: partyID, InviteCode: inviteCode, Mode: mode, MemberCount: memberCount, ExpiresAt: expiresAt}, nil
}

func (s *pgStore) ListPartyInvitations(userID string, limit int) ([]PartyInvitation, error) {
	limit = boundedSocialLimit(limit, 10, 50)
	rows, err := s.pool.Query(context.Background(), `
		select pi.id::text,p.id::text,p.invite_code,p.mode,count(pm.user_id)::int,
		  u.id::text,u.display_name,coalesce(u.avatar_url,''),pi.created_at,pi.expires_at
		from party_invitations pi join parties p on p.id=pi.party_id
		join users u on u.id=pi.inviter_user_id
		left join party_members pm on pm.party_id=p.id and pm.left_at is null
		where pi.recipient_user_id=$1 and pi.status='pending' and pi.expires_at>now()
		  and p.state='open' and p.expires_at>now()
		group by pi.id,p.id,u.id order by pi.created_at desc limit $2
	`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []PartyInvitation{}
	for rows.Next() {
		var item PartyInvitation
		if err := rows.Scan(&item.ID, &item.PartyID, &item.InviteCode, &item.Mode, &item.MemberCount,
			&item.Inviter.UserID, &item.Inviter.DisplayName, &item.Inviter.AvatarURL, &item.CreatedAt, &item.ExpiresAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *pgStore) RespondPartyInvitation(userID, invitationID, response string) (PartyInvitation, error) {
	status := "declined"
	if response == "accept" {
		status = "accepted"
	}
	var item PartyInvitation
	err := s.pool.QueryRow(context.Background(), `
		update party_invitations pi set status=$3,responded_at=now()
		from parties p
		where pi.id=$1 and pi.recipient_user_id=$2 and pi.status='pending' and pi.expires_at>now()
		  and p.id=pi.party_id and p.state='open' and p.expires_at>now()
		returning pi.id::text,p.id::text,p.invite_code,p.mode,pi.expires_at
	`, invitationID, userID, status).Scan(&item.ID, &item.PartyID, &item.InviteCode, &item.Mode, &item.ExpiresAt)
	if err != nil {
		return PartyInvitation{}, ErrSocialNotFound
	}
	_, _ = s.pool.Exec(context.Background(), `update user_notifications set read_at=coalesce(read_at,now()) where user_id=$1 and dedupe_key='party_invitation:'||$2`, userID, invitationID)
	return item, nil
}

func (s *pgStore) TouchLastSeen(userID string, seenAt time.Time) error {
	_, err := s.pool.Exec(context.Background(), `update users set last_seen_at=greatest(coalesce(last_seen_at,$2),$2) where id=$1`, userID, seenAt)
	return err
}

func (s *pgStore) AppendUserEvent(userID, eventType string, payload any) (int64, error) {
	ctx := context.Background()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	sequence := int64(0)
	if err := appendUserEventTxScan(ctx, tx, userID, eventType, payload, &sequence); err != nil {
		return 0, err
	}
	return sequence, tx.Commit(ctx)
}

func appendUserEventTx(ctx context.Context, tx pgx.Tx, userID, eventType string, payload any) error {
	return appendUserEventTxScan(ctx, tx, userID, eventType, payload, new(int64))
}

func appendUserEventTxScan(ctx context.Context, tx pgx.Tx, userID, eventType string, payload any, sequence *int64) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if err := tx.QueryRow(ctx, `
		insert into user_event_sequences(user_id,sequence) values($1,1)
		on conflict(user_id) do update set sequence=user_event_sequences.sequence+1
		returning sequence
	`, userID).Scan(sequence); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `insert into user_events(user_id,sequence,type,payload_json) values($1,$2,$3,$4::jsonb)`,
		userID, *sequence, eventType, string(body))
	return err
}

func (s *pgStore) ListUserEvents(userID string, after int64, limit int) ([]UserEvent, error) {
	limit = boundedSocialLimit(limit, 100, 500)
	rows, err := s.pool.Query(context.Background(), `
		select sequence,type,payload_json::text,created_at from user_events
		where user_id=$1 and sequence>$2 order by sequence limit $3
	`, userID, after, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []UserEvent{}
	for rows.Next() {
		var event UserEvent
		var payload string
		if err := rows.Scan(&event.Sequence, &event.Type, &payload, &event.OccurredAt); err != nil {
			return nil, err
		}
		event.Payload = []byte(payload)
		out = append(out, event)
	}
	return out, rows.Err()
}

func boundedSocialLimit(limit, fallback, maximum int) int {
	if limit <= 0 {
		return fallback
	}
	if limit > maximum {
		return maximum
	}
	return limit
}
