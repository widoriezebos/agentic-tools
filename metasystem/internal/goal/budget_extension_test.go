package goal

import (
	"strings"
	"testing"
	"time"
)

func budgetExtensionEndpointBed(t *testing.T) (Endpoint, VerbRequest, BudgetExtensionOffer, uint64) {
	t.Helper()
	endpoint, _ := fakeGoalEndpoint(t)
	return budgetExtensionSetup(t, endpoint)
}

func budgetExtensionSetup(t *testing.T, endpoint Endpoint) (Endpoint, VerbRequest, BudgetExtensionOffer, uint64) {
	t.Helper()
	req := verbReqFor(endpoint, "01J5X00000000000000000EX01", "mac-a")
	opened, err := Open(req, "earned-raise", "Ship the bounded change.", OriginMain, "Implement it.")
	if err != nil || opened.Outcome != OutcomeConfirmed {
		t.Fatalf("open: %+v %v", opened, err)
	}
	claimed, err := claimApprovedForTest(t, verbReqFor(endpoint, "01J5X00000000000000000EX02", "mac-a"), "earned-raise", testBudget())
	if err != nil || claimed.Outcome != OutcomeConfirmed {
		t.Fatalf("claim: %+v %v", claimed, err)
	}
	tree, err := loadTreeFor(endpoint, claimed.Tip)
	if err != nil {
		t.Fatal(err)
	}
	revision := tree.Live["earned-raise"].Claimed.Revision
	offer := BudgetExtensionOffer{
		EvidenceKind: "landing", EvidenceID: "1789000000-0123456789012345678901234567890123456789",
		EvidenceAt: "2026-08-20T21:30:00Z", AttemptLimitFrom: 4, AttemptLimitTo: 14,
		ReservedJobMinutesFrom: 240, ReservedJobMinutesTo: 1440,
	}
	extend := verbReqFor(endpoint, "01J5X00000000000000000EX03", "mac-a")
	extend.Now = time.Date(2026, 8, 20, 22, 5, 0, 0, time.UTC)
	return endpoint, extend, offer, revision
}

func budgetExtensionBed(t *testing.T) (string, VerbRequest, BudgetExtensionOffer, uint64) {
	t.Helper()
	_, root, _ := twoClones(t)
	seedLedger(t, root)
	endpoint, request, offer, revision := budgetExtensionSetup(t, endpointFor(root))
	return endpoint.Root, request, offer, revision
}

func TestExtendBudgetRaisesOnlyConsumptionMembersAndWritesMarker(t *testing.T) {
	t.Parallel()
	endpoint, req, offer, claimRevision := budgetExtensionEndpointBed(t)
	result, err := ExtendBudget(req, "earned-raise", offer)
	if err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("extend budget: %+v %v", result, err)
	}
	tree, err := loadTreeFor(endpoint, result.Tip)
	if err != nil {
		t.Fatal(err)
	}
	f := tree.Live["earned-raise"]
	if f.Budget.AttemptLimit != 14 || f.Budget.ReservedJobMinutesLimit != 1440 ||
		f.Budget.ElapsedLimit != "4h" || f.Budget.ActiveJobLimit != 2 || f.Budget.ReviewRoundLimit != 0 {
		t.Fatalf("extension changed the wrong tuple members: %+v", f.Budget)
	}
	if f.Claimed.Revision != claimRevision || f.Claimed.AccountingRevision != claimRevision || f.Episode != nil {
		t.Fatalf("extension rebound claim or accounting: claim=%+v episode=%+v", f.Claimed, f.Episode)
	}
	if f.BudgetExtension == nil || f.BudgetExtension.EvidenceKind != "landing" || f.BudgetExtension.Opid != req.opid() ||
		len(f.History) == 0 || f.History[len(f.History)-1].Verb != "extend-budget" {
		t.Fatalf("extension marker/history missing: marker=%+v history=%+v", f.BudgetExtension, f.History)
	}
	if err := f.ValidateApprovalRecord(); err != nil {
		t.Fatalf("pre-extension human approval did not validate against the reconstructed tuple: %v", err)
	}
	parsed, problems := ParseFile(RenderFile(f))
	if len(problems) != 0 || parsed.BudgetExtension == nil || *parsed.BudgetExtension != *f.BudgetExtension {
		t.Fatalf("BudgetExtension did not round-trip: parsed=%+v problems=%v", parsed.BudgetExtension, problems)
	}

	second := req
	second.Ulid = "01J5X00000000000000000EX04"
	secondResult, err := ExtendBudget(second, "earned-raise", offer)
	if err != nil || secondResult.Outcome == OutcomeConfirmed || !strings.Contains(secondResult.Detail, "extended once at") {
		t.Fatalf("second extension did not name the marker: %+v %v", secondResult, err)
	}
	foreign := second
	foreign.Actor.Machine = "mac-b"
	foreign.Ulid = "01J5X00000000000000000EX05"
	foreignResult, err := ExtendBudget(foreign, "earned-raise", offer)
	if err != nil || foreignResult.Outcome == OutcomeConfirmed || !strings.Contains(foreignResult.Detail, "only that pair") {
		t.Fatalf("foreign pair was not refused: %+v %v", foreignResult, err)
	}
	human := second
	human.Actor.Human = "Wido"
	if _, err := ExtendBudget(human, "earned-raise", offer); err == nil || !strings.Contains(err.Error(), "person uses goal set-budget") {
		t.Fatalf("person was not refused: %v", err)
	}
}

