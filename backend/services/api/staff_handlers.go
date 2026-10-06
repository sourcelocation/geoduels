package main

import (
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"geoduels/internal/curation"
	"geoduels/internal/moderation"
	staffctx "geoduels/internal/staff"
	"geoduels/pkg/contracts"
	"geoduels/pkg/maintenance"
	pkgstaff "geoduels/pkg/staff"
)

const defaultLobbyChangelog = `
### Changes

- **Mobile fixed**
- **Accurate "Online players"**
- Intuitive reconnects
- Improved stability on bad networks
- Upgraded server hardware

### A personal message

I never imagined being able to play against real people in my own game, where you get matchmaked in under 10 seconds...

It's just surreal. And you guys made it possible.

Thank you everyone! And keep Dueling ⚔️

---

_Posted on March 19, 2026 by sourcelocation_
`

var defaultLobbyChangelogContent = LobbyChangelogContent{
	Eyebrow:  "Latest News",
	Title:    "GeoDuels v1.1",
	Markdown: strings.TrimSpace(defaultLobbyChangelog),
}

// staffActor authenticates the caller and rejects banned accounts. Capability
// checks live in the staff application service.
func (a *api) staffActor(r *http.Request) (pkgstaff.Actor, error) {
	identity, err := a.authenticatedIdentity(r)
	if err != nil {
		return pkgstaff.Actor{}, err
	}
	if identity.IsBanned {
		return pkgstaff.Actor{}, pkgstaff.ErrForbidden
	}
	return identity.StaffActor(), nil
}

// requireStaff authenticates and requires one of the given capabilities. Used
// only by map handlers whose service methods take no actor.
func (a *api) requireStaff(r *http.Request, caps ...pkgstaff.Capability) (pkgstaff.Actor, error) {
	actor, err := a.staffActor(r)
	if err != nil {
		return actor, err
	}
	for _, cap := range caps {
		if actor.Can(cap) {
			return actor, nil
		}
	}
	return actor, pkgstaff.ErrForbidden
}

func staffError(c echo.Context, err error) error {
	switch {
	case errors.Is(err, pkgstaff.ErrForbidden):
		return plainTextError(c, http.StatusForbidden, "forbidden")
	case errors.Is(err, staffctx.ErrNotFound), errors.Is(err, moderation.ErrNotFound), errors.Is(err, ErrNoRows):
		return plainTextError(c, http.StatusNotFound, "not found")
	case errors.Is(err, curation.ErrUnavailable):
		return plainTextError(c, http.StatusNotFound, err.Error())
	case errors.Is(err, curation.ErrInvalidTime):
		return plainTextError(c, http.StatusBadRequest, err.Error())
	case errors.Is(err, curation.ErrScheduleChanged):
		return plainTextError(c, http.StatusConflict, err.Error())
	default:
		return plainTextError(c, http.StatusInternalServerError, err.Error())
	}
}

func (a *api) adminBootstrap(c echo.Context) error {
	r := c.Request()
	claims, identity, err := a.authenticatedAccount(r)
	if err != nil {
		return plainTextError(c, http.StatusUnauthorized, "identity not found")
	}
	email := strings.ToLower(strings.TrimSpace(identity.Email))
	if email == "" {
		return plainTextError(c, http.StatusForbidden, "email required")
	}
	if _, ok := a.adminBootstrapEmails[email]; !ok {
		return plainTextError(c, http.StatusForbidden, "not allowlisted")
	}
	if !identity.IsAdmin {
		if err := a.staff.BootstrapAdmin(r.Context(), identity.Sub); err != nil {
			return plainTextError(c, http.StatusInternalServerError, "failed to promote account")
		}
		identity, err = a.accounts.GetIdentity(claims.Sub)
		if err != nil {
			return plainTextError(c, http.StatusUnauthorized, "identity not found")
		}
	}
	payload, err := a.issueAuthSessionPayload(identity, claims.SessionID)
	if err != nil {
		return plainTextError(c, http.StatusInternalServerError, "issue session failed")
	}
	return writeJSON(c, payload)
}

// ---- Players & review ----

func (a *api) adminPlayers(c echo.Context) error {
	actor, err := a.staffActor(c.Request())
	if err != nil {
		return staffError(c, err)
	}
	players, err := a.moderation.SearchSubjects(c.Request().Context(), actor, c.QueryParam("query"), 30)
	if err != nil {
		return staffError(c, err)
	}
	return writeJSON(c, map[string]any{"players": players})
}

