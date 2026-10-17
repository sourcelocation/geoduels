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
	"geoduels/pkg/entityid"
	"geoduels/pkg/gameplay"
	"geoduels/pkg/matchkind"
	db "geoduels/pkg/persistence/sqlc/db"
)

var ErrPartyMapUnavailable = errors.New("selected map is not accessible or ready")

// --- snapshot reads (PartyReads) ---

func (s *PGStore) GetCurrentParty(ctx context.Context, userID string) (*CurrentParty, error) {
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
	return &CurrentParty{ID: row.ID.String(), InviteCode: row.InviteCode}, nil
}

func (s *PGStore) GetPartyByID(ctx context.Context, partyID string) (Party, bool, error) {
	partyID = strings.TrimSpace(partyID)
	if partyID == "" {
		return Party{}, false, nil
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

func (s *PGStore) GetPartyByInviteCode(ctx context.Context, inviteCode string) (Party, bool, error) {
	code := strings.ToUpper(strings.TrimSpace(inviteCode))
	return s.getParty(ctx, func(ctx context.Context) (partySnapshotRow, error) {
		row, err := s.q().GetPartySnapshotByInviteCode(ctx, code)
		return partySnapshotRow{
			ID: row.ID, OwnerUserID: row.OwnerUserID, InviteCode: row.InviteCode,
			State: string(row.State), Mode: string(row.Mode), MapScope: row.MapScope,
			LastMatchID: anyText(row.LastMatchID), LastMatchStatus: row.LastMatchStatus,
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
	return PartyStateOwner{State: State(row.State), OwnerUserID: row.OwnerUserID.String()}, nil
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
	return PartyStateExpiry{State: State(row.State), ExpiresAt: row.ExpiresAt.Time}, nil
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

func (s *PGStore) LockOpenPartyMode(ctx context.Context, partyID string) (matchkind.Mode, error) {
	id, err := profileUUID(partyID)
	if err != nil {
		return "", partyStoreError(err)
	}
	mode, err := s.q().LockOpenPartyMode(ctx, id)
	return matchkind.Mode(mode), partyStoreError(err)
}

func (s *PGStore) ShufflePartyTeams(ctx context.Context, partyID string) error {
	id, err := profileUUID(partyID)
	if err != nil {
		return err
	}
	return s.q().ShufflePartyTeams(ctx, id)
}

func (s *PGStore) SetPartyMode(ctx context.Context, partyID string, mode matchkind.Mode) error {
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

// MarkMatchStartedTx makes the match the party's current one and clears its members' ready flags, in
// the transaction that starts the match. The party is in the match for as long as the match is live.
func MarkMatchStartedTx(ctx context.Context, tx pgx.Tx, partyID, matchID string) error {
	partyUUID, matchUUID, err := profileUUID2(partyID, matchID)
	if err != nil {
		return err
	}
	if _, err := db.New(tx).MarkPartyMatchStarted(ctx, db.MarkPartyMatchStartedParams{ID: partyUUID, LastMatchID: matchUUID}); err != nil {
		return err
	}
	return nil
}

// TouchMemberSeen records that the member has the party open. It reports whether that changed
// their presence, so the party should hear about it.
func (s *PGStore) TouchMemberSeen(ctx context.Context, partyID, userID string) (bool, error) {
	partyUUID, userUUID, err := profileUUID2(partyID, userID)
	if err != nil {
		return false, err
	}
	previous, err := s.q().TouchPartyMemberSeen(ctx, db.TouchPartyMemberSeenParams{PartyID: partyUUID, UserID: userUUID})
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return memberPresence(time.Now(), previous) != PresenceOnline, nil
}

// ClearMemberSeen marks the member as gone from the party. It reports whether they were there.
func (s *PGStore) ClearMemberSeen(ctx context.Context, partyID, userID string) (bool, error) {
	partyUUID, userUUID, err := profileUUID2(partyID, userID)
	if err != nil {
		return false, err
	}
	cleared, err := s.q().ClearPartyMemberSeen(ctx, db.ClearPartyMemberSeenParams{PartyID: partyUUID, UserID: userUUID})
	return cleared > 0, err
}

const (
	partyPresenceOnlineFor = 15 * time.Second
	partyPresenceAwayFor   = 60 * time.Second
)

// memberPresence is online while the member had the party open within the last 15 seconds, away
// within the last minute, and offline after.
func memberPresence(now time.Time, seenAt pgtype.Timestamptz) Presence {
	if !seenAt.Valid {
		return PresenceOffline
	}
	switch age := now.Sub(seenAt.Time); {
	case age <= partyPresenceOnlineFor:
		return PresenceOnline
	case age <= partyPresenceAwayFor:
		return PresenceAway
	default:
		return PresenceOffline
	}
}

func (s *PGStore) ExpireParty(ctx context.Context, partyID string) error {
	id, err := profileUUID(partyID)
	if err != nil {
		return err
	}
	return s.q().ExpireParty(ctx, id)
}

// --- maintenance (PartyMaintenance) ---

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

// --- snapshot shaping ---

type partySnapshotRow struct {
	ID, OwnerUserID                   pgtype.UUID
	InviteCode, State, Mode, MapScope string
	LastMatchID, LastMatchStatus      string
	ConfigJSON, MapID                 string
	CreatedAt, ExpiresAt              pgtype.Timestamptz
	MapName                           string
	MapLocationCount                  int32
}

func (s *PGStore) getParty(ctx context.Context, fetch func(ctx context.Context) (partySnapshotRow, error)) (Party, bool, error) {
	var snap Party
	row, err := fetch(ctx)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Party{}, false, nil
		}
		return Party{}, false, err
	}
	snap.ID, snap.InviteCode, snap.OwnerUserID = row.ID.String(), row.InviteCode, row.OwnerUserID.String()
	snap.State, snap.Mode, snap.MapScope = State(row.State), matchkind.Mode(row.Mode), row.MapScope
	snap.LastMatchID = row.LastMatchID
	// An open party whose last match is starting or live is in that match.
	if snap.State == StateOpen && lastMatchOpen(row.LastMatchStatus) {
		snap.State = StateInMatch
		snap.ActiveMatchID = row.LastMatchID
	}
	snap.CreatedAt, snap.ExpiresAt = row.CreatedAt.Time, row.ExpiresAt.Time
	snap.MapName, snap.MapLocationCount = row.MapName, int(row.MapLocationCount)
	_ = json.Unmarshal([]byte(row.ConfigJSON), &snap.Config)
	snap.Config = gameplay.NormalizeMatchConfig(snap.Config)
	if row.MapID != "" {
		snap.Config.MapID = row.MapID
		snap.Config.MapName = snap.MapName
	}
	members, err := s.listPartyMembers(ctx, snap.ID)
	if err != nil {
		return Party{}, false, err
	}
	snap.Members = members
	return snap, true, nil
}

// lastMatchOpen reports whether gd_match_status says a party's last match is starting or live, as
// the party state queries do.
func lastMatchOpen(status string) bool {
	return status == "starting" || status == "live"
}

func toPartySnapshotRow(row db.GetPartySnapshotByIDRow) partySnapshotRow {
	return partySnapshotRow{
		ID: row.ID, OwnerUserID: row.OwnerUserID, InviteCode: row.InviteCode,
		State: string(row.State), Mode: string(row.Mode), MapScope: row.MapScope,
		LastMatchID: anyText(row.LastMatchID), LastMatchStatus: row.LastMatchStatus,
		CreatedAt: row.CreatedAt, ExpiresAt: row.ExpiresAt,
		ConfigJSON: string(row.ConfigJson), MapID: anyText(row.MapID),
		MapName: row.DisplayName, MapLocationCount: row.LocationCount,
	}
}

func (s *PGStore) listPartyMembers(ctx context.Context, partyID string) ([]Member, error) {
	partyUUID, err := profileUUID(partyID)
	if err != nil {
		return nil, err
	}
	rows, err := s.q().ListPartyMembers(ctx, partyUUID)
	if err != nil {
		return nil, err
	}
	out := []Member{}
	selected := map[string]string{}
	now := time.Now()
	for _, row := range rows {
		var member Member
		member.UserID = row.UserID.String()
		member.DisplayName = row.DisplayName
		member.AvatarURL = row.AvatarUrl
		member.IsGuest = row.IsGuest
		member.IsAdmin = row.IsAdmin
		member.TeamID = string(row.TeamID)
		member.Role = string(row.Role)
		member.Ready = row.Ready
		member.JoinedAt = row.JoinedAt.Time
		member.InActiveMatch = row.InActiveMatch
		member.PresenceStatus = memberPresence(now, row.SeenAt)
		member.Connected = member.PresenceStatus == PresenceOnline
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

func (s *PGStore) selectedPartyBadges(ctx context.Context, selected map[string]string) (map[string]*badges.Badge, error) {
	if len(selected) == 0 {
		return map[string]*badges.Badge{}, nil
	}
	userIDs := make([]pgtype.UUID, 0, len(selected))
	for userID := range selected {
		if strings.TrimSpace(userID) != "" {
			userIDs = append(userIDs, chatUUID(userID))
		}
	}
	if len(userIDs) == 0 {
		return map[string]*badges.Badge{}, nil
	}
	rows, err := s.q().ListPartyMemberBadges(ctx, userIDs)
	if err != nil {
		return nil, err
	}
	out := map[string]*badges.Badge{}
	fallback := map[string]*badges.Badge{}
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
