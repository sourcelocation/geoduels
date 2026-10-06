package maps

import (
	"io"

	"geoduels/pkg/contentfilter"
	"geoduels/pkg/contracts"
	pkgstaff "geoduels/pkg/staff"
)

// Store is the narrow persistence capability required by the maps use cases.
type Store interface {
	ListMaps(userID string, opts contracts.MapListOptions) ([]contracts.CustomMap, error)
	GetMap(userID, mapID string) (contracts.MapDetails, bool, error)
	GetMapUploadQuota(userID string) (contracts.MapUploadQuota, error)
	CreateCustomMap(userID, displayName, description, visibility, difficulty, thumbnailKey string, thumbnailVariant int, source io.Reader) (contracts.CustomMap, error)
	ImportOfficialMap(adminUserID string, input OfficialMapImportInput, source io.Reader) (contracts.CustomMap, error)
	ReplaceCustomMapLocations(userID, mapID string, source io.Reader) (contracts.CustomMap, error)
	UpdateCustomMap(userID, mapID string, update contracts.CustomMapUpdate) (contracts.CustomMap, error)
	PublishCustomMap(userID, mapID string) (contracts.CustomMap, error)
	SetMapFavorite(userID, mapID string, favorite bool) (contracts.CustomMap, error)
	SetMapOfficial(adminUserID, mapID string, official bool) (contracts.CustomMap, error)
	SetGameplayMapRole(adminUserID, mapID, role string) (contracts.CustomMap, error)
	ListMapComments(userID, mapID string) ([]contracts.MapComment, error)
	CreateMapComment(userID, mapID string, input contracts.MapCommentCreate) (contracts.MapComment, error)
	DeleteMapComment(userID, mapID, commentID string, moderator bool) error
	SetMapCommentLike(userID, mapID, commentID string, liked bool) (contracts.MapComment, error)
	ArchiveCustomMap(userID, mapID string, allowAnyMap bool) error
	// ReplaceMapLocations is the administrative revision path for official
	// map datasets, used by admins and the development bootstrap tool.
	ReplaceMapLocations(mapKey, displayName string, dataset []byte) (contracts.MapImportSummary, error)
	GetGameplayMapSettings() (contracts.GameplayMapSettings, error)
	ResolveGameplayMapID(mode contracts.MatchMode, ruleset contracts.GameRuleset, requestedMapID string) (string, error)
	MapCreatorAdminRepository
}

// MapCreatorAdminRepository is the administrative capability for overriding a
// creator's trust tier.
type MapCreatorAdminRepository interface {
	SetMapCreatorTierOverride(userID string, tier *int) (contracts.MapUploadQuota, error)
}

// Service owns maps orchestration. Persistence already provides atomic
// operations, so most use cases are honest pass-throughs; the content policy
// for user-supplied text runs here so it cannot be bypassed by another
// transport built on the same service.
type Service struct{ store Store }

func NewService(store Store) *Service { return &Service{store: store} }

func (s *Service) ListMaps(userID string, opts contracts.MapListOptions) ([]contracts.CustomMap, error) {
	return s.store.ListMaps(userID, opts)
}

func (s *Service) GetMap(userID, mapID string) (contracts.MapDetails, bool, error) {
	return s.store.GetMap(userID, mapID)
}

func (s *Service) GetMapUploadQuota(userID string) (contracts.MapUploadQuota, error) {
	return s.store.GetMapUploadQuota(userID)
}

// CreateCustomMap validates user-supplied map text before ingesting the
// dataset, mirroring the transport policy that previously lived in the API
// handlers.
func (s *Service) CreateCustomMap(userID, displayName, description, visibility, difficulty, thumbnailKey string, thumbnailVariant int, source io.Reader) (contracts.CustomMap, error) {
	if err := contentfilter.RejectAbusiveText(displayName, description); err != nil {
		return contracts.CustomMap{}, err
	}
	return s.store.CreateCustomMap(userID, displayName, description, visibility, difficulty, thumbnailKey, thumbnailVariant, source)
}

