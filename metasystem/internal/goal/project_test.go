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
		if strings.Contains(b, "APPETITE BREACH") {
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
		if strings.Contains(b, "APPETITE BREACH") && strings.Contains(b, "hungry") && strings.Contains(b, "raise it with Wido") {
			found = true
		}
	}
	if !found {
		t.Fatalf("the breach banners with the covenant's words: %v", late.Banners)
	}
}
