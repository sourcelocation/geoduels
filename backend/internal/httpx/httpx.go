// Package httpx holds the HTTP plumbing shared by the services: CORS, origin
// checks, and response writers. It is Echo-based; services use Echo for
// routing.
package httpx

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"

	"github.com/labstack/echo/v4"
)

// CORS returns an Echo middleware implementing the services' CORS policy.
// The Allow-Methods header is the same superset for every service; the
// per-service routers decide what actually answers.
func CORS(next echo.HandlerFunc) echo.HandlerFunc {
	allowed := AllowedOriginsSet()
	return func(c echo.Context) error {
		r := c.Request()
		origin := strings.TrimSpace(r.Header.Get("Origin"))
		if origin != "" && (allowed["*"] || allowed[origin]) {
			if allowed["*"] {
				c.Response().Header().Set("Access-Control-Allow-Origin", "*")
			} else {
				c.Response().Header().Set("Access-Control-Allow-Origin", origin)
				c.Response().Header().Set("Access-Control-Allow-Credentials", "true")
			}
			c.Response().Header().Set("Vary", "Origin")
			c.Response().Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			c.Response().Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		}
		if r.Method == http.MethodOptions {
			return c.NoContent(http.StatusNoContent)
		}
		return next(c)
	}
}

func AllowedOriginsSet() map[string]bool {
	raw := os.Getenv("CORS_ALLOWED_ORIGINS")
	if raw == "" {
		raw = "http://localhost:24000,http://127.0.0.1:24000"
	}
	out := map[string]bool{}
	for _, s := range strings.Split(raw, ",") {
		origin := strings.TrimSpace(s)
		if origin == "" {
			continue
		}
		out[origin] = true
	}
	return out
}

// WSOriginAllowed is the gorilla/websocket CheckOrigin policy shared by the
// services that accept browser websocket connections.
func WSOriginAllowed(r *http.Request) bool {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		return true
	}
	allowed := AllowedOriginsSet()
	return allowed["*"] || allowed[origin]
}

// JSON writes an "application/json" response using the standard encoder, the
// exact shape the services served before migrating to Echo.
func JSON(c echo.Context, status int, value any) error {
	c.Response().Header().Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	c.Response().WriteHeader(status)
	return json.NewEncoder(c.Response()).Encode(value)
}

// PlainTextError mirrors http.Error, including the nosniff header.
func PlainTextError(c echo.Context, status int, msg string) error {
	c.Response().Header().Set(echo.HeaderContentType, echo.MIMETextPlainCharsetUTF8)
	c.Response().Header().Set("X-Content-Type-Options", "nosniff")
	c.Response().WriteHeader(status)
	_, err := c.Response().Write([]byte(msg + "\n"))
	return err
}
