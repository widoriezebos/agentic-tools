package goal

// The one edit the interface makes without a proposal: three fields of a goal
// that is still queued and that nobody has approved.
//
// What is proven here is the allowlist itself — that it admits the queued
// case and refuses the other three in the sentences a human acts on — that
// the line the edit appends names the signed-in session, and that a field
// this act says nothing about is a field it leaves alone, which is the whole
// reason the fields are nullable.

import (
	"strings"
	"testing"
)

func TestQueuedOnlyEditAdmitsAQueuedGoalAndNamesTheSession(t *testing.T) {
	t.Parallel()
	_, root := oneClone(t)
	seedLedger(t, root)
	if result, err := Open(verbReq(root, "01J5X00000000000000000QE00", "mac-a"), "queued-edit",
		"Work a signed-in human rewrites.", OriginMain, "Run it."); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("open: %+v %v", result, err)
	}

	request := verbReq(root, "01J5X00000000000000000QE10", "mac-a")
	request.Actor.Human = "Wido"
	request.Authority = sessionProofForTest(t, root, request.Now)
	intent := "Refunds are issued within a day."
	next := "Take the worker to a working end state; the approach is yours."
	labels := []string{"ui", "board"}
	result, err := Edit(request, "queued-edit", EditFields{
		QueuedOnly: true, Intent: &intent, NextStep: &next, Labels: &labels,
	})
	if err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("edit a queued goal under a session: %+v %v", result, err)
	}

	tree, err := loadTree(root, result.Tip)
	if err != nil {
		t.Fatal(err)
	}
	file := tree.Live["queued-edit"]
	if file.Intent != intent || file.NextStep != next {
		t.Fatalf("the three fields did not land: %+v", file)
	}
	// The engine writes labels sorted and deduplicated, and the browser reads
	// them back in that form.
	if strings.Join(file.Labels, " ") != "board ui" {
		t.Fatalf("labels = %v, want the canonical pair", file.Labels)
	}
	last := file.History[len(file.History)-1]
	if last.Verb != "edit" {
		t.Fatalf("the last verb is %q", last.Verb)
	}
	assertSessionLine(t, last)
	if parsed, problems := ParseFile(RenderFile(file)); parsed == nil || len(problems) != 0 {
		t.Fatalf("ParseFile after a session edit reported %v", problems)
	}
	if problems := ValidateTree(tree); len(problems) != 0 {
		t.Fatalf("the edited tree is invalid: %v", problems)
	}
}

