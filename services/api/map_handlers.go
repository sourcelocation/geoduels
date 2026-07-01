package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v5"

	"geoduels/pkg/contentfilter"
	"geoduels/pkg/contracts"
	"geoduels/pkg/persistence"
)

const maxMapUploadBytes = int64(128 << 20)

func (a *api) allowMapUploadAttempt(userID string) (bool, time.Duration, error) {
	if a.redis == nil {
		return false, 0, errors.New("map upload rate limit unavailable")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	result, err := guestSignupRateLimitScript.Run(ctx, a.redis, []string{
		"api:ratelimit:map_upload:hour:" + userID,
		"api:ratelimit:map_upload:day:" + userID,
	}, time.Hour.Milliseconds(), 10, (24 * time.Hour).Milliseconds(), 30).Slice()
	if err != nil {
		return false, 0, err
	}
	if len(result) != 2 {
		return false, 0, errors.New("unexpected map upload rate limit response")
	}
	allowed, err := redisInt64(result[0])
	if err != nil {
		return false, 0, err
	}
	ttl, err := redisInt64(result[1])
	if err != nil {
		return false, 0, err
	}
	return allowed == 1, time.Duration(ttl) * time.Millisecond, nil
}

func (a *api) allowMapComment(userID, mapID string) (bool, time.Duration, error) {
	if a.redis == nil {
		return false, 0, errors.New("comment rate limit unavailable")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	check := func(keys []string, args ...any) (bool, time.Duration, error) {
		result, err := guestSignupRateLimitScript.Run(ctx, a.redis, keys, args...).Slice()
		if err != nil {
			return false, 0, err
		}
		if len(result) != 2 {
			return false, 0, errors.New("unexpected comment rate limit response")
		}
		allowed, err := redisInt64(result[0])
		if err != nil {
			return false, 0, err
		}
		ttl, err := redisInt64(result[1])
		if err != nil {
			return false, 0, err
		}
		return allowed == 1, time.Duration(ttl) * time.Millisecond, nil
	}
	userID = strings.TrimSpace(userID)
	mapID = strings.TrimSpace(mapID)
	allowed, retryAfter, err := check(
		[]string{"api:ratelimit:map_comment:min:" + userID, "api:ratelimit:map_comment:day:" + userID},
		time.Minute.Milliseconds(), 5,
		(24 * time.Hour).Milliseconds(), 100,
	)
	if err != nil || !allowed {
		return allowed, retryAfter, err
	}
	return check(
		[]string{"api:ratelimit:map_comment:hour:" + userID, "api:ratelimit:map_comment:maphour:" + userID + ":" + mapID},
		time.Hour.Milliseconds(), 30,
		time.Hour.Milliseconds(), 10,
	)
}

func (a *api) mapCatalog(w http.ResponseWriter) (persistence.MapCatalog, bool) {
	catalog, ok := a.store.(persistence.MapCatalog)
	if !ok {
		http.Error(w, "map catalog unavailable", http.StatusServiceUnavailable)
	}
	return catalog, ok
}

func (a *api) mapUser(w http.ResponseWriter, r *http.Request, registeredRequired bool) (string, bool) {
	claims, err := a.authenticatedClaims(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return "", false
	}
	if registeredRequired {
		profile, err := a.store.GetProfile(claims.Sub)
		if err != nil {
			http.Error(w, "profile unavailable", http.StatusInternalServerError)
			return "", false
		}
		if profile.IsGuest {
			http.Error(w, "guest accounts cannot interact with maps", http.StatusForbidden)
			return "", false
		}
	}
	return claims.Sub, true
}

func (a *api) optionalMapUser(r *http.Request) string {
	claims, err := a.authenticatedClaims(r)
	if err != nil {
		return ""
	}
	return claims.Sub
}

func (a *api) listMaps(w http.ResponseWriter, r *http.Request) {
	userID := a.optionalMapUser(r)
	scope := strings.TrimSpace(r.URL.Query().Get("scope"))
	if scope == "mine" || scope == "favorites" {
		var ok bool
		userID, ok = a.mapUser(w, r, false)
		if !ok {
			return
		}
	}
	catalog, ok := a.mapCatalog(w)
	if !ok {
		return
	}
	items, err := catalog.ListMaps(userID, contracts.MapListOptions{Scope: scope, Sort: r.URL.Query().Get("sort"), Search: r.URL.Query().Get("search")})
	if err != nil {
		http.Error(w, "maps unavailable", http.StatusInternalServerError)
		return
	}
	writeJSONResponse(w, items)
}

func (a *api) mapUploadQuota(w http.ResponseWriter, r *http.Request) {
	userID, ok := a.mapUser(w, r, true)
	if !ok {
		return
	}
	catalog, ok := a.mapCatalog(w)
	if !ok {
		return
	}
	quota, err := catalog.GetMapUploadQuota(userID)
	if err != nil {
		http.Error(w, "map quota unavailable", http.StatusInternalServerError)
		return
	}
	writeJSONResponse(w, quota)
}

func (a *api) getMap(w http.ResponseWriter, r *http.Request) {
	userID := a.optionalMapUser(r)
	catalog, ok := a.mapCatalog(w)
	if !ok {
		return
	}
	item, found, err := catalog.GetMap(userID, resolveCompactEntityID(mux.Vars(r)["id"]))
	if err != nil {
		http.Error(w, "map unavailable", http.StatusInternalServerError)
		return
	}
	if !found {
		http.NotFound(w, r)
		return
	}
	writeJSONResponse(w, item)
}

func (a *api) createMap(w http.ResponseWriter, r *http.Request) {
	userID, ok := a.mapUser(w, r, true)
	if !ok {
		return
	}
	if allowed, retryAfter, err := a.allowMapUploadAttempt(userID); err != nil {
		http.Error(w, "map uploads temporarily unavailable", http.StatusServiceUnavailable)
		return
	} else if !allowed {
		if retryAfter > 0 {
			w.Header().Set("Retry-After", fmt.Sprintf("%d", max(1, int(retryAfter.Seconds()))))
		}
		http.Error(w, "map upload rate limit exceeded", http.StatusTooManyRequests)
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
	if err := contentfilter.RejectAbusiveText(r.FormValue("displayName"), r.FormValue("description")); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	item, err := catalog.CreateCustomMap(userID, r.FormValue("displayName"), r.FormValue("description"), r.FormValue("visibility"), r.FormValue("difficulty"), r.FormValue("thumbnailKey"), atoiDefault(r.FormValue("thumbnailVariant"), 1), file)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	writeJSONResponse(w, item)
}

func (a *api) replaceMapLocations(w http.ResponseWriter, r *http.Request) {
	userID, ok := a.mapUser(w, r, true)
	if !ok {
		return
	}
	if allowed, retryAfter, err := a.allowMapUploadAttempt(userID); err != nil {
		http.Error(w, "map uploads temporarily unavailable", http.StatusServiceUnavailable)
		return
	} else if !allowed {
		if retryAfter > 0 {
			w.Header().Set("Retry-After", fmt.Sprintf("%d", max(1, int(retryAfter.Seconds()))))
		}
		http.Error(w, "map upload rate limit exceeded", http.StatusTooManyRequests)
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
	item, err := catalog.ReplaceCustomMapLocations(userID, resolveCompactEntityID(mux.Vars(r)["id"]), file)
	if errors.Is(err, pgx.ErrNoRows) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSONResponse(w, item)
}

func (a *api) updateMap(w http.ResponseWriter, r *http.Request) {
	userID, ok := a.mapUser(w, r, true)
	if !ok {
		return
	}
	catalog, ok := a.mapCatalog(w)
	if !ok {
		return
	}
	var update contracts.CustomMapUpdate
	if err := json.NewDecoder(io.LimitReader(r.Body, 16<<10)).Decode(&update); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}
	if err := contentfilter.RejectAbusiveText(update.DisplayName, update.Description); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	item, err := catalog.UpdateCustomMap(userID, resolveCompactEntityID(mux.Vars(r)["id"]), update)
	if errors.Is(err, pgx.ErrNoRows) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSONResponse(w, item)
}

func (a *api) archiveMap(w http.ResponseWriter, r *http.Request) {
	userID, ok := a.mapUser(w, r, true)
	if !ok {
		return
	}
	catalog, ok := a.mapCatalog(w)
	if !ok {
		return
	}
	identity, err := a.store.GetIdentity(userID)
	if err != nil {
		http.Error(w, "identity unavailable", http.StatusInternalServerError)
		return
	}
	err = catalog.ArchiveCustomMap(userID, resolveCompactEntityID(mux.Vars(r)["id"]), identity.IsAdmin)
	if errors.Is(err, pgx.ErrNoRows) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "could not archive map", http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *api) publishMap(w http.ResponseWriter, r *http.Request) {
	userID, ok := a.mapUser(w, r, true)
	if !ok {
		return
	}
	catalog, ok := a.mapCatalog(w)
	if !ok {
		return
	}
	item, err := catalog.PublishCustomMap(userID, resolveCompactEntityID(mux.Vars(r)["id"]))
	if errors.Is(err, pgx.ErrNoRows) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSONResponse(w, item)
}

func (a *api) setMapOfficial(w http.ResponseWriter, r *http.Request) {
	admin, err := a.adminIdentity(r)
	if err != nil {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	catalog, ok := a.mapCatalog(w)
	if !ok {
		return
	}
	item, err := catalog.SetMapOfficial(admin.Sub, resolveCompactEntityID(mux.Vars(r)["id"]), true)
	if errors.Is(err, pgx.ErrNoRows) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSONResponse(w, item)
}

func (a *api) unsetMapOfficial(w http.ResponseWriter, r *http.Request) {
	admin, err := a.adminIdentity(r)
	if err != nil {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	catalog, ok := a.mapCatalog(w)
	if !ok {
		return
	}
	item, err := catalog.SetMapOfficial(admin.Sub, resolveCompactEntityID(mux.Vars(r)["id"]), false)
	if errors.Is(err, pgx.ErrNoRows) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSONResponse(w, item)
}

func (a *api) setGameplayMapRole(w http.ResponseWriter, r *http.Request) {
	admin, err := a.adminIdentity(r)
	if err != nil {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	catalog, ok := a.mapCatalog(w)
	if !ok {
		return
	}
	item, err := catalog.SetGameplayMapRole(admin.Sub, resolveCompactEntityID(mux.Vars(r)["id"]), mux.Vars(r)["role"])
	if errors.Is(err, pgx.ErrNoRows) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSONResponse(w, item)
}

func (a *api) favoriteMap(w http.ResponseWriter, r *http.Request) {
	userID, ok := a.mapUser(w, r, true)
	if !ok {
		return
	}
	catalog, ok := a.mapCatalog(w)
	if !ok {
		return
	}
	item, err := catalog.SetMapFavorite(userID, resolveCompactEntityID(mux.Vars(r)["id"]), true)
	if errors.Is(err, pgx.ErrNoRows) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "could not favorite map", http.StatusBadRequest)
		return
	}
	writeJSONResponse(w, item)
}

func (a *api) unfavoriteMap(w http.ResponseWriter, r *http.Request) {
	userID, ok := a.mapUser(w, r, true)
	if !ok {
		return
	}
	catalog, ok := a.mapCatalog(w)
	if !ok {
		return
	}
	item, err := catalog.SetMapFavorite(userID, resolveCompactEntityID(mux.Vars(r)["id"]), false)
	if errors.Is(err, pgx.ErrNoRows) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "could not unfavorite map", http.StatusBadRequest)
		return
	}
	writeJSONResponse(w, item)
}

func (a *api) listMapComments(w http.ResponseWriter, r *http.Request) {
	userID := a.optionalMapUser(r)
	catalog, ok := a.mapCatalog(w)
	if !ok {
		return
	}
	items, err := catalog.ListMapComments(userID, resolveCompactEntityID(mux.Vars(r)["id"]))
	if errors.Is(err, pgx.ErrNoRows) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "comments unavailable", http.StatusInternalServerError)
		return
	}
	writeJSONResponse(w, items)
}

