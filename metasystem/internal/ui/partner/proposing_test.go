package partner_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/backlog"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/partner"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/partner/fakeacp"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/snapshot"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/uitools"
)

// Who decides whether an act the Partner proposed reaches the human at all.
//
// The tool server validates shape and bounds and has no ledger; the host has no
// turn. This service holds the reading the turn was composed from, so it is the
// one place that can say whether the goals an action names are at the accepted
// tip — and when they are not, it says so on the card rather than dropping the
// action in silence.
//
// What it does NOT judge is whether the act would be allowed: that is the
// engine's answer at the act, in its own words, exactly as it is for every
// button on every page.

const proposedAt = "2026-09-26T12:00:00Z"

// proposingLedger is the tip these tests admit against: four live goals, one of
// them approved with a budget and a tier, so an approve's own reading has
// something to carry, and one of them waiting for another, so an abandon's
// reading has a live dependent to list.
func proposingLedger() snapshot.Observation {
	at, _ := time.Parse(time.RFC3339, proposedAt)
	tree := &goal.TreeGoals{
		Root: &goal.RootRecord{Identity: "01ARZ3NDEKTSV4RRFFQ69G5FAV", FormatVersion: "2",
			SyncMode: goal.SyncLocal, Revision: 1},
		Live:      map[string]*goal.GoalFile{},
		Done:      map[string]*goal.GoalFile{},
		Abandoned: map[string]*goal.GoalFile{},
	}
	tree.Live["fleet-presence"] = &goal.GoalFile{
		Id: "fleet-presence", State: goal.StateQueued, Origin: "human",
		Intent:   "Fleet presence is read from the census, not polled. It has been for a week.",
		NextStep: "Read the census.", OpenedAt: "2026-08-23T00:00:00Z", Revision: 9,
		Tier: 2, Labels: []string{"fleet", "browser-interface"},
	}
	tree.Live["refunds"] = &goal.GoalFile{
		Id: "refunds", State: goal.StateApproved, Origin: "human",
		Intent: "Refunds land within a day.", NextStep: "Read the retry loop.",
		OpenedAt: "2026-08-23T00:00:00Z", Revision: 4, Tier: 3,
	}
	tree.Live["bank-sandbox"] = &goal.GoalFile{
		Id: "bank-sandbox", State: goal.StateQueued, Origin: "human",
		Intent: "The bank sandbox answers late.", NextStep: "Read it.",
		OpenedAt: "2026-08-23T00:00:00Z", Revision: 2,
	}
	// One goal waits for another, which is what an abandon's own reading is
	// about: what becomes of the goals that wait for the one being retired.
	tree.Live["census-format"] = &goal.GoalFile{
		Id: "census-format", State: goal.StateQueued, Origin: "human",
		Intent: "The census format is settled.", NextStep: "Write it down.",
		OpenedAt: "2026-08-23T00:00:00Z", Revision: 1,
		Blocked: []string{"fleet-presence"},
	}
	horizon := goal.NewApprovalHorizon(tree, at)
	return snapshot.Observation{
		ObservedAt: at, State: snapshot.StateRead, Tip: "c5d517f427e35e19c2944ab9aefee4d9c9cd9e6c",
		Tree: tree, Horizon: horizon, SyncMode: goal.SyncLocal,
		Admission: backlog.Admit(goal.Projection{Tree: tree, Horizon: horizon}),
	}
}

// proposed is one canned propose call, in the tool's own fixed form.
func proposed(verb, subject string, lines []string, why string) fakeacp.Read {
	frame := uitools.ProposalHeader + verb + "\n"
	if verb != uitools.ProposeOpen {
		frame += uitools.ProposalGoal + subject + "\n"
	}
	for _, line := range lines {
		frame += line + "\n"
	}
	return fakeacp.Read{
		Name:   "mcp__" + uitools.ServerName + "__" + uitools.OpPropose,
		Title:  "propose(" + verb + " " + subject + ")",
		Result: uitools.PreparedProposalLine + "\n" + frame + uitools.ProposalSeparator + "\n" + why + "\n",
	}
}

