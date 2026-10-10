package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"geoduels/internal/envcfg"
	"geoduels/internal/httpx"
)

func (a *api) healthLive(c echo.Context) error {
	c.Response().WriteHeader(http.StatusOK)
	_, _ = c.Response().Write([]byte("ok"))
	return nil
}

func (a *api) healthReady(c echo.Context) error {
	if a.draining.Load() {
		return plainTextError(c, http.StatusServiceUnavailable, "draining")
	}
	ctx, cancel := context.WithTimeout(c.Request().Context(), 2*time.Second)
	defer cancel()
	if err := a.db.Pool().Ping(ctx); err != nil {
		return plainTextError(c, http.StatusServiceUnavailable, "database not ready")
	}
	c.Response().WriteHeader(http.StatusOK)
	_, _ = c.Response().Write([]byte("ready"))
	return nil
}

func corsMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return httpx.CORS(next)
}

func allowedOriginsSet() map[string]bool {
	return httpx.AllowedOriginsSet()
}

func getenv(k, fallback string) string {
	return envcfg.Get(k, fallback)
}

func getenvDuration(k string, fallback time.Duration) time.Duration {
	return envcfg.Duration(k, fallback)
}

func getenvInt(k string, fallback int) int {
	return envcfg.Int(k, fallback)
}

func getenvBool(k string, fallback bool) bool {
	return envcfg.Bool(k, fallback)
}

func getenvSameSite(k string, fallback http.SameSite) http.SameSite {
	return envcfg.SameSite(k, fallback)
}

func defaultStr(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return v
}

func requiredSecret(k string, minLen int) ([]byte, error) {
	return envcfg.RequiredSecret(k, minLen)
}

func decodeJSONBody(r *http.Request, dst any) error {
	if r.Body == nil {
		return nil
	}
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	return nil
}

// writeJSONStatus keeps the previous net/http response shape: an
// "application/json" content type and the standard encoder's HTML escaping.
func writeJSONStatus(c echo.Context, status int, value any) error {
	return httpx.JSON(c, status, value)
}

func writeJSON(c echo.Context, value any) error {
	return writeJSONStatus(c, http.StatusOK, value)
}

// plainTextError mirrors http.Error, including the nosniff header.
func plainTextError(c echo.Context, status int, msg string) error {
	return httpx.PlainTextError(c, status, msg)
}

func queryLimit(r *http.Request, fallback int) int {
	value, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}
