package content

import pkgstaff "geoduels/pkg/staff"

type Store interface {
	GetLobbyChangelog(defaultContent LobbyChangelogContent) (LobbyChangelogContent, error)
	SetLobbyChangelog(content LobbyChangelogContent) error
	ListChangelogPosts(includeUnpublished bool) ([]ChangelogPost, error)
	GetChangelogPostBySlug(slug string, publishedOnly bool) (ChangelogPost, bool, error)
	CreateChangelogPost(input ChangelogPostInput) (ChangelogPost, error)
	UpdateChangelogPost(id int64, input ChangelogPostInput) (ChangelogPost, bool, error)
	GetModerationSettings() (ModerationSettings, error)
	SetModerationSettings(settings ModerationSettings) error
	GetDiscordIntegrationSettings() (DiscordIntegrationSettings, error)
	SetDiscordIntegrationSettings(settings DiscordIntegrationSettings) error
}

// Service serves public content and the staff operations on it. Staff
// operations check their own capabilities.
type Service struct{ store Store }

func NewService(store Store) *Service { return &Service{store: store} }

func (s *Service) LobbyChangelog(defaultContent LobbyChangelogContent) (LobbyChangelogContent, error) {
	return s.store.GetLobbyChangelog(defaultContent)
}

func (s *Service) PublishedPosts() ([]ChangelogPost, error) {
	return s.store.ListChangelogPosts(false)
}

func (s *Service) PublishedPost(slug string) (ChangelogPost, bool, error) {
	return s.store.GetChangelogPostBySlug(slug, true)
}

// AllPosts includes unpublished drafts.
func (s *Service) AllPosts(actor pkgstaff.Actor) ([]ChangelogPost, error) {
	if err := actor.RequireCap(pkgstaff.CapManageContent); err != nil {
		return nil, err
	}
	return s.store.ListChangelogPosts(true)
}

func (s *Service) CreatePost(actor pkgstaff.Actor, input ChangelogPostInput) (ChangelogPost, error) {
	if err := actor.RequireCap(pkgstaff.CapManageContent); err != nil {
		return ChangelogPost{}, err
	}
	return s.store.CreateChangelogPost(input)
}

func (s *Service) UpdatePost(actor pkgstaff.Actor, id int64, input ChangelogPostInput) (ChangelogPost, bool, error) {
	if err := actor.RequireCap(pkgstaff.CapManageContent); err != nil {
		return ChangelogPost{}, false, err
	}
	return s.store.UpdateChangelogPost(id, input)
}

func (s *Service) ModerationSettings(actor pkgstaff.Actor) (ModerationSettings, error) {
	if err := actor.RequireCap(pkgstaff.CapManageConfig); err != nil {
		return ModerationSettings{}, err
	}
	return s.store.GetModerationSettings()
}

func (s *Service) SetModerationSettings(actor pkgstaff.Actor, settings ModerationSettings) (ModerationSettings, error) {
	if err := actor.RequireCap(pkgstaff.CapManageConfig); err != nil {
		return ModerationSettings{}, err
	}
	if err := s.store.SetModerationSettings(settings); err != nil {
		return ModerationSettings{}, err
	}
	return settings, nil
}

func (s *Service) DiscordSettings(actor pkgstaff.Actor) (DiscordIntegrationSettings, error) {
	if err := actor.RequireCap(pkgstaff.CapManageConfig); err != nil {
		return DiscordIntegrationSettings{}, err
	}
	return s.store.GetDiscordIntegrationSettings()
}

// SetDiscordSettings saves staff-editable settings. Managed role IDs are
// maintained by the store, never by the caller.
func (s *Service) SetDiscordSettings(actor pkgstaff.Actor, settings DiscordIntegrationSettings) (DiscordIntegrationSettings, error) {
	if err := actor.RequireCap(pkgstaff.CapManageConfig); err != nil {
		return DiscordIntegrationSettings{}, err
	}
	settings.ManagedRoleIDs = nil
	if err := s.store.SetDiscordIntegrationSettings(settings); err != nil {
		return DiscordIntegrationSettings{}, err
	}
	return s.store.GetDiscordIntegrationSettings()
}