func TestExtendBudgetAcceptsApprovalEarlierInTheSameSecond(t *testing.T) {
	t.Parallel()
	endpoint, req, offer, _ := budgetExtensionEndpointBed(t)
	req.Now = time.Date(2026, 8, 20, 22, 0, 0, 0, time.UTC)
	result, err := ExtendBudget(req, "earned-raise", offer)
	if err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("same-second extension: %+v %v", result, err)
	}
	tree, err := loadTreeFor(endpoint, result.Tip)
	if err != nil {
		t.Fatal(err)
	}
	file := tree.Live["earned-raise"]
	if file.BudgetExtension == nil || file.Approved.At != file.BudgetExtension.At {
		t.Fatalf("fixture did not exercise same-second ordering: approved=%+v extension=%+v", file.Approved, file.BudgetExtension)
	}
	if err := file.ValidateApprovalRecord(); err != nil {
		t.Fatalf("same-second earlier approval did not validate: %v", err)
	}
}

func TestExtendBudgetRecoveryReplaysJournaledOffer(t *testing.T) {
	t.Parallel()
	endpoint, req, offer, _ := budgetExtensionEndpointBed(t)
	publish := extendBudgetRequest(req, "earned-raise", offer)
	entry := Entry{Opid: publish.Opid, Machine: req.Actor.Machine, Lineage: req.Actor.Lineage, Intent: publish.Intent}
	rebuilt, err := requestForEntry(req.Endpoint, entry)
	if err != nil {
		t.Fatal(err)
	}
	result, err := Publish(req.Endpoint, rebuilt)
	if err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("replayed extension: %+v %v", result, err)
	}
	tree, err := loadTreeFor(endpoint, result.Tip)
	if err != nil || tree.Live["earned-raise"].BudgetExtension == nil ||
		tree.Live["earned-raise"].BudgetExtension.EvidenceID != offer.EvidenceID {
		t.Fatalf("recovery lost the journaled offer: marker=%+v err=%v", tree.Live["earned-raise"].BudgetExtension, err)
	}

	for _, scenario := range []struct {
		name       string
		opid       string
		invalidTip bool
	}{
		{name: "sighted push advances accepted", opid: "fake-recovery-valid"},
		{name: "invalid canonical tip refuses accepted advancement", opid: "fake-recovery-invalid", invalidTip: true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			endpoint, client := fakeGoalEndpoint(t)
			previousTip := acceptedTipForEndpoint(t, endpoint)
			opid := scenario.opid
			published, err := Publish(endpoint, fakeRootPublishRequest(endpoint, opid))
			if err != nil || published.Outcome != OutcomeConfirmed {
				t.Fatalf("publish recovered operation: %+v %v", published, err)
			}
			entry, err := ReadEntry(endpoint.Root, opid)
			if err != nil {
				t.Fatal(err)
			}
			entry.Phase = PhasePushed
			entry.Outcome = ""
			entry.TerminalAt = ""
			if err := writeEntry(endpoint.Root, entry); err != nil {
				t.Fatal(err)
			}

			canonicalTip := published.Tip
			if scenario.invalidTip {
				invalid, err := client.Build("invalid-goal-bytes", canonicalTip,
					[]Change{{Path: goalsPrefix + "backlog.md", Content: []byte("invalid goal root\n")}}, "invalid canonical goal")
				if err != nil {
					t.Fatal(err)
				}
				outcome, err := client.Publish(canonicalTip, invalid)
				if err != nil || outcome != CASLanded {
					t.Fatalf("publish invalid canonical tip: %s %v", outcome, err)
				}
				canonicalTip = invalid
			}
			client.store.mu.Lock()
			client.accepted = previousTip
			buildsBefore := client.store.serial
			client.store.mu.Unlock()
			if got := acceptedTipForEndpoint(t, endpoint); got != previousTip {
				t.Fatalf("lost-acknowledgment fixture accepted=%s, want %s", got, previousTip)
			}

			reports, err := Recover(endpoint)
			if err != nil || len(reports) != 1 || reports[0].Opid != opid || reports[0].Action != ActionConfirm {
				t.Fatalf("sighted operation was not confirmed: reports=%+v err=%v", reports, err)
			}
			terminal, err := ReadEntry(endpoint.Root, opid)
			if err != nil || terminal.Phase != PhaseTerminal || terminal.Outcome != OutcomeConfirmed {
				t.Fatalf("recovery journal not terminal: %+v err=%v", terminal, err)
			}
			client.store.mu.Lock()
			gotTip, buildsAfter := client.store.canonical, client.store.serial
			client.store.mu.Unlock()
			if gotTip != canonicalTip || buildsAfter != buildsBefore {
				t.Fatalf("recovery rebuilt or moved canonical history: tip=%s want=%s builds=%d want=%d",
					gotTip, canonicalTip, buildsAfter, buildsBefore)
			}
			accepted := acceptedTipForEndpoint(t, endpoint)
			if scenario.invalidTip {
				if accepted != previousTip || !strings.Contains(reports[0].Detail, "accepted NOT advanced (the tip does not validate)") ||
					!strings.Contains(reports[0].Detail, "the ledger tree at "+canonicalTip+" does not validate") {
					t.Fatalf("invalid canonical tip advanced accepted or hid validation refusal: accepted=%s report=%+v", accepted, reports[0])
				}
			} else if accepted != canonicalTip || reports[0].Detail != "confirmed on the canonical tip" {
				t.Fatalf("validated canonical tip did not advance accepted: accepted=%s want=%s report=%+v", accepted, canonicalTip, reports[0])
			}
		})
	}
}

