package social

import (
	"context"
	"errors"
	"sync"
	"time"
)

var (
	ErrNotFound             = errors.New("social resource not found")
	ErrBlocked              = errors.New("social action unavailable")
	ErrLimit                = errors.New("social limit reached")
	ErrRegistrationRequired = errors.New("social registration required")
)

// SocialAccount is the eligibility shape the policy layer reasons about.
type SocialAccount struct {
	IsGuest             bool
	RequestsEnabled     bool
	PartyInvitesEnabled bool
}

// Service owns social orchestration. Policy, limits, TTLs and transaction
// boundaries live here; the store handles persistence.
type Service struct {
	store Store
	clock func() time.Time
}

func NewService(store Store) *Service {
	return &Service{store: store, clock: time.Now}
}

func (s *Service) Authorize(ctx context.Context, userID string) error {
	account, err := s.store.GetSocialAccount(ctx, userID)
	if err != nil {
		return err
	}
	if account.IsGuest {
		return ErrRegistrationRequired
	}
	return nil
}

type FriendsPageResult struct {
	Friends      []CompactPlayer
	Incoming     []FriendRequest
	Outgoing     []FriendRequest
	Recent       []CompactPlayer
	PartyInvites map[string]CompactPartyInvite
}

func (s *Service) FriendsPage(ctx context.Context, userID string, partyID string) (FriendsPageResult, error) {
	var result FriendsPageResult
	var errs [4]error
	var wg sync.WaitGroup
	wg.Add(4)
	go func() { defer wg.Done(); result.Friends, errs[0] = s.store.ListFriends(ctx, userID, 100) }()
	go func() {
		defer wg.Done()
		result.Incoming, errs[1] = s.store.ListFriendRequests(ctx, userID, "incoming", 20)
	}()
	go func() {
		defer wg.Done()
		result.Outgoing, errs[2] = s.store.ListFriendRequests(ctx, userID, "outgoing", 20)
	}()
	go func() { defer wg.Done(); result.Recent, errs[3] = s.store.ListRecentPlayers(ctx, userID, 3) }()
	wg.Wait()
	if err := errors.Join(errs[:]...); err != nil {
		return FriendsPageResult{}, err
	}
	if partyID != "" {
		result.PartyInvites, errs[0] = s.store.ListPartyInviteStatus(ctx, userID, partyID)
		if errs[0] != nil {
			return FriendsPageResult{}, errs[0]
		}
	}
	return result, nil
}

func (s *Service) GetSocialAccount(ctx context.Context, id string) (bool, bool, bool, error) {
	account, err := s.store.GetSocialAccount(ctx, id)
	if err != nil {
		return false, false, false, err
	}
	return account.IsGuest, account.RequestsEnabled, account.PartyInvitesEnabled, nil
}

func (s *Service) GetSocialSettings(ctx context.Context, id string) (SocialSettings, error) {
	return s.store.GetSocialSettings(ctx, id)
}

func (s *Service) UpdateSocialSettings(ctx context.Context, id string, v SocialSettings) (SocialSettings, error) {
	return s.store.UpdateSocialSettings(ctx, id, v)
}

func (s *Service) Relationship(ctx context.Context, a, b string) (RelationshipState, string, error) {
	return s.store.Relationship(ctx, a, b)
}

func (s *Service) ListFriends(ctx context.Context, id string, n int) ([]CompactPlayer, error) {
	return s.store.ListFriends(ctx, id, n)
}

func (s *Service) ListFriendRequests(ctx context.Context, id, d string, n int) ([]FriendRequest, error) {
	return s.store.ListFriendRequests(ctx, id, d, n)
}

func (s *Service) SearchSocialPlayers(ctx context.Context, id, q string, n int) ([]CompactPlayer, error) {
	return s.store.SearchSocialPlayers(ctx, id, q, n)
}

func (s *Service) ListRecentPlayers(ctx context.Context, id string, n int) ([]CompactPlayer, error) {
	return s.store.ListRecentPlayers(ctx, id, n)
}

