package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"geoduels/internal/badges"
	"geoduels/internal/maps"
	"geoduels/internal/matches"
	"geoduels/internal/parties"
	"geoduels/pkg/entityid"
	"geoduels/pkg/gameplay"
	"geoduels/pkg/maintenance"
	"geoduels/pkg/matchkind"
)

// Every match starts here, whether the queue paired its players, a party's owner started it, or a
// player started one alone: the players are admitted, seated with their profiles, the map is
// planned, and the match is written for a gameplay node to pick up, all in one transaction.

type startRequest struct {
	Kind   matchkind.Kind
	Seats  []matchkind.Seat
	Config gameplay.MatchConfig
	// PartyID is the party the match was started from.
	PartyID string
	// MapAccessUserID is whose private maps the match may play.
	MapAccessUserID string
	ReturnTarget    *matches.ReturnTarget
}

// admissionError says why a player may not play now.
type admissionError struct {
	UserID string
	Name   string
	Reason string
}

func (e *admissionError) Error() string {
	if e.Name != "" {
		return e.Name + ": " + e.Reason
	}
	return e.Reason
}

func (a *api) maintenanceStatus(ctx context.Context) (maintenance.Status, error) {
	readCtx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	return maintenance.Read(readCtx, a.db.Pool())
}

// admit checks that every player may play a match of this kind now.
func (a *api) admit(ctx context.Context, spec matchkind.Spec, userIDs []string) error {
	status, err := a.maintenanceStatus(ctx)
	if err != nil {
		return err
	}
	if status.PlayBlocked() {
		return &admissionError{Reason: maintenancePlayMessage(status)}
	}
	for _, userID := range userIDs {
		identity, err := a.accounts.GetIdentity(userID)
		if err != nil {
			return &admissionError{UserID: userID, Reason: "account unavailable"}
		}
		refuse := func(reason string) error {
			return &admissionError{UserID: userID, Name: identity.DisplayName, Reason: reason}
		}
		switch {
		case identity.IsBanned:
			return refuse("account is banned")
		case identity.NicknameRequired:
			return refuse("nickname required")
		case identity.AuthMigrationRequired:
			return refuse("connect discord to continue")
		case identity.IsGuest && !spec.GuestsAllow:
			return refuse("account required")
		}
	}
	return nil
}

// startedMatch is what its players and party are told once the match is committed.
type startedMatch struct {
	summary activeMatchSummary
	players []string
	partyID string
}

// startMatch starts a match in its own transaction, and tells everyone.
func (a *api) startMatch(ctx context.Context, req startRequest) (string, error) {
	match, err := a.startMatchIn(ctx, nil, req)
	if err != nil {
		return "", err
	}
	a.tellMatchStarted(ctx, match)
	return match.summary.MatchID, nil
}

func (a *api) tellMatchStarted(ctx context.Context, match startedMatch) {
	a.tell(match.players, matchStarted{Match: match.summary})
	a.tellParty(ctx, match.partyID)
	for _, userID := range match.players {
		a.tellPresenceChanged(ctx, userID)
	}
}

// startMatchIn starts a match in tx, or in a transaction of its own when tx is nil. The caller tells
// people, once the transaction has committed.
func (a *api) startMatchIn(ctx context.Context, tx pgx.Tx, req startRequest) (startedMatch, error) {
	spec, ok := matchkind.Lookup(req.Kind)
	if !ok {
		return startedMatch{}, fmt.Errorf("unknown match kind %q", req.Kind)
	}
	if err := spec.CheckSeats(req.Seats); err != nil {
		return startedMatch{}, err
	}
	userIDs := make([]string, len(req.Seats))
	for i, seat := range req.Seats {
		userIDs[i] = seat.UserID
	}
	if err := a.admit(ctx, spec, userIDs); err != nil {
		return startedMatch{}, err
	}
	matchID := entityid.New()
	params := matches.StartParams{
		MatchID: matchID, Kind: req.Kind, Config: req.Config,
		PartyID: req.PartyID, ReturnTarget: req.ReturnTarget,
	}
	for _, seat := range req.Seats {
		profile, err := a.profiles.GetProfile(seat.UserID)
		if err != nil {
			return startedMatch{}, fmt.Errorf("profile unavailable: %w", err)
		}
		if profile.DisplayName == "" {
			profile.DisplayName = seat.UserID
		}
		if spec.Rated && params.SeasonID == "" {
			params.SeasonID = profile.SeasonID
		}
		params.Seats = append(params.Seats, matches.Seat{Team: seat.Team, Profile: gameplay.PlayerProfile{
			UserID: seat.UserID, DisplayName: profile.DisplayName, AvatarURL: profile.AvatarURL,
			MMR: profile.MMR, RatingRD: profile.RatingRD, RankedGamesPlayed: profile.RankedGamesPlayed,
			IsGuest: profile.IsGuest, IsAdmin: profile.IsAdmin, SelectedBadge: matchBadge(profile.SelectedBadge),
		}})
	}
	params.Plan = func(ctx context.Context, tx pgx.Tx) (matches.Plan, error) {
		plan, err := a.mapsStore.PlanMatchTx(ctx, tx, maps.MatchPlanRequest{
			MatchID: matchID, Spec: spec, Config: req.Config, MapAccessUserID: req.MapAccessUserID, Players: userIDs,
		})
		return matches.Plan{Config: plan.Config, MapID: plan.MapID}, err
	}
	params.After = func(ctx context.Context, tx pgx.Tx) error {
		if req.PartyID != "" {
			return parties.MarkMatchStartedTx(ctx, tx, req.PartyID, matchID)
		}
		return nil
	}
	var err error
	if tx == nil {
		err = a.matchStore.Start(ctx, params)
	} else {
		err = a.matchStore.StartTx(ctx, tx, params)
	}
	if err != nil {
		return startedMatch{}, err
	}
	return startedMatch{
		summary: activeMatchSummary{MatchID: matchID, Kind: req.Kind, Mode: spec.Mode, Status: matchViewStarting},
		players: userIDs, partyID: req.PartyID,
	}, nil
}

// matchBadge is the badge a player shows in a match, as it is when the match starts.
func matchBadge(b *badges.Badge) *gameplay.Badge {
	if b == nil {
		return nil
	}
	return &gameplay.Badge{
		ID: b.ID, Kind: b.Kind, Label: b.Label, Description: b.Description, ImageURL: b.ImageURL, Rarity: b.Rarity,
		Level: b.Level, MaxLevel: b.MaxLevel, Extra: b.Extra, Owned: b.Owned, Unobtainable: b.Unobtainable,
	}
}

// startErrorMessage is what a player is told when a match could not start.
func (a *api) startErrorMessage(err error) string {
	var admission *admissionError
	if errors.As(err, &admission) {
		return admission.Error()
	}
	var conflict *matches.ConflictError
	if errors.As(err, &conflict) {
		name := conflict.UserID
		if identity, err := a.accounts.GetIdentity(conflict.UserID); err == nil && strings.TrimSpace(identity.DisplayName) != "" {
			name = identity.DisplayName
		}
		if conflict.Kind == "" {
			return name + " is already in a match"
		}
		return name + " is already in a " + strings.ReplaceAll(string(conflict.Kind), "_", " ") + " match"
	}
	return "match could not start"
}
