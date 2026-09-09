package goal

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
)

func TestNextPriority(t *testing.T) {
	root := t.TempDir()
	seedGoalNormConfig(t, root)

	t.Run("pins", func(t *testing.T) {
		a := nextPriorityGoal("a", 1, 1, "m1")
		b := nextPriorityGoal("b", 1, 2, "")
		c := nextPriorityGoal("c", 1, 3, "m2")
		projection := Projection{Root: root, Tree: &TreeGoals{Live: map[string]*GoalFile{
			a.Id: a, b.Id: b, c.Id: c,
		}, Done: map[string]*GoalFile{}}}

		for _, test := range []struct {
			machine string
			want    string
		}{
			{machine: "m1", want: "a"},
			{machine: "m2", want: "b"},
			{machine: "m3", want: "b"},
		} {
			frontier, err := Next(projection, test.machine)
			if err != nil {
				t.Fatalf("machine %s frontier: %v", test.machine, err)
			}
			selection := SelectNext(frontier)
			if selection.Kind != NextSelectionReady || selection.GoalID != test.want {
				t.Fatalf("machine %s selection = %+v, want ready %s; frontier=%+v", test.machine, selection, test.want, frontier)
			}
		}
	})

	t.Run("held", func(t *testing.T) {
		heldA := vGoal("held-a", StateClaimed)
		heldA.Priority, heldA.Sequence, heldA.Arc = 1, 1, "held-arc"
		heldA.Labels = []string{"hidden-by-filter"}
		heldB := vGoal("held-b", StateClaimed)
		heldB.Priority, heldB.Sequence, heldB.Arc = 1, 2, "held-arc"
		heldB.Labels = []string{"hidden-by-filter"}
		ready := nextPriorityGoal("ready", 1, 3, "")
		ready.Labels = []string{"wanted"}
		projection := Projection{Root: root, Tree: &TreeGoals{Live: map[string]*GoalFile{
			heldA.Id: heldA, heldB.Id: heldB, ready.Id: ready,
		}, Done: map[string]*GoalFile{}}}

		frontier, err := Next(projection, "mac-a", "wanted")
		if err != nil {
			t.Fatal(err)
		}
		selection := SelectNext(frontier)
		if got := strings.Join(frontier.Claimed, ","); got != "held-a,held-b" ||
			strings.Join(frontier.Ready, ",") != "ready" || selection.Kind != NextSelectionContinue || selection.GoalID != "held-a" {
			t.Fatalf("one held arc did not outrank the retained ready frontier: selection=%+v frontier=%+v", selection, frontier)
		}
	})

	t.Run("blocked-expired", func(t *testing.T) {
		blocked := nextPriorityGoal("blocked", 1, 1, "")
		blocked.Blocked = []string{"dependency"}
		expired := nextPriorityGoal("expired", 1, 2, "")
		expired.Approved.Authority = ApprovalAuthorityRelayed
		expired.Approved.ReviewBy = "2026-09-01"
		ready := nextPriorityGoal("ready", 1, 3, "")
		dependency := vGoal("dependency", StateQueued)
		projection := Projection{
			Root: root,
			Tree: &TreeGoals{Live: map[string]*GoalFile{
				blocked.Id: blocked, expired.Id: expired, ready.Id: ready, dependency.Id: dependency,
			}, Done: map[string]*GoalFile{}},
			Horizon: ApprovalHorizon{Now: time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC)},
		}

		frontier, err := Next(projection, "m1")
		if err != nil {
			t.Fatal(err)
		}
		selection := SelectNext(frontier)
		if strings.Join(frontier.Blocked, ",") != "blocked" || strings.Join(frontier.Awaiting, ",") != "expired,dependency" ||
			selection.Kind != NextSelectionReady || selection.GoalID != "ready" {
			t.Fatalf("blocked or expired work displaced the eligible ranked goal: selection=%+v frontier=%+v", selection, frontier)
		}
	})

	t.Run("none", func(t *testing.T) {
		foreign := nextPriorityGoal("foreign", 1, 1, "other-machine")
		parked := vGoal("parked", StateParked)
		projection := Projection{Root: root, Tree: &TreeGoals{Live: map[string]*GoalFile{
			foreign.Id: foreign, parked.Id: parked,
		}, Done: map[string]*GoalFile{}}}

		frontier, err := Next(projection, "m1")
		if err != nil {
			t.Fatal(err)
		}
		if selection := SelectNext(frontier); selection.Kind != NextSelectionNone || selection.GoalID != "" {
			t.Fatalf("an ineligible frontier returned a candidate: selection=%+v frontier=%+v", selection, frontier)
		}
	})

	t.Run("labels", func(t *testing.T) {
		higher := nextPriorityGoal("higher", 1, 1, "")
		higher.Labels = []string{"other"}
		lower := nextPriorityGoal("lower", 1, 2, "")
		lower.Labels = []string{"wanted"}
		projection := Projection{Root: root, Tree: &TreeGoals{Live: map[string]*GoalFile{
			higher.Id: higher, lower.Id: lower,
		}, Done: map[string]*GoalFile{}}}

		frontier, err := Next(projection, "m1", "wanted")
		if err != nil {
			t.Fatal(err)
		}
		selection := SelectNext(frontier)
		if selection.Kind != NextSelectionReady || selection.GoalID != "lower" || strings.Join(frontier.Ready, ",") != "lower" {
			t.Fatalf("the label filter did not retain ranked traversal: selection=%+v frontier=%+v", selection, frontier)
		}
	})

	t.Run("over-norm", func(t *testing.T) {
		overNormBudget := testBudget()
		overNormBudget.ReservedJobMinutesLimit = 2400
		overNorm := approvedGoalFixture(vGoal("over-norm", StateQueued), overNormBudget)
		overNorm.Priority, overNorm.Sequence = 1, 1
		claimable := nextPriorityGoal("claimable", 1, 2, "")
		projection := Projection{Root: root, Tree: &TreeGoals{Live: map[string]*GoalFile{
			overNorm.Id: overNorm, claimable.Id: claimable,
		}, Done: map[string]*GoalFile{}}}
		if _, err := requireApprovedForClaim(root, projection.Tree, overNorm, projection.Horizon.Now, "claim"); err == nil || !strings.Contains(err.Error(), "GOAL_NORM_REFUSED") {
			t.Fatalf("over-norm fixture did not isolate the norm refusal: %v", err)
		}
		if _, err := requireApprovedForClaim(root, projection.Tree, claimable, projection.Horizon.Now, "claim"); err != nil {
			t.Fatalf("lower-ranked fixture was not otherwise claimable: %v", err)
		}

		frontier, err := Next(projection, "m1")
		if err != nil {
			t.Fatal(err)
		}
		selection := SelectNext(frontier)
		if selection.Kind != NextSelectionReady || selection.GoalID != "claimable" {
			t.Fatalf("over-norm head blocked claimable ranked work: selection=%+v frontier=%+v", selection, frontier)
		}
	})

	t.Run("configuration-failure-keeps-idle-refusal", func(t *testing.T) {
		candidate := nextPriorityGoal("candidate", 1, 1, "")
		root := servingBed(t, "bed-m1", map[string]*GoalFile{candidate.Id: candidate})
		if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte(config.Tier3BudgetKey+"=malformed\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		projection, err := Project(Endpoint{Root: root, Remote: "local", Branch: LocalLedgerBranch}, false, time.Now())
		if err != nil {
			t.Fatal(err)
		}
		frontier, frontierErr := Next(projection, "bed-m1")
		if frontierErr == nil || !strings.Contains(frontierErr.Error(), config.Tier3BudgetKey) || len(frontier.Ready) != 0 {
			t.Fatalf("configuration uncertainty became a frontier: frontier=%+v err=%v", frontier, frontierErr)
		}

		work, workErr := ReadClaimableBudgetedWork(root, time.Now())
		if workErr == nil || !strings.Contains(workErr.Error(), config.Tier3BudgetKey) {
			t.Fatalf("claimable-work read swallowed configuration uncertainty: work=%+v err=%v", work, workErr)
		}
		verdict := Verdict{}
		session := &sessionState{}
		(&Store{}).enforceIdleBacklog(&verdict, &work, workErr, session, "session", "main", TurnVerdictOptions{})
		if !verdict.ShouldBlock || !verdict.IdleRefusal || session.IdleBlocks != 1 || !strings.Contains(verdict.Display, "IDLE WITH BACKLOG cannot be ruled out") {
			t.Fatalf("indeterminate backlog disabled idle refusal: verdict=%+v session=%+v", verdict, session)
		}
	})

	t.Run("configuration-loads-once", func(t *testing.T) {
		previous := loadClaimAdmissionTierBoxes
		loads := 0
		loadClaimAdmissionTierBoxes = func(path string) (*config.TierBoxSet, error) {
			loads++
			return previous(path)
		}
		defer func() { loadClaimAdmissionTierBoxes = previous }()

		root := t.TempDir()
		seedGoalNormConfig(t, root)
		one := nextPriorityGoal("one", 1, 1, "")
		twoFile := vGoal("two", StateQueued)
		twoFile.Tier = 2
		two := approvedGoalFixture(twoFile, testBudget())
		two.Priority, two.Sequence = 1, 2
		threeFile := vGoal("three", StateQueued)
		threeFile.Tier = 1
		three := approvedGoalFixture(threeFile, testBudget())
		three.Priority, three.Sequence = 1, 3
		projection := Projection{Root: root, Tree: &TreeGoals{Live: map[string]*GoalFile{
			one.Id: one, two.Id: two, three.Id: three,
		}, Done: map[string]*GoalFile{}}}
		frontier, err := Next(projection, "m1")
		if err != nil || strings.Join(frontier.Ready, ",") != "one,two,three" || loads != 1 {
			t.Fatalf("multi-goal frontier loaded configuration %d times: frontier=%+v err=%v", loads, frontier, err)
		}
	})
}

