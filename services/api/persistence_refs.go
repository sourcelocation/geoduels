package main

import (
	"github.com/jackc/pgx/v5"

	"geoduels/pkg/persistence"
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
	SocialSettings                 = persistence.SocialSettings
	SocialRepository               = persistence.SocialRepository
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
	ErrSocialNotFound             = persistence.ErrSocialNotFound
	ErrSocialLimit                = persistence.ErrSocialLimit
	ErrSocialBlocked              = persistence.ErrSocialBlocked
	ErrPreferenceRevisionConflict = persistence.ErrPreferenceRevisionConflict
)
