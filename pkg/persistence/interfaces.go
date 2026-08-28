package persistence

import (
	"context"
	"encoding/json"
	"time"

	"geoduels/pkg/contracts"
)

type AccountRepository interface {
	UpsertProviderIdentity(provider, providerUserID, email, providerName, avatarURL, linkUserID string) (Identity, error)
	LinkProviderIdentity(provider, providerUserID, email, providerName, avatarURL, linkUserID string) (Identity, error)
	UpsertGoogleIdentity(googleSub, email, googleName, avatarURL, linkUserID string) (Identity, error)
	ProviderIdentityExists(provider, providerUserID string) (bool, error)
	GoogleIdentityExists(googleSub string) (bool, error)
	IsProviderIdentityBanned(provider, providerUserID string) (bool, string, error)
	UnlinkProviderIdentity(userID, provider string) (Identity, error)
	CreateGuestIdentity() (Identity, error)
	GetIdentity(sub string) (Identity, error)
	SetNickname(sub, displayName string) error
	SuggestNickname(sub, displayName string) (string, error)
	DeleteAccount(userID string) error
	DeleteGuestAccountsOlderThan(ttl time.Duration, limit int) (int, error)
}

type SessionRepository interface {
	CreateAuthSession(userID, refreshTokenHash string, expiresAt time.Time, params AuthSessionParams) (RefreshTokenRecord, error)
	GetAuthSessionByRefreshToken(hash string) (RefreshTokenRecord, bool, error)
	RotateAuthSession(sessionID, currentHash, nextHash string, expiresAt time.Time, usedAt time.Time) (RefreshTokenRecord, bool, error)
	RevokeAuthSession(sessionID string) error
	RevokeAuthSessionsForUser(userID string) error
}

type ProfileRepository interface {
	UpsertUser(userID, email, displayName string) error
	GetProfile(userID string) (Profile, error)
	GetPublicPlayerProfileByNickname(nickname string) (PublicPlayerProfile, error)
	UpdateSelectedBadge(userID, badgeID string) (Profile, error)
}

type PreferenceRepository interface {
	GetUserPreferences(ctx context.Context, userID string) (UserPreferences, error)
	UpdateUserPreferences(ctx context.Context, userID string, schemaVersion int, preferences json.RawMessage, expectedRevision int64) (UserPreferences, error)
}

type BadgeRepository interface {
	SyncLoginBadges(userID string) error
	AwardDiscordServerMemberByDiscordID(discordUserID string) (bool, error)
	ClaimPendingDiscordSync(now time.Time) (DiscordSyncOutboxItem, bool, error)
	MarkDiscordSyncProcessed(id int64) error
	MarkDiscordSyncFailed(id int64, nextAttemptAt time.Time, lastError string) error
	GetDiscordLinkedUser(discordUserID string) (DiscordLinkedUser, bool, error)
	CreateDonationRef(userID string) (string, error)
	AwardSupporterByDonationRef(ref string) (bool, error)
}

type LeaderboardRepository interface {
	ListLeaderboard(ctx context.Context, mode, seasonID string, limit, offset int) ([]LeaderboardEntry, error)
	GetLeaderboardOverview(ctx context.Context, userID, mode, seasonID string, limit int) (LeaderboardOverview, error)
}

type MatchRepository interface {
	FinalizeMatch(snap contracts.MatchSnapshot, ownerEpoch int64) (contracts.MatchSnapshot, error)
	RenewMatchSessionLeases(nodeID string, ownerEpoch int64, matchIDs []string, ttl time.Duration) error
	GetFinalMatchSnapshot(matchID string) ([]byte, bool, error)
	ListPlayerMatchHistory(userID string, limit int) ([]MatchHistorySummary, error)
	ListPlayerMatchHistoryPage(userID string, limit int, beforeEndedAt time.Time, beforeMatchID string, rankedOnly bool) (MatchHistoryPage, error)
	PlayerParticipatedInMatch(userID, matchID string) (bool, error)
}

type ModerationRepository interface {
	CreatePlayerReportSignal(params CreatePlayerReportSignalParams) (ModerationSignalCreated, error)
	ListSubjectModerationProfile(userID string) (ModerationSubjectProfile, error)
	ListModerationSignals(limit int) ([]ModerationSignalSummary, error)
	ListModerationLog(limit int) ([]ModerationAuditLogEntry, error)
	SetPlayerBan(userID, reason, actorUserID string, banned bool) error
	SetPlayerMute(userID, kind, reason, actorUserID string, until time.Time, muted bool) error
	BanPlayerForCheating(userID, reason, actorUserID string) (CheatingBanSummary, error)
	ClearReporterMute(userID string) error
	IssueEloRefundsForCheater(userID string, lookback time.Duration) (EloRefundSummary, error)
	AddSignupIPBan(ipAddress, reason, createdBy string) error
	RemoveSignupIPBan(ipAddress string) error
	ListSignupIPBans(limit int) ([]SignupIPBan, error)
	IsSignupIPBanned(ipAddress string) (bool, error)
	PreviewCommunityPardon(olderThan time.Duration) (CommunityPardonSummary, error)
	PardonBannedPlayers(olderThan time.Duration, actorUserID string) (CommunityPardonSummary, error)
}

