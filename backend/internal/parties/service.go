package parties

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"geoduels/pkg/contracts"
	"geoduels/pkg/matchkind"
)

// MemberRole is the policy-level party role used when composing membership
// primitives. The persistence layer maps it onto the database enum.
type MemberRole string

const (
	MemberRoleOwner  MemberRole = "owner"
	MemberRoleMember MemberRole = "member"
)

// PartyStateOwner is a primitive ownership/state read.
type PartyStateOwner struct {
	State       contracts.PartyState
	OwnerUserID string
}

// PartyStateExpiry is a primitive state/expiry read.
type PartyStateExpiry struct {
	State     contracts.PartyState
	ExpiresAt time.Time
}

// PartyInsert is the primitive create request assembled by the service so the
// store never reads the clock or generates identifiers.
type PartyInsert struct {
	ID          string
	InviteCode  string
	OwnerUserID string
	Mode        contracts.MatchMode
	MapScope    string
	MapID       string
	ExpiresAt   time.Time
}

// Service owns party orchestration. Policy, validation, expiry and transaction
// boundaries live here; the store handles persistence.
type Service struct {
	store Store
	clock func() time.Time
}

func NewService(store Store) *Service {
	return &Service{store: store, clock: time.Now}
}

// partyOpTimeout bounds the detached context used by the no-context transport
// API that predates context threading in this feature.
const partyOpTimeout = 8 * time.Second

func (s *Service) op() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), partyOpTimeout)
}

// CreateParty owns validation, defaults, map resolution, expiry, invite-code
// allocation and the retry loop. Persistence only inserts.
func (s *Service) CreateParty(ownerUserID string, mode contracts.MatchMode, mapScope string, ttl time.Duration) (contracts.PartySnapshot, error) {
	ownerUserID = strings.TrimSpace(ownerUserID)
	if ownerUserID == "" {
		return contracts.PartySnapshot{}, errors.New("owner required")
	}
	if mode == "" {
		mode = contracts.ModeDuel
	}
	if _, ok := matchkind.ForParty(mode); !ok {
		return contracts.PartySnapshot{}, errors.New("unsupported party mode")
	}
	if strings.TrimSpace(mapScope) == "" {
		mapScope = "world"
	}
	if ttl <= 0 {
		ttl = 2 * time.Hour
	}
	mapID, err := s.store.ResolveGameplayMapID(contracts.ModeDuel, contracts.RulesetMoving, "")
	if err != nil {
		return contracts.PartySnapshot{}, fmt.Errorf("resolve party map: %w", err)
	}
	expiresAt := s.now().Add(ttl)
	ctx, cancel := s.op()
	defer cancel()
	for attempt := 0; attempt < 5; attempt++ {
		partyID := newPartyID()
		inviteCode := newPartyCode(s.now())
		err := s.store.WithinTx(ctx, func(store Store) error {
			return store.InsertParty(ctx, PartyInsert{
				ID:          partyID,
				InviteCode:  inviteCode,
				OwnerUserID: ownerUserID,
				Mode:        mode,
				MapScope:    mapScope,
				MapID:       mapID,
				ExpiresAt:   expiresAt,
			})
		})
		if err != nil {
			if attempt == 4 {
				return contracts.PartySnapshot{}, fmt.Errorf("insert party: %w", err)
			}
			continue
		}
		snap, _, err := s.store.GetPartyByID(ctx, partyID)
		if err != nil {
			return contracts.PartySnapshot{}, fmt.Errorf("read created party: %w", err)
		}
		return snap, nil
	}
	return contracts.PartySnapshot{}, errors.New("could not allocate party invite code")
}

func (s *Service) GetCurrentParty(userID string) (*contracts.CurrentParty, error) {
	ctx, cancel := s.op()
	defer cancel()
	return s.store.GetCurrentParty(ctx, userID)
}

func (s *Service) GetPartyByID(lobbyID string) (contracts.PartySnapshot, bool, error) {
	ctx, cancel := s.op()
	defer cancel()
	return s.store.GetPartyByID(ctx, lobbyID)
}

func (s *Service) GetPartyByInviteCode(inviteCode string) (contracts.PartySnapshot, bool, error) {
	ctx, cancel := s.op()
	defer cancel()
	return s.store.GetPartyByInviteCode(ctx, inviteCode)
}

