package jobs

import (
	"time"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
)

// PeriodicJobs returns the maintenance schedule. Each job is unique until it
// finishes so multiple worker replicas cannot double-run it. River requires the
// unique states to include available, pending, running and scheduled.
func PeriodicJobs(cfg PeriodicConfig) []*river.PeriodicJob {
	unique := &river.InsertOpts{UniqueOpts: river.UniqueOpts{
		ByArgs:  true,
		ByState: []rivertype.JobState{
			rivertype.JobStateAvailable, rivertype.JobStatePending, rivertype.JobStateRetryable,
			rivertype.JobStateRunning, rivertype.JobStateScheduled,
		},
	}}
	opts := func(id string) *river.PeriodicJobOpts {
		return &river.PeriodicJobOpts{ID: id, RunOnStart: false}
	}
	every := func(d, fallback time.Duration) time.Duration {
		if d <= 0 {
			return fallback
		}
		return d
	}
	return []*river.PeriodicJob{
		river.NewPeriodicJob(river.PeriodicInterval(every(cfg.GuestCleanupInterval, time.Hour)),
			func() (river.JobArgs, *river.InsertOpts) { return GuestCleanupArgs{}, unique }, opts("guest_cleanup")),
		river.NewPeriodicJob(river.PeriodicInterval(every(cfg.StorageCleanupInterval, time.Hour)),
			func() (river.JobArgs, *river.InsertOpts) { return StorageCleanupArgs{}, unique }, opts("storage_cleanup")),
		river.NewPeriodicJob(river.PeriodicInterval(every(cfg.SeasonResetInterval, time.Minute)),
			func() (river.JobArgs, *river.InsertOpts) { return SeasonResetArgs{}, unique }, opts("season_reset")),
		river.NewPeriodicJob(river.PeriodicInterval(every(cfg.CurationInterval, time.Minute)),
			func() (river.JobArgs, *river.InsertOpts) { return CurationSweepArgs{}, unique }, opts("curation_sweep")),
	}
}
