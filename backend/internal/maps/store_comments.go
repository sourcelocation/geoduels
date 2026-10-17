package maps

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"geoduels/internal/storekit"
	"geoduels/pkg/entityid"
	db "geoduels/pkg/persistence/sqlc/db"
)

func (s *PGStore) ListMapComments(userID, mapID string) ([]Comment, error) {
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
	visible, err = s.q().MapCommentsVisible(ctx, db.MapCommentsVisibleParams{MapID: id, ViewerUserID: strings.TrimSpace(userID)})
	if err != nil {
		return nil, err
	}
	if !visible {
		return nil, pgx.ErrNoRows
	}
	return s.listMapComments(ctx, strings.TrimSpace(userID), canonicalID)
}

func (s *PGStore) CreateMapComment(userID, mapID string, input NewComment) (Comment, error) {
	body := sanitizeMapCommentBody(input.Body)
	if body == "" {
		return Comment{}, errors.New("comment is empty")
	}
	parentID := strings.TrimSpace(input.ParentID)
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Comment{}, err
	}
	defer tx.Rollback(ctx)
	canonicalID, _, err := resolveMapIdentity(ctx, tx, mapID)
	if err != nil {
		return Comment{}, err
	}
	mapID = canonicalID
	var visible bool
	mapUID, err := mapUUID(mapID)
	if err != nil {
		return Comment{}, err
	}
	visible, err = db.New(tx).MapCommentsVisible(ctx, db.MapCommentsVisibleParams{MapID: mapUID, ViewerUserID: strings.TrimSpace(userID)})
	if err != nil {
		return Comment{}, err
	}
	if !visible {
		return Comment{}, pgx.ErrNoRows
	}
	if parentID != "" {
		pid, err := mapUUID(parentID)
		if err != nil {
			return Comment{}, err
		}
		parentParent, err := db.New(tx).MapCommentParent(ctx, db.MapCommentParentParams{CommentID: pid, MapID: mapUID})
		if err != nil {
			return Comment{}, err
		}
		if parentParent.Valid {
			return Comment{}, errors.New("only one reply level is supported")
		}
		var dup bool
		dup, err = db.New(tx).MapCommentDuplicate(ctx, db.MapCommentDuplicateParams{MapID: mapUID, UserID: mustMapUUID(userID), ParentID: parentID, Body: pgtype.Text{String: body, Valid: true}})
		if err != nil {
			return Comment{}, err
		}
		if dup {
			return Comment{}, errors.New("duplicate comment")
		}
	} else {
		var dup bool
		dup, err := db.New(tx).MapCommentDuplicate(ctx, db.MapCommentDuplicateParams{MapID: mapUID, UserID: mustMapUUID(userID), Body: pgtype.Text{String: body, Valid: true}})
		if err != nil {
			return Comment{}, err
		}
		if dup {
			return Comment{}, errors.New("duplicate comment")
		}
	}
	id := entityid.New()
	if err := db.New(tx).InsertMapComment(ctx, db.InsertMapCommentParams{CommentID: mustMapUUID(id), MapID: mustMapUUID(mapID), ParentID: parentID, UserID: mustMapUUID(userID), Body: pgtype.Text{String: body, Valid: true}}); err != nil {
		return Comment{}, err
	}
	if err := incrementMapCommentStats(ctx, tx, strings.TrimSpace(mapID), strings.TrimSpace(userID)); err != nil {
		return Comment{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Comment{}, err
	}
	comments, err := s.ListMapComments(userID, mapID)
	if err != nil {
		return Comment{}, err
	}
	for _, comment := range flattenMapComments(comments) {
		if comment.ID == id {
			return comment, nil
		}
	}
	return Comment{}, pgx.ErrNoRows
}

func (s *PGStore) DeleteMapComment(userID, mapID, commentID string, moderator bool) error {
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

func (s *PGStore) SetMapCommentLike(userID, mapID, commentID string, liked bool) (Comment, error) {
	userID, mapID, commentID = strings.TrimSpace(userID), strings.TrimSpace(mapID), strings.TrimSpace(commentID)
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Comment{}, err
	}
	defer tx.Rollback(ctx)
	canonicalID, _, err := resolveMapIdentity(ctx, tx, mapID)
	if err != nil {
		return Comment{}, err
	}
	mapID = canonicalID
	var visible bool
	q := db.New(tx)
	visible, err = q.MapCommentLikeVisible(ctx, db.MapCommentLikeVisibleParams{CommentID: mustMapUUID(commentID), MapID: mustMapUUID(mapID), ViewerUserID: userID})
	if err != nil {
		return Comment{}, err
	}
	if !visible {
		return Comment{}, pgx.ErrNoRows
	}
	var changed bool
	if liked {
		tag, err := q.AddMapCommentLike(ctx, db.AddMapCommentLikeParams{CommentID: mustMapUUID(commentID), UserID: mustMapUUID(userID)})
		if err != nil {
			return Comment{}, err
		}
		changed = tag > 0
		if changed {
			if err := q.IncrementMapCommentLike(ctx, mustMapUUID(commentID)); err != nil {
				return Comment{}, err
			}
		}
	} else {
		tag, err := q.RemoveMapCommentLike(ctx, db.RemoveMapCommentLikeParams{CommentID: mustMapUUID(commentID), UserID: mustMapUUID(userID)})
		if err != nil {
			return Comment{}, err
		}
		changed = tag > 0
		if changed {
			if err := q.DecrementMapCommentLike(ctx, mustMapUUID(commentID)); err != nil {
				return Comment{}, err
			}
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return Comment{}, err
	}
	comments, err := s.ListMapComments(userID, mapID)
	if err != nil {
		return Comment{}, err
	}
	for _, comment := range flattenMapComments(comments) {
		if comment.ID == commentID {
			return comment, nil
		}
	}
	return Comment{}, pgx.ErrNoRows
}

func (s *PGStore) listMapComments(ctx context.Context, userID, mapID string) ([]Comment, error) {
	canModerate := s.commentModerationRights(ctx, userID)
	rows, err := s.q().ListMapComments(ctx, db.ListMapCommentsParams{MapID: mustMapUUID(mapID), ViewerUserID: userID, CanModerate: canModerate})
	if err != nil {
		return nil, err
	}
	roots := []Comment{}
	byID := map[string]int{}
	replies := map[string][]Comment{}
	for _, row := range rows {
		var item Comment
		item.ID, item.MapID, item.UserID = storekit.UUIDVal(row.ID), storekit.UUIDVal(row.MapID), storekit.UUIDVal(row.UserID)
		item.ParentID = storekit.UUIDVal(row.ParentID)
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

func flattenMapComments(items []Comment) []Comment {
	out := []Comment{}
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
	if len(runes) > maxMapCommentRunes {
		body = strings.TrimSpace(string(runes[:maxMapCommentRunes]))
	}
	return body
}
