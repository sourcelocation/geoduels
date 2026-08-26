package main

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
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

var socialRateScript = redis.NewScript(`
local function consume(key, limit, ttl)
  local count = redis.call("INCR", key)
  if count == 1 then redis.call("PEXPIRE", key, ttl) end
  if count > limit then return redis.call("PTTL", key) end
  return 0
end
local account_ttl = consume(KEYS[1], tonumber(ARGV[1]), tonumber(ARGV[3]))
if account_ttl > 0 then return account_ttl end
return consume(KEYS[2], tonumber(ARGV[2]), tonumber(ARGV[3]))
`)

func (a *api) allowSocialAction(r *http.Request, userID, policyName string) (bool, time.Duration, error) {
	policy, ok := socialRatePolicies[policyName]
	if !ok {
		return false, 0, errors.New("unknown social rate policy")
	}
	if a.redis == nil {
		// Tests and local store-only use stay operable; production always supplies Redis.
		return true, 0, nil
	}
	ip := a.clientIP(r)
	ctx, cancel := context.WithTimeout(r.Context(), 400*time.Millisecond)
	defer cancel()
	ttl, err := socialRateScript.Run(ctx, a.redis, []string{
		"api:ratelimit:social:" + policy.Name + ":account:" + userID,
		"api:ratelimit:social:" + policy.Name + ":ip:" + ip,
	}, policy.AccountLimit, policy.IPLimit, policy.Window.Milliseconds()).Int64()
	if err != nil {
		return false, 0, err
	}
	return ttl <= 0, time.Duration(ttl) * time.Millisecond, nil
}

func writeSocialRateLimited(w http.ResponseWriter, retryAfter time.Duration) {
	if retryAfter > 0 {
		seconds := max(1, int(retryAfter.Round(time.Second).Seconds()))
		w.Header().Set("Retry-After", strconv.Itoa(seconds))
	}
	writeSocialError(w, http.StatusTooManyRequests, "rate_limited")
}
