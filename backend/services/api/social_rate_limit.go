package main

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"

	"geoduels/pkg/ratelimit"
)

type socialRatePolicy struct {
	Name         string
	AccountLimit int
	IPLimit      int
	Window       time.Duration
}

var socialRatePolicies = map[string]socialRatePolicy{
	"friend_request": {Name: "friend_request", AccountLimit: 30, IPLimit: 80, Window: time.Hour},
	"player_search":  {Name: "player_search", AccountLimit: 120, IPLimit: 240, Window: time.Hour},
	"code_resolve":   {Name: "code_resolve", AccountLimit: 30, IPLimit: 60, Window: time.Hour},
	"party_invite":   {Name: "party_invite", AccountLimit: 60, IPLimit: 120, Window: time.Hour},
	"socket_connect": {Name: "socket_connect", AccountLimit: 30, IPLimit: 60, Window: time.Minute},
}

func (a *api) allowSocialAction(r *http.Request, userID, policyName string) (bool, time.Duration, error) {
	policy, ok := socialRatePolicies[policyName]
	if !ok {
		return false, 0, errors.New("unknown social rate policy")
	}
	if a.db == nil {
		// Store-only tests stay operable; production always has the database.
		return true, 0, nil
	}
	ip := a.clientIP(r)
	ctx, cancel := context.WithTimeout(r.Context(), 400*time.Millisecond)
	defer cancel()
	prefix := "api:ratelimit:social:" + policy.Name
	allowed, retryAfter, err := ratelimit.Hit(ctx, a.db.Pool(), prefix+":account:"+userID, policy.AccountLimit, policy.Window)
	if err != nil || !allowed {
		return allowed, retryAfter, err
	}
	return ratelimit.Hit(ctx, a.db.Pool(), prefix+":ip:"+ip, policy.IPLimit, policy.Window)
}

func writeSocialRateLimited(c echo.Context, retryAfter time.Duration) error {
	if retryAfter > 0 {
		seconds := max(1, int(retryAfter.Round(time.Second).Seconds()))
		c.Response().Header().Set("Retry-After", strconv.Itoa(seconds))
	}
	return writeSocialError(c, http.StatusTooManyRequests, "rate_limited")
}
