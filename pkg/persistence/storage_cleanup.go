package persistence

import (
	"context"
	"time"
)

const storageCleanupAdvisoryLock int64 = 0x47444d41494e54

func (s *pgStore) CleanupStorage(batchSize int) (StorageCleanupResult, error) {
	if batchSize <= 0 {
		batchSize = 1000
	}
	if batchSize > 10_000 {
		batchSize = 10_000
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return StorageCleanupResult{}, err
	}
	defer tx.Rollback(ctx)

	var locked bool
	if err := tx.QueryRow(ctx, `select pg_try_advisory_xact_lock($1)`, storageCleanupAdvisoryLock).Scan(&locked); err != nil {
		return StorageCleanupResult{}, err
	}
	if !locked {
		return StorageCleanupResult{}, nil
	}

	var out StorageCleanupResult
	rows, err := tx.Query(ctx, `
		select match_id,replay_json::text
		from match_history
		where replay_zstd is null
		  and replay_json is not null
		  and (replay_expires_at is null or replay_expires_at > now())
		order by ended_at desc
		limit $1
		for update skip locked
	`, min(batchSize, 1000))
	if err != nil {
		return out, err
	}
	type legacyReplay struct {
		matchID string
		raw     []byte
	}
	legacy := make([]legacyReplay, 0, min(batchSize, 1000))
	for rows.Next() {
		var item legacyReplay
		if err := rows.Scan(&item.matchID, &item.raw); err != nil {
			rows.Close()
			return out, err
		}
		legacy = append(legacy, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return out, err
	}
	rows.Close()
	for _, item := range legacy {
		compressed, sum, err := compressReplay(item.raw)
		if err != nil {
			return out, err
		}
		tag, err := tx.Exec(ctx, `
			update match_history
			set replay_zstd=$2,replay_codec=$3,replay_schema_version=$4,
			    replay_uncompressed_bytes=$5,replay_sha256=$6,replay_json=null
			where match_id=$1 and replay_zstd is null
		`, item.matchID, compressed, replayCodecZstd, replaySchemaVersion, len(item.raw), sum[:])
		if err != nil {
			return out, err
		}
		out.ReplaysCompressed += tag.RowsAffected()
	}
	run := func(dest *int64, query string, args ...any) error {
		tag, err := tx.Exec(ctx, query, args...)
		if err != nil {
			return err
		}
		*dest = tag.RowsAffected()
		return nil
	}
	if err := run(&out.ExpiredReplays, `
		with expired as (
			select match_id from match_history
			where replay_expires_at <= now()
			  and (replay_zstd is not null or replay_json is not null)
			order by replay_expires_at
			limit $1
			for update skip locked
		)
		update match_history h
		set replay_zstd=null, replay_json=null,
		    replay_codec=null, replay_schema_version=null,
		    replay_uncompressed_bytes=null, replay_sha256=null
		from expired e where h.match_id=e.match_id
	`, batchSize); err != nil {
		return out, err
	}
	if err := run(&out.MatchPlans, `
		delete from match_round_plans where ctid in (
			select p.ctid from match_round_plans p
			join match_history h on h.match_id=p.match_id
			where h.ended_at < now()-interval '1 hour'
			limit $1
		)
	`, batchSize); err != nil {
		return out, err
	}
	if err := run(&out.MatchSessions, `
		delete from match_sessions where match_id in (
			select match_id from match_sessions
			where state='ended' and ended_at < now()-interval '1 hour'
			order by ended_at limit $1
		)
	`, batchSize); err != nil {
		return out, err
	}
	if err := run(&out.RuntimeMatches, `
		delete from runtime_matches where id in (
			select id from runtime_matches
			where ended_at < now()-interval '1 hour'
			order by ended_at limit $1
		)
	`, batchSize); err != nil {
		return out, err
	}
	if err := run(&out.ChatMessages, `
		delete from chat_messages where id in (
			select id from chat_messages
			where created_at < now()-interval '7 days'
			order by created_at limit $1
		)
	`, batchSize); err != nil {
		return out, err
	}
	if err := run(&out.ChatConversations, `
		delete from chat_conversations c where c.id in (
			select c2.id from chat_conversations c2
			where not exists(select 1 from chat_messages m where m.conversation_id=c2.id)
			limit $1
		)
	`, batchSize); err != nil {
		return out, err
	}
	if err := run(&out.AuthSessions, `
		delete from auth_sessions where id in (
			select id from auth_sessions
			where (expires_at < now()-interval '24 hours')
			   or (revoked_at < now()-interval '24 hours')
			order by coalesce(revoked_at,expires_at) limit $1
		)
	`, batchSize); err != nil {
		return out, err
	}
	if err := run(&out.Parties, `
		delete from parties where id in (
			select id from parties
			where state in ('closed','expired') and updated_at < now()-interval '24 hours'
			order by updated_at limit $1
		)
	`, batchSize); err != nil {
		return out, err
	}
	if err := run(&out.MapUploadEvents, `
		delete from map_upload_events where id in (
			select id from map_upload_events
			where created_at < now()-interval '24 hours'
			order by created_at limit $1
		)
	`, batchSize); err != nil {
		return out, err
	}
	if err := run(&out.MapDailyUsers, `
		delete from map_daily_users where ctid in (
			select ctid from map_daily_users where day < current_date-8 limit $1
		)
	`, batchSize); err != nil {
		return out, err
	}
	if err := run(&out.UserNotifications, `
		delete from user_notifications where id in (
			select id from user_notifications
			where (read_at is not null and read_at < now()-interval '30 days')
			   or (read_at is null and created_at < now()-interval '90 days')
			order by created_at limit $1
		)
	`, batchSize); err != nil {
		return out, err
	}
	var userEventsDeleted int64
	if err := run(&userEventsDeleted, `
		delete from user_events where (user_id,sequence) in (
			select user_id,sequence from user_events
			where created_at < now()-interval '7 days'
			order by created_at limit $1
		)
	`, batchSize); err != nil {
		return out, err
	}
	if err := run(&out.NotificationOutbox, `
		delete from notification_outbox where id in (
			select id from notification_outbox
			where sent_at < now()-interval '24 hours'
			order by sent_at limit $1
		)
	`, batchSize); err != nil {
		return out, err
	}
	if err := run(&out.DiscordSyncOutbox, `
		delete from discord_sync_outbox where id in (
			select id from discord_sync_outbox
			where processed_at < now()-interval '7 days'
			order by processed_at limit $1
		)
	`, batchSize); err != nil {
		return out, err
	}
	if err := tx.Commit(ctx); err != nil {
		return out, err
	}
	return out, nil
}

func (s *pgStore) ReconcileStaleMatchSessions(grace time.Duration, batchSize int) (int64, error) {
	if grace <= 0 {
		grace = 5 * time.Minute
	}
	if batchSize <= 0 {
		batchSize = 1000
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	rows, err := tx.Query(ctx, `
		select match_id::text
		from match_sessions
		where state='live' and lease_expires_at < now()-$1::interval
		order by lease_expires_at
		limit $2
		for update skip locked
	`, grace.String(), min(batchSize, 10_000))
	if err != nil {
		return 0, err
	}
	var matchIDs []string
	for rows.Next() {
		var matchID string
		if err := rows.Scan(&matchID); err != nil {
			rows.Close()
			return 0, err
		}
		matchIDs = append(matchIDs, matchID)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, err
	}
	rows.Close()
	if len(matchIDs) == 0 {
		return 0, tx.Commit(ctx)
	}

	if _, err := tx.Exec(ctx, `
		update match_sessions
		set state='ended', ended_at=coalesce(ended_at,now()),
		    lease_expires_at=null, updated_at=now()
		where match_id::text=any($1)
	`, matchIDs); err != nil {
		return 0, err
	}
	if _, err := tx.Exec(ctx, `
		update runtime_matches
		set state='ended', ended_at=coalesce(ended_at,now())
		where id::text=any($1)
	`, matchIDs); err != nil {
		return 0, err
	}
	if _, err := tx.Exec(ctx, `
		update parties
		set state='open',
		    last_match_id=case when active_match_id::text=any($1)
		        then active_match_id else started_match_id end,
		    active_match_id=null, started_match_id=null, updated_at=now()
		where active_match_id::text=any($1) or started_match_id::text=any($1)
	`, matchIDs); err != nil {
		return 0, err
	}
	if _, err := tx.Exec(ctx, `
		update party_members pm
		set ready=false
		from match_sessions ms
		where ms.match_id::text=any($1) and pm.party_id=ms.source_party_id
	`, matchIDs); err != nil {
		return 0, err
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return int64(len(matchIDs)), nil
}

var _ StorageMaintenance = (*pgStore)(nil)
