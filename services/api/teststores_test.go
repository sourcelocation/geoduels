package main

import "geoduels/pkg/persistence"

// testRepositories is the aggregate narrow-repository surface used by API test
// doubles. Fakes embed it so any subset of fields can be populated; calls to
// unimplemented methods panic at the interface boundary, matching the previous
// embedded aggregate behavior.
type testRepositories interface {
	persistence.AccountRepository
	persistence.SessionRepository
	persistence.ProfileRepository
	persistence.PreferenceRepository
	persistence.BadgeRepository
	persistence.LeaderboardRepository
	persistence.MatchRepository
	persistence.ModerationRepository
	persistence.AdminRepository
	persistence.ContentRepository
	persistence.SeasonRepository
	persistence.GameplayMapRepository
	persistence.RuntimeRepository
	persistence.ChatRepository
	persistence.PartyRepository
	persistence.SocialRepository
}

// withTestRepositories populates every repository field of an api under test.
func withTestRepositories(store testRepositories) func(*api) {
	return func(a *api) {
		a.accounts = store
		a.sessions = store
		a.profiles = store
		a.preferenceStore = store
		a.badges = store
		a.leaderboardStore = store
		a.matchStore = store
		a.moderation = store
		a.admin = store
		a.content = store
		a.seasons = store
		a.gameplayMaps = store
		a.runtimeStore = store
		a.chatStore = store
		a.parties = store
		a.social = store
	}
}
