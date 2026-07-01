package persistence

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"geoduels/pkg/contracts"
	"geoduels/pkg/entityid"
)

func (s *pgStore) ListMapComments(userID, mapID string) ([]contracts.MapComment, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	canonicalID, _, err := resolveMapIdentity(ctx, s.pool, mapID)
	if err != nil {
		return nil, err
	}
	var visible bool
	if err := s.pool.QueryRow(ctx, `select exists(select 1 from maps where id=$1 and archived_at is null and `+mapVisibleToUserSQL("maps", 2, true)+`)`, canonicalID, strings.TrimSpace(userID)).Scan(&visible); err != nil {
		return nil, err
	}
	if !visible {
		return nil, pgx.ErrNoRows
	}
	return s.listMapComments(ctx, strings.TrimSpace(userID), canonicalID)
}

func (s *pgStore) CreateMapComment(userID, mapID string, input contracts.MapCommentCreate) (contracts.MapComment, error) {
	body := sanitizeMapCommentBody(input.Body)
	if body == "" {
		return contracts.MapComment{}, errors.New("comment is empty")
	}
	parentID := strings.TrimSpace(input.ParentID)
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return contracts.MapComment{}, err
	}
	defer tx.Rollback(ctx)
	canonicalID, _, err := resolveMapIdentity(ctx, tx, mapID)
	if err != nil {
		return contracts.MapComment{}, err
	}
	mapID = canonicalID
	var visible bool
	if err := tx.QueryRow(ctx, `select exists(select 1 from maps where id=$1 and archived_at is null and `+mapVisibleToUserSQL("maps", 2, true)+`)`, strings.TrimSpace(mapID), strings.TrimSpace(userID)).Scan(&visible); err != nil {
		return contracts.MapComment{}, err
	}
	if !visible {
		return contracts.MapComment{}, pgx.ErrNoRows
	}
	if parentID != "" {
		var parentParent sql.NullString
		if err := tx.QueryRow(ctx, `select parent_id from map_comments where id=$1 and map_id=$2 and status='visible'`, parentID, strings.TrimSpace(mapID)).Scan(&parentParent); err != nil {
			return contracts.MapComment{}, err
		}
		if parentParent.Valid && strings.TrimSpace(parentParent.String) != "" {
			return contracts.MapComment{}, errors.New("only one reply level is supported")
		}
		var dup bool
		if err := tx.QueryRow(ctx, `select exists(select 1 from map_comments where map_id=$1 and user_id=$2 and parent_id=$3 and body=$4 and created_at > now()-interval '2 minutes')`, strings.TrimSpace(mapID), strings.TrimSpace(userID), parentID, body).Scan(&dup); err != nil {
			return contracts.MapComment{}, err
		}
		if dup {
			return contracts.MapComment{}, errors.New("duplicate comment")
		}
	} else {
		var dup bool
		if err := tx.QueryRow(ctx, `select exists(select 1 from map_comments where map_id=$1 and user_id=$2 and parent_id is null and body=$3 and created_at > now()-interval '2 minutes')`, strings.TrimSpace(mapID), strings.TrimSpace(userID), body).Scan(&dup); err != nil {
			return contracts.MapComment{}, err
		}
		if dup {
			return contracts.MapComment{}, errors.New("duplicate comment")
		}
	}
	id := entityid.New()
	if _, err := tx.Exec(ctx, `insert into map_comments(id,map_id,parent_id,user_id,body) values($1,$2,nullif($3,'')::uuid,$4,$5)`, id, strings.TrimSpace(mapID), parentID, strings.TrimSpace(userID), body); err != nil {
		return contracts.MapComment{}, err
	}
	if err := incrementMapCommentStats(ctx, tx, strings.TrimSpace(mapID), strings.TrimSpace(userID)); err != nil {
		return contracts.MapComment{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return contracts.MapComment{}, err
	}
	comments, err := s.ListMapComments(userID, mapID)
	if err != nil {
		return contracts.MapComment{}, err
	}
	for _, comment := range flattenMapComments(comments) {
		if comment.ID == id {
			return comment, nil
		}
	}
	return contracts.MapComment{}, pgx.ErrNoRows
}