func (a *api) adminPlayerDetail(c echo.Context) error {
	actor, err := a.staffActor(c.Request())
	if err != nil {
		return staffError(c, err)
	}
	detail, err := a.moderation.GetSubject(c.Request().Context(), actor, a.resolveEntityID("user", c.Param("id")))
	if err != nil {
		return staffError(c, err)
	}
	return writeJSON(c, detail)
}

func (a *api) moderatorSubject(c echo.Context) error {
	actor, err := a.staffActor(c.Request())
	if err != nil {
		return staffError(c, err)
	}
	profile, err := a.moderation.SubjectProfile(c.Request().Context(), actor, a.resolveEntityID("user", c.Param("id")))
	if err != nil {
		return staffError(c, err)
	}
	return writeJSON(c, profile)
}

func (a *api) moderatorSignals(c echo.Context) error {
	actor, err := a.staffActor(c.Request())
	if err != nil {
		return staffError(c, err)
	}
	signals, err := a.moderation.ListSignals(c.Request().Context(), actor, 100)
	if err != nil {
		return staffError(c, err)
	}
	return writeJSON(c, map[string]any{"signals": signals})
}

func (a *api) moderatorLog(c echo.Context) error {
	actor, err := a.staffActor(c.Request())
	if err != nil {
		return staffError(c, err)
	}
	entries, err := a.moderation.ListAudit(c.Request().Context(), actor, 100)
	if err != nil {
		return staffError(c, err)
	}
	return writeJSON(c, map[string]any{"log": entries})
}

// ---- Enforcement ----

func (a *api) adminBanPlayer(c echo.Context) error {
	actor, err := a.staffActor(c.Request())
	if err != nil {
		return staffError(c, err)
	}
	return a.banPlayerForCheating(c, actor, c.Param("id"))
}

func (a *api) banPlayerForCheating(c echo.Context, actor pkgstaff.Actor, rawUserID string) error {
	var req struct {
		Reason string `json:"reason"`
	}
	if err := decodeJSONBody(c.Request(), &req); err != nil && !errors.Is(err, io.EOF) {
		return plainTextError(c, http.StatusBadRequest, "invalid payload")
	}
	summary, err := a.moderation.BanCheater(c.Request().Context(), actor, a.resolveEntityID("user", rawUserID), req.Reason)
	if err != nil {
		return staffError(c, err)
	}
	return writeJSON(c, summary)
}