// serviceProposing is a Partner that answers in words and prepares the canned
// actions given, against the ledger above.
func serviceProposing(t *testing.T, reads ...fakeacp.Read) *partner.Service {
	t.Helper()
	root := t.TempDir()
	host := partner.NewHostOn(
		partner.Runtime{Name: "fake", ReadOnly: "a fake server reads nothing"},
		root, fakeacp.Open(fakeacp.Script{
			Reads:  reads,
			Chunks: []string{"I have proposed them. ", "Read them and apply the ones you want."},
		}))
	t.Cleanup(host.Close)
	return partner.NewService(host.Runtime(), host,
		func(human string) (*partner.Conversation, error) { return partner.OpenConversation(root, human) },
		partner.Facts{Observe: proposingLedger},
		func() time.Time { return time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC) })
}

func onDecisions() partner.Page {
	return partner.Page{Section: "Decisions", Path: "/decisions"}
}

// ask runs one turn of a proposing service and answers the beats and the
// answer's own proposals.
func askProposing(t *testing.T, service *partner.Service, question string) ([]partner.Event, []partner.Proposal) {
	t.Helper()
	events, stop := service.Subscribe()
	defer stop()
	_, err := service.Submit(context.Background(), "Wido", "key-1", question, onDecisions())
	testutil.Require(t, "the turn is admitted", err, nil)
	beats := drain(t, events)
	read, err := service.Snapshot("Wido", 100)
	testutil.Require(t, "the conversation reads back", err, nil)
	return beats, read.Messages[len(read.Messages)-1].Proposals
}

// An action whose goal is at the tip is admitted, carried on the stream as it is
// admitted, stamped with the title the pages use, and kept on the answer.
func TestAnActionOnAGoalAtTheTipIsOffered(t *testing.T) {
	t.Parallel()
	service := serviceProposing(t, proposed(uitools.ProposePark, "fleet-presence",
		[]string{uitools.ProposalBecause + "superseded by the seat inventory (g1-s42)"},
		"the five name the fleet inventory g1-s42 built"))
	beats, kept := askProposing(t, service, "these are superseded; put them away")

	offered := []partner.Proposal{}
	at, ended := -1, -1
	for index, beat := range beats {
		if beat.Kind == partner.EventProposal && beat.Proposal != nil {
			offered = append(offered, *beat.Proposal)
			if at < 0 {
				at = index
			}
		}
		if beat.Kind == partner.EventDone {
			ended = index
		}
	}
	testutil.Require(t, "one proposal beat", len(offered), 1)
	testutil.Expect(t, "it is offered", offered[0].Offered, true)
	testutil.Expect(t, "the route it dispatches to", offered[0].Verb, uitools.ProposePark)
	testutil.Expect(t, "the goal", offered[0].Goal, "fleet-presence")
	// The title is the pages' own: the first sentence of the intent, never the
	// Partner's prose and never the whole paragraph.
	testutil.Expect(t, "the title as the pages say it", offered[0].Title,
		"Fleet presence is read from the census, not polled")
	testutil.Expect(t, "the field under the body's own name",
		offered[0].Fields[uitools.FieldBecause], "superseded by the seat inventory (g1-s42)")
	testutil.Expect(t, "the Partner's own words", offered[0].Why,
		"the five name the fleet inventory g1-s42 built")
	testutil.Expect(t, "it starts waiting", offered[0].State, partner.ProposalWaiting)
	testutil.Expect(t, "at version one", offered[0].Version, 1)
	testutil.Expect(t, "a park carries no reading", offered[0].Read == nil, true)
	// It arrives before the answer ends, which is what fills the card line by
	// line while the answer is still being written.
	testutil.Expect(t, "before the turn ended", at >= 0 && at < ended, true)

	testutil.Require(t, "the answer keeps it", len(kept), 1)
	testutil.Expect(t, "with its index", kept[0].Index, 0)
	testutil.Expect(t, "its state", kept[0].State, partner.ProposalWaiting)
	testutil.Expect(t, "and its version", kept[0].Version, 1)
}

