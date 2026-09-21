package backlog

import (
	"encoding/json"
	"sort"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)

var observedAt = time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)

func liveGoal(id, state string) *goal.GoalFile {
	return &goal.GoalFile{
		Id: id, State: state, Intent: "Do " + id, Origin: "main",
		NextStep: "Start " + id + ".", OpenedAt: "2026-08-23T00:00:00Z", Revision: 3,
		History: []goal.HistoryLine{
			{At: "2026-08-23T00:00:00Z", Opid: "op-open", Verb: "open", Actor: "m1+coordinator"},
			{At: "2026-08-24T00:00:00Z", Opid: "op-edit", Verb: "edit", Actor: "m1+coordinator"},
		},
	}
}

func treeOf(files ...*goal.GoalFile) *goal.TreeGoals {
	tree := &goal.TreeGoals{
		Root:      &goal.RootRecord{Identity: "01ARZ3NDEKTSV4RRFFQ69G5FAV", FormatVersion: "2", SyncMode: goal.SyncLocal, Revision: 1},
		Live:      map[string]*goal.GoalFile{},
		Done:      map[string]*goal.GoalFile{},
		Abandoned: map[string]*goal.GoalFile{},
	}
	for _, file := range files {
		switch file.State {
		case goal.StateDone:
			tree.Done[file.Id] = file
		case goal.StateAbandoned:
			tree.Abandoned[file.Id] = file
		default:
			tree.Live[file.Id] = file
		}
	}
	return tree
}

func answered(ready, blocked, awaiting []string, refused map[string]string) Admission {
	admission := Admission{
		Answered: true,
		Ready:    map[string]bool{}, Blocked: map[string]bool{}, Awaiting: map[string]bool{},
		Refused: map[string]string{},
	}
	for _, id := range ready {
		admission.Ready[id] = true
	}
	for _, id := range blocked {
		admission.Blocked[id] = true
	}
	for _, id := range awaiting {
		admission.Awaiting[id] = true
	}
	for id, cause := range refused {
		admission.Refused[id] = cause
	}
	return admission
}

// relayed marks a goal approved by a relayed human word, whose admission
// therefore depends on the review date the horizon is judged against.
func relayed(f *goal.GoalFile, reviewBy string) *goal.GoalFile {
	f.State = goal.StateApproved
	f.Approved = &goal.ApprovalRecord{
		By: "human:wido", At: "2026-08-24T00:00:00Z", Revision: 2, Opid: "op-approve",
		Authority: goal.ApprovalAuthorityRelayed, ReviewBy: reviewBy,
	}
	return f
}

