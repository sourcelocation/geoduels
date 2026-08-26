package main

import (
	"context"
	"errors"
	"net"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gorilla/mux"
	"github.com/redis/go-redis/v9"

	"geoduels/pkg/auth"
	"geoduels/pkg/contracts"
	"geoduels/pkg/coordinator"
	"geoduels/pkg/observability"
	"geoduels/pkg/persistence"
)

type api struct {
	matchCoordinator        string
	store                   persistence.Store
	coord                   *coordinator.Store
	redis                   *redis.Client
	httpClient              *http.Client
	googleVerifier          *auth.GoogleVerifier
	googleClientID          string
	googleSecret            string
	discordClientID         string
	discordSecret           string
	stripeMode              string
	stripeTestPaymentLink   string
	stripeLivePaymentLink   string
	stripeLegacyPaymentURL  string
	stripeTestWebhook       string
	stripeLiveWebhook       string
	stripeLegacyWebhook     string
	appAuthSecret           []byte
	ticketAuth              []byte
	internalSecret          string
	accessTokenTTL          time.Duration
	refreshTokenTTL         time.Duration
	refreshCookieName       string
	refreshCookieDomain     string
	refreshCookieSameSite   http.SameSite
	guestSignupIPLimit      int
	guestSignupIPWindow     time.Duration
	guestSignupDailyLimit   int
	guestSignupDailyWindow  time.Duration
	guestAccountTTL         time.Duration
	guestCleanupInterval    time.Duration
	guestCleanupBatchSize   int
	storageCleanupInterval  time.Duration
	storageCleanupBatchSize int
	staleMatchGrace         time.Duration
	turnstileSecret         string
	turnstileVerifyURL      string
	turnstileHostname       string
	guestTurnstileRequired  bool
	trustedProxyCIDRs       []*net.IPNet
	adminBootstrapEmails    map[string]struct{}
	metrics                 *observability.APIMetrics
	globalStatus            *globalStatusHub
	draining                atomic.Bool
}

