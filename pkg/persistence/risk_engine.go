package persistence

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

type riskEngineClient struct {
	baseURL string
	token   string
	client  *http.Client
}

type riskEngineAnalyzeRequest struct {
	RequestID    string                    `json:"requestId,omitempty"`
	MatchID      string                    `json:"matchId"`
	FactsVersion string                    `json:"factsVersion"`
	GeneratedAt  time.Time                 `json:"generatedAt"`
	Players      []riskEnginePlayerHistory `json:"players"`
}

type riskEnginePlayerHistory struct {
	UserID        string                 `json:"userId"`
	CurrentRating int                    `json:"currentRating,omitempty"`
	RankedGames   int                    `json:"rankedGames,omitempty"`
	Events        []riskEngineGuessEvent `json:"events"`
}

type riskEngineGuessEvent struct {
	MatchID     string    `json:"matchId"`
	RoundNumber int       `json:"roundNumber"`
	Score       int       `json:"score"`
	GuessMS     int       `json:"guessMs"`
	Evidence    float64   `json:"evidence"`
	OccurredAt  time.Time `json:"occurredAt"`
}

type riskEngineAnalyzeResponse struct {
	DetectorVersion string             `json:"detectorVersion"`
	FactsVersion    string             `json:"factsVersion"`
	Signals         []riskEngineSignal `json:"signals"`
}

type riskEngineSignal struct {
	SubjectUserID    string         `json:"subjectUserId"`
	SignalType       string         `json:"signalType"`
	DetectorKey      string         `json:"detectorKey"`
	DetectorVersion  string         `json:"detectorVersion"`
	Severity         string         `json:"severity"`
	EvidenceStrength string         `json:"evidenceStrength"`
	ReasonCode       string         `json:"reasonCode"`
	RecommendedQueue bool           `json:"recommendedQueue"`
	Score            float64        `json:"score"`
	Payload          map[string]any `json:"payload,omitempty"`
	OccurredAt       time.Time      `json:"occurredAt"`
}

func riskEngineClientFromEnv() (riskEngineClient, bool) {
	baseURL := strings.TrimRight(strings.TrimSpace(os.Getenv("RISK_ENGINE_URL")), "/")
	if baseURL == "" {
		return riskEngineClient{}, false
	}
	timeout := riskEngineTimeoutFromEnv("RISK_ENGINE_TIMEOUT", 750*time.Millisecond)
	return riskEngineClient{
		baseURL: baseURL,
		token:   strings.TrimSpace(os.Getenv("RISK_ENGINE_TOKEN")),
		client:  &http.Client{Timeout: timeout},
	}, true
}

func (c riskEngineClient) analyze(ctx context.Context, req riskEngineAnalyzeRequest) (riskEngineAnalyzeResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return riskEngineAnalyzeResponse{}, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/analyze", bytes.NewReader(body))
	if err != nil {
		return riskEngineAnalyzeResponse{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.client.Do(httpReq)
	if err != nil {
		return riskEngineAnalyzeResponse{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return riskEngineAnalyzeResponse{}, errors.New("risk engine returned " + resp.Status)
	}
	var out riskEngineAnalyzeResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return riskEngineAnalyzeResponse{}, err
	}
	return out, nil
}

func riskSignalQueues(severity string) bool {
	switch strings.ToLower(strings.TrimSpace(severity)) {
	case "high", "critical":
		return true
	default:
		return false
	}
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

func normalizeRiskSignalType(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "gameplay_integrity"
	}
	return value
}

func normalizeRiskSeverity(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "low", "medium", "high", "critical":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return "low"
	}
}

func normalizeRiskEvidenceStrength(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "weak", "limited", "substantial", "strong":
		return strings.ToLower(strings.TrimSpace(value))
	case "very_low", "low":
		return "weak"
	case "medium":
		return "limited"
	case "high":
		return "substantial"
	case "very_high":
		return "strong"
	default:
		return "weak"
	}
}

func nonempty(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}