// TestLaneOfPlacesEveryRecordedShape walks every record this projection can
// meet and pins where it lands, what phase it reports, and what it admits it
// does not know.
func TestLaneOfPlacesEveryRecordedShape(t *testing.T) {
	t.Parallel()

	blockerLive := liveGoal("blocker-live", goal.StateQueued)
	blockerDone := liveGoal("blocker-done", goal.StateDone)

	cases := []struct {
		name      string
		file      *goal.GoalFile
		tree      *goal.TreeGoals
		horizon   goal.ApprovalHorizon
		admission Admission
		lane      Lane
		phase     string
		gaps      []string
	}{
		{
			name: "queued", file: liveGoal("q", goal.StateQueued),
			lane: LaneToDo, gaps: []string{"not approved"},
		},
		{
			name: "queued behind an open blocker",
			file: func() *goal.GoalFile {
				f := liveGoal("q-blocked", goal.StateQueued)
				f.Blocked = []string{"blocker-live"}
				return f
			}(),
			lane: LaneToDo, gaps: []string{"not approved"},
		},
		{
			name: "approved and ready", file: liveGoal("ready", goal.StateApproved),
			admission: answered([]string{"ready"}, nil, nil, nil),
			lane:      LaneReady,
		},
		{
			name: "approved behind a live blocker",
			file: func() *goal.GoalFile {
				f := liveGoal("blocked", goal.StateApproved)
				f.Blocked = []string{"blocker-live"}
				return f
			}(),
			admission: answered(nil, []string{"blocked"}, nil, nil),
			lane:      LaneWaiting,
		},
		{
			name: "approved behind a blocker that is no goal",
			file: func() *goal.GoalFile {
				f := liveGoal("blocked-unknown", goal.StateApproved)
				f.Blocked = []string{"nowhere"}
				return f
			}(),
			admission: answered(nil, []string{"blocked-unknown"}, nil, nil),
			lane:      LaneWaiting,
		},
		{
			name: "approved whose only blocker is done",
			file: func() *goal.GoalFile {
				f := liveGoal("cleared", goal.StateApproved)
				f.Blocked = []string{"blocker-done"}
				return f
			}(),
			admission: answered([]string{"cleared"}, nil, nil, nil),
			lane:      LaneReady,
		},
		{
			name:      "approved with a review date behind us",
			file:      relayed(liveGoal("expired", goal.StateApproved), "2026-08-01"),
			admission: answered(nil, nil, []string{"expired"}, nil),
			lane:      LaneToDo, gaps: []string{"approval expired: the review date 2026-08-01 has passed"},
		},
		{
			name: "approved, expired, and also blocked",
			file: func() *goal.GoalFile {
				f := relayed(liveGoal("expired-blocked", goal.StateApproved), "2026-08-01")
				f.Blocked = []string{"blocker-live"}
				return f
			}(),
			admission: answered(nil, nil, []string{"expired-blocked"}, nil),
			lane:      LaneToDo, gaps: []string{"approval expired: the review date 2026-08-01 has passed"},
		},
		{
			name:      "approved and refused by the claim gate",
			file:      liveGoal("refused", goal.StateApproved),
			admission: answered(nil, nil, nil, map[string]string{"refused": "APPROVAL_REQUIRED: goal refused has an invalid approval"}),
			lane:      LaneToDo, gaps: []string{"claim refused: APPROVAL_REQUIRED: goal refused has an invalid approval"},
		},
		{
			name:      "approved under an unanswerable frontier",
			file:      liveGoal("unanswered", goal.StateApproved),
			admission: Admission{Message: "cannot answer claimable backlog: resolve metasystem.budget.tier-3"},
			lane:      LaneUnknown,
			gaps:      []string{"claim admission not answered: cannot answer claimable backlog: resolve metasystem.budget.tier-3"},
		},
		{
			name:      "approved and placed in no bucket at all",
			file:      liveGoal("unplaced", goal.StateApproved),
			admission: answered(nil, nil, nil, nil),
			lane:      LaneUnknown, gaps: []string{"the engine's frontier did not place this goal"},
		},
		{
			name: "claimed", file: func() *goal.GoalFile {
				f := liveGoal("claimed", goal.StateClaimed)
				f.Claimed = &goal.ClaimRecord{Machine: "m1", Lineage: "coordinator", At: "2026-08-25T00:00:00Z"}
				return f
			}(),
			lane: LaneInProgress, phase: PhaseNotRecorded, gaps: []string{"phase not recorded"},
		},
		{
			name: "claimed and waiting to land", file: func() *goal.GoalFile {
				f := liveGoal("landing", goal.StateClaimed)
				f.Claimed = &goal.ClaimRecord{Machine: "m1", Lineage: "coordinator", At: "2026-08-25T00:00:00Z"}
				f.Landing = &goal.LandingRecord{At: "2026-08-26T00:00:00Z", Opid: "op-land"}
				return f
			}(),
			lane: LaneReview, phase: "landing",
		},
		{
			name: "claimed and fenced", file: func() *goal.GoalFile {
				f := liveGoal("fenced", goal.StateClaimed)
				f.Claimed = &goal.ClaimRecord{Machine: "m1", Lineage: "coordinator", At: "2026-08-25T00:00:00Z"}
				f.StopFence = &goal.StopFence{StopID: "stop-1", ClosedAt: "2026-08-27T00:00:00Z", Reason: goal.StopReasonElapsedLimit}
				return f
			}(),
			lane: LaneWaiting, phase: PhaseNotRecorded,
		},
		{
			name: "claimed, landing, and fenced", file: func() *goal.GoalFile {
				f := liveGoal("fenced-landing", goal.StateClaimed)
				f.Claimed = &goal.ClaimRecord{Machine: "m1", Lineage: "coordinator", At: "2026-08-25T00:00:00Z"}
				f.Landing = &goal.LandingRecord{At: "2026-08-26T00:00:00Z", Opid: "op-land"}
				f.StopFence = &goal.StopFence{StopID: "stop-1", ClosedAt: "2026-08-27T00:00:00Z", Reason: goal.StopReasonElapsedLimit}
				return f
			}(),
			lane: LaneWaiting, phase: PhaseNotRecorded,
		},
		{
			name: "parked over another pair's claim", file: func() *goal.GoalFile {
				f := liveGoal("parked-displaced", goal.StateParked)
				f.Parked = &goal.ParkRecord{By: "human:wido", At: "2026-08-28T00:00:00Z", Because: "waiting on a ruling", Displaced: "m2+builder@2026-08-27T00:00:00Z"}
				return f
			}(),
			lane: LaneWaiting, phase: PhaseNotRecorded,
		},
		{
			name: "parked over the pair's own claim", file: func() *goal.GoalFile {
				f := liveGoal("parked-own", goal.StateParked)
				f.Parked = &goal.ParkRecord{By: "m1+coordinator", At: "2026-08-28T00:00:00Z", Because: "waiting on a ruling"}
				f.Episode = &goal.EpisodeRecord{Machine: "m1", Lineage: "coordinator", Released: "2026-08-28T00:00:00Z"}
				return f
			}(),
			lane: LaneWaiting, phase: PhaseNotRecorded,
		},
		{
			name: "parked with an older episode", file: func() *goal.GoalFile {
				f := liveGoal("parked-older", goal.StateParked)
				f.Parked = &goal.ParkRecord{By: "m1+coordinator", At: "2026-08-28T00:00:00Z", Because: "waiting on a ruling"}
				f.Episode = &goal.EpisodeRecord{Machine: "m1", Lineage: "coordinator", Released: "2026-08-20T00:00:00Z"}
				return f
			}(),
			lane: LaneWaiting, phase: PhaseNotRecorded,
		},
		{
			name: "parked from rest", file: func() *goal.GoalFile {
				f := liveGoal("parked-rest", goal.StateParked)
				f.Parked = &goal.ParkRecord{By: "human:wido", At: "2026-08-28T00:00:00Z", Because: "not now"}
				return f
			}(),
			lane: LaneWaiting, phase: PhaseNotRecorded,
		},
		{name: "done", file: liveGoal("done", goal.StateDone), lane: LaneDone},
		{name: "abandoned", file: liveGoal("abandoned", goal.StateAbandoned), lane: LaneAbandoned},
		{
			name: "a state this build does not know", file: liveGoal("weird", "weird"),
			lane: LaneUnknown, gaps: []string{"state weird is not placed by this build"},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			tree := testCase.tree
			if tree == nil {
				tree = treeOf(blockerLive, blockerDone, testCase.file)
			}
			horizon := testCase.horizon
			if horizon.Now.IsZero() {
				horizon = goal.NewApprovalHorizon(tree, observedAt)
			}
			lane, phase, gaps := LaneOf(testCase.file, tree, horizon, testCase.admission)
			testutil.Expect(t, "lane", lane, testCase.lane)
			testutil.Expect(t, "phase", phase, testCase.phase)
			testutil.Expect(t, "gaps", gaps, testCase.gaps)
		})
	}
}

