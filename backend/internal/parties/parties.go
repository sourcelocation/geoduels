package parties

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"geoduels/internal/badges"
	mapsdomain "geoduels/internal/maps"
	"geoduels/internal/storekit"
	"geoduels/pkg/contracts"
	"geoduels/pkg/entityid"
	db "geoduels/pkg/persistence/sqlc/db"
)

var ErrPartyMapUnavailable = errors.New("selected map is not accessible or ready")

// --- snapshot reads (PartyReads) ---

func (s *PGStore) GetCurrentParty(ctx context.Context, userID string) (*contracts.CurrentParty, error) {
	id, err := profileUUID(userID)
	if err != nil {
		return nil, err
	}
	row, err := s.q().GetCurrentParty(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &contracts.CurrentParty{ID: row.ID.String(), InviteCode: row.InviteCode}, nil
}

func (s *PGStore) GetPartyByID(ctx context.Context, partyID string) (contracts.PartySnapshot, bool, error) {
	partyID = strings.TrimSpace(partyID)
	if partyID == "" {
		return contracts.PartySnapshot{}, false, nil
	}
	return s.getParty(ctx, func(ctx context.Context) (partySnapshotRow, error) {
		id, err := profileUUID(partyID)
		if err != nil {
			return partySnapshotRow{}, err
		}
		row, err := s.q().GetPartySnapshotByID(ctx, id)
		return toPartySnapshotRow(row), err
	})
}

func (s *PGStore) GetPartyByInviteCode(ctx context.Context, inviteCode string) (contracts.PartySnapshot, bool, error) {
	code := strings.ToUpper(strings.TrimSpace(inviteCode))
	return s.getParty(ctx, func(ctx context.Context) (partySnapshotRow, error) {
		row, err := s.q().GetPartySnapshotByInviteCode(ctx, code)
		return partySnapshotRow{
			ID: row.ID, OwnerUserID: row.OwnerUserID, InviteCode: row.InviteCode,
			State: string(row.State), Mode: string(row.Mode), MapScope: row.MapScope,
			ActiveMatchID: anyText(row.ActiveMatchID), LastMatchID: anyText(row.LastMatchID), StartedMatchID: anyText(row.StartedMatchID),
			CreatedAt: row.CreatedAt, ExpiresAt: row.ExpiresAt,
			ConfigJSON: string(row.ConfigJson), MapID: anyText(row.MapID),
			MapName: row.DisplayName, MapLocationCount: row.LocationCount,
		}, err
	})
}

func (s *PGStore) GetPartyByMatchID(ctx context.Context, matchID string) (contracts.PartySnapshot, bool, error) {
	matchID = strings.TrimSpace(matchID)
	if matchID == "" {
		return contracts.PartySnapshot{}, false, nil
	}
	return s.getParty(ctx, func(ctx context.Context) (partySnapshotRow, error) {
		id, err := profileUUID(matchID)
		if err != nil {
			return partySnapshotRow{}, err
		}
		row, err := s.q().GetPartySnapshotByMatchID(ctx, id)
		return partySnapshotRow{
			ID: row.ID, OwnerUserID: row.OwnerUserID, InviteCode: row.InviteCode,
			State: string(row.State), Mode: string(row.Mode), MapScope: row.MapScope,
			ActiveMatchID: anyText(row.ActiveMatchID), LastMatchID: anyText(row.LastMatchID), StartedMatchID: anyText(row.StartedMatchID),
			CreatedAt: row.CreatedAt, ExpiresAt: row.ExpiresAt,
			ConfigJSON: string(row.ConfigJson), MapID: anyText(row.MapID),
			MapName: row.DisplayName, MapLocationCount: row.LocationCount,
		}, err
	})
}

// --- state/ownership/membership reads (PartyState) ---

func (s *PGStore) GetPartyStateAndOwner(ctx context.Context, partyID string) (PartyStateOwner, error) {
	id, err := profileUUID(partyID)
	if err != nil {
		return PartyStateOwner{}, err
	}
	row, err := s.q().GetPartyStateAndOwner(ctx, id)
	if err != nil {
		return PartyStateOwner{}, err
	}
	return PartyStateOwner{State: contracts.PartyState(row.State), OwnerUserID: row.OwnerUserID.String()}, nil
}

func (s *PGStore) GetPartyStateAndExpiry(ctx context.Context, partyID string) (PartyStateExpiry, error) {
	id, err := profileUUID(partyID)
	if err != nil {
		return PartyStateExpiry{}, err
	}
	row, err := s.q().GetPartyStateAndExpiry(ctx, id)
	if err != nil {
		return PartyStateExpiry{}, err
	}
	return PartyStateExpiry{State: contracts.PartyState(row.State), ExpiresAt: row.ExpiresAt.Time}, nil
}

func (s *PGStore) PartyMemberActive(ctx context.Context, partyID, userID string) (bool, error) {
	partyUUID, userUUID, err := profileUUID2(partyID, userID)
	if err != nil {
		return false, err
	}
	return s.q().PartyMemberActive(ctx, db.PartyMemberActiveParams{PartyID: partyUUID, UserID: userUUID})
}

func (s *PGStore) CountActivePartyMembers(ctx context.Context, partyID, userID string) (int, error) {
	partyUUID, userUUID, err := profileUUID2(partyID, userID)
	if err != nil {
		return 0, err
	}
	count, err := s.q().CountActivePartyMembers(ctx, db.CountActivePartyMembersParams{PartyID: partyUUID, UserID: userUUID})
	return int(count), err
}

// --- mutations (PartyWrites) ---

func (s *PGStore) InsertParty(ctx context.Context, input PartyInsert) error {
	if _, err := s.requireTx(); err != nil {
		return err
	}
	partyUUID, err := profileUUID(input.ID)
	if err != nil {
		return err
	}
	ownerUUID, err := profileUUID(input.OwnerUserID)
	if err != nil {
		return err
	}
	mapUUID, err := profileUUID(input.MapID)
	if err != nil {
		return err
	}
	q := s.q()
	if err := q.CreateParty(ctx, db.CreatePartyParams{
		ID: partyUUID, InviteCode: input.InviteCode, OwnerUserID: ownerUUID,
		Mode: db.GdMatchMode(input.Mode), MapScope: input.MapScope,
		ExpiresAt: pgtype.Timestamptz{Time: input.ExpiresAt, Valid: true}, MapID: mapUUID,
	}); err != nil {
		return err
	}
	return q.AddPartyOwner(ctx, db.AddPartyOwnerParams{PartyID: partyUUID, UserID: ownerUUID})
}

func (s *PGStore) LockOpenPartyMode(ctx context.Context, partyID string) (contracts.MatchMode, error) {
	id, err := profileUUID(partyID)
	if err != nil {
		return "", partyStoreError(err)
	}
	mode, err := s.q().LockOpenPartyMode(ctx, id)
	return contracts.MatchMode(mode), partyStoreError(err)
}

func (s *PGStore) ShufflePartyTeams(ctx context.Context, partyID string) error {
	id, err := profileUUID(partyID)
	if err != nil {
		return err
	}
	return s.q().ShufflePartyTeams(ctx, id)
}

func (s *PGStore) SetPartyMode(ctx context.Context, partyID string, mode contracts.MatchMode) error {
	id, err := profileUUID(partyID)
	if err != nil {
		return err
	}
	return s.q().SetPartyMode(ctx, db.SetPartyModeParams{ID: id, Mode: db.GdMatchMode(mode)})
}

func (s *PGStore) LockOpenPartyOwner(ctx context.Context, partyID string) (string, error) {
	id, err := profileUUID(partyID)
	if err != nil {
		return "", partyStoreError(err)
	}
	owner, err := s.q().LockOpenPartyOwner(ctx, id)
	if err != nil {
		return "", partyStoreError(err)
	}
	return owner.String(), nil
}

// ResolveMapIdentity canonicalizes a public map ID/alias to its database ID
// using the caller's transaction so it is consistent with the write that
// follows.
func (s *PGStore) ResolveMapIdentity(ctx context.Context, requestedMapID string) (string, error) {
	tx, err := s.requireTx()
	if err != nil {
		return "", err
	}
	id, _, err := mapsdomain.ResolveMapIdentity(ctx, tx, requestedMapID)
	return id, err
}

func (s *PGStore) PartyMapAccessible(ctx context.Context, mapID, ownerUserID string) (bool, error) {
	mapUUID, err := profileUUID(mapID)
	if err != nil {
		return false, ErrPartyMapUnavailable
	}
	ownerUUID, err := profileUUID(ownerUserID)
	if err != nil {
		return false, err
	}
	return s.q().PartyMapAccessible(ctx, db.PartyMapAccessibleParams{ID: mapUUID, OwnerUserID: ownerUUID})
}

func (s *PGStore) SetPartyConfig(ctx context.Context, partyID string, configJSON []byte, mapID string) error {
	partyUUID, mapUUID, err := profileUUID2(partyID, mapID)
	if err != nil {
		return ErrPartyMapUnavailable
	}
	return s.q().SetPartyConfig(ctx, db.SetPartyConfigParams{PartyID: partyUUID, ConfigJson: configJSON, MapID: mapUUID})
}

func (s *PGStore) ResetPartyMembersReady(ctx context.Context, partyID string) error {
	id, err := profileUUID(partyID)
	if err != nil {
		return err
	}
	return s.q().ResetPartyMembersReady(ctx, id)
}

func (s *PGStore) JoinPartyMember(ctx context.Context, partyID, userID string, role MemberRole) error {
	partyUUID, userUUID, err := profileUUID2(partyID, userID)
	if err != nil {
		return err
	}
	dbRole := db.GdPartyRoleMember
	if role == MemberRoleOwner {
		dbRole = db.GdPartyRoleOwner
	}
	return s.q().JoinPartyMember(ctx, db.JoinPartyMemberParams{PartyID: partyUUID, UserID: userUUID, Role: dbRole})
}

func (s *PGStore) LeavePartyMember(ctx context.Context, partyID, userID string) (int64, error) {
	partyUUID, userUUID, err := profileUUID2(partyID, userID)
	if err != nil {
		return 0, err
	}
	return s.q().LeavePartyMember(ctx, db.LeavePartyMemberParams{PartyID: partyUUID, UserID: userUUID})
}

func (s *PGStore) NextPartyOwnerID(ctx context.Context, partyID string) (string, error) {
	id, err := profileUUID(partyID)
	if err != nil {
		return "", partyStoreError(err)
	}
	next, err := s.q().NextPartyOwnerID(ctx, id)
	if err != nil {
		return "", partyStoreError(err)
	}
	return next.String(), nil
}

func (s *PGStore) CloseParty(ctx context.Context, partyID string) error {
	id, err := profileUUID(partyID)
	if err != nil {
		return err
	}
	return s.q().CloseParty(ctx, id)
}

func (s *PGStore) TransferPartyOwner(ctx context.Context, partyID, targetUserID string) error {
	partyUUID, targetUUID, err := profileUUID2(partyID, targetUserID)
	if err != nil {
		return err
	}
	return s.q().TransferPartyOwner(ctx, db.TransferPartyOwnerParams{ID: partyUUID, OwnerUserID: targetUUID})
}

func (s *PGStore) ReassignPartyRoles(ctx context.Context, partyID, targetUserID string) error {
	partyUUID, targetUUID, err := profileUUID2(partyID, targetUserID)
	if err != nil {
		return err
	}
	return s.q().ReassignPartyRoles(ctx, db.ReassignPartyRolesParams{PartyID: partyUUID, UserID: targetUUID})
}

func (s *PGStore) KickPartyMember(ctx context.Context, partyID, targetUserID string) (int64, error) {
	partyUUID, targetUUID, err := profileUUID2(partyID, targetUserID)
	if err != nil {
		return 0, err
	}
	return s.q().KickPartyMember(ctx, db.KickPartyMemberParams{PartyID: partyUUID, UserID: targetUUID})
}

func (s *PGStore) SetPartyMemberTeam(ctx context.Context, partyID, userID, teamID string) (int64, error) {
	partyUUID, userUUID, err := profileUUID2(partyID, userID)
	if err != nil {
		return 0, err
	}
	return s.q().SetPartyMemberTeam(ctx, db.SetPartyMemberTeamParams{PartyID: partyUUID, UserID: userUUID, TeamID: db.GdTeamID(teamID)})
}

func (s *PGStore) TouchPartyUpdated(ctx context.Context, partyID string) error {
	id, err := profileUUID(partyID)
	if err != nil {
		return err
	}
	return s.q().TouchPartyUpdated(ctx, id)
}

func (s *PGStore) TouchOpenParty(ctx context.Context, partyID string) error {
	id, err := profileUUID(partyID)
	if err != nil {
		return err
	}
	return s.q().TouchOpenParty(ctx, id)
}

func (s *PGStore) MarkPartyInMatch(ctx context.Context, partyID, matchID string) error {
	partyUUID, matchUUID, err := profileUUID2(partyID, matchID)
	if err != nil {
		return err
	}
	affected, err := s.q().MarkPartyInMatch(ctx, db.MarkPartyInMatchParams{ID: partyUUID, ActiveMatchID: matchUUID})
	if err != nil {
		return err
	}
	if affected == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (s *PGStore) ExpireParty(ctx context.Context, partyID string) error {
	id, err := profileUUID(partyID)
	if err != nil {
		return err
	}
	return s.q().ExpireParty(ctx, id)
}

// --- maintenance (PartyMaintenance) ---

func (s *PGStore) ExpireOpenParties(ctx context.Context) error {
	_, err := s.q().ExpireOpenParties(ctx)
	return err
}

func (s *PGStore) ListOpenPartyIDs(ctx context.Context) ([]string, error) {
	rows, err := s.q().ListOpenPartyIDs(ctx)
	if err != nil {
		return nil, err
	}
	ids := []string{}
	for _, row := range rows {
		ids = append(ids, row.String())
	}
	return ids, nil
}

func (s *PGStore) CloseInactiveOpenParties(ctx context.Context, partyIDs []string, inactiveFor time.Duration) (int64, error) {
	if len(partyIDs) == 0 || inactiveFor <= 0 {
		return 0, nil
	}
	return s.q().CloseInactiveOpenParties(ctx, db.CloseInactiveOpenPartiesParams{
		PartyIds:        chatUUIDs(partyIDs),
		InactiveSeconds: inactiveFor.Seconds(),
	})
}

func (s *PGStore) ReopenEndedParties(ctx context.Context) (int64, error) {
	return s.q().ReopenEndedParties(ctx)
}

// --- snapshot shaping ---

type partySnapshotRow struct {
	ID, OwnerUserID                   pgtype.UUID
	InviteCode, State, Mode, MapScope string
	ActiveMatchID, LastMatchID        string
	StartedMatchID, ConfigJSON, MapID string
	CreatedAt, ExpiresAt              pgtype.Timestamptz
	MapName                           string
	MapLocationCount                  int32
}

func (s *PGStore) getParty(ctx context.Context, fetch func(ctx context.Context) (partySnapshotRow, error)) (contracts.PartySnapshot, bool, error) {
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
		ConfigJSON: string(row.ConfigJson), MapID: anyText(row.MapID),
		MapName: row.DisplayName, MapLocationCount: row.LocationCount,
	}
}

func (s *PGStore) listPartyMembers(ctx context.Context, partyID string) ([]contracts.PartyMember, error) {
	partyUUID, err := profileUUID(partyID)
	if err != nil {
		return nil, err
	}
	rows, err := s.q().ListPartyMembers(ctx, partyUUID)
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
		member.IsGuest = row.IsGuest
		member.IsAdmin = row.IsAdmin
		member.TeamID = string(row.TeamID)
		member.Role = string(row.Role)
		member.Ready = row.Ready
		member.JoinedAt = row.JoinedAt.Time
		selected[member.UserID] = badges.IDFromCode(row.SelectedBadgeCode)
		out = append(out, member)
	}
	selectedBadges, err := s.selectedPartyBadges(ctx, selected)
	if err != nil {
		return nil, err
	}
	for i := range out {
		out[i].SelectedBadge = selectedBadges[out[i].UserID]
	}
	return out, nil
}

