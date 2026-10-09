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

	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"

	"geoduels/internal/accounts"
	"geoduels/internal/authsession"
	"geoduels/internal/badges"
	"geoduels/internal/content"
	"geoduels/internal/curation"
	"geoduels/internal/jobs"
	"geoduels/internal/leaderboard"
	"geoduels/internal/maps"
	"geoduels/internal/matches"
	"geoduels/internal/moderation"
	"geoduels/internal/notifications"
	"geoduels/internal/parties"
	preferencesdomain "geoduels/internal/preferences"
	"geoduels/internal/profiles"
	"geoduels/internal/seasons"
	socialdomain "geoduels/internal/social"
	staffctx "geoduels/internal/staff"
	"geoduels/pkg/auth"
	"geoduels/pkg/coordinator"
	"geoduels/pkg/observability"
	"geoduels/pkg/persistence"
)

type api struct {
	staff                  *staffctx.Service
	moderation             *moderation.Service
	curation               *curation.Service
	matchCoordinator       string
	db                     *persistence.DB
	accounts               *accounts.Service
	sessions               authsession.Store
	profiles               profiles.Store
	badges                 *badges.Service
	matchStore             matches.Store
	content                *content.Service
	seasons                *seasons.Service
	gameplayMaps           maps.Store
	parties                *parties.Service
	social                 *socialdomain.Service
	maps                   *maps.Service
	mapsStore              *maps.PGStore
	preferences            *preferencesdomain.Service
	leaderboardService     *leaderboard.Service
	notificationService    *notifications.Service
	authSessionService     *authsession.Service
	coord                  *coordinator.Store
	redis                  *redis.Client
	httpClient             *http.Client
	googleVerifier         *auth.GoogleVerifier
	googleClientID         string
	googleSecret           string
	discordClientID        string
	discordSecret          string
	stripeMode             string
	stripeTestPaymentLink  string
	stripeLivePaymentLink  string
	stripeLegacyPaymentURL string
	stripeTestWebhook      string
	stripeLiveWebhook      string
	stripeLegacyWebhook    string
	appAuthSecret          []byte
	ticketAuth             []byte
	internalSecret         string
	accessTokenTTL         time.Duration
	refreshTokenTTL        time.Duration
	refreshCookieName      string
	refreshCookieDomain    string
	refreshCookieSameSite  http.SameSite
	guestSignupIPLimit     int
	guestSignupIPWindow    time.Duration
	guestSignupDailyLimit  int
	guestSignupDailyWindow time.Duration
	turnstileSecret        string
	turnstileVerifyURL     string
	turnstileHostname      string
	guestTurnstileRequired bool
	trustedProxyCIDRs      []*net.IPNet
	adminBootstrapEmails   map[string]struct{}
	metrics                *observability.APIMetrics
	globalStatus           *globalStatusHub
	live                   *liveHub
	lastSeen               lastSeenWriter
	draining               atomic.Bool
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
	pool := store.Pool()
	mapsStore := maps.NewPGStore(pool)
	matchStore := matches.NewPGStore(pool, nil)
	partyStore := parties.NewPGStore(pool, mapsStore)
	partyService := parties.NewService(partyStore)
	if err := partyService.ExpireOpenParties(); err != nil {
		store.Close()
		return nil, err
	}
	socialStore := socialdomain.NewPGStore(pool)
	jobsClient, err := jobs.NewClient(pool, nil, nil)
	if err != nil {
		store.Close()
		return nil, err
	}
	contentStore := content.NewPGStore(pool, jobsClient)
	seasonStore := seasons.NewPGStore(pool)
	badgeStore := badges.NewPGStore(pool, jobsClient)
	accountsStore := accounts.NewPGStore(pool, jobsClient)
	accountsService := accounts.NewService(accountsStore)
	mapsService := maps.NewService(mapsStore)
	instance := &api{
		staff:                  staffctx.NewService(staffctx.NewPGStore(pool)),
		moderation:             moderation.NewService(moderation.NewPGStore(pool, jobsClient), moderation.NewRiskEngineFromEnv()),
		curation:               curation.NewService(curation.NewPGStore(pool)),
		matchCoordinator:       getenv("MATCH_COORDINATOR_URL", getenv("QUEUE_COORDINATOR_URL", "http://localhost:8090")),
		db:                     store,
		accounts:               accountsService,
		sessions:               authsession.NewPGStore(pool),
		profiles:               profiles.NewPGStore(pool),
		badges:                 badges.NewService(badgeStore),
		matchStore:             matchStore,
		content:                content.NewService(contentStore),
		seasons:                seasons.NewService(seasonStore),
		gameplayMaps:           mapsStore,
		parties:                partyService,
		social:                 socialdomain.NewService(socialStore),
		maps:                   mapsService,
		mapsStore:              mapsStore,
		lastSeen:               socialStore,
		preferences:            preferencesdomain.NewService(preferencesdomain.NewPGStore(pool)),
		leaderboardService:     leaderboard.NewService(leaderboard.NewPGStore(pool)),
		notificationService:    notifications.NewService(notifications.NewPGStore(pool)),
		authSessionService:     authsession.NewService(authsession.NewPGStore(pool)),
		coord:                  coordinator.NewStore(rdb, getenvDuration("GAMEPLAY_NODE_TTL", 10*time.Second), 2*time.Hour, singleplayerTTL, 5*time.Second),
		redis:                  rdb,
		httpClient:             &http.Client{Timeout: 3 * time.Second},
		googleVerifier:         googleVerifier,
		googleClientID:         googleClientID,
		googleSecret:           googleSecret,
		discordClientID:        discordClientID,
		discordSecret:          discordSecret,
		stripeMode:             stripeMode,
		stripeTestPaymentLink:  stripeTestPaymentLink,
		stripeLivePaymentLink:  stripeLivePaymentLink,
		stripeLegacyPaymentURL: stripeLegacyPaymentURL,
		stripeTestWebhook:      stripeTestWebhook,
		stripeLiveWebhook:      stripeLiveWebhook,
		stripeLegacyWebhook:    stripeLegacyWebhook,
		appAuthSecret:          appAuthSecret,
		ticketAuth:             ticketAuth,
		internalSecret:         internalSecret,
		accessTokenTTL:         getenvDuration("APP_ACCESS_TOKEN_TTL", 15*time.Minute),
		refreshTokenTTL:        getenvDuration("APP_REFRESH_TOKEN_TTL", 30*24*time.Hour),
		refreshCookieName:      getenv("APP_REFRESH_COOKIE_NAME", "geoduels_refresh"),
		refreshCookieDomain:    strings.TrimSpace(os.Getenv("APP_REFRESH_COOKIE_DOMAIN")),
		refreshCookieSameSite:  getenvSameSite("APP_REFRESH_COOKIE_SAMESITE", http.SameSiteLaxMode),
		guestSignupIPLimit:     getenvInt("GUEST_SIGNUP_IP_LIMIT", 5),
		guestSignupIPWindow:    getenvDuration("GUEST_SIGNUP_IP_WINDOW", 10*time.Minute),
		guestSignupDailyLimit:  getenvInt("GUEST_SIGNUP_IP_DAILY_LIMIT", 10),
		guestSignupDailyWindow: getenvDuration("GUEST_SIGNUP_IP_DAILY_WINDOW", 24*time.Hour),
		turnstileSecret:        turnstileSecret,
		turnstileVerifyURL:     getenv("TURNSTILE_VERIFY_URL", turnstileSiteverifyURL),
		turnstileHostname:      strings.TrimSpace(os.Getenv("TURNSTILE_EXPECTED_HOSTNAME")),
		guestTurnstileRequired: guestTurnstileRequired,
		trustedProxyCIDRs:      trustedProxyCIDRs,
		adminBootstrapEmails:   parseEmailAllowlist(os.Getenv("ADMIN_BOOTSTRAP_EMAILS")),
		metrics:                observability.NewAPIMetrics(),
	}
	instance.globalStatus = newGlobalStatusHub(instance)
	instance.globalStatus.start()
	instance.live = newLiveHub(instance)
	instance.live.start()
	return instance, nil
}