// TestLaneOfNamesTheEnrolledTerminalWhenItExpiresAnApproval proves the gap
// carries the horizon's own reason, not a reason this package invented.
func TestLaneOfNamesTheEnrolledTerminalWhenItExpiresAnApproval(t *testing.T) {
	t.Parallel()
	file := relayed(liveGoal("enrolled", goal.StateApproved), "2036-01-01")
	tree := treeOf(file)
	tree.Root.FleetEnrollment = &goal.FleetEnrollmentRecord{At: "2026-08-01T00:00:00Z", Machine: "m1", Generation: 1, Opid: "op-enroll"}

	horizon := goal.NewApprovalHorizon(tree, observedAt)
	lane, _, gaps := LaneOf(file, tree, horizon, answered(nil, nil, []string{"enrolled"}, nil))

	testutil.Expect(t, "lane", lane, LaneToDo)
	testutil.Expect(t, "gaps", gaps, []string{"approval expired: the fleet's first terminal was enrolled at 2026-08-01T00:00:00Z"})
}

func TestRowCarriesTheRecordsOwnFacts(t *testing.T) {
	t.Parallel()

	t.Run("a parked goal keeps its reason and the state it left", func(t *testing.T) {
		t.Parallel()
		file := liveGoal("parked", goal.StateParked)
		file.Parked = &goal.ParkRecord{By: "human:wido", At: "2026-08-28T00:00:00Z", Because: "waiting on a ruling", Displaced: "m2+builder@2026-08-27T00:00:00Z", Blocker: "other"}
		tree := treeOf(file)
		board := Project(tree, goal.NewApprovalHorizon(tree, observedAt), answered(nil, nil, nil, nil))

		testutil.Require(t, "rows", len(board.Rows), 1)
		row := board.Rows[0]
		testutil.Expect(t, "waiting", row.Waiting, &Waiting{
			Reason: "waiting on a ruling", Since: "2026-08-28T00:00:00Z", By: "human:wido",
			From: goal.StateClaimed, Blocker: "other",
		})
		testutil.Expect(t, "identity", row.Ref, Ref{Kind: "goal", ID: "parked", Revision: 3})
		testutil.Expect(t, "the last history line", []string{row.LastVerb, row.LastChangeAt}, []string{"edit", "2026-08-24T00:00:00Z"})
	})

	t.Run("a park that proves nothing says so", func(t *testing.T) {
		t.Parallel()
		file := liveGoal("parked", goal.StateParked)
		file.Parked = &goal.ParkRecord{By: "human:wido", At: "2026-08-28T00:00:00Z", Because: "not now"}
		tree := treeOf(file)
		board := Project(tree, goal.NewApprovalHorizon(tree, observedAt), answered(nil, nil, nil, nil))

		testutil.Expect(t, "waiting from", board.Rows[0].Waiting.From, PhaseNotRecorded)
	})

	t.Run("open blockers exclude only concluded work", func(t *testing.T) {
		t.Parallel()
		file := liveGoal("blocked", goal.StateApproved)
		file.Blocked = []string{"done-one", "live-one", "dropped-one", "nowhere"}
		tree := treeOf(file,
			liveGoal("done-one", goal.StateDone),
			liveGoal("live-one", goal.StateQueued),
			liveGoal("dropped-one", goal.StateAbandoned))
		board := Project(tree, goal.NewApprovalHorizon(tree, observedAt), answered(nil, []string{"blocked"}, nil, nil))

		row := board.Rows[0]
		testutil.Expect(t, "blocked by", row.BlockedBy, []string{"done-one", "live-one", "dropped-one", "nowhere"})
		testutil.Expect(t, "open blockers", row.OpenBlockers, []string{"live-one", "dropped-one", "nowhere"})
		testutil.Expect(t, "gaps", row.Gaps, []string{"blocker nowhere is an unknown goal"})
		testutil.Expect(t, "waiting from", row.Waiting, &Waiting{From: goal.StateApproved})
	})

	t.Run("a split parent is marked, not called delivered", func(t *testing.T) {
		t.Parallel()
		file := liveGoal("split-parent", goal.StateDone)
		tree := treeOf(file)
		tree.Root.Decomposed = []goal.DecomposedEntry{{Id: "split-parent"}}
		board := Project(tree, goal.NewApprovalHorizon(tree, observedAt), answered(nil, nil, nil, nil))

		testutil.Require(t, "closed rows", len(board.Closed), 1)
		testutil.Expect(t, "decomposed", board.Closed[0].Decomposed, true)
		testutil.Expect(t, "lane", board.Closed[0].Lane, LaneDone)
	})

	t.Run("an abandoned record keeps its reason", func(t *testing.T) {
		t.Parallel()
		file := liveGoal("dropped", goal.StateAbandoned)
		file.Abandoned = &goal.AbandonRecord{By: "human:wido", At: "2026-08-29T00:00:00Z", Because: "overtaken"}
		tree := treeOf(file)
		board := Project(tree, goal.NewApprovalHorizon(tree, observedAt), answered(nil, nil, nil, nil))

		testutil.Expect(t, "abandoned", board.Closed[0].Abandoned, &Abandoned{By: "human:wido", At: "2026-08-29T00:00:00Z", Because: "overtaken"})
	})

	t.Run("a fenced claim carries both records", func(t *testing.T) {
		t.Parallel()
		file := liveGoal("fenced", goal.StateClaimed)
		file.Claimed = &goal.ClaimRecord{Machine: "m1", Lineage: "coordinator", At: "2026-08-25T00:00:00Z"}
		file.Landing = &goal.LandingRecord{At: "2026-08-26T00:00:00Z", Opid: "op-land"}
		file.StopFence = &goal.StopFence{StopID: "stop-1", ClosedAt: "2026-08-27T00:00:00Z", Reason: goal.StopReasonElapsedLimit}
		tree := treeOf(file)
		board := Project(tree, goal.NewApprovalHorizon(tree, observedAt), answered(nil, nil, nil, nil))

		row := board.Rows[0]
		testutil.Expect(t, "claim", row.Claim, &Claim{Machine: "m1", Lineage: "coordinator", At: "2026-08-25T00:00:00Z", LandingAt: "2026-08-26T00:00:00Z"})
		testutil.Expect(t, "fence", row.Fence, &Fence{Reason: goal.StopReasonElapsedLimit, ClosedAt: "2026-08-27T00:00:00Z"})
		testutil.Expect(t, "lane", row.Lane, LaneWaiting)
	})
}

