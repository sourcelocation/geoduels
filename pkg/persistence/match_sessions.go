package persistence

import (
	"context"
	"encoding/json"
	"errors"
	"geoduels/pkg/contracts"
	"geoduels/pkg/sessionpolicy"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"strings"
	"time"

	db "geoduels/pkg/persistence/sqlc/db"
)

func nullableSessionText(v string) any {
	if v == "" {
		return nil
	}
	return v
}

func anySessionText(v any, fallback string) string {
	if value := anyText(v); value != "" {
		return value
	}
	return fallback
}

func nullableSessionUUID(value string) (pgtype.UUID, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return pgtype.UUID{}, nil
	}
	return profileUUID(value)
}

const defaultMatchLeaseTTL = 45 * time.Second

func (s *DB) UpsertMatchSession(ctx context.Context, p MatchSessionUpsert) error {
	f := p.Found
	f.Mode = sessionpolicy.NormalizeMode(f.Mode, f.MatchID)
	f.Config = contracts.NormalizeMatchConfig(f.Config)
	f.ReturnTarget = contracts.NormalizeMatchReturnTarget(f.ReturnTarget)
	if strings.TrimSpace(f.MatchID) == "" || len(f.Players) == 0 {
		return nil
	}
	ctx, c := context.WithTimeout(ctx, 4*time.Second)
	defer c()
	return s.withTx(ctx, func(tx pgx.Tx) error {
		matchUUID, err := profileUUID(f.MatchID)
		if err != nil {
			return err
		}
		q := s.db.WithTx(tx)
		src := "queue"
		if f.SourcePartyID != "" {
			src = "party"
		} else if f.Mode == contracts.ModeSingleplayer {
			src = "solo"
		}
		b, _ := json.Marshal(f.Config)
		r := f.ReturnTarget
		sourcePartyID, err := nullableSessionUUID(f.SourcePartyID)
		if err != nil {
			return err
		}
		mapID, err := nullableSessionUUID(resolvedMapID(f))
		if err != nil {
			return err
		}
		returnMap, err := nullableSessionUUID(r.MapID)
		if err != nil {
			return err
		}
		returnParty, err := nullableSessionUUID(r.PartyID)
		if err != nil {
			return err
		}
		e := q.UpsertMatchSession(ctx, db.UpsertMatchSessionParams{
			MatchID:               matchUUID,
			PresetID:              db.GdMatchPreset(matchPresetID(f)),
			Mode:                  db.GdMatchMode(f.Mode),
			Ranked:                !f.Unranked && f.Mode == contracts.ModeDuel && f.SourcePartyID == "",
			SourceKind:            db.GdMatchSource(src),
			SourcePartyID:         sourcePartyID,
			SourcePartyInviteCode: pgtype.Text{String: f.SourcePartyInviteCode, Valid: f.SourcePartyInviteCode != ""},
			NodeID:                pgtype.Text{String: p.NodeID, Valid: true},
			NodeEpoch:             pgtype.Int8{Int64: p.NodeEpoch, Valid: true},
			PublicRoute:           pgtype.Text{String: p.PublicRoute, Valid: true},
			ConfigJson:            b,
			MapID:                 mapID,
			ReturnTargetKind:      pgtype.Text{String: string(r.Kind), Valid: r.Kind != ""},
			ReturnTargetMapID:     returnMap,
			ReturnTargetPartyID:   returnParty,
			LeaseTtl:              pgtype.Interval{Microseconds: defaultMatchLeaseTTL.Microseconds(), Valid: true},
		})
		if e != nil {
			return e
		}
		for _, u := range f.Players {
			u = strings.TrimSpace(u)
			if u == "" {
				continue
			}
			pr := f.Profiles[u]
			team := ""
			if f.Teams != nil {
				team = f.Teams[u]
			}
			joined := pgtype.Timestamptz{}
			if f.SourcePartyID != "" {
				partyUUID, err := profileUUID(f.SourcePartyID)
				if err != nil {
					return err
				}
				userUUID, err := profileUUID(u)
				if err != nil {
					return err
				}
				t, e := q.ParticipantJoinedAt(ctx, db.ParticipantJoinedAtParams{PartyID: partyUUID, UserID: userUUID})
				if e == nil {
					joined = t
				} else if !errors.Is(e, pgx.ErrNoRows) {
					return e
				}
			}
			userUUID, err := profileUUID(u)
			if err != nil {
				return err
			}
			team = strings.TrimSpace(team)
			if e = q.UpsertMatchParticipant(ctx, db.UpsertMatchParticipantParams{MatchID: matchUUID, UserID: userUUID, TeamID: pgtype.Text{String: team, Valid: team != ""}, DisplayName: pr.DisplayName, AvatarUrl: pr.AvatarURL, JoinedPartyAt: joined}); e != nil {
				return e
			}
		}
		return nil
	})
}
func (s *DB) MatchSessionSourceParty(ctx context.Context, id string) (string, string, bool, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return "", "", false, nil
	}
	ctx, c := context.WithTimeout(ctx, 4*time.Second)
	defer c()
	source, e := s.db.GetMatchSessionSource(ctx, chatUUID(id))
	if errors.Is(e, pgx.ErrNoRows) {
		return "", "", false, nil
	}
	if e != nil {
		return "", "", false, e
	}
	sourcePartyID := uuidVal(source.SourcePartyID)
	return sourcePartyID, source.SourcePartyInviteCode, sourcePartyID != "", nil
}
func (s *DB) MatchSessionReturnTarget(ctx context.Context, id string) (*contracts.MatchReturnTarget, bool, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, false, nil
	}
	ctx, c := context.WithTimeout(ctx, 4*time.Second)
	defer c()
	target, e := s.db.GetMatchSessionReturnTarget(ctx, chatUUID(id))
	if errors.Is(e, pgx.ErrNoRows) {
		return nil, false, nil
	}
	if e != nil {
		return nil, false, e
	}
	k, m, p := anySessionText(target.ReturnTargetKind, "home"), anySessionText(target.ReturnTargetMapID, ""), anySessionText(target.ReturnTargetPartyID, "")
	t := contracts.NormalizeMatchReturnTarget(&contracts.MatchReturnTarget{Kind: contracts.MatchReturnTargetKind(k), MapID: m, PartyID: p})
	return t, true, nil
}
func (s *DB) RenewMatchSessionLeases(node string, epoch int64, ids []string, ttl time.Duration) error {
	if strings.TrimSpace(node) == "" || epoch == 0 || len(ids) == 0 {
		return nil
	}
	if ttl <= 0 {
		ttl = defaultMatchLeaseTTL
	}
	ctx, c := context.WithTimeout(context.Background(), 4*time.Second)
	defer c()
	return s.db.RenewMatchSessionLeases(ctx, db.RenewMatchSessionLeasesParams{
		NodeID:    pgtype.Text{String: node, Valid: true},
		NodeEpoch: pgtype.Int8{Int64: epoch, Valid: true},
		MatchIds:  chatUUIDs(ids),
		Ttl:       pgtype.Interval{Microseconds: ttl.Microseconds(), Valid: true},
	})
}
func matchPresetID(f contracts.MatchFound) contracts.MatchPresetID {
	switch f.Mode {
	case contracts.ModeSingleplayer:
		return contracts.MatchPresetSolo
	case contracts.ModeTeamDuel:
		return contracts.MatchPresetTeamDuel
	case contracts.ModeFreeForAll:
		return contracts.MatchPresetFreeForAll
	case contracts.ModeDuel:
		if f.SourcePartyID != "" || f.Unranked {
			return contracts.MatchPresetPrivateDuel
		}
		return contracts.MatchPresetRankedDuel
	default:
		return contracts.MatchPresetPrivateDuel
	}
}
func resolvedMapID(f contracts.MatchFound) string {
	if strings.TrimSpace(f.ResolvedMap.MapID) != "" {
		return f.ResolvedMap.MapID
	}
	return f.Config.MapID
}