// An approve, an edit and an abandon carry the goal as it was read; the other
// seven do not.
func TestAnApproveAndAnEditCarryTheGoalAsItWasRead(t *testing.T) {
	t.Parallel()
	service := serviceProposing(t,
		proposed(uitools.ProposeApprove, "fleet-presence", nil, "it is ready"),
		proposed(uitools.ProposeEdit, "fleet-presence",
			[]string{uitools.ProposalIntent + "Fleet presence is read from the census."}, "tighter"),
		proposed(uitools.ProposeUnpark, "fleet-presence", nil, "back to the queue"),
	)
	_, kept := askProposing(t, service, "approve it, tidy the intent and return it")
	testutil.Require(t, "three actions", len(kept), 3)

	for _, at := range []int{0, 1} {
		read := kept[at].Read
		testutil.Require(t, kept[at].Verb+" carries the reading", read != nil, true)
		testutil.Expect(t, kept[at].Verb+" carries the intent as read", read.Intent,
			"Fleet presence is read from the census, not polled. It has been for a week.")
		testutil.Expect(t, kept[at].Verb+" carries the next step as read", read.NextStep, "Read the census.")
		testutil.Expect(t, kept[at].Verb+" carries the tier as read", read.Tier, 2)
		testutil.Expect(t, kept[at].Verb+" carries the labels as read", read.Labels,
			[]string{"fleet", "browser-interface"})
	}
	testutil.Expect(t, "an unpark carries no reading", kept[2].Read == nil, true)
}

// "Open X, then say Y waits for X" is one proposal: a goal an earlier open of
// the same answer named counts as being there.
func TestAGoalAnEarlierOpenNamedCountsAsBeingThere(t *testing.T) {
	t.Parallel()
	service := serviceProposing(t,
		proposed(uitools.ProposeOpen, "refund-worker", []string{
			uitools.ProposalIntent + "Every refund lands within a day, with nobody touching the queue.",
			uitools.ProposalNextStep + "Read the retry loop.",
			uitools.ProposalID + "refund-worker",
			uitools.ProposalSeverity + "2", uitools.ProposalNovelty + "1",
			uitools.ProposalExposure + "2", uitools.ProposalAccumulation + "1",
			uitools.ProposalBasis + "payments, one team",
		}, "as discussed"),
		proposed(uitools.ProposeBlock, "bank-sandbox",
			[]string{uitools.ProposalBlocker + "refund-worker"}, "it waits for the new one"),
	)
	_, kept := askProposing(t, service, "Create it, and make the sandbox wait for it")
	testutil.Require(t, "two actions", len(kept), 2)
	testutil.Expect(t, "the open is offered", kept[0].Offered, true)
	// An open's title is the intent's first sentence, which is what the pages
	// call a goal that has one.
	testutil.Expect(t, "titled from its own intent", kept[0].Title,
		"Every refund lands within a day, with nobody touching the queue")
	testutil.Expect(t, "the body carries its own id", kept[0].Fields[uitools.FieldID], "refund-worker")
	testutil.Expect(t, "and the edge naming it is offered too", kept[1].Offered, true)
	testutil.Expect(t, "with nothing to explain", kept[1].Reason, "")
}