// TouchMemberSeen records that the member has the party open, reporting whether their presence
// changed.
func (s *Service) TouchMemberSeen(partyID, userID string) (bool, error) {
	ctx, cancel := s.op()
	defer cancel()
	return s.store.TouchMemberSeen(ctx, partyID, userID)
}

// ClearMemberSeen marks the member as gone from the party, reporting whether they were there.
func (s *Service) ClearMemberSeen(partyID, userID string) (bool, error) {
	ctx, cancel := s.op()
	defer cancel()
	return s.store.ClearMemberSeen(ctx, partyID, userID)
}

// SetPartyMode owns the party-open check and the team-shuffle decision that
// runs when switching into team duel.
func (s *Service) SetPartyMode(lobbyID string, mode contracts.MatchMode) error {
	partyID := strings.TrimSpace(lobbyID)
	if _, ok := matchkind.ForParty(mode); partyID == "" || !ok {
		return errors.New("invalid party mode")
	}
	ctx, cancel := s.op()
	defer cancel()
	return s.store.WithinTx(ctx, func(store Store) error {
		current, err := store.LockOpenPartyMode(ctx, partyID)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				return errors.New("party is not open")
			}
			return err
		}
		if current != contracts.ModeTeamDuel && mode == contracts.ModeTeamDuel {
			if err := store.ShufflePartyTeams(ctx, partyID); err != nil {
				return err
			}
		}
		return store.SetPartyMode(ctx, partyID, mode)
	})
}

// SetPartyConfig owns map canonicalization, accessibility and ready-reset
// policy inside one transaction.
func (s *Service) SetPartyConfig(lobbyID string, cfg contracts.MatchConfig) (contracts.PartySnapshot, error) {
	partyID := strings.TrimSpace(lobbyID)
	cfg = contracts.NormalizeMatchConfig(cfg)
	ctx, cancel := s.op()
	defer cancel()
	err := s.store.WithinTx(ctx, func(store Store) error {
		ownerUserID, err := store.LockOpenPartyOwner(ctx, partyID)
		if err != nil {
			return err
		}
		canonicalMapID, err := store.ResolveMapIdentity(ctx, cfg.MapID)
		if err != nil {
			return ErrPartyMapUnavailable
		}
		cfg.MapID = canonicalMapID
		accessible, err := store.PartyMapAccessible(ctx, cfg.MapID, ownerUserID)
		if err != nil {
			return err
		}
		if !accessible {
			return ErrPartyMapUnavailable
		}
		body, _ := json.Marshal(cfg)
		if err := store.SetPartyConfig(ctx, partyID, body, cfg.MapID); err != nil {
			return err
		}
		return store.ResetPartyMembersReady(ctx, partyID)
	})
	if err != nil {
		return contracts.PartySnapshot{}, err
	}
	snap, _, err := s.store.GetPartyByID(ctx, partyID)
	return snap, err
}

// JoinParty owns joinability, capacity and owner-role policy.
func (s *Service) JoinParty(lobbyID, userID string) (contracts.PartySnapshot, error) {
	partyID := strings.TrimSpace(lobbyID)
	ctx, cancel := s.op()
	defer cancel()
	err := s.store.WithinTx(ctx, func(store Store) error {
		if err := s.requireJoinable(ctx, store, partyID); err != nil {
			return err
		}
		active, err := store.CountActivePartyMembers(ctx, partyID, userID)
		if err != nil {
			return err
		}
		if active >= contracts.MaxPartyMembers {
			return errors.New("party is full")
		}
		role := MemberRoleMember
		snap, found, err := store.GetPartyByID(ctx, partyID)
		if err != nil {
			return err
		}
		if !found {
			return ErrNotFound
		}
		if snap.OwnerUserID == userID {
			role = MemberRoleOwner
		}
		if err := store.JoinPartyMember(ctx, partyID, userID, role); err != nil {
			return err
		}
		return store.TouchPartyUpdated(ctx, partyID)
	})
	if err != nil {
		return contracts.PartySnapshot{}, err
	}
	snap, _, err := s.store.GetPartyByID(ctx, partyID)
	return snap, err
}

