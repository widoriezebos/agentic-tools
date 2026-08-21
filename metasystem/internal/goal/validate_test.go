package goal

import (
	"strings"
	"testing"
)

// vRoot is a minimal valid root record for validator fixtures.
func vRoot() *RootRecord {
	return &RootRecord{
		Identity: "01J5X000000000000000000000", FormatVersion: "1",
		SyncMode: SyncRemote, MigrationEpoch: "2026-08-20T00:00:00Z",
		ManifestDigest: strings.Repeat("ab", 32), MigrationMode: "manifest",
		Revision: 1,
		History: []HistoryLine{{
			At: "2026-08-20T09:00:00Z", Opid: "01J5X0000000000000000000A0-mac-a-1a2b3c4d",
			Verb: "migrate", Actor: "mac-a+lin-1", Keep: -1,
		}},
	}
}

// vGoal is a minimal valid goal file for validator fixtures.
func vGoal(id, state string) *GoalFile {
	f := &GoalFile{
		Id: id, State: state, Intent: "Do the thing called " + id,
		Origin: "main", OpenedAt: "2026-08-20T10:00:00Z", Revision: 1,
		History: []HistoryLine{{
			At: "2026-08-20T10:00:00Z", Opid: "01J5X0000000000000000000B0-mac-a-1a2b3c4d",
			Verb: "open", Actor: "mac-a+lin-1", Targets: []string{id}, Keep: -1,
		}},
	}
	if state == StateClaimed {
		f.Claimed = &ClaimRecord{Machine: "mac-a", Lineage: "lin-1", At: "2026-08-20T10:05:00Z"}
	}
	if state == StateParked {
		f.Parked = &ParkRecord{By: "human:wido", At: "2026-08-20T10:05:00Z", Because: "paused"}
	}
	if state == StateDone {
		f.Conclude = "Shipped and verified."
	}
	return f
}

// vTree renders a whole fixture tree: live goals, archived goals,
// and the root record.
func vTree(root *RootRecord, live []*GoalFile, done []*GoalFile) map[string][]byte {
	files := map[string][]byte{}
	if root != nil {
		files[goalsPrefix+"backlog.md"] = RenderRoot(root)
	}
	for _, f := range live {
		files[goalsPrefix+f.Id+".md"] = RenderFile(f)
	}
	for _, f := range done {
		files[goalsPrefix+"done/"+f.Id+".md"] = RenderFile(f)
	}
	return files
}

func problemsOf(files map[string][]byte) []Problem {
	t, problems := ParseTreeFiles(files)
	if len(problems) > 0 {
		return problems
	}
	return ValidateTree(t)
}

func expectProblem(t *testing.T, problems []Problem, fragment string) {
	t.Helper()
	for _, p := range problems {
		if strings.Contains(string(p), fragment) {
			return
		}
	}
	t.Fatalf("no problem names %q; got %v", fragment, problems)
}

func TestAValidTreeValidates(t *testing.T) {
	blocker := vGoal("base", StateDone)
	claimed := vGoal("current-work", StateClaimed)
	claimed.Blocked = []string{"base"}
	files := vTree(vRoot(),
		[]*GoalFile{claimed, vGoal("later", StateQueued), vGoal("paused", StateParked)},
		[]*GoalFile{blocker})
	if problems := problemsOf(files); len(problems) != 0 {
		t.Fatalf("a lawful tree validates whole: %v", problems)
	}
}

func TestIntegrityTamperRefusesByName(t *testing.T) {
	files := vTree(vRoot(), []*GoalFile{vGoal("g", StateQueued)}, nil)
	key := goalsPrefix + "g.md"
	files[key] = []byte(strings.Replace(string(files[key]), "Do the thing", "Do another thing", 1))
	expectProblem(t, problemsOf(files), "Integrity mismatch")
}

func TestPlacementAndStateMustAgree(t *testing.T) {
	stray := vGoal("stray", StateDone)
	files := vTree(vRoot(), []*GoalFile{stray}, nil)
	expectProblem(t, problemsOf(files), "State done outside the archive")

	misfiled := vGoal("misfiled", StateQueued)
	files = vTree(vRoot(), nil, []*GoalFile{misfiled})
	expectProblem(t, problemsOf(files), "inside the archive")
}

func TestMissingRootRecordRefuses(t *testing.T) {
	files := vTree(nil, []*GoalFile{vGoal("g", StateQueued)}, nil)
	expectProblem(t, problemsOf(files), "root record is missing")
}

func TestDanglingBlockerRefuses(t *testing.T) {
	g := vGoal("g", StateQueued)
	g.Blocked = []string{"ghost"}
	files := vTree(vRoot(), []*GoalFile{g}, nil)
	expectProblem(t, problemsOf(files), "does not exist: ghost")
}

func TestComposedCycleRefuses(t *testing.T) {
	a := vGoal("a", StateQueued)
	a.Blocked = []string{"b"}
	b := vGoal("b", StateQueued)
	b.Blocked = []string{"c"}
	c := vGoal("c", StateQueued)
	c.Blocked = []string{"a"}
	files := vTree(vRoot(), []*GoalFile{a, b, c}, nil)
	expectProblem(t, problemsOf(files), "blockedBy cycle")
}

