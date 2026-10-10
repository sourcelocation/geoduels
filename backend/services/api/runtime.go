package main

func (a *api) close() {
	if a.db != nil {
		a.db.Close()
	}
}
