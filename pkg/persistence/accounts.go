package persistence

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"geoduels/pkg/contentfilter"
	db "geoduels/pkg/persistence/sqlc/db"
)

var ErrNicknameTaken = errors.New("nickname already taken")

var ErrOAuthEmailConflict = errors.New("verified email is linked to multiple accounts")

func accountNullableText(value any) pgtype.Text {
	var result pgtype.Text
	_ = result.Scan(value)
	return result
}

func chooseProviderIdentityUser(existingProviderUserID, existingEmailUserID, existingEmailAccountType, linkUserID, linkAccountType string) (string, bool) {
	if existingProviderUserID != "" {
		return existingProviderUserID, false
	}
	if existingEmailUserID != "" {
		return existingEmailUserID, existingEmailAccountType == "guest"
	}
	if linkUserID != "" && linkAccountType != "" {
		return linkUserID, linkAccountType == "guest"
	}
	return newUserID(), false
}

func chooseGoogleIdentityUser(existingGoogleUserID, existingEmailUserID, existingEmailAccountType, linkUserID, linkAccountType string) (string, bool) {
	return chooseProviderIdentityUser(existingGoogleUserID, existingEmailUserID, existingEmailAccountType, linkUserID, linkAccountType)
}

func providerUsesAccountEmail(provider string) bool {
	return provider == IdentityProviderGoogle || provider == IdentityProviderDiscord
}

func isSyntheticOAuthEmail(email string) bool {
	email = strings.TrimSpace(strings.ToLower(email))
	return strings.HasSuffix(email, "@oauth.invalid") || strings.HasSuffix(email, ".oauth.invalid")
}

func providerAccountEmail(provider, email string) any {
	email = strings.TrimSpace(email)
	if providerUsesAccountEmail(provider) && email != "" && !isSyntheticOAuthEmail(email) {
		return email
	}
	return nil
}

func lockOAuthEmail(ctx context.Context, tx pgx.Tx, email string) error {
	if email == "" || isSyntheticOAuthEmail(email) {
		return nil
	}
	// Account discovery and canonical-email claims must be serialized together.
	// A transaction-scoped advisory lock works through PgBouncer transaction pools.
	return db.New(tx).LockOAuthEmail(ctx, email)
}

func findUserByVerifiedEmail(ctx context.Context, tx pgx.Tx, email string) (string, string, error) {
	if email == "" || isSyntheticOAuthEmail(email) {
		return "", "", nil
	}
	rows, err := db.New(tx).FindUserByVerifiedEmail(ctx, email)
	if err != nil {
		return "", "", err
	}
	var userID, accountType string
	if len(rows) > 0 {
		userID, accountType = rows[0].ID.String(), string(rows[0].AccountType)
	}
	if len(rows) > 1 {
		return "", "", ErrOAuthEmailConflict
	}
	return userID, accountType, nil
}

func (s *DB) UpsertGoogleIdentity(googleSub, email, googleName, avatarURL, linkUserID string) (Identity, error) {
	return s.UpsertProviderIdentity(IdentityProviderGoogle, googleSub, email, googleName, avatarURL, linkUserID)
}