// SendFriendRequest owns eligibility, the friend limit, the request TTL, and
// the crossed-request auto-accept inside one transaction.
func (s *Service) SendFriendRequest(ctx context.Context, userID, targetID string) (FriendRequest, error) {
	if userID == targetID {
		return FriendRequest{}, ErrBlocked
	}
	var out FriendRequest
	err := s.store.WithinTx(ctx, func(store Store) error {
		account, err := store.GetSocialAccount(ctx, userID)
		if err != nil {
			return err
		}
		allowed, err := store.SendAllowed(ctx, userID, targetID)
		if err != nil {
			return err
		}
		friendCount, err := store.CountFriends(ctx, userID)
		if err != nil {
			return err
		}
		if err := Authorize(Account{
			IsGuest:       account.IsGuest,
			ActionEnabled: account.RequestsEnabled,
			Blocked:       !allowed,
			TargetExists:  true,
			AtLimit:       friendCount >= FriendLimit,
			SameUser:      userID == targetID,
		}); err != nil {
			return err
		}

		if crossedID, ok, err := store.CrossedRequest(ctx, targetID, userID); err != nil {
			return err
		} else if ok {
			if err := store.AcceptFriendRequest(ctx, crossedID, userID); err != nil {
				return err
			}
			out = FriendRequest{ID: crossedID, Direction: "incoming"}
			return nil
		}

		now := s.now()
		request, err := store.InsertFriendRequest(ctx, userID, targetID, now.Add(FriendRequestTTL))
		if err != nil {
			return err
		}
		if err := store.NotifyUser(ctx, targetID, "friend_request_received", "friend_request:"+request.ID,
			map[string]any{"requestId": request.ID, "expiresAt": request.ExpiresAt.UTC().Format(time.RFC3339)}, userID); err != nil {
			return err
		}
		out = FriendRequest{ID: request.ID, Direction: "outgoing", CreatedAt: request.CreatedAt, ExpiresAt: request.ExpiresAt}
		return nil
	})
	if err != nil {
		return FriendRequest{}, err
	}
	return out, nil
}

func (s *Service) RespondFriendRequest(ctx context.Context, userID, requestID, response string) error {
	return s.store.WithinTx(ctx, func(store Store) error {
		if response == "accept" {
			senderID, err := store.FriendRequestSender(ctx, requestID, userID)
			if err != nil {
				return ErrNotFound
			}
			if err := store.AcceptFriendRequest(ctx, requestID, userID); err != nil {
				return err
			}
			if err := store.NotifyUser(ctx, senderID, "friendship_accepted", "friendship_accepted:"+requestID, map[string]any{}, userID); err != nil {
				return err
			}
		} else if response == "cancel" {
			if err := store.CancelFriendRequest(ctx, requestID, userID); err != nil {
				return ErrNotFound
			}
		} else {
			if err := store.DeclineFriendRequest(ctx, requestID, userID); err != nil {
				return ErrNotFound
			}
		}
		if err := store.MarkFriendRequestNotificationRead(ctx, requestID); err != nil {
			return err
		}
		return nil
	})
}

func (s *Service) RemoveFriend(ctx context.Context, a, b string) error {
	return s.store.WithinTx(ctx, func(store Store) error {
		return store.RemoveFriend(ctx, a, b)
	})
}

// SetUserBlock owns the cascade: blocking removes the friendship and cancels
// any pending requests in both directions.
func (s *Service) SetUserBlock(ctx context.Context, userID, targetID string, blocked bool) error {
	if userID == targetID {
		return ErrBlocked
	}
	return s.store.WithinTx(ctx, func(store Store) error {
		if !blocked {
			return store.RemoveUserBlock(ctx, userID, targetID)
		}
		if err := store.AddUserBlock(ctx, userID, targetID); err != nil {
			return err
		}
		if err := store.RemoveFriend(ctx, userID, targetID); err != nil {
			return err
		}
		if err := store.CancelPairFriendRequests(ctx, userID, targetID); err != nil {
			return err
		}
		return nil
	})
}

