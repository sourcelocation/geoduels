package main

import (
	"net/http/httptest"
	"testing"
)

func TestUserEventsOriginAllowed(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGINS", "http://localhost:3000,https://play.example.com")

	tests := []struct {
		name   string
		host   string
		origin string
		want   bool
	}{
		{name: "missing origin", host: "localhost:8080", want: true},
		{name: "same origin", host: "localhost:8080", origin: "http://localhost:8080", want: true},
		{name: "configured local web origin", host: "localhost:8080", origin: "http://localhost:3000", want: true},
		{name: "configured production origin", host: "api.example.com", origin: "https://play.example.com", want: true},
		{name: "unconfigured origin", host: "localhost:8080", origin: "http://attacker.example", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "http://"+tt.host+"/v1/me/events/ws", nil)
			req.Host = tt.host
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}
			if got := userEventsOriginAllowed(req); got != tt.want {
				t.Fatalf("userEventsOriginAllowed() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUserEventsOriginAllowedWildcard(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGINS", "*")
	req := httptest.NewRequest("GET", "http://localhost:8080/v1/me/events/ws", nil)
	req.Header.Set("Origin", "https://preview.example.com")
	if !userEventsOriginAllowed(req) {
		t.Fatal("userEventsOriginAllowed() rejected an origin with wildcard configuration")
	}
}