func (s *PGStore) selectedPartyBadges(ctx context.Context, selected map[string]string) (map[string]*contracts.PlayerBadge, error) {
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
	rows, err := s.q().ListPartyMemberBadges(ctx, userIDs)
	if err != nil {
		return nil, err
	}
	out := map[string]*contracts.PlayerBadge{}
	fallback := map[string]*contracts.PlayerBadge{}
	for _, row := range rows {
		userID := row.UserID.String()
		badge := badges.FromParts(row.BadgeCode, row.Level, row.Extra, true)
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

// --- helpers ---

func anyText(v any) string {
	switch value := v.(type) {
	case string:
		return value
	case pgtype.UUID:
		return storekit.UUIDVal(value)
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

// newPartyCode generates a random invite code. The clock is injected so the
// package never reads time directly.
func newPartyCode(now time.Time) string {
	const alphabet = "23456789ABCDEFGHJKMNPQRSTUVWXYZ"
	buf := make([]byte, 6)
	random := make([]byte, 6)
	if _, err := rand.Read(random); err != nil {
		fallback := strings.ToUpper(now.Format("150405"))
		return fallback[:6]
	}
	for i, b := range random {
		buf[i] = alphabet[int(b)%len(alphabet)]
	}
	return string(buf)
}

func partyStoreError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}
