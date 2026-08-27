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

func TestAppetiteBreachBannersEveryRead(t *testing.T) {
	if d, ok := ParseAppetite("Appetite: 4h then prose"); !ok || d != 4*time.Hour {
		t.Fatalf("token parse: %v %v", d, ok)
	}
	if d, ok := ParseAppetite("Appetite: 1d — a day is eight hours"); !ok || d != 8*time.Hour {
		t.Fatalf("day parse: %v %v", d, ok)
	}
	if _, ok := ParseAppetite("Appetite: half a day of prose"); ok {
		t.Fatal("prose-only appetites declare nothing enforceable")
	}
	if _, ok := ParseAppetite("Design the thing."); ok {
		t.Fatal("no prefix, no appetite")
	}

	_, a, _ := twoClones(t)
	seedLedger(t, a)
	if res, err := Open(verbReq(a, "01J5X00000000000000000AB00", "mac-a"), "hungry", "Bounded work.", "main", "Appetite: 2h build the thing."); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("open: %+v %v", res, err)
	}
	if res, err := Claim(verbReq(a, "01J5X00000000000000000AB10", "mac-a"), "hungry"); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("claim: %+v %v", res, err)
	}
	e := endpointFor(a)
	if _, err := FetchAdvance(e); err != nil {
		t.Fatal(err)
	}
	// Within the appetite: no breach banner.
	early, err := Project(e, false, time.Date(2026, 8, 20, 23, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	for _, b := range early.Banners {
		if strings.Contains(b, "BREACH-") {
			t.Fatalf("no breach inside the appetite: %v", early.Banners)
		}
	}
	// Past it: the breach banners on ANY machine's read.
	late, err := Project(e, false, time.Date(2026, 8, 21, 5, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, b := range late.Banners {
		if strings.Contains(b, "BREACH-STOP") && strings.Contains(b, "hungry") && strings.Contains(b, "Wido's word") {
			found = true
		}
	}
	if !found {
		t.Fatalf("the breach banners with the covenant's words: %v", late.Banners)
	}
}

func TestAppetiteBandMath(t *testing.T) {
	appetite := 4 * time.Hour
	for _, tc := range []struct {
		name      string
		elapsed   time.Duration
		remaining time.Duration
		want      AppetiteBand
	}{
		{"within", 4 * time.Hour, 0, BandWithin},
		{"measured escalation", 4*time.Hour + time.Minute, 0, BandBreachEscalate},
		{"grace edge remains escalation", 5 * time.Hour, 0, BandBreachEscalate},
		{"measured stop", 5*time.Hour + time.Minute, 0, BandBreachStop},
		{"estimate cannot clear measured stop", 5*time.Hour + time.Minute, time.Minute, BandBreachStop},
		{"estimate at edge", 3 * time.Hour, 2 * time.Hour, BandWithin},
		{"estimate tightens to stop", 3 * time.Hour, 2*time.Hour + time.Minute, BandBreachStop},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := EvaluateAppetiteBand(tc.elapsed, appetite, tc.remaining, 25); got != tc.want {
				t.Fatalf("band = %s, want %s", got, tc.want)
			}
		})
	}
	if got := EvaluateAppetiteBand(time.Hour, time.Hour, time.Minute, 0); got != BandBreachStop {
		t.Fatalf("zero grace must still honor the estimate stop path: %s", got)
	}
	maxDuration := time.Duration(1<<63 - 1)
	if got := EvaluateAppetiteBand(maxDuration, maxDuration, 0, 100); got != BandWithin {
		t.Fatalf("large duration overflow changed the measured band: %s", got)
	}
	if got := EvaluateAppetiteBand(maxDuration, maxDuration, time.Nanosecond, 100); got != BandBreachStop {
		t.Fatalf("large duration overflow hid the estimate stop path: %s", got)
	}
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

func TestClaimSnapshotAndHumanEditAuthority(t *testing.T) {
	_, a, _ := twoClones(t)
	seedLedger(t, a)
	if _, err := Open(verbReq(a, "01J5X00000000000000000AC00", "mac-a"), "authority-appetite", "Bounded.", "main", "Appetite: 2h start."); err != nil {
		t.Fatal(err)
	}
	if _, err := Claim(verbReq(a, "01J5X00000000000000000AC10", "mac-a"), "authority-appetite"); err != nil {
		t.Fatal(err)
	}
	project := func(at time.Time) AppetiteBand {
		t.Helper()
		p, err := Project(endpointFor(a), false, at)
		if err != nil {
			t.Fatal(err)
		}
		if len(p.AppetiteBanners) == 0 {
			return BandWithin
		}
		return p.AppetiteBanners[0].Band
	}
	at := time.Date(2026, 8, 21, 1, 0, 0, 0, time.UTC)
	if got := project(at); got != BandBreachStop {
		t.Fatalf("claim snapshot should stop at three hours: %s", got)
	}
	claimantNext := "Appetite: 20h claimant prose cannot move authority."
	if _, err := Edit(verbReq(a, "01J5X00000000000000000AC20", "mac-a"), "authority-appetite", EditFields{NextStep: &claimantNext}); err != nil {
		t.Fatal(err)
	}
	if got := project(at); got != BandBreachStop {
		t.Fatalf("claimant edit moved the threshold: %s", got)
	}
	humanReq := verbReq(a, "01J5X00000000000000000AC30", "mac-a")
	humanReq.Actor.Human = "wido"
	humanNext := "Appetite: 6h Wido raises it."
	if _, err := Edit(humanReq, "authority-appetite", EditFields{NextStep: &humanNext}); err != nil {
		t.Fatal(err)
	}
	if got := project(at); got != BandWithin {
		t.Fatalf("human edit did not raise the threshold: %s", got)
	}
	claimantAgain := "Appetite: 30m claimant prose still cannot move authority."
	if _, err := Edit(verbReq(a, "01J5X00000000000000000AC40", "mac-a"), "authority-appetite", EditFields{NextStep: &claimantAgain}); err != nil {
		t.Fatal(err)
	}
	if got := project(at); got != BandWithin {
		t.Fatalf("later claimant edit displaced the human revision: %s", got)
	}
}

func TestEstimateUsesTheLatestClaimantEvent(t *testing.T) {
	_, a, _ := twoClones(t)
	seedLedger(t, a)
	if _, err := Open(verbReq(a, "01J5X00000000000000000AD00", "mac-a"), "estimate-appetite", "Estimate.", "main", "Appetite: 4h start."); err != nil {
		t.Fatal(err)
	}
	if _, err := Claim(verbReq(a, "01J5X00000000000000000AD10", "mac-a"), "estimate-appetite"); err != nil {
		t.Fatal(err)
	}
	if _, err := Estimate(verbReq(a, "01J5X00000000000000000AD20", "mac-a"), "estimate-appetite", "2h1m"); err != nil {
		t.Fatal(err)
	}
	p, err := Project(endpointFor(a), false, time.Date(2026, 8, 21, 1, 0, 0, 0, time.UTC))
	if err != nil || len(p.AppetiteBanners) != 1 || p.AppetiteBanners[0].Band != BandBreachStop {
		t.Fatalf("estimate should tighten to STOP: %+v %v", p.AppetiteBanners, err)
	}
	if _, err := Estimate(verbReq(a, "01J5X00000000000000000AD30", "mac-a"), "estimate-appetite", "1h"); err != nil {
		t.Fatal(err)
	}
	p, err = Project(endpointFor(a), false, time.Date(2026, 8, 21, 1, 0, 0, 0, time.UTC))
	if err != nil || len(p.AppetiteBanners) != 0 {
		t.Fatalf("latest within-band estimate should clear only the forecast stop: %+v %v", p.AppetiteBanners, err)
	}
}
