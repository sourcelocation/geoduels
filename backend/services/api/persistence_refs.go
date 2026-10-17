package main

import (
	"github.com/jackc/pgx/v5"

	"geoduels/internal/accounts"
	"geoduels/internal/badges"
	"geoduels/internal/content"
	"geoduels/internal/maps"
	socialdomain "geoduels/internal/social"
)

type (
	Identity                   = accounts.Identity
	ChangelogPostInput         = content.ChangelogPostInput
	LobbyChangelogContent      = content.LobbyChangelogContent
	ModerationSettings         = content.ModerationSettings
	DiscordIntegrationSettings = content.DiscordIntegrationSettings
	MapCreatorAdminRepository  = maps.MapCreatorAdminRepository
	OfficialMapImportInput     = maps.OfficialMapImportInput
)

const (
	IdentityProviderGoogle  = accounts.IdentityProviderGoogle
	IdentityProviderDiscord = accounts.IdentityProviderDiscord
)

var (
	ErrNoRows                = pgx.ErrNoRows
	ErrNicknameTaken         = accounts.ErrNicknameTaken
	ErrOAuthEmailConflict    = accounts.ErrOAuthEmailConflict
	ErrBadgeNicknameRequired = badges.ErrBadgeNicknameRequired
	ErrBadgeUnavailable      = badges.ErrBadgeUnavailable
	ErrBadgeUserNotFound     = badges.ErrBadgeUserNotFound
	ErrSocialNotFound        = socialdomain.ErrNotFound
	ErrSocialLimit           = socialdomain.ErrLimit
	ErrSocialBlocked         = socialdomain.ErrBlocked
)
