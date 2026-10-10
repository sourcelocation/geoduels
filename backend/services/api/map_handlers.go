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

	"github.com/labstack/echo/v4"

	"geoduels/pkg/contracts"
	"geoduels/pkg/ratelimit"
	pkgstaff "geoduels/pkg/staff"
)

const maxMapUploadBytes = int64(128 << 20)

// allowHits counts one hit against each limit in turn, stopping at the first one exceeded.
func (a *api) allowHits(ctx context.Context, limits ...rateLimit) (bool, time.Duration, error) {
	for _, limit := range limits {
		allowed, retryAfter, err := ratelimit.Hit(ctx, a.db.Pool(), limit.key, limit.max, limit.window)
		if err != nil || !allowed {
			return allowed, retryAfter, err
		}
	}
	return true, 0, nil
}

type rateLimit struct {
	key    string
	max    int
	window time.Duration
}

func (a *api) allowMapUploadAttempt(userID string) (bool, time.Duration, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	return a.allowHits(ctx,
		rateLimit{"api:ratelimit:map_upload:hour:" + userID, 10, time.Hour},
		rateLimit{"api:ratelimit:map_upload:day:" + userID, 30, 24 * time.Hour},
	)
}

func (a *api) allowMapComment(userID, mapID string) (bool, time.Duration, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	userID = strings.TrimSpace(userID)
	mapID = strings.TrimSpace(mapID)
	return a.allowHits(ctx,
		rateLimit{"api:ratelimit:map_comment:min:" + userID, 5, time.Minute},
		rateLimit{"api:ratelimit:map_comment:day:" + userID, 100, 24 * time.Hour},
		rateLimit{"api:ratelimit:map_comment:hour:" + userID, 30, time.Hour},
		rateLimit{"api:ratelimit:map_comment:maphour:" + userID + ":" + mapID, 10, time.Hour},
	)
}