// TestProjectPlacesEveryGoalOnce is the projection's own contract: no record
// is dropped, none is shown twice, and the counts add up to the tree.
func TestProjectPlacesEveryGoalOnce(t *testing.T) {
	t.Parallel()

	ranked := liveGoal("ranked", goal.StateQueued)
	ranked.Priority, ranked.Sequence = 1, 1
	alsoRanked := liveGoal("also-ranked", goal.StateQueued)
	alsoRanked.Priority, alsoRanked.Sequence = 1, 2
	claimed := liveGoal("claimed", goal.StateClaimed)
	claimed.Claimed = &goal.ClaimRecord{Machine: "m1", Lineage: "coordinator", At: "2026-08-25T00:00:00Z"}
	tree := treeOf(ranked, alsoRanked, claimed,
		liveGoal("unranked", goal.StateQueued),
		liveGoal("zebra-done", goal.StateDone),
		liveGoal("alpha-done", goal.StateDone),
		liveGoal("zebra-dropped", goal.StateAbandoned),
		liveGoal("alpha-dropped", goal.StateAbandoned))

	board := Project(tree, goal.NewApprovalHorizon(tree, observedAt), answered(nil, nil, nil, nil))

	testutil.Expect(t, "live order", ids(board.Rows), goal.OrderedOpenGoalIDs(tree.Live))
	testutil.Expect(t, "closed order", ids(board.Closed),
		[]string{"alpha-done", "zebra-done", "alpha-dropped", "zebra-dropped"})

	seen := map[string]int{}
	for _, row := range append(append([]Row{}, board.Rows...), board.Closed...) {
		seen[row.ID]++
	}
	testutil.Expect(t, "every goal once", len(seen), len(tree.Live)+len(tree.Done)+len(tree.Abandoned))
	for id, count := range seen {
		if count != 1 {
			t.Fatalf("goal %s appears %d times", id, count)
		}
	}

	live, closed := 0, 0
	for lane, count := range board.Counts {
		switch lane {
		case LaneDone, LaneAbandoned:
			closed += count
		default:
			live += count
		}
	}
	testutil.Expect(t, "live counts", live, len(tree.Live))
	testutil.Expect(t, "closed counts", closed, len(tree.Done)+len(tree.Abandoned))

	for _, lane := range LaneOrder {
		if lane == LaneDraft {
			if _, counted := board.Counts[lane]; counted {
				t.Fatalf("the draft lane carries a count")
			}
			continue
		}
		if _, counted := board.Counts[lane]; !counted {
			t.Fatalf("lane %s has no count", lane)
		}
	}
	testutil.Expect(t, "the draft statement", board.Draft.Statement, DraftStatement)
}

