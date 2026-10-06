package moderation

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"
	"time"
)

// HTTPRiskEngine is the HTTP adapter for the private detector service.
type HTTPRiskEngine struct {
	baseURL string
	token   string
	client  *http.Client
}

// NewRiskEngineFromEnv builds the detector client, or nil when unconfigured.
func NewRiskEngineFromEnv() *HTTPRiskEngine {
	baseURL := strings.TrimRight(strings.TrimSpace(os.Getenv("RISK_ENGINE_URL")), "/")
	if baseURL == "" {
		return nil
	}
	return &HTTPRiskEngine{
		baseURL: baseURL,
		token:   strings.TrimSpace(os.Getenv("RISK_ENGINE_TOKEN")),
		client:  &http.Client{Timeout: riskEngineTimeoutFromEnv("RISK_ENGINE_TIMEOUT", 750*time.Millisecond)},
	}
}

// Enabled reports whether the detector is configured.
func (r *HTTPRiskEngine) Enabled() bool { return r != nil && r.baseURL != "" }

// Analyze implements HTTPRiskEngine.
func (r *HTTPRiskEngine) Analyze(ctx context.Context, req RiskRequest) (RiskResponse, error) {
	if !r.Enabled() {
		return RiskResponse{}, nil
	}
	body, err := json.Marshal(req)
	if err != nil {
		return RiskResponse{}, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, r.baseURL+"/v1/analyze", bytes.NewReader(body))
	if err != nil {
		return RiskResponse{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if r.token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+r.token)
	}
	resp, err := r.client.Do(httpReq)
	if err != nil {
		return RiskResponse{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return RiskResponse{}, errors.New("risk engine returned " + resp.Status)
	}
	var out RiskResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return RiskResponse{}, err
	}
	return out, nil
}

func riskEngineTimeoutFromEnv(key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}
