package persistence

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	db "geoduels/pkg/persistence/sqlc/db"
	"github.com/jackc/pgx/v5"
)

func (s *DB) GetLobbyChangelog(defaultContent LobbyChangelogContent) (LobbyChangelogContent, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	posts, err := s.ListChangelogPosts(false)
	if err == nil && len(posts) > 0 {
		post := posts[0]
		return LobbyChangelogContent{
			Eyebrow:   "Latest News",
			Title:     post.Title,
			Markdown:  post.Markdown,
			Slug:      post.Slug,
			UpdatedAt: post.UpdatedAt,
		}, nil
	}
	var raw string
	raw, err = s.db.GetSetting(ctx, "lobby_changelog")
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return defaultContent, nil
		}
		return LobbyChangelogContent{}, err
	}
	var content LobbyChangelogContent
	if err := json.Unmarshal([]byte(raw), &content); err != nil {
		return defaultContent, nil
	}
	if strings.TrimSpace(content.Eyebrow) == "" {
		content.Eyebrow = defaultContent.Eyebrow
	}
	if strings.TrimSpace(content.Title) == "" {
		content.Title = defaultContent.Title
	}
	if strings.TrimSpace(content.Markdown) == "" {
		content.Markdown = defaultContent.Markdown
	}
	return content, nil
}

func (s *DB) SetLobbyChangelog(content LobbyChangelogContent) error {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	payload, err := json.Marshal(content)
	if err != nil {
		return err
	}
	return s.db.SetSetting(ctx, db.SetSettingParams{SettingKey: "lobby_changelog", ValueJson: payload})
}

func (s *DB) ListChangelogPosts(includeUnpublished bool) ([]ChangelogPost, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	rows, err := s.db.ListChangelogPosts(ctx, includeUnpublished)
	posts := make([]ChangelogPost, len(rows))
	for i, post := range rows {
		posts[i] = ChangelogPost{ID: post.ID, Slug: post.Slug, Title: post.Title, Markdown: post.Markdown, Published: post.Published, CreatedAt: post.CreatedAt.Time, UpdatedAt: post.UpdatedAt.Time}
	}
	return posts, err
}

func (s *DB) GetChangelogPostBySlug(slug string, publishedOnly bool) (ChangelogPost, bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	post, err := s.db.GetChangelogPost(ctx, db.GetChangelogPostParams{Slug: slug, IncludeUnpublished: publishedOnly})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ChangelogPost{}, false, nil
		}
		return ChangelogPost{}, false, err
	}
	return ChangelogPost{ID: post.ID, Slug: post.Slug, Title: post.Title, Markdown: post.Markdown, Published: post.Published, CreatedAt: post.CreatedAt.Time, UpdatedAt: post.UpdatedAt.Time}, true, nil
}

func (s *DB) CreateChangelogPost(input ChangelogPostInput) (ChangelogPost, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	post, err := s.db.CreateChangelogPost(ctx, db.CreateChangelogPostParams{Slug: input.Slug, Title: input.Title, Markdown: input.Markdown, Published: input.Published})
	return ChangelogPost{ID: post.ID, Slug: post.Slug, Title: post.Title, Markdown: post.Markdown, Published: post.Published, CreatedAt: post.CreatedAt.Time, UpdatedAt: post.UpdatedAt.Time}, err
}

func (s *DB) UpdateChangelogPost(id int64, input ChangelogPostInput) (ChangelogPost, bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	post, err := s.db.UpdateChangelogPost(ctx, db.UpdateChangelogPostParams{ID: id, Slug: input.Slug, Title: input.Title, Markdown: input.Markdown, Published: input.Published})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ChangelogPost{}, false, nil
		}
		return ChangelogPost{}, false, err
	}
	return ChangelogPost{ID: post.ID, Slug: post.Slug, Title: post.Title, Markdown: post.Markdown, Published: post.Published, CreatedAt: post.CreatedAt.Time, UpdatedAt: post.UpdatedAt.Time}, true, nil
}

