package persistence

import (
	"os"
	"strings"
	"testing"
)

func TestPartyInvitationUpsertHasUniqueConflictTarget(t *testing.T) {
	query, err := os.ReadFile("../../db/queries/social.sql")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(query), "on conflict(party_id,recipient_user_id) where status='pending'") {
		t.Fatal("party invitation upsert must conflict on the pending party/recipient pair")
	}
	if !strings.Contains(string(query), "ListOutgoingPartyInvitations") {
		t.Fatal("friends-page party invite status needs ListOutgoingPartyInvitations")
	}
	if !strings.Contains(string(query), "GetPendingPartyInvitation") {
		t.Fatal("party invite resend cooldown needs GetPendingPartyInvitation")
	}

	migration, err := os.ReadFile("../../db/migrations/002003_party_invitation_active_unique.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(migration), "create unique index idx_party_invitations_active") {
		t.Fatal("party invitation upsert requires a unique pending (party_id, recipient_user_id) index")
	}
}

func TestSocialRankJoinsRequireSeason(t *testing.T) {
	query, err := os.ReadFile("../../db/queries/social.sql")
	if err != nil {
		t.Fatal(err)
	}

	sql := string(query)
	rankJoins := strings.Count(sql, "left join ranks r")
	seasonScopedRankJoins := strings.Count(sql, "and r.season_id=sqlc.arg(season_id)")
	if rankJoins == 0 {
		t.Fatal("social queries must include rank joins")
	}
	if seasonScopedRankJoins != rankJoins {
		t.Fatalf("all social rank joins must select exactly one season: got %d scoped joins for %d rank joins", seasonScopedRankJoins, rankJoins)
	}
}
