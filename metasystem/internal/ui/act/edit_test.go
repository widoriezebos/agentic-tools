package act

// The goal editor's first gate, from a signed-in browser.
//
// These drive the whole chain: the act layer's Edit, the QueuedOnly flag it
// sets always, the engine's four refused states, and the History line the
// ledger keeps. They run over the same fixture ledger the approval tests run
// over — a bare repository in t.TempDir, a fake runtime, no remote anybody
// else can reach.

import (
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)

// The act itself: three fields, landed, with the ledger naming the hand.
func TestEditFromASignedInSessionLandsTheThreeFieldsAndNamesTheSession(t *testing.T) {
	t.Parallel()
	bed := ledger(t)
	openGoal(t, bed, "ui-edit")
	authority := sessionFor(t, bed)

	intent := "Refunds are issued within a day, with nobody touching the queue."
	next := "Take the worker to a working end state; the approach is yours."
	labels := []string{"ui", "board"}
	if err := authority.Edit("ui-edit", Edited{Intent: &intent, NextStep: &next, Labels: &labels}); err != nil {
		t.Fatalf("edit: %v", err)
	}

	file := readGoal(t, bed, "ui-edit")
	testutil.Expect(t, "the intent", file.Intent, intent)
	testutil.Expect(t, "the next step", file.NextStep, next)
	// The engine writes labels sorted and deduplicated.
	testutil.Expect(t, "the labels", strings.Join(file.Labels, " "), "board ui")
	testutil.Expect(t, "the goal is still queued", file.State, goal.StateQueued)

	last := file.History[len(file.History)-1]
	testutil.Expect(t, "the last verb", last.Verb, "edit")
	testutil.Expect(t, "the History outcome", last.AuthorityOutcome, goal.AuthorityOutcomeSignedInSession)
	testutil.Expect(t, "the issuer", last.ChannelProvider, "browser")
	testutil.Expect(t, "the handle", last.ChannelUser, "Wido")
	testutil.Expect(t, "the session", last.ChannelRef, testSession)
	if _, problems := goal.ParseFile(goal.RenderFile(file)); len(problems) != 0 {
		t.Fatalf("the written goal does not read back clean: %v", problems)
	}
}

// Only what the sheet said changed travels. A save that names the next step
// says nothing about the intent, and an intent changed elsewhere stands.
func TestAnEditCarriesOnlyTheFieldsItWasGiven(t *testing.T) {
	t.Parallel()
	bed := ledger(t)
	openGoal(t, bed, "ui-partial")
	authority := sessionFor(t, bed)
	before := readGoal(t, bed, "ui-partial")

	next := "Only this line changed."
	if err := authority.Edit("ui-partial", Edited{NextStep: &next}); err != nil {
		t.Fatalf("edit: %v", err)
	}

	file := readGoal(t, bed, "ui-partial")
	testutil.Expect(t, "the next step changed", file.NextStep, next)
	testutil.Expect(t, "the intent was left alone", file.Intent, before.Intent)
	testutil.Expect(t, "the labels were left alone", len(file.Labels), len(before.Labels))
}

// The flag is set here and always, so each of the three states the design
// names is refused at the tip rather than displaced — including a goal
// claimed between the page's read and this act.
func TestEditRefusesAnApprovedClaimedOrParkedGoal(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name  string
		id    string
		stage func(t *testing.T, bed *ledgerBed, authority Authority, id string)
		want  string
	}{
		{
			name: "approved", id: "ui-edit-approved",
			stage: func(t *testing.T, _ *ledgerBed, authority Authority, id string) {
				t.Helper()
				if err := authority.Approve(id, box()); err != nil {
					t.Fatalf("approve: %v", err)
				}
			},
			want: "is approved: withdraw the approval, edit it, then approve it again",
		},
		{
			name: "claimed", id: "ui-edit-claimed",
			stage: func(t *testing.T, bed *ledgerBed, authority Authority, id string) {
				t.Helper()
				if err := authority.Approve(id, box()); err != nil {
					t.Fatalf("approve: %v", err)
				}
				// The page has read the queue. Now a seat claims the goal.
				claim(t, bed, id)
			},
			want: "; edit it at a terminal",
		},
		{
			name: "parked", id: "ui-edit-parked",
			stage: func(t *testing.T, _ *ledgerBed, authority Authority, id string) {
				t.Helper()
				if err := authority.Park(id, "not now"); err != nil {
					t.Fatalf("park: %v", err)
				}
			},
			want: "is parked: return it to the queue to edit it",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			bed := ledger(t)
			openGoal(t, bed, test.id)
			authority := sessionFor(t, bed)
			before := readGoal(t, bed, test.id)
			test.stage(t, bed, authority, test.id)

			intent := "A rewrite this state does not admit."
			err := authority.Edit(test.id, Edited{Intent: &intent})

			refusal, ok := err.(*Refusal)
			if !ok {
				t.Fatalf("edit of a %s goal = %v, want an act.Refusal", test.name, err)
			}
			testutil.Expect(t, "the engine refused it", refusal.Kind, KindEngine)
			if !strings.Contains(refusal.Message, test.want) {
				t.Fatalf("the refusal does not say what to do instead: %q", refusal.Message)
			}
			testutil.Expect(t, "the intent stands", readGoal(t, bed, test.id).Intent, before.Intent)
		})
	}
}

// What this layer refuses before anything is published: an act with no goal,
// an act that changes nothing, and either line emptied. The blanks are the
// open act's own sentences, because there is one rule for what an intent is.
func TestEditRefusesWhatItCannotPublish(t *testing.T) {
	t.Parallel()
	bed := ledger(t)
	openGoal(t, bed, "ui-edit-guards")
	authority := sessionFor(t, bed)
	blank := "   "
	intent := "A perfectly good intent."

	for _, test := range []struct {
		name string
		id   string
		what Edited
		code string
	}{
		{"no goal", "  ", Edited{Intent: &intent}, "no-goal"},
		{"nothing changed", "ui-edit-guards", Edited{}, "no-change"},
		{"a blank intent", "ui-edit-guards", Edited{Intent: &blank}, "no-intent"},
		{"a blank next step", "ui-edit-guards", Edited{NextStep: &blank}, "no-next-step"},
	} {
		err := authority.Edit(test.id, test.what)
		refusal, ok := err.(*Refusal)
		if !ok {
			t.Fatalf("%s = %v, want an act.Refusal", test.name, err)
		}
		testutil.Expect(t, test.name+" is the request's own fault", refusal.Kind, KindRequest)
		testutil.Expect(t, test.name+" names its code", refusal.Code, test.code)
	}

	// Nothing was published by any of them.
	file := readGoal(t, bed, "ui-edit-guards")
	testutil.Expect(t, "the goal's history is untouched", file.History[len(file.History)-1].Verb, "open")
}

// A server that proved no human writes nothing, and says so before it reads
// what was sent.
func TestEditFromAnUnprovenServerWritesNothing(t *testing.T) {
	t.Parallel()
	intent := "A rewrite nobody is behind."

	err := Unproven("the interface was started by an agent process").Edit("ui-edit", Edited{Intent: &intent})

	refusal, ok := err.(*Refusal)
	if !ok {
		t.Fatalf("edit from an unproven server = %v, want an act.Refusal", err)
	}
	testutil.Expect(t, "the kind", refusal.Kind, KindUnproven)
	testutil.Expect(t, "the reason is the proof's own", refusal.Message,
		"the interface was started by an agent process")
}