func nextPriorityGoal(id string, priority uint8, sequence uint64, pin string) *GoalFile {
	file := approvedGoalFixture(vGoal(id, StateQueued), testBudget())
	file.Priority, file.Sequence, file.Pinned = priority, sequence, pin
	return file
}

func TestPriorityUnranked(t *testing.T) {
	t.Run("sort-last", func(t *testing.T) {
		unrankedA := vGoal("a-unranked", StateQueued)
		unrankedB := vGoal("b-unranked", StateParked)
		rankedZ := vGoal("z-ranked", StateApproved)
		rankedZ.Priority = 3
		rankedZ.Sequence = 1
		live := map[string]*GoalFile{
			unrankedA.Id: unrankedA,
			unrankedB.Id: unrankedB,
			rankedZ.Id:   rankedZ,
		}

		if got, want := OrderedOpenGoalIDs(live), []string{"z-ranked", "a-unranked", "b-unranked"}; !reflect.DeepEqual(got, want) {
			t.Fatalf("ordered open goals = %v, want %v", got, want)
		}
	})

	t.Run("grammar", func(t *testing.T) {
		base := vGoal("rank-grammar", StateQueued)
		base.Priority = 1
		base.Sequence = ^uint64(0)
		rendered := RenderFile(base)
		stateAt := strings.Index(string(rendered), "- State: queued\n")
		priorityAt := strings.Index(string(rendered), "- Priority: 1\n")
		sequenceAt := strings.Index(string(rendered), "- Sequence: 18446744073709551615\n")
		if stateAt < 0 || priorityAt <= stateAt || sequenceAt <= priorityAt {
			t.Fatalf("rank fields are not rendered immediately after State:\n%s", rendered)
		}
		if parsed, problems := ParseFile(rendered); len(problems) != 0 || parsed.Priority != 1 || parsed.Sequence != ^uint64(0) {
			t.Fatalf("maximum lawful rank did not round-trip: file=%+v problems=%v", parsed, problems)
		}

		for _, test := range []struct {
			name     string
			fields   string
			fragment string
		}{
			{name: "priority alone", fields: "- Priority: 1\n", fragment: "both be present"},
			{name: "sequence alone", fields: "- Sequence: 1\n", fragment: "both be present"},
			{name: "empty priority", fields: "- Priority: \n- Sequence: 1\n", fragment: "Priority"},
			{name: "empty sequence", fields: "- Priority: 1\n- Sequence: \n", fragment: "Sequence"},
			{name: "signed priority", fields: "- Priority: +1\n- Sequence: 1\n", fragment: "Priority"},
			{name: "signed sequence", fields: "- Priority: 1\n- Sequence: -1\n", fragment: "Sequence"},
			{name: "fraction", fields: "- Priority: 1\n- Sequence: 1.5\n", fragment: "Sequence"},
			{name: "overflow", fields: "- Priority: 1\n- Sequence: 18446744073709551616\n", fragment: "Sequence"},
			{name: "zero priority", fields: "- Priority: 0\n- Sequence: 1\n", fragment: "Priority"},
			{name: "out of scale", fields: "- Priority: 4\n- Sequence: 1\n", fragment: "Priority"},
			{name: "zero sequence", fields: "- Priority: 1\n- Sequence: 0\n", fragment: "Sequence"},
			{name: "duplicate", fields: "- Priority: 1\n- Priority: 2\n- Sequence: 1\n", fragment: "duplicate field"},
		} {
			t.Run(test.name, func(t *testing.T) {
				_, problems := ParseFile(insertRankFields(vGoal("rank-grammar", StateQueued), test.fields))
				expectProblem(t, problems, test.fragment)
			})
		}

		duplicateA := vGoal("duplicate-a", StateQueued)
		duplicateA.Priority, duplicateA.Sequence = 1, 1
		duplicateB := vGoal("duplicate-b", StateQueued)
		duplicateB.Priority, duplicateB.Sequence = 1, 1
		expectProblem(t, problemsOf(vTree(vRoot(), []*GoalFile{duplicateA, duplicateB}, nil)), "priority 1")

		holeA := vGoal("hole-a", StateQueued)
		holeA.Priority, holeA.Sequence = 2, 1
		holeB := vGoal("hole-b", StateParked)
		holeB.Priority, holeB.Sequence = 2, 3
		expectProblem(t, problemsOf(vTree(vRoot(), []*GoalFile{holeA, holeB}, nil)), "priority 2")

		root := rankedGoalBed(t, map[string][2]uint64{"a": {1, 1}, "b": {1, 2}})
		tip := acceptedTip(t, root)
		tree, err := loadTree(root, tip)
		if err != nil {
			t.Fatal(err)
		}
		tree.Live["b"].Sequence = 3
		malformed, err := BuildCommit(endpointFor(root), "priority-malformed-fixture", tip, []Change{{
			Path: livePath("b"), Content: RenderFile(tree.Live["b"]),
		}}, "seed malformed accepted priority tree")
		if err != nil {
			t.Fatal(err)
		}
		mustGit(t, root, "push", "-q", "origin", malformed+":refs/heads/main")
		mustGit(t, root, "update-ref", LocalLedgerBranch, malformed)
		mustGit(t, root, "update-ref", AcceptedRef, malformed)
		request := priorityVerbReq(root, "01J5X000000000000000000R01", "mac-a")
		result, err := SetPriority(request, "a", 1, sequencePointer(1), testHumanAuthority(t, root, request.Now))
		if err == nil || !strings.Contains(err.Error(), "captured tip does not validate") || !strings.Contains(err.Error(), "priority 1") || !strings.Contains(err.Error(), "goal repair --accept-remote") {
			t.Fatalf("set-priority did not refuse a malformed accepted order: %+v %v", result, err)
		}
		if after := acceptedTip(t, root); after != malformed {
			t.Fatalf("malformed-tree refusal changed the accepted tip: before=%s after=%s", malformed, after)
		}
	})

	t.Run("manual-edit", func(t *testing.T) {
		_, root := oneClone(t)
		seedLedger(t, root)
		file := vGoal("ranked-edit", StateQueued)
		file.Priority, file.Sequence = 1, 1
		publishGoalFixtures(t, root, file)
		materialize(t, root, acceptedTip(t, root))
		editFile(t, root, livePath(file.Id), func(edited *GoalFile) {
			edited.Sequence = 2
			edited.NextStep = "A lawful edit combined with an unlawful rank edit."
		})

		_, err := Reconcile(humanReconcileReq(root, "01J5X000000000000000000R00"))
		if err == nil || !strings.Contains(err.Error(), "Sequence") || !strings.Contains(err.Error(), "set-priority") {
			t.Fatalf("reconcile did not refuse a direct rank edit with its remedy: %v", err)
		}
	})
}

