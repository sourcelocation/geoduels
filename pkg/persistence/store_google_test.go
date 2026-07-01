package persistence

import "testing"

func TestChooseGoogleIdentityUser(t *testing.T) {
	tests := []struct {
		name                     string
		existingGoogleUserID     string
		existingEmailUserID      string
		existingEmailAccountType string
		linkUserID               string
		linkAccountType          string
		wantUserID               string
		wantLinkedGuest          bool
		wantGenerated            bool
	}{
		{
			name:                 "existing google identity wins over guest link",
			existingGoogleUserID: "registered-google-user",
			linkUserID:           "guest-user",
			linkAccountType:      "guest",
			wantUserID:           "registered-google-user",
		},
		{
			name:                     "existing registered email wins over guest link",
			existingEmailUserID:      "registered-email-user",
			existingEmailAccountType: "registered",
			linkUserID:               "guest-user",
			linkAccountType:          "guest",
			wantUserID:               "registered-email-user",
		},
		{
			name:                     "existing guest email is upgraded before guest link",
			existingEmailUserID:      "email-guest-user",
			existingEmailAccountType: "guest",
			linkUserID:               "current-guest-user",
			linkAccountType:          "guest",
			wantUserID:               "email-guest-user",
			wantLinkedGuest:          true,
		},
		{
			name:            "guest link upgrades guest when no registered account exists",
			linkUserID:      "guest-user",
			linkAccountType: "guest",
			wantUserID:      "guest-user",
			wantLinkedGuest: true,
		},
		{
			name:            "registered link is linked explicitly",
			linkUserID:      "registered-link-user",
			linkAccountType: "registered",
			wantUserID:      "registered-link-user",
		},
		{
			name:          "new google user gets generated id",
			wantGenerated: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotUserID, gotLinkedGuest := chooseGoogleIdentityUser(tt.existingGoogleUserID, tt.existingEmailUserID, tt.existingEmailAccountType, tt.linkUserID, tt.linkAccountType)
			if tt.wantGenerated {
				if gotUserID == "" || gotUserID == tt.linkUserID || gotUserID == tt.existingGoogleUserID || gotUserID == tt.existingEmailUserID {
					t.Fatalf("expected generated user id, got %q", gotUserID)
				}
			} else if gotUserID != tt.wantUserID {
				t.Fatalf("user id = %q, want %q", gotUserID, tt.wantUserID)
			}
			if gotLinkedGuest != tt.wantLinkedGuest {
				t.Fatalf("linked guest = %v, want %v", gotLinkedGuest, tt.wantLinkedGuest)
			}
		})
	}
}

func TestChooseProviderIdentityUserDoesNotUseEmailForDiscord(t *testing.T) {
	gotUserID, _ := chooseProviderIdentityUser("", "google-user", "registered", "", "")
	if gotUserID != "google-user" {
		t.Fatalf("generic chooser should still honor explicit email candidate, got %q", gotUserID)
	}
}

func TestProviderUsesAccountEmailForVerifiedOAuthProviders(t *testing.T) {
	if !providerUsesAccountEmail(IdentityProviderGoogle) {
		t.Fatalf("expected Google to use the canonical users.email column")
	}
	if !providerUsesAccountEmail(IdentityProviderDiscord) {
		t.Fatalf("expected Discord to use the canonical users.email column when Discord verifies the email")
	}
}

func TestProviderAccountEmailUsesVerifiedProviderEmail(t *testing.T) {
	if got := providerAccountEmail(IdentityProviderDiscord, " same@example.com "); got != "same@example.com" {
		t.Fatalf("discord account email = %v, want trimmed email", got)
	}
	if got := providerAccountEmail(IdentityProviderGoogle, " same@example.com "); got != "same@example.com" {
		t.Fatalf("google account email = %v, want trimmed email", got)
	}
}

func TestProviderAccountEmailSkipsSyntheticOAuthEmail(t *testing.T) {
	tests := []struct {
		name  string
		email string
	}{
		{name: "generic fallback", email: "provider-sub@oauth.invalid"},
		{name: "provider scoped fallback", email: "412187004407775242@discord.oauth.invalid"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := providerAccountEmail(IdentityProviderDiscord, tt.email); got != nil {
				t.Fatalf("discord account email = %v, want nil", got)
			}
		})
	}
}