func (s *Service) CreateFriendCode(ctx context.Context, userID string, ttl time.Duration) (FriendCode, error) {
	if ttl <= 0 {
		ttl = DefaultFriendCodeTTL
	}
	var lastErr error
	for attempt := 0; attempt < 8; attempt++ {
		code, err := randomFriendCode()
		if err != nil {
			return FriendCode{}, err
		}
		expiresAt := s.now().Add(ttl)
		err = s.store.WithinTx(ctx, func(store Store) error {
			if err := store.RevokeFriendCodes(ctx, userID); err != nil {
				return err
			}
			return store.InsertFriendCode(ctx, userID, code, expiresAt)
		})
		if err == nil {
			return FriendCode{Code: code, ExpiresAt: expiresAt}, nil
		}
		lastErr = err
	}
	return FriendCode{}, lastErr
}

func (s *Service) ResolveFriendCode(ctx context.Context, id, code string) (CompactPlayer, error) {
	return s.store.ResolveFriendCode(ctx, id, code)
}

// CreatePartyInvitation owns the resend window so the store never reads the
// clock. It returns the still-pending invitation when one was sent recently.
func (s *Service) CreatePartyInvitation(ctx context.Context, partyID, inviterID, recipientID string, ttl time.Duration) (PartyInvitation, error) {
	if ttl <= 0 {
		ttl = DefaultPartyInviteTTL
	}
	var out PartyInvitation
	err := s.store.WithinTx(ctx, func(store Store) error {
		eligibility, err := store.InvitationEligibility(ctx, partyID, inviterID, recipientID)
		if err != nil {
			return ErrBlocked
		}
		if existing, ok, err := store.PendingPartyInvitation(ctx, partyID, recipientID); err != nil {
			return err
		} else if ok && s.now().Sub(existing.CreatedAt) < PartyInviteResendAfter {
			existing.InviteCode = eligibility.InviteCode
			existing.Mode = eligibility.Mode
			existing.MemberCount = eligibility.MemberCount
			out = existing
			return nil
		}
		now := s.now()
		invitation, err := store.UpsertPartyInvitation(ctx, partyID, inviterID, recipientID, now.Add(ttl), now)
		if err != nil {
			return err
		}
		invitation.PartyID = partyID
		invitation.InviteCode = eligibility.InviteCode
		invitation.Mode = eligibility.Mode
		invitation.MemberCount = eligibility.MemberCount
		if err := store.NotifyUser(ctx, recipientID, "party_invitation_received", "party_invitation:"+invitation.ID,
			map[string]any{"invitationId": invitation.ID, "expiresAt": invitation.ExpiresAt.UTC().Format(time.RFC3339)}, inviterID); err != nil {
			return err
		}
		out = invitation
		return nil
	})
	if err != nil {
		return PartyInvitation{}, err
	}
	return out, nil
}

func (s *Service) ListPartyInviteStatus(ctx context.Context, a, b string) (map[string]CompactPartyInvite, error) {
	return s.store.ListPartyInviteStatus(ctx, a, b)
}

func (s *Service) RespondPartyInvitation(ctx context.Context, userID, invitationID, response string) (PartyInvitation, error) {
	var out PartyInvitation
	err := s.store.WithinTx(ctx, func(store Store) error {
		invitation, err := store.RespondPartyInvitation(ctx, userID, invitationID, response)
		if err != nil {
			return err
		}
		if err := store.MarkPartyInvitationNotificationRead(ctx, userID, invitationID); err != nil {
			return err
		}
		out = invitation
		return nil
	})
	if err != nil {
		return PartyInvitation{}, err
	}
	return out, nil
}

func (s *Service) ListPartyInvitations(ctx context.Context, a string, n int) ([]PartyInvitation, error) {
	return s.store.ListPartyInvitations(ctx, a, n)
}

func (s *Service) now() time.Time { return s.clock().UTC() }