func (s *pgStore) DeleteMapComment(userID, mapID, commentID string, moderator bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	canonicalID, _, err := resolveMapIdentity(ctx, tx, mapID)
	if err != nil {
		return err
	}
	mapID = canonicalID
	status := "deleted"
	if moderator {
		status = "moderated"
	}
	query := `update map_comments set status=$5, body='', like_count=0, deleted_by=$1, deleted_at=now(), updated_at=now()
		where id=$2 and map_id=$3 and status='visible' and ($4 or user_id=$1)`
	tag, err := tx.Exec(ctx, query, strings.TrimSpace(userID), strings.TrimSpace(commentID), strings.TrimSpace(mapID), moderator, status)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	if _, err := tx.Exec(ctx, `delete from map_comment_likes where comment_id=$1`, strings.TrimSpace(commentID)); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `update maps set comment_count=greatest(comment_count-1,0), updated_at=now() where id=$1`, strings.TrimSpace(mapID)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *pgStore) SetMapCommentLike(userID, mapID, commentID string, liked bool) (contracts.MapComment, error) {
	userID, mapID, commentID = strings.TrimSpace(userID), strings.TrimSpace(mapID), strings.TrimSpace(commentID)
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return contracts.MapComment{}, err
	}
	defer tx.Rollback(ctx)
	canonicalID, _, err := resolveMapIdentity(ctx, tx, mapID)
	if err != nil {
		return contracts.MapComment{}, err
	}
	mapID = canonicalID
	var visible bool
	if err := tx.QueryRow(ctx, `
		select exists(
			select 1
			from map_comments c
			join maps m on m.id=c.map_id
			where c.id=$1 and c.map_id=$2 and c.status='visible'
			  and m.archived_at is null
			  and `+mapVisibleToUserSQL("m", 3, true)+`
		)
	`, commentID, mapID, userID).Scan(&visible); err != nil {
		return contracts.MapComment{}, err
	}
	if !visible {
		return contracts.MapComment{}, pgx.ErrNoRows
	}
	var changed bool
	if liked {
		tag, err := tx.Exec(ctx, `
			insert into map_comment_likes(comment_id,user_id)
			values($1,$2)
			on conflict do nothing
		`, commentID, userID)
		if err != nil {
			return contracts.MapComment{}, err
		}
		changed = tag.RowsAffected() > 0
		if changed {
			if _, err := tx.Exec(ctx, `update map_comments set like_count=like_count+1 where id=$1`, commentID); err != nil {
				return contracts.MapComment{}, err
			}
		}
	} else {
		tag, err := tx.Exec(ctx, `delete from map_comment_likes where comment_id=$1 and user_id=$2`, commentID, userID)
		if err != nil {
			return contracts.MapComment{}, err
		}
		changed = tag.RowsAffected() > 0
		if changed {
			if _, err := tx.Exec(ctx, `update map_comments set like_count=greatest(like_count-1,0) where id=$1`, commentID); err != nil {
				return contracts.MapComment{}, err
			}
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return contracts.MapComment{}, err
	}
	comments, err := s.ListMapComments(userID, mapID)
	if err != nil {
		return contracts.MapComment{}, err
	}
	for _, comment := range flattenMapComments(comments) {
		if comment.ID == commentID {
			return comment, nil
		}
	}
	return contracts.MapComment{}, pgx.ErrNoRows
}

func (s *pgStore) listMapComments(ctx context.Context, userID, mapID string) ([]contracts.MapComment, error) {
	profile, _ := s.GetProfile(userID)
	canModerate := profile.IsAdmin || profile.IsModerator
	rows, err := s.pool.Query(ctx, `
		select c.id, c.map_id, coalesce(c.parent_id::text,''), c.user_id, coalesce(u.display_name, c.user_id::text), coalesce(u.avatar_url, ''),
		       case when c.status='visible' then c.body else '' end,
		       c.status, (c.user_id=nullif($2,'')::uuid or $3), c.like_count,
		       exists(select 1 from map_comment_likes l where l.comment_id=c.id and l.user_id=nullif($2,'')::uuid),
		       c.created_at, c.updated_at
		from map_comments c
		left join users u on u.id = c.user_id
		where c.map_id=$1
		order by coalesce(c.parent_id, c.id), c.created_at asc
		limit 300
	`, strings.TrimSpace(mapID), strings.TrimSpace(userID), canModerate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	roots := []contracts.MapComment{}
	byID := map[string]int{}
	replies := map[string][]contracts.MapComment{}
	for rows.Next() {
		var item contracts.MapComment
		if err := rows.Scan(&item.ID, &item.MapID, &item.ParentID, &item.UserID, &item.UserDisplayName, &item.AvatarURL, &item.Body, &item.Status, &item.CanDelete, &item.LikeCount, &item.Liked, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		if item.Status != "visible" {
			if item.Status == "moderated" {
				item.Body = "Comment removed by moderation."
			} else {
				item.Body = "Comment deleted."
			}
		}
		if item.ParentID == "" {
			byID[item.ID] = len(roots)
			roots = append(roots, item)
		} else {
			replies[item.ParentID] = append(replies[item.ParentID], item)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for parentID, children := range replies {
		if idx, ok := byID[parentID]; ok {
			roots[idx].Replies = children
		}
	}
	return roots, nil
}

func flattenMapComments(items []contracts.MapComment) []contracts.MapComment {
	out := []contracts.MapComment{}
	for _, item := range items {
		out = append(out, item)
		out = append(out, item.Replies...)
	}
	return out
}

func sanitizeMapCommentBody(body string) string {
	body = strings.TrimSpace(body)
	body = strings.Join(strings.Fields(body), " ")
	runes := []rune(body)
	if len(runes) > 1000 {
		body = strings.TrimSpace(string(runes[:1000]))
	}
	return body
}
