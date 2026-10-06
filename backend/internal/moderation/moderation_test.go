package moderation

import (
	"context"
	"errors"
	"maps"
	"slices"
	"testing"
	"time"

	"geoduels/internal/audit"
	"geoduels/pkg/contracts"
	pkgstaff "geoduels/pkg/staff"
)

// Unused Store methods deliberately panic via the embedded nil interface.
// Tests supply state for the persistence operations exercised by real services.
type fakeStore struct {
	Store
	calls, cheats       int
	beginErr, notifyErr error
	audit               []audit.Entry
	signals             []RiskSignal
	registrationIP      string
	relatedCheater      bool
	ipBans              []string
	candidates          []RefundCandidate
	ratings             map[string]RatingState
	refunds             map[string]RefundAward
	notifications       []map[string]any
	reportEligibility   ReportEligibility
	reports             []PlayerReport
	warnings            []Warning
	resets              []string
}

func newFakeStore(beginErr error) *fakeStore {
	return &fakeStore{beginErr: beginErr, ratings: map[string]RatingState{}, refunds: map[string]RefundAward{}}
}

func (f *fakeStore) WithinTx(_ context.Context, fn func(Store) error) error {
	f.calls++
	if f.beginErr != nil {
		return f.beginErr
	}
	tx := *f
	tx.audit, tx.signals, tx.ipBans = slices.Clone(f.audit), slices.Clone(f.signals), slices.Clone(f.ipBans)
	tx.ratings, tx.refunds = maps.Clone(f.ratings), maps.Clone(f.refunds)
	tx.notifications, tx.reports = slices.Clone(f.notifications), slices.Clone(f.reports)
	tx.warnings, tx.resets = slices.Clone(f.warnings), slices.Clone(f.resets)
	if err := fn(&tx); err != nil {
		return err
	}
	*f = tx
	return nil
}

func (f *fakeStore) RecordAudit(_ context.Context, entry audit.Entry) (int64, error) {
	f.audit = append(f.audit, entry)
	return int64(len(f.audit)), nil
}
func (f *fakeStore) ApplyCheatingBan(context.Context, string, string, string) (string, error) {
	f.cheats++
	return f.registrationIP, nil
}
func (f *fakeStore) HasRelatedCheater(context.Context, string, string) (bool, error) {
	return f.relatedCheater, nil
}
func (f *fakeStore) AddIPBan(_ context.Context, ip, _, _ string) error {
	f.ipBans = append(f.ipBans, ip)
	return nil
}
func (f *fakeStore) NotifyCheatingBan(context.Context, string, string, int64) error {
	if f.notifyErr != nil {
		return f.notifyErr
	}
	f.notifications = append(f.notifications, map[string]any{"type": "ban"})
	return nil
}
func (f *fakeStore) RefundCandidates(context.Context, string) ([]RefundCandidate, error) {
	return f.candidates, nil
}
func (f *fakeStore) LockRefundRating(_ context.Context, id, _ string) (RatingState, bool, error) {
	r, ok := f.ratings[id]
	return r, ok, nil
}
func (f *fakeStore) SaveRefund(_ context.Context, award RefundAward) (bool, error) {
	key := award.UserID + ":" + award.MatchID + ":" + award.CheaterID
	if _, exists := f.refunds[key]; exists {
		return false, nil
	}
	f.refunds[key] = award
	state := f.ratings[award.UserID]
	state.MMR = award.After
	f.ratings[award.UserID] = state
	return true, nil
}
func (f *fakeStore) ActiveSeasonID(context.Context) (string, error) { return "s1", nil }
func (f *fakeStore) MatchPlayerIDs(context.Context, string) ([]string, error) {
	return []string{"u1"}, nil
}
func (f *fakeStore) RecentGuessEvents(context.Context, string) ([]RiskGuessEvent, error) {
	return nil, nil
}
func (f *fakeStore) PlayerContext(context.Context, string, string) (int, int, error) {
	return 1000, 10, nil
}
func (f *fakeStore) RecordSignal(_ context.Context, _ string, signal RiskSignal, _ time.Time) error {
	f.signals = append(f.signals, signal)
	return nil
}
func (f *fakeStore) ReportEligibility(context.Context, string, string, string) (ReportEligibility, error) {
	return f.reportEligibility, nil
}
func (f *fakeStore) SaveReport(_ context.Context, report PlayerReport) (contracts.ModerationSignalCreated, error) {
	f.reports = append(f.reports, report)
	return contracts.ModerationSignalCreated{SignalID: int64(len(f.reports)), Status: "created"}, nil
}
func (f *fakeStore) UserExists(_ context.Context, id string) (bool, error) { return id == "u1", nil }
func (f *fakeStore) ResetNickname(_ context.Context, id string) (string, string, error) {
	f.resets = append(f.resets, id)
	return "Offensive", "Player00000001", nil
}
func (f *fakeStore) InsertWarning(_ context.Context, w Warning) error {
	f.warnings = append(f.warnings, w)
	return nil
}
func (f *fakeStore) ListWarnings(context.Context, string) ([]Warning, error) {
	return slices.Clone(f.warnings), nil
}
func (f *fakeStore) NotifyWarning(_ context.Context, _ string, _ int64, payload map[string]any) error {
	f.notifications = append(f.notifications, payload)
	return nil
}

