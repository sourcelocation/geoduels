// Package notifications tells people about their account. Each notification is shown once, on
// whichever device sees it first, and that device marks it seen; there is no inbox to manage.
package notifications

import (
	"context"
	"time"

	"geoduels/pkg/contracts"
)

// Announcement is a notification together with the person it is for.
type Announcement struct {
	UserID       string
	Notification contracts.UserNotification
}

type Store interface {
	ListUserNotifications(ctx context.Context, userID string, limit int) ([]contracts.UserNotification, error)
	ListNotificationInbox(ctx context.Context, userID string, limit int, beforeID int64) ([]contracts.UserNotification, error)
	ListUnseenSince(ctx context.Context, userIDs []string, since time.Time) ([]Announcement, error)
	MarkUserNotificationRead(ctx context.Context, userID string, id int64) error
	MarkAllUserNotificationsRead(ctx context.Context, userID string) error
}

type Service struct{ store Store }

func NewService(store Store) *Service { return &Service{store: store} }

// List is what is left to show, oldest first.
func (s *Service) List(ctx context.Context, user string, limit int) ([]contracts.UserNotification, error) {
	return s.store.ListUserNotifications(ctx, user, limit)
}

// Inbox is the notification history that clients before the unseen queue still page through.
func (s *Service) Inbox(ctx context.Context, user string, limit int, before int64) ([]contracts.UserNotification, error) {
	return s.store.ListNotificationInbox(ctx, user, limit, before)
}

// UnseenSince is what any service wrote for these people since a moment, still unshown.
func (s *Service) UnseenSince(ctx context.Context, users []string, since time.Time) ([]Announcement, error) {
	if len(users) == 0 {
		return nil, nil
	}
	return s.store.ListUnseenSince(ctx, users, since)
}

// MarkRead records that a device showed the notification, so no other shows it again.
func (s *Service) MarkRead(ctx context.Context, user string, id int64) error {
	return s.store.MarkUserNotificationRead(ctx, user, id)
}

func (s *Service) MarkAllRead(ctx context.Context, user string) error {
	return s.store.MarkAllUserNotificationsRead(ctx, user)
}
