package maps

import (
	"io"

	"geoduels/pkg/contentfilter"
	"geoduels/pkg/gameplay"
	"geoduels/pkg/matchkind"
	pkgstaff "geoduels/pkg/staff"
)

// Store is the narrow persistence capability required by the maps use cases.
type Store interface {
	ListMaps(userID string, opts ListOptions) ([]Map, error)
	GetMap(userID, mapID string) (Details, bool, error)
	GetMapUploadQuota(userID string) (UploadQuota, error)
	CreateCustomMap(userID, displayName, description, visibility, difficulty, thumbnailKey string, thumbnailVariant int, source io.Reader) (Map, error)
	ImportOfficialMap(adminUserID string, input OfficialMapImportInput, source io.Reader) (Map, error)
	ReplaceCustomMapLocations(userID, mapID string, source io.Reader) (Map, error)
	UpdateCustomMap(userID, mapID string, update MapEdit) (Map, error)
	PublishCustomMap(userID, mapID string) (Map, error)
	SetMapFavorite(userID, mapID string, favorite bool) (Map, error)
	SetMapOfficial(adminUserID, mapID string, official bool) (Map, error)
	SetGameplayMapRole(adminUserID, mapID, role string) (Map, error)
	ListMapComments(userID, mapID string) ([]Comment, error)
	CreateMapComment(userID, mapID string, input NewComment) (Comment, error)
	DeleteMapComment(userID, mapID, commentID string, moderator bool) error
	SetMapCommentLike(userID, mapID, commentID string, liked bool) (Comment, error)
	ArchiveCustomMap(userID, mapID string, allowAnyMap bool) error
	// ReplaceMapLocations is the administrative revision path for official
	// map datasets, used by admins and the development bootstrap tool.
	ReplaceMapLocations(mapKey, displayName string, dataset []byte) (ImportSummary, error)
	GetGameplayMapSettings() (GameplaySettings, error)
	ResolveGameplayMapID(mode matchkind.Mode, ruleset gameplay.Ruleset, requestedMapID string) (string, error)
	MapCreatorAdminRepository
}

// MapCreatorAdminRepository is the administrative capability for overriding a
// creator's trust tier.
type MapCreatorAdminRepository interface {
	SetMapCreatorTierOverride(userID string, tier *int) (UploadQuota, error)
}

// Service owns maps orchestration. Persistence already provides atomic
// operations, so most use cases are honest pass-throughs; the content policy
// for user-supplied text runs here so it cannot be bypassed by another
// transport built on the same service.
type Service struct{ store Store }

func NewService(store Store) *Service { return &Service{store: store} }

func (s *Service) ListMaps(userID string, opts ListOptions) ([]Map, error) {
	return s.store.ListMaps(userID, opts)
}

func (s *Service) GetMap(userID, mapID string) (Details, bool, error) {
	return s.store.GetMap(userID, mapID)
}

func (s *Service) GetMapUploadQuota(userID string) (UploadQuota, error) {
	return s.store.GetMapUploadQuota(userID)
}

// CreateCustomMap validates user-supplied map text before ingesting the
// dataset, mirroring the transport policy that previously lived in the API
// handlers.
func (s *Service) CreateCustomMap(userID, displayName, description, visibility, difficulty, thumbnailKey string, thumbnailVariant int, source io.Reader) (Map, error) {
	if err := contentfilter.RejectAbusiveText(displayName, description); err != nil {
		return Map{}, err
	}
	return s.store.CreateCustomMap(userID, displayName, description, visibility, difficulty, thumbnailKey, thumbnailVariant, source)
}

func (s *Service) ImportOfficialMap(actor pkgstaff.Actor, input OfficialMapImportInput, source io.Reader) (Map, error) {
	if err := actor.RequireCap(pkgstaff.CapManageMaps); err != nil {
		return Map{}, err
	}
	return s.store.ImportOfficialMap(actor.ID, input, source)
}

func (s *Service) ReplaceCustomMapLocations(userID, mapID string, source io.Reader) (Map, error) {
	return s.store.ReplaceCustomMapLocations(userID, mapID, source)
}

// UpdateCustomMap validates user-supplied map text before applying the update.
func (s *Service) UpdateCustomMap(userID, mapID string, update MapEdit) (Map, error) {
	if err := contentfilter.RejectAbusiveText(update.DisplayName, update.Description); err != nil {
		return Map{}, err
	}
	return s.store.UpdateCustomMap(userID, mapID, update)
}

func (s *Service) PublishCustomMap(userID, mapID string) (Map, error) {
	return s.store.PublishCustomMap(userID, mapID)
}

func (s *Service) SetMapFavorite(userID, mapID string, favorite bool) (Map, error) {
	return s.store.SetMapFavorite(userID, mapID, favorite)
}

func (s *Service) SetMapOfficial(adminUserID, mapID string, official bool) (Map, error) {
	return s.store.SetMapOfficial(adminUserID, mapID, official)
}

func (s *Service) SetGameplayMapRole(adminUserID, mapID, role string) (Map, error) {
	return s.store.SetGameplayMapRole(adminUserID, mapID, role)
}

func (s *Service) ListMapComments(userID, mapID string) ([]Comment, error) {
	return s.store.ListMapComments(userID, mapID)
}

// CreateMapComment validates user-supplied comment text before persisting it.
func (s *Service) CreateMapComment(userID, mapID string, input NewComment) (Comment, error) {
	if err := contentfilter.RejectAbusiveText(input.Body); err != nil {
		return Comment{}, err
	}
	return s.store.CreateMapComment(userID, mapID, input)
}

func (s *Service) DeleteMapComment(userID, mapID, commentID string, moderator bool) error {
	return s.store.DeleteMapComment(userID, mapID, commentID, moderator)
}

func (s *Service) SetMapCommentLike(userID, mapID, commentID string, liked bool) (Comment, error) {
	return s.store.SetMapCommentLike(userID, mapID, commentID, liked)
}

func (s *Service) ArchiveCustomMap(userID, mapID string, allowAnyMap bool) error {
	return s.store.ArchiveCustomMap(userID, mapID, allowAnyMap)
}

func (s *Service) ReplaceMapLocations(actor pkgstaff.Actor, mapKey, displayName string, dataset []byte) (ImportSummary, error) {
	if err := actor.RequireCap(pkgstaff.CapManageMaps); err != nil {
		return ImportSummary{}, err
	}
	return s.store.ReplaceMapLocations(mapKey, displayName, dataset)
}

func (s *Service) GetGameplayMapSettings() (GameplaySettings, error) {
	return s.store.GetGameplayMapSettings()
}

func (s *Service) ResolveGameplayMapID(mode matchkind.Mode, ruleset gameplay.Ruleset, requestedMapID string) (string, error) {
	return s.store.ResolveGameplayMapID(mode, ruleset, requestedMapID)
}

// SetMapCreatorTierOverride overrides a creator's trust tier; nil restores
// the automatic tier.
func (s *Service) SetMapCreatorTierOverride(actor pkgstaff.Actor, userID string, tier *int) (UploadQuota, error) {
	if err := actor.RequireCap(pkgstaff.CapManageMaps); err != nil {
		return UploadQuota{}, err
	}
	return s.store.SetMapCreatorTierOverride(userID, tier)
}
