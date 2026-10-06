package badges

import (
	"geoduels/pkg/contracts"
	pkgstaff "geoduels/pkg/staff"
)

type Store interface {
	SyncLoginBadges(userID string) error
	AwardDiscordServerMemberByDiscordID(discordUserID string) (bool, error)
	GetDiscordLinkedUser(discordUserID string) (DiscordLinkedUser, bool, error)
	CreateDonationRef(userID string) (string, error)
	AwardSupporterByDonationRef(ref string) (bool, error)
	ListAdminGrantableBadges() []AdminBadgeDefinition
	GrantBadgeToUser(nickname, badgeID, actorUserID string) (contracts.PlayerBadge, bool, error)
}

type Service struct{ store Store }

func NewService(store Store) *Service { return &Service{store: store} }

// Grant is the result of granting a badge by nickname.
type Grant struct {
	Badge   contracts.PlayerBadge `json:"badge"`
	Changed bool                  `json:"changed"`
}

func (s *Service) SyncLoginBadges(userID string) error { return s.store.SyncLoginBadges(userID) }

func (s *Service) CreateDonationRef(userID string) (string, error) {
	return s.store.CreateDonationRef(userID)
}

func (s *Service) AwardSupporterByDonationRef(ref string) (bool, error) {
	return s.store.AwardSupporterByDonationRef(ref)
}

// Catalog lists the badges staff may grant by hand.
func (s *Service) Catalog(actor pkgstaff.Actor) ([]AdminBadgeDefinition, error) {
	if err := actor.RequireCap(pkgstaff.CapManageAccess); err != nil {
		return nil, err
	}
	return s.store.ListAdminGrantableBadges(), nil
}

// Grant awards a badge to the player who claimed nickname.
func (s *Service) Grant(actor pkgstaff.Actor, nickname, badgeID string) (Grant, error) {
	if err := actor.RequireCap(pkgstaff.CapManageAccess); err != nil {
		return Grant{}, err
	}
	badge, changed, err := s.store.GrantBadgeToUser(nickname, badgeID, actor.ID)
	if err != nil {
		return Grant{}, err
	}
	return Grant{Badge: badge, Changed: changed}, nil
}
