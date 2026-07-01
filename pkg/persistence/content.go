package persistence

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

func (s *pgStore) GetLobbyChangelog(defaultContent LobbyChangelogContent) (LobbyChangelogContent, error) {
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
	err = s.pool.QueryRow(ctx, `
		select value_json::text
		from site_settings
		where key = 'lobby_changelog'
	`).Scan(&raw)
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

func (s *pgStore) SetLobbyChangelog(content LobbyChangelogContent) error {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	payload, err := json.Marshal(content)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `
		insert into site_settings(key, value_json, updated_at)
		values('lobby_changelog', $1::jsonb, now())
		on conflict (key) do update set
			value_json = excluded.value_json,
			updated_at = now()
	`, string(payload))
	return err
}

func (s *pgStore) ListChangelogPosts(includeUnpublished bool) ([]ChangelogPost, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	query := `
		select id, slug, title, markdown, published, created_at, updated_at
		from changelog_posts
		where ($1::boolean or published = true)
		order by updated_at desc, id desc
	`
	rows, err := s.pool.Query(ctx, query, includeUnpublished)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var posts []ChangelogPost
	for rows.Next() {
		var post ChangelogPost
		if err := rows.Scan(
			&post.ID,
			&post.Slug,
			&post.Title,
			&post.Markdown,
			&post.Published,
			&post.CreatedAt,
			&post.UpdatedAt,
		); err != nil {
			return nil, err
		}
		posts = append(posts, post)
	}
	return posts, rows.Err()
}

func (s *pgStore) GetChangelogPostBySlug(slug string, publishedOnly bool) (ChangelogPost, bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	var post ChangelogPost
	err := s.pool.QueryRow(ctx, `
		select id, slug, title, markdown, published, created_at, updated_at
		from changelog_posts
		where slug = $1 and ($2::boolean = false or published = true)
	`, slug, publishedOnly).Scan(
		&post.ID,
		&post.Slug,
		&post.Title,
		&post.Markdown,
		&post.Published,
		&post.CreatedAt,
		&post.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ChangelogPost{}, false, nil
		}
		return ChangelogPost{}, false, err
	}
	return post, true, nil
}

func (s *pgStore) CreateChangelogPost(input ChangelogPostInput) (ChangelogPost, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	var post ChangelogPost
	err := s.pool.QueryRow(ctx, `
		insert into changelog_posts(slug, title, markdown, published, updated_at)
		values($1, $2, $3, $4, now())
		returning id, slug, title, markdown, published, created_at, updated_at
	`, input.Slug, input.Title, input.Markdown, input.Published).Scan(
		&post.ID,
		&post.Slug,
		&post.Title,
		&post.Markdown,
		&post.Published,
		&post.CreatedAt,
		&post.UpdatedAt,
	)
	return post, err
}

func (s *pgStore) UpdateChangelogPost(id int64, input ChangelogPostInput) (ChangelogPost, bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	var post ChangelogPost
	err := s.pool.QueryRow(ctx, `
		update changelog_posts
		set slug = $2,
			title = $3,
			markdown = $4,
			published = $5,
			updated_at = now()
		where id = $1
		returning id, slug, title, markdown, published, created_at, updated_at
	`, id, input.Slug, input.Title, input.Markdown, input.Published).Scan(
		&post.ID,
		&post.Slug,
		&post.Title,
		&post.Markdown,
		&post.Published,
		&post.CreatedAt,
		&post.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ChangelogPost{}, false, nil
		}
		return ChangelogPost{}, false, err
	}
	return post, true, nil
}

func (s *pgStore) GetModerationSettings() (ModerationSettings, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	var raw string
	err := s.pool.QueryRow(ctx, `
		select value_json::text
		from site_settings
		where key = 'moderation_settings'
	`).Scan(&raw)
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

func (s *pgStore) SetModerationSettings(settings ModerationSettings) error {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	settings.DiscordWebhookURL = strings.TrimSpace(settings.DiscordWebhookURL)
	payload, err := json.Marshal(settings)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `
		insert into site_settings(key, value_json, updated_at)
		values('moderation_settings', $1::jsonb, now())
		on conflict (key) do update set
			value_json = excluded.value_json,
			updated_at = now()
	`, string(payload))
	return err
}

func (s *pgStore) GetDiscordIntegrationSettings() (DiscordIntegrationSettings, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	var raw string
	err := s.pool.QueryRow(ctx, `
		select value_json::text
		from site_settings
		where key = 'discord_integration'
	`).Scan(&raw)
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

func (s *pgStore) SetDiscordIntegrationSettings(settings DiscordIntegrationSettings) error {
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
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `
		insert into site_settings(key, value_json, updated_at)
		values('discord_integration', $1::jsonb, now())
		on conflict (key) do update set
			value_json = excluded.value_json,
			updated_at = now()
	`, string(payload)); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `
		insert into discord_sync_outbox(action, discord_user_id)
		select $1, provider_user_id
		from user_identities
		where provider = $2
		on conflict (action, discord_user_id) where processed_at is null do update set
			next_attempt_at = least(discord_sync_outbox.next_attempt_at, excluded.next_attempt_at),
			last_error = null
	`, DiscordSyncActionSync, IdentityProviderDiscord); err != nil {
		return err
	}
	return tx.Commit(ctx)
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