func (a *api) createMapComment(w http.ResponseWriter, r *http.Request) {
	userID, ok := a.mapUser(w, r, true)
	if !ok {
		return
	}
	if allowed, retryAfter, err := a.allowMapComment(userID, resolveCompactEntityID(mux.Vars(r)["id"])); err != nil {
		http.Error(w, "comments temporarily unavailable", http.StatusServiceUnavailable)
		return
	} else if !allowed {
		if retryAfter > 0 {
			w.Header().Set("Retry-After", fmt.Sprintf("%d", max(1, int(retryAfter.Seconds()))))
		}
		http.Error(w, "comment rate limit exceeded", http.StatusTooManyRequests)
		return
	}
	catalog, ok := a.mapCatalog(w)
	if !ok {
		return
	}
	var input contracts.MapCommentCreate
	if err := json.NewDecoder(io.LimitReader(r.Body, 8<<10)).Decode(&input); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}
	if err := contentfilter.RejectAbusiveText(input.Body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	item, err := catalog.CreateMapComment(userID, resolveCompactEntityID(mux.Vars(r)["id"]), input)
	if errors.Is(err, pgx.ErrNoRows) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusCreated)
	writeJSONResponse(w, item)
}

func (a *api) deleteMapComment(w http.ResponseWriter, r *http.Request) {
	userID, ok := a.mapUser(w, r, false)
	if !ok {
		return
	}
	profile, err := a.store.GetProfile(userID)
	if err != nil {
		http.Error(w, "profile unavailable", http.StatusInternalServerError)
		return
	}
	catalog, ok := a.mapCatalog(w)
	if !ok {
		return
	}
	err = catalog.DeleteMapComment(userID, resolveCompactEntityID(mux.Vars(r)["id"]), a.resolveEntityID("comment", mux.Vars(r)["commentId"]), profile.IsAdmin || profile.IsModerator)
	if errors.Is(err, pgx.ErrNoRows) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "could not delete comment", http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *api) likeMapComment(w http.ResponseWriter, r *http.Request) {
	a.setMapCommentLike(w, r, true)
}

