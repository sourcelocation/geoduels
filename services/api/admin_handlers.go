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

	"github.com/gorilla/mux"

	"geoduels/pkg/contracts"
	"geoduels/pkg/maintenance"
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

func (a *api) adminIdentity(r *http.Request) (Identity, error) {
	identity, err := a.authenticatedIdentity(r)
	if err != nil {
		return Identity{}, err
	}
	if identity.IsBanned || !identity.IsAdmin {
		return Identity{}, errors.New("forbidden")
	}
	return identity, nil
}

func (a *api) moderatorIdentity(r *http.Request) (Identity, error) {
	identity, err := a.authenticatedIdentity(r)
	if err != nil {
		return Identity{}, err
	}
	if identity.IsBanned || (!identity.IsAdmin && !identity.IsModerator) {
		return Identity{}, errors.New("forbidden")
	}
	return identity, nil
}

func (a *api) adminBootstrap(w http.ResponseWriter, r *http.Request) {
	claims, identity, err := a.authenticatedAccount(r)
	if err != nil {
		http.Error(w, "identity not found", http.StatusUnauthorized)
		return
	}
	email := strings.ToLower(strings.TrimSpace(identity.Email))
	if email == "" {
		http.Error(w, "email required", http.StatusForbidden)
		return
	}
	if _, ok := a.adminBootstrapEmails[email]; !ok {
		http.Error(w, "not allowlisted", http.StatusForbidden)
		return
	}
	if !identity.IsAdmin {
		if err := a.admin.SetUserAdmin(identity.Sub, true); err != nil {
			http.Error(w, "failed to promote account", http.StatusInternalServerError)
			return
		}
		identity, err = a.accounts.GetIdentity(claims.Sub)
		if err != nil {
			http.Error(w, "identity not found", http.StatusUnauthorized)
			return
		}
	}
	payload, err := a.issueAuthSessionPayload(identity, claims.SessionID)
	if err != nil {
		http.Error(w, "issue session failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(payload)
}

func (a *api) adminPlayers(w http.ResponseWriter, r *http.Request) {
	identity, err := a.moderatorIdentity(r)
	if err != nil {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	players, err := a.admin.SearchPlayers(r.URL.Query().Get("query"), 30)
	if err != nil {
		http.Error(w, "player search unavailable", http.StatusInternalServerError)
		return
	}
	if !identity.IsAdmin {
		sanitizeAdminPlayerSummariesForModerator(players)
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"players": players})
}

func (a *api) adminPlayerDetail(w http.ResponseWriter, r *http.Request) {
	identity, err := a.moderatorIdentity(r)
	if err != nil {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	detail, err := a.admin.GetAdminPlayerDetail(a.resolveEntityID("user", mux.Vars(r)["id"]))
	if err != nil {
		if errors.Is(err, ErrNoRows) {
			http.Error(w, "player not found", http.StatusNotFound)
			return
		}
		http.Error(w, "player detail unavailable", http.StatusInternalServerError)
		return
	}
	if !identity.IsAdmin {
		sanitizeAdminPlayerSummaryForModerator(&detail.Player)
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(detail)
}

func (a *api) adminPlayerMatches(w http.ResponseWriter, r *http.Request) {
	if _, err := a.moderatorIdentity(r); err != nil {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	matches, err := a.matchStore.ListPlayerMatchHistory(a.resolveEntityID("user", mux.Vars(r)["id"]), 50)
	if err != nil {
		http.Error(w, "match history unavailable", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"matches": matches})
}

func (a *api) adminMatchChat(w http.ResponseWriter, r *http.Request) {
	if _, err := a.moderatorIdentity(r); err != nil {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	limit := 200
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			http.Error(w, "invalid limit", http.StatusBadRequest)
			return
		}
		limit = parsed
	}
	matchID := a.resolveEntityID("match", mux.Vars(r)["id"])
	messages, err := a.chatStore.ListChatMessages("match:"+matchID, limit)
	if err != nil {
		http.Error(w, "chat log unavailable", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"messages": messages})
}

func sanitizeAdminPlayerSummariesForModerator(players []AdminPlayerSummary) {
	for i := range players {
		sanitizeAdminPlayerSummaryForModerator(&players[i])
	}
}

func sanitizeAdminPlayerSummaryForModerator(player *AdminPlayerSummary) {
	if player == nil {
		return
	}
	player.Email = ""
	player.LastIPAddress = ""
	player.Identities = nil
}

func (a *api) moderatorSubject(w http.ResponseWriter, r *http.Request) {
	if _, err := a.moderatorIdentity(r); err != nil {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	profile, err := a.moderation.ListSubjectModerationProfile(a.resolveEntityID("user", mux.Vars(r)["userId"]))
	if err != nil {
		http.Error(w, "moderation subject unavailable", http.StatusInternalServerError)
		return
	}
	sanitizeAdminPlayerSummaryForModerator(&profile.Player)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(profile)
}

func (a *api) moderatorSignals(w http.ResponseWriter, r *http.Request) {
	if _, err := a.moderatorIdentity(r); err != nil {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	signals, err := a.moderation.ListModerationSignals(100)
	if err != nil {
		http.Error(w, "moderation signals unavailable", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"signals": signals})
}

func (a *api) adminBanPlayer(w http.ResponseWriter, r *http.Request) {
	admin, err := a.moderatorIdentity(r)
	if err != nil {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	a.banPlayerForCheating(w, r, mux.Vars(r)["id"], admin.Sub)
}

func (a *api) moderatorSubjectCheatingBan(w http.ResponseWriter, r *http.Request) {
	moderator, err := a.moderatorIdentity(r)
	if err != nil {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	a.banPlayerForCheating(w, r, mux.Vars(r)["userId"], moderator.Sub)
}

func (a *api) moderatorSubjectUnban(w http.ResponseWriter, r *http.Request) {
	moderator, err := a.moderatorIdentity(r)
	if err != nil {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	var req struct {
		Reason string `json:"reason"`
	}
	if err := decodeJSONBody(r, &req); err != nil && !errors.Is(err, io.EOF) {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}
	if err := a.moderation.SetPlayerBan(a.resolveEntityID("user", mux.Vars(r)["userId"]), req.Reason, moderator.Sub, false); err != nil {
		http.Error(w, "failed to unban player", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *api) moderatorSubjectMute(w http.ResponseWriter, r *http.Request) {
	moderator, err := a.moderatorIdentity(r)
	if err != nil {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	var req struct {
		Reason        string `json:"reason"`
		DurationHours int    `json:"durationHours"`
	}
	if err := decodeJSONBody(r, &req); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}
	if req.DurationHours <= 0 {
		req.DurationHours = 7 * 24
	}
	if err := a.moderation.SetPlayerMute(a.resolveEntityID("user", mux.Vars(r)["userId"]), mux.Vars(r)["kind"], req.Reason, moderator.Sub, time.Now().Add(time.Duration(req.DurationHours)*time.Hour), true); err != nil {
		http.Error(w, "failed to mute player", http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *api) moderatorSubjectUnmute(w http.ResponseWriter, r *http.Request) {
	moderator, err := a.moderatorIdentity(r)
	if err != nil {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if err := a.moderation.SetPlayerMute(a.resolveEntityID("user", mux.Vars(r)["userId"]), mux.Vars(r)["kind"], "", moderator.Sub, time.Time{}, false); err != nil {
		http.Error(w, "failed to unmute player", http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *api) banPlayerForCheating(w http.ResponseWriter, r *http.Request, rawUserID, actorUserID string) {
	var req struct {
		Reason string `json:"reason"`
	}
	if err := decodeJSONBody(r, &req); err != nil && !errors.Is(err, io.EOF) {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}
	summary, err := a.moderation.BanPlayerForCheating(a.resolveEntityID("user", rawUserID), req.Reason, actorUserID)
	if err != nil {
		http.Error(w, "failed to ban player", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(summary)
}

func (a *api) adminUnbanPlayer(w http.ResponseWriter, r *http.Request) {
	admin, err := a.adminIdentity(r)
	if err != nil {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if err := a.moderation.SetPlayerBan(a.resolveEntityID("user", mux.Vars(r)["id"]), "", admin.Sub, false); err != nil {
		http.Error(w, "failed to unban player", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *api) adminCommunityPardonPreview(w http.ResponseWriter, r *http.Request) {
	if _, err := a.adminIdentity(r); err != nil {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	summary, err := a.moderation.PreviewCommunityPardon(7 * 24 * time.Hour)
	if err != nil {
		http.Error(w, "failed to preview community pardon", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(summary)
}

func (a *api) adminCommunityPardon(w http.ResponseWriter, r *http.Request) {
	admin, err := a.adminIdentity(r)
	if err != nil {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	var req struct {
		Confirm bool `json:"confirm"`
	}
	if err := decodeJSONBody(r, &req); err != nil || !req.Confirm {
		http.Error(w, "explicit confirmation required", http.StatusBadRequest)
		return
	}
	summary, err := a.moderation.PardonBannedPlayers(7*24*time.Hour, admin.Sub)
	if err != nil {
		http.Error(w, "failed to pardon banned players", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(summary)
}

func (a *api) adminClearReporterMute(w http.ResponseWriter, r *http.Request) {
	if _, err := a.adminIdentity(r); err != nil {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if err := a.moderation.ClearReporterMute(a.resolveEntityID("user", mux.Vars(r)["id"])); err != nil {
		http.Error(w, "failed to unmute reporter", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *api) adminPromoteModerator(w http.ResponseWriter, r *http.Request) {
	if _, err := a.adminIdentity(r); err != nil {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	userID := a.resolveEntityID("user", mux.Vars(r)["id"])
	if err := a.admin.SetUserModerator(userID, true); err != nil {
		http.Error(w, "failed to promote moderator", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *api) adminDemoteModerator(w http.ResponseWriter, r *http.Request) {
	if _, err := a.adminIdentity(r); err != nil {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if err := a.admin.SetUserModerator(a.resolveEntityID("user", mux.Vars(r)["id"]), false); err != nil {
		http.Error(w, "failed to demote moderator", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *api) adminSetMapCreatorTier(w http.ResponseWriter, r *http.Request) {
	if _, err := a.adminIdentity(r); err != nil {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	var req struct {
		Tier string `json:"tier"`
	}
	if err := decodeJSONBody(r, &req); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
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
		http.Error(w, "tier must be auto, base, trusted, or established", http.StatusBadRequest)
		return
	}
	repository := a.db
	quota, err := repository.SetMapCreatorTierOverride(a.resolveEntityID("user", mux.Vars(r)["id"]), tier)
	if errors.Is(err, ErrNoRows) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "failed to update map creator tier", http.StatusInternalServerError)
		return
	}
	writeJSONResponse(w, quota)
}

func (a *api) adminListRoles(w http.ResponseWriter, r *http.Request) {
	if _, err := a.adminIdentity(r); err != nil {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	roles, err := a.admin.ListUserRoles()
	if err != nil {
		http.Error(w, "roles unavailable", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"roles": roles})
}

func (a *api) adminGrantRole(w http.ResponseWriter, r *http.Request) {
	admin, err := a.adminIdentity(r)
	if err != nil {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	var req struct {
		UserID string `json:"userId"`
		Role   string `json:"role"`
		Reason string `json:"reason"`
	}
	if err := decodeJSONBody(r, &req); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}
	if err := a.admin.GrantUserRole(a.resolveEntityID("user", req.UserID), req.Role, admin.Sub, req.Reason); err != nil {
		http.Error(w, "failed to grant role", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *api) adminRevokeRole(w http.ResponseWriter, r *http.Request) {
	admin, err := a.adminIdentity(r)
	if err != nil {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	var req struct {
		Reason string `json:"reason"`
	}
	if err := decodeJSONBody(r, &req); err != nil && !errors.Is(err, io.EOF) {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}
	if err := a.admin.RevokeUserRole(a.resolveEntityID("user", mux.Vars(r)["id"]), strings.TrimSpace(mux.Vars(r)["role"]), admin.Sub, req.Reason); err != nil {
		http.Error(w, "failed to revoke role", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *api) adminBadgeDefinitions(w http.ResponseWriter, r *http.Request) {
	if _, err := a.adminIdentity(r); err != nil {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"badges": a.admin.ListAdminGrantableBadges()})
}

func (a *api) adminGrantBadge(w http.ResponseWriter, r *http.Request) {
	admin, err := a.adminIdentity(r)
	if err != nil {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	var req struct {
		Nickname string `json:"nickname"`
		BadgeID  string `json:"badgeId"`
	}
	if err := decodeJSONBody(r, &req); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}
	nickname := strings.TrimSpace(req.Nickname)
	if nickname == "" {
		http.Error(w, "nickname required", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.BadgeID) == "" {
		http.Error(w, "badge id required", http.StatusBadRequest)
		return
	}
	badge, changed, err := a.admin.GrantBadgeToUser(nickname, req.BadgeID, admin.Sub)
	if err != nil {
		switch {
		case errors.Is(err, ErrBadgeUserNotFound):
			http.Error(w, err.Error(), http.StatusNotFound)
		case errors.Is(err, ErrBadgeUnavailable), errors.Is(err, ErrBadgeNicknameRequired):
			http.Error(w, err.Error(), http.StatusBadRequest)
		default:
			http.Error(w, "failed to grant badge", http.StatusInternalServerError)
		}
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"badge": badge, "changed": changed})
}

func (a *api) moderatorLog(w http.ResponseWriter, r *http.Request) {
	if _, err := a.moderatorIdentity(r); err != nil {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	entries, err := a.moderation.ListModerationLog(100)
	if err != nil {
		http.Error(w, "moderation log unavailable", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"log": entries})
}

func (a *api) adminListSignupIPBans(w http.ResponseWriter, r *http.Request) {
	if _, err := a.adminIdentity(r); err != nil {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	bans, err := a.moderation.ListSignupIPBans(100)
	if err != nil {
		http.Error(w, "ip bans unavailable", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"bans": bans})
}

func (a *api) adminAddSignupIPBan(w http.ResponseWriter, r *http.Request) {
	admin, err := a.adminIdentity(r)
	if err != nil {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	var req struct {
		IPAddress string `json:"ipAddress"`
		Reason    string `json:"reason"`
	}
	if err := decodeJSONBody(r, &req); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}
	if err := a.moderation.AddSignupIPBan(strings.TrimSpace(req.IPAddress), req.Reason, admin.Sub); err != nil {
		http.Error(w, "failed to ban ip", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *api) adminRemoveSignupIPBan(w http.ResponseWriter, r *http.Request) {
	if _, err := a.adminIdentity(r); err != nil {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	ip, err := url.PathUnescape(strings.TrimSpace(mux.Vars(r)["ip"]))
	if err != nil {
		http.Error(w, "invalid ip", http.StatusBadRequest)
		return
	}
	if err := a.moderation.RemoveSignupIPBan(ip); err != nil {
		http.Error(w, "failed to remove ip ban", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *api) adminGetMaintenance(w http.ResponseWriter, r *http.Request) {
	if _, err := a.adminIdentity(r); err != nil {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	status, err := maintenance.Read(r.Context(), a.redis)
	if err != nil {
		http.Error(w, "maintenance unavailable", http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(status)
}

func (a *api) adminPutMaintenance(w http.ResponseWriter, r *http.Request) {
	if _, err := a.adminIdentity(r); err != nil {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if a.redis == nil {
		http.Error(w, "redis unavailable", http.StatusBadGateway)
		return
	}
	var status maintenance.Status
	if err := decodeJSONBody(r, &status); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}
	status = status.Normalized()
	body, err := json.Marshal(status)
	if err != nil {
		http.Error(w, "invalid maintenance status", http.StatusBadRequest)
		return
	}
	if err := a.redis.Set(r.Context(), maintenance.RedisKey, body, 0).Err(); err != nil {
		http.Error(w, "failed to save maintenance", http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(status)
}

func (a *api) adminClearMaintenance(w http.ResponseWriter, r *http.Request) {
	if _, err := a.adminIdentity(r); err != nil {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if a.redis == nil {
		http.Error(w, "redis unavailable", http.StatusBadGateway)
		return
	}
	if err := a.redis.Del(r.Context(), maintenance.RedisKey).Err(); err != nil {
		http.Error(w, "failed to clear maintenance", http.StatusBadGateway)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *api) adminGetModerationSettings(w http.ResponseWriter, r *http.Request) {
	if _, err := a.adminIdentity(r); err != nil {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	settings, err := a.content.GetModerationSettings()
	if err != nil {
		http.Error(w, "moderation settings unavailable", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(settings)
}

func (a *api) adminPutModerationSettings(w http.ResponseWriter, r *http.Request) {
	if _, err := a.adminIdentity(r); err != nil {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	var req ModerationSettings
	if err := decodeJSONBody(r, &req); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}
	webhookURL, err := normalizeDiscordWebhookURL(req.DiscordWebhookURL)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	settings := ModerationSettings{DiscordWebhookURL: webhookURL}
	if err := a.content.SetModerationSettings(settings); err != nil {
		http.Error(w, "failed to save moderation settings", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(settings)
}

func (a *api) adminGetDiscordIntegrationSettings(w http.ResponseWriter, r *http.Request) {
	if _, err := a.adminIdentity(r); err != nil {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	settings, err := a.content.GetDiscordIntegrationSettings()
	if err != nil {
		http.Error(w, "discord integration settings unavailable", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(settings)
}

func (a *api) adminPutDiscordIntegrationSettings(w http.ResponseWriter, r *http.Request) {
	if _, err := a.adminIdentity(r); err != nil {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	var settings DiscordIntegrationSettings
	if err := decodeJSONBody(r, &settings); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}
	for label, value := range map[string]string{
		"guild id":         settings.GuildID,
		"joins channel id": settings.JoinsChannelID,
		"1k role id":       settings.Elo1000RoleID,
		"1.5k role id":     settings.Elo1500RoleID,
		"2k role id":       settings.Elo2000RoleID,
	} {
		if err := validateOptionalDiscordSnowflake(label, value); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}
	if settings.ReconcileIntervalMinutes < 1 || settings.ReconcileIntervalMinutes > 1440 {
		http.Error(w, "reconcile interval must be between 1 and 1440 minutes", http.StatusBadRequest)
		return
	}
	// Managed role history is server-owned so old configured roles can be
	// removed safely after an administrator changes a role ID.
	settings.ManagedRoleIDs = nil
	if err := a.content.SetDiscordIntegrationSettings(settings); err != nil {
		http.Error(w, "failed to save discord integration settings", http.StatusInternalServerError)
		return
	}
	saved, err := a.content.GetDiscordIntegrationSettings()
	if err != nil {
		http.Error(w, "discord integration settings unavailable", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(saved)
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

func (a *api) adminGetRankedSeason(w http.ResponseWriter, r *http.Request) {
	if _, err := a.adminIdentity(r); err != nil {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	settings, err := a.seasons.GetRankedSeasonSettings()
	if err != nil {
		http.Error(w, "season settings unavailable", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(settings)
}

func (a *api) adminPutRankedSeasonResetRule(w http.ResponseWriter, r *http.Request) {
	if _, err := a.adminIdentity(r); err != nil {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	var req struct {
		MonthlyResetDay int `json:"monthlyResetDay"`
	}
	if err := decodeJSONBody(r, &req); err != nil && !errors.Is(err, io.EOF) {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}
	settings, err := a.seasons.SetRankedSeasonResetRule(req.MonthlyResetDay)
	if err != nil {
		msg := strings.ToLower(err.Error())
		if strings.Contains(msg, "reset day") {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		http.Error(w, "season settings update failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(settings)
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

func (a *api) publicLobbyChangelog(w http.ResponseWriter, r *http.Request) {
	content, err := a.content.GetLobbyChangelog(defaultLobbyChangelogContent)
	if err != nil {
		http.Error(w, "changelog unavailable", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(content)
}

func (a *api) publicChangelogPosts(w http.ResponseWriter, r *http.Request) {
	posts, err := a.content.ListChangelogPosts(false)
	if err != nil {
		http.Error(w, "changelog unavailable", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"posts": posts})
}

func (a *api) publicChangelogPost(w http.ResponseWriter, r *http.Request) {
	slug := strings.TrimSpace(mux.Vars(r)["slug"])
	post, ok, err := a.content.GetChangelogPostBySlug(slug, true)
	if err != nil {
		http.Error(w, "changelog unavailable", http.StatusInternalServerError)
		return
	}
	if !ok {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(post)
}

func (a *api) adminGetChangelog(w http.ResponseWriter, r *http.Request) {
	if _, err := a.adminIdentity(r); err != nil {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	posts, err := a.content.ListChangelogPosts(true)
	if err != nil {
		http.Error(w, "changelog unavailable", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"posts": posts})
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

func (a *api) adminCreateChangelogPost(w http.ResponseWriter, r *http.Request) {
	if _, err := a.adminIdentity(r); err != nil {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	var req ChangelogPostInput
	if err := decodeJSONBody(r, &req); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}
	input, err := normalizeChangelogPostInput(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	post, err := a.content.CreateChangelogPost(input)
	if err != nil {
		http.Error(w, "failed to save changelog", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(post)
}

func (a *api) adminUpdateChangelogPost(w http.ResponseWriter, r *http.Request) {
	if _, err := a.adminIdentity(r); err != nil {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	id, err := strconv.ParseInt(strings.TrimSpace(mux.Vars(r)["id"]), 10, 64)
	if err != nil || id <= 0 {
		http.Error(w, "invalid post id", http.StatusBadRequest)
		return
	}
	var req ChangelogPostInput
	if err := decodeJSONBody(r, &req); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}
	input, err := normalizeChangelogPostInput(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	post, ok, err := a.content.UpdateChangelogPost(id, input)
	if err != nil {
		http.Error(w, "failed to save changelog", http.StatusInternalServerError)
		return
	}
	if !ok {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(post)
}

func (a *api) adminImportOfficialMap(w http.ResponseWriter, r *http.Request) {
	identity, err := a.adminIdentity(r)
	if err != nil {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	catalog, ok := a.mapCatalog(w)
	if !ok {
		return
	}
	file, closeFile, err := mapUploadFile(w, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
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
	item, err := catalog.ImportOfficialMap(identity.Sub, input, file)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	writeJSONResponse(w, item)
}

func (a *api) adminUploadCurrentMap(w http.ResponseWriter, r *http.Request) {
	a.uploadMap(w, r, contracts.MapKeyMoving)
}

func (a *api) adminUploadMap(w http.ResponseWriter, r *http.Request) {
	mapKey := strings.TrimSpace(mux.Vars(r)["mapKey"])
	if mapKey != contracts.MapKeyMoving && mapKey != contracts.MapKeyNMPZ {
		http.Error(w, "unsupported map key", http.StatusBadRequest)
		return
	}
	a.uploadMap(w, r, mapKey)
}

func (a *api) uploadMap(w http.ResponseWriter, r *http.Request, mapKey string) {
	if _, err := a.adminIdentity(r); err != nil {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if err := r.ParseMultipartForm(64 << 20); err != nil {
		http.Error(w, "invalid multipart form", http.StatusBadRequest)
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "file is required", http.StatusBadRequest)
		return
	}
	defer file.Close()
	dataset, err := readUploadedFile(file, header)
	if err != nil {
		http.Error(w, "failed to read file", http.StatusBadRequest)
		return
	}
	summary, err := a.seasons.ReplaceMapLocations(mapKey, mapKey, dataset)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(summary)
}

func readUploadedFile(file multipart.File, _ *multipart.FileHeader) ([]byte, error) {
	return io.ReadAll(file)
}

var _ contracts.MapImportSummary