func TestBudgetExtensionMarkerSurvivesClaimLifecycle(t *testing.T) {
	t.Parallel()
	endpoint, req, offer, _ := budgetExtensionEndpointBed(t)
	extended, err := ExtendBudget(req, "earned-raise", offer)
	if err != nil || extended.Outcome != OutcomeConfirmed {
		t.Fatalf("extend: %+v %v", extended, err)
	}
	act := func(ulid string, minute int) VerbRequest {
		next := req
		next.Ulid = ulid
		next.Now = req.Now.Add(time.Duration(minute) * time.Minute)
		return next
	}
	markerAt := req.stamp()
	assertLiveMarker := func(tip string) {
		t.Helper()
		tree, readErr := loadTreeFor(endpoint, tip)
		if readErr != nil || tree.Live["earned-raise"] == nil || tree.Live["earned-raise"].BudgetExtension == nil ||
			tree.Live["earned-raise"].BudgetExtension.At != markerAt {
			t.Fatalf("live marker was lost: goal=%+v err=%v", tree.Live["earned-raise"], readErr)
		}
	}
	released, err := Release(act("01J5X00000000000000000EX07", 1), "earned-raise")
	if err != nil || released.Outcome != OutcomeConfirmed {
		t.Fatalf("release: %+v %v", released, err)
	}
	assertLiveMarker(released.Tip)
	reclaimed, err := Claim(act("01J5X00000000000000000EX08", 2), "earned-raise")
	if err != nil || reclaimed.Outcome != OutcomeConfirmed {
		t.Fatalf("reclaim: %+v %v", reclaimed, err)
	}
	assertLiveMarker(reclaimed.Tip)
	done, err := Done(act("01J5X00000000000000000EX09", 3), "earned-raise", "Shipped.")
	if err != nil || done.Outcome != OutcomeConfirmed {
		t.Fatalf("done: %+v %v", done, err)
	}
	tree, err := loadTreeFor(endpoint, done.Tip)
	if err != nil || tree.Done["earned-raise"].BudgetExtension == nil {
		t.Fatalf("archived parent lost marker: %+v %v", tree.Done["earned-raise"], err)
	}
	reopened, err := Reopen(act("01J5X00000000000000000EX10", 4), "earned-raise")
	if err != nil || reopened.Outcome != OutcomeConfirmed {
		t.Fatalf("reopen: %+v %v", reopened, err)
	}
	assertLiveMarker(reopened.Tip)
	tree, err = loadTreeFor(endpoint, reopened.Tip)
	if err != nil || tree.Live["earned-raise"].Budget != nil || tree.Live["earned-raise"].Approved != nil {
		t.Fatalf("reopen lifecycle shape changed: %+v %v", tree.Live["earned-raise"], err)
	}
}