// An act on a goal nothing carries, and an open of an id the tip already has,
// are recorded as refused with their reasons — never dropped.
func TestAnActionTheTipCannotCarryIsRecordedWithItsReason(t *testing.T) {
	t.Parallel()
	service := serviceProposing(t,
		proposed(uitools.ProposePark, "no-such-goal",
			[]string{uitools.ProposalBecause + "away"}, "put it away"),
		proposed(uitools.ProposeOpen, "refunds", []string{
			uitools.ProposalIntent + "Refunds land within a day.",
			uitools.ProposalNextStep + "Read it.", uitools.ProposalID + "refunds",
			uitools.ProposalSeverity + "1", uitools.ProposalNovelty + "1",
			uitools.ProposalExposure + "1", uitools.ProposalAccumulation + "1",
			uitools.ProposalBasis + "b",
		}, "create it"),
		proposed(uitools.ProposeBlock, "refunds",
			[]string{uitools.ProposalBlocker + "nobody"}, "it waits"),
		proposed(uitools.ProposeUnpark, "refunds", nil, "back to the queue"),
	)
	_, kept := askProposing(t, service, "do these four")
	testutil.Require(t, "all four are recorded", len(kept), 4)

	testutil.Expect(t, "an act on an unknown goal is not offered", kept[0].Offered, false)
	testutil.Expect(t, "with the tip's own reason", kept[0].Reason, "the accepted tip carries no goal no-such-goal")
	testutil.Expect(t, "and titled by its id where there is nothing else", kept[0].Title, "no-such-goal")

	testutil.Expect(t, "an open of an id the tip has is not offered", kept[1].Offered, false)
	testutil.Expect(t, "saying so", kept[1].Reason, "goal refunds already exists")

	testutil.Expect(t, "an edge naming an unknown blocker is not offered", kept[2].Offered, false)
	testutil.Expect(t, "naming the end that is missing", kept[2].Reason, "the accepted tip carries no goal nobody")

	// A refusal holds its place, so the index the outcome route names is the
	// index of every action the answer carried.
	testutil.Expect(t, "the fourth keeps its index", kept[3].Index, 3)
	testutil.Expect(t, "and is offered", kept[3].Offered, true)
}

// A frame whose fields are past the ledger's own bounds is refused here, and the
// refusal keeps none of the oversized value.
//
// The tool bounds every field it frames, and this bounds the FRAME, which is not
// the same check: a frame arrives as text a runtime reported, so a runtime that
// composed one itself could write a field of any length into the human's own
// transcript. It could, and the cost was the conversation: a propose naming a
// goal of three hundred thousand characters was refused for a goal nothing
// carries, the refusal persisted the whole id, and the next open of that
// transcript failed on the line — taking the Decisions page, which reads the same
// transcript, down with it (Astra B-01). So this is the one refusal that does not
// carry what it refused.
func TestAFramePastTheLedgersBoundsIsRefusedWithoutKeepingIt(t *testing.T) {
	t.Parallel()
	service := serviceProposing(t,
		proposed(uitools.ProposeUnpark, strings.Repeat("a", 300000), nil, "resume it"),
		proposed(uitools.ProposeEdit, "refunds",
			[]string{uitools.ProposalIntent + strings.Repeat("i", 300000)}, "reword it"),
		proposed(uitools.ProposeUnpark, "refunds", nil, "back to the queue"),
	)
	_, kept := askProposing(t, service, "do these three")
	testutil.Require(t, "all three are recorded", len(kept), 3)

	testutil.Expect(t, "an oversized goal is not offered", kept[0].Offered, false)
	testutil.Expect(t, "with the ledger's own bound as the reason",
		strings.Contains(kept[0].Reason, "a goal id is at most 64 bytes in the ledger"), true)
	testutil.Expect(t, "and the transcript keeps none of it", len(kept[0].Goal) < 100, true)

	testutil.Expect(t, "an oversized intent is not offered either", kept[1].Offered, false)
	testutil.Expect(t, "bounded by the sheets' own line",
		strings.Contains(kept[1].Reason, "carries at most 2000 characters"), true)
	testutil.Expect(t, "and none of it is kept", len(kept[1].Fields), 0)

	// A refusal holds its place, exactly as every other refusal does.
	testutil.Expect(t, "the third keeps its index", kept[2].Index, 2)
	testutil.Expect(t, "and is offered", kept[2].Offered, true)
}

// openingLines is one open's framing: the seven fields its route body cannot do
// without, its own id, and whatever edges the case is about.
func openingLines(id string, edges ...string) []string {
	return append([]string{
		uitools.ProposalIntent + "Every refund of " + id + " lands within a day.",
		uitools.ProposalNextStep + "Read the retry loop.",
		uitools.ProposalID + id,
		uitools.ProposalSeverity + "2", uitools.ProposalNovelty + "1",
		uitools.ProposalExposure + "2", uitools.ProposalAccumulation + "1",
		uitools.ProposalBasis + "payments, one team",
	}, edges...)
}