func (s *DB) UpsertProviderIdentity(provider, providerUserID, email, providerName, avatarURL, linkUserID string) (Identity, error) {
	provider = strings.TrimSpace(strings.ToLower(provider))
	if provider == "" {
		return Identity{}, errors.New("provider required")
	}
	if providerUserID == "" {
		return Identity{}, errors.New("provider subject required")
	}
	if email == "" {
		email = providerUserID + "@oauth.invalid"
	}
	if providerName == "" {
		providerName = providerUserID
	}
	providerIdentityBanned, _, err := s.IsProviderIdentityBanned(provider, providerUserID)
	if err != nil {
		return Identity{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Identity{}, err
	}
	defer tx.Rollback(ctx)
	seasonID, err := activeSeasonIDTx(ctx, tx)
	if err != nil {
		return Identity{}, err
	}
	if providerUsesAccountEmail(provider) {
		if err := lockOAuthEmail(ctx, tx, email); err != nil {
			return Identity{}, err
		}
	}

	var existingProviderUserID string
	writeQ := db.New(tx)
	existingProviderUUID, findErr := writeQ.FindProviderIdentityUser(ctx, db.FindProviderIdentityUserParams{Provider: db.GdOauthProvider(provider), ProviderUserID: providerUserID})
	if findErr == nil {
		existingProviderUserID = existingProviderUUID.String()
	}
	if findErr != nil && !errors.Is(findErr, pgx.ErrNoRows) {
		return Identity{}, findErr
	}
	// A provider ban prevents account creation/evasion, but an identity that is
	// still attached to its banned account may authenticate into that account.
	if providerIdentityBanned && existingProviderUserID == "" {
		return Identity{}, errors.New("provider identity banned")
	}
	var previousLinkedProviderUserID string
	if provider == IdentityProviderDiscord && linkUserID != "" {
		linkUUID, parseErr := profileUUID(linkUserID)
		if parseErr != nil {
			return Identity{}, parseErr
		}
		previousLinkedProviderUserID, err = writeQ.FindProviderUserIdentity(ctx, db.FindProviderUserIdentityParams{UserID: linkUUID, Provider: db.GdOauthProvider(provider)})
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return Identity{}, err
		}
	}
	var existingEmailUserID string
	var existingEmailAccountType string
	if providerUsesAccountEmail(provider) && existingProviderUserID == "" {
		existingEmailUserID, existingEmailAccountType, err = findUserByVerifiedEmail(ctx, tx, email)
		if err != nil {
			return Identity{}, err
		}
	}
	var linkAccountType string
	if existingProviderUserID == "" && existingEmailUserID == "" && linkUserID != "" {
		linkUUID, parseErr := profileUUID(linkUserID)
		if parseErr != nil {
			return Identity{}, parseErr
		}
		accountType, accountErr := writeQ.GetUserAccountType(ctx, linkUUID)
		if accountErr == nil {
			linkAccountType = string(accountType)
		}
		if accountErr != nil && !errors.Is(accountErr, pgx.ErrNoRows) {
			return Identity{}, accountErr
		}
	}
	userID, _ := chooseProviderIdentityUser(existingProviderUserID, existingEmailUserID, existingEmailAccountType, linkUserID, linkAccountType)
	userEmail := providerAccountEmail(provider, email)
	userUUID, err := profileUUID(userID)
	if err != nil {
		return Identity{}, err
	}
	if err := writeQ.UpsertRegisteredUser(ctx, db.UpsertRegisteredUserParams{ID: userUUID, Email: accountNullableText(userEmail), DisplayName: providerName, AvatarUrl: accountNullableText(nullable(avatarURL))}); err != nil {
		return Identity{}, err
	}
	identityParams := db.UpsertIdentityByProviderSubjectParams{UserID: userUUID, Provider: db.GdOauthProvider(provider), ProviderUserID: providerUserID, Email: accountNullableText(email), ProviderName: accountNullableText(providerName), AvatarUrl: accountNullableText(nullable(avatarURL))}
	if existingProviderUserID != "" {
		if err := writeQ.UpsertIdentityByProviderSubject(ctx, identityParams); err != nil {
			return Identity{}, err
		}
	} else {
		if err := writeQ.UpsertIdentityByUserProvider(ctx, db.UpsertIdentityByUserProviderParams(identityParams)); err != nil {
			return Identity{}, err
		}
	}
	if err := recordUserIdentityHistory(ctx, tx, userID, provider, providerUserID, email, providerName); err != nil {
		return Identity{}, err
	}
	if err := writeQ.EnsureAccountRank(ctx, db.EnsureAccountRankParams{UserID: userUUID, Mode: db.GdMatchMode(modeDuel), SeasonID: seasonID, Mmr: int32(initialMMR)}); err != nil {
		return Identity{}, err
	}
	if err := writeQ.EnsureAccountStats(ctx, userUUID); err != nil {
		return Identity{}, err
	}
	if err := writeQ.EnsureAccountRankedStats(ctx, db.EnsureAccountRankedStatsParams{UserID: userUUID, Mode: db.GdMatchMode(modeDuel), SeasonID: seasonID}); err != nil {
		return Identity{}, err
	}
	if provider == IdentityProviderDiscord {
		if previousLinkedProviderUserID != "" && previousLinkedProviderUserID != providerUserID {
			if err := enqueueDiscordSyncTx(ctx, tx, DiscordSyncActionCleanupRoles, previousLinkedProviderUserID); err != nil {
				return Identity{}, err
			}
		}
		if err := enqueueDiscordSyncTx(ctx, tx, DiscordSyncActionSync, providerUserID); err != nil {
			return Identity{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return Identity{}, err
	}
	return s.GetIdentity(userID)
}

func (s *DB) LinkProviderIdentity(provider, providerUserID, email, providerName, avatarURL, linkUserID string) (Identity, error) {
	provider = strings.TrimSpace(strings.ToLower(provider))
	providerUserID = strings.TrimSpace(providerUserID)
	linkUserID = strings.TrimSpace(linkUserID)
	if provider == "" {
		return Identity{}, errors.New("provider required")
	}
	if providerUserID == "" {
		return Identity{}, errors.New("provider subject required")
	}
	if linkUserID == "" {
		return Identity{}, errors.New("link user required")
	}
	if email == "" {
		email = providerUserID + "@oauth.invalid"
	}
	if providerName == "" {
		providerName = providerUserID
	}
	if banned, _, err := s.IsProviderIdentityBanned(provider, providerUserID); err != nil {
		return Identity{}, err
	} else if banned {
		return Identity{}, errors.New("provider identity banned")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Identity{}, err
	}
	defer tx.Rollback(ctx)
	seasonID, err := activeSeasonIDTx(ctx, tx)
	if err != nil {
		return Identity{}, err
	}
	if providerUsesAccountEmail(provider) {
		if err := lockOAuthEmail(ctx, tx, email); err != nil {
			return Identity{}, err
		}
	}
	writeQ := db.New(tx)
	linkUUID, err := profileUUID(linkUserID)
	if err != nil {
		return Identity{}, errors.New("link user not found")
	}

	_, err = writeQ.GetUserAccountType(ctx, linkUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Identity{}, errors.New("link user not found")
		}
		return Identity{}, err
	}

	var existingProviderUserID string
	existingProviderUUID, err := writeQ.FindProviderIdentityUser(ctx, db.FindProviderIdentityUserParams{Provider: db.GdOauthProvider(provider), ProviderUserID: providerUserID})
	if err == nil {
		existingProviderUserID = existingProviderUUID.String()
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return Identity{}, err
	}
	if existingProviderUserID != "" && existingProviderUserID != linkUserID {
		return Identity{}, errors.New("provider identity already linked")
	}
	var previousLinkedProviderUserID string
	if provider == IdentityProviderDiscord {
		previousLinkedProviderUserID, err = writeQ.FindProviderUserIdentity(ctx, db.FindProviderUserIdentityParams{UserID: linkUUID, Provider: db.GdOauthProvider(provider)})
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return Identity{}, err
		}
	}
	if providerUsesAccountEmail(provider) && email != "" && !isSyntheticOAuthEmail(email) {
		existingEmailUserID, _, err := findUserByVerifiedEmail(ctx, tx, email)
		if err != nil && !errors.Is(err, ErrOAuthEmailConflict) {
			return Identity{}, err
		}
		if errors.Is(err, ErrOAuthEmailConflict) {
			return Identity{}, errors.New("provider identity already linked")
		}
		if existingEmailUserID != "" && existingEmailUserID != linkUserID {
			return Identity{}, errors.New("provider identity already linked")
		}
	}

	userEmail := providerAccountEmail(provider, email)
	if err := writeQ.PromoteLinkedUser(ctx, db.PromoteLinkedUserParams{ID: linkUUID, Email: accountNullableText(userEmail), DisplayName: providerName, AvatarUrl: accountNullableText(nullable(avatarURL))}); err != nil {
		return Identity{}, err
	}
	if err := writeQ.UpsertIdentityByUserProvider(ctx, db.UpsertIdentityByUserProviderParams{UserID: linkUUID, Provider: db.GdOauthProvider(provider), ProviderUserID: providerUserID, Email: accountNullableText(email), ProviderName: accountNullableText(providerName), AvatarUrl: accountNullableText(nullable(avatarURL))}); err != nil {
		return Identity{}, err
	}
	if err := recordUserIdentityHistory(ctx, tx, linkUserID, provider, providerUserID, email, providerName); err != nil {
		return Identity{}, err
	}
	if err := writeQ.EnsureAccountRank(ctx, db.EnsureAccountRankParams{UserID: linkUUID, Mode: db.GdMatchMode(modeDuel), SeasonID: seasonID, Mmr: int32(initialMMR)}); err != nil {
		return Identity{}, err
	}
	if err := writeQ.EnsureAccountRankedStats(ctx, db.EnsureAccountRankedStatsParams{UserID: linkUUID, Mode: db.GdMatchMode(modeDuel), SeasonID: seasonID}); err != nil {
		return Identity{}, err
	}
	if provider == IdentityProviderDiscord {
		if previousLinkedProviderUserID != "" && previousLinkedProviderUserID != providerUserID {
			if err := enqueueDiscordSyncTx(ctx, tx, DiscordSyncActionCleanupRoles, previousLinkedProviderUserID); err != nil {
				return Identity{}, err
			}
		}
		if err := enqueueDiscordSyncTx(ctx, tx, DiscordSyncActionSync, providerUserID); err != nil {
			return Identity{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return Identity{}, err
	}
	return s.GetIdentity(linkUserID)
}

func (s *DB) GoogleIdentityExists(googleSub string) (bool, error) {
	return s.ProviderIdentityExists(IdentityProviderGoogle, googleSub)
}

func (s *DB) ProviderIdentityExists(provider, providerUserID string) (bool, error) {
	provider = strings.TrimSpace(strings.ToLower(provider))
	if provider == "" || strings.TrimSpace(providerUserID) == "" {
		return false, errors.New("provider and subject required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	exists, err := s.db.ProviderIdentityExists(ctx, db.ProviderIdentityExistsParams{Provider: db.GdOauthProvider(provider), ProviderUserID: providerUserID})
	if err != nil {
		return false, err
	}
	return exists, nil
}

func (s *DB) IsProviderIdentityBanned(provider, providerUserID string) (bool, string, error) {
	provider = strings.TrimSpace(strings.ToLower(provider))
	providerUserID = strings.TrimSpace(providerUserID)
	if provider == "" || providerUserID == "" {
		return false, "", errors.New("provider and subject required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	reason, err := s.db.ProviderIdentityBanned(ctx, db.ProviderIdentityBannedParams{Provider: db.GdOauthProvider(provider), ProviderUserID: providerUserID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, "", nil
		}
		return false, "", err
	}
	return true, reason, nil
}

func recordUserIdentityHistory(ctx context.Context, tx pgx.Tx, userID, provider, providerUserID, email, providerName string) error {
	u, err := profileUUID(userID)
	if err != nil {
		return err
	}
	return db.New(tx).RecordUserIdentityHistory(ctx, db.RecordUserIdentityHistoryParams{UserID: u, Provider: db.GdOauthProvider(provider), ProviderUserID: providerUserID, Email: pgtype.Text{String: email, Valid: true}, ProviderName: pgtype.Text{String: providerName, Valid: true}})
}

func enqueueDiscordSyncTx(ctx context.Context, tx pgx.Tx, action, discordUserID string) error {
	action = strings.TrimSpace(action)
	discordUserID = strings.TrimSpace(discordUserID)
	if action == "" || discordUserID == "" {
		return nil
	}
	return db.New(tx).EnqueueDiscordSync(ctx, db.EnqueueDiscordSyncParams{Action: db.GdDiscordSyncAction(action), DiscordUserID: discordUserID})
}

func enqueueDiscordSyncForUserTx(ctx context.Context, tx pgx.Tx, userID string) error {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil
	}
	u, err := profileUUID(userID)
	if err != nil {
		return err
	}
	discordUserIDs, err := db.New(tx).ListDiscordIdentities(ctx, db.ListDiscordIdentitiesParams{UserID: u, Provider: db.GdOauthProvider(IdentityProviderDiscord)})
	if err != nil {
		return err
	}
	for _, discordUserID := range discordUserIDs {
		if err := enqueueDiscordSyncTx(ctx, tx, DiscordSyncActionSync, discordUserID); err != nil {
			return err
		}
	}
	return nil
}

func (s *DB) UnlinkProviderIdentity(userID, provider string) (Identity, error) {
	userID = strings.TrimSpace(userID)
	provider = strings.TrimSpace(strings.ToLower(provider))
	if userID == "" || provider == "" {
		return Identity{}, errors.New("user and provider required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Identity{}, err
	}
	defer tx.Rollback(ctx)
	userUUID, err := profileUUID(userID)
	if err != nil {
		return Identity{}, err
	}
	q := db.New(tx)
	providerCount, err := q.CountUserProviders(ctx, userUUID)
	if err != nil {
		return Identity{}, err
	}
	if providerCount <= 1 {
		return Identity{}, errors.New("cannot unlink the last sign-in method")
	}
	var unlinkedProviderUserID string
	if provider == IdentityProviderDiscord {
		unlinkedProviderUserID, err = q.FindProviderUserIdentity(ctx, db.FindProviderUserIdentityParams{UserID: userUUID, Provider: db.GdOauthProvider(provider)})
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return Identity{}, err
		}
	}
	affected, err := q.DeleteUserProvider(ctx, db.DeleteUserProviderParams{UserID: userUUID, Provider: db.GdOauthProvider(provider)})
	if err != nil {
		return Identity{}, err
	}
	if affected == 0 {
		return Identity{}, errors.New("provider is not linked")
	}
	if err := q.MarkIdentityHistoryDeleted(ctx, db.MarkIdentityHistoryDeletedParams{UserID: userUUID, Provider: db.GdOauthProvider(provider)}); err != nil {
		return Identity{}, err
	}
	if provider == IdentityProviderGoogle {
		if err := q.ClearUserEmailWithoutGoogle(ctx, userUUID); err != nil {
			return Identity{}, err
		}
	}
	if provider == IdentityProviderDiscord {
		if err := enqueueDiscordSyncTx(ctx, tx, DiscordSyncActionCleanupRoles, unlinkedProviderUserID); err != nil {
			return Identity{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return Identity{}, err
	}
	return s.GetIdentity(userID)
}

func (s *DB) CreateGuestIdentity() (Identity, error) {
	userID := newUserID()
	if err := s.UpsertUser(userID, "", "Guest"); err != nil {
		return Identity{}, err
	}
	return s.GetIdentity(userID)
}

func (s *DB) GetIdentity(sub string) (Identity, error) {
	if sub == "" {
		return Identity{}, errors.New("subject required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	u, err := profileUUID(sub)
	if err != nil {
		return Identity{}, err
	}
	r, err := s.db.GetIdentity(ctx, u)
	var out Identity
	if err := err; err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Identity{}, errors.New("identity not found")
		}
		return Identity{}, err
	}
	out.Sub = r.UID
	out.Email = r.Email
	out.GoogleName = r.ProviderName
	out.AvatarURL = r.AvatarUrl
	out.NicknameRequired, _ = r.Coalesce.(bool)
	out.DisplayName = r.ProviderName_2.String
	out.AccountType = string(r.AccountType)
	out.IsAdmin = r.IsAdmin
	out.IsModerator = r.IsModerator
	out.IsBanned, _ = r.Coalesce_2.(bool)
	out.BanReason = r.BanReason
	out.ProviderName = out.GoogleName
	out.LinkedProviders, _ = s.userProviders(ctx, sub)
	out.AuthMigrationRequired = false
	out.RecoveryAvailable = false
	return out, nil
}

func (s *DB) userProviders(ctx context.Context, userID string) ([]string, error) {
	u, err := profileUUID(userID)
	if err != nil {
		return nil, err
	}
	providers, err := s.db.ListIdentityProviders(ctx, u)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(providers))
	for _, p := range providers {
		out = append(out, string(p))
	}
	return out, nil
}

func containsString(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}

func (s *DB) SetNickname(sub, displayName string) error {
	if sub == "" {
		return errors.New("subject required")
	}
	if displayName == "" {
		return errors.New("display name required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	u, err := profileUUID(sub)
	if err != nil {
		return err
	}
	tag, err := s.db.SetNickname(ctx, db.SetNicknameParams{ID: u, DisplayName: displayName})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "users_claimed_nickname_unique" {
			return ErrNicknameTaken
		}
		return err
	}
	if tag == 0 {
		return errors.New("user not found")
	}
	return nil
}

func (s *DB) SuggestNickname(sub, displayName string) (string, error) {
	base := contentfilter.NicknameSuggestionBase(displayName)
	if _, err := contentfilter.ValidateNickname(base); err != nil {
		base = "Player"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	available := func(candidate string) (bool, error) {
		var taken bool
		u, err := profileUUID(sub)
		if err != nil {
			return false, err
		}
		taken, err = s.db.NicknameTaken(ctx, db.NicknameTakenParams{ID: u, Lower: candidate})
		return !taken, err
	}
	if ok, err := available(base); err != nil {
		return "", err
	} else if ok {
		return base, nil
	}
	prefix := base
	if len(prefix) > contentfilter.MaxNicknameLength-4 {
		prefix = prefix[:contentfilter.MaxNicknameLength-4]
	}
	for range 32 {
		value, err := rand.Int(rand.Reader, big.NewInt(9000))
		if err != nil {
			return "", err
		}
		candidate := fmt.Sprintf("%s%04d", prefix, value.Int64()+1000)
		if ok, err := available(candidate); err != nil {
			return "", err
		} else if ok {
			return candidate, nil
		}
	}
	return "", errors.New("nickname suggestion unavailable")
}

func (s *DB) SetUserAdmin(userID string, isAdmin bool) error {
	if isAdmin {
		return s.GrantUserRole(userID, "admin", "", "admin toggle")
	}
	return s.RevokeUserRole(userID, "admin", "", "admin toggle")
}

func (s *DB) SetUserModerator(userID string, isModerator bool) error {
	if isModerator {
		return s.GrantUserRole(userID, "moderator", "", "moderator toggle")
	}
	return s.RevokeUserRole(userID, "moderator", "", "moderator toggle")
}
