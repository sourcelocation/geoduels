package persistence

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"geoduels/pkg/contracts"
	"geoduels/pkg/sessionpolicy"
)

const defaultMatchLeaseTTL = 45 * time.Second

func (s *pgStore) UpsertMatchSession(ctx context.Context, params MatchSessionUpsert) error {
	found := params.Found
	found.Mode = sessionpolicy.NormalizeMode(found.Mode, found.MatchID)
	found.Config = contracts.NormalizeMatchConfig(found.Config)
	found.ReturnTarget = contracts.NormalizeMatchReturnTarget(found.ReturnTarget)
	if strings.TrimSpace(found.MatchID) == "" || len(found.Players) == 0 {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	presetID := matchPresetID(found)
	sourceKind := "queue"
	if found.SourcePartyID != "" {
		sourceKind = "party"
	} else if found.Mode == contracts.ModeSingleplayer {
		sourceKind = "solo"
	}
	cfgJSON, _ := json.Marshal(found.Config)
	returnTarget := found.ReturnTarget
	ranked := !found.Unranked && found.Mode == contracts.ModeDuel && found.SourcePartyID == ""
	if _, err := tx.Exec(ctx, `
		insert into match_sessions(
			match_id, preset_id, mode, state, ranked, source_kind, source_party_id, source_party_invite_code,
			node_id, node_epoch, public_route, config_json, map_id, return_target_kind, return_target_map_id,
			return_target_party_id, lease_expires_at, updated_at
		)
		values(
			$1,$2,$3,'live',$4,$5,nullif($6,'')::uuid,nullif($7,''),
			$8,$9,$10,$11::jsonb,nullif($12,'')::uuid,nullif($13,''),nullif($14,'')::uuid,
			nullif($15,'')::uuid,now()+$16::interval,now()
		)
		on conflict (match_id) do update set
			preset_id = excluded.preset_id,
			mode = excluded.mode,
			state = case when match_sessions.state='ended' then match_sessions.state else excluded.state end,
			ranked = excluded.ranked,
			source_kind = excluded.source_kind,
			source_party_id = excluded.source_party_id,
			source_party_invite_code = excluded.source_party_invite_code,
			node_id = excluded.node_id,
			node_epoch = excluded.node_epoch,
			public_route = excluded.public_route,
			config_json = excluded.config_json,
			map_id = excluded.map_id,
			return_target_kind = excluded.return_target_kind,
			return_target_map_id = excluded.return_target_map_id,
			return_target_party_id = excluded.return_target_party_id,
			lease_expires_at = case when match_sessions.state='ended'
				then match_sessions.lease_expires_at else excluded.lease_expires_at end,
			updated_at = now()
	`, found.MatchID, string(presetID), string(found.Mode), ranked, sourceKind,
		found.SourcePartyID, found.SourcePartyInviteCode, params.NodeID, params.NodeEpoch,
		params.PublicRoute, string(cfgJSON), resolvedMapID(found), string(returnTarget.Kind), returnTarget.MapID,
		returnTarget.PartyID, defaultMatchLeaseTTL.String()); err != nil {
		return err
	}
	for _, userID := range found.Players {
		userID = strings.TrimSpace(userID)
		if userID == "" {
			continue
		}
		profile := found.Profiles[userID]
		teamID := ""
		if found.Teams != nil {
			teamID = found.Teams[userID]
		}
		var joinedAt any
		if found.SourcePartyID != "" {
			var ts time.Time
			err := tx.QueryRow(ctx, `
				select joined_at
				from party_members
				where party_id = $1 and user_id = $2
			`, found.SourcePartyID, userID).Scan(&ts)
			if err == nil {
				joinedAt = ts
			} else if !errors.Is(err, pgx.ErrNoRows) {
				return err
			}
		}
		if _, err := tx.Exec(ctx, `
			insert into match_participants(match_id, user_id, team_id, display_name, avatar_url, joined_party_at)
			values($1, $2, nullif($3, ''), $4, $5, $6)
			on conflict (match_id, user_id) do update set
				team_id = excluded.team_id,
				display_name = excluded.display_name,
				avatar_url = excluded.avatar_url,
				joined_party_at = coalesce(match_participants.joined_party_at, excluded.joined_party_at)
		`, found.MatchID, userID, teamID, profile.DisplayName, profile.AvatarURL, joinedAt); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (s *pgStore) MatchSessionSourceParty(ctx context.Context, matchID string) (string, string, bool, error) {
	matchID = strings.TrimSpace(matchID)
	if matchID == "" {
		return "", "", false, nil
	}
	ctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	var partyID, inviteCode string
	err := s.pool.QueryRow(ctx, `
		select coalesce(source_party_id::text, ''), coalesce(source_party_invite_code, '')
		from match_sessions
		where match_id = $1
	`, matchID).Scan(&partyID, &inviteCode)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", "", false, nil
		}
		return "", "", false, err
	}
	return partyID, inviteCode, partyID != "", nil
}

// MatchSessionReturnTarget returns the persisted navigation intent. It is an
// optional repository capability so older in-memory repositories can continue
// serving match history while they are migrated.
func (s *pgStore) MatchSessionReturnTarget(ctx context.Context, matchID string) (*contracts.MatchReturnTarget, bool, error) {
	matchID = strings.TrimSpace(matchID)
	if matchID == "" {
		return nil, false, nil
	}
	ctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	var kind, mapID, partyID string
	err := s.pool.QueryRow(ctx, `
		select coalesce(return_target_kind, 'home'), coalesce(return_target_map_id::text, ''),
		       coalesce(return_target_party_id::text, '')
		from match_sessions where match_id = $1
	`, matchID).Scan(&kind, &mapID, &partyID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, false, nil
		}
		return nil, false, err
	}
	target := contracts.NormalizeMatchReturnTarget(&contracts.MatchReturnTarget{
		Kind: contracts.MatchReturnTargetKind(kind), MapID: mapID, PartyID: partyID,
	})
	return target, true, nil
}

func (s *pgStore) RenewMatchSessionLeases(nodeID string, ownerEpoch int64, matchIDs []string, ttl time.Duration) error {
	if strings.TrimSpace(nodeID) == "" || ownerEpoch == 0 || len(matchIDs) == 0 {
		return nil
	}
	if ttl <= 0 {
		ttl = defaultMatchLeaseTTL
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	_, err := s.pool.Exec(ctx, `
		update match_sessions
		set lease_expires_at=now()+$4::interval, updated_at=now()
		where node_id=$1 and node_epoch=$2 and state='live'
		  and match_id::text=any($3)
	`, nodeID, ownerEpoch, matchIDs, ttl.String())
	return err
}

func matchPresetID(found contracts.MatchFound) contracts.MatchPresetID {
	switch found.Mode {
	case contracts.ModeSingleplayer:
		return contracts.MatchPresetSolo
	case contracts.ModeTeamDuel:
		return contracts.MatchPresetTeamDuel
	case contracts.ModeFreeForAll:
		return contracts.MatchPresetFreeForAll
	case contracts.ModeDuel:
		if found.SourcePartyID != "" || found.Unranked {
			return contracts.MatchPresetPrivateDuel
		}
		return contracts.MatchPresetRankedDuel
	default:
		return contracts.MatchPresetPrivateDuel
	}
}

func resolvedMapID(found contracts.MatchFound) string {
	if strings.TrimSpace(found.ResolvedMap.MapID) != "" {
		return found.ResolvedMap.MapID
	}
	return found.Config.MapID
}