// An open's own two lists are checked exactly as a blocker is.
//
// They were not, and a blocker was: an open naming a goal nobody has in Blocked
// by or Blocks was offered whole, and the edge the human pressed for was refused
// by the act afterwards, which is the refusal this whole design moves to before
// the card exists (Sol's read of g1-s58, deferred). Each id has to be at the
// accepted tip or be the id of an open admitted earlier in the same answer — the
// same two ways the subject of every other act is there.
func TestAnOpensBlockedByAndBlocksAreCheckedAsABlockerIs(t *testing.T) {
	t.Parallel()
	service := serviceProposing(t,
		proposed(uitools.ProposeOpen, "refund-worker", openingLines("refund-worker"), "create it"),
		proposed(uitools.ProposeOpen, "refund-audit", openingLines("refund-audit",
			uitools.ProposalBlockedBy+"refund-worker, bank-sandbox",
			uitools.ProposalBlocks+"refunds"), "create it, between the two"),
		proposed(uitools.ProposeOpen, "refund-ledger", openingLines("refund-ledger",
			uitools.ProposalBlockedBy+"nobody"), "create it"),
		proposed(uitools.ProposeOpen, "refund-report", openingLines("refund-report",
			uitools.ProposalBlocks+"bank-sandbox, ghost"), "create it"),
	)
	_, kept := askProposing(t, service, "open these four")
	testutil.Require(t, "all four are recorded", len(kept), 4)

	testutil.Expect(t, "the plain open is offered", kept[0].Offered, true)

	// One list naming a goal at the tip and a goal this very answer opened, the
	// other naming a goal at the tip: every end is there, so the card stands.
	testutil.Expect(t, "an open whose edges are all there is offered", kept[1].Offered, true)
	testutil.Expect(t, "with nothing to explain", kept[1].Reason, "")

	testutil.Expect(t, "an open waiting for a goal nobody has is not offered", kept[2].Offered, false)
	testutil.Expect(t, "naming the end that is missing", kept[2].Reason,
		"the accepted tip carries no goal nobody")

	// And the second name of a list is read as well as the first, which is the
	// whole point of reading a list rather than a field.
	testutil.Expect(t, "an open blocking a goal nobody has is not offered", kept[3].Offered, false)
	testutil.Expect(t, "naming that end", kept[3].Reason, "the accepted tip carries no goal ghost")
}

// Admission checks existence and nothing else: whether the act is allowed in the
// goal's state is the engine's answer at the act.
func TestAdmissionJudgesExistenceAndNotTheGoalsState(t *testing.T) {
	t.Parallel()
	// An unpark of a goal that is not parked, and an approve of one already
	// approved: both would be answered by the engine, so both are offered here.
	service := serviceProposing(t,
		proposed(uitools.ProposeUnpark, "refunds", nil, "return it"),
		proposed(uitools.ProposeApprove, "refunds", nil, "approve it"),
	)
	_, kept := askProposing(t, service, "return it and approve it")
	testutil.Require(t, "both are recorded", len(kept), 2)
	testutil.Expect(t, "the unpark of an unparked goal is offered", kept[0].Offered, true)
	testutil.Expect(t, "the approve of an approved goal is offered", kept[1].Offered, true)
}

