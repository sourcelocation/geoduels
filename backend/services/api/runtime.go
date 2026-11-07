package main

import "context"

func (a *api) close() {
	_ = a.jobs.Stop(context.Background())
	a.db.Close()
}
