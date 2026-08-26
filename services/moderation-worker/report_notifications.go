package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"geoduels/pkg/contracts"
	"geoduels/pkg/observability"
)

const (
	reportNotificationType     = "moderation_signal_queued"
	reportNotificationInterval = 15 * time.Second
	reportNotificationBatch    = 5
)

type discordWebhookMessage struct {
	Username string         `json:"username,omitempty"`
	Embeds   []discordEmbed `json:"embeds"`
}

type discordEmbed struct {
	Title     string              `json:"title"`
	Color     int                 `json:"color"`
	Fields    []discordEmbedField `json:"fields"`
	Timestamp string              `json:"timestamp,omitempty"`
}

type discordEmbedField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline,omitempty"`
}

func (w *worker) startReportNotificationWorker(ctx context.Context) {
	go w.runReportNotificationWorker(ctx)
}

func (w *worker) runReportNotificationWorker(ctx context.Context) {
	ticker := time.NewTicker(reportNotificationInterval)
	defer ticker.Stop()
	for {
		w.drainReportNotifications(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (w *worker) drainReportNotifications(ctx context.Context) {
	for i := 0; i < reportNotificationBatch; i++ {
		if ctx.Err() != nil {
			return
		}
		processed, err := w.processOneReportNotification(ctx)
		if err != nil {
			observability.Log("warn", "report notification processing failed", map[string]any{"error": err.Error()})
			return
		}
		if !processed {
			return
		}
	}
}

func (w *worker) processOneReportNotification(ctx context.Context) (bool, error) {
	item, ok, err := w.store.ClaimPendingNotification(reportNotificationType, time.Now())
	if err != nil || !ok {
		return false, err
	}
	var payload contracts.ModerationSignalNotificationPayload
	if err := json.Unmarshal(item.PayloadJSON, &payload); err != nil {
		return true, w.store.MarkNotificationFailed(item.ID, time.Now().Add(24*time.Hour), "invalid notification payload: "+err.Error())
	}
	settings, err := w.store.GetModerationSettings()
	if err != nil {
		return true, w.store.MarkNotificationFailed(item.ID, nextReportNotificationAttempt(item.Attempts), "load moderation settings: "+err.Error())
	}
	if settings.DiscordWebhookURL == "" {
		return true, w.store.MarkNotificationFailed(item.ID, time.Now().Add(time.Minute), "discord webhook disabled")
	}
	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	retryAfter, err := w.sendDiscordReportNotification(timeoutCtx, settings.DiscordWebhookURL, payload)
	if err != nil {
		next := nextReportNotificationAttempt(item.Attempts)
		if retryAfter > 0 {
			next = time.Now().Add(retryAfter)
		}
		return true, w.store.MarkNotificationFailed(item.ID, next, err.Error())
	}
	if err := w.store.MarkNotificationSent(item.ID); err != nil {
		return true, err
	}
	observability.Log("info", "moderation signal notification sent", map[string]any{"signal_id": payload.SignalID, "subject_user_id": payload.SubjectUserID})
	return true, nil
}

func nextReportNotificationAttempt(attempts int) time.Time {
	if attempts <= 0 {
		attempts = 1
	}
	delays := []time.Duration{
		15 * time.Second,
		30 * time.Second,
		time.Minute,
		2 * time.Minute,
		5 * time.Minute,
		10 * time.Minute,
	}
	idx := attempts - 1
	if idx >= len(delays) {
		idx = len(delays) - 1
	}
	return time.Now().Add(delays[idx])
}

func (w *worker) sendDiscordReportNotification(ctx context.Context, webhookURL string, payload contracts.ModerationSignalNotificationPayload) (time.Duration, error) {
	client := w.httpClient
	if client == nil {
		client = http.DefaultClient
	}
	body, err := json.Marshal(discordWebhookMessage{
		Username: "GeoDuels Moderation",
		Embeds: []discordEmbed{
			{
				Title: "Moderation signal needs review",
				Color: 0xff4d4f,
				Fields: []discordEmbedField{
					{Name: "Signal", Value: fmt.Sprintf("#%d", payload.SignalID), Inline: true},
					{Name: "Severity", Value: strings.ToUpper(payload.Severity), Inline: true},
					{Name: "Subject", Value: discordUserValue(payload.SubjectName, payload.SubjectUserID), Inline: false},
					{Name: "Evidence", Value: fmt.Sprintf("%s / %s", strings.ToUpper(payload.ReasonCode), strings.ToUpper(payload.EvidenceStrength)), Inline: false},
				},
				Timestamp: payload.OccurredAt.UTC().Format(time.RFC3339),
			},
		},
	})
	if err != nil {
		return 0, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return 0, nil
	}
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	retryAfter := discordRetryAfter(resp, raw)
	msg := strings.TrimSpace(string(raw))
	if msg == "" {
		msg = resp.Status
	}
	return retryAfter, fmt.Errorf("discord webhook returned %s: %s", resp.Status, msg)
}

func discordUserValue(name, userID string) string {
	name = strings.TrimSpace(name)
	userID = strings.TrimSpace(userID)
	if name == "" {
		name = userID
	}
	if userID == "" || userID == name {
		return discordFieldValue(name, "Unknown")
	}
	return discordFieldValue(name+"\n`"+userID+"`", "Unknown")
}

func discordFieldValue(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		value = fallback
	}
	if len(value) > 1000 {
		value = value[:997] + "..."
	}
	return value
}

func discordRetryAfter(resp *http.Response, rawBody []byte) time.Duration {
	if resp == nil {
		return 0
	}
	if header := strings.TrimSpace(resp.Header.Get("Retry-After")); header != "" {
		if seconds, err := strconv.ParseFloat(header, 64); err == nil && seconds > 0 {
			return time.Duration(seconds * float64(time.Second))
		}
	}
	if resp.StatusCode != http.StatusTooManyRequests {
		return 0
	}
	var body struct {
		RetryAfter float64 `json:"retry_after"`
	}
	if err := json.Unmarshal(rawBody, &body); err != nil {
		return 0
	}
	if body.RetryAfter <= 0 {
		return 0
	}
	return time.Duration(body.RetryAfter * float64(time.Second))
}