func insertRankFields(file *GoalFile, fields string) []byte {
	file.Priority, file.Sequence = 0, 0
	body, _, _ := splitIntegrity(RenderFile(file))
	rewritten := strings.Replace(string(body), "- State: "+file.State+"\n", "- State: "+file.State+"\n"+fields, 1)
	return []byte(rewritten + "Integrity: sha256=" + IntegrityDigest([]byte(rewritten)) + "\n")
}

func TestPriorityReordersAndResequences(t *testing.T) {
	t.Run("insert", func(t *testing.T) {
		root := rankedGoalBed(t, map[string][2]uint64{
			"a": {1, 1}, "b": {1, 2}, "c": {1, 3},
		})
		request := priorityVerbReq(root, "01J5X000000000000000000R10", "mac-a")
		result, err := SetPriority(request, "c", 1, sequencePointer(2), testHumanAuthority(t, root, request.Now))
		if err != nil || result.Outcome != OutcomeConfirmed {
			t.Fatalf("insert c at 1:2: %+v %v", result, err)
		}
		tree, err := loadTree(root, result.Tip)
		if err != nil {
			t.Fatal(err)
		}
		assertPriorityOrder(t, tree, []string{"a", "c", "b"})
		if tree.Live["a"].Revision != 1 || tree.Live["b"].Revision != 2 || tree.Live["c"].Revision != 2 {
			t.Fatalf("only displaced records gained one revision: a=%d b=%d c=%d", tree.Live["a"].Revision, tree.Live["b"].Revision, tree.Live["c"].Revision)
		}
		for _, id := range []string{"b", "c"} {
			last := tree.Live[id].History[len(tree.Live[id].History)-1]
			if last.Opid != request.opid() || last.Verb != "set-priority" || last.Actor != "human:wido" || !reflect.DeepEqual(last.Targets, []string{"b", "c"}) ||
				!strings.Contains(last.Reason, "subject=c") || !strings.Contains(last.Reason, "requested-sequence=2") {
				t.Fatalf("%s lacks the shared human order event: %+v", id, last)
			}
		}
	})

	t.Run("move-priority", func(t *testing.T) {
		root := rankedGoalBed(t, map[string][2]uint64{
			"a": {1, 1}, "b": {1, 2}, "c": {1, 3}, "d": {2, 1},
		})
		request := priorityVerbReq(root, "01J5X000000000000000000R20", "mac-a")
		result, err := SetPriority(request, "b", 2, nil, testHumanAuthority(t, root, request.Now))
		if err != nil || result.Outcome != OutcomeConfirmed {
			t.Fatalf("move b to priority 2 tail: %+v %v", result, err)
		}
		tree, _ := loadTree(root, result.Tip)
		assertPriorityOrder(t, tree, []string{"a", "c", "d", "b"})
		assertPair(t, tree.Live["c"], 1, 2)
		assertPair(t, tree.Live["b"], 2, 2)
		if reason := tree.Live["b"].History[len(tree.Live["b"].History)-1].Reason; !strings.Contains(reason, "requested-sequence=append") {
			t.Fatalf("omitted cross-priority sequence was not recorded as append: %q", reason)
		}
	})

	t.Run("append", func(t *testing.T) {
		root := rankedGoalBed(t, map[string][2]uint64{
			"a": {1, 1}, "b": {1, 2}, "c": {1, 3},
		})
		request := priorityVerbReq(root, "01J5X000000000000000000R30", "mac-a")
		result, err := SetPriority(request, "a", 1, sequencePointer(3), testHumanAuthority(t, root, request.Now))
		if err != nil || result.Outcome != OutcomeConfirmed {
			t.Fatalf("append a: %+v %v", result, err)
		}
		tree, _ := loadTree(root, result.Tip)
		assertPriorityOrder(t, tree, []string{"b", "c", "a"})
	})

	t.Run("same-priority-noop", func(t *testing.T) {
		root := rankedGoalBed(t, map[string][2]uint64{
			"a": {1, 1}, "b": {1, 2}, "c": {1, 3},
		})
		before := acceptedTip(t, root)
		request := priorityVerbReq(root, "01J5X000000000000000000R40", "mac-a")
		result, err := SetPriority(request, "b", 1, nil, testHumanAuthority(t, root, request.Now))
		if err != nil || result.Outcome != OutcomeAbandoned || result.Tip != before {
			t.Fatalf("same-priority omitted sequence must be a no-write no-op: %+v %v", result, err)
		}
		if after := acceptedTip(t, root); after != before {
			t.Fatalf("no-op changed the accepted tip: before=%s after=%s", before, after)
		}
	})

	t.Run("claimed-peer", func(t *testing.T) {
		root := rankedGoalBed(t, map[string][2]uint64{
			"a": {1, 1}, "b": {1, 2}, "c": {1, 3},
		})
		claimRequest := verbReq(root, "01J5X000000000000000000R50", "mac-b")
		if result, err := claimApprovedForTest(t, claimRequest, "b", testBudget()); err != nil || result.Outcome != OutcomeConfirmed {
			t.Fatalf("claim ranked peer: %+v %v", result, err)
		}
		beforeTree, _ := loadTree(root, acceptedTip(t, root))
		beforeClaim := *beforeTree.Live["b"].Claimed
		beforeApproval := *beforeTree.Live["b"].Approved
		beforeBudget := *beforeTree.Live["b"].Budget
		beforeCapability := *beforeTree.Live["b"].StopCapability

		request := priorityVerbReq(root, "01J5X000000000000000000R60", "mac-a")
		result, err := SetPriority(request, "c", 1, sequencePointer(2), testHumanAuthority(t, root, request.Now))
		if err != nil || result.Outcome != OutcomeConfirmed {
			t.Fatalf("move around claimed peer: %+v %v", result, err)
		}
		tree, _ := loadTree(root, result.Tip)
		peer := tree.Live["b"]
		if !reflect.DeepEqual(*peer.Claimed, beforeClaim) || !reflect.DeepEqual(*peer.Approved, beforeApproval) ||
			!reflect.DeepEqual(*peer.Budget, beforeBudget) || !reflect.DeepEqual(*peer.StopCapability, beforeCapability) {
			t.Fatalf("rank-only touch changed claim, approval, budget, or stop authority: before=%+v after=%+v", beforeTree.Live["b"], peer)
		}
		assertPair(t, peer, 1, 3)
	})
}