// LeaveParty owns the in-progress leader rule and the ownership handoff policy.
func (s *Service) LeaveParty(lobbyID, userID string) (contracts.PartySnapshot, error) {
	partyID := strings.TrimSpace(lobbyID)
	ctx, cancel := s.op()
	defer cancel()
	err := s.store.WithinTx(ctx, func(store Store) error {
		party, err := store.GetPartyStateAndOwner(ctx, partyID)
		if err != nil {
			return err
		}
		state := party.State
		if state != contracts.PartyOpen && state != contracts.PartyInMatch {
			return errors.New("party is not joinable")
		}
		if party.OwnerUserID == userID && state != contracts.PartyOpen {
			return errors.New("leader cannot leave the party while a game is in progress")
		}
		removed, err := store.LeavePartyMember(ctx, partyID, userID)
		if err != nil {
			return err
		}
		if removed == 0 {
			return ErrNotFound
		}
		if party.OwnerUserID == userID {
			nextOwner, err := store.NextPartyOwnerID(ctx, partyID)
			if errors.Is(err, ErrNotFound) {
				if err := store.CloseParty(ctx, partyID); err != nil {
					return err
				}
			} else if err != nil {
				return err
			} else {
				active, err := store.PartyMemberActive(ctx, partyID, nextOwner)
				if err != nil {
					return err
				}
				if !active {
					return ErrNotFound
				}
				if err := store.TransferPartyOwner(ctx, partyID, nextOwner); err != nil {
					return err
				}
				if err := store.ReassignPartyRoles(ctx, partyID, nextOwner); err != nil {
					return err
				}
			}
		}
		return store.TouchPartyUpdated(ctx, partyID)
	})
	if err != nil {
		return contracts.PartySnapshot{}, err
	}
	next, _, err := s.store.GetPartyByID(ctx, partyID)
	return next, err
}

// ShufflePartyTeams owns ownership authorization and the team-duel-only rule.
func (s *Service) ShufflePartyTeams(lobbyID, ownerUserID string) (contracts.PartySnapshot, error) {
	partyID := strings.TrimSpace(lobbyID)
	ownerUserID = strings.TrimSpace(ownerUserID)
	if partyID == "" || ownerUserID == "" {
		return contracts.PartySnapshot{}, errors.New("invalid party")
	}
	ctx, cancel := s.op()
	defer cancel()
	err := s.store.WithinTx(ctx, func(store Store) error {
		if err := authorizeOwner(ctx, store, partyID, ownerUserID); err != nil {
			return err
		}
		snap, found, err := store.GetPartyByID(ctx, partyID)
		if err != nil {
			return err
		}
		if !found {
			return ErrNotFound
		}
		if snap.Mode != contracts.ModeTeamDuel {
			return errors.New("shuffle is only available in team duels")
		}
		if err := store.ShufflePartyTeams(ctx, partyID); err != nil {
			return err
		}
		return store.TouchOpenParty(ctx, partyID)
	})
	if err != nil {
		return contracts.PartySnapshot{}, err
	}
	next, _, err := s.store.GetPartyByID(ctx, partyID)
	return next, err
}

// SetPartyMemberTeam owns open-state and team-value validation.
func (s *Service) SetPartyMemberTeam(lobbyID, userID, teamID string) (contracts.PartySnapshot, error) {
	partyID := strings.TrimSpace(lobbyID)
	userID = strings.TrimSpace(userID)
	teamID = strings.ToLower(strings.TrimSpace(teamID))
	if partyID == "" || userID == "" || (teamID != "a" && teamID != "b") {
		return contracts.PartySnapshot{}, errors.New("invalid party team")
	}
	ctx, cancel := s.op()
	defer cancel()
	err := s.store.WithinTx(ctx, func(store Store) error {
		if err := s.requireOpen(ctx, store, partyID); err != nil {
			return err
		}
		updated, err := store.SetPartyMemberTeam(ctx, partyID, userID, teamID)
		if err != nil {
			return err
		}
		if updated == 0 {
			return ErrNotFound
		}
		return store.TouchOpenParty(ctx, partyID)
	})
	if err != nil {
		return contracts.PartySnapshot{}, err
	}
	snap, _, err := s.store.GetPartyByID(ctx, partyID)
	return snap, err
}