func TestBudgetExtensionMarkerSurvivesHumanActsStealAndSplit(t *testing.T) {
	t.Parallel()
	endpoint, req, offer, _ := budgetExtensionEndpointBed(t)
	extended, err := ExtendBudget(req, "earned-raise", offer)
	if err != nil || extended.Outcome != OutcomeConfirmed {
		t.Fatalf("extend: %+v %v", extended, err)
	}
	act := func(ulid string, minute int, human bool) VerbRequest {
		next := req
		next.Ulid = ulid
		next.Now = req.Now.Add(time.Duration(minute) * time.Minute)
		next.Actor = Actor{Machine: "mac-b", Lineage: "lin-2"}
		if human {
			next.Actor.Human = "Wido"
		}
		return next
	}
	assertMarker := func(tip string, archived bool) *GoalFile {
		t.Helper()
		tree, readErr := loadTreeFor(endpoint, tip)
		if readErr != nil {
			t.Fatal(readErr)
		}
		file := tree.Live["earned-raise"]
		if archived {
			file = tree.Done["earned-raise"]
		}
		if file == nil || file.BudgetExtension == nil || file.BudgetExtension.Opid != req.opid() {
			t.Fatalf("extension marker was lost: %+v", file)
		}
		return file
	}

	stolen, err := Steal(act("01J5X00000000000000000EY01", 1, true), "earned-raise")
	if err != nil || stolen.Outcome != OutcomeConfirmed {
		t.Fatalf("steal: %+v %v", stolen, err)
	}
	if file := assertMarker(stolen.Tip, false); file.Claimed.Machine != "mac-b" || file.Claimed.Lineage != "lin-2" {
		t.Fatalf("steal did not move the claim: %+v", file.Claimed)
	}

	current := Budget{ElapsedLimit: "2h", AttemptLimit: 3, ReservedJobMinutesLimit: 180, ActiveJobLimit: 1, ReviewRoundLimit: 3}
	set := act("01J5X00000000000000000EY02", 2, true)
	setResult, err := SetBudgetApproved(set, "earned-raise", current, testHumanAuthority(t, endpoint.Root, set.Now))
	if err != nil || setResult.Outcome != OutcomeConfirmed {
		t.Fatalf("set-budget: %+v %v", setResult, err)
	}
	if file := assertMarker(setResult.Tip, false); file.ValidateApprovalRecord() != nil || *file.Budget != current || !strings.HasPrefix(file.Approved.At, set.stamp()) {
		t.Fatalf("later set-budget did not validate against the current tuple: %+v", file)
	}

	unapprove := act("01J5X00000000000000000EY03", 3, true)
	unapproved, err := Unapprove(unapprove, "earned-raise", "reconsider the next member", testHumanAuthority(t, endpoint.Root, unapprove.Now))
	if err != nil || unapproved.Outcome != OutcomeConfirmed {
		t.Fatalf("unapprove: %+v %v", unapproved, err)
	}
	if file := assertMarker(unapproved.Tip, false); file.Budget != nil || file.Approved != nil {
		t.Fatalf("unapprove retained approval state: %+v", file)
	}
	unpark := act("01J5X00000000000000000EY04", 4, true)
	unpark.Authority = testTerminalAuthority(t, endpoint.Root, unpark.Now)
	unparked, err := Unpark(unpark, "earned-raise")
	if err != nil || unparked.Outcome != OutcomeConfirmed {
		t.Fatalf("unpark: %+v %v", unparked, err)
	}
	approve := act("01J5X00000000000000000EY05", 5, true)
	approved, err := Approve(approve, []string{"earned-raise"}, &current, testHumanAuthority(t, endpoint.Root, approve.Now))
	if err != nil || approved.Outcome != OutcomeConfirmed {
		t.Fatalf("approve after marker: %+v %v", approved, err)
	}
	if file := assertMarker(approved.Tip, false); file.ValidateApprovalRecord() != nil || *file.Budget != current {
		t.Fatalf("later approval did not bind the current tuple: %+v", file)
	}
	claimed, err := Claim(act("01J5X00000000000000000EY06", 6, false), "earned-raise")
	if err != nil || claimed.Outcome != OutcomeConfirmed {
		t.Fatalf("claim after approval: %+v %v", claimed, err)
	}
	members := testMembers("earned-raise")
	split, err := Split(act("01J5X00000000000000000EY07", 7, false), "earned-raise", members,
		mainRatification("earned-raise", members), nil)
	if err != nil || split.Outcome != OutcomeConfirmed {
		t.Fatalf("split: %+v %v", split, err)
	}
	assertMarker(split.Tip, true)
	tree, err := loadTreeFor(endpoint, split.Tip)
	if err != nil {
		t.Fatal(err)
	}
	for _, member := range members {
		if tree.Live[member.ID] == nil || tree.Live[member.ID].BudgetExtension != nil {
			t.Fatalf("split member inherited the extension marker: %+v", tree.Live[member.ID])
		}
	}
}