func newAPI() (*api, error) {
	store, err := persistence.NewFromEnv()
	if err != nil {
		return nil, err
	}
	rdb, _, err := redisFromEnv()
	if err != nil {
		store.Close()
		return nil, err
	}
	googleClientID := strings.TrimSpace(os.Getenv("GOOGLE_CLIENT_ID"))
	googleSecret := strings.TrimSpace(os.Getenv("GOOGLE_CLIENT_SECRET"))
	discordClientID := strings.TrimSpace(os.Getenv("DISCORD_CLIENT_ID"))
	discordSecret := strings.TrimSpace(os.Getenv("DISCORD_CLIENT_SECRET"))
	stripeMode := strings.TrimSpace(strings.ToLower(os.Getenv("STRIPE_MODE")))
	stripeTestPaymentLink := strings.TrimSpace(os.Getenv("STRIPE_TEST_PAYMENT_LINK_URL"))
	stripeLivePaymentLink := strings.TrimSpace(os.Getenv("STRIPE_LIVE_PAYMENT_LINK_URL"))
	stripeLegacyPaymentURL := strings.TrimSpace(os.Getenv("STRIPE_PAYMENT_LINK_URL"))
	stripeTestWebhook := strings.TrimSpace(os.Getenv("STRIPE_TEST_WEBHOOK_SECRET"))
	stripeLiveWebhook := strings.TrimSpace(os.Getenv("STRIPE_LIVE_WEBHOOK_SECRET"))
	stripeLegacyWebhook := strings.TrimSpace(os.Getenv("STRIPE_WEBHOOK_SECRET"))
	var googleVerifier *auth.GoogleVerifier
	if googleClientID != "" && googleSecret != "" {
		googleVerifier, err = auth.NewGoogleVerifier(context.Background(), googleClientID, getenv("GOOGLE_ISSUER", ""))
		if err != nil {
			store.Close()
			return nil, err
		}
	}
	appAuthSecret, err := requiredSecret("APP_AUTH_SECRET", 32)
	if err != nil {
		store.Close()
		return nil, err
	}
	ticketAuth, err := requiredSecret("GAMEPLAY_TICKET_SECRET", 32)
	if err != nil {
		store.Close()
		return nil, err
	}
	internalSecret := strings.TrimSpace(os.Getenv("COORDINATOR_INTERNAL_SECRET"))
	if internalSecret == "" {
		store.Close()
		return nil, errors.New("COORDINATOR_INTERNAL_SECRET is required")
	}
	trustedProxyCIDRs, err := parseCIDRs(os.Getenv("TRUSTED_PROXY_CIDRS"))
	if err != nil {
		store.Close()
		return nil, err
	}
	guestTurnstileRequired := getenvBool("TURNSTILE_GUEST_REQUIRED", false)
	turnstileSecret := strings.TrimSpace(os.Getenv("TURNSTILE_SECRET_KEY"))
	if guestTurnstileRequired && turnstileSecret == "" {
		store.Close()
		return nil, errors.New("TURNSTILE_SECRET_KEY is required when TURNSTILE_GUEST_REQUIRED=true")
	}
	singleplayerTTL := getenvDuration("SINGLEPLAYER_SESSION_TTL", 24*time.Hour)
	if err := store.ExpireStaleRuntimeMatches(context.Background(), string(contracts.ModeSingleplayer), singleplayerTTL); err != nil {
		store.Close()
		return nil, err
	}
	if err := store.ExpireOpenParties(); err != nil {
		store.Close()
		return nil, err
	}
	instance := &api{
		matchCoordinator:        getenv("MATCH_COORDINATOR_URL", getenv("QUEUE_COORDINATOR_URL", "http://localhost:8090")),
		store:                   store,
		coord:                   coordinator.NewStore(rdb, getenvDuration("GAMEPLAY_NODE_TTL", 10*time.Second), 2*time.Hour, singleplayerTTL, 5*time.Second),
		redis:                   rdb,
		httpClient:              &http.Client{Timeout: 3 * time.Second},
		googleVerifier:          googleVerifier,
		googleClientID:          googleClientID,
		googleSecret:            googleSecret,
		discordClientID:         discordClientID,
		discordSecret:           discordSecret,
		stripeMode:              stripeMode,
		stripeTestPaymentLink:   stripeTestPaymentLink,
		stripeLivePaymentLink:   stripeLivePaymentLink,
		stripeLegacyPaymentURL:  stripeLegacyPaymentURL,
		stripeTestWebhook:       stripeTestWebhook,
		stripeLiveWebhook:       stripeLiveWebhook,
		stripeLegacyWebhook:     stripeLegacyWebhook,
		appAuthSecret:           appAuthSecret,
		ticketAuth:              ticketAuth,
		internalSecret:          internalSecret,
		accessTokenTTL:          getenvDuration("APP_ACCESS_TOKEN_TTL", 15*time.Minute),
		refreshTokenTTL:         getenvDuration("APP_REFRESH_TOKEN_TTL", 30*24*time.Hour),
		refreshCookieName:       getenv("APP_REFRESH_COOKIE_NAME", "geoduels_refresh"),
		refreshCookieDomain:     strings.TrimSpace(os.Getenv("APP_REFRESH_COOKIE_DOMAIN")),
		refreshCookieSameSite:   getenvSameSite("APP_REFRESH_COOKIE_SAMESITE", http.SameSiteLaxMode),
		guestSignupIPLimit:      getenvInt("GUEST_SIGNUP_IP_LIMIT", 5),
		guestSignupIPWindow:     getenvDuration("GUEST_SIGNUP_IP_WINDOW", 10*time.Minute),
		guestSignupDailyLimit:   getenvInt("GUEST_SIGNUP_IP_DAILY_LIMIT", 10),
		guestSignupDailyWindow:  getenvDuration("GUEST_SIGNUP_IP_DAILY_WINDOW", 24*time.Hour),
		guestAccountTTL:         getenvDuration("GUEST_ACCOUNT_TTL", 24*time.Hour),
		guestCleanupInterval:    getenvDuration("GUEST_ACCOUNT_CLEANUP_INTERVAL", time.Hour),
		guestCleanupBatchSize:   getenvInt("GUEST_ACCOUNT_CLEANUP_BATCH_SIZE", 1000),
		storageCleanupInterval:  getenvDuration("STORAGE_CLEANUP_INTERVAL", time.Minute),
		storageCleanupBatchSize: getenvInt("STORAGE_CLEANUP_BATCH_SIZE", 1000),
		staleMatchGrace:         getenvDuration("MATCH_SESSION_STALE_GRACE", 5*time.Minute),
		turnstileSecret:         turnstileSecret,
		turnstileVerifyURL:      getenv("TURNSTILE_VERIFY_URL", turnstileSiteverifyURL),
		turnstileHostname:       strings.TrimSpace(os.Getenv("TURNSTILE_EXPECTED_HOSTNAME")),
		guestTurnstileRequired:  guestTurnstileRequired,
		trustedProxyCIDRs:       trustedProxyCIDRs,
		adminBootstrapEmails:    parseEmailAllowlist(os.Getenv("ADMIN_BOOTSTRAP_EMAILS")),
		metrics:                 observability.NewAPIMetrics(),
	}
	instance.globalStatus = newGlobalStatusHub(instance)
	instance.globalStatus.start()
	return instance, nil
}