// Each of the three states a queued-only edit refuses, in the words that say
// which act would let it through. The state is read at the tip, so an
// approval or a claim that lands between a page's read and this act is
// refused here rather than being quietly displaced.
func TestQueuedOnlyEditRefusesAnApprovedClaimedOrParkedGoal(t *testing.T) {
	t.Parallel()
	_, root := oneClone(t)
	seedLedger(t, root)

	for name, bed := range map[string]struct {
		id    string
		seed  string
		edit  string
		stage func(t *testing.T, root, id string)
		want  string
	}{
		"approved": {
			id: "edit-approved", seed: "01J5X00000000000000000QA00", edit: "01J5X00000000000000000QA99",
			stage: func(t *testing.T, root, id string) {
				t.Helper()
				request := verbReq(root, "01J5X00000000000000000QA10", "mac-a")
				request.Actor.Human = "Wido"
				budget := testBudget()
				if result, err := Approve(request, []string{id}, &budget,
					sessionProofForTest(t, root, request.Now)); err != nil || result.Outcome != OutcomeConfirmed {
					t.Fatalf("approve: %+v %v", result, err)
				}
			},
			want: "goal edit-approved is approved: withdraw the approval, edit it, then approve it again",
		},
		"claimed": {
			id: "edit-claimed", seed: "01J5X00000000000000000QC00", edit: "01J5X00000000000000000QC99",
			stage: func(t *testing.T, root, id string) {
				t.Helper()
				approve := verbReq(root, "01J5X00000000000000000QC10", "mac-a")
				approve.Actor.Human = "Wido"
				budget := testBudget()
				if result, err := Approve(approve, []string{id}, &budget,
					sessionProofForTest(t, root, approve.Now)); err != nil || result.Outcome != OutcomeConfirmed {
					t.Fatalf("approve: %+v %v", result, err)
				}
				if result, err := Claim(verbReq(root, "01J5X00000000000000000QC20", "mac-b"), id); err != nil ||
					result.Outcome != OutcomeConfirmed {
					t.Fatalf("claim: %+v %v", result, err)
				}
			},
			want: "goal edit-claimed is claimed by mac-b+lin-1; edit it at a terminal",
		},
		"parked": {
			id: "edit-parked", seed: "01J5X00000000000000000QP00", edit: "01J5X00000000000000000QP99",
			stage: func(t *testing.T, root, id string) {
				t.Helper()
				request := verbReq(root, "01J5X00000000000000000QP10", "mac-a")
				request.Actor.Human = "Wido"
				request.Authority = sessionProofForTest(t, root, request.Now)
				if result, err := Park(request, id, "not now"); err != nil || result.Outcome != OutcomeConfirmed {
					t.Fatalf("park: %+v %v", result, err)
				}
			},
			want: "goal edit-parked is parked: return it to the queue to edit it",
		},
	} {
		if result, err := Open(verbReq(root, bed.seed, "mac-a"), bed.id,
			"Work in a state the browser may not edit.", OriginMain, "Run it."); err != nil ||
			result.Outcome != OutcomeConfirmed {
			t.Fatalf("%s: open: %+v %v", name, result, err)
		}
		bed.stage(t, root, bed.id)

		request := verbReq(root, bed.edit, "mac-a")
		request.Actor.Human = "Wido"
		request.Authority = sessionProofForTest(t, root, request.Now)
		intent := "A rewrite this state does not admit."
		// A mutation that refuses is a rejected publication carrying the
		// engine's sentence, not a returned error: that is the shape the act
		// layer reads, and it is what reaches the page.
		result, err := Edit(request, bed.id, EditFields{QueuedOnly: true, Intent: &intent})
		if err != nil {
			t.Fatalf("%s: edit: %v", name, err)
		}
		if result.Outcome != OutcomeRejected {
			t.Fatalf("%s: a queued-only edit was admitted: %+v", name, result)
		}
		if result.Detail != bed.want {
			t.Fatalf("%s: refusal = %q, want %q", name, result.Detail, bed.want)
		}
	}
}

// The reason the three fields are nullable: a browser that republished every
// field would overwrite what somebody changed at a terminal while its page
// was open. A save that names only the next step says nothing about the
// intent, and the intent that landed in the meantime stands.
func TestATerminalIntentChangeSurvivesABrowserNextStepSave(t *testing.T) {
	t.Parallel()
	_, root := oneClone(t)
	seedLedger(t, root)
	if result, err := Open(verbReq(root, "01J5X00000000000000000QS00", "mac-a"), "queued-race",
		"The intent the page read.", OriginMain, "The next step the page read."); err != nil ||
		result.Outcome != OutcomeConfirmed {
		t.Fatalf("open: %+v %v", result, err)
	}

	// The terminal's edit, which carries no flag and names only the intent.
	atTerminal := "The intent a terminal wrote while the page was open."
	if result, err := Edit(verbReq(root, "01J5X00000000000000000QS10", "mac-a"), "queued-race",
		EditFields{Intent: &atTerminal}); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("terminal edit: %+v %v", result, err)
	}

	// The browser's save, which names only the next step a human changed.
	request := verbReq(root, "01J5X00000000000000000QS20", "mac-a")
	request.Actor.Human = "Wido"
	request.Authority = sessionProofForTest(t, root, request.Now)
	inBrowser := "The next step the human typed."
	result, err := Edit(request, "queued-race", EditFields{QueuedOnly: true, NextStep: &inBrowser})
	if err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("browser edit: %+v %v", result, err)
	}

	tree, err := loadTree(root, result.Tip)
	if err != nil {
		t.Fatal(err)
	}
	file := tree.Live["queued-race"]
	if file.Intent != atTerminal {
		t.Fatalf("intent = %q; the browser's save overwrote the terminal's edit", file.Intent)
	}
	if file.NextStep != inBrowser {
		t.Fatalf("next step = %q, want the browser's", file.NextStep)
	}
}