type fakeRisk struct {
	enabled bool
	resp    RiskResponse
}

func (f *fakeRisk) Enabled() bool { return f.enabled }
func (f *fakeRisk) Analyze(context.Context, RiskRequest) (RiskResponse, error) {
	return f.resp, nil
}

func judgeActor() pkgstaff.Actor {
	return pkgstaff.Actor{ID: "judge-1", Roles: pkgstaff.Roles{pkgstaff.Judge}}
}

func TestTransactionFailurePropagates(t *testing.T) {
	f := newFakeStore(errors.New("begin failed"))
	if _, err := NewService(f, nil).BanCheater(context.Background(), judgeActor(), "u1", "cheating"); err == nil {
		t.Fatal("expected transaction error")
	}
	if f.cheats != 0 {
		t.Fatal("enforcement must not run when the transaction cannot be opened")
	}
}

func TestEvaluateMatchRecordsSignalsOnlyWhenEnabled(t *testing.T) {
	f := newFakeStore(nil)
	risk := &fakeRisk{}
	if err := NewService(f, risk).EvaluateMatch(context.Background(), "m1"); err != nil {
		t.Fatalf("disabled evaluation must succeed: %v", err)
	}
	if len(f.signals) != 0 || f.calls != 0 {
		t.Fatal("disabled detector must not record signals")
	}

	risk.enabled = true
	risk.resp = RiskResponse{Signals: []RiskSignal{{SubjectUserID: "u1", Severity: "high"}}}
	if err := NewService(f, risk).EvaluateMatch(context.Background(), "m1"); err != nil {
		t.Fatalf("evaluation failed: %v", err)
	}
	if len(f.signals) != 1 {
		t.Fatalf("expected one recorded signal, got %d", len(f.signals))
	}
}

func TestBanCheaterRefundsLossesOnceAndBlocksRelatedIP(t *testing.T) {
	f := newFakeStore(nil)
	now := time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC)
	f.registrationIP, f.relatedCheater = "192.0.2.1", true
	f.ratings["victim"] = RatingState{MMR: 1000, RD: 60, UpdatedAt: now}
	f.candidates = []RefundCandidate{
		{MatchID: "loss", UserID: "victim", CheaterMMR: 1000, CheaterRD: 60, OriginalDelta: -5},
		{MatchID: "win", UserID: "victim", CheaterMMR: 1000, CheaterRD: 60, OriginalDelta: 10},
	}
	svc := NewService(f, nil)
	svc.clock = func() time.Time { return now }
	result, err := svc.BanCheater(context.Background(), judgeActor(), "u1", "")
	if err != nil {
		t.Fatal(err)
	}
	if result.Refunds.RefundsIssued != 1 || result.Refunds.TotalRefunded <= 0 || result.Refunds.TotalRefunded > 5 {
		t.Fatalf("invalid refunds: %+v", result)
	}
	if !result.IPSignupBanned || len(f.ipBans) != 1 || result.Reason != "cheating" || len(f.notifications) != 1 {
		t.Fatalf("incomplete ban: %+v", result)
	}
	rating := f.ratings["victim"].MMR
	result, err = svc.BanCheater(context.Background(), judgeActor(), "u1", "again")
	if err != nil {
		t.Fatal(err)
	}
	if result.Refunds.RefundsIssued != 0 || f.ratings["victim"].MMR != rating {
		t.Fatal("repeat ban refunded the same loss twice")
	}
}

