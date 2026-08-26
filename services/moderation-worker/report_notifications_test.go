package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"geoduels/pkg/contracts"
)

func TestSendDiscordReportNotification(t *testing.T) {
	var received discordWebhookMessage
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Fatalf("content-type = %q", got)
		}
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Fatalf("decode webhook body: %v", err)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	w := &worker{httpClient: server.Client()}
	retryAfter, err := w.sendDiscordReportNotification(context.Background(), server.URL, contracts.ModerationSignalNotificationPayload{
		SignalID:         42,
		SubjectUserID:    "reported-1",
		SubjectName:      "Reported",
		Severity:         "high",
		EvidenceStrength: "substantial",
		ReasonCode:       "cheating",
		OccurredAt:       time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("send notification: %v", err)
	}
	if retryAfter != 0 {
		t.Fatalf("retryAfter = %s, want 0", retryAfter)
	}
	if received.Username != "GeoDuels Moderation" || len(received.Embeds) != 1 {
		t.Fatalf("unexpected webhook message: %+v", received)
	}
	if received.Embeds[0].Title != "Moderation signal needs review" {
		t.Fatalf("embed title = %q", received.Embeds[0].Title)
	}
}

func TestDiscordRetryAfterFromBody(t *testing.T) {
	resp := &http.Response{StatusCode: http.StatusTooManyRequests, Header: http.Header{}}
	got := discordRetryAfter(resp, []byte(`{"retry_after":1.5}`))
	if got != 1500*time.Millisecond {
		t.Fatalf("retryAfter = %s", got)
	}
}