// The running turn carries them, so a reload mid-answer shows the card that is
// filling rather than losing the lines that have arrived.
func TestTheSnapshotCarriesARunningTurnsProposals(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	host := partner.NewHostOn(
		partner.Runtime{Name: "fake", ReadOnly: "a fake server reads nothing"},
		root, fakeacp.Open(fakeacp.Script{
			Reads: []fakeacp.Read{proposed(uitools.ProposePark, "fleet-presence",
				[]string{uitools.ProposalBecause + "away"}, "put it away")},
			Chunks: []string{"one ", "two ", "three"},
			Pause:  20 * time.Millisecond,
		}))
	t.Cleanup(host.Close)
	service := partner.NewService(host.Runtime(), host,
		func(human string) (*partner.Conversation, error) { return partner.OpenConversation(root, human) },
		partner.Facts{Observe: proposingLedger},
		func() time.Time { return time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC) })

	events, stop := service.Subscribe()
	defer stop()
	_, err := service.Submit(context.Background(), "Wido", "key-1", "put them away", onDecisions())
	testutil.Require(t, "the turn is admitted", err, nil)
	// The beat that carries the proposal, and the snapshot taken while the turn
	// is still running.
	for beat := range events {
		if beat.Kind == partner.EventProposal {
			break
		}
	}
	read, err := service.Snapshot("Wido", 100)
	testutil.Require(t, "the snapshot reads back", err, nil)
	testutil.Expect(t, "the turn is running", read.Busy, true)
	testutil.Require(t, "and carries the proposal", len(read.Proposals), 1)
	testutil.Expect(t, "waiting for the human", read.Proposals[0].State, partner.ProposalWaiting)
	drain(t, events)
}

// The next question is told what happened to what the last answers proposed,
// from the states the messages record rather than from what the Partner prepared.
func TestTheNextQuestionIsToldWhatHappenedToTheProposals(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	opener, handed := fakeacp.OpenWatched(fakeacp.Script{
		Reads: []fakeacp.Read{
			proposed(uitools.ProposePark, "fleet-presence",
				[]string{uitools.ProposalBecause + "away"}, "put it away"),
			proposed(uitools.ProposePark, "no-such-goal",
				[]string{uitools.ProposalBecause + "away"}, "and this one"),
		},
		Chunks: []string{"proposed."},
	})
	host := partner.NewHostOn(
		partner.Runtime{Name: "fake", ReadOnly: "a fake server reads nothing"}, root, opener)
	t.Cleanup(host.Close)
	service := partner.NewService(host.Runtime(), host,
		func(human string) (*partner.Conversation, error) { return partner.OpenConversation(root, human) },
		partner.Facts{Observe: proposingLedger},
		func() time.Time { return time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC) })

	events, stop := service.Subscribe()
	_, err := service.Submit(context.Background(), "Wido", "key-1", "put them away", onDecisions())
	testutil.Require(t, "the first turn is admitted", err, nil)
	drain(t, events)
	stop()

	// The first prompt cannot carry the line: there was nothing proposed yet.
	first, had := handed.First()
	testutil.Require(t, "there was a first prompt", had, true)
	testutil.Expect(t, "which said nothing about proposals",
		strings.Contains(first, "What happened to the actions you proposed"), false)

	events, stop = service.Subscribe()
	_, err = service.Submit(context.Background(), "Wido", "key-2", "and now?", onDecisions())
	testutil.Require(t, "the second turn is admitted", err, nil)
	drain(t, events)
	stop()

	prompts := handed.Prompts()
	testutil.Require(t, "two prompts", len(prompts) >= 2, true)
	second := prompts[len(prompts)-1]
	testutil.Expect(t, "the block is there",
		strings.Contains(second, "What happened to the actions you proposed"), true)
	testutil.Expect(t, "counted by state, with the refusal named",
		strings.Contains(second,
			"Of the 2 actions you proposed, 1 waiting, 1 not offered "+
				"(park-goal no-such-goal: the accepted tip carries no goal no-such-goal)."), true)
	testutil.Expect(t, "and it says nothing was applied by the Partner",
		strings.Contains(second, "every one of them was the human's own press"), true)
}

// The instructions say it: an act is proposed, not made, and words alone propose
// nothing.
func TestTheSkillTellsThePartnerToPropose(t *testing.T) {
	t.Parallel()
	skill := partner.Skill()
	for _, said := range []string{
		"Call `propose` once per action, beside the answer you give in words",
		// The grammar it speaks: the public action and its own flags, with the
		// command's own help as the place to read them (g1-s62 D4).
		"the goal action's public name, the goal, the action's own flags as " +
			"`metasystem goal ACTION --help` shows them",
		"the interface has ten of those actions today and the tool names them",
		"Words alone propose nothing.",
		"do not do it and do not say it is done",
		"the card under your answer is the only place one is applied",
		"a goal you propose to open may be named by a later action of the same answer",
	} {
		testutil.Expect(t, "the skill says "+said, strings.Contains(skill, said), true)
	}
}