func TestPriorityLifecycle(t *testing.T) {
	t.Run("done-reopen", func(t *testing.T) {
		root := rankedGoalBed(t, map[string][2]uint64{
			"a": {1, 1}, "b": {1, 2}, "c": {1, 3},
		})
		doneRequest := verbReq(root, "01J5X000000000000000000S10", "mac-a")
		done, err := Done(doneRequest, "b", "Finished the middle goal.")
		if err != nil || done.Outcome != OutcomeConfirmed {
			t.Fatalf("done ranked middle: %+v %v", done, err)
		}
		tree, _ := loadTree(root, done.Tip)
		assertPriorityOrder(t, tree, []string{"a", "c"})
		assertPair(t, tree.Live["c"], 1, 2)
		assertPair(t, tree.Done["b"], 1, 2)
		if tree.Live["a"].Revision != 1 || tree.Live["c"].Revision != 2 {
			t.Fatalf("done compaction touched the wrong survivors: a=%d c=%d", tree.Live["a"].Revision, tree.Live["c"].Revision)
		}
		compacted := tree.Live["c"].History[len(tree.Live["c"].History)-1]
		if compacted.Verb != "done" || !reflect.DeepEqual(compacted.Targets, []string{"b", "c"}) || !strings.Contains(compacted.Reason, "from=1:3 to=1:2") {
			t.Fatalf("survivor lacks the done compaction event: %+v", compacted)
		}

		reopenRequest := verbReq(root, "01J5X000000000000000000S20", "mac-a")
		reopened, err := Reopen(reopenRequest, "b")
		if err != nil || reopened.Outcome != OutcomeConfirmed {
			t.Fatalf("reopen ranked goal: %+v %v", reopened, err)
		}
		tree, _ = loadTree(root, reopened.Tip)
		assertPriorityOrder(t, tree, []string{"a", "c", "b"})
		assertPair(t, tree.Live["b"], 1, 3)
		last := tree.Live["b"].History[len(tree.Live["b"].History)-1]
		if last.Verb != "reopen" || !strings.Contains(last.Reason, "from=1:2 to=1:3") {
			t.Fatalf("reopen did not record its append-at-tail rank: %+v", last)
		}
	})

	t.Run("split", func(t *testing.T) {
		_, root := oneClone(t)
		seedLedger(t, root)
		files := []*GoalFile{
			rankedGoal("a", 1, 1),
			rankedGoal("parent", 1, 2),
			rankedGoal("c", 1, 3),
			rankedGoal("dependent", 1, 4),
		}
		files[3].Blocked = []string{"parent"}
		publishGoalFixtures(t, root, files...)
		members := testMembers("parent")
		request := verbReq(root, "01J5X000000000000000000S30", "mac-a")
		result, err := Split(request, "parent", members, mainRatification("parent", members), nil)
		if err != nil || result.Outcome != OutcomeConfirmed {
			t.Fatalf("split ranked parent: %+v %v", result, err)
		}
		tree, _ := loadTree(root, result.Tip)
		assertPriorityOrder(t, tree, []string{"a", "c", "dependent", "parent-one", "parent-two"})
		assertPair(t, tree.Done["parent"], 1, 2)
		assertPair(t, tree.Live["c"], 1, 2)
		assertPair(t, tree.Live["dependent"], 1, 3)
		for _, id := range []string{"parent-one", "parent-two"} {
			if member := tree.Live[id]; member == nil || member.Priority != 0 || member.Sequence != 0 {
				t.Fatalf("split member %s inherited a rank: %+v", id, member)
			}
		}
		dependent := tree.Live["dependent"]
		if dependent.Revision != 2 || len(dependent.History) != 2 {
			t.Fatalf("dependent blocker and rank changes were not one touch: %+v", dependent)
		}
		last := dependent.History[len(dependent.History)-1]
		if last.Verb != "split" || !strings.Contains(last.Reason, "from=1:4 to=1:3") {
			t.Fatalf("dependent's split event lacks its merged compaction: %+v", last)
		}
	})
}