func (a *api) adminUnbanPlayer(c echo.Context) error {
	actor, err := a.staffActor(c.Request())
	if err != nil {
		return staffError(c, err)
	}
	if err := a.moderation.SetBan(c.Request().Context(), actor, a.resolveEntityID("user", c.Param("id")), "", false); err != nil {
		return staffError(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}

func (a *api) moderatorSubjectMute(c echo.Context) error {
	actor, err := a.staffActor(c.Request())
	if err != nil {
		return staffError(c, err)
	}
	var req struct {
		Reason        string `json:"reason"`
		DurationHours int    `json:"durationHours"`
	}
	if err := decodeJSONBody(c.Request(), &req); err != nil {
		return plainTextError(c, http.StatusBadRequest, "invalid payload")
	}
	if req.DurationHours <= 0 {
		req.DurationHours = 7 * 24
	}
	if err := a.moderation.SetMute(c.Request().Context(), actor, a.resolveEntityID("user", c.Param("id")), c.Param("kind"), req.Reason, time.Now().Add(time.Duration(req.DurationHours)*time.Hour), true); err != nil {
		return staffError(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}

func (a *api) moderatorSubjectUnmute(c echo.Context) error {
	actor, err := a.staffActor(c.Request())
	if err != nil {
		return staffError(c, err)
	}
	if err := a.moderation.SetMute(c.Request().Context(), actor, a.resolveEntityID("user", c.Param("id")), c.Param("kind"), "", time.Time{}, false); err != nil {
		return staffError(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}

func (a *api) adminClearReporterMute(c echo.Context) error {
	actor, err := a.staffActor(c.Request())
	if err != nil {
		return staffError(c, err)
	}
	if err := a.moderation.ClearReporterMute(c.Request().Context(), actor, a.resolveEntityID("user", c.Param("id"))); err != nil {
		return staffError(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}

func (a *api) adminCommunityPardonPreview(c echo.Context) error {
	actor, err := a.staffActor(c.Request())
	if err != nil {
		return staffError(c, err)
	}
	summary, err := a.moderation.PreviewPardon(c.Request().Context(), actor, 7*24*time.Hour)
	if err != nil {
		return staffError(c, err)
	}
	return writeJSON(c, summary)
}

func (a *api) adminCommunityPardon(c echo.Context) error {
	actor, err := a.staffActor(c.Request())
	if err != nil {
		return staffError(c, err)
	}
	var req struct {
		Confirm bool `json:"confirm"`
	}
	if err := decodeJSONBody(c.Request(), &req); err != nil || !req.Confirm {
		return plainTextError(c, http.StatusBadRequest, "explicit confirmation required")
	}
	summary, err := a.moderation.Pardon(c.Request().Context(), actor, 7*24*time.Hour)
	if err != nil {
		return staffError(c, err)
	}
	return writeJSON(c, summary)
}

// ---- Access ----

func (a *api) adminGrantRoleByParam(c echo.Context) error {
	actor, err := a.staffActor(c.Request())
	if err != nil {
		return staffError(c, err)
	}
	var req struct {
		Reason string `json:"reason"`
	}
	if err := decodeJSONBody(c.Request(), &req); err != nil && !errors.Is(err, io.EOF) {
		return plainTextError(c, http.StatusBadRequest, "invalid payload")
	}
	if err := a.staff.SetRole(c.Request().Context(), actor, a.resolveEntityID("user", c.Param("id")), strings.TrimSpace(c.Param("role")), req.Reason, true); err != nil {
		return staffError(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}

func (a *api) adminListRoles(c echo.Context) error {
	actor, err := a.staffActor(c.Request())
	if err != nil {
		return staffError(c, err)
	}
	roles, err := a.staff.ListRoleGrants(c.Request().Context(), actor)
	if err != nil {
		return staffError(c, err)
	}
	return writeJSON(c, map[string]any{"roles": roles})
}

func (a *api) adminGrantRole(c echo.Context) error {
	actor, err := a.staffActor(c.Request())
	if err != nil {
		return staffError(c, err)
	}
	var req struct {
		UserID string `json:"userId"`
		Role   string `json:"role"`
		Reason string `json:"reason"`
	}
	if err := decodeJSONBody(c.Request(), &req); err != nil {
		return plainTextError(c, http.StatusBadRequest, "invalid payload")
	}
	if err := a.staff.SetRole(c.Request().Context(), actor, a.resolveEntityID("user", req.UserID), req.Role, req.Reason, true); err != nil {
		return staffError(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}

func (a *api) adminRevokeRole(c echo.Context) error {
	actor, err := a.staffActor(c.Request())
	if err != nil {
		return staffError(c, err)
	}
	var req struct {
		Reason string `json:"reason"`
	}
	if err := decodeJSONBody(c.Request(), &req); err != nil && !errors.Is(err, io.EOF) {
		return plainTextError(c, http.StatusBadRequest, "invalid payload")
	}
	if err := a.staff.SetRole(c.Request().Context(), actor, a.resolveEntityID("user", c.Param("id")), strings.TrimSpace(c.Param("role")), req.Reason, false); err != nil {
		return staffError(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}

func (a *api) adminBadgeDefinitions(c echo.Context) error {
	actor, err := a.staffActor(c.Request())
	if err != nil {
		return staffError(c, err)
	}
	badges, err := a.badges.Catalog(actor)
	if err != nil {
		return staffError(c, err)
	}
	return writeJSON(c, map[string]any{"badges": badges})
}

func (a *api) adminGrantBadge(c echo.Context) error {
	actor, err := a.staffActor(c.Request())
	if err != nil {
		return staffError(c, err)
	}
	var req struct {
		Nickname string `json:"nickname"`
		BadgeID  string `json:"badgeId"`
	}
	if err := decodeJSONBody(c.Request(), &req); err != nil {
		return plainTextError(c, http.StatusBadRequest, "invalid payload")
	}
	if strings.TrimSpace(req.Nickname) == "" {
		return plainTextError(c, http.StatusBadRequest, "nickname required")
	}
	if strings.TrimSpace(req.BadgeID) == "" {
		return plainTextError(c, http.StatusBadRequest, "badge id required")
	}
	grant, err := a.badges.Grant(actor, strings.TrimSpace(req.Nickname), strings.TrimSpace(req.BadgeID))
	if err != nil {
		switch {
		case errors.Is(err, ErrBadgeUserNotFound):
			return plainTextError(c, http.StatusNotFound, err.Error())
		case errors.Is(err, ErrBadgeUnavailable), errors.Is(err, ErrBadgeNicknameRequired):
			return plainTextError(c, http.StatusBadRequest, err.Error())
		default:
			return staffError(c, err)
		}
	}
	return writeJSON(c, map[string]any{"badge": grant.Badge, "changed": grant.Changed})
}

// ---- IP signup blocks ----

func (a *api) adminListSignupIPBans(c echo.Context) error {
	actor, err := a.staffActor(c.Request())
	if err != nil {
		return staffError(c, err)
	}
	bans, err := a.moderation.ListIPBans(c.Request().Context(), actor, 100)
	if err != nil {
		return staffError(c, err)
	}
	return writeJSON(c, map[string]any{"bans": bans})
}

func (a *api) adminAddSignupIPBan(c echo.Context) error {
	actor, err := a.staffActor(c.Request())
	if err != nil {
		return staffError(c, err)
	}
	var req struct {
		IPAddress string `json:"ipAddress"`
		Reason    string `json:"reason"`
	}
	if err := decodeJSONBody(c.Request(), &req); err != nil {
		return plainTextError(c, http.StatusBadRequest, "invalid payload")
	}
	if err := a.moderation.AddIPBan(c.Request().Context(), actor, strings.TrimSpace(req.IPAddress), req.Reason); err != nil {
		return staffError(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}

func (a *api) adminRemoveSignupIPBan(c echo.Context) error {
	actor, err := a.staffActor(c.Request())
	if err != nil {
		return staffError(c, err)
	}
	ip, err := url.PathUnescape(strings.TrimSpace(c.Param("ip")))
	if err != nil {
		return plainTextError(c, http.StatusBadRequest, "invalid ip")
	}
	if err := a.moderation.RemoveIPBan(c.Request().Context(), actor, ip); err != nil {
		return staffError(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}

// ---- Operations ----

func (a *api) adminGetMaintenance(c echo.Context) error {
	actor, err := a.staffActor(c.Request())
	if err != nil {
		return staffError(c, err)
	}
	status, err := maintenance.StaffRead(c.Request().Context(), a.redis, actor)
	if err != nil {
		return staffError(c, err)
	}
	return writeJSON(c, status)
}

func (a *api) adminPutMaintenance(c echo.Context) error {
	actor, err := a.staffActor(c.Request())
	if err != nil {
		return staffError(c, err)
	}
	var status maintenance.Status
	if err := decodeJSONBody(c.Request(), &status); err != nil {
		return plainTextError(c, http.StatusBadRequest, "invalid payload")
	}
	saved, err := maintenance.Save(c.Request().Context(), a.redis, actor, status)
	if err != nil {
		return staffError(c, err)
	}
	return writeJSON(c, saved)
}

func (a *api) adminClearMaintenance(c echo.Context) error {
	actor, err := a.staffActor(c.Request())
	if err != nil {
		return staffError(c, err)
	}
	if err := maintenance.Clear(c.Request().Context(), a.redis, actor); err != nil {
		return staffError(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}

func (a *api) adminGetModerationSettings(c echo.Context) error {
	actor, err := a.staffActor(c.Request())
	if err != nil {
		return staffError(c, err)
	}
	settings, err := a.content.ModerationSettings(actor)
	if err != nil {
		return staffError(c, err)
	}
	return writeJSON(c, settings)
}

func (a *api) adminPutModerationSettings(c echo.Context) error {
	actor, err := a.staffActor(c.Request())
	if err != nil {
		return staffError(c, err)
	}
	var req ModerationSettings
	if err := decodeJSONBody(c.Request(), &req); err != nil {
		return plainTextError(c, http.StatusBadRequest, "invalid payload")
	}
	webhookURL, err := normalizeDiscordWebhookURL(req.DiscordWebhookURL)
	if err != nil {
		return plainTextError(c, http.StatusBadRequest, err.Error())
	}
	settings, err := a.content.SetModerationSettings(actor, ModerationSettings{DiscordWebhookURL: webhookURL})
	if err != nil {
		return staffError(c, err)
	}
	return writeJSON(c, settings)
}

func (a *api) adminGetDiscordIntegrationSettings(c echo.Context) error {
	actor, err := a.staffActor(c.Request())
	if err != nil {
		return staffError(c, err)
	}
	settings, err := a.content.DiscordSettings(actor)
	if err != nil {
		return staffError(c, err)
	}
	return writeJSON(c, settings)
}

func (a *api) adminPutDiscordIntegrationSettings(c echo.Context) error {
	actor, err := a.staffActor(c.Request())
	if err != nil {
		return staffError(c, err)
	}
	var settings DiscordIntegrationSettings
	if err := decodeJSONBody(c.Request(), &settings); err != nil {
		return plainTextError(c, http.StatusBadRequest, "invalid payload")
	}
	for label, value := range map[string]string{
		"guild id":         settings.GuildID,
		"joins channel id": settings.JoinsChannelID,
		"1k role id":       settings.Elo1000RoleID,
		"1.5k role id":     settings.Elo1500RoleID,
		"2k role id":       settings.Elo2000RoleID,
	} {
		if err := validateOptionalDiscordSnowflake(label, value); err != nil {
			return plainTextError(c, http.StatusBadRequest, err.Error())
		}
	}
	if settings.ReconcileIntervalMinutes < 1 || settings.ReconcileIntervalMinutes > 1440 {
		return plainTextError(c, http.StatusBadRequest, "reconcile interval must be between 1 and 1440 minutes")
	}
	saved, err := a.content.SetDiscordSettings(actor, settings)
	if err != nil {
		return staffError(c, err)
	}
	return writeJSON(c, saved)
}

func (a *api) adminGetRankedSeason(c echo.Context) error {
	actor, err := a.staffActor(c.Request())
	if err != nil {
		return staffError(c, err)
	}
	settings, err := a.seasons.StaffSettings(actor)
	if err != nil {
		return staffError(c, err)
	}
	return writeJSON(c, settings)
}

func (a *api) adminPutRankedSeasonResetRule(c echo.Context) error {
	actor, err := a.staffActor(c.Request())
	if err != nil {
		return staffError(c, err)
	}
	var req struct {
		MonthlyResetDay int `json:"monthlyResetDay"`
	}
	if err := decodeJSONBody(c.Request(), &req); err != nil && !errors.Is(err, io.EOF) {
		return plainTextError(c, http.StatusBadRequest, "invalid payload")
	}
	settings, err := a.seasons.SetResetRule(actor, req.MonthlyResetDay)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "reset day") {
			return plainTextError(c, http.StatusBadRequest, err.Error())
		}
		return staffError(c, err)
	}
	return writeJSON(c, settings)
}

// ---- Content ----

func (a *api) publicLobbyChangelog(c echo.Context) error {
	content, err := a.content.LobbyChangelog(defaultLobbyChangelogContent)
	if err != nil {
		return plainTextError(c, http.StatusInternalServerError, "changelog unavailable")
	}
	return writeJSON(c, content)
}

func (a *api) publicChangelogPosts(c echo.Context) error {
	posts, err := a.content.PublishedPosts()
	if err != nil {
		return plainTextError(c, http.StatusInternalServerError, "changelog unavailable")
	}
	return writeJSON(c, map[string]any{"posts": posts})
}

func (a *api) publicChangelogPost(c echo.Context) error {
	slug := strings.TrimSpace(c.Param("slug"))
	post, ok, err := a.content.PublishedPost(slug)
	if err != nil {
		return plainTextError(c, http.StatusInternalServerError, "changelog unavailable")
	}
	if !ok {
		return plainTextError(c, http.StatusNotFound, "404 page not found")
	}
	return writeJSON(c, post)
}

func (a *api) adminGetChangelog(c echo.Context) error {
	actor, err := a.staffActor(c.Request())
	if err != nil {
		return staffError(c, err)
	}
	posts, err := a.content.AllPosts(actor)
	if err != nil {
		if errors.Is(err, pkgstaff.ErrForbidden) {
			return staffError(c, err)
		}
		return plainTextError(c, http.StatusInternalServerError, "changelog unavailable")
	}
	return writeJSON(c, map[string]any{"posts": posts})
}

func (a *api) adminCreateChangelogPost(c echo.Context) error {
	actor, err := a.staffActor(c.Request())
	if err != nil {
		return staffError(c, err)
	}
	var req ChangelogPostInput
	if err := decodeJSONBody(c.Request(), &req); err != nil {
		return plainTextError(c, http.StatusBadRequest, "invalid payload")
	}
	input, err := normalizeChangelogPostInput(req)
	if err != nil {
		return plainTextError(c, http.StatusBadRequest, err.Error())
	}
	post, err := a.content.CreatePost(actor, input)
	if err != nil {
		return staffError(c, err)
	}
	return writeJSONStatus(c, http.StatusCreated, post)
}

func (a *api) adminUpdateChangelogPost(c echo.Context) error {
	actor, err := a.staffActor(c.Request())
	if err != nil {
		return staffError(c, err)
	}
	id, err := strconv.ParseInt(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil || id <= 0 {
		return plainTextError(c, http.StatusBadRequest, "invalid post id")
	}
	var req ChangelogPostInput
	if err := decodeJSONBody(c.Request(), &req); err != nil {
		return plainTextError(c, http.StatusBadRequest, "invalid payload")
	}
	input, err := normalizeChangelogPostInput(req)
	if err != nil {
		return plainTextError(c, http.StatusBadRequest, err.Error())
	}
	post, ok, err := a.content.UpdatePost(actor, id, input)
	if err != nil {
		return staffError(c, err)
	}
	if !ok {
		return plainTextError(c, http.StatusNotFound, "404 page not found")
	}
	return writeJSON(c, post)
}

func normalizeChangelogPostInput(req ChangelogPostInput) (ChangelogPostInput, error) {
	req.Title = strings.TrimSpace(req.Title)
	req.Markdown = strings.TrimSpace(req.Markdown)
	req.Slug = slugifyChangelogPost(req.Slug)
	if req.Slug == "" {
		req.Slug = slugifyChangelogPost(req.Title)
	}
	if req.Title == "" {
		return ChangelogPostInput{}, errors.New("title is required")
	}
	if req.Slug == "" {
		return ChangelogPostInput{}, errors.New("slug is required")
	}
	if len(req.Slug) > 120 {
		return ChangelogPostInput{}, errors.New("slug is too long")
	}
	if len(req.Title) > 160 {
		return ChangelogPostInput{}, errors.New("title is too long")
	}
	return req, nil
}

func slugifyChangelogPost(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	lastDash := false
	for _, r := range value {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

func normalizeDiscordWebhookURL(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", nil
	}
	if len(value) > 2000 {
		return "", errors.New("discord webhook url is too long")
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return "", errors.New("discord webhook url must be an https url")
	}
	host := strings.ToLower(parsed.Hostname())
	if host != "discord.com" && host != "discordapp.com" && host != "canary.discord.com" && host != "ptb.discord.com" {
		return "", errors.New("discord webhook url must be a Discord webhook")
	}
	if !strings.HasPrefix(parsed.EscapedPath(), "/api/webhooks/") {
		return "", errors.New("discord webhook url must be a Discord webhook")
	}
	return value, nil
}

func validateOptionalDiscordSnowflake(label, raw string) error {
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil
	}
	if len(value) < 17 || len(value) > 20 {
		return fmt.Errorf("%s must be a Discord ID", label)
	}
	for _, char := range value {
		if char < '0' || char > '9' {
			return fmt.Errorf("%s must be a Discord ID", label)
		}
	}
	return nil
}

// ---- Maps administration ----

func (a *api) adminSetMapCreatorTier(c echo.Context) error {
	actor, err := a.staffActor(c.Request())
	if err != nil {
		return staffError(c, err)
	}
	var req struct {
		Tier string `json:"tier"`
	}
	if err := decodeJSONBody(c.Request(), &req); err != nil {
		return plainTextError(c, http.StatusBadRequest, "invalid payload")
	}
	var tier *int
	switch strings.ToLower(strings.TrimSpace(req.Tier)) {
	case "auto":
	case "base":
		value := 0
		tier = &value
	case "trusted":
		value := 1
		tier = &value
	case "established":
		value := 2
		tier = &value
	default:
		return plainTextError(c, http.StatusBadRequest, "tier must be auto, base, trusted, or established")
	}
	quota, err := a.maps.SetMapCreatorTierOverride(actor, a.resolveEntityID("user", c.Param("id")), tier)
	if err != nil {
		return staffError(c, err)
	}
	return writeJSON(c, quota)
}

func (a *api) adminImportOfficialMap(c echo.Context) error {
	actor, err := a.staffActor(c.Request())
	if err != nil {
		return staffError(c, err)
	}
	r := c.Request()
	file, closeFile, err := mapUploadFile(c)
	if err != nil {
		return plainTextError(c, http.StatusBadRequest, err.Error())
	}
	defer closeFile()
	input := OfficialMapImportInput{
		MapKey:             r.FormValue("mapKey"),
		DisplayName:        r.FormValue("displayName"),
		Description:        r.FormValue("description"),
		Visibility:         r.FormValue("visibility"),
		Difficulty:         r.FormValue("difficulty"),
		ThumbnailKey:       r.FormValue("thumbnailKey"),
		ThumbnailVariant:   atoiDefault(r.FormValue("thumbnailVariant"), 1),
		OfficialRegionType: r.FormValue("officialRegionType"),
		OfficialRegionCode: r.FormValue("officialRegionCode"),
	}
	item, err := a.maps.ImportOfficialMap(actor, input, file)
	if err != nil {
		return staffError(c, err)
	}
	return writeJSON(c, item)
}

func (a *api) adminUploadMap(c echo.Context) error {
	mapKey := strings.TrimSpace(c.Param("mapKey"))
	if mapKey != contracts.MapKeyMoving && mapKey != contracts.MapKeyNMPZ {
		return plainTextError(c, http.StatusBadRequest, "unsupported map key")
	}
	return a.uploadMap(c, mapKey)
}

func (a *api) uploadMap(c echo.Context, mapKey string) error {
	actor, err := a.staffActor(c.Request())
	if err != nil {
		return staffError(c, err)
	}
	r := c.Request()
	if err := r.ParseMultipartForm(64 << 20); err != nil {
		return plainTextError(c, http.StatusBadRequest, "invalid multipart form")
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		return plainTextError(c, http.StatusBadRequest, "file is required")
	}
	defer file.Close()
	dataset, err := readUploadedFile(file, header)
	if err != nil {
		return plainTextError(c, http.StatusBadRequest, "failed to read file")
	}
	summary, err := a.maps.ReplaceMapLocations(actor, mapKey, mapKey, dataset)
	if err != nil {
		return staffError(c, err)
	}
	return writeJSON(c, summary)
}

func readUploadedFile(file multipart.File, _ *multipart.FileHeader) ([]byte, error) {
	return io.ReadAll(file)
}

// ---- Map of the Week ----

func (a *api) rescheduleMOTW(c echo.Context) error {
	actor, err := a.staffActor(c.Request())
	if err != nil {
		return staffError(c, err)
	}
	var input struct {
		ExpectedClosesAt time.Time `json:"expectedClosesAt"`
		ClosesAt         time.Time `json:"closesAt"`
	}
	if err := decodeJSONBody(c.Request(), &input); err != nil || input.ExpectedClosesAt.IsZero() || input.ClosesAt.IsZero() {
		return plainTextError(c, http.StatusBadRequest, "expectedClosesAt and closesAt must be valid timestamps with a timezone")
	}
	if err := a.curation.Reschedule(c.Request().Context(), actor, input.ExpectedClosesAt, input.ClosesAt); err != nil {
		return staffError(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}

func (a *api) motwNominations(c echo.Context) error {
	actor, err := a.staffActor(c.Request())
	if err != nil {
		return staffError(c, err)
	}
	page, _ := strconv.Atoi(c.QueryParam("page"))
	result, err := a.curation.ListNominations(c.Request().Context(), actor, page)
	if err != nil {
		return staffError(c, err)
	}
	return writeJSON(c, result)
}

func (a *api) nominateMOTW(c echo.Context) error {
	actor, err := a.staffActor(c.Request())
	if err != nil {
		return staffError(c, err)
	}
	if err := a.curation.NominateMap(c.Request().Context(), actor, a.resolveEntityID("map", c.Param("id"))); err != nil {
		return staffError(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}

func (a *api) likeMOTW(c echo.Context) error {
	actor, err := a.staffActor(c.Request())
	if err != nil {
		return staffError(c, err)
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return plainTextError(c, http.StatusBadRequest, "invalid nomination")
	}
	var input struct {
		Liked bool `json:"liked"`
	}
	if err := decodeJSONBody(c.Request(), &input); err != nil {
		return plainTextError(c, http.StatusBadRequest, "invalid payload")
	}
	if err := a.curation.LikeNomination(c.Request().Context(), actor, id, input.Liked); err != nil {
		return staffError(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}