func TestReconcileRefusesBudgetExtensionEdits(t *testing.T) {
	t.Parallel()
	base := approvedGoalFixture(vGoal("extension-edit", StateQueued), testBudget())
	base.State = StateClaimed
	base.Claimed = &ClaimRecord{Machine: "mac-a", Lineage: "lin-1", At: base.History[0].At, Revision: 1, AccountingRevision: 1}
	base.Revision++
	event := HistoryLine{At: "2026-08-20T22:00:00Z", Opid: "01J5X00000000000000000EX06-mac-a-1a2b3c4d", Verb: "extend-budget", Actor: "mac-a+lin-1", Targets: []string{base.Id}, Keep: -1}
	base.History = append(base.History, event)
	base.BudgetExtension = &BudgetExtensionRecord{At: event.At, Opid: event.Opid, AttemptLimitFrom: 4, AttemptLimitTo: 14,
		ReservedJobMinutesFrom: 240, ReservedJobMinutesTo: 1440, EvidenceKind: "review", EvidenceID: "critic-root", EvidenceAt: "2026-08-20T21:30:00Z"}
	base.Budget.AttemptLimit, base.Budget.ReservedJobMinutesLimit = 14, 1440
	base.Approved.Digest = ApprovalDigest(base.Intent, base.Tier, Budget{ElapsedLimit: "4h", AttemptLimit: 4, ReservedJobMinutesLimit: 240, ActiveJobLimit: 2})

	for name, mutate := range map[string]func(*GoalFile){
		"remove": func(f *GoalFile) { f.BudgetExtension = nil },
		"alter":  func(f *GoalFile) { copy := *f.BudgetExtension; copy.EvidenceID = "other"; f.BudgetExtension = &copy },
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			edited := *base
			mutate(&edited)
			if _, err := mapOneChange("plans/goals/extension-edit.md", base, &edited); err == nil || !strings.Contains(err.Error(), "BudgetExtension is a generated field") {
				t.Fatalf("reconcile accepted marker %s: %v", name, err)
			}
		})
	}
	without := *base
	without.BudgetExtension = nil
	added := without
	added.BudgetExtension = base.BudgetExtension
	if _, err := mapOneChange("plans/goals/extension-edit.md", &without, &added); err == nil || !strings.Contains(err.Error(), "BudgetExtension is a generated field") {
		t.Fatalf("reconcile accepted a hand-added marker: %v", err)
	}
}