func (a *api) mapUser(c echo.Context, registeredRequired bool) (string, bool) {
	claims, err := a.authenticatedClaims(c.Request())
	if err != nil {
		_ = plainTextError(c, http.StatusUnauthorized, "unauthorized")
		return "", false
	}
	if registeredRequired {
		profile, err := a.profiles.GetProfile(claims.Sub)
		if err != nil {
			_ = plainTextError(c, http.StatusInternalServerError, "profile unavailable")
			return "", false
		}
		if profile.IsGuest {
			_ = plainTextError(c, http.StatusForbidden, "guest accounts cannot interact with maps")
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

func (a *api) listMaps(c echo.Context) error {
	r := c.Request()
	userID := a.optionalMapUser(r)
	scope := strings.TrimSpace(c.QueryParam("scope"))
	if scope == "mine" || scope == "favorites" {
		var ok bool
		userID, ok = a.mapUser(c, false)
		if !ok {
			return nil
		}
	}
	items, err := a.maps.ListMaps(userID, contracts.MapListOptions{Scope: scope, Sort: c.QueryParam("sort"), Search: c.QueryParam("search")})
	if err != nil {
		return plainTextError(c, http.StatusInternalServerError, "maps unavailable")
	}
	return writeJSONResponse(c, items)
}

func (a *api) mapUploadQuota(c echo.Context) error {
	userID, ok := a.mapUser(c, true)
	if !ok {
		return nil
	}
	quota, err := a.maps.GetMapUploadQuota(userID)
	if err != nil {
		return plainTextError(c, http.StatusInternalServerError, "map quota unavailable")
	}
	return writeJSONResponse(c, quota)
}

func (a *api) getMap(c echo.Context) error {
	r := c.Request()
	userID := a.optionalMapUser(r)
	item, found, err := a.maps.GetMap(userID, resolveCompactEntityID(c.Param("id")))
	if err != nil {
		return plainTextError(c, http.StatusInternalServerError, "map unavailable")
	}
	if !found {
		return plainTextError(c, http.StatusNotFound, "404 page not found")
	}
	return writeJSONResponse(c, item)
}

func (a *api) createMap(c echo.Context) error {
	r := c.Request()
	userID, ok := a.mapUser(c, true)
	if !ok {
		return nil
	}
	if allowed, retryAfter, err := a.allowMapUploadAttempt(userID); err != nil {
		return plainTextError(c, http.StatusServiceUnavailable, "map uploads temporarily unavailable")
	} else if !allowed {
		if retryAfter > 0 {
			c.Response().Header().Set("Retry-After", fmt.Sprintf("%d", max(1, int(retryAfter.Seconds()))))
		}
		return plainTextError(c, http.StatusTooManyRequests, "map upload rate limit exceeded")
	}
	file, closeFile, err := mapUploadFile(c)
	if err != nil {
		return plainTextError(c, http.StatusBadRequest, err.Error())
	}
	defer closeFile()
	item, err := a.maps.CreateCustomMap(userID, r.FormValue("displayName"), r.FormValue("description"), r.FormValue("visibility"), r.FormValue("difficulty"), r.FormValue("thumbnailKey"), atoiDefault(r.FormValue("thumbnailVariant"), 1), file)
	if err != nil {
		return plainTextError(c, http.StatusBadRequest, err.Error())
	}
	c.Response().Header().Set("Content-Type", "application/json")
	c.Response().WriteHeader(http.StatusCreated)
	return writeJSONResponse(c, item)
}

func (a *api) replaceMapLocations(c echo.Context) error {
	userID, ok := a.mapUser(c, true)
	if !ok {
		return nil
	}
	if allowed, retryAfter, err := a.allowMapUploadAttempt(userID); err != nil {
		return plainTextError(c, http.StatusServiceUnavailable, "map uploads temporarily unavailable")
	} else if !allowed {
		if retryAfter > 0 {
			c.Response().Header().Set("Retry-After", fmt.Sprintf("%d", max(1, int(retryAfter.Seconds()))))
		}
		return plainTextError(c, http.StatusTooManyRequests, "map upload rate limit exceeded")
	}
	file, closeFile, err := mapUploadFile(c)
	if err != nil {
		return plainTextError(c, http.StatusBadRequest, err.Error())
	}
	defer closeFile()
	item, err := a.maps.ReplaceCustomMapLocations(userID, resolveCompactEntityID(c.Param("id")), file)
	if errors.Is(err, ErrNoRows) {
		return plainTextError(c, http.StatusNotFound, "404 page not found")
	}
	if err != nil {
		return plainTextError(c, http.StatusBadRequest, err.Error())
	}
	return writeJSONResponse(c, item)
}

func (a *api) updateMap(c echo.Context) error {
	r := c.Request()
	userID, ok := a.mapUser(c, true)
	if !ok {
		return nil
	}
	var update contracts.CustomMapUpdate
	if err := json.NewDecoder(io.LimitReader(r.Body, 16<<10)).Decode(&update); err != nil {
		return plainTextError(c, http.StatusBadRequest, "invalid payload")
	}
	item, err := a.maps.UpdateCustomMap(userID, resolveCompactEntityID(c.Param("id")), update)
	if errors.Is(err, ErrNoRows) {
		return plainTextError(c, http.StatusNotFound, "404 page not found")
	}
	if err != nil {
		return plainTextError(c, http.StatusBadRequest, err.Error())
	}
	return writeJSONResponse(c, item)
}

func (a *api) archiveMap(c echo.Context) error {
	userID, ok := a.mapUser(c, true)
	if !ok {
		return nil
	}
	identity, err := a.accounts.GetIdentity(userID)
	if err != nil {
		return plainTextError(c, http.StatusInternalServerError, "identity unavailable")
	}
	err = a.maps.ArchiveCustomMap(userID, resolveCompactEntityID(c.Param("id")), identity.IsAdmin)
	if errors.Is(err, ErrNoRows) {
		return plainTextError(c, http.StatusNotFound, "404 page not found")
	}
	if err != nil {
		return plainTextError(c, http.StatusBadRequest, "could not archive map")
	}
	return c.NoContent(http.StatusNoContent)
}

func (a *api) publishMap(c echo.Context) error {
	userID, ok := a.mapUser(c, true)
	if !ok {
		return nil
	}
	item, err := a.maps.PublishCustomMap(userID, resolveCompactEntityID(c.Param("id")))
	if errors.Is(err, ErrNoRows) {
		return plainTextError(c, http.StatusNotFound, "404 page not found")
	}
	if err != nil {
		return plainTextError(c, http.StatusBadRequest, err.Error())
	}
	return writeJSONResponse(c, item)
}

func (a *api) setMapOfficial(c echo.Context) error {
	actor, err := a.requireStaff(c.Request(), pkgstaff.CapManageMaps)
	if err != nil {
		return staffError(c, err)
	}
	item, err := a.maps.SetMapOfficial(actor.ID, resolveCompactEntityID(c.Param("id")), true)
	if errors.Is(err, ErrNoRows) {
		return plainTextError(c, http.StatusNotFound, "404 page not found")
	}
	if err != nil {
		return plainTextError(c, http.StatusBadRequest, err.Error())
	}
	return writeJSONResponse(c, item)
}

func (a *api) unsetMapOfficial(c echo.Context) error {
	actor, err := a.requireStaff(c.Request(), pkgstaff.CapManageMaps)
	if err != nil {
		return staffError(c, err)
	}
	item, err := a.maps.SetMapOfficial(actor.ID, resolveCompactEntityID(c.Param("id")), false)
	if errors.Is(err, ErrNoRows) {
		return plainTextError(c, http.StatusNotFound, "404 page not found")
	}
	if err != nil {
		return plainTextError(c, http.StatusBadRequest, err.Error())
	}
	return writeJSONResponse(c, item)
}

func (a *api) setGameplayMapRole(c echo.Context) error {
	actor, err := a.requireStaff(c.Request(), pkgstaff.CapManageMaps)
	if err != nil {
		return staffError(c, err)
	}
	item, err := a.maps.SetGameplayMapRole(actor.ID, resolveCompactEntityID(c.Param("id")), c.Param("role"))
	if errors.Is(err, ErrNoRows) {
		return plainTextError(c, http.StatusNotFound, "404 page not found")
	}
	if err != nil {
		return plainTextError(c, http.StatusBadRequest, err.Error())
	}
	return writeJSONResponse(c, item)
}

func (a *api) favoriteMap(c echo.Context) error {
	userID, ok := a.mapUser(c, true)
	if !ok {
		return nil
	}
	item, err := a.maps.SetMapFavorite(userID, resolveCompactEntityID(c.Param("id")), true)
	if errors.Is(err, ErrNoRows) {
		return plainTextError(c, http.StatusNotFound, "404 page not found")
	}
	if err != nil {
		return plainTextError(c, http.StatusBadRequest, "could not favorite map")
	}
	return writeJSONResponse(c, item)
}

func (a *api) unfavoriteMap(c echo.Context) error {
	userID, ok := a.mapUser(c, true)
	if !ok {
		return nil
	}
	item, err := a.maps.SetMapFavorite(userID, resolveCompactEntityID(c.Param("id")), false)
	if errors.Is(err, ErrNoRows) {
		return plainTextError(c, http.StatusNotFound, "404 page not found")
	}
	if err != nil {
		return plainTextError(c, http.StatusBadRequest, "could not unfavorite map")
	}
	return writeJSONResponse(c, item)
}

func (a *api) listMapComments(c echo.Context) error {
	userID := a.optionalMapUser(c.Request())
	items, err := a.maps.ListMapComments(userID, resolveCompactEntityID(c.Param("id")))
	if errors.Is(err, ErrNoRows) {
		return plainTextError(c, http.StatusNotFound, "404 page not found")
	}
	if err != nil {
		return plainTextError(c, http.StatusInternalServerError, "comments unavailable")
	}
	return writeJSONResponse(c, items)
}

func (a *api) createMapComment(c echo.Context) error {
	r := c.Request()
	userID, ok := a.mapUser(c, true)
	if !ok {
		return nil
	}
	if allowed, retryAfter, err := a.allowMapComment(userID, resolveCompactEntityID(c.Param("id"))); err != nil {
		return plainTextError(c, http.StatusServiceUnavailable, "comments temporarily unavailable")
	} else if !allowed {
		if retryAfter > 0 {
			c.Response().Header().Set("Retry-After", fmt.Sprintf("%d", max(1, int(retryAfter.Seconds()))))
		}
		return plainTextError(c, http.StatusTooManyRequests, "comment rate limit exceeded")
	}
	var input contracts.MapCommentCreate
	if err := json.NewDecoder(io.LimitReader(r.Body, 8<<10)).Decode(&input); err != nil {
		return plainTextError(c, http.StatusBadRequest, "invalid payload")
	}
	item, err := a.maps.CreateMapComment(userID, resolveCompactEntityID(c.Param("id")), input)
	if errors.Is(err, ErrNoRows) {
		return plainTextError(c, http.StatusNotFound, "404 page not found")
	}
	if err != nil {
		return plainTextError(c, http.StatusBadRequest, err.Error())
	}
	c.Response().WriteHeader(http.StatusCreated)
	return writeJSONResponse(c, item)
}

func (a *api) deleteMapComment(c echo.Context) error {
	userID, ok := a.mapUser(c, false)
	if !ok {
		return nil
	}
	profile, err := a.profiles.GetProfile(userID)
	if err != nil {
		return plainTextError(c, http.StatusInternalServerError, "profile unavailable")
	}
	err = a.maps.DeleteMapComment(userID, resolveCompactEntityID(c.Param("id")), a.resolveEntityID("comment", c.Param("commentId")), profile.IsAdmin || profile.IsModerator)
	if errors.Is(err, ErrNoRows) {
		return plainTextError(c, http.StatusNotFound, "404 page not found")
	}
	if err != nil {
		return plainTextError(c, http.StatusBadRequest, "could not delete comment")
	}
	return c.NoContent(http.StatusNoContent)
}

func (a *api) likeMapComment(c echo.Context) error {
	return a.setMapCommentLike(c, true)
}

func (a *api) unlikeMapComment(c echo.Context) error {
	return a.setMapCommentLike(c, false)
}

func (a *api) setMapCommentLike(c echo.Context, liked bool) error {
	userID, ok := a.mapUser(c, true)
	if !ok {
		return nil
	}
	item, err := a.maps.SetMapCommentLike(userID, resolveCompactEntityID(c.Param("id")), a.resolveEntityID("comment", c.Param("commentId")), liked)
	if errors.Is(err, ErrNoRows) {
		return plainTextError(c, http.StatusNotFound, "404 page not found")
	}
	if err != nil {
		return plainTextError(c, http.StatusBadRequest, "could not update comment like")
	}
	return writeJSONResponse(c, item)
}

func mapUploadFile(c echo.Context) (io.Reader, func(), error) {
	r := c.Request()
	r.Body = http.MaxBytesReader(c.Response(), r.Body, maxMapUploadBytes)
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

func writeJSONResponse(c echo.Context, value any) error {
	c.Response().Header().Set("Content-Type", "application/json")
	return json.NewEncoder(c.Response()).Encode(value)
}

func atoiDefault(raw string, fallback int) int {
	var out int
	if _, err := fmt.Sscanf(strings.TrimSpace(raw), "%d", &out); err != nil {
		return fallback
	}
	return out
}