func (a *api) unlikeMapComment(w http.ResponseWriter, r *http.Request) {
	a.setMapCommentLike(w, r, false)
}

func (a *api) setMapCommentLike(w http.ResponseWriter, r *http.Request, liked bool) {
	userID, ok := a.mapUser(w, r, true)
	if !ok {
		return
	}
	catalog, ok := a.mapCatalog(w)
	if !ok {
		return
	}
	item, err := catalog.SetMapCommentLike(userID, resolveCompactEntityID(mux.Vars(r)["id"]), a.resolveEntityID("comment", mux.Vars(r)["commentId"]), liked)
	if errors.Is(err, pgx.ErrNoRows) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "could not update comment like", http.StatusBadRequest)
		return
	}
	writeJSONResponse(w, item)
}

func mapUploadFile(w http.ResponseWriter, r *http.Request) (io.Reader, func(), error) {
	r.Body = http.MaxBytesReader(w, r.Body, maxMapUploadBytes)
	if err := r.ParseMultipartForm(maxMapUploadBytes); err != nil {
		return nil, func() {}, errors.New("map upload must be multipart JSON under 128 MiB")
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		return nil, func() {}, errors.New("file is required")
	}
	if header.Size > maxMapUploadBytes {
		file.Close()
		return nil, func() {}, errors.New("map file exceeds 128 MiB")
	}
	if name := strings.ToLower(header.Filename); name != "" && !strings.HasSuffix(name, ".json") {
		file.Close()
		return nil, func() {}, errors.New("map file must be JSON")
	}
	return file, func() {
		_ = file.Close()
		if r.MultipartForm != nil {
			_ = r.MultipartForm.RemoveAll()
		}
	}, nil
}

func writeJSONResponse(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(value)
}

func atoiDefault(raw string, fallback int) int {
	var out int
	if _, err := fmt.Sscanf(strings.TrimSpace(raw), "%d", &out); err != nil {
		return fallback
	}
	return out
}
