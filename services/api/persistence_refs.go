package main

import (
	"github.com/jackc/pgx/v5"

	"geoduels/pkg/persistence"
	socialdomain "geoduels/pkg/social"
)

// Handler files depend on these persistence DTOs, errors, and narrow
// repository interfaces through the aliases below so the transport layer
// stays free of direct persistence package references.

type (
	Identity                       = persistence.Identity
	RefreshTokenRecord             = persistence.RefreshTokenRecord
	AuthSessionParams              = persistence.AuthSessionParams
	ChangelogPostInput             = persistence.ChangelogPostInput
	LobbyChangelogContent          = persistence.LobbyChangelogContent
	ModerationSettings             = persistence.ModerationSettings
	DiscordIntegrationSettings     = persistence.DiscordIntegrationSettings
	AdminPlayerSummary             = persistence.AdminPlayerSummary
	UserNotification               = persistence.UserNotification
	MapCatalog                     = persistence.MapCatalog
	MapCreatorAdminRepository      = persistence.MapCreatorAdminRepository
	OfficialMapImportInput         = persistence.OfficialMapImportInput
	CreatePlayerReportSignalParams = persistence.CreatePlayerReportSignalParams
)

const (
	IdentityProviderGoogle  = persistence.IdentityProviderGoogle
	IdentityProviderDiscord = persistence.IdentityProviderDiscord
)

var (
	// ErrNoRows is re-exported so handler files never import pgx directly.
	ErrNoRows                     = pgx.ErrNoRows
	ErrNicknameTaken              = persistence.ErrNicknameTaken
	ErrOAuthEmailConflict         = persistence.ErrOAuthEmailConflict
	ErrBadgeNicknameRequired      = persistence.ErrBadgeNicknameRequired
	ErrBadgeUnavailable           = persistence.ErrBadgeUnavailable
	ErrBadgeUserNotFound          = persistence.ErrBadgeUserNotFound
	ErrSocialNotFound             = socialdomain.ErrNotFound
	ErrSocialLimit                = socialdomain.ErrLimit
	ErrSocialBlocked              = socialdomain.ErrBlocked
	ErrPreferenceRevisionConflict = persistence.ErrPreferenceRevisionConflict
)