func TestPriorityRace(t *testing.T) {
	t.Run("same-position", func(t *testing.T) {
		a, b := priorityRaceBed(t, []*GoalFile{
			rankedGoal("a", 1, 1), rankedGoal("b", 1, 2),
			vGoal("d", StateQueued), vGoal("e", StateQueued),
		})
		requestA := priorityVerbReq(a, "01J5X000000000000000000V10", "mac-a")
		requestB := priorityVerbReq(b, "01J5X000000000000000000V20", "mac-b")
		held := setPriorityRequest(requestA, "d", 1, sequencePointer(1))
		attempts := 0
		injected := false
		held.BeforePush = func(attempt int) error {
			attempts = attempt
			if injected {
				return nil
			}
			injected = true
			result, err := Publish(endpointFor(b), setPriorityRequest(requestB, "e", 1, sequencePointer(1)))
			if err != nil || result.Outcome != OutcomeConfirmed {
				return fmt.Errorf("competing insertion did not publish: %+v %v", result, err)
			}
			return nil
		}
		result, err := Publish(endpointFor(a), held)
		if err != nil || result.Outcome != OutcomeConfirmed || attempts < 2 {
			t.Fatalf("held insertion did not retry and confirm: attempts=%d result=%+v err=%v", attempts, result, err)
		}
		tree, _ := loadTree(a, result.Tip)
		assertPriorityOrder(t, tree, []string{"d", "e", "a", "b"})
		assertHistoryContainsOpid(t, tree.Live["d"], requestA.opid())
		assertHistoryContainsOpid(t, tree.Live["e"], requestB.opid())
	})

	t.Run("same-target", func(t *testing.T) {
		a, b := priorityRaceBed(t, []*GoalFile{
			rankedGoal("a", 1, 1), rankedGoal("b", 1, 2), vGoal("target", StateQueued),
		})
		requestA := priorityVerbReq(a, "01J5X000000000000000000V30", "mac-a")
		requestB := priorityVerbReq(b, "01J5X000000000000000000V40", "mac-b")
		held := setPriorityRequest(requestA, "target", 1, sequencePointer(2))
		injected := false
		held.BeforePush = func(int) error {
			if injected {
				return nil
			}
			injected = true
			result, err := Publish(endpointFor(b), setPriorityRequest(requestB, "target", 1, sequencePointer(1)))
			if err != nil || result.Outcome != OutcomeConfirmed {
				return fmt.Errorf("first same-target edit did not publish: %+v %v", result, err)
			}
			return nil
		}
		result, err := Publish(endpointFor(a), held)
		if err != nil || result.Outcome != OutcomeConfirmed {
			t.Fatalf("later same-target edit did not publish: %+v %v", result, err)
		}
		tree, _ := loadTree(a, result.Tip)
		assertPriorityOrder(t, tree, []string{"a", "target", "b"})
		assertHistoryContainsOpid(t, tree.Live["target"], requestA.opid())
		assertHistoryContainsOpid(t, tree.Live["target"], requestB.opid())
	})

	t.Run("range-changed", func(t *testing.T) {
		a, b := priorityRaceBed(t, []*GoalFile{
			rankedGoal("a", 1, 1), rankedGoal("b", 1, 2), vGoal("target", StateQueued),
		})
		requestA := priorityVerbReq(a, "01J5X000000000000000000V50", "mac-a")
		held := setPriorityRequest(requestA, "target", 1, sequencePointer(3))
		injected := false
		held.BeforePush = func(int) error {
			if injected {
				return nil
			}
			injected = true
			result, err := Done(verbReq(b, "01J5X000000000000000000V60", "mac-b"), "b", "Removed before the held insertion.")
			if err != nil || result.Outcome != OutcomeConfirmed {
				return fmt.Errorf("competing removal did not publish: %+v %v", result, err)
			}
			return nil
		}
		result, err := Publish(endpointFor(a), held)
		if err != nil || result.Outcome != OutcomeRejected || !strings.Contains(result.Detail, "range 1..2") {
			t.Fatalf("retry did not refuse its newly impossible range: %+v %v", result, err)
		}
		tree, _ := loadTree(a, result.Tip)
		assertPriorityOrder(t, tree, []string{"a", "target"})
		if tree.Live["target"].Priority != 0 || tree.Live["target"].Sequence != 0 {
			t.Fatalf("refused insertion ranked its target: %+v", tree.Live["target"])
		}
	})

	t.Run("claim-first", func(t *testing.T) {
		target := approvedGoalFixture(rankedGoal("target", 1, 2), testBudget())
		a, b := priorityRaceBed(t, []*GoalFile{rankedGoal("peer", 1, 1), target})
		rankRequest := priorityVerbReq(a, "01J5X000000000000000000V70", "mac-a")
		claimVerb := verbReq(b, "01J5X000000000000000000V80", "mac-b")
		held := setPriorityRequest(rankRequest, "target", 1, sequencePointer(1))
		attempts := 0
		injected := false
		var publishedClaim ClaimRecord
		held.BeforePush = func(attempt int) error {
			attempts = attempt
			if injected {
				return nil
			}
			injected = true
			result, err := Publish(endpointFor(b), claimRequest(claimVerb, "target", nil))
			if err != nil || result.Outcome != OutcomeConfirmed {
				return fmt.Errorf("competing claim did not publish: %+v %v", result, err)
			}
			tree, treeErr := loadTree(b, result.Tip)
			if treeErr != nil {
				return treeErr
			}
			publishedClaim = *tree.Live["target"].Claimed
			return nil
		}
		result, err := Publish(endpointFor(a), held)
		if err != nil || result.Outcome != OutcomeConfirmed || attempts < 2 {
			t.Fatalf("rank after claim did not rebuild and confirm: attempts=%d result=%+v err=%v", attempts, result, err)
		}
		tree, _ := loadTree(a, result.Tip)
		assertPriorityOrder(t, tree, []string{"target", "peer"})
		if tree.Live["target"].Claimed == nil || !reflect.DeepEqual(*tree.Live["target"].Claimed, publishedClaim) || tree.Live["target"].Approved == nil {
			t.Fatalf("rank retry overwrote the published claim or approval: %+v", tree.Live["target"])
		}
		assertHistoryContainsOpid(t, tree.Live["target"], rankRequest.opid())
		assertHistoryContainsOpid(t, tree.Live["target"], claimVerb.opid())
	})

	t.Run("priority-first", func(t *testing.T) {
		target := approvedGoalFixture(rankedGoal("target", 1, 2), testBudget())
		a, b := priorityRaceBed(t, []*GoalFile{rankedGoal("peer", 1, 1), target})
		rankRequest := priorityVerbReq(a, "01J5X000000000000000000V90", "mac-a")
		claimVerb := verbReq(b, "01J5X000000000000000000VA0", "mac-b")
		held := claimRequest(claimVerb, "target", nil)
		attempts := 0
		injected := false
		held.BeforePush = func(attempt int) error {
			attempts = attempt
			if injected {
				return nil
			}
			injected = true
			result, err := Publish(endpointFor(a), setPriorityRequest(rankRequest, "target", 1, sequencePointer(1)))
			if err != nil || result.Outcome != OutcomeConfirmed {
				return fmt.Errorf("competing priority edit did not publish: %+v %v", result, err)
			}
			return nil
		}
		result, err := Publish(endpointFor(b), held)
		if err != nil || result.Outcome != OutcomeConfirmed || attempts < 2 {
			t.Fatalf("claim after priority did not rebuild and confirm: attempts=%d result=%+v err=%v", attempts, result, err)
		}
		tree, _ := loadTree(b, result.Tip)
		assertPriorityOrder(t, tree, []string{"target", "peer"})
		claimed := tree.Live["target"]
		if claimed.Claimed == nil || claimed.Claimed.Machine != "mac-b" || claimed.Approved == nil {
			t.Fatalf("claim retry overwrote rank or approval: %+v", claimed)
		}
		assertHistoryContainsOpid(t, claimed, rankRequest.opid())
		assertHistoryContainsOpid(t, claimed, claimVerb.opid())
	})
}