func routes(a *api) *mux.Router {
	r := mux.NewRouter()
	r.HandleFunc("/health", a.healthReady).Methods(http.MethodGet)
	r.HandleFunc("/health/live", a.healthLive).Methods(http.MethodGet)
	r.HandleFunc("/health/ready", a.healthReady).Methods(http.MethodGet)
	r.HandleFunc("/v1/auth/guest", a.guestLogin).Methods(http.MethodPost)
	r.HandleFunc("/v1/auth/google/start", a.googleOAuthStart).Methods(http.MethodPost)
	r.HandleFunc("/v1/auth/google/callback", a.googleOAuthCallback).Methods(http.MethodGet)
	r.HandleFunc("/v1/auth/discord/start", a.discordOAuthStart).Methods(http.MethodPost)
	r.HandleFunc("/v1/auth/discord/callback", a.discordOAuthCallback).Methods(http.MethodGet)
	r.HandleFunc("/v1/auth/session", a.session).Methods(http.MethodGet)
	r.HandleFunc("/v1/auth/refresh", a.refresh).Methods(http.MethodPost)
	r.HandleFunc("/v1/auth/logout", a.logout).Methods(http.MethodPost)
	r.HandleFunc("/v1/auth/logout-all", a.logoutAll).Methods(http.MethodPost)
	r.HandleFunc("/v1/status", a.publicGlobalStatus).Methods(http.MethodGet)
	r.HandleFunc("/v1/admin/bootstrap", a.adminBootstrap).Methods(http.MethodPost)
	r.HandleFunc("/v1/me", a.me).Methods(http.MethodGet)
	r.Handle("/v1/me/badge", a.active(a.updateSelectedBadge)).Methods(http.MethodPatch)
	r.Handle("/v1/me/nickname", a.active(a.updateNickname)).Methods(http.MethodPut, http.MethodPatch)
	r.HandleFunc("/v1/me/preferences", a.userPreferences).Methods(http.MethodGet)
	r.Handle("/v1/me/preferences", a.active(a.updateUserPreferences)).Methods(http.MethodPatch)
	r.HandleFunc("/v1/me", a.deleteAccount).Methods(http.MethodDelete)
	r.HandleFunc("/v1/me/auth-providers/{provider}", a.unlinkAuthProvider).Methods(http.MethodDelete)
	r.HandleFunc("/v1/me/notifications", a.userNotifications).Methods(http.MethodGet)
	r.HandleFunc("/v1/me/notifications/read-all", a.markAllUserNotificationsRead).Methods(http.MethodPost)
	r.HandleFunc("/v1/me/notifications/{id}/read", a.markUserNotificationRead).Methods(http.MethodPost)
	r.HandleFunc("/v1/me/social-summary", a.socialSummary).Methods(http.MethodGet)
	r.HandleFunc("/v1/me/social-settings", a.socialSettings).Methods(http.MethodGet)
	r.Handle("/v1/me/social-settings", a.active(a.socialSettings)).Methods(http.MethodPatch)
	r.HandleFunc("/v1/me/friends", a.friends).Methods(http.MethodGet)
	r.HandleFunc("/v1/me/friend-requests", a.friendRequests).Methods(http.MethodGet)
	r.HandleFunc("/v1/me/recent-players", a.recentPlayers).Methods(http.MethodGet)
	r.Handle("/v1/me/friend-code", a.active(a.createFriendCode)).Methods(http.MethodPost)
	r.HandleFunc("/v1/me/party-invitations", a.partyInvitations).Methods(http.MethodGet)
	r.HandleFunc("/v1/me/events/ws", a.userEventsWS).Methods(http.MethodGet)
	r.Handle("/v1/friend-requests", a.active(a.sendFriendRequest)).Methods(http.MethodPost)
	r.Handle("/v1/friend-requests/{id}/{action}", a.active(a.respondFriendRequest)).Methods(http.MethodPost)
	r.Handle("/v1/friends/{userId}", a.active(a.removeFriend)).Methods(http.MethodDelete)
	r.Handle("/v1/blocks/{userId}", a.active(a.userBlock)).Methods(http.MethodPost, http.MethodDelete)
	r.HandleFunc("/v1/friend-codes/{code}", a.resolveFriendCode).Methods(http.MethodGet)
	r.Handle("/v1/friend-codes/{code}/request", a.active(a.sendFriendCodeRequest)).Methods(http.MethodPost)
	r.Handle("/v1/parties/{id}/invitations", a.active(a.partyInvitations)).Methods(http.MethodPost)
	r.Handle("/v1/party-invitations/{id}/{action}", a.active(a.respondPartyInvitation)).Methods(http.MethodPost)
	r.Handle("/v1/party-invitations", a.active(a.createPartyAndInvite)).Methods(http.MethodPost)
	r.Handle("/v1/support/donate", a.active(a.createSupportDonation)).Methods(http.MethodPost)
	r.HandleFunc("/v1/integrations/stripe/webhook", a.stripeWebhook).Methods(http.MethodPost)
	r.HandleFunc("/v1/content/lobby-changelog", a.publicLobbyChangelog).Methods(http.MethodGet)
	r.HandleFunc("/v1/content/changelog", a.publicChangelogPosts).Methods(http.MethodGet)
	r.HandleFunc("/v1/content/changelog/{slug}", a.publicChangelogPost).Methods(http.MethodGet)

	r.HandleFunc("/v1/leaderboard", a.leaderboard).Methods(http.MethodGet)
	r.HandleFunc("/v1/players/{nickname}", a.publicPlayerProfile).Methods(http.MethodGet)
	r.HandleFunc("/v1/players/{nickname}/matches", a.publicPlayerMatches).Methods(http.MethodGet)
	r.HandleFunc("/v1/players/{nickname}/relationship", a.playerRelationship).Methods(http.MethodGet)
	r.HandleFunc("/v1/player-search", a.socialPlayerSearch).Methods(http.MethodGet)
	r.HandleFunc("/v1/matches/{id}", a.match).Methods(http.MethodGet)
	r.HandleFunc("/v1/matches/{id}/bootstrap", a.matchBootstrap).Methods(http.MethodGet)
	r.HandleFunc("/v1/matches/{id}/route", a.matchRoute).Methods(http.MethodGet)
	r.Handle("/v1/matches/{id}/session", a.active(a.matchSession)).Methods(http.MethodGet)
	r.Handle("/v1/matches/{id}/reports", a.active(a.createMatchReport)).Methods(http.MethodPost)
	r.HandleFunc("/v1/session/resumable", a.sessionResumable).Methods(http.MethodGet)
	r.Handle("/v1/sessions", a.active(a.startSession)).Methods(http.MethodPost)
	r.Handle("/v1/singleplayer/session", a.active(a.startSingleplayerSession)).Methods(http.MethodPost)
	r.HandleFunc("/v1/maps", a.listMaps).Methods(http.MethodGet)
	r.Handle("/v1/maps", a.active(a.createMap)).Methods(http.MethodPost)
	r.HandleFunc("/v1/maps/quota", a.mapUploadQuota).Methods(http.MethodGet)
	r.HandleFunc("/v1/maps/{id}", a.getMap).Methods(http.MethodGet)
	r.Handle("/v1/maps/{id}", a.active(a.updateMap)).Methods(http.MethodPatch)
	r.Handle("/v1/maps/{id}", a.active(a.archiveMap)).Methods(http.MethodDelete)
	r.Handle("/v1/maps/{id}/publish", a.active(a.publishMap)).Methods(http.MethodPost)
	r.HandleFunc("/v1/maps/{id}/official", a.setMapOfficial).Methods(http.MethodPost)
	r.HandleFunc("/v1/maps/{id}/official", a.unsetMapOfficial).Methods(http.MethodDelete)
	r.HandleFunc("/v1/maps/{id}/roles/{role}", a.setGameplayMapRole).Methods(http.MethodPost)
	r.Handle("/v1/maps/{id}/favorite", a.active(a.favoriteMap)).Methods(http.MethodPost)
	r.Handle("/v1/maps/{id}/favorite", a.active(a.unfavoriteMap)).Methods(http.MethodDelete)
	r.HandleFunc("/v1/maps/{id}/comments", a.listMapComments).Methods(http.MethodGet)
	r.Handle("/v1/maps/{id}/comments", a.active(a.createMapComment)).Methods(http.MethodPost)
	r.Handle("/v1/maps/{id}/comments/{commentId}", a.active(a.deleteMapComment)).Methods(http.MethodDelete)
	r.Handle("/v1/maps/{id}/comments/{commentId}/like", a.active(a.likeMapComment)).Methods(http.MethodPost)
	r.Handle("/v1/maps/{id}/comments/{commentId}/like", a.active(a.unlikeMapComment)).Methods(http.MethodDelete)
	r.Handle("/v1/maps/{id}/locations", a.active(a.replaceMapLocations)).Methods(http.MethodPut)
	r.HandleFunc("/v1/admin/players", a.adminPlayers).Methods(http.MethodGet)
	r.HandleFunc("/v1/admin/players/{id}", a.adminPlayerDetail).Methods(http.MethodGet)
	r.HandleFunc("/v1/admin/players/{id}/matches", a.adminPlayerMatches).Methods(http.MethodGet)
	r.HandleFunc("/v1/admin/players/{id}/ban", a.adminBanPlayer).Methods(http.MethodPost)
	r.HandleFunc("/v1/admin/players/{id}/unban", a.adminUnbanPlayer).Methods(http.MethodPost)
	r.HandleFunc("/v1/admin/players/{id}/report-mute", a.adminClearReporterMute).Methods(http.MethodDelete)
	r.HandleFunc("/v1/admin/moderation/community-pardon", a.adminCommunityPardonPreview).Methods(http.MethodGet)
	r.HandleFunc("/v1/admin/moderation/community-pardon", a.adminCommunityPardon).Methods(http.MethodPost)
	r.HandleFunc("/v1/admin/players/{id}/moderator", a.adminPromoteModerator).Methods(http.MethodPost)
	r.HandleFunc("/v1/admin/players/{id}/moderator", a.adminDemoteModerator).Methods(http.MethodDelete)
	r.HandleFunc("/v1/admin/players/{id}/map-tier", a.adminSetMapCreatorTier).Methods(http.MethodPut)
	r.HandleFunc("/v1/admin/roles", a.adminListRoles).Methods(http.MethodGet)
	r.HandleFunc("/v1/admin/roles", a.adminGrantRole).Methods(http.MethodPost)
	r.HandleFunc("/v1/admin/roles/{id}/{role}", a.adminRevokeRole).Methods(http.MethodDelete)
	r.HandleFunc("/v1/admin/badges", a.adminBadgeDefinitions).Methods(http.MethodGet)
	r.HandleFunc("/v1/admin/badges/grant", a.adminGrantBadge).Methods(http.MethodPost)
	r.HandleFunc("/v1/admin/matches/{id}/chat", a.adminMatchChat).Methods(http.MethodGet)
	r.HandleFunc("/v1/moderator/subjects/{userId}", a.moderatorSubject).Methods(http.MethodGet)
	r.HandleFunc("/v1/moderator/subjects/{userId}/cheating-ban", a.moderatorSubjectCheatingBan).Methods(http.MethodPost)
	r.HandleFunc("/v1/moderator/subjects/{userId}/unban", a.moderatorSubjectUnban).Methods(http.MethodPost)
	r.HandleFunc("/v1/moderator/subjects/{userId}/mutes/{kind}", a.moderatorSubjectMute).Methods(http.MethodPost)
	r.HandleFunc("/v1/moderator/subjects/{userId}/mutes/{kind}", a.moderatorSubjectUnmute).Methods(http.MethodDelete)
	r.HandleFunc("/v1/moderator/signals", a.moderatorSignals).Methods(http.MethodGet)
	r.HandleFunc("/v1/moderator/log", a.moderatorLog).Methods(http.MethodGet)
	r.HandleFunc("/v1/admin/ip-signup-bans", a.adminListSignupIPBans).Methods(http.MethodGet)
	r.HandleFunc("/v1/admin/ip-signup-bans", a.adminAddSignupIPBan).Methods(http.MethodPost)
	r.HandleFunc("/v1/admin/ip-signup-bans/{ip}", a.adminRemoveSignupIPBan).Methods(http.MethodDelete)
	r.HandleFunc("/v1/admin/maintenance", a.adminGetMaintenance).Methods(http.MethodGet)
	r.HandleFunc("/v1/admin/maintenance", a.adminPutMaintenance).Methods(http.MethodPut)
	r.HandleFunc("/v1/admin/maintenance", a.adminClearMaintenance).Methods(http.MethodDelete)
	r.HandleFunc("/v1/admin/moderation/settings", a.adminGetModerationSettings).Methods(http.MethodGet)
	r.HandleFunc("/v1/admin/moderation/settings", a.adminPutModerationSettings).Methods(http.MethodPut)
	r.HandleFunc("/v1/admin/integrations/discord", a.adminGetDiscordIntegrationSettings).Methods(http.MethodGet)
	r.HandleFunc("/v1/admin/integrations/discord", a.adminPutDiscordIntegrationSettings).Methods(http.MethodPut)
	r.HandleFunc("/v1/admin/seasons", a.adminGetRankedSeason).Methods(http.MethodGet)
	r.HandleFunc("/v1/admin/seasons/reset-rule", a.adminPutRankedSeasonResetRule).Methods(http.MethodPut)
	r.HandleFunc("/v1/admin/changelog", a.adminGetChangelog).Methods(http.MethodGet)
	r.HandleFunc("/v1/admin/changelog", a.adminCreateChangelogPost).Methods(http.MethodPost)
	r.HandleFunc("/v1/admin/changelog/{id}", a.adminUpdateChangelogPost).Methods(http.MethodPut)
	r.HandleFunc("/v1/admin/maps/official/import", a.adminImportOfficialMap).Methods(http.MethodPost)
	r.HandleFunc("/v1/admin/maps/current/upload", a.adminUploadCurrentMap).Methods(http.MethodPost)
	r.HandleFunc("/v1/admin/maps/{mapKey}/upload", a.adminUploadMap).Methods(http.MethodPost)
	r.Handle("/metrics", observability.Handler(a.metrics.Registry)).Methods(http.MethodGet)
	return r
}

func parseEmailAllowlist(raw string) map[string]struct{} {
	out := map[string]struct{}{}
	for _, part := range strings.Split(raw, ",") {
		email := strings.ToLower(strings.TrimSpace(part))
		if email == "" {
			continue
		}
		out[email] = struct{}{}
	}
	return out
}
