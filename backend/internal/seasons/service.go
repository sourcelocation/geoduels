package seasons

import (
	"time"

	pkgstaff "geoduels/pkg/staff"
)

type Store interface {
	GetRankedSeasonSettings() (RankedSeasonSettings, error)
	SetRankedSeasonResetRule(monthlyResetDay int) (RankedSeasonSettings, error)
	RunDueRankedSeasonReset(now time.Time) (RankedSeasonResetResult, bool, error)
}

type Service struct{ store Store }

func NewService(store Store) *Service { return &Service{store: store} }

func (s *Service) GetRankedSeasonSettings() (RankedSeasonSettings, error) {
	return s.store.GetRankedSeasonSettings()
}

// StaffSettings returns the settings for the staff configuration page.
func (s *Service) StaffSettings(actor pkgstaff.Actor) (RankedSeasonSettings, error) {
	if err := actor.RequireCap(pkgstaff.CapManageConfig); err != nil {
		return RankedSeasonSettings{}, err
	}
	return s.store.GetRankedSeasonSettings()
}

func (s *Service) SetResetRule(actor pkgstaff.Actor, monthlyResetDay int) (RankedSeasonSettings, error) {
	if err := actor.RequireCap(pkgstaff.CapManageConfig); err != nil {
		return RankedSeasonSettings{}, err
	}
	return s.store.SetRankedSeasonResetRule(monthlyResetDay)
}