func TestClaimedImpliesBlockersDone(t *testing.T) {
	open := vGoal("open-dep", StateQueued)
	claimed := vGoal("eager", StateClaimed)
	claimed.Blocked = []string{"open-dep"}
	files := vTree(vRoot(), []*GoalFile{open, claimed}, nil)
	expectProblem(t, problemsOf(files), "claimed while blocker open-dep is not done")
}

func TestBlockedDoneRefuses(t *testing.T) {
	open := vGoal("still-open", StateQueued)
	concluded := vGoal("hasty", StateDone)
	concluded.Blocked = []string{"still-open"}
	files := vTree(vRoot(), []*GoalFile{open}, []*GoalFile{concluded})
	expectProblem(t, problemsOf(files), "done while blocker still-open is not done")
}

func TestQuotaIsOneClaimPerMachine(t *testing.T) {
	first := vGoal("first", StateClaimed)
	second := vGoal("second", StateClaimed)
	files := vTree(vRoot(), []*GoalFile{first, second}, nil)
	expectProblem(t, problemsOf(files), "quota is one claim per machine")

	// One ARC under one claimant counts once (R4-08).
	first.Arc = "the-arc"
	second.Arc = "the-arc"
	files = vTree(vRoot(), []*GoalFile{first, second}, nil)
	if problems := problemsOf(files); len(problems) != 0 {
		t.Fatalf("one arc's members under one claimant count once: %v", problems)
	}

	// Two machines, one goal each: lawful.
	second.Arc = ""
	first.Arc = ""
	second.Claimed.Machine = "mac-b"
	files = vTree(vRoot(), []*GoalFile{first, second}, nil)
	if problems := problemsOf(files); len(problems) != 0 {
		t.Fatalf("different machines claim independently: %v", problems)
	}
}

func TestGoalFreeExcludesQueuedAndClaimed(t *testing.T) {
	root := vRoot()
	root.Free = &FreeRecord{Declared: "2026-08-20T11:00:00Z", Origin: "main", Digest: strings.Repeat("cd", 32)}
	files := vTree(root, []*GoalFile{vGoal("g", StateQueued)}, nil)
	expectProblem(t, problemsOf(files), "Goal-free declared while g is queued")

	// Parked and done coexist with the declaration.
	files = vTree(root, []*GoalFile{vGoal("p", StateParked)}, []*GoalFile{vGoal("d", StateDone)})
	if problems := problemsOf(files); len(problems) != 0 {
		t.Fatalf("Goal-free coexists with parked and done: %v", problems)
	}
}

func TestGoalInBothLiveAndArchiveRefuses(t *testing.T) {
	twin := vGoal("twin", StateQueued)
	twinDone := vGoal("twin", StateDone)
	files := vTree(vRoot(), []*GoalFile{twin}, []*GoalFile{twinDone})
	expectProblem(t, problemsOf(files), "also present in the archive")
}

func TestFileNameAndIdMustAgree(t *testing.T) {
	g := vGoal("actual", StateQueued)
	files := vTree(vRoot(), nil, nil)
	files[goalsPrefix+"pretend.md"] = RenderFile(g)
	expectProblem(t, problemsOf(files), "file name and Id disagree")
}

func TestValidateCommitReadsARealTree(t *testing.T) {
	_, a, _ := twoClones(t)
	e := endpointFor(a)

	files := vTree(vRoot(), []*GoalFile{vGoal("real", StateQueued)}, nil)
	var changes []Change
	for p, content := range files {
		changes = append(changes, Change{Path: p, Content: content})
	}
	res, err := Publish(e, PublishRequest{
		Opid: "op-validate", Machine: "mac-a", Lineage: "l1",
		Intent: testIntentFor("open"), Message: "goal open real",
		Mutate: func(tip string) ([]Change, error) { return changes, nil },
		Validate: func(commit string) error {
			return ValidateCommit(a, commit)
		},
	})
	if err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("a valid tree publishes through the real validator: %+v %v", res, err)
	}

	// A torn tree is refused BY the same seam, named.
	tampered := vGoal("torn", StateQueued)
	raw := RenderFile(tampered)
	raw = []byte(strings.Replace(string(raw), "Do the thing", "tampered", 1))
	res, err = Publish(e, PublishRequest{
		Opid: "op-torn", Machine: "mac-a", Lineage: "l1",
		Intent: testIntentFor("open"), Message: "goal open torn",
		Mutate: func(tip string) ([]Change, error) {
			return []Change{{Path: goalsPrefix + "torn.md", Content: raw}}, nil
		},
		Validate: func(commit string) error {
			return ValidateCommit(a, commit)
		},
	})
	if err != nil || res.Outcome != OutcomeRejected {
		t.Fatalf("a torn tree is a definite rejection: %+v %v", res, err)
	}
	if !strings.Contains(res.Detail, "Integrity mismatch") {
		t.Fatalf("the refusal names the file and rule: %s", res.Detail)
	}
}
