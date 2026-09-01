package goal

import (
	"strings"
	"testing"
	"time"
)

func TestProjectionReadsTheAcceptedTreeOnly(t *testing.T) {
	_, a, b := twoClones(t)
	seedLedger(t, a)
	if res, err := Open(verbReq(a, "01J5X00000000000000000E000", "mac-a"), "seen", "Visible.", "main", "Go."); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("open: %+v %v", res, err)
	}
	// B publishes past A's accepted tree; A's OFFLINE projection
	// still shows only what A accepted — mid-edit remote state is
	// invisible until a fetch.
	if res, err := Open(verbReq(b, "01J5X00000000000000000E010", "mac-b"), "unseen", "Not yet.", "main", "Go."); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("B opens: %+v %v", res, err)
	}
	now := time.Now()
	p, err := Project(endpointFor(a), false, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, visible := p.Tree.Live["unseen"]; visible {
		t.Fatal("the offline projection never shows an unaccepted tip")
	}
	if _, visible := p.Tree.Live["seen"]; !visible {
		t.Fatal("the accepted world is fully visible")
	}
	// With --fetch the read-side validator advances first.
	p2, err := Project(endpointFor(a), true, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, visible := p2.Tree.Live["unseen"]; !visible {
		t.Fatal("the fetching projection advances onto the validated tip")
	}
	// The frontier read: both queued goals are ready.
	v := Next(p2, "mac-a")
	if len(v.Ready) != 2 || len(v.Claimed) != 0 {
		t.Fatalf("the frontier names the ready set: %+v", v)
	}
}

func TestNextFiltersCandidatesButNeverTheHeldClaim(t *testing.T) {
	held := vGoal("held", StateClaimed)
	held.Claimed = &ClaimRecord{Machine: "mac-a", Lineage: "lin-1", At: "2026-08-20T10:05:00Z"}
	held.Labels = []string{"other"}
	one := vGoal("one", StateQueued)
	one.Labels = []string{"alpha", "shared"}
	two := vGoal("two", StateQueued)
	two.Labels = []string{"beta", "shared"}
	p := Projection{Tree: &TreeGoals{Live: map[string]*GoalFile{
		"held": held, "one": one, "two": two,
	}, Done: map[string]*GoalFile{}}}

	v := Next(p, "mac-a", "alpha")
	if len(v.Claimed) != 1 || v.Claimed[0] != "held" || len(v.Ready) != 1 || v.Ready[0] != "one" {
		t.Fatalf("the held claim remains first while the candidate set narrows: %+v", v)
	}
	v = Next(p, "other-machine", "shared", "beta")
	if len(v.Claimed) != 0 || len(v.Ready) != 1 || v.Ready[0] != "two" {
		t.Fatalf("repeated labels combine with AND: %+v", v)
	}
	v = Next(p, "other-machine", "absent")
	if len(v.Claimed) != 0 || len(v.Ready) != 0 || len(v.Blocked) != 0 {
		t.Fatalf("an empty filtered candidate set is distinguishable: %+v", v)
	}
}

func TestNextTreatsArcMemberPinsIndependently(t *testing.T) {
	foreign := vGoal("foreign-pinned", StateQueued)
	foreign.Arc = "shared-arc"
	foreign.Pinned = "mac-b"
	local := vGoal("local-member", StateQueued)
	local.Arc = "shared-arc"
	p := Projection{Tree: &TreeGoals{Live: map[string]*GoalFile{
		foreign.Id: foreign, local.Id: local,
	}, Done: map[string]*GoalFile{}}}
	verdict := Next(p, "mac-a")
	if strings.Join(verdict.Ready, ",") != "local-member" {
		t.Fatalf("a sibling's foreign pin hid an independently claimable member: %+v", verdict)
	}
}

func TestProjectionBannersStalenessAndLocalMode(t *testing.T) {
	repo := soloLedgerRepo(t)
	e := Endpoint{Root: repo, Remote: "local", Branch: "refs/heads/main"}
	// Read far in the future: the staleness banner names the age;
	// the local-mode banner names the promotion goal.
	later := time.Now().Add(2 * time.Hour)
	p, err := Project(e, false, later)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(p.Banners, "; ")
	if !strings.Contains(joined, "old") {
		t.Fatalf("staleness banners past the threshold: %v", p.Banners)
	}
	if !strings.Contains(joined, "backlog-local-promotion") {
		t.Fatalf("local mode banners the promotion goal: %v", p.Banners)
	}
	// The frontier reads the solo goal ready.
	v := Next(p, "mac-solo")
	if len(v.Ready) != 1 || v.Ready[0] != "solo-goal" {
		t.Fatalf("the solo frontier: %+v", v)
	}
}

func TestSyncModeMismatchRefusesByName(t *testing.T) {
	repo := soloLedgerRepo(t)
	// The ledger says local; the config says remote: the forbidden
	// promotion refuses naming the goal.
	e := Endpoint{Root: repo, Remote: "origin", Branch: "refs/heads/main"}
	_, err := Project(e, false, time.Now())
	if err == nil || !strings.Contains(err.Error(), "backlog-local-promotion") {
		t.Fatalf("the mode flip refuses toward the promotion goal: %v", err)
	}
}

// soloLedgerRepo builds a single-machine repo whose ledger branch
// carries a LOCAL-mode root record and one queued goal, with the
// accepted ref set — the world after a local-mode migration.
func soloLedgerRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	mustGit(t, t.TempDir(), "init", "-q", "-b", "main", repo)
	mustGit(t, repo, "commit", "-q", "--allow-empty", "-m", "seed")
	mustGit(t, repo, "update-ref", LocalLedgerBranch, "HEAD")
	root := vRoot()
	root.SyncMode = SyncLocal
	files := vTree(root, []*GoalFile{vGoal("solo-goal", StateQueued)}, nil)
	var changes []Change
	for p, content := range files {
		changes = append(changes, Change{Path: p, Content: content})
	}
	e := Endpoint{Root: repo, Remote: "local", Branch: "refs/heads/main"}
	res, err := Publish(e, PublishRequest{
		Opid: "op-solo-seed", Machine: "mac-solo", Lineage: "l1",
		Intent: testIntentFor("migrate"), Message: "seed solo ledger",
		Mutate: func(tip string) ([]Change, error) { return changes, nil },
	})
	if err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("solo seed: %+v %v", res, err)
	}
	return repo
}

func TestWorkingDurationGrammar(t *testing.T) {
	for token, want := range map[string]time.Duration{
		"30m":   30 * time.Minute,
		"2h30m": 2*time.Hour + 30*time.Minute,
		"1d2h":  10 * time.Hour,
	} {
		got, ok := ParseWorkingDuration(token)
		if !ok || got != want || FormatWorkingDuration(got) != token {
			t.Fatalf("duration %s parsed as %v, %v and formatted %s", token, got, ok, FormatWorkingDuration(got))
		}
	}
	for _, token := range []string{"", "0h", "2.5h", "2h30", "1s"} {
		if _, ok := ParseWorkingDuration(token); ok {
			t.Fatalf("invalid duration %q was accepted", token)
		}
	}
}
