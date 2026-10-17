package main

import (
	"encoding/json"
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

	"geoduels/internal/audit"
	"geoduels/internal/curation"
	"geoduels/internal/maps"
	"geoduels/internal/moderation"
	staffctx "geoduels/internal/staff"
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

// adminPlayerSummary is a player as the staff tools show them.
type adminPlayerSummary struct {
	UserID            string              `json:"userId"`
	Email             string              `json:"email,omitempty"`
	DisplayName       string              `json:"displayName"`
	AvatarURL         string              `json:"avatarUrl,omitempty"`
	MMR               int                 `json:"mmr"`
	GamesPlayed       int                 `json:"gamesPlayed"`
	Wins              int                 `json:"wins"`
	RankedGamesPlayed int                 `json:"rankedGamesPlayed"`
	TrackedMatches    int                 `json:"trackedMatches"`
	RankedMatches     int                 `json:"rankedMatches"`
	DuelMatches       int                 `json:"duelMatches"`
	SingleplayerRuns  int                 `json:"singleplayerRuns"`
	Losses            int                 `json:"losses"`
	IsGuest           bool                `json:"isGuest"`
	IsAdmin           bool                `json:"isAdmin"`
	IsModerator       bool                `json:"isModerator"`
	IsBanned          bool                `json:"isBanned"`
	BanReason         string              `json:"banReason,omitempty"`
	BannedAt          time.Time           `json:"bannedAt,omitempty"`
	BanExpiresAt      time.Time           `json:"banExpiresAt,omitempty"`
	ChatMuteReason    string              `json:"chatMuteReason,omitempty"`
	ChatMutedAt       time.Time           `json:"chatMutedAt,omitempty"`
	ChatMutedUntil    time.Time           `json:"chatMutedUntil,omitempty"`
	ReportMuteReason  string              `json:"reportMuteReason,omitempty"`
	ReportMutedAt     time.Time           `json:"reportMutedAt,omitempty"`
	LastIPAddress     string              `json:"lastIpAddress,omitempty"`
	ReportMutedUntil  time.Time           `json:"reportMutedUntil,omitempty"`
	Identities        []adminUserIdentity `json:"identities,omitempty"`
}

type adminUserIdentity struct {
	Provider       string    `json:"provider"`
	ProviderUserID string    `json:"providerUserId"`
	Email          string    `json:"email,omitempty"`
	ProviderName   string    `json:"providerName,omitempty"`
	LastSeenAt     time.Time `json:"lastSeenAt,omitempty"`
	DeletedAt      time.Time `json:"deletedAt,omitempty"`
}

func newAdminPlayerSummary(p moderation.PlayerSummary) adminPlayerSummary {
	out := adminPlayerSummary{
		UserID: p.UserID, Email: p.Email, DisplayName: p.DisplayName, AvatarURL: p.AvatarURL, MMR: p.MMR,
		GamesPlayed: p.GamesPlayed, Wins: p.Wins, RankedGamesPlayed: p.RankedGamesPlayed, TrackedMatches: p.TrackedMatches,
		RankedMatches: p.RankedMatches, DuelMatches: p.DuelMatches, SingleplayerRuns: p.SingleplayerRuns, Losses: p.Losses,
		IsGuest: p.IsGuest, IsAdmin: p.IsAdmin, IsModerator: p.IsModerator, IsBanned: p.IsBanned, BanReason: p.BanReason,
		BannedAt: p.BannedAt, BanExpiresAt: p.BanExpiresAt, ChatMuteReason: p.ChatMuteReason, ChatMutedAt: p.ChatMutedAt,
		ChatMutedUntil: p.ChatMutedUntil, ReportMuteReason: p.ReportMuteReason, ReportMutedAt: p.ReportMutedAt,
		LastIPAddress: p.LastIPAddress, ReportMutedUntil: p.ReportMutedUntil,
	}
	if p.Identities != nil {
		out.Identities = make([]adminUserIdentity, len(p.Identities))
		for i, id := range p.Identities {
			out.Identities[i] = adminUserIdentity{
				Provider: id.Provider, ProviderUserID: id.ProviderUserID, Email: id.Email, ProviderName: id.ProviderName,
				LastSeenAt: id.LastSeenAt, DeletedAt: id.DeletedAt,
			}
		}
	}
	return out
}

func newAdminPlayerSummaries(list []moderation.PlayerSummary) []adminPlayerSummary {
	if list == nil {
		return nil
	}
	out := make([]adminPlayerSummary, len(list))
	for i, p := range list {
		out[i] = newAdminPlayerSummary(p)
	}
	return out
}

type moderationSignalSummary struct {
	ID               int64           `json:"id"`
	SubjectUserID    string          `json:"subjectUserId"`
	SubjectName      string          `json:"subjectName,omitempty"`
	SignalType       string          `json:"signalType"`
	Source           string          `json:"source"`
	Severity         string          `json:"severity"`
	EvidenceStrength string          `json:"evidenceStrength"`
	DetectorKey      string          `json:"detectorKey,omitempty"`
	DetectorVersion  string          `json:"detectorVersion,omitempty"`
	ReasonCode       string          `json:"reasonCode"`
	Score            float64         `json:"score"`
	RecommendedQueue bool            `json:"recommendedQueue"`
	ReporterUserID   string          `json:"reporterUserId,omitempty"`
	ReporterName     string          `json:"reporterName,omitempty"`
	MatchID          string          `json:"matchId,omitempty"`
	Payload          json.RawMessage `json:"payload,omitempty"`
	OccurredAt       time.Time       `json:"occurredAt"`
	CreatedAt        time.Time       `json:"createdAt"`
	ReviewedAt       time.Time       `json:"reviewedAt,omitempty"`
	ReviewedBy       string          `json:"reviewedBy,omitempty"`
	Outcome          string          `json:"outcome,omitempty"`
}

func newModerationSignals(list []moderation.Signal) []moderationSignalSummary {
	if list == nil {
		return nil
	}
	out := make([]moderationSignalSummary, len(list))
	for i, x := range list {
		out[i] = moderationSignalSummary{
			ID: x.ID, SubjectUserID: x.SubjectUserID, SubjectName: x.SubjectName, SignalType: x.SignalType, Source: x.Source,
			Severity: x.Severity, EvidenceStrength: x.EvidenceStrength, DetectorKey: x.DetectorKey,
			DetectorVersion: x.DetectorVersion, ReasonCode: x.ReasonCode, Score: x.Score, RecommendedQueue: x.RecommendedQueue,
			ReporterUserID: x.ReporterUserID, ReporterName: x.ReporterName, MatchID: x.MatchID, Payload: x.Payload,
			OccurredAt: x.OccurredAt, CreatedAt: x.CreatedAt, ReviewedAt: x.ReviewedAt, ReviewedBy: x.ReviewedBy, Outcome: x.Outcome,
		}
	}
	return out
}

type moderationAuditLogEntry struct {
	ID            int64           `json:"id"`
	SubjectUserID string          `json:"subjectUserId,omitempty"`
	SubjectName   string          `json:"subjectName,omitempty"`
	ActorUserID   string          `json:"actorUserId,omitempty"`
	ActorName     string          `json:"actorName,omitempty"`
	Action        string          `json:"action"`
	Reason        string          `json:"reason,omitempty"`
	ExpiresAt     time.Time       `json:"expiresAt,omitempty"`
	SignalIDs     []int64         `json:"signalIds,omitempty"`
	Metadata      json.RawMessage `json:"metadata,omitempty"`
	CreatedAt     time.Time       `json:"createdAt"`
}

func newModerationAuditLog(list []audit.LogEntry) []moderationAuditLogEntry {
	if list == nil {
		return nil
	}
	out := make([]moderationAuditLogEntry, len(list))
	for i, x := range list {
		out[i] = moderationAuditLogEntry{
			ID: x.ID, SubjectUserID: x.SubjectUserID, SubjectName: x.SubjectName, ActorUserID: x.ActorUserID,
			ActorName: x.ActorName, Action: x.Action, Reason: x.Reason, ExpiresAt: x.ExpiresAt, SignalIDs: x.SignalIDs,
			Metadata: x.Metadata, CreatedAt: x.CreatedAt,
		}
	}
	return out
}

type moderationSubjectDetail struct {
	Player adminPlayerSummary `json:"player"`
}

type moderationSubjectProfile struct {
	Player  adminPlayerSummary        `json:"player"`
	Signals []moderationSignalSummary `json:"signals"`
	Log     []moderationAuditLogEntry `json:"log"`
}

func (a *api) adminPlayers(c echo.Context) error {
	actor, err := a.staffActor(c.Request())
	if err != nil {
		return staffError(c, err)
	}
	players, err := a.moderation.SearchSubjects(c.Request().Context(), actor, c.QueryParam("query"), 30)
	if err != nil {
		return staffError(c, err)
	}
	return writeJSON(c, map[string]any{"players": newAdminPlayerSummaries(players)})
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
	return writeJSON(c, moderationSubjectDetail{Player: newAdminPlayerSummary(detail.Player)})
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
	return writeJSON(c, moderationSubjectProfile{
		Player: newAdminPlayerSummary(profile.Player), Signals: newModerationSignals(profile.Signals), Log: newModerationAuditLog(profile.Log),
	})
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
	return writeJSON(c, map[string]any{"signals": newModerationSignals(signals)})
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
	return writeJSON(c, map[string]any{"log": newModerationAuditLog(entries)})
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

type userRoleGrant struct {
	UserID      string    `json:"userId"`
	DisplayName string    `json:"displayName,omitempty"`
	Email       string    `json:"email,omitempty"`
	Role        string    `json:"role"`
	GrantedBy   string    `json:"grantedBy,omitempty"`
	GrantedAt   time.Time `json:"grantedAt"`
	RevokedAt   time.Time `json:"revokedAt,omitempty"`
	Reason      string    `json:"reason,omitempty"`
}

// adminBadgeDefinition is a badge staff may grant by hand.
type adminBadgeDefinition struct {
	ID          string `json:"id"`
	Kind        string `json:"kind"`
	Label       string `json:"label"`
	Description string `json:"description"`
	ImageURL    string `json:"imageUrl"`
	Rarity      string `json:"rarity,omitempty"`
	MaxLevel    int    `json:"maxLevel"`
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
	grants := make([]userRoleGrant, len(roles))
	for i, g := range roles {
		grants[i] = userRoleGrant{
			UserID: g.UserID, DisplayName: g.DisplayName, Email: g.Email, Role: g.Role, GrantedBy: g.GrantedBy,
			GrantedAt: g.GrantedAt, Reason: g.Reason,
		}
	}
	return writeJSON(c, map[string]any{"roles": grants})
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
	catalog, err := a.badges.Catalog(actor)
	if err != nil {
		return staffError(c, err)
	}
	definitions := make([]adminBadgeDefinition, len(catalog))
	for i, b := range catalog {
		definitions[i] = adminBadgeDefinition{
			ID: b.ID, Kind: b.Kind, Label: b.Label, Description: b.Description, ImageURL: b.ImageURL, Rarity: b.Rarity, MaxLevel: b.MaxLevel,
		}
	}
	return writeJSON(c, map[string]any{"badges": definitions})
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
	return writeJSON(c, map[string]any{"badge": newPlayerBadge(grant.Badge), "changed": grant.Changed})
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
	status, err := maintenance.StaffRead(c.Request().Context(), a.db.Pool(), actor)
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
	saved, err := maintenance.Save(c.Request().Context(), a.db.Pool(), actor, status)
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
	if err := maintenance.Clear(c.Request().Context(), a.db.Pool(), actor); err != nil {
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
	return writeJSON(c, newMapUploadQuota(quota))
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
	return writeJSON(c, newCustomMap(item))
}

func (a *api) adminUploadMap(c echo.Context) error {
	mapKey := strings.TrimSpace(c.Param("mapKey"))
	if mapKey != maps.MapKeyMoving && mapKey != maps.MapKeyNMPZ {
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
	return writeJSON(c, mapImportSummary{
		MapID: summary.MapID, MapKey: summary.MapKey, LocationCount: summary.LocationCount, DisplayName: summary.DisplayName,
	})
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