func TestPriorityRecovery(t *testing.T) {
	t.Run("unlanded", func(t *testing.T) {
		root := rankedGoalBed(t, map[string][2]uint64{"target": {1, 1}})
		opid := Opid("01J5X000000000000000000W10", "mac-a", "lin-1")
		intent := Intent{Verb: "set-priority", Targets: []string{"target"}, Args: map[string]string{
			"by": "wido", "priority": "2", "sequence": "1",
		}}
		strandEntry(t, root, opid, PhaseCreated, intent)
		entry, err := ReadEntry(root, opid)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := requestForEntry(endpointFor(root), entry); err == nil || !strings.Contains(err.Error(), "enrolled terminal") {
			t.Fatalf("stored human name reconstructed rank authority: %v", err)
		}
		reports, err := Recover(endpointFor(root))
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, report := range reports {
			if report.Opid == opid {
				found = report.Action == ActionComplete && strings.Contains(report.Detail, "enrolled terminal")
			}
		}
		if !found {
			t.Fatalf("recovery did not close toward a fresh terminal act: %+v", reports)
		}
		tree, _ := loadTree(root, acceptedTip(t, root))
		assertPair(t, tree.Live["target"], 1, 1)
		entry, _ = ReadEntry(root, opid)
		if entry.Outcome != OutcomeRejected {
			t.Fatalf("unlanded sensitive act did not close rejected: %+v", entry)
		}
	})

	t.Run("landed", func(t *testing.T) {
		a, b := priorityRaceBed(t, []*GoalFile{rankedGoal("a", 1, 1), vGoal("target", StateQueued)})
		request := priorityVerbReq(b, "01J5X000000000000000000W20", "mac-b")
		publish := setPriorityRequest(request, "target", 1, sequencePointer(2))
		result, err := Publish(endpointFor(b), publish)
		if err != nil || result.Outcome != OutcomeConfirmed {
			t.Fatalf("publish rank before recovery loses its journal: %+v %v", result, err)
		}
		strandEntryAt(t, a, request.opid(), "mac-b", PhasePushed, publish.Intent)
		reports, err := Recover(endpointFor(a))
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, report := range reports {
			if report.Opid == request.opid() {
				found = report.Action == ActionConfirm && strings.Contains(report.Detail, "canonical tip")
			}
		}
		if !found {
			t.Fatalf("recovery did not confirm the visible operation: %+v", reports)
		}
		tree, _ := loadTree(a, result.Tip)
		assertPair(t, tree.Live["target"], 1, 2)
		count := 0
		for _, event := range tree.Live["target"].History {
			if event.Opid == request.opid() {
				count++
			}
		}
		if count != 1 {
			t.Fatalf("landed recovery duplicated the rank event: count=%d history=%+v", count, tree.Live["target"].History)
		}
	})
}

