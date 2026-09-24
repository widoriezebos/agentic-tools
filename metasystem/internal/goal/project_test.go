package goal

import (
	"strings"
	"testing"
	"time"
)

func TestProjectionReadsTheAcceptedTreeOnly(t *testing.T) {
	t.Parallel()
	a, b := fakeGoalEndpointPair(t)
	if res, err := Open(verbReqFor(a, "01J5X00000000000000000E000", "mac-a"), "seen", "Visible.", "main", "Go."); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("open: %+v %v", res, err)
	}
	// B publishes past A's accepted tree; A's OFFLINE projection
	// still shows only what A accepted — mid-edit remote state is
	// invisible until a fetch.
	if res, err := Open(verbReqFor(b, "01J5X00000000000000000E010", "mac-b"), "unseen", "Not yet.", "main", "Go."); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("B opens: %+v %v", res, err)
	}
	now := time.Now()
	p, err := Project(a, false, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, visible := p.Tree.Live["unseen"]; visible {
		t.Fatal("the offline projection never shows an unaccepted tip")
	}
	if _, visible := p.Tree.Live["seen"]; !visible {
		t.Fatal("the accepted world is fully visible")
	}
	// With --fetch the read-side validator advances first. The fixture owns both
	// deadline boundaries so delayed scheduling cannot turn this semantic proof
	// into a transport-timeout assertion.
	fetchStarted := make(chan struct{})
	releaseFetch := make(chan struct{})
	released := false
	defer func() {
		if !released {
			close(releaseFetch)
		}
	}()
	type projectionAnswer struct {
		projection Projection
		err        error
	}
	projected := make(chan projectionAnswer, 1)
	go func() {
		projection, projectErr := project(a, true, now, projectionDependencies{
			deadline: make(chan time.Time),
			fetch: func(endpoint Endpoint) (AdvanceResult, error) {
				close(fetchStarted)
				<-releaseFetch
				return FetchAdvance(endpoint)
			},
		})
		projected <- projectionAnswer{projection: projection, err: projectErr}
	}()
	<-fetchStarted
	select {
	case answer := <-projected:
		t.Fatalf("fetch returned before its delayed-event release: %+v %v", answer.projection, answer.err)
	default:
	}
	close(releaseFetch)
	released = true
	answer := <-projected
	p2, err := answer.projection, answer.err
	if err != nil {
		t.Fatal(err)
	}
	if _, visible := p2.Tree.Live["unseen"]; !visible {
		t.Fatal("the fetching projection advances onto the validated tip")
	}
	// The frontier read: both queued goals are ready.
	v, err := Next(p2, "mac-a")
	if err != nil {
		t.Fatal(err)
	}
	if len(v.Awaiting) != 2 || len(v.Ready) != 0 || len(v.Claimed) != 0 {
		t.Fatalf("the frontier names unapproved work as awaiting: %+v", v)
	}
}

func TestNextFiltersCandidatesButNeverTheHeldClaim(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	seedGoalNormConfig(t, root)
	held := vGoal("held", StateClaimed)
	held.Claimed = &ClaimRecord{Machine: "mac-a", Lineage: "lin-1", At: "2026-08-20T10:05:00Z"}
	held.Labels = []string{"other"}
	one := approvedGoalFixture(vGoal("one", StateQueued), testBudget())
	one.Labels = []string{"alpha", "shared"}
	two := approvedGoalFixture(vGoal("two", StateQueued), testBudget())
	two.Labels = []string{"beta", "shared"}
	p := Projection{Root: root, Tree: &TreeGoals{Live: map[string]*GoalFile{
		"held": held, "one": one, "two": two,
	}, Done: map[string]*GoalFile{}}}

	v, err := Next(p, "mac-a", "alpha")
	if err != nil {
		t.Fatal(err)
	}
	if len(v.Claimed) != 1 || v.Claimed[0] != "held" || len(v.Ready) != 1 || v.Ready[0] != "one" {
		t.Fatalf("the held claim remains first while the candidate set narrows: %+v", v)
	}
	v, err = Next(p, "other-machine", "shared", "beta")
	if err != nil {
		t.Fatal(err)
	}
	if len(v.Claimed) != 0 || len(v.Ready) != 1 || v.Ready[0] != "two" {
		t.Fatalf("repeated labels combine with AND: %+v", v)
	}
	v, err = Next(p, "other-machine", "absent")
	if err != nil {
		t.Fatal(err)
	}
	if len(v.Claimed) != 0 || len(v.Ready) != 0 || len(v.Blocked) != 0 {
		t.Fatalf("an empty filtered candidate set is distinguishable: %+v", v)
	}
}

func TestNextTreatsArcMemberPinsIndependently(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	seedGoalNormConfig(t, root)
	foreign := approvedGoalFixture(vGoal("foreign-pinned", StateQueued), testBudget())
	foreign.Arc = "shared-arc"
	foreign.Pinned = "mac-b"
	local := approvedGoalFixture(vGoal("local-member", StateQueued), testBudget())
	local.Arc = "shared-arc"
	p := Projection{Root: root, Tree: &TreeGoals{Live: map[string]*GoalFile{
		foreign.Id: foreign, local.Id: local,
	}, Done: map[string]*GoalFile{}}}
	verdict, err := Next(p, "mac-a")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(verdict.Ready, ",") != "local-member" {
		t.Fatalf("a sibling's foreign pin hid an independently claimable member: %+v", verdict)
	}
}

func TestProjectionBannersStalenessAndLocalMode(t *testing.T) {
	t.Parallel()
	e := soloLedgerFakeEndpoint(t)
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
	v, err := Next(p, "mac-solo")
	if err != nil {
		t.Fatal(err)
	}
	if len(v.Ready) != 1 || v.Ready[0] != "solo-goal" {
		t.Fatalf("the solo frontier: %+v", v)
	}
}

func TestSyncModeMismatchRefusesByName(t *testing.T) {
	t.Parallel()
	e := soloLedgerFakeEndpoint(t)
	// The ledger says local; the config says remote: the forbidden
	// promotion refuses naming the goal.
	e.Remote = "origin"
	_, err := Project(e, false, time.Now())
	if err == nil || !strings.Contains(err.Error(), "backlog-local-promotion") {
		t.Fatalf("the mode flip refuses toward the promotion goal: %v", err)
	}
}

func soloLedgerFakeEndpoint(t *testing.T) Endpoint {
	t.Helper()
	root := t.TempDir()
	seedGoalNormConfig(t, root)
	store := newFakeGoalStore()
	e := Endpoint{Root: root, Remote: "local", Branch: LocalLedgerBranch, Repository: store.client()}
	localRoot := vRoot()
	localRoot.SyncMode = SyncLocal
	files := vTree(localRoot, []*GoalFile{approvedGoalFixture(vGoal("solo-goal", StateQueued), testBudget())}, nil)
	changes := make([]Change, 0, len(files))
	for path, content := range files {
		changes = append(changes, Change{Path: path, Content: content})
	}
	res, err := Publish(e, PublishRequest{
		Opid: "op-solo-seed", Machine: "mac-solo", Lineage: "l1",
		Intent: testIntentFor("migrate"), Message: "seed solo ledger",
		Mutate: func(string) ([]Change, error) { return changes, nil },
	})
	if err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("solo seed: %+v %v", res, err)
	}
	return e
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
	files := vTree(root, []*GoalFile{approvedGoalFixture(vGoal("solo-goal", StateQueued), testBudget())}, nil)
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
	t.Parallel()
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
