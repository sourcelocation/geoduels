package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"

	preferencesdomain "geoduels/pkg/preferences"
)

const maxPreferencesBytes = 32 * 1024

func writePreferences(w http.ResponseWriter, preferences preferencesdomain.UserPreferences) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(struct {
		Preferences json.RawMessage `json:"preferences"`
		Revision    int64           `json:"revision"`
	}{
		Preferences: preferences.Preferences,
		Revision:    preferences.Revision,
	})
}

func (a *api) updateUserPreferences(w http.ResponseWriter, r *http.Request) {
	claims, err := a.authenticatedClaims(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var req struct {
		Preferences json.RawMessage `json:"preferences"`
		Revision    int64           `json:"revision"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxPreferencesBytes)
	if err := decodeJSONBody(r, &req); err != nil || len(req.Preferences) == 0 || !json.Valid(req.Preferences) {
		http.Error(w, "invalid preferences", http.StatusBadRequest)
		return
	}
	trimmed := bytes.TrimSpace(req.Preferences)
	if len(trimmed) < 2 || trimmed[0] != '{' || trimmed[len(trimmed)-1] != '}' || req.Revision < 0 {
		http.Error(w, "invalid preferences", http.StatusBadRequest)
		return
	}
	var header struct {
		Version int `json:"version"`
	}
	if a.preferences == nil {
		http.Error(w, "preferences unavailable", http.StatusNotImplemented)
		return
	}
	if json.Unmarshal(req.Preferences, &header) != nil {
		http.Error(w, "unsupported preference version", http.StatusBadRequest)
		return
	}
	preferences, err := a.preferences.Update(r.Context(), claims.Sub, header.Version, req.Preferences, req.Revision)
	if errors.Is(err, preferencesdomain.ErrUnsupportedVersion) {
		http.Error(w, "unsupported preference version", http.StatusBadRequest)
		return
	}
	if errors.Is(err, preferencesdomain.ErrRevisionConflict) {
		http.Error(w, "preferences changed in another session", http.StatusConflict)
		return
	}
	if err != nil {
		http.Error(w, "failed to save preferences", http.StatusInternalServerError)
		return
	}
	writePreferences(w, preferences)
}