func routes(a *api) *echo.Echo {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true
	e.HTTPErrorHandler = func(err error, c echo.Context) {
		if c.Response().Committed {
			return
		}
		code := http.StatusInternalServerError
		if he, ok := err.(*echo.HTTPError); ok {
			code = he.Code
		}
		// gorilla/mux wrote empty bodies for unrouted paths and methods.
		if code == http.StatusNotFound || code == http.StatusMethodNotAllowed {
			_ = c.NoContent(code)
			return
		}
		e.DefaultHTTPErrorHandler(err, c)
	}
	e.Use(corsMiddleware)
	if a.metrics != nil {
		e.Use(a.metrics.EchoMiddleware)
	}

	e.GET("/health", a.healthReady)
	e.GET("/health/live", a.healthLive)
	e.GET("/health/ready", a.healthReady)
	e.POST("/api/auth/guest", a.guestLogin)
	e.POST("/api/auth/google/start", a.googleOAuthStart)
	e.GET("/api/auth/google/callback", a.googleOAuthCallback)
	e.POST("/api/auth/discord/start", a.discordOAuthStart)
	e.GET("/api/auth/discord/callback", a.discordOAuthCallback)
	e.GET("/api/bootstrap", a.bootstrap)
	e.POST("/api/auth/refresh", a.refresh)
	e.POST("/api/auth/logout", a.logout)
	e.POST("/api/auth/logout-all", a.logoutAll)
	e.GET("/api/status", a.publicGlobalStatus)
	e.PATCH("/api/me/badge", a.updateSelectedBadge, a.active)
	e.GET("/api/v1/staff/players/:id/warnings", a.staffWarnings)
	e.POST("/api/v1/staff/players/:id/warnings", a.staffWarn, a.active)
	e.POST("/api/v1/staff/players/:id/warnings/:warningId/withdraw", a.staffWithdrawWarning, a.active)
	e.GET("/api/v1/me/warnings", a.userWarnings)
	e.POST("/api/v1/me/warnings/:warningId/acknowledge", a.acknowledgeWarning)
	e.PUT("/api/me/nickname", a.updateNickname, a.active)
	e.PATCH("/api/me/nickname", a.updateNickname, a.active)
	e.PATCH("/api/me/preferences", a.updateUserPreferences, a.active)
	e.DELETE("/api/me", a.deleteAccount)
	e.DELETE("/api/me/auth-providers/:provider", a.unlinkAuthProvider)
	e.GET("/api/me/live", a.userLive)
	e.GET("/api/me/notifications", a.userNotifications)
	e.POST("/api/me/notifications/read-all", a.markAllUserNotificationsRead)
	e.POST("/api/me/notifications/:id/read", a.markUserNotificationRead)
	e.GET("/api/me/social-settings", a.socialSettings)
	e.PATCH("/api/me/social-settings", a.socialSettings, a.active)
	e.GET("/api/me/friends-page", a.friendsPage)
	e.POST("/api/me/friend-code", a.createFriendCode, a.active)
	e.GET("/api/me/party-invitations", a.partyInvitations)
	e.POST("/api/friend-requests", a.sendFriendRequest, a.active)
	e.POST("/api/friend-requests/:id/:action", a.respondFriendRequest, a.active)
	e.DELETE("/api/friends/:userId", a.removeFriend, a.active)
	e.POST("/api/blocks/:userId", a.userBlock, a.active)
	e.DELETE("/api/blocks/:userId", a.userBlock, a.active)
	e.GET("/api/friend-codes/:code", a.resolveFriendCode)
	e.POST("/api/friend-codes/:code/request", a.sendFriendCodeRequest, a.active)
	e.POST("/api/parties/:id/invitations", a.partyInvitations, a.active)
	e.POST("/api/party-invitations/:id/:action", a.respondPartyInvitation, a.active)
	e.POST("/api/party-invitations", a.createPartyAndInvite, a.active)
	e.POST("/api/support/donate", a.createSupportDonation, a.active)
	e.POST("/api/integrations/stripe/webhook", a.stripeWebhook)
	e.GET("/api/content/lobby-changelog", a.publicLobbyChangelog)
	e.GET("/api/content/changelog", a.publicChangelogPosts)
	e.GET("/api/content/changelog/:slug", a.publicChangelogPost)

	e.GET("/api/leaderboard", a.leaderboard)
	e.GET("/api/players/:nickname", a.publicPlayerProfile)
	e.GET("/api/players/:nickname/matches", a.publicPlayerMatches)
	e.GET("/api/players/:nickname/relationship", a.playerRelationship)
	e.GET("/api/player-search", a.socialPlayerSearch)
	e.GET("/api/matches/:id", a.match)
	e.GET("/api/matches/:id/bootstrap", a.matchBootstrap)
	e.GET("/api/matches/:id/route", a.matchRoute)
	e.GET("/api/matches/:id/session", a.matchSession, a.active)
	e.POST("/api/matches/:id/reports", a.createMatchReport, a.active)
	e.POST("/api/sessions", a.startSession, a.active)
	e.POST("/api/singleplayer/session", a.startSingleplayerSession, a.active)
	e.GET("/api/maps", a.listMaps)
	e.POST("/api/maps", a.createMap, a.active)
	e.GET("/api/maps/quota", a.mapUploadQuota)
	e.GET("/api/maps/:id", a.getMap)
	e.PATCH("/api/maps/:id", a.updateMap, a.active)
	e.DELETE("/api/maps/:id", a.archiveMap, a.active)
	e.POST("/api/maps/:id/publish", a.publishMap, a.active)
	e.POST("/api/maps/:id/official", a.setMapOfficial)
	e.DELETE("/api/maps/:id/official", a.unsetMapOfficial)
	e.POST("/api/maps/:id/roles/:role", a.setGameplayMapRole)
	e.POST("/api/maps/:id/favorite", a.favoriteMap, a.active)
	e.DELETE("/api/maps/:id/favorite", a.unfavoriteMap, a.active)
	e.GET("/api/maps/:id/comments", a.listMapComments)
	e.POST("/api/maps/:id/comments", a.createMapComment, a.active)
	e.DELETE("/api/maps/:id/comments/:commentId", a.deleteMapComment, a.active)
	e.POST("/api/maps/:id/comments/:commentId/like", a.likeMapComment, a.active)
	e.DELETE("/api/maps/:id/comments/:commentId/like", a.unlikeMapComment, a.active)
	e.PUT("/api/maps/:id/locations", a.replaceMapLocations, a.active)
	staff := e.Group("/api/staff")
	staff.POST("/bootstrap", a.adminBootstrap)
	staff.GET("/players", a.adminPlayers)
	staff.GET("/players/:id", a.adminPlayerDetail)
	staff.POST("/players/:id/ban", a.adminBanPlayer)
	staff.POST("/players/:id/unban", a.adminUnbanPlayer)
	staff.POST("/players/:id/mutes/:kind", a.moderatorSubjectMute)
	staff.DELETE("/players/:id/mutes/:kind", a.moderatorSubjectUnmute)
	staff.DELETE("/players/:id/report-mute", a.adminClearReporterMute)
	staff.PUT("/players/:id/map-tier", a.adminSetMapCreatorTier)
	staff.POST("/players/:id/roles/:role", a.adminGrantRoleByParam)
	staff.DELETE("/players/:id/roles/:role", a.adminRevokeRole)
	staff.GET("/players/:id/review", a.moderatorSubject)
	staff.GET("/reports", a.moderatorSignals)
	staff.GET("/audit", a.moderatorLog)
	staff.GET("/roles", a.adminListRoles)
	staff.POST("/roles", a.adminGrantRole)
	staff.DELETE("/roles/:id/:role", a.adminRevokeRole)
	staff.GET("/badges", a.adminBadgeDefinitions)
	staff.POST("/badges/grant", a.adminGrantBadge)
	staff.GET("/community-pardon", a.adminCommunityPardonPreview)
	staff.POST("/community-pardon", a.adminCommunityPardon)
	staff.GET("/ip-bans", a.adminListSignupIPBans)
	staff.POST("/ip-bans", a.adminAddSignupIPBan)
	staff.DELETE("/ip-bans/:ip", a.adminRemoveSignupIPBan)
	staff.GET("/maintenance", a.adminGetMaintenance)
	staff.PUT("/maintenance", a.adminPutMaintenance)
	staff.DELETE("/maintenance", a.adminClearMaintenance)
	staff.GET("/settings/moderation", a.adminGetModerationSettings)
	staff.PUT("/settings/moderation", a.adminPutModerationSettings)
	staff.GET("/settings/discord", a.adminGetDiscordIntegrationSettings)
	staff.PUT("/settings/discord", a.adminPutDiscordIntegrationSettings)
	staff.GET("/seasons", a.adminGetRankedSeason)
	staff.PUT("/seasons/reset-rule", a.adminPutRankedSeasonResetRule)
	staff.GET("/changelog", a.adminGetChangelog)
	staff.POST("/changelog", a.adminCreateChangelogPost)
	staff.PUT("/changelog/:id", a.adminUpdateChangelogPost)
	staff.POST("/maps/official/import", a.adminImportOfficialMap)
	staff.POST("/maps/:mapKey/upload", a.adminUploadMap)
	staff.GET("/motw", a.motwNominations)
	staff.POST("/motw/maps/:id", a.nominateMOTW)
	staff.PUT("/motw/nominations/:id/like", a.likeMOTW)
	staff.PUT("/motw/schedule", a.rescheduleMOTW)
	if a.metrics != nil {
		e.GET("/metrics", echo.WrapHandler(observability.Handler(a.metrics.Registry)))
	}
	return e
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
