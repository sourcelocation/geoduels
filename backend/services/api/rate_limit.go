package main

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"

	"geoduels/pkg/ratelimit"
)

const guestSignupRateLimitKeyPrefix = "api:ratelimit:guest_signup:ip:"
const guestSignupDailyRateLimitKeyPrefix = "api:ratelimit:guest_signup:daily:ip:"

func (a *api) checkGuestSignupRateLimit(r *http.Request) (bool, time.Duration, error) {
	if a.guestSignupIPLimit <= 0 && a.guestSignupDailyLimit <= 0 {
		return true, 0, nil
	}
	window := a.guestSignupIPWindow
	if window <= 0 {
		window = 10 * time.Minute
	}
	ip := a.clientIP(r)
	if ip == "" {
		ip = "unknown"
	}
	dailyWindow := a.guestSignupDailyWindow
	if dailyWindow <= 0 {
		dailyWindow = 24 * time.Hour
	}
	ctx, cancel := context.WithTimeout(r.Context(), 500*time.Millisecond)
	defer cancel()
	allowed, retryAfter, err := ratelimit.Hit(ctx, a.db.Pool(), guestSignupRateLimitKeyPrefix+ip, a.guestSignupIPLimit, window)
	if err != nil || !allowed {
		return allowed, retryAfter, err
	}
	return ratelimit.Hit(ctx, a.db.Pool(), guestSignupDailyRateLimitKeyPrefix+ip, a.guestSignupDailyLimit, dailyWindow)
}

func writeRateLimited(c echo.Context, retryAfter time.Duration) error {
	if retryAfter > 0 {
		seconds := int(retryAfter.Round(time.Second).Seconds())
		if seconds < 1 {
			seconds = 1
		}
		c.Response().Header().Set("Retry-After", strconv.Itoa(seconds))
	}
	return plainTextError(c, http.StatusTooManyRequests, "too many guest signups")
}
