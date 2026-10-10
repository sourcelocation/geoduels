package parties

import (
	"context"
	"time"

	"geoduels/pkg/contracts"
)

// Store persists parties data. WithinTx supplies a store bound to one
// transaction; callback errors roll back all its writes.
type Store interface {
	WithinTx(ctx context.Context, fn func(Store) error) error
	ResolveGameplayMapID(mode contracts.MatchMode, ruleset contracts.GameRuleset, requestedMapID string) (string, error)
	GetCurrentParty(ctx context.Context, userID string) (*contracts.CurrentParty, error)
	GetPartyByID(ctx context.Context, partyID string) (contracts.PartySnapshot, bool, error)
	GetPartyByInviteCode(ctx context.Context, inviteCode string) (contracts.PartySnapshot, bool, error)
	GetPartyStateAndOwner(ctx context.Context, partyID string) (PartyStateOwner, error)
	GetPartyStateAndExpiry(ctx context.Context, partyID string) (PartyStateExpiry, error)
	PartyMemberActive(ctx context.Context, partyID, userID string) (bool, error)
	CountActivePartyMembers(ctx context.Context, partyID, userID string) (int, error)
	InsertParty(ctx context.Context, input PartyInsert) error
	LockOpenPartyMode(ctx context.Context, partyID string) (contracts.MatchMode, error)
	ShufflePartyTeams(ctx context.Context, partyID string) error
	SetPartyMode(ctx context.Context, partyID string, mode contracts.MatchMode) error
	LockOpenPartyOwner(ctx context.Context, partyID string) (string, error)
	ResolveMapIdentity(ctx context.Context, requestedMapID string) (string, error)
	PartyMapAccessible(ctx context.Context, mapID, ownerUserID string) (bool, error)
	SetPartyConfig(ctx context.Context, partyID string, configJSON []byte, mapID string) error
	ResetPartyMembersReady(ctx context.Context, partyID string) error
	JoinPartyMember(ctx context.Context, partyID, userID string, role MemberRole) error
	LeavePartyMember(ctx context.Context, partyID, userID string) (int64, error)
	NextPartyOwnerID(ctx context.Context, partyID string) (string, error)
	CloseParty(ctx context.Context, partyID string) error
	TransferPartyOwner(ctx context.Context, partyID, targetUserID string) error
	ReassignPartyRoles(ctx context.Context, partyID, targetUserID string) error
	KickPartyMember(ctx context.Context, partyID, targetUserID string) (int64, error)
	SetPartyMemberTeam(ctx context.Context, partyID, userID, teamID string) (int64, error)
	TouchPartyUpdated(ctx context.Context, partyID string) error
	TouchOpenParty(ctx context.Context, partyID string) error
	TouchMemberSeen(ctx context.Context, partyID, userID string) (bool, error)
	ClearMemberSeen(ctx context.Context, partyID, userID string) (bool, error)
	ExpireParty(ctx context.Context, partyID string) error
	ListOpenPartyIDs(ctx context.Context) ([]string, error)
	CloseInactiveOpenParties(ctx context.Context, partyIDs []string, inactiveFor time.Duration) (int64, error)
}
