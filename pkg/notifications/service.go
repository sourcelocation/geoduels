package notifications

import (
	"context"
	"time"

	"geoduels/pkg/contracts"
)

type Store interface {
	ListUserNotifications(string, int) ([]contracts.UserNotification, error)
	ListNotificationInbox(string, int, int64) ([]contracts.UserNotification, error)
	MarkUserNotificationRead(string, int64) error
	MarkAllUserNotificationsRead(string) error
	ClaimPendingNotification(string, time.Time) (contracts.NotificationOutboxItem, bool, error)
	MarkNotificationSent(int64) error
	MarkNotificationFailed(int64, time.Time, string) error
}

type Service struct{ store Store }

func NewService(store Store) *Service { return &Service{store: store} }
func (s *Service) List(ctx context.Context, user string, limit int) ([]contracts.UserNotification, error) {
	_ = ctx
	return s.store.ListUserNotifications(user, limit)
}
func (s *Service) Inbox(ctx context.Context, user string, limit int, before int64) ([]contracts.UserNotification, error) {
	_ = ctx
	return s.store.ListNotificationInbox(user, limit, before)
}
func (s *Service) MarkRead(ctx context.Context, user string, id int64) error {
	_ = ctx
	return s.store.MarkUserNotificationRead(user, id)
}
func (s *Service) MarkAllRead(ctx context.Context, user string) error {
	_ = ctx
	return s.store.MarkAllUserNotificationsRead(user)
}
func (s *Service) Claim(ctx context.Context, typ string, now time.Time) (contracts.NotificationOutboxItem, bool, error) {
	_ = ctx
	return s.store.ClaimPendingNotification(typ, now)
}
func (s *Service) Sent(ctx context.Context, id int64) error {
	_ = ctx
	return s.store.MarkNotificationSent(id)
}
func (s *Service) Failed(ctx context.Context, id int64, next time.Time, reason string) error {
	_ = ctx
	return s.store.MarkNotificationFailed(id, next, reason)
}
