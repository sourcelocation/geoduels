package main

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"geoduels/internal/accounts"
	"geoduels/internal/moderation"
)

// Warning routes are v1 contracts; existing moderation routes remain compatible.
func (a *api) staffWarnings(c echo.Context) error {
	actor, err := a.staffActor(c.Request())
	if err != nil {
		return staffError(c, err)
	}
	warnings, err := a.moderation.Warnings(c.Request().Context(), actor, a.resolveEntityID("user", c.Param("id")))
	if err != nil {
		return staffError(c, err)
	}
	return writeJSON(c, map[string]any{"warnings": warnings})
}

func (a *api) staffWarn(c echo.Context) error {
	actor, err := a.staffActor(c.Request())
	if err != nil {
		return staffError(c, err)
	}
	var input moderation.WarningInput
	if err := decodeJSONBody(c.Request(), &input); err != nil {
		return plainTextError(c, http.StatusBadRequest, "invalid payload")
	}
	err = a.moderation.Warn(c.Request().Context(), actor, a.resolveEntityID("user", c.Param("id")), input)
	if errors.Is(err, moderation.ErrInvalidWarning) {
		return plainTextError(c, http.StatusBadRequest, "choose a category and enter a message of 1–1000 characters; evidence must belong to this player")
	}
	if errors.Is(err, accounts.ErrGuestNickname) {
		return plainTextError(c, http.StatusBadRequest, "guests always play as Guest; their nickname cannot be reset")
	}
	if err != nil {
		return staffError(c, err)
	}
	return c.NoContent(http.StatusCreated)
}

func warningID(c echo.Context) (int64, error) {
	id, err := strconv.ParseInt(c.Param("warningId"), 10, 64)
	if err != nil || id <= 0 {
		return 0, moderation.ErrInvalidWarning
	}
	return id, nil
}

func (a *api) staffWithdrawWarning(c echo.Context) error {
	actor, err := a.staffActor(c.Request())
	if err != nil {
		return staffError(c, err)
	}
	id, err := warningID(c)
	if err != nil {
		return plainTextError(c, http.StatusBadRequest, "invalid warning id")
	}
	if err := a.moderation.WithdrawWarning(c.Request().Context(), actor, a.resolveEntityID("user", c.Param("id")), id); err != nil {
		return staffError(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}

func (a *api) userWarnings(c echo.Context) error {
	claims, _, err := a.authenticatedAccount(c.Request())
	if err != nil {
		return plainTextError(c, http.StatusUnauthorized, "unauthorized")
	}
	warnings, err := a.moderation.PlayerWarnings(c.Request().Context(), claims.Sub)
	if err != nil {
		return staffError(c, err)
	}
	return writeJSON(c, map[string]any{"warnings": warnings})
}

func (a *api) acknowledgeWarning(c echo.Context) error {
	claims, _, err := a.authenticatedAccount(c.Request())
	if err != nil {
		return plainTextError(c, http.StatusUnauthorized, "unauthorized")
	}
	id, err := warningID(c)
	if err != nil {
		return plainTextError(c, http.StatusBadRequest, "invalid warning id")
	}
	if err := a.moderation.AcknowledgeWarning(c.Request().Context(), claims.Sub, id); err != nil {
		return staffError(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}
