package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"geoduels/internal/presence"
)

// apiClient asks the API what the gateway needs to know. The API owns every answer; these calls hold
// no state, so any API replica can serve them.
type apiClient struct {
	url    string
	secret string
	http   *http.Client
}

func (c *apiClient) do(ctx context.Context, method, path, cookie string, body any) ([]byte, int, error) {
	var payload io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return nil, 0, err
		}
		payload = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.url+path, payload)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("X-Internal-Secret", c.secret)
	req.Header.Set("Content-Type", "application/json")
	if cookie != "" {
		req.Header.Set("Cookie", cookie)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, err
	}
	return data, resp.StatusCode, nil
}

func (c *apiClient) ok(ctx context.Context, method, path, cookie string, body any) ([]byte, error) {
	data, status, err := c.do(ctx, method, path, cookie, body)
	if err != nil {
		return nil, err
	}
	if status >= 300 {
		return nil, fmt.Errorf("%s %s: %d", method, path, status)
	}
	return data, nil
}

// identify says whose session the cookie holds, or empty for a visitor.
func (c *apiClient) identify(ctx context.Context, cookie string) (string, error) {
	data, err := c.ok(ctx, http.MethodGet, "/internal/live/identify", cookie, nil)
	if err != nil {
		return "", err
	}
	var out struct {
		UserID string `json:"userId"`
	}
	return out.UserID, json.Unmarshal(data, &out)
}

// connect records the connection and returns its session, ready to send.
func (c *apiClient) connect(ctx context.Context, gatewayID string, connection presence.Connection) ([]byte, error) {
	return c.ok(ctx, http.MethodPost, "/internal/live/connect", "", map[string]any{"gatewayId": gatewayID, "connection": connection})
}

func (c *apiClient) disconnect(ctx context.Context, connection presence.Connection) error {
	_, err := c.ok(ctx, http.MethodPost, "/internal/live/disconnect", "", connection)
	return err
}

func (c *apiClient) heartbeat(ctx context.Context, gatewayID string, connections []presence.Connection) error {
	_, err := c.ok(ctx, http.MethodPost, "/internal/live/heartbeat", "", map[string]any{"gatewayId": gatewayID, "connections": connections})
	return err
}

func (c *apiClient) status(ctx context.Context) ([]byte, error) {
	return c.ok(ctx, http.MethodGet, "/internal/live/status", "", nil)
}

// matchRoute is where a player's match socket goes.
type matchRoute struct {
	UserID  string `json:"userId"`
	MatchID string `json:"matchId"`
	NodeURL string `json:"nodeUrl"`
}

// route returns the match route, or the status to refuse with.
func (c *apiClient) route(ctx context.Context, matchID, cookie string) (matchRoute, int, error) {
	data, status, err := c.do(ctx, http.MethodGet, "/internal/live/match-route/"+matchID, cookie, nil)
	if err != nil {
		return matchRoute{}, http.StatusBadGateway, err
	}
	if status >= 300 {
		return matchRoute{}, status, nil
	}
	var out matchRoute
	if err := json.Unmarshal(data, &out); err != nil {
		return matchRoute{}, http.StatusBadGateway, err
	}
	return out, http.StatusOK, nil
}