func TestProjectAnswersAnEmptyTreeWithEmptyCollections(t *testing.T) {
	t.Parallel()
	board := Project(nil, goal.NewApprovalHorizon(nil, observedAt), Admission{})
	encoded, err := json.Marshal(board)
	testutil.Require(t, "encode", err, nil)
	testutil.Expect(t, "the empty board", string(encoded),
		`{"rows":[],"closed":[],"counts":{"abandoned":0,"done":0,"in-progress":0,"ready":0,"review":0,"to-do":0,"unknown":0,"waiting":0},"draft":{"statement":"`+DraftStatement+`"}}`)
}

// TestRowFieldNames pins the wire shape a reader parses.
func TestRowFieldNames(t *testing.T) {
	t.Parallel()
	file := liveGoal("named", goal.StateClaimed)
	file.Claimed = &goal.ClaimRecord{Machine: "m1", Lineage: "coordinator", At: "2026-08-25T00:00:00Z"}
	file.StopFence = &goal.StopFence{StopID: "stop-1", ClosedAt: "2026-08-27T00:00:00Z", Reason: goal.StopReasonElapsedLimit}
	file.Parked = &goal.ParkRecord{By: "human:wido", At: "2026-08-28T00:00:00Z", Because: "held"}
	file.Abandoned = &goal.AbandonRecord{By: "human:wido", At: "2026-08-29T00:00:00Z", Because: "overtaken"}
	file.Approved = &goal.ApprovalRecord{By: "human:wido", At: "2026-08-24T00:00:00Z", Authority: goal.ApprovalAuthorityProven}
	tree := treeOf(file)
	board := Project(tree, goal.NewApprovalHorizon(tree, observedAt), answered(nil, nil, nil, nil))

	encoded, err := json.Marshal(board.Rows[0])
	testutil.Require(t, "encode", err, nil)
	var decoded map[string]json.RawMessage
	testutil.Require(t, "decode", json.Unmarshal(encoded, &decoded), nil)
	testutil.Expect(t, "row field names", keysOf(decoded), []string{
		"abandoned", "approved", "arc", "blockedBy", "claim", "concluded", "decomposed",
		"fence", "gaps", "intent", "labels", "lane", "lastChangeAt", "lastVerb", "nextStep",
		"openBlockers", "openedAt", "origin", "phase", "pinned", "priority", "ref", "sequence",
		"sliced", "state", "tier", "waiting", "where",
	})

	var identity map[string]json.RawMessage
	testutil.Require(t, "decode the reference", json.Unmarshal(decoded["ref"], &identity), nil)
	testutil.Expect(t, "reference field names", keysOf(identity), []string{"id", "kind", "revision"})
}

func TestRowOmitsEveryAbsentRecord(t *testing.T) {
	t.Parallel()
	tree := treeOf(liveGoal("bare", goal.StateQueued))
	board := Project(tree, goal.NewApprovalHorizon(tree, observedAt), answered(nil, nil, nil, nil))

	encoded, err := json.Marshal(board.Rows[0])
	testutil.Require(t, "encode", err, nil)
	var decoded map[string]json.RawMessage
	testutil.Require(t, "decode", json.Unmarshal(encoded, &decoded), nil)
	for _, absent := range []string{"approved", "claim", "waiting", "abandoned", "fence"} {
		if _, present := decoded[absent]; present {
			t.Fatalf("a record the goal does not carry was serialized: %s", absent)
		}
	}
	testutil.Expect(t, "empty lists serialize as lists",
		[]string{string(decoded["labels"]), string(decoded["blockedBy"]), string(decoded["openBlockers"])},
		[]string{"[]", "[]", "[]"})
}

func ids(rows []Row) []string {
	out := make([]string, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.ID)
	}
	return out
}

func keysOf(object map[string]json.RawMessage) []string {
	keys := make([]string, 0, len(object))
	for key := range object {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