// KickPartyMember owns ownership authorization and member validation.
func (s *Service) KickPartyMember(lobbyID, ownerUserID, targetUserID string) (contracts.PartySnapshot, error) {
	partyID := strings.TrimSpace(lobbyID)
	ownerUserID = strings.TrimSpace(ownerUserID)
	targetUserID = strings.TrimSpace(targetUserID)
	if ownerUserID == "" || targetUserID == "" || ownerUserID == targetUserID {
		return contracts.PartySnapshot{}, errors.New("invalid party member")
	}
	ctx, cancel := s.op()
	defer cancel()
	err := s.store.WithinTx(ctx, func(store Store) error {
		if err := authorizeOwner(ctx, store, partyID, ownerUserID); err != nil {
			return err
		}
		removed, err := store.KickPartyMember(ctx, partyID, targetUserID)
		if err != nil {
			return err
		}
		if removed == 0 {
			return ErrNotFound
		}
		return nil
	})
	if err != nil {
		return contracts.PartySnapshot{}, err
	}
	snap, _, err := s.store.GetPartyByID(ctx, partyID)
	return snap, err
}

// TransferPartyOwner owns ownership authorization, target validation and the
// active-member check before reassigning roles.
func (s *Service) TransferPartyOwner(lobbyID, ownerUserID, targetUserID string) (contracts.PartySnapshot, error) {
	partyID := strings.TrimSpace(lobbyID)
	ownerUserID = strings.TrimSpace(ownerUserID)
	targetUserID = strings.TrimSpace(targetUserID)
	ctx, cancel := s.op()
	defer cancel()
	err := s.store.WithinTx(ctx, func(store Store) error {
		if err := authorizeOwner(ctx, store, partyID, ownerUserID); err != nil {
			return err
		}
		if ownerUserID == targetUserID {
			return errors.New("invalid party member")
		}
		active, err := store.PartyMemberActive(ctx, partyID, targetUserID)
		if err != nil {
			return err
		}
		if !active {
			return ErrNotFound
		}
		if err := store.TransferPartyOwner(ctx, partyID, targetUserID); err != nil {
			return err
		}
		return store.ReassignPartyRoles(ctx, partyID, targetUserID)
	})
	if err != nil {
		return contracts.PartySnapshot{}, err
	}
	snap, _, err := s.store.GetPartyByID(ctx, partyID)
	return snap, err
}

func (s *Service) ListOpenPartyIDs() ([]string, error) {
	ctx, cancel := s.op()
	defer cancel()
	return s.store.ListOpenPartyIDs(ctx)
}

func (s *Service) CloseInactiveOpenParties(lobbyIDs []string, inactiveFor time.Duration) (int64, error) {
	ctx, cancel := s.op()
	defer cancel()
	var closed int64
	err := s.store.WithinTx(ctx, func(store Store) error {
		n, err := store.CloseInactiveOpenParties(ctx, lobbyIDs, inactiveFor)
		if err != nil {
			return err
		}
		closed = n
		return nil
	})
	return closed, err
}

// authorizeOwner is the shared ownership gate: the party must be open and the
// caller must be its owner.
func authorizeOwner(ctx context.Context, store Store, partyID, ownerUserID string) error {
	party, err := store.GetPartyStateAndOwner(ctx, partyID)
	if err != nil {
		return err
	}
	if party.State != contracts.PartyOpen {
		return errors.New("party is not open")
	}
	if party.OwnerUserID != ownerUserID {
		return errors.New("forbidden")
	}
	return nil
}

// requireOpen rejects anything but a live, unexpired open party.
func (s *Service) requireOpen(ctx context.Context, store Store, partyID string) error {
	party, err := store.GetPartyStateAndExpiry(ctx, partyID)
	if err != nil {
		return err
	}
	if party.State != contracts.PartyOpen {
		return errors.New("party is not open")
	}
	if s.now().After(party.ExpiresAt) {
		return errors.New("party expired")
	}
	return nil
}

// requireJoinable accepts the states a player may join and rejects expired
// parties.
func (s *Service) requireJoinable(ctx context.Context, store Store, partyID string) error {
	party, err := store.GetPartyStateAndExpiry(ctx, partyID)
	if err != nil {
		return err
	}
	if party.State != contracts.PartyOpen && party.State != contracts.PartyInMatch {
		return errors.New("party is not joinable")
	}
	if s.now().After(party.ExpiresAt) {
		return errors.New("party expired")
	}
	return nil
}

func (s *Service) now() time.Time { return s.clock().UTC() }
