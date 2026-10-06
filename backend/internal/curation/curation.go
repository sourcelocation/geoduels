// Package curation runs Map of the Week: staff nominations and likes, and the
// weekly award to the winning map's creator.
package curation

import (
	"context"
	"errors"
	"time"

	pkgstaff "geoduels/pkg/staff"
)

var (
	// ErrUnavailable marks a map or nomination that is not eligible.
	ErrUnavailable     = errors.New("target unavailable")
	ErrInvalidTime     = errors.New("promotion time must be in the future")
	ErrScheduleChanged = errors.New("promotion schedule changed or is already due; refresh the schedule and try again")
)

// Window is the fixed weekly curation cycle.
const Window = 7 * 24 * time.Hour

type Cycle struct{ StartsAt, ClosesAt time.Time }

type Winner struct {
	MapID, Name, CreatorID string
	Likes                  int
}

// Nomination is one Map-of-the-Week submission.
type Nomination struct {
	ID           int64     `json:"id"`
	MapID        string    `json:"mapId"`
	Name         string    `json:"name"`
	ThumbnailKey string    `json:"thumbnailKey"`
	AuthorName   string    `json:"authorName"`
	Likes        int       `json:"likes"`
	Liked        bool      `json:"liked"`
	NominatedAt  time.Time `json:"nominatedAt"`
}

// Page is a page of nominations plus the closing time.
type Page struct {
	Items    []Nomination `json:"items"`
	Total    int          `json:"total"`
	Page     int          `json:"page"`
	PageSize int          `json:"pageSize"`
	ClosesAt time.Time    `json:"closesAt"`
}

type Service struct {
	store Store
	clock func() time.Time
}

func NewService(store Store) *Service {
	return &Service{store: store, clock: time.Now}
}

func (s *Service) now() time.Time { return s.clock().UTC() }

// Operations share the cycle transaction with any due award.

func (s *Service) NominateMap(ctx context.Context, actor pkgstaff.Actor, mapID string) error {
	if err := actor.RequireCap(pkgstaff.CapCurate); err != nil {
		return err
	}
	return s.withCycle(ctx, func(store Store, cycle Cycle) error {
		return store.NominateMap(ctx, actor.ID, mapID, cycle.StartsAt)
	})
}

func (s *Service) LikeNomination(ctx context.Context, actor pkgstaff.Actor, id int64, liked bool) error {
	if err := actor.RequireCap(pkgstaff.CapCurate); err != nil {
		return err
	}
	return s.withCycle(ctx, func(store Store, cycle Cycle) error {
		return store.SetNominationLike(ctx, actor.ID, id, liked, cycle.StartsAt)
	})
}

func (s *Service) ListNominations(ctx context.Context, actor pkgstaff.Actor, page int) (Page, error) {
	if err := actor.RequireCap(pkgstaff.CapCurate); err != nil {
		return Page{}, err
	}
	if page < 1 {
		page = 1
	}
	if page > 100000 {
		page = 100000
	}
	var result Page
	err := s.withCycle(ctx, func(store Store, cycle Cycle) error {
		var err error
		result, err = store.ListNominations(ctx, actor.ID, page, cycle)
		return err
	})
	if err != nil {
		return Page{}, err
	}
	return result, nil
}

// Reschedule preserves the cycle's nominations and current award. The same row
// lock used by selection prevents races with the promotion worker.
func (s *Service) Reschedule(ctx context.Context, actor pkgstaff.Actor, expectedClosesAt, closesAt time.Time) error {
	if err := actor.RequireCap(pkgstaff.CapCurate); err != nil {
		return err
	}
	return s.store.WithinTx(ctx, func(store Store) error {
		cycle, err := store.LockCycle(ctx)
		if err != nil {
			return err
		}
		now := s.now()
		if !closesAt.After(now) || !closesAt.After(cycle.StartsAt) {
			return ErrInvalidTime
		}
		if !cycle.ClosesAt.Equal(expectedClosesAt) || !cycle.ClosesAt.After(now) {
			return ErrScheduleChanged
		}
		cycle.ClosesAt = closesAt.UTC()
		return store.AdvanceCycle(ctx, cycle, time.Time{})
	})
}

// RunSweep closes any due weekly cycle. Used by the background job.
func (s *Service) RunSweep(ctx context.Context) error {
	return s.withCycle(ctx, func(Store, Cycle) error { return nil })
}

// NextCycle advances past downtime without inventing awards for unplayed weeks.
func NextCycle(closesAt, now time.Time) (time.Time, time.Time) {
	start := closesAt
	for !start.Add(Window).After(now) {
		start = start.Add(Window)
	}
	return start, start.Add(Window)
}

// withCycle serializes selection, awards and the caller's nomination
// operation in the same transaction. Candidate ranking stays a store read;
// choosing the fallback and awarding the creator are application decisions.
func (s *Service) withCycle(ctx context.Context, fn func(Store, Cycle) error) error {
	return s.store.WithinTx(ctx, func(store Store) error {
		cycle, err := store.LockCycle(ctx)
		if err != nil {
			return err
		}
		now := s.now()
		if !now.Before(cycle.ClosesAt) {
			winner, found, err := store.Winner(ctx, cycle.StartsAt)
			if err != nil {
				return err
			}
			source := "nomination"
			if !found {
				winner, found, err = store.TrendingMap(ctx)
				if err != nil {
					return err
				}
				source = "trending"
			}
			var awardedAt time.Time
			if found {
				if err := store.SaveAward(ctx, cycle.StartsAt, now, winner, source); err != nil {
					return err
				}
				if winner.CreatorID != "" {
					if err := store.AwardBadge(ctx, winner.CreatorID, "map-of-the-week"); err != nil {
						return err
					}
				}
				awardedAt = cycle.StartsAt
			}
			cycle.StartsAt, cycle.ClosesAt = NextCycle(cycle.ClosesAt, now)
			if err := store.AdvanceCycle(ctx, cycle, awardedAt); err != nil {
				return err
			}
		}
		return fn(store, cycle)
	})
}
