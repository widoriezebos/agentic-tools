package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/ownercall"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
)

// wholeOwnerLanding is one claimed, approved, land-ready goal with a single
// reader-record unit and a plan fold on its pushed goal branch, in temporary
// repositories with a temporary bare upstream.
type wholeOwnerLanding struct {
	goalRoot, mainRoot, upstream, base, unit, branchTip string
	receipts                                            []string
}

func newWholeOwnerLanding(t *testing.T) *wholeOwnerLanding {
	t.Helper()
	root, upstream, _ := goalBranchTemplateCLIFixture(t, "m1")
	f := &wholeOwnerLanding{goalRoot: root, upstream: upstream, mainRoot: goalBranchHolderRoot(root)}
	// The public commands resolve a self-hosted checkout by its template
	// signal (metasystem.template=true); the design page is ordinary content.
	writeTestingFixtureFile(t, filepath.Join(filepath.Dir(f.mainRoot), "development", "metasystem-design.md"), []byte("# fixture\n"), 0o644)
	goalSyncMutationGit(t, root, "config", "goal.human.Wido", "Wido Approver <wido@example.invalid>")
	pagePath := filepath.Join(root, "plans", "goals", "standing-validation.md")
	pageData, err := os.ReadFile(pagePath)
	if err != nil {
		t.Fatal(err)
	}
	file, problems := goal.ParseFile(pageData)
	if len(problems) != 0 {
		t.Fatalf("parse goal page: %v", problems)
	}
	landReadyOpid := goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5FB0", "mac-cli", "m1")
	file.Revision++
	file.Landing = &goal.LandingRecord{At: "2026-09-17T09:00:00Z", Opid: landReadyOpid}
	file.History = append(file.History, goal.HistoryLine{At: file.Landing.At, Opid: landReadyOpid,
		Verb: "land-ready", Actor: "mac-cli+m1", Targets: []string{file.Id}, Keep: -1})
	writeTestingFixtureFile(t, pagePath, goal.RenderFile(file), 0o644)
	writeTestingFixtureFile(t, filepath.Join(root, "memory", "receipts.log"),
		[]byte("1|1970-01-01T00:00:00Z|RECEIPT|type=seed|outcome=shipped\n"), 0o644)
	goalSyncMutationGit(t, root, "add", "plans/goals/standing-validation.md", "memory/receipts.log")
	goalSyncMutationGit(t, root, "commit", "-qm", "mark fixture land ready")
	f.base = goalSyncMutationGit(t, root, "rev-parse", "HEAD")
	goalSyncMutationGit(t, root, "push", "-q", "upstream", "HEAD:main")
	goalSyncMutationGit(t, root, "update-ref", goal.LocalLedgerBranch, f.base)
	goalSyncMutationGit(t, root, "update-ref", goal.AcceptedRef, f.base)
	claim := func() error { return nil }
	writeTestingFixtureFile(t, filepath.Join(root, "owned.go"), []byte("package fixture\n\nconst Landed = 1\n"), 0o644)
	goalSyncMutationGit(t, root, "add", "owned.go")
	f.unit, err = branch.CommitStaged(branch.CommitRequest{Repo: root, Remote: "upstream", EndpointTip: f.base,
		GoalID: "standing-validation", Unit: "u1", OpID: "owner-unit", Kind: branch.Unit, CheckClaim: claim})
	if err != nil {
		t.Fatal(err)
	}
	digest, err := branch.UnitDigest(root, f.unit)
	if err != nil {
		t.Fatal(err)
	}
	readerRecord := "metasystem/records/misc/owner-read.md"
	writeTestingFixtureFile(t, filepath.Join(filepath.Dir(root), filepath.FromSlash(readerRecord)), []byte(f.unit+" "+digest+"\n"), 0o644)
	unitTree := goalSyncMutationGit(t, root, "rev-parse", f.unit+"^{tree}")
	if _, _, err := branch.CommitRead(branch.CommitReadRequest{Repo: root, Remote: "upstream", EndpointTip: f.base,
		GoalID: "standing-validation", Unit: "u1", OpID: "owner-read", ReaderRecord: readerRecord,
		GateRunID: "fast-clean", GateTree: unitTree, CheckClaim: claim}); err != nil {
		t.Fatal(err)
	}
	writeTestingFixtureFile(t, filepath.Join(root, "plans", "fold.md"), []byte("folded plan\n"), 0o644)
	goalSyncMutationGit(t, root, "add", "plans/fold.md")
	if _, err := branch.CommitStaged(branch.CommitRequest{Repo: root, Remote: "upstream", EndpointTip: f.base,
		GoalID: "standing-validation", OpID: "owner-fold", Kind: branch.Plan, CheckClaim: claim}); err != nil {
		t.Fatal(err)
	}
	if _, err := branch.Push(branch.PushRequest{Repo: root, Remote: "upstream", EndpointTip: f.base,
		GoalID: "standing-validation", OpID: "owner-push", CheckClaim: claim}); err != nil {
		t.Fatal(err)
	}
	f.branchTip = goalSyncMutationGit(t, root, "rev-parse", "refs/heads/goal/standing-validation")
	return f
}