// An abandon carries the goal as it was read, and with it the live goals that
// wait for it — which is the whole of what its line tells the human before they
// press (Astra S64-01, S64-02).
func TestAnAbandonCarriesTheGoalAndItsLiveDependentsAsRead(t *testing.T) {
	t.Parallel()
	service := serviceProposing(t,
		proposed(uitools.ProposeAbandon, "fleet-presence", []string{
			uitools.ProposalBecause + "superseded by the seat inventory (g1-s42)",
			uitools.ProposalSuccessor + "refunds",
		}, "the inventory answers what this one was for"),
	)
	_, kept := askProposing(t, service, "this one will never be worked; retire it")

	testutil.Require(t, "one action", len(kept), 1)
	testutil.Expect(t, "the route it dispatches to", kept[0].Verb, uitools.ProposeAbandon)
	testutil.Expect(t, "the reason under the body's own name",
		kept[0].Fields[uitools.FieldBecause], "superseded by the seat inventory (g1-s42)")
	testutil.Expect(t, "the successor under its own",
		kept[0].Fields[uitools.FieldSuccessor], "refunds")
	read := kept[0].Read
	testutil.Require(t, "it carries the reading", read != nil, true)
	testutil.Expect(t, "the intent as read", read.Intent,
		"Fleet presence is read from the census, not polled. It has been for a week.")
	testutil.Expect(t, "the next step as read", read.NextStep, "Read the census.")
	testutil.Expect(t, "the tier as read", read.Tier, 2)
	testutil.Expect(t, "the labels as read", read.Labels, []string{"fleet", "browser-interface"})
	testutil.Expect(t, "and the live goals that wait for it", read.Dependents, []string{"census-format"})
}

// A goal nothing waits for carries no dependents at all, which is absent rather
// than empty: a reader never has to tell one from the other.
func TestAnAbandonOfAGoalNothingWaitsForCarriesNoDependents(t *testing.T) {
	t.Parallel()
	service := serviceProposing(t,
		proposed(uitools.ProposeAbandon, "bank-sandbox",
			[]string{uitools.ProposalBecause + "the sandbox is gone"}, "nothing waits for it"),
	)
	_, kept := askProposing(t, service, "this one will never be worked; retire it")

	testutil.Require(t, "one action", len(kept), 1)
	read := kept[0].Read
	testutil.Require(t, "it carries the reading", read != nil, true)
	testutil.Expect(t, "with no dependents", len(read.Dependents), 0)
	testutil.Expect(t, "and no successor was named",
		kept[0].Fields[uitools.FieldSuccessor], "")
}

// An edit's labels are the whole list its route takes, composed here from the
// delta the Partner proposed and the labels this admission read.
//
// It is here and nowhere else, and that is the point. The tool server has no
// ledger reading: `--label` adds and `--unlabel` removes, and what they add to
// and remove from is the goal as the tip holds it. So the delta travels under the
// public flags' own spellings and the command's own owner composes it here —
// `goal.ApplyLabelDelta`, the same function `metasystem goal edit --label` uses,
// so an edit proposed in the browser and an edit typed at a terminal compose one
// list.
func TestAnEditsLabelsAreComposedFromTheLabelsRead(t *testing.T) {
	t.Parallel()
	service := serviceProposing(t,
		proposed(uitools.ProposeEdit, "fleet-presence", []string{
			uitools.ProposalLabel + "payments",
			uitools.ProposalUnlabel + "fleet",
		}, "one moves"),
		// Every label removed IS how they are cleared: the composed list is
		// empty, which the route takes as an empty list rather than as silence.
		proposed(uitools.ProposeEdit, "refunds", []string{
			uitools.ProposalIntent + "Refunds land within a day, always.",
		}, "tighter"),
	)
	_, kept := askProposing(t, service, "move that label and tidy the other intent")
	testutil.Require(t, "two actions", len(kept), 2)

	testutil.Expect(t, "the delta is composed onto the labels read",
		kept[0].Fields[uitools.FieldLabels], "browser-interface, payments")
	_, adding := kept[0].Fields[uitools.FieldLabel]
	_, removing := kept[0].Fields[uitools.FieldUnlabel]
	testutil.Expect(t, "and the delta itself does not travel on", adding || removing, false)
	// The whole list is what the route takes, so the reading the card shows is
	// still the labels as they stood.
	testutil.Require(t, "the edit carries the reading", kept[0].Read != nil, true)
	testutil.Expect(t, "with the labels as read", kept[0].Read.Labels,
		[]string{"fleet", "browser-interface"})

	// An edit that says nothing about labels leaves them alone: no field at all.
	_, touched := kept[1].Fields[uitools.FieldLabels]
	testutil.Expect(t, "an edit that says nothing about labels carries none", touched, false)
}