func (s *DB) GetModerationSettings() (ModerationSettings, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	var raw string
	raw, err := s.db.GetSetting(ctx, "moderation_settings")
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ModerationSettings{}, nil
		}
		return ModerationSettings{}, err
	}
	var settings ModerationSettings
	if err := json.Unmarshal([]byte(raw), &settings); err != nil {
		return ModerationSettings{}, nil
	}
	settings.DiscordWebhookURL = strings.TrimSpace(settings.DiscordWebhookURL)
	return settings, nil
}

func (s *DB) SetModerationSettings(settings ModerationSettings) error {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	settings.DiscordWebhookURL = strings.TrimSpace(settings.DiscordWebhookURL)
	payload, err := json.Marshal(settings)
	if err != nil {
		return err
	}
	return s.db.SetSetting(ctx, db.SetSettingParams{SettingKey: "moderation_settings", ValueJson: payload})
}

func (s *DB) GetDiscordIntegrationSettings() (DiscordIntegrationSettings, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	var raw string
	raw, err := s.db.GetSetting(ctx, "discord_integration")
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return normalizeDiscordIntegrationSettings(DiscordIntegrationSettings{}), nil
		}
		return DiscordIntegrationSettings{}, err
	}
	var settings DiscordIntegrationSettings
	if err := json.Unmarshal([]byte(raw), &settings); err != nil {
		return normalizeDiscordIntegrationSettings(DiscordIntegrationSettings{}), nil
	}
	return normalizeDiscordIntegrationSettings(settings), nil
}

func (s *DB) SetDiscordIntegrationSettings(settings DiscordIntegrationSettings) error {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	current, err := s.GetDiscordIntegrationSettings()
	if err != nil {
		return err
	}
	settings.ManagedRoleIDs = append(settings.ManagedRoleIDs, current.ManagedRoleIDs...)
	settings.ManagedRoleIDs = append(settings.ManagedRoleIDs,
		current.Elo1000RoleID,
		current.Elo1500RoleID,
		current.Elo2000RoleID,
	)
	settings = normalizeDiscordIntegrationSettings(settings)
	payload, err := json.Marshal(settings)
	if err != nil {
		return err
	}
	return s.withTx(ctx, func(tx pgx.Tx) error {
		q := s.db.WithTx(tx)
		if err := q.SetSetting(ctx, db.SetSettingParams{SettingKey: "discord_integration", ValueJson: payload}); err != nil {
			return err
		}
		return q.EnqueueDiscordSyncAll(ctx, db.EnqueueDiscordSyncAllParams{Action: DiscordSyncActionSync, Provider: IdentityProviderDiscord})
	})
}

func normalizeDiscordIntegrationSettings(settings DiscordIntegrationSettings) DiscordIntegrationSettings {
	settings.GuildID = strings.TrimSpace(settings.GuildID)
	settings.JoinsChannelID = strings.TrimSpace(settings.JoinsChannelID)
	settings.Elo1000RoleID = strings.TrimSpace(settings.Elo1000RoleID)
	settings.Elo1500RoleID = strings.TrimSpace(settings.Elo1500RoleID)
	settings.Elo2000RoleID = strings.TrimSpace(settings.Elo2000RoleID)
	seenRoleIDs := map[string]bool{}
	managedRoleIDs := make([]string, 0, len(settings.ManagedRoleIDs))
	for _, roleID := range settings.ManagedRoleIDs {
		roleID = strings.TrimSpace(roleID)
		if roleID == "" || seenRoleIDs[roleID] {
			continue
		}
		seenRoleIDs[roleID] = true
		managedRoleIDs = append(managedRoleIDs, roleID)
	}
	settings.ManagedRoleIDs = managedRoleIDs
	if settings.ReconcileIntervalMinutes <= 0 {
		settings.ReconcileIntervalMinutes = 15
	}
	return settings
}