// land runs the public land command from the main installation, through
// the production owners. Only the landing proof's execution is replaced: it
// returns a green schema-3 receipt of exactly the tree the owner asked to
// prove, so receipt parsing and candidate matching stay the owners' own.
func (f *wholeOwnerLanding) land(t *testing.T) (int, intentResult) {
	t.Helper()
	owners := defaultIntentOwners()
	delivery := belowTheGate(defaultIntentDeliveryOwners())
	delivery.process = func(process intentProcess) intentProcessResult {
		want := []string{"landing", "test-receipt", "--root", f.mainRoot, "--tree", flagValue(process.argv, "--tree"), "--mode", "auto", "--goal", "standing-validation"}
		if len(process.argv) < 2 || !slices.Equal(process.argv[1:], want) {
			t.Fatalf("unexpected owner subprocess %v", process.argv)
		}
		tree := flagValue(process.argv, "--tree")
		f.receipts = append(f.receipts, tree)
		receipt, _ := json.Marshal(map[string]any{"schemaVersion": 3, "tree": tree, "exitStatus": 0,
			"time": "2026-09-17T10:00:00Z", "proof": map[string]any{"attemptId": "whole-owner-proof"}})
		return intentProcessResult{stdout: receipt}
	}
	owners.delivery = processBackedReceipt(delivery)
	command, ok := findIntentCommand("work land")
	if !ok {
		t.Fatal("land command missing")
	}
	var stdout, stderr bytes.Buffer
	code := runIntentIn(command, []string{"standing-validation", "--repo", f.mainRoot, "--json"}, &stdout, &stderr, f.mainRoot, owners)
	var result intentResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("land printed no result: %v; stdout=%q stderr=%q", err, stdout.String(), stderr.String())
	}
	return code, result
}

func (f *wholeOwnerLanding) remote(t *testing.T, ref string) string {
	t.Helper()
	line := goalSyncMutationGit(t, f.mainRoot, "ls-remote", "--refs", "upstream", ref)
	tip, _, _ := strings.Cut(line, "\t")
	return tip
}

func (f *wholeOwnerLanding) retained(t *testing.T, result intentResult) (string, branch.PreparedLanding) {
	t.Helper()
	data, _ := result.Data.(map[string]any)
	dir, _ := data["retained"].(string)
	prepared, err := branch.ReadPreparedLanding(filepath.Join(dir, "prepared"))
	if dir == "" || err != nil {
		t.Fatalf("no retained prepared landing in %+v: %v", result, err)
	}
	return dir, prepared
}

