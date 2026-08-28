package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const defaultStripePaymentLinkURL = "https://donate.stripe.com/bJe28jfIN0BP1ps3D20oM02"

type stripeRuntimeConfig struct {
	Mode           string
	PaymentLinkURL string
	WebhookSecret  string
}

func (a *api) stripeRuntimeConfig() stripeRuntimeConfig {
	mode := strings.TrimSpace(strings.ToLower(a.stripeMode))
	switch mode {
	case "test":
		return stripeRuntimeConfig{
			Mode:           "test",
			PaymentLinkURL: strings.TrimSpace(a.stripeTestPaymentLink),
			WebhookSecret:  strings.TrimSpace(a.stripeTestWebhook),
		}
	case "live":
		paymentLink := strings.TrimSpace(a.stripeLivePaymentLink)
		if paymentLink == "" {
			paymentLink = strings.TrimSpace(a.stripeLegacyPaymentURL)
		}
		webhookSecret := strings.TrimSpace(a.stripeLiveWebhook)
		if webhookSecret == "" {
			webhookSecret = strings.TrimSpace(a.stripeLegacyWebhook)
		}
		return stripeRuntimeConfig{
			Mode:           "live",
			PaymentLinkURL: paymentLink,
			WebhookSecret:  webhookSecret,
		}
	default:
		return stripeRuntimeConfig{
			Mode:           "legacy",
			PaymentLinkURL: strings.TrimSpace(a.stripeLegacyPaymentURL),
			WebhookSecret:  strings.TrimSpace(a.stripeLegacyWebhook),
		}
	}
}

func (a *api) createSupportDonation(w http.ResponseWriter, r *http.Request) {
	claims, err := a.authenticatedClaims(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	stripeConfig := a.stripeRuntimeConfig()
	paymentLink := strings.TrimSpace(stripeConfig.PaymentLinkURL)
	if paymentLink == "" && stripeConfig.Mode == "legacy" {
		paymentLink = defaultStripePaymentLinkURL
	}
	if paymentLink == "" {
		http.Error(w, "donation unavailable: stripe "+stripeConfig.Mode+" payment link is not configured", http.StatusServiceUnavailable)
		return
	}
	ref, err := a.badges.CreateDonationRef(claims.Sub)
	if err != nil {
		http.Error(w, "donation unavailable", http.StatusInternalServerError)
		return
	}
	donationURL, err := donationURLWithRef(paymentLink, ref)
	if err != nil {
		http.Error(w, "donation unavailable", http.StatusInternalServerError)
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]string{
		"donationUrl": donationURL,
	})
}

func donationURLWithRef(rawURL, ref string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return "", err
	}
	if parsed.Scheme != "https" {
		return "", errors.New("payment link must be https")
	}
	q := parsed.Query()
	q.Set("client_reference_id", ref)
	parsed.RawQuery = q.Encode()
	return parsed.String(), nil
}

func (a *api) stripeWebhook(w http.ResponseWriter, r *http.Request) {
	stripeConfig := a.stripeRuntimeConfig()
	webhookSecret := strings.TrimSpace(stripeConfig.WebhookSecret)
	if webhookSecret == "" {
		http.Error(w, "stripe webhook unavailable", http.StatusServiceUnavailable)
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		http.Error(w, "invalid webhook", http.StatusBadRequest)
		return
	}
	if err := verifyStripeSignature(body, r.Header.Get("Stripe-Signature"), webhookSecret, time.Now()); err != nil {
		http.Error(w, "invalid signature", http.StatusBadRequest)
		return
	}
	var event struct {
		Type string `json:"type"`
		Data struct {
			Object struct {
				ClientReferenceID string `json:"client_reference_id"`
			} `json:"object"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &event); err != nil {
		http.Error(w, "invalid webhook", http.StatusBadRequest)
		return
	}
	if event.Type == "checkout.session.completed" {
		if strings.TrimSpace(event.Data.Object.ClientReferenceID) == "" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if _, err := a.badges.AwardSupporterByDonationRef(event.Data.Object.ClientReferenceID); err != nil {
			http.Error(w, "failed to award supporter", http.StatusInternalServerError)
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

func verifyStripeSignature(body []byte, header, secret string, now time.Time) error {
	var timestamp string
	var signatures []string
	for _, part := range strings.Split(header, ",") {
		key, value, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok {
			continue
		}
		switch key {
		case "t":
			timestamp = value
		case "v1":
			signatures = append(signatures, value)
		}
	}
	if timestamp == "" || len(signatures) == 0 {
		return errors.New("missing signature")
	}
	unix, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return err
	}
	eventTime := time.Unix(unix, 0)
	if now.Sub(eventTime) > 5*time.Minute || eventTime.Sub(now) > 5*time.Minute {
		return errors.New("signature timestamp outside tolerance")
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp))
	mac.Write([]byte("."))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	for _, sig := range signatures {
		if subtle.ConstantTimeCompare([]byte(expected), []byte(sig)) == 1 {
			return nil
		}
	}
	return errors.New("signature mismatch")
}
