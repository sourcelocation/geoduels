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
	"github.com/jackc/pgx/v5/pgtype"

	"geoduels/pkg/contracts"
	"geoduels/pkg/entityid"
	db "geoduels/pkg/persistence/sqlc/db"
)

var ErrPartyMapUnavailable = errors.New("selected map is not accessible or ready")

func (s *DB) CreateParty(ownerUserID string, mode contracts.MatchMode, mapScope string, ttl time.Duration) (contracts.PartySnapshot, error) {
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
		partyUUID, parseErr := profileUUID(partyID)
		if parseErr != nil {
			_ = tx.Rollback(ctx)
			cancel()
			return contracts.PartySnapshot{}, parseErr
		}
		ownerUUID, parseErr := profileUUID(ownerUserID)
		if parseErr != nil {
			_ = tx.Rollback(ctx)
			cancel()
			return contracts.PartySnapshot{}, parseErr
		}
		mapUUID, parseErr := profileUUID(mapID)
		if parseErr != nil {
			_ = tx.Rollback(ctx)
			cancel()
			return contracts.PartySnapshot{}, parseErr
		}
		q := db.New(tx)
		err = q.CreateParty(ctx, db.CreatePartyParams{ID: partyUUID, InviteCode: inviteCode, OwnerUserID: ownerUUID, Mode: db.GdMatchMode(mode), MapScope: mapScope, ExpiresAt: pgtype.Timestamptz{Time: expiresAt, Valid: true}, MapID: mapUUID})
		if err != nil {
			_ = tx.Rollback(ctx)
			cancel()
			if attempt == 4 {
				return contracts.PartySnapshot{}, fmt.Errorf("insert party: %w", err)
			}
			continue
		}
		if err := q.AddPartyOwner(ctx, db.AddPartyOwnerParams{PartyID: partyUUID, UserID: ownerUUID}); err != nil {
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

func (s *DB) GetPartyByID(partyID string) (contracts.PartySnapshot, bool, error) {
	partyID = strings.TrimSpace(partyID)
	if partyID == "" {
		return contracts.PartySnapshot{}, false, nil
	}
	return s.getParty(func(ctx context.Context) (partySnapshotRow, error) {
		id, err := profileUUID(partyID)
		if err != nil {
			return partySnapshotRow{}, err
		}
		row, err := s.db.GetPartySnapshotByID(ctx, id)
		return toPartySnapshotRow(row), err
	})
}

func (s *DB) SetPartyMode(partyID string, mode contracts.MatchMode) error {
	partyID = strings.TrimSpace(partyID)
	if partyID == "" || !contracts.IsPrivatePartyMode(mode) {
		return errors.New("invalid party mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	partyUUID, err := profileUUID(partyID)
	if err != nil {
		return err
	}
	q := db.New(tx)
	currentMode, err := q.LockOpenPartyMode(ctx, partyUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errors.New("party is not open")
		}
		return err
	}
	if string(currentMode) != string(contracts.ModeTeamDuel) && mode == contracts.ModeTeamDuel {
		if err := q.ShufflePartyTeams(ctx, partyUUID); err != nil {
			return err
		}
	}
	if err := q.SetPartyMode(ctx, db.SetPartyModeParams{ID: partyUUID, Mode: db.GdMatchMode(mode)}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *DB) SetPartyConfig(partyID string, cfg contracts.MatchConfig) (contracts.PartySnapshot, error) {
	partyID = strings.TrimSpace(partyID)
	cfg = contracts.NormalizeMatchConfig(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return contracts.PartySnapshot{}, err
	}
	defer tx.Rollback(ctx)
	partyUUID, err := profileUUID(partyID)
	if err != nil {
		return contracts.PartySnapshot{}, err
	}
	q := db.New(tx)
	ownerUUID, err := q.LockOpenPartyOwner(ctx, partyUUID)
	if err != nil {
		return contracts.PartySnapshot{}, err
	}
	canonicalMapID, _, err := resolveMapIdentity(ctx, tx, cfg.MapID)
	if err != nil {
		return contracts.PartySnapshot{}, ErrPartyMapUnavailable
	}
	cfg.MapID = canonicalMapID
	mapUUID, err := profileUUID(cfg.MapID)
	if err != nil {
		return contracts.PartySnapshot{}, ErrPartyMapUnavailable
	}
	accessible, err := q.PartyMapAccessible(ctx, db.PartyMapAccessibleParams{ID: mapUUID, OwnerUserID: ownerUUID})
	if err != nil {
		return contracts.PartySnapshot{}, err
	}
	if !accessible {
		return contracts.PartySnapshot{}, ErrPartyMapUnavailable
	}
	body, _ := json.Marshal(cfg)
	if err := q.SetPartyConfig(ctx, db.SetPartyConfigParams{ID: partyUUID, Column2: body, MapID: mapUUID}); err != nil {
		return contracts.PartySnapshot{}, err
	}
	if err := q.ResetPartyMembersReady(ctx, partyUUID); err != nil {
		return contracts.PartySnapshot{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return contracts.PartySnapshot{}, err
	}
	snap, _, err := s.GetPartyByID(partyID)
	return snap, err
}

func (s *DB) GetPartyByInviteCode(inviteCode string) (contracts.PartySnapshot, bool, error) {
	code := strings.ToUpper(strings.TrimSpace(inviteCode))
	return s.getParty(func(ctx context.Context) (partySnapshotRow, error) {
		row, err := s.db.GetPartySnapshotByInviteCode(ctx, code)
		return partySnapshotRow{
			ID: row.ID, OwnerUserID: row.OwnerUserID, InviteCode: row.InviteCode,
			State: string(row.State), Mode: string(row.Mode), MapScope: row.MapScope,
			ActiveMatchID: anyText(row.ActiveMatchID), LastMatchID: anyText(row.LastMatchID), StartedMatchID: anyText(row.StartedMatchID),
			CreatedAt: row.CreatedAt, ExpiresAt: row.ExpiresAt,
			ConfigJSON: row.LConfigJson, MapID: anyText(row.MapID),
			MapName: row.DisplayName, MapLocationCount: row.LocationCount,
		}, err
	})
}

func (s *DB) GetPartyByMatchID(matchID string) (contracts.PartySnapshot, bool, error) {
	matchID = strings.TrimSpace(matchID)
	if matchID == "" {
		return contracts.PartySnapshot{}, false, nil
	}
	return s.getParty(func(ctx context.Context) (partySnapshotRow, error) {
		id, err := profileUUID(matchID)
		if err != nil {
			return partySnapshotRow{}, err
		}
		row, err := s.db.GetPartySnapshotByMatchID(ctx, id)
		return partySnapshotRow{
			ID: row.ID, OwnerUserID: row.OwnerUserID, InviteCode: row.InviteCode,
			State: string(row.State), Mode: string(row.Mode), MapScope: row.MapScope,
			ActiveMatchID: anyText(row.ActiveMatchID), LastMatchID: anyText(row.LastMatchID), StartedMatchID: anyText(row.StartedMatchID),
			CreatedAt: row.CreatedAt, ExpiresAt: row.ExpiresAt,
			ConfigJSON: row.LConfigJson, MapID: anyText(row.MapID),
			MapName: row.DisplayName, MapLocationCount: row.LocationCount,
		}, err
	})
}

func (s *DB) JoinParty(partyID, userID string) (contracts.PartySnapshot, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	if err := s.ensurePartyJoinable(ctx, partyID); err != nil {
		return contracts.PartySnapshot{}, err
	}
	partyUUID, userIDUUID, err := profileUUID2(partyID, userID)
	if err != nil {
		return contracts.PartySnapshot{}, err
	}
	activeMembers, err := s.db.CountActivePartyMembers(ctx, db.CountActivePartyMembersParams{PartyID: partyUUID, UserID: userIDUUID})
	if err != nil {
		return contracts.PartySnapshot{}, err
	}
	if activeMembers >= contracts.MaxPartyMembers {
		return contracts.PartySnapshot{}, errors.New("party is full")
	}
	role := db.GdPartyRoleMember
	if snap, ok, err := s.GetPartyByID(partyID); err != nil {
		return contracts.PartySnapshot{}, err
	} else if !ok {
		return contracts.PartySnapshot{}, pgx.ErrNoRows
	} else if snap.OwnerUserID == userID {
		role = db.GdPartyRoleOwner
	}
	if err := s.db.JoinPartyMember(ctx, db.JoinPartyMemberParams{PartyID: partyUUID, UserID: userIDUUID, Role: role}); err != nil {
		return contracts.PartySnapshot{}, err
	}
	if err := s.db.TouchPartyUpdated(ctx, partyUUID); err != nil {
		return contracts.PartySnapshot{}, err
	}
	snap, _, err := s.GetPartyByID(partyID)
	return snap, err
}

func (s *DB) LeaveParty(partyID, userID string) (contracts.PartySnapshot, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return contracts.PartySnapshot{}, err
	}
	defer tx.Rollback(ctx)
	partyUUID, err := profileUUID(partyID)
	if err != nil {
		return contracts.PartySnapshot{}, err
	}
	q := db.New(tx)
	party, err := q.GetPartyOwnerAndState(ctx, partyUUID)
	if err != nil {
		return contracts.PartySnapshot{}, err
	}
	ownerUserID := party.OwnerUserID.String()
	state := string(party.State)
	if state != string(contracts.PartyOpen) && state != string(contracts.PartyInMatch) && state != string(contracts.PartyStarted) {
		return contracts.PartySnapshot{}, errors.New("party is not joinable")
	}
	if ownerUserID == userID && state != string(contracts.PartyOpen) {
		return contracts.PartySnapshot{}, errors.New("leader cannot leave the party while a game is in progress")
	}
	userUUID, err := profileUUID(userID)
	if err != nil {
		return contracts.PartySnapshot{}, err
	}
	tag, err := q.LeavePartyMember(ctx, db.LeavePartyMemberParams{PartyID: partyUUID, UserID: userUUID})
	if err != nil {
		return contracts.PartySnapshot{}, err
	}
	if tag == 0 {
		return contracts.PartySnapshot{}, pgx.ErrNoRows
	}
	if ownerUserID == userID {
		nextOwner, err := q.NextPartyOwnerID(ctx, partyUUID)
		if errors.Is(err, pgx.ErrNoRows) {
			if err := q.CloseParty(ctx, partyUUID); err != nil {
				return contracts.PartySnapshot{}, err
			}
		} else if err != nil {
			return contracts.PartySnapshot{}, err
		} else {
			if err := transferPartyOwnerTx(ctx, q, partyUUID, userUUID, nextOwner); err != nil {
				return contracts.PartySnapshot{}, err
			}
		}
	}
	if err := q.TouchPartyUpdated(ctx, partyUUID); err != nil {
		return contracts.PartySnapshot{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return contracts.PartySnapshot{}, err
	}
	next, _, err := s.GetPartyByID(partyID)
	return next, err
}

func (s *DB) SetPartyMemberTeam(partyID, userID, teamID string) (contracts.PartySnapshot, error) {
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
	partyUUID, userUUID, err := profileUUID2(partyID, userID)
	if err != nil {
		return contracts.PartySnapshot{}, err
	}
	tag, err := s.db.SetPartyMemberTeam(ctx, db.SetPartyMemberTeamParams{PartyID: partyUUID, UserID: userUUID, TeamID: db.NullGdTeamID{GdTeamID: db.GdTeamID(teamID), Valid: true}})
	if err != nil {
		return contracts.PartySnapshot{}, err
	}
	if tag == 0 {
		return contracts.PartySnapshot{}, pgx.ErrNoRows
	}
	if err := s.db.TouchOpenParty(ctx, partyUUID); err != nil {
		return contracts.PartySnapshot{}, err
	}
	snap, _, err := s.GetPartyByID(partyID)
	return snap, err
}

func (s *DB) ExpireOpenParties() error {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	_, err := s.db.ExpireOpenParties(ctx)
	return err
}

func (s *DB) ListOpenPartyIDs() ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	rows, err := s.db.ListOpenPartyIDs(ctx)
	if err != nil {
		return nil, err
	}
	ids := []string{}
	for _, row := range rows {
		ids = append(ids, row.String())
	}
	return ids, nil
}

func (s *DB) CloseInactiveOpenParties(partyIDs []string, inactiveFor time.Duration) (int64, error) {
	if len(partyIDs) == 0 || inactiveFor <= 0 {
		return 0, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	return s.db.CloseInactiveOpenParties(ctx, db.CloseInactiveOpenPartiesParams{
		Column1: chatUUIDs(partyIDs),
		Column2: inactiveFor.Seconds(),
	})
}

func (s *DB) KickPartyMember(partyID, ownerUserID, targetUserID string) (contracts.PartySnapshot, error) {
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
	partyUUID, targetUUID, err := profileUUID2(partyID, targetUserID)
	if err != nil {
		return contracts.PartySnapshot{}, err
	}
	tag, err := s.db.KickPartyMember(ctx, db.KickPartyMemberParams{PartyID: partyUUID, UserID: targetUUID})
	if err != nil {
		return contracts.PartySnapshot{}, err
	}
	if tag == 0 {
		return contracts.PartySnapshot{}, pgx.ErrNoRows
	}
	snap, _, err := s.GetPartyByID(partyID)
	return snap, err
}

func (s *DB) TransferPartyOwner(partyID, ownerUserID, targetUserID string) (contracts.PartySnapshot, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return contracts.PartySnapshot{}, err
	}
	defer tx.Rollback(ctx)
	partyUUID, err := profileUUID(partyID)
	if err != nil {
		return contracts.PartySnapshot{}, err
	}
	q := db.New(tx)
	if err := ensurePartyOwnerTx(ctx, q, partyUUID, ownerUserID); err != nil {
		return contracts.PartySnapshot{}, err
	}
	ownerUUID, err := profileUUID(ownerUserID)
	if err != nil {
		return contracts.PartySnapshot{}, err
	}
	targetUUID, err := profileUUID(targetUserID)
	if err != nil {
		return contracts.PartySnapshot{}, err
	}
	if err := transferPartyOwnerTx(ctx, q, partyUUID, ownerUUID, targetUUID); err != nil {
		return contracts.PartySnapshot{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return contracts.PartySnapshot{}, err
	}
	snap, _, err := s.GetPartyByID(partyID)
	return snap, err
}

func (s *DB) MarkPartyInMatch(partyID, matchID string) (contracts.PartySnapshot, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	partyUUID, matchUUID, err := profileUUID2(partyID, matchID)
	if err != nil {
		return contracts.PartySnapshot{}, err
	}
	tag, err := s.db.MarkPartyInMatch(ctx, db.MarkPartyInMatchParams{ID: partyUUID, ActiveMatchID: matchUUID})
	if err != nil {
		return contracts.PartySnapshot{}, err
	}
	if tag == 0 {
		return contracts.PartySnapshot{}, pgx.ErrNoRows
	}
	snap, _, err := s.GetPartyByID(partyID)
	return snap, err
}

func (s *DB) ReopenEndedParties() (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	tag, err := s.db.ReopenEndedParties(ctx, db.GdRuntimeState(contracts.MatchEnded))
	if err != nil {
		return 0, err
	}
	if err := s.db.EndSessionsForEndedRuntimeMatches(ctx, db.GdRuntimeState(contracts.MatchEnded)); err != nil {
		return 0, err
	}
	return tag, nil
}

func (s *DB) ensurePartyOpen(ctx context.Context, partyID string) error {
	partyUUID, err := profileUUID(partyID)
	if err != nil {
		return err
	}
	party, err := s.db.GetPartyStateAndExpiry(ctx, partyUUID)
	if err != nil {
		return err
	}
	if string(party.State) != string(contracts.PartyOpen) {
		return errors.New("party is not open")
	}
	if time.Now().After(party.ExpiresAt.Time) {
		_ = s.db.ExpireParty(ctx, partyUUID)
		return errors.New("party expired")
	}
	return nil
}

func (s *DB) ensurePartyJoinable(ctx context.Context, partyID string) error {
	partyUUID, err := profileUUID(partyID)
	if err != nil {
		return err
	}
	party, err := s.db.GetPartyStateAndExpiry(ctx, partyUUID)
	if err != nil {
		return err
	}
	state := string(party.State)
	if state != string(contracts.PartyOpen) && state != string(contracts.PartyInMatch) && state != string(contracts.PartyStarted) {
		return errors.New("party is not joinable")
	}
	if time.Now().After(party.ExpiresAt.Time) {
		return errors.New("party expired")
	}
	return nil
}

func (s *DB) ensurePartyOwner(ctx context.Context, partyID, ownerUserID string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	partyUUID, err := profileUUID(partyID)
	if err != nil {
		return err
	}
	return ensurePartyOwnerTx(ctx, db.New(tx), partyUUID, ownerUserID)
}

func ensurePartyOwnerTx(ctx context.Context, q *db.Queries, partyUUID pgtype.UUID, ownerUserID string) error {
	party, err := q.GetPartyStateAndOwner(ctx, partyUUID)
	if err != nil {
		return err
	}
	if string(party.State) != string(contracts.PartyOpen) {
		return errors.New("party is not open")
	}
	if party.OwnerUserID.String() != ownerUserID {
		return errors.New("forbidden")
	}
	return nil
}

func transferPartyOwnerTx(ctx context.Context, q *db.Queries, partyUUID, ownerUserID, targetUserID pgtype.UUID) error {
	if !ownerUserID.Valid || !targetUserID.Valid || ownerUserID.String() == targetUserID.String() {
		return errors.New("invalid party member")
	}
	targetActive, err := q.PartyMemberActive(ctx, db.PartyMemberActiveParams{PartyID: partyUUID, UserID: targetUserID})
	if err != nil {
		return err
	}
	if !targetActive {
		return pgx.ErrNoRows
	}
	if err := q.TransferPartyOwner(ctx, db.TransferPartyOwnerParams{ID: partyUUID, OwnerUserID: targetUserID}); err != nil {
		return err
	}
	return q.ReassignPartyRoles(ctx, db.ReassignPartyRolesParams{PartyID: partyUUID, UserID: targetUserID})
}

type partySnapshotRow struct {
	ID, OwnerUserID                   pgtype.UUID
	InviteCode, State, Mode, MapScope string
	ActiveMatchID, LastMatchID        string
	StartedMatchID, ConfigJSON, MapID string
	CreatedAt, ExpiresAt              pgtype.Timestamptz
	MapName                           string
	MapLocationCount                  int32
}

func (s *DB) getParty(fetch func(ctx context.Context) (partySnapshotRow, error)) (contracts.PartySnapshot, bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	var snap contracts.PartySnapshot
	row, err := fetch(ctx)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return contracts.PartySnapshot{}, false, nil
		}
		return contracts.PartySnapshot{}, false, err
	}
	snap.ID, snap.InviteCode, snap.OwnerUserID = row.ID.String(), row.InviteCode, row.OwnerUserID.String()
	snap.State, snap.Mode, snap.MapScope = contracts.PartyState(row.State), contracts.MatchMode(row.Mode), row.MapScope
	snap.ActiveMatchID, snap.LastMatchID, snap.StartedMatchID = row.ActiveMatchID, row.LastMatchID, row.StartedMatchID
	snap.CreatedAt, snap.ExpiresAt = row.CreatedAt.Time, row.ExpiresAt.Time
	snap.MapName, snap.MapLocationCount = row.MapName, int(row.MapLocationCount)
	_ = json.Unmarshal([]byte(row.ConfigJSON), &snap.Config)
	snap.Config = contracts.NormalizeMatchConfig(snap.Config)
	if row.MapID != "" {
		snap.Config.MapID = row.MapID
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

func toPartySnapshotRow(row db.GetPartySnapshotByIDRow) partySnapshotRow {
	return partySnapshotRow{
		ID: row.ID, OwnerUserID: row.OwnerUserID, InviteCode: row.InviteCode,
		State: string(row.State), Mode: string(row.Mode), MapScope: row.MapScope,
		ActiveMatchID: anyText(row.ActiveMatchID), LastMatchID: anyText(row.LastMatchID), StartedMatchID: anyText(row.StartedMatchID),
		CreatedAt: row.CreatedAt, ExpiresAt: row.ExpiresAt,
		ConfigJSON: row.LConfigJson, MapID: anyText(row.MapID),
		MapName: row.DisplayName, MapLocationCount: row.LocationCount,
	}
}

func (s *DB) listPartyMembers(ctx context.Context, partyID string) ([]contracts.PartyMember, error) {
	partyUUID, err := profileUUID(partyID)
	if err != nil {
		return nil, err
	}
	rows, err := s.db.ListPartyMembers(ctx, partyUUID)
	if err != nil {
		return nil, err
	}
	out := []contracts.PartyMember{}
	selected := map[string]string{}
	for _, row := range rows {
		var member contracts.PartyMember
		member.UserID = row.UserID.String()
		member.DisplayName = row.DisplayName
		member.AvatarURL = row.AvatarUrl
		member.IsGuest = row.Column4
		member.IsAdmin = row.IsAdmin
		member.TeamID = anyText(row.Coalesce)
		member.Role = string(row.Role)
		member.Ready = row.Ready
		member.JoinedAt = row.JoinedAt.Time
		selected[member.UserID] = badgeIDFromCode(row.SelectedBadgeCode)
		out = append(out, member)
	}
	badges, err := s.selectedPartyBadges(ctx, selected)
	if err != nil {
		return nil, err
	}
	for i := range out {
		out[i].SelectedBadge = badges[out[i].UserID]
	}
	return out, nil
}

func (s *DB) selectedPartyBadges(ctx context.Context, selected map[string]string) (map[string]*contracts.PlayerBadge, error) {
	if len(selected) == 0 {
		return map[string]*contracts.PlayerBadge{}, nil
	}
	userIDs := make([]pgtype.UUID, 0, len(selected))
	for userID := range selected {
		if strings.TrimSpace(userID) != "" {
			userIDs = append(userIDs, chatUUID(userID))
		}
	}
	if len(userIDs) == 0 {
		return map[string]*contracts.PlayerBadge{}, nil
	}
	rows, err := s.db.ListPartyMemberBadges(ctx, userIDs)
	if err != nil {
		return nil, err
	}
	out := map[string]*contracts.PlayerBadge{}
	fallback := map[string]*contracts.PlayerBadge{}
	for _, row := range rows {
		userID := row.UserID.String()
		badge := badgeFromParts(row.BadgeCode, row.Level, row.Extra, true)
		if badge.ID == "" {
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
	for userID, badge := range fallback {
		if out[userID] == nil && selected[userID] != "" {
			out[userID] = badge
		}
	}
	return out, nil
}

func anyText(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func profileUUID2(a, b string) (pgtype.UUID, pgtype.UUID, error) {
	first, err := profileUUID(a)
	if err != nil {
		return pgtype.UUID{}, pgtype.UUID{}, err
	}
	second, err := profileUUID(b)
	if err != nil {
		return pgtype.UUID{}, pgtype.UUID{}, err
	}
	return first, second, nil
}

func chatUUIDs(values []string) []pgtype.UUID {
	out := make([]pgtype.UUID, 0, len(values))
	for _, v := range values {
		out = append(out, chatUUID(v))
	}
	return out
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
