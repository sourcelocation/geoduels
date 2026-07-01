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
	turnstileSecret         string
	turnstileVerifyURL      string
	turnstileHostname       string
	guestTurnstileRequired  bool
	trustedProxyCIDRs       []*net.IPNet
	adminBootstrapEmails    map[string]struct{}
	metrics                 *observability.APIMetrics
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
	if err := store.ExpireStaleRuntimeMatches(string(contracts.ModeSingleplayer), singleplayerTTL); err != nil {
		store.Close()
		return nil, err
	}
	if err := store.ExpireOpenParties(); err != nil {
		store.Close()
		return nil, err
	}
	return &api{
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
		turnstileSecret:         turnstileSecret,
		turnstileVerifyURL:      getenv("TURNSTILE_VERIFY_URL", turnstileSiteverifyURL),
		turnstileHostname:       strings.TrimSpace(os.Getenv("TURNSTILE_EXPECTED_HOSTNAME")),
		guestTurnstileRequired:  guestTurnstileRequired,
		trustedProxyCIDRs:       trustedProxyCIDRs,
		adminBootstrapEmails:    parseEmailAllowlist(os.Getenv("ADMIN_BOOTSTRAP_EMAILS")),
		metrics:                 observability.NewAPIMetrics(),
	}, nil
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
	r.HandleFunc("/v1/admin/bootstrap", a.adminBootstrap).Methods(http.MethodPost)
	r.HandleFunc("/v1/me", a.me).Methods(http.MethodGet)
	r.HandleFunc("/v1/me/badge", a.updateSelectedBadge).Methods(http.MethodPatch)
	r.HandleFunc("/v1/me/nickname", a.updateNickname).Methods(http.MethodPut, http.MethodPatch)
	r.HandleFunc("/v1/me", a.deleteAccount).Methods(http.MethodDelete)
	r.HandleFunc("/v1/me/auth-providers/{provider}", a.unlinkAuthProvider).Methods(http.MethodDelete)
	r.HandleFunc("/v1/me/notifications", a.userNotifications).Methods(http.MethodGet)
	r.HandleFunc("/v1/me/notifications/{id}/read", a.markUserNotificationRead).Methods(http.MethodPost)
	r.HandleFunc("/v1/support/donate", a.createSupportDonation).Methods(http.MethodPost)
	r.HandleFunc("/v1/integrations/stripe/webhook", a.stripeWebhook).Methods(http.MethodPost)
	r.HandleFunc("/v1/content/lobby-changelog", a.publicLobbyChangelog).Methods(http.MethodGet)
	r.HandleFunc("/v1/content/changelog", a.publicChangelogPosts).Methods(http.MethodGet)
	r.HandleFunc("/v1/content/changelog/{slug}", a.publicChangelogPost).Methods(http.MethodGet)

	r.HandleFunc("/v1/leaderboard", a.leaderboard).Methods(http.MethodGet)
	r.HandleFunc("/v1/players/{nickname}", a.publicPlayerProfile).Methods(http.MethodGet)
	r.HandleFunc("/v1/players/{nickname}/matches", a.publicPlayerMatches).Methods(http.MethodGet)
	r.HandleFunc("/v1/matches/{id}", a.match).Methods(http.MethodGet)
	r.HandleFunc("/v1/matches/{id}/bootstrap", a.matchBootstrap).Methods(http.MethodGet)
	r.HandleFunc("/v1/matches/{id}/route", a.matchRoute).Methods(http.MethodGet)
	r.HandleFunc("/v1/matches/{id}/session", a.matchSession).Methods(http.MethodGet)
	r.HandleFunc("/v1/matches/{id}/reports", a.createMatchReport).Methods(http.MethodPost)
	r.HandleFunc("/v1/session/resumable", a.sessionResumable).Methods(http.MethodGet)
	r.HandleFunc("/v1/sessions", a.startSession).Methods(http.MethodPost)
	r.HandleFunc("/v1/singleplayer/session", a.startSingleplayerSession).Methods(http.MethodPost)
	r.HandleFunc("/v1/maps", a.listMaps).Methods(http.MethodGet)
	r.HandleFunc("/v1/maps", a.createMap).Methods(http.MethodPost)
	r.HandleFunc("/v1/maps/quota", a.mapUploadQuota).Methods(http.MethodGet)
	r.HandleFunc("/v1/maps/{id}", a.getMap).Methods(http.MethodGet)
	r.HandleFunc("/v1/maps/{id}", a.updateMap).Methods(http.MethodPatch)
	r.HandleFunc("/v1/maps/{id}", a.archiveMap).Methods(http.MethodDelete)
	r.HandleFunc("/v1/maps/{id}/publish", a.publishMap).Methods(http.MethodPost)
	r.HandleFunc("/v1/maps/{id}/official", a.setMapOfficial).Methods(http.MethodPost)
	r.HandleFunc("/v1/maps/{id}/official", a.unsetMapOfficial).Methods(http.MethodDelete)
	r.HandleFunc("/v1/maps/{id}/roles/{role}", a.setGameplayMapRole).Methods(http.MethodPost)
	r.HandleFunc("/v1/maps/{id}/favorite", a.favoriteMap).Methods(http.MethodPost)
	r.HandleFunc("/v1/maps/{id}/favorite", a.unfavoriteMap).Methods(http.MethodDelete)
	r.HandleFunc("/v1/maps/{id}/comments", a.listMapComments).Methods(http.MethodGet)
	r.HandleFunc("/v1/maps/{id}/comments", a.createMapComment).Methods(http.MethodPost)
	r.HandleFunc("/v1/maps/{id}/comments/{commentId}", a.deleteMapComment).Methods(http.MethodDelete)
	r.HandleFunc("/v1/maps/{id}/comments/{commentId}/like", a.likeMapComment).Methods(http.MethodPost)
	r.HandleFunc("/v1/maps/{id}/comments/{commentId}/like", a.unlikeMapComment).Methods(http.MethodDelete)
	r.HandleFunc("/v1/maps/{id}/locations", a.replaceMapLocations).Methods(http.MethodPut)
	r.HandleFunc("/v1/admin/players", a.adminPlayers).Methods(http.MethodGet)
	r.HandleFunc("/v1/admin/players/{id}", a.adminPlayerDetail).Methods(http.MethodGet)
	r.HandleFunc("/v1/admin/players/{id}/matches", a.adminPlayerMatches).Methods(http.MethodGet)
	r.HandleFunc("/v1/admin/players/{id}/ban", a.adminBanPlayer).Methods(http.MethodPost)
	r.HandleFunc("/v1/admin/players/{id}/unban", a.adminUnbanPlayer).Methods(http.MethodPost)
	r.HandleFunc("/v1/admin/players/{id}/report-mute", a.adminClearReporterMute).Methods(http.MethodDelete)
	r.HandleFunc("/v1/admin/players/{id}/moderator", a.adminPromoteModerator).Methods(http.MethodPost)
	r.HandleFunc("/v1/admin/players/{id}/moderator", a.adminDemoteModerator).Methods(http.MethodDelete)
	r.HandleFunc("/v1/admin/players/{id}/map-tier", a.adminSetMapCreatorTier).Methods(http.MethodPut)
	r.HandleFunc("/v1/admin/roles", a.adminListRoles).Methods(http.MethodGet)
	r.HandleFunc("/v1/admin/roles", a.adminGrantRole).Methods(http.MethodPost)
	r.HandleFunc("/v1/admin/roles/{id}/{role}", a.adminRevokeRole).Methods(http.MethodDelete)
	r.HandleFunc("/v1/admin/matches/{id}/chat", a.adminMatchChat).Methods(http.MethodGet)
	r.HandleFunc("/v1/moderator/tasks", a.moderatorTasks).Methods(http.MethodGet)
	r.HandleFunc("/v1/moderator/tasks/{id}/claim", a.moderatorTaskClaim).Methods(http.MethodPost)
	r.HandleFunc("/v1/moderator/tasks/{id}/release", a.moderatorTaskRelease).Methods(http.MethodPost)
	r.HandleFunc("/v1/moderator/incidents/{id}", a.moderatorIncident).Methods(http.MethodGet)
	r.HandleFunc("/v1/moderator/incidents/{id}/verdicts", a.moderatorIncidentVerdict).Methods(http.MethodPost)
	r.HandleFunc("/v1/moderator/subjects/{userId}", a.moderatorSubject).Methods(http.MethodGet)
	r.HandleFunc("/v1/moderator/subjects/{userId}/cheating-ban", a.moderatorSubjectCheatingBan).Methods(http.MethodPost)
	r.HandleFunc("/v1/moderator/subjects/{userId}/unban", a.moderatorSubjectUnban).Methods(http.MethodPost)
	r.HandleFunc("/v1/moderator/signals", a.moderatorSignals).Methods(http.MethodGet)
	r.HandleFunc("/v1/moderator/enforcement/actions", a.moderatorEnforcementActions).Methods(http.MethodGet)
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