func TestBanFailureRollsBackRefundAndAudit(t *testing.T) {
	f := newFakeStore(nil)
	f.notifyErr = errors.New("notification failed")
	f.ratings["victim"] = RatingState{MMR: 1000, RD: 60, UpdatedAt: time.Now()}
	f.candidates = []RefundCandidate{{MatchID: "loss", UserID: "victim", CheaterMMR: 1000, CheaterRD: 60, OriginalDelta: -5}}
	svc := NewService(f, nil)
	result, err := svc.BanCheater(context.Background(), judgeActor(), "u1", "cheating")
	if !errors.Is(err, f.notifyErr) || result != (CheatingBanSummary{}) {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if f.cheats != 0 || len(f.audit) != 0 || len(f.refunds) != 0 || f.ratings["victim"].MMR != 1000 {
		t.Fatal("failed ban leaked changes")
	}
}

func TestReportPolicyRejectsUnavailableReports(t *testing.T) {
	for _, tc := range []struct {
		name, target string
		facts        ReportEligibility
		allowed      bool
	}{
		{"self", "reporter", ReportEligibility{TargetParticipated: true}, false},
		{"absent target", "target", ReportEligibility{}, false},
		{"muted", "target", ReportEligibility{TargetParticipated: true, ReporterMuted: true}, false},
		{"allowed", "target", ReportEligibility{TargetParticipated: true}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFakeStore(nil)
			f.reportEligibility = tc.facts
			_, err := NewService(f, nil).CreateReport(context.Background(), "match", "reporter", tc.target, " HARASSMENT ", "reason")
			if (err == nil) != tc.allowed {
				t.Fatalf("unexpected error: %v", err)
			}
			if !tc.allowed && len(f.reports) != 0 {
				t.Fatal("rejected report persisted")
			}
			if tc.allowed && (len(f.reports) != 1 || f.reports[0].Category != "harassment" || f.reports[0].Score != 1.5 || f.reports[0].Severity != "medium") {
				t.Fatalf("wrong report: %+v", f.reports)
			}
		})
	}
}

func TestWarnRejectsBeforeTouchingPersistence(t *testing.T) {
	player := pkgstaff.Actor{ID: "player"}
	valid := WarningInput{Category: "chat_abuse", Message: "Keep chat civil."}
	for _, tc := range []struct {
		name  string
		actor pkgstaff.Actor
		input WarningInput
		want  error
	}{
		{"not staff", player, valid, pkgstaff.ErrForbidden},
		{"unknown category", judgeActor(), WarningInput{Category: "spam", Message: "x"}, ErrInvalidWarning},
		{"empty message", judgeActor(), WarningInput{Category: "other", Message: "  "}, ErrInvalidWarning},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFakeStore(nil)
			if err := NewService(f, nil).Warn(context.Background(), tc.actor, "u1", tc.input); !errors.Is(err, tc.want) {
				t.Fatalf("expected %v, got %v", tc.want, err)
			}
			if f.calls != 0 || len(f.warnings) != 0 {
				t.Fatal("rejected warning touched persistence")
			}
		})
	}
}

func TestWarnWithNicknameResetRecordsBothNames(t *testing.T) {
	f := newFakeStore(nil)
	input := WarningInput{Category: "inappropriate_nickname", Message: "Please pick another name.", ResetNickname: true}
	if err := NewService(f, nil).Warn(context.Background(), judgeActor(), "u1", input); err != nil {
		t.Fatal(err)
	}
	if len(f.resets) != 1 || len(f.warnings) != 1 || len(f.audit) != 1 || len(f.notifications) != 1 {
		t.Fatalf("resets=%v warnings=%v audit=%v notifications=%v", f.resets, f.warnings, f.audit, f.notifications)
	}
	w := f.warnings[0]
	if w.PreviousNickname != "Offensive" || w.ResetNickname != "Player00000001" || w.ID != 1 {
		t.Fatalf("wrong warning: %+v", w)
	}
	if f.notifications[0]["nicknameReset"] != "Player00000001" {
		t.Fatalf("notification must name the new nickname: %v", f.notifications[0])
	}
}

func TestPlayerWarningsOmitWithdrawnAndStaffDetails(t *testing.T) {
	withdrawn := time.Now()
	f := newFakeStore(nil)
	f.warnings = []Warning{
		{ID: 2, Category: "other", Message: "active", ActorName: "judge", EvidenceMessageID: "msg", PreviousNickname: "Old", ResetNickname: "New"},
		{ID: 1, Category: "other", Message: "withdrawn", WithdrawnAt: &withdrawn},
	}
	warnings, err := NewService(f, nil).PlayerWarnings(context.Background(), "u1")
	if err != nil {
		t.Fatal(err)
	}
	want := []contracts.PlayerWarning{{ID: 2, Category: "other", Message: "active", ResetNickname: "New"}}
	if !slices.Equal(warnings, want) {
		t.Fatalf("got %+v", warnings)
	}
}
