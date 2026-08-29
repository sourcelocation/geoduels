package persistence

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"geoduels/pkg/contracts"
	"geoduels/pkg/entityid"
	db "geoduels/pkg/persistence/sqlc/db"
)

func (s *DB) ListMapComments(userID, mapID string) ([]contracts.MapComment, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	canonicalID, _, err := resolveMapIdentity(ctx, s.pool, mapID)
	if err != nil {
		return nil, err
	}
	var visible bool
	id, err := mapUUID(canonicalID)
	if err != nil {
		return nil, err
	}
	visible, err = s.db.MapCommentsVisible(ctx, db.MapCommentsVisibleParams{MapID: id, ViewerUserID: strings.TrimSpace(userID)})
	if err != nil {
		return nil, err
	}
	if !visible {
		return nil, pgx.ErrNoRows
	}
	return s.listMapComments(ctx, strings.TrimSpace(userID), canonicalID)
}

func (s *DB) CreateMapComment(userID, mapID string, input contracts.MapCommentCreate) (contracts.MapComment, error) {
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
	mapUID, err := mapUUID(mapID)
	if err != nil {
		return contracts.MapComment{}, err
	}
	visible, err = db.New(tx).MapCommentsVisible(ctx, db.MapCommentsVisibleParams{MapID: mapUID, ViewerUserID: strings.TrimSpace(userID)})
	if err != nil {
		return contracts.MapComment{}, err
	}
	if !visible {
		return contracts.MapComment{}, pgx.ErrNoRows
	}
	if parentID != "" {
		pid, err := mapUUID(parentID)
		if err != nil {
			return contracts.MapComment{}, err
		}
		parentParent, err := db.New(tx).MapCommentParent(ctx, db.MapCommentParentParams{CommentID: pid, MapID: mapUID})
		if err != nil {
			return contracts.MapComment{}, err
		}
		if parentParent.Valid {
			return contracts.MapComment{}, errors.New("only one reply level is supported")
		}
		var dup bool
		dup, err = db.New(tx).MapCommentDuplicate(ctx, db.MapCommentDuplicateParams{MapID: mapUID, UserID: mustMapUUID(userID), ParentID: parentID, Body: pgtype.Text{String: body, Valid: true}})
		if err != nil {
			return contracts.MapComment{}, err
		}
		if dup {
			return contracts.MapComment{}, errors.New("duplicate comment")
		}
	} else {
		var dup bool
		dup, err := db.New(tx).MapCommentDuplicate(ctx, db.MapCommentDuplicateParams{MapID: mapUID, UserID: mustMapUUID(userID), Body: pgtype.Text{String: body, Valid: true}})
		if err != nil {
			return contracts.MapComment{}, err
		}
		if dup {
			return contracts.MapComment{}, errors.New("duplicate comment")
		}
	}
	id := entityid.New()
	if err := db.New(tx).InsertMapComment(ctx, db.InsertMapCommentParams{CommentID: mustMapUUID(id), MapID: mustMapUUID(mapID), ParentID: parentID, UserID: mustMapUUID(userID), Body: pgtype.Text{String: body, Valid: true}}); err != nil {
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

func (s *DB) DeleteMapComment(userID, mapID, commentID string, moderator bool) error {
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
	q := db.New(tx)
	tag, err := q.DeleteMapComment(ctx, db.DeleteMapCommentParams{DeletedBy: mustMapUUID(userID), CommentID: mustMapUUID(commentID), MapID: mustMapUUID(mapID), IsModerator: moderator, Status: db.GdCommentStatus(status)})
	if err != nil {
		return err
	}
	if tag == 0 {
		return pgx.ErrNoRows
	}
	if err := q.DeleteMapCommentLikes(ctx, mustMapUUID(commentID)); err != nil {
		return err
	}
	if err := q.DecrementMapCommentCount(ctx, mustMapUUID(mapID)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *DB) SetMapCommentLike(userID, mapID, commentID string, liked bool) (contracts.MapComment, error) {
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
	q := db.New(tx)
	visible, err = q.MapCommentLikeVisible(ctx, db.MapCommentLikeVisibleParams{CommentID: mustMapUUID(commentID), MapID: mustMapUUID(mapID), ViewerUserID: userID})
	if err != nil {
		return contracts.MapComment{}, err
	}
	if !visible {
		return contracts.MapComment{}, pgx.ErrNoRows
	}
	var changed bool
	if liked {
		tag, err := q.AddMapCommentLike(ctx, db.AddMapCommentLikeParams{CommentID: mustMapUUID(commentID), UserID: mustMapUUID(userID)})
		if err != nil {
			return contracts.MapComment{}, err
		}
		changed = tag > 0
		if changed {
			if err := q.IncrementMapCommentLike(ctx, mustMapUUID(commentID)); err != nil {
				return contracts.MapComment{}, err
			}
		}
	} else {
		tag, err := q.RemoveMapCommentLike(ctx, db.RemoveMapCommentLikeParams{CommentID: mustMapUUID(commentID), UserID: mustMapUUID(userID)})
		if err != nil {
			return contracts.MapComment{}, err
		}
		changed = tag > 0
		if changed {
			if err := q.DecrementMapCommentLike(ctx, mustMapUUID(commentID)); err != nil {
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

func (s *DB) listMapComments(ctx context.Context, userID, mapID string) ([]contracts.MapComment, error) {
	profile, _ := s.GetProfile(userID)
	canModerate := profile.IsAdmin || profile.IsModerator
	rows, err := s.db.ListMapComments(ctx, db.ListMapCommentsParams{MapID: mustMapUUID(mapID), ViewerUserID: userID, CanModerate: canModerate})
	if err != nil {
		return nil, err
	}
	roots := []contracts.MapComment{}
	byID := map[string]int{}
	replies := map[string][]contracts.MapComment{}
	for _, row := range rows {
		var item contracts.MapComment
		item.ID, item.MapID, item.UserID = uuidVal(row.ID), uuidVal(row.MapID), uuidVal(row.UserID)
		item.ParentID = uuidVal(row.ParentID)
		item.UserDisplayName = row.UserDisplayName
		item.AvatarURL = row.AvatarUrl
		item.Body = row.Body
		item.Status = string(row.Status)
		item.CanDelete, _ = row.CanDelete.(bool)
		item.LikeCount = int(row.LikeCount)
		item.Liked = row.Liked
		item.CreatedAt = row.CreatedAt.Time
		item.UpdatedAt = row.UpdatedAt.Time
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

func mustMapUUID(value string) pgtype.UUID {
	id, err := mapUUID(strings.TrimSpace(value))
	if err != nil {
		return pgtype.UUID{}
	}
	return id
}