func rankedGoalBed(t *testing.T, ranks map[string][2]uint64) string {
	t.Helper()
	_, root := oneClone(t)
	seedLedger(t, root)
	ids := make([]string, 0, len(ranks))
	for id := range ranks {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	files := make([]*GoalFile, 0, len(ids))
	for _, id := range ids {
		files = append(files, rankedGoal(id, uint8(ranks[id][0]), ranks[id][1]))
	}
	publishGoalFixtures(t, root, files...)
	return root
}

func rankedGoal(id string, priority uint8, sequence uint64) *GoalFile {
	file := vGoal(id, StateQueued)
	file.Priority = priority
	file.Sequence = sequence
	return file
}

func priorityVerbReq(root, ulid, machine string) VerbRequest {
	request := verbReq(root, ulid, machine)
	request.Actor.Human = "wido"
	return request
}

func sequencePointer(sequence uint64) *uint64 { return &sequence }

func assertPriorityOrder(t *testing.T, tree *TreeGoals, want []string) {
	t.Helper()
	if got := OrderedOpenGoalIDs(tree.Live); !reflect.DeepEqual(got, want) {
		t.Fatalf("priority order = %v, want %v", got, want)
	}
	if problems := ValidateTree(tree); len(problems) != 0 {
		t.Fatalf("ordered tree is invalid: %v", problems)
	}
}

func assertPair(t *testing.T, file *GoalFile, priority uint8, sequence uint64) {
	t.Helper()
	if file == nil || file.Priority != priority || file.Sequence != sequence {
		t.Fatalf("goal pair = %+v, want %d:%d", file, priority, sequence)
	}
}

func priorityRaceBed(t *testing.T, files []*GoalFile) (string, string) {
	t.Helper()
	_, a, b := twoClones(t)
	seedLedger(t, a)
	publishGoalFixtures(t, a, files...)
	return a, b
}

func assertHistoryContainsOpid(t *testing.T, file *GoalFile, opid string) {
	t.Helper()
	for _, event := range file.History {
		if event.Opid == opid {
			return
		}
	}
	t.Fatalf("goal %s history does not contain operation %s: %+v", file.Id, opid, file.History)
}