type AdminRepository interface {
	SetUserAdmin(userID string, isAdmin bool) error
	SetUserModerator(userID string, isModerator bool) error
	SearchPlayers(query string, limit int) ([]AdminPlayerSummary, error)
	GetAdminPlayerDetail(userID string) (AdminPlayerDetail, error)
	ListUserRoles() ([]UserRoleGrant, error)
	GrantUserRole(userID, role, grantedBy, reason string) error
	RevokeUserRole(userID, role, revokedBy, reason string) error
	ListAdminGrantableBadges() []AdminBadgeDefinition
	GrantBadgeToUser(nickname, badgeID, actorUserID string) (contracts.PlayerBadge, bool, error)
}

type MapCreatorAdminRepository interface {
	SetMapCreatorTierOverride(userID string, tier *int) (contracts.MapUploadQuota, error)
}

type ContentRepository interface {
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

type SeasonRepository interface {
	GetRankedSeasonSettings() (RankedSeasonSettings, error)
	SetRankedSeasonResetRule(monthlyResetDay int) (RankedSeasonSettings, error)
	RunDueRankedSeasonReset(now time.Time) (RankedSeasonResetResult, bool, error)
	ReplaceMapLocations(mapKey, displayName string, dataset []byte) (MapImportSummary, error)
}

type GameplayMapRepository interface {
	GetGameplayMapSettings() (contracts.GameplayMapSettings, error)
	ResolveGameplayMapID(mode contracts.MatchMode, ruleset contracts.GameRuleset, requestedMapID string) (string, error)
}

type NotificationRepository interface {
	ListUserNotifications(userID string, limit int) ([]UserNotification, error)
	ListNotificationInbox(userID string, limit int, beforeID int64) ([]UserNotification, error)
	MarkUserNotificationRead(userID string, notificationID int64) error
	MarkAllUserNotificationsRead(userID string) error
	ClaimPendingNotification(notificationType string, now time.Time) (NotificationOutboxItem, bool, error)
	MarkNotificationSent(id int64) error
	MarkNotificationFailed(id int64, nextAttemptAt time.Time, lastError string) error
}

type RuntimeRepository interface {
	GetRuntimeMatch(ctx context.Context, matchID string) (RuntimeMatch, bool, error)
	RecordRuntimeMatch(ctx context.Context, matchID, state string, ownerEpoch int64, terminal bool) error
	UpsertMatchSession(ctx context.Context, params MatchSessionUpsert) error
	MatchSessionSourceParty(ctx context.Context, matchID string) (string, string, bool, error)
	ExpireStaleRuntimeMatches(ctx context.Context, prefix string, olderThan time.Duration) error
}

type StorageCleanupResult struct {
	ReplaysCompressed  int64
	ExpiredReplays     int64
	RuntimeMatches     int64
	MatchSessions      int64
	MatchPlans         int64
	ChatMessages       int64
	ChatConversations  int64
	AuthSessions       int64
	Parties            int64
	MapUploadEvents    int64
	MapDailyUsers      int64
	UserNotifications  int64
	NotificationOutbox int64
	DiscordSyncOutbox  int64
}

type StorageMaintenance interface {
	CleanupStorage(batchSize int) (StorageCleanupResult, error)
	ReconcileStaleMatchSessions(grace time.Duration, batchSize int) (int64, error)
}

type ChatRepository interface {
	RecordChatMessage(conversationID, scopeKind, scopeID string, message ChatMessage) error
	ListChatMessages(conversationID string, limit int) ([]ChatMessage, error)
	ListChatMessagesForUser(conversationID, userID string, limit int) ([]ChatMessage, error)
	ActivePartyChatTeam(partyID, userID string) (matchID, teamID string, ok bool, err error)
	ChatTeamForMatch(matchID, userID string) (teamID string, ok bool, err error)
	GetActiveChatRestriction(userID string) (ChatRestriction, bool, error)
}

type PartyRepository interface {
	ExpireOpenParties() error
	ListOpenPartyIDs() ([]string, error)
	CloseInactiveOpenParties(lobbyIDs []string, inactiveFor time.Duration) (int64, error)
	CreateParty(ownerUserID string, mode contracts.MatchMode, mapScope string, ttl time.Duration) (contracts.PartySnapshot, error)
	SetPartyMode(lobbyID string, mode contracts.MatchMode) error
	GetPartyByID(lobbyID string) (contracts.PartySnapshot, bool, error)
	GetPartyByInviteCode(inviteCode string) (contracts.PartySnapshot, bool, error)
	GetPartyByMatchID(matchID string) (contracts.PartySnapshot, bool, error)
	JoinParty(lobbyID, userID string) (contracts.PartySnapshot, error)
	LeaveParty(lobbyID, userID string) (contracts.PartySnapshot, error)
	SetPartyMemberTeam(lobbyID, userID, teamID string) (contracts.PartySnapshot, error)
	KickPartyMember(lobbyID, ownerUserID, targetUserID string) (contracts.PartySnapshot, error)
	TransferPartyOwner(lobbyID, ownerUserID, targetUserID string) (contracts.PartySnapshot, error)
	MarkPartyInMatch(lobbyID, matchID string) (contracts.PartySnapshot, error)
	ReopenEndedParties() (int64, error)
}

type PartyConfigRepository interface {
	SetPartyConfig(lobbyID string, config contracts.MatchConfig) (contracts.PartySnapshot, error)
}
