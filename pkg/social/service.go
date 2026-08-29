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

// Store is the narrow persistence capability required by social use cases.
type Store interface {
	GetSocialAccount(context.Context, string) (bool, bool, bool, error)
	GetSocialSettings(context.Context, string) (SocialSettings, error)
	UpdateSocialSettings(context.Context, string, SocialSettings) (SocialSettings, error)
	Relationship(context.Context, string, string) (RelationshipState, string, error)
	ListFriends(context.Context, string, int) ([]CompactPlayer, error)
	ListFriendRequests(context.Context, string, string, int) ([]FriendRequest, error)
	SearchSocialPlayers(context.Context, string, string, int) ([]CompactPlayer, error)
	ListRecentPlayers(context.Context, string, int) ([]CompactPlayer, error)
	SendFriendRequest(context.Context, string, string) (FriendRequest, error)
	RespondFriendRequest(context.Context, string, string, string) error
	RemoveFriend(context.Context, string, string) error
	SetUserBlock(context.Context, string, string, bool) error
	CreateFriendCode(context.Context, string, time.Duration) (FriendCode, error)
	ResolveFriendCode(context.Context, string, string) (CompactPlayer, error)
	CreatePartyInvitation(context.Context, string, string, string, time.Duration) (PartyInvitation, error)
	ListPartyInviteStatus(context.Context, string, string) (map[string]CompactPartyInvite, error)
	RespondPartyInvitation(context.Context, string, string, string) (PartyInvitation, error)
	ListPartyInvitations(context.Context, string, int) ([]PartyInvitation, error)
}

// Service owns social orchestration and is deliberately thin where persistence
// already provides an atomic operation.
type Service struct{ store Store }

func NewService(store Store) *Service { return &Service{store: store} }
func (s *Service) Authorize(ctx context.Context, userID string) error {
	isGuest, _, _, err := s.store.GetSocialAccount(ctx, userID)
	if err != nil {
		return err
	}
	if isGuest {
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
	return s.store.GetSocialAccount(ctx, id)
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
func (s *Service) SendFriendRequest(ctx context.Context, a, b string) (FriendRequest, error) {
	return s.store.SendFriendRequest(ctx, a, b)
}
func (s *Service) RespondFriendRequest(ctx context.Context, a, b, c string) error {
	return s.store.RespondFriendRequest(ctx, a, b, c)
}
func (s *Service) RemoveFriend(ctx context.Context, a, b string) error {
	return s.store.RemoveFriend(ctx, a, b)
}
func (s *Service) SetUserBlock(ctx context.Context, a, b string, v bool) error {
	return s.store.SetUserBlock(ctx, a, b, v)
}
func (s *Service) CreateFriendCode(ctx context.Context, id string, d time.Duration) (FriendCode, error) {
	return s.store.CreateFriendCode(ctx, id, d)
}
func (s *Service) ResolveFriendCode(ctx context.Context, id, c string) (CompactPlayer, error) {
	return s.store.ResolveFriendCode(ctx, id, c)
}
func (s *Service) CreatePartyInvitation(ctx context.Context, a, b, c string, d time.Duration) (PartyInvitation, error) {
	return s.store.CreatePartyInvitation(ctx, a, b, c, d)
}
func (s *Service) ListPartyInviteStatus(ctx context.Context, a, b string) (map[string]CompactPartyInvite, error) {
	return s.store.ListPartyInviteStatus(ctx, a, b)
}
func (s *Service) RespondPartyInvitation(ctx context.Context, a, b, c string) (PartyInvitation, error) {
	return s.store.RespondPartyInvitation(ctx, a, b, c)
}
func (s *Service) ListPartyInvitations(ctx context.Context, a string, n int) ([]PartyInvitation, error) {
	return s.store.ListPartyInvitations(ctx, a, n)
}
