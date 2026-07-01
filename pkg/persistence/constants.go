package persistence

const (
	modeDuel                        = "duel"
	defaultSeasonID                 = "s2"
	moderationProjectionAdvisoryKey = int64(0x67646d6f646572)
	moderationActiveRiskThreshold   = 1.5
	IdentityProviderGoogle          = "google"
	IdentityProviderDiscord         = "discord"
	badgeCodeDiscordMember          = int16(1)
	badgeCodeGeoDuelsTeam           = int16(2)
	badgeCodeDiscordServerMember    = int16(3)
	badgeCodeSupporter              = int16(4)
	badgeCodeSpeedrunner            = int16(5)
	badgeCodeElo1000                = int16(6)
	badgeCodeElo1500                = int16(7)
	badgeCodeElo2000                = int16(8)
	badgeCodeSeasonRank             = int16(10)
)
const (
	DiscordSyncActionSync         = "sync"
	DiscordSyncActionCleanupRoles = "cleanup_roles"
)