// Clearing a goal's labels is naming every one of them to remove: the composed
// list is empty, and an empty list is what the route takes as "none".
func TestAnEditThatRemovesEveryLabelClearsThem(t *testing.T) {
	t.Parallel()
	service := serviceProposing(t,
		proposed(uitools.ProposeEdit, "fleet-presence", []string{
			uitools.ProposalUnlabel + "fleet, browser-interface",
		}, "the labels moved to the arc"),
	)
	_, kept := askProposing(t, service, "clear its labels")
	testutil.Require(t, "one action", len(kept), 1)
	testutil.Expect(t, "it is offered", kept[0].Offered, true)
	said, given := kept[0].Fields[uitools.FieldLabels]
	testutil.Expect(t, "the whole list is there", given, true)
	testutil.Expect(t, "and says nothing, which is how they are cleared", said, "")
}

// A label named in both lists is refused in the composing owner's own words,
// recorded on the card rather than dropped.
func TestALabelInBothListsIsRefusedInTheOwnersWords(t *testing.T) {
	t.Parallel()
	service := serviceProposing(t,
		proposed(uitools.ProposeEdit, "fleet-presence", []string{
			uitools.ProposalLabel + "fleet",
			uitools.ProposalUnlabel + "fleet",
		}, "both at once"),
	)
	_, kept := askProposing(t, service, "add and remove that label")
	testutil.Require(t, "one action", len(kept), 1)
	testutil.Expect(t, "it is not offered", kept[0].Offered, false)
	testutil.Expect(t, "and the owner's own words say why", kept[0].Reason,
		`label "fleet" cannot be both --label and --unlabel in one edit`)
}

// An edit of a goal an earlier open of the same answer named composes against
// that open's own labels: there is nothing at the tip to read them from.
func TestAnEditOfAGoalOpenedInTheSameAnswerComposesOnItsLabels(t *testing.T) {
	t.Parallel()
	service := serviceProposing(t,
		proposed(uitools.ProposeOpen, "refund-worker", []string{
			uitools.ProposalIntent + "Every refund lands within a day.",
			uitools.ProposalNextStep + "Read the retry loop.",
			uitools.ProposalID + "refund-worker",
			uitools.ProposalSeverity + "2", uitools.ProposalNovelty + "1",
			uitools.ProposalExposure + "2", uitools.ProposalAccumulation + "1",
			uitools.ProposalBasis + "payments, one team",
			uitools.ProposalLabels + "payments, robustness",
		}, "as discussed"),
		proposed(uitools.ProposeEdit, "refund-worker", []string{
			uitools.ProposalLabel + "queue",
			uitools.ProposalUnlabel + "robustness",
		}, "one more label, one fewer"),
	)
	_, kept := askProposing(t, service, "Create it, then fix its labels")
	testutil.Require(t, "two actions", len(kept), 2)
	testutil.Expect(t, "the open is offered", kept[0].Offered, true)
	testutil.Expect(t, "the edit is offered too", kept[1].Offered, true)
	testutil.Expect(t, "and composes on the labels the open carried",
		kept[1].Fields[uitools.FieldLabels], "payments, queue")
}