func (s *Service) ImportOfficialMap(actor pkgstaff.Actor, input OfficialMapImportInput, source io.Reader) (contracts.CustomMap, error) {
	if err := actor.RequireCap(pkgstaff.CapManageMaps); err != nil {
		return contracts.CustomMap{}, err
	}
	return s.store.ImportOfficialMap(actor.ID, input, source)
}

func (s *Service) ReplaceCustomMapLocations(userID, mapID string, source io.Reader) (contracts.CustomMap, error) {
	return s.store.ReplaceCustomMapLocations(userID, mapID, source)
}

// UpdateCustomMap validates user-supplied map text before applying the update.
func (s *Service) UpdateCustomMap(userID, mapID string, update contracts.CustomMapUpdate) (contracts.CustomMap, error) {
	if err := contentfilter.RejectAbusiveText(update.DisplayName, update.Description); err != nil {
		return contracts.CustomMap{}, err
	}
	return s.store.UpdateCustomMap(userID, mapID, update)
}

func (s *Service) PublishCustomMap(userID, mapID string) (contracts.CustomMap, error) {
	return s.store.PublishCustomMap(userID, mapID)
}

func (s *Service) SetMapFavorite(userID, mapID string, favorite bool) (contracts.CustomMap, error) {
	return s.store.SetMapFavorite(userID, mapID, favorite)
}

func (s *Service) SetMapOfficial(adminUserID, mapID string, official bool) (contracts.CustomMap, error) {
	return s.store.SetMapOfficial(adminUserID, mapID, official)
}

func (s *Service) SetGameplayMapRole(adminUserID, mapID, role string) (contracts.CustomMap, error) {
	return s.store.SetGameplayMapRole(adminUserID, mapID, role)
}

func (s *Service) ListMapComments(userID, mapID string) ([]contracts.MapComment, error) {
	return s.store.ListMapComments(userID, mapID)
}

// CreateMapComment validates user-supplied comment text before persisting it.
func (s *Service) CreateMapComment(userID, mapID string, input contracts.MapCommentCreate) (contracts.MapComment, error) {
	if err := contentfilter.RejectAbusiveText(input.Body); err != nil {
		return contracts.MapComment{}, err
	}
	return s.store.CreateMapComment(userID, mapID, input)
}

func (s *Service) DeleteMapComment(userID, mapID, commentID string, moderator bool) error {
	return s.store.DeleteMapComment(userID, mapID, commentID, moderator)
}

func (s *Service) SetMapCommentLike(userID, mapID, commentID string, liked bool) (contracts.MapComment, error) {
	return s.store.SetMapCommentLike(userID, mapID, commentID, liked)
}

func (s *Service) ArchiveCustomMap(userID, mapID string, allowAnyMap bool) error {
	return s.store.ArchiveCustomMap(userID, mapID, allowAnyMap)
}

func (s *Service) ReplaceMapLocations(actor pkgstaff.Actor, mapKey, displayName string, dataset []byte) (contracts.MapImportSummary, error) {
	if err := actor.RequireCap(pkgstaff.CapManageMaps); err != nil {
		return contracts.MapImportSummary{}, err
	}
	return s.store.ReplaceMapLocations(mapKey, displayName, dataset)
}

func (s *Service) GetGameplayMapSettings() (contracts.GameplayMapSettings, error) {
	return s.store.GetGameplayMapSettings()
}

func (s *Service) ResolveGameplayMapID(mode contracts.MatchMode, ruleset contracts.GameRuleset, requestedMapID string) (string, error) {
	return s.store.ResolveGameplayMapID(mode, ruleset, requestedMapID)
}

// SetMapCreatorTierOverride overrides a creator's trust tier; nil restores
// the automatic tier.
func (s *Service) SetMapCreatorTierOverride(actor pkgstaff.Actor, userID string, tier *int) (contracts.MapUploadQuota, error) {
	if err := actor.RequireCap(pkgstaff.CapManageMaps); err != nil {
		return contracts.MapUploadQuota{}, err
	}
	return s.store.SetMapCreatorTierOverride(userID, tier)
}