// assertLanded checks the physical landing the owners published: main moved
// to the prepared landing, its series verifies against the attested unit,
// it names its source and carries the code and fold, and the goal stays open.
func (f *wholeOwnerLanding) assertLanded(t *testing.T, prepared branch.PreparedLanding) {
	t.Helper()
	if main := f.remote(t, "refs/heads/main"); main != prepared.Landing || main == f.base {
		t.Fatalf("upstream main = %s, prepared landing %s, base %s", main, prepared.Landing, f.base)
	}
	goalSyncMutationGit(t, f.mainRoot, "fetch", "-q", "upstream", "main")
	series, err := branch.VerifyLandedSeries(f.mainRoot, prepared.Landing)
	if err != nil || len(series) == 0 {
		t.Fatalf("landed series = %+v, %v", series, err)
	}
	for _, entry := range series {
		if entry.Actual != entry.Expected {
			t.Fatalf("landed series entry %+v differs", entry)
		}
	}
	message := goalSyncMutationGit(t, f.mainRoot, "log", "-1", "--format=%B", prepared.Landing)
	if !strings.Contains(message, "Goal-Source: "+f.unit) || !strings.Contains(message, "Goal-Last: standing-validation") {
		t.Fatalf("landing message lacks its source or last marker:\n%s", message)
	}
	if code := goalSyncMutationGit(t, f.mainRoot, "show", prepared.Landing+":metasystem/owned.go"); !strings.Contains(code, "const Landed = 1") {
		t.Fatalf("landed code = %q", code)
	}
	if fold := goalSyncMutationGit(t, f.mainRoot, "show", prepared.Landing+":metasystem/plans/fold.md"); fold != "folded plan" {
		t.Fatalf("landed fold = %q", fold)
	}
	endpoint, err := branch.MainEndpoint(f.mainRoot)
	if err != nil {
		t.Fatal(err)
	}
	projection, err := goal.Project(endpoint, true, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if file := projection.Tree.Live["standing-validation"]; file == nil || file.State == goal.StateDone {
		t.Fatalf("landing concluded the goal: %+v", file)
	}
}

// TestIntentLandWholeOwnerGitAdapter drives the public land command through
// the unchanged hand-landing owners: candidate composition, land-prep,
// land-push and sweep. Physical Git is the claim here: the composed
// candidate tree, the receipt's tree identity, the atomic main and
// landing-ref publication and the goal-ref sweep must agree across those
// owners. Only the expensive proof run is a fake effect.
func TestIntentLandWholeOwnerGitAdapter(t *testing.T) {
	t.Parallel()
	t.Run("lands once and repeats unchanged", func(t *testing.T) {
		f := newWholeOwnerLanding(t)
		code, result := f.land(t)
		if code != 0 || result.Outcome != intentConfirmed || result.Data.(map[string]any)["route"] != "hand" || len(f.receipts) != 1 {
			t.Fatalf("land = %d %+v; receipts %v", code, result, f.receipts)
		}
		if candidate := result.Data.(map[string]any)["candidate"]; candidate != f.receipts[0] {
			t.Fatalf("receipt proved %s, candidate-only composition was %v", f.receipts[0], candidate)
		}
		dir, prepared := f.retained(t, result)
		f.assertLanded(t, prepared)
		if refs := goalSyncMutationGit(t, f.mainRoot, "ls-remote", "--refs", "upstream", "refs/heads/goal/standing-validation", "refs/heads/landing/standing-validation"); refs != "" {
			t.Fatalf("the sweep left branches: %s", refs)
		}
		before, _ := os.Stat(filepath.Join(dir, "prepared", "trunk"))
		code, again := f.land(t)
		if code != 0 || again.Outcome != intentUnchanged || len(f.receipts) != 1 || f.remote(t, "refs/heads/main") != prepared.Landing {
			t.Fatalf("repeat = %d %+v; receipts %v", code, again, f.receipts)
		}
		if after, _ := os.Stat(filepath.Join(dir, "prepared", "trunk")); !after.ModTime().Equal(before.ModTime()) {
			t.Fatal("the repeat regenerated the prepared landing")
		}
	})

	t.Run("refused atomic publication retries the prepared landing", func(t *testing.T) {
		f := newWholeOwnerLanding(t)
		goalSyncMutationGit(t, f.upstream, "config", "receive.denyDeletes", "true")
		code, result := f.land(t)
		if code == 0 || result.Outcome != intentPartial || len(f.receipts) != 1 {
			t.Fatalf("refused publication = %d %+v", code, result)
		}
		_, prepared := f.retained(t, result)
		if f.remote(t, "refs/heads/main") != f.base || f.remote(t, "refs/heads/landing/standing-validation") != prepared.Landing {
			t.Fatal("a refused atomic publication moved main or lost the landing ref")
		}
		goalSyncMutationGit(t, f.upstream, "config", "receive.denyDeletes", "false")
		code, result = f.land(t)
		if code != 0 || result.Outcome != intentConfirmed || len(f.receipts) != 1 {
			t.Fatalf("publication retry = %d %+v; receipts %v", code, result, f.receipts)
		}
		f.assertLanded(t, prepared)
	})

	t.Run("a failed sweep resumes without a second proof", func(t *testing.T) {
		f := newWholeOwnerLanding(t)
		lock := filepath.Join(f.upstream, "refs", "heads", "goal", "standing-validation.lock")
		writeTestingFixtureFile(t, lock, []byte("held\n"), 0o644)
		code, result := f.land(t)
		if code == 0 || result.Outcome != intentPartial || len(f.receipts) != 1 {
			t.Fatalf("sweep failure = %d %+v", code, result)
		}
		dir, prepared := f.retained(t, result)
		var landed intentLanded
		encoded, err := os.ReadFile(filepath.Join(dir, "landed.json"))
		if err != nil || json.Unmarshal(encoded, &landed) != nil || landed.Landing != prepared.Landing || landed.Swept {
			t.Fatalf("retained landing = %+v %v", landed, err)
		}
		if f.remote(t, "refs/heads/main") != prepared.Landing || f.remote(t, "refs/heads/landing/standing-validation") != "" ||
			f.remote(t, "refs/heads/goal/standing-validation") != f.branchTip {
			t.Fatal("publication must have landed while the goal ref stays for the sweep")
		}
		if err := os.Remove(lock); err != nil {
			t.Fatal(err)
		}
		code, result = f.land(t)
		if code != 0 || result.Outcome != intentConfirmed || len(f.receipts) != 1 || f.remote(t, "refs/heads/goal/standing-validation") != "" {
			t.Fatalf("resumed sweep = %d %+v; receipts %v", code, result, f.receipts)
		}
		f.assertLanded(t, prepared)
	})
}

// TestIntentLandProvesTheReceiptInThisProcess is the U9a witness that the
// public land runs its landing proof through the landing test-receipt owner
// in this process (design 6.2): no engine child runs `landing test-receipt`,
// the owner receives the argv the child carried, and it is supplied this
// process as the caller its proof admission classifies.
func TestIntentLandProvesTheReceiptInThisProcess(t *testing.T) {
	t.Parallel()
	f := newWholeOwnerLanding(t)
	owners := defaultIntentOwners()
	delivery := belowTheGate(defaultIntentDeliveryOwners())
	delivery.process = func(process intentProcess) intentProcessResult {
		t.Errorf("an engine child ran: %v", process.argv)
		return intentProcessResult{code: 1}
	}
	var supplied []ownercall.Process
	var reached [][]string
	delivery.calls = defaultIntentOwnerCalls()
	delivery.calls.landingTestReceipt = func(caller ownercall.Process, stdout, stderr io.Writer, dir string, args []string) int {
		supplied, reached = append(supplied, caller), append(reached, args)
		tree := flagValue(args, "--tree")
		receipt, _ := json.Marshal(map[string]any{"schemaVersion": 3, "tree": tree, "exitStatus": 0,
			"time": "2026-09-17T10:00:00Z", "proof": map[string]any{"attemptId": "whole-owner-proof"}})
		stdout.Write(receipt)
		return 0
	}
	owners.delivery = delivery
	command, _ := findIntentCommand("work land")
	var stdout, stderr bytes.Buffer
	code := runIntentIn(command, []string{"standing-validation", "--repo", f.mainRoot, "--json"}, &stdout, &stderr, f.mainRoot, owners)
	if code != 0 {
		t.Fatalf("land = %d; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if len(reached) != 1 || !slices.Equal(reached[0], []string{"--root", f.mainRoot, "--tree", flagValue(reached[0], "--tree"), "--mode", "auto", "--goal", "standing-validation"}) {
		t.Fatalf("receipt owner argv = %q", reached)
	}
	if len(supplied) != 1 || supplied[0].Pid != int64(os.Getpid()) {
		t.Fatalf("the receipt owner was supplied %+v, want this process %d", supplied, os.Getpid())
	}
}

// TestWorkLandHandsInOverRealGit drives public work land over a real goal
// branch with a clean read, pushed to origin, into a registered plain lane
// whose checkout is a nested clone of origin: the seat's own branch reads
// and gates run, one line with the branch's tip lands in the lane
// installation's queue.jsonl, and a repeat appends nothing. Nothing is
// proved or pushed by the seat.
func TestWorkLandHandsInOverRealGit(t *testing.T) {
	t.Parallel()
	f := newWholeOwnerLanding(t)
	landingRoot := filepath.Join(t.TempDir(), "landing")
	goalSyncMutationGit(t, filepath.Dir(landingRoot), "clone", "-q", "-b", "main", f.upstream, landingRoot)
	owners := defaultIntentOwners()
	delivery := belowTheGate(defaultIntentDeliveryOwners())
	delivery.laneRoot = func(string, time.Time) (string, bool, error) { return landingRoot, true, nil }
	delivery.process = func(process intentProcess) intentProcessResult {
		t.Fatalf("the hand-in ran a subprocess %v", process.argv)
		return intentProcessResult{}
	}
	owners.delivery = delivery
	command, _ := findIntentCommand("work land")
	run := func() intentResult {
		t.Helper()
		var stdout, stderr bytes.Buffer
		runIntentIn(command, []string{"standing-validation", "--repo", f.mainRoot, "--json"}, &stdout, &stderr, f.mainRoot, owners)
		var result intentResult
		if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
			t.Fatalf("land printed no result: %v; stdout=%q stderr=%q", err, stdout.String(), stderr.String())
		}
		return result
	}
	result := run()
	if data, _ := result.Data.(map[string]any); result.Outcome != intentConfirmed || data["route"] != "lane" || !strings.Contains(result.Summary, "handed to the lane") {
		t.Fatalf("hand-in = %+v", result)
	}
	// The lane is nested: its records are in the installation, not at the
	// checkout's top.
	install := filepath.Join(landingRoot, "metasystem")
	if _, err := os.Stat(filepath.Join(landingRoot, "artifacts", "agents", "landing", "queue.jsonl")); err == nil {
		t.Fatal("the hand-in wrote the queue at the checkout's top")
	}
	entries, err := plain.Entries(install)
	if err != nil || len(entries) != 1 || entries[0].SHA != f.branchTip || entries[0].Branch != "goal/standing-validation" {
		t.Fatalf("the queue line = %+v %v; want the branch tip %s", entries, err, f.branchTip)
	}
	if again := run(); again.Outcome != intentUnchanged || !strings.Contains(again.Summary, "waiting") {
		t.Fatalf("a repeat = %+v", again)
	}
	if entries, _ := plain.Entries(install); len(entries) != 1 {
		t.Fatalf("a repeat appended: %+v", entries)
	}
	if f.remote(t, "refs/heads/main") != f.base {
		t.Fatal("a hand-in moved main")
	}
}
