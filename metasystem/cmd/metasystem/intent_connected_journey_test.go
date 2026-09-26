package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

// journeyBed is the connected journey's real-owner bed: a checkout with a
// remote, a synced ledger, a fixture testing contract, an announced holder,
// the real close owner, and the fake critic completion the reap performs.
type journeyBed struct {
	c            *connectionBed
	owners       intentOwners
	b            *deliveryBed
	do           func(args ...string) (int, intentResult)
	dispositions string
	job          func(install, id string) map[string]any
	finish       func(install, id, commit string)
	finishWith   func(install, id, commit string, findings []any)
	finishRound  func(install, root, id, commit string, round int, findings []any)
	landingRoot  string
	baseline     string
}

func newJourneyBed(t *testing.T) *journeyBed {
	c := newConnectionBed(t)
	owners := c.connectionOwners()
	b := &deliveryBed{intentBed: c.intentBed, install: c.root(), owners: owners.delivery}
	realCloseOwner(t, b)
	conf := filepath.Join(c.root(), "metasystem.conf")
	confBytes, _ := os.ReadFile(conf)
	os.WriteFile(conf, append(confBytes, []byte("\nevidence.root="+t.TempDir()+"\n")...), 0o644)
	// The close scripts are part of the baseline every goal checkout starts
	// from, so the critic's own checkout closes its own chain.
	// The same native goal bytes the bed projects become a physical
	// accepted goal at the baseline: the backlog root syncs with origin, the
	// goal configuration names this machine and the approver, and the batch
	// owners find a module and a tiny committed testing contract.
	backlogPath := filepath.Join(c.root(), "plans", "goals", "backlog.md")
	backlogBytes, err := os.ReadFile(backlogPath)
	if err != nil {
		t.Fatal(err)
	}
	backlog, problems := goal.ParseRoot(backlogBytes)
	if len(problems) != 0 {
		t.Fatalf("backlog root: %v", problems)
	}
	backlog.SyncMode = goal.SyncRemote
	os.WriteFile(backlogPath, goal.RenderRoot(backlog), 0o644)
	for key, value := range map[string]string{"metasystem.goal.machine": "mac-cli", "goal.sync-remote": "origin",
		"goal.sync-branch": "refs/heads/main", "goal.human.Wido": "Wido Approver <wido@example.invalid>"} {
		connectionGit(t, c.root(), "config", key, value)
	}
	confBytes, _ = os.ReadFile(conf)
	os.WriteFile(conf, append(confBytes, []byte("goal.human.wido=Wido Approver <wido@example.invalid>\ntesting.contract=contracts/fixture.json\n")...), 0o644)
	os.WriteFile(filepath.Join(c.root(), "go.mod"), []byte("module fixture\n"), 0o644)
	contract, err := json.Marshal(testpolicy.Contract{
		SchemaVersion: 1,
		ProjectRisk:   testpolicy.ProjectRisk{Severity: 1, Exposure: 1, Reversibility: "revert", Detection: "immediate", Recovery: "bounded"},
		Surfaces:      []testpolicy.Surface{{ID: "fixture", Paths: []string{"go.mod", "*.txt"}, Standard: []string{"go-fixture"}, Critical: []string{"go-fixture"}}},
		Groups: []testpolicy.Group{{ID: "go-fixture", Kind: "unit", Adapter: "go", CWD: ".", Inputs: []string{"go.mod", "*.txt"},
			Obligations: []string{"go-fixture"}, Platforms: []string{"any"}, TargetMS: 1, Packages: []string{"./..."}, Tests: json.RawMessage(`"all"`)}},
		Always: testpolicy.Always{Canary: []string{"go-fixture"}}, Unknown: []string{"go-fixture"}, Cadence: []string{},
	})
	if err != nil {
		t.Fatal(err)
	}
	os.MkdirAll(filepath.Join(c.root(), "contracts"), 0o700)
	os.WriteFile(filepath.Join(c.root(), "contracts", "fixture.json"), contract, 0o644)
	connectionGit(t, c.root(), "add", "-A")
	connectionGit(t, c.root(), "commit", "-q", "-m", "close owner baseline")
	connectionGit(t, c.root(), "push", "-q", "origin", "main")
	baseline := connectionGit(t, c.root(), "rev-parse", "HEAD")
	connectionGit(t, c.root(), "update-ref", goal.LocalLedgerBranch, baseline)
	connectionGit(t, c.root(), "update-ref", goal.AcceptedRef, baseline)
	// Fixture lease authority for this test process, and a dedicated landing
	// checkout cloned from the same bare origin.
	exact, state, err := (identity.KernelProber{}).Probe(int64(os.Getpid()))
	if err != nil || state != identity.Alive {
		t.Fatalf("fixture process identity: %s %v", state, err)
	}
	if _, err := lease.AnnounceWithPair(c.root(), "connection-fixture", int64(os.Getpid()), exact.StartedAt.Unix(), exact.StartTicks, exact.BootID, "fixture", "fake", "m1"); err != nil {
		t.Fatal(err)
	}
	landingRoot := filepath.Join(t.TempDir(), "landing")
	connectionGit(t, filepath.Dir(landingRoot), "clone", "-q", c.origin, landingRoot)
	do := func(args ...string) (int, intentResult) {
		t.Helper()
		command, ok := findIntentCommand(args[0])
		if !ok {
			t.Fatalf("no public command %q", args[0])
		}
		var stdout, stderr bytes.Buffer
		code := runIntentIn(command, append([]string{"--json"}, args[1:]...), &stdout, &stderr, c.root(), owners)
		var result intentResult
		if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
			t.Fatalf("%v printed no JSON result: %v; stdout=%q stderr=%q", args, err, stdout.String(), stderr.String())
		}
		return code, result
	}
	dispositions := filepath.Join(t.TempDir(), "dispositions.md")
	os.WriteFile(dispositions, []byte(deliveryDispositionsHeader+"| F1 | noted | fixture non-material observation | none |\n"), 0o600)
	job := func(install, id string) map[string]any {
		var record map[string]any
		data, err := os.ReadFile(filepath.Join(install, "artifacts", "agents", "jobs", id+".json"))
		if err != nil || json.Unmarshal(data, &record) != nil {
			t.Fatalf("job %s unreadable: %v", id, err)
		}
		return record
	}
	// finish is the fake model's completion of critic id on commit: the
	// frozen subject, the terminal record and a return bound to the subject
	// tree, then the actual register advance a reap performs.
	var finishWith func(install, id, commit string, findings []any)
	finish := func(install, id, commit string) {
		t.Helper()
		finishWith(install, id, commit, []any{map[string]any{"id": "F1", "material": false}})
	}
	var finishRound func(install, root, id, commit string, round int, findings []any)
	finishWith = func(install, id, commit string, findings []any) {
		t.Helper()
		finishRound(install, id, id, commit, 1, findings)
	}
	// finishRound completes round N of root's chain as job id: the round's
	// own subject and return under the root, and the round's register advance.
	finishRound = func(install, root, id, commit string, round int, findings []any) {
		t.Helper()
		subject, present, err := dispatchcore.ComputeReadSubject(dispatchcore.ReadSubjectRequest{RepoRoot: install, Role: "code-critic", Reviews: "commit:" + commit})
		if err != nil || !present {
			t.Fatalf("critic %s subject present=%v err=%v", id, present, err)
		}
		agents := filepath.Join(install, "artifacts", "agents")
		rounds := filepath.Join(agents, root, "rounds", strconv.Itoa(round))
		c.writeJSON(filepath.Join(rounds, "subject.json"), subject)
		c.writeJSON(filepath.Join(rounds, "return.json"),
			map[string]any{"jobId": id, "round": round, "findings": findings, "verdict": fmt.Sprintf("%d finding(s)", len(findings)), "reviewedTree": subject.Tree})
		c.writeJSON(filepath.Join(agents, "capabilities", "close.json"), map[string]any{"ok": true})
		os.MkdirAll(filepath.Join(agents, "record-locks"), 0o700)
		record := job(install, id)
		var parent any
		if round > 1 {
			parent = root
		}
		for key, value := range map[string]any{"status": "completed", "destructiveReach": "DESIGN-BEARING", "dispatchMode": "fresh",
			"sessionId": root + "-session", "parentJob": parent, "capabilitySnapshot": "artifacts/agents/capabilities/close.json",
			"endedAt": "2026-09-25T12:00:00Z", "reviewRoundLimit": 3} {
			record[key] = value
		}
		c.writeJSON(filepath.Join(agents, "jobs", id+".json"), record)
		if outcome, err := dispatchcore.CritiqueRegisterAdvance(install, root, id); err != nil || outcome != "advanced" {
			t.Fatalf("register advance of %s = %q, %v", id, outcome, err)
		}
	}
	return &journeyBed{c: c, owners: owners, b: b, do: do, dispositions: dispositions, job: job, finish: finish, finishWith: finishWith, finishRound: finishRound, landingRoot: landingRoot, baseline: baseline}
}

// TestIntentConnectedJourneyRealClose (VMI-10) drives one goal's units
// through the public surface on one physical repository with a bare origin:
// build A, review unit A, real close, collection; build B the same way; fold
// A and review it again while B survives, and review the rewritten B commit;
// then public land admits exactly those subjects into the actual batch join
// on the same physical accepted goal. Each committed critic is closed by
// the actual scripts/agents/dispatch.sh close and the real engine, after the
// actual register advance of a subject-bound fake model return; nothing here
// stamps a closure. Model launches, proof, the fast gate, the claim and the
// commit token stay the connection bed's declared fixture effects; the lease
// and brain fence are realCloseOwner's; the batch join's goal binding,
// claim handover, admission proof, plan and owner start are declared fixture
// effects. Asynchronous sealing, proof and push are proved separately
// (TestBatchLandingLifecycleEndToEnd). It sets METASYSTEM_BIN and the
// connection bed's Git configuration environment, so it runs serially.
func TestIntentConnectedJourneyRealClose(t *testing.T) {
	j := newJourneyBed(t)
	c, owners, b, do, dispositions, job, finish, landingRoot, baseline := j.c, j.owners, j.b, j.do, j.dispositions, j.job, j.finish, j.landingRoot, j.baseline
	_ = owners
	// reviewed takes run through review unit, the fake model's completion,
	// the printed close run by the real owner, and the collecting review.
	reviewed := func(run string, extra ...string) (critic, commit string) {
		t.Helper()
		code, result := do(append([]string{"review", "unit", run}, extra...)...)
		if result.Outcome != "in-progress" || len(c.delegates) == 0 {
			t.Fatalf("review unit %s: code=%d %+v", run, code, result)
		}
		critic, commit = "crit"+strconv.Itoa(len(c.delegates)), c.delegates[len(c.delegates)-1]
		finish(c.worktree, critic, commit)
		code, result = do(append([]string{"review", "unit", run}, extra...)...)
		if result.Outcome != "in-progress" || !strings.Contains(result.Decision, "review unit "+run) || !strings.Contains(result.Decision, "--dispositions FILE") ||
			strings.Contains(result.Decision, "metasystem close") {
			t.Fatalf("an unclosed critic must print its public decision route: code=%d %+v", code, result)
		}
		calls := len(b.calls)
		if code, result = do("close", critic, "--repo", c.worktree, "--dispositions", dispositions); code != 0 || result.Outcome != intentConfirmed {
			t.Fatalf("close %s: code=%d %+v calls=%v", critic, code, result, b.calls[calls:])
		}
		record := job(c.worktree, critic)
		if record["chainClosed"] != true || record["runnerClosed"] != nil || record["closure"] == nil {
			t.Fatalf("the real close owner must stamp %s closed: %v", critic, record)
		}
		if _, err := os.Stat(filepath.Join(c.worktree, "artifacts", "agents", "locks", critic+".d")); !os.IsNotExist(err) {
			t.Fatalf("close left %s's lock behind", critic)
		}
		calls = len(b.calls)
		if code, result = do("close", critic, "--repo", c.worktree, "--dispositions", dispositions); code != 0 || len(b.calls) != calls {
			t.Fatalf("a repeated close must change nothing: code=%d %+v calls=%v", code, result, b.calls[calls:])
		}
		publications := c.publications
		if code, result = do(append([]string{"review", "unit", run}, extra...)...); code != 0 || result.Outcome != intentConfirmed || c.publications != publications+1 {
			t.Fatalf("collect %s: code=%d %+v", run, code, result)
		}
		return critic, commit
	}
	originFile := func(path string) string {
		return connectionGit(t, c.root(), "--git-dir", c.origin, "show", "refs/heads/goal/"+c.id+":"+path)
	}

	brief := c.brief("brief.md", "Build the unit.\n")
	build := func(unit string, edits map[string]string) string {
		t.Helper()
		c.edits = edits
		code, result := do(append([]string{"build", c.id, unit, "--brief", brief, "--lines", "5"}, workCheck...)...)
		if code != 0 || result.Outcome != intentConfirmed {
			t.Fatalf("build %s: code=%d %+v", unit, code, result)
		}
		return resultData(t, result)["run"].(string)
	}
	runA := build("unit-a", map[string]string{"a.txt": "first A\n"})
	criticA, commitA := reviewed(runA)
	// The default path names no critic model; a caller's --model reaches the
	// committed read's dispatch through the branch read owner.
	for _, args := range c.reads {
		if slices.Contains(args, "requested-critic") {
			t.Fatalf("the default review named a caller model: %v", args)
		}
	}
	runB := build("unit-b", map[string]string{"b.txt": "the B bytes\n"})
	// Coexistence: A's read is collected and published while B is built and
	// unread. A publication the push owner has not recorded makes A's read
	// collected but unpublished, and the same review G publishes it again.
	stages := func() map[string]string {
		t.Helper()
		code, result := do("status", "goal", c.id)
		views, _ := resultData(t, result)["work"].([]any)
		found := map[string]string{}
		for _, view := range views {
			item, _ := view.(map[string]any)
			found[fmt.Sprint(item["work"])] = fmt.Sprint(item["stage"])
		}
		if code != 0 || len(found) != 2 {
			t.Fatalf("status goal %s: code=%d %+v", c.id, code, result)
		}
		return found
	}
	if found := stages(); !strings.HasPrefix(found["unit-a"], "reviewed; its read is collected and published") || found["unit-b"] != "built, ready for review" {
		t.Fatalf("collected-published beside built-unread: %v", found)
	}
	originTip := "refs/metasystem/goals/origin/" + c.id
	published := connectionGit(t, c.root(), "rev-parse", originTip)
	connectionGit(t, c.root(), "update-ref", originTip, commitA)
	if found := stages(); !strings.HasPrefix(found["unit-a"], "reviewed; its read is collected but not yet published") || found["unit-b"] != "built, ready for review" {
		t.Fatalf("collected-unpublished beside built-unread: %v", found)
	}
	if code, result := do("status", "goal", c.id, "--work", "unit-a"); code != 0 || result.Next == nil || !slices.Equal(result.Next.Argv[1:], []string{"review", c.id, "--work", "unit-a"}) {
		t.Fatalf("an unpublished read continues with its review: code=%d %+v", code, result)
	}
	if code, result := do("review", c.id, "--work", "unit-a"); code != 0 || (result.Outcome != intentConfirmed && result.Outcome != intentUnchanged) ||
		resultData(t, result)["state"] != "already-collected" {
		t.Fatalf("review G republishes the collected read: code=%d %+v", code, result)
	}
	if found := stages(); !strings.HasPrefix(found["unit-a"], "reviewed; its read is collected and published") ||
		connectionGit(t, c.root(), "rev-parse", originTip) != published {
		t.Fatalf("after republication: %v", found)
	}
	reads := len(c.reads)
	criticB, commitB := reviewed(runB, "--model", "requested-critic")
	if model := flagValue(c.reads[reads], "--model"); model != "requested-critic" || flagValue(c.reads[reads], "--unit") != commitB {
		t.Fatalf("review unit --model must reach the read owner for B's commit: %v", c.reads[reads])
	}
	if code, result := do("review", "unit", runB, "--effort", "high"); code != 2 || result.Outcome != intentRefused {
		t.Fatalf("a review effort override stays refused: code=%d %+v", code, result)
	}
	if status := c.landAdmission(); status.Prefix != 2 || status.Units[0].Commit != commitA || status.Units[1].Commit != commitB {
		t.Fatalf("both units must be read on the published branch: %+v", status)
	}

	// A same-unit correction: fold A, review it again. B's unit survives
	// with its exact bytes; A's old read does not carry over.
	c.edits = map[string]string{"a.txt": "amended A\n"}
	if code, result := do("fold", "unit", runA, "--brief", c.brief("fix.md", "Amend A.\n")); code != 0 || result.Outcome != intentConfirmed {
		t.Fatalf("fold unit A: code=%d %+v", code, result)
	}
	criticA2, commitA2 := reviewed(runA)
	if commitA2 == commitA || criticA2 == criticA {
		t.Fatalf("the fold must be read as a new subject by a new critic: %s/%s", criticA2, commitA2)
	}
	if originFile("a.txt") != "amended A" || originFile("b.txt") != "the B bytes" {
		t.Fatalf("the published branch must carry amended A and B's exact bytes")
	}
	status := c.landAdmission()
	var subjects []string
	for _, unit := range status.Units {
		subjects = append(subjects, unit.Commit)
	}
	if len(status.Units) != 2 || status.Units[0].Commit != commitA2 || slices.Contains(subjects, commitA) {
		t.Fatalf("admission must see the replacement A first and never the old A: %+v", status)
	}
	// Rewriting A's prefix leaves B's replayed commit without an accepted
	// read; the current B commit gets its own committed review, closed by
	// the real owner and collected, never a copied closure.
	currentB := status.Units[1].Commit
	if status.Prefix != 1 || currentB == commitB || status.Units[1].ReadState != "built" {
		t.Fatalf("the replayed B must await its own read: %+v", status)
	}
	review := []string{"review", "commit", currentB, "--goal", c.id, "--brief", brief, "--repo", c.worktree}
	if code, result := do(review...); result.Outcome != "in-progress" || c.delegates[len(c.delegates)-1] != currentB {
		t.Fatalf("review commit of the current B: code=%d %+v", code, result)
	}
	criticB2 := "crit" + strconv.Itoa(len(c.delegates))
	finish(c.worktree, criticB2, currentB)
	// One public review with the author's decisions: the real whole close
	// owner closes and mirrors the chain, then the read is collected and
	// published.
	if code, result := do(append(review, "--dispositions", dispositions)...); code != 0 || result.Outcome != intentConfirmed || job(c.worktree, criticB2)["chainClosed"] != true {
		t.Fatalf("decide, close and collect the current B read: code=%d %+v", code, result)
	}
	status = c.landAdmission()
	if status.Prefix != 2 || status.Units[0].Commit != commitA2 || status.Units[1].Commit != currentB {
		t.Fatalf("admission must see replacement A and current B read: %+v", status)
	}
	if originFile("b.txt") != "the B bytes" {
		t.Fatal("B's bytes changed")
	}
	if len(c.delegates) != 4 || criticB == criticB2 {
		t.Fatalf("exactly four committed critics: %v", c.delegates)
	}

	// Public land of exactly these subjects into the actual batch join. The
	// binding, handover, admission proof, plan and owner start are declared
	// fixture effects; member reading, transport, assembly, the protected
	// test gate and the batch store are production code.
	publishedTip := connectionGit(t, c.root(), "--git-dir", c.origin, "rev-parse", "refs/heads/goal/"+c.id)
	landOwners, counts := journeyLandOwners(t, j)
	projected := func(root string, at time.Time) *goal.GoalFile {
		endpoint, err := goalBranchEndpoint(root)
		if err != nil {
			t.Fatal(err)
		}
		projection, err := goal.Project(endpoint, true, at)
		if err != nil {
			t.Fatal(err)
		}
		return projection.Tree.Live[c.id]
	}
	effects := func() [4]int { return [4]int{len(b.calls), len(c.delegates), c.commits, c.commitReads} }
	before := effects()
	land := func() intentResult {
		t.Helper()
		command, _ := findIntentCommand("land")
		var stdout, stderr bytes.Buffer
		runIntentIn(command, []string{c.id, "--repo", c.root(), "--json"}, &stdout, &stderr, c.root(), landOwners)
		var result intentResult
		if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
			t.Fatalf("land printed no result: %v; %q %q", err, stdout.String(), stderr.String())
		}
		return result
	}
	result := land()
	data, _ := result.Data.(map[string]any)
	if result.Outcome != intentInProgress || data["route"] != "batch" || data["joinedNow"] != true || counts.handovers != 1 || counts.admissions != 1 || counts.ensures != 1 {
		t.Fatalf("public land = %+v; counts %+v", result, *counts)
	}
	id, _ := data["batchId"].(string)
	record, err := batch.NewStore(landingRoot, identity.KernelProber{}).Load(id)
	if err != nil || len(record.Units) != 1 {
		t.Fatalf("stored batch %s = %+v, %v", id, record, err)
	}
	member := record.Units[0]
	if member.State != batch.UnitJoined || member.Admission == nil || member.Admission.Status != "verified" || member.Admission.AttemptID != "connection-admission" ||
		!member.GoalLast || member.BranchTip != publishedTip || member.Approver != "Wido" || len(member.Builds) != 2 {
		t.Fatalf("the admitted member does not bind this goal branch: %+v admission %+v", member, member.Admission)
	}
	for index, want := range []struct{ commit, critic string }{{commitA2, criticA2}, {currentB, criticB2}} {
		build := member.Builds[index]
		if build.Commit != want.commit || build.Attestation.Source.Kind != "critic-root" || build.Attestation.Source.RootJob != want.critic {
			t.Fatalf("member build %d = %s from %+v; want %s read by closed %s", index, build.Commit, build.Attestation.Source, want.commit, want.critic)
		}
		if closed := job(c.worktree, want.critic); closed["chainClosed"] != true {
			t.Fatalf("%s's chain is not closed", want.critic)
		}
	}
	again := land()
	if again.Outcome != intentInProgress || again.Data.(map[string]any)["joinedNow"] != false || again.Data.(map[string]any)["batchId"] != id ||
		counts.handovers != 1 || counts.admissions != 1 || effects() != before {
		t.Fatalf("a repeat reads the stored membership and never joins again: %+v; effects %v -> %v", again, before, effects())
	}
	if file := projected(c.root(), time.Now()); file == nil || file.State == goal.StateDone {
		t.Fatalf("landing admission must not conclude the goal: %+v", file)
	}
	if tip := connectionGit(t, c.root(), "--git-dir", c.origin, "rev-parse", "refs/heads/main"); tip != baseline {
		t.Fatalf("origin main moved to %s; admission never seals or pushes", tip)
	}
}

// journeyLandCounts are the declared fixture effects of a public land.
type journeyLandCounts struct{ handovers, admissions, ensures int }

// journeyLandOwners are the journey bed's owners for public land into the
// actual batch join at its landing root. The binding, handover, admission
// proof, plan and owner start are declared fixture effects; member reading,
// transport, assembly, the protected test gate and the batch store are
// production code.
func journeyLandOwners(t *testing.T, j *journeyBed) (intentOwners, *journeyLandCounts) {
	c, owners := j.c, j.owners
	counts := &journeyLandCounts{}
	deps := productionBatchJoinDependencies()
	deps.costForecast = nil
	deps.handover = func(batchJoinRequest, string, batch.Claim) error { counts.handovers++; return nil }
	deps.admissionRun = func(_ string, _ string, unit batch.Unit) (batch.JoinAdmission, error) {
		counts.admissions++
		if unit.State != batch.UnitJoining || unit.Admission == nil || unit.Admission.Status != "handed-over" {
			t.Fatalf("admission ran before handover: %+v", unit)
		}
		return batch.JoinAdmission{Tree: unit.Admission.Tree, Status: "verified", AttemptID: "connection-admission"}, nil
	}
	deps.plan = func(string, string, string) (testpolicy.Plan, error) {
		return testpolicy.Plan{SelectedGroups: []string{"go-fixture"}, RequiredGroups: []string{"go-fixture"}}, nil
	}
	deps.ensure = func(string) error { counts.ensures++; return nil }
	projected := func(root string, at time.Time) *goal.GoalFile {
		endpoint, err := goalBranchEndpoint(root)
		if err != nil {
			t.Fatal(err)
		}
		projection, err := goal.Project(endpoint, true, at)
		if err != nil {
			t.Fatal(err)
		}
		return projection.Tree.Live[c.id]
	}
	deps.binding = func(root, goalID string, at time.Time) (dispatchcore.GoalBinding, error) {
		file := projected(root, at)
		if file == nil || file.Claimed == nil {
			t.Fatalf("the physical accepted goal %s is not claimed", goalID)
		}
		binding := dispatchcore.GoalBinding{GoalID: goalID, Revision: file.Revision, Machine: "mac-cli", Lineage: "m1", File: file}
		binding.Capability.ClaimEpoch = 1
		return binding, nil
	}
	landOwners := owners
	delivery := defaultIntentDeliveryOwners()
	delivery.branchRead, delivery.process = owners.delivery.branchRead, owners.delivery.process
	delivery.batchRoot = func(string, time.Time) (string, bool, error) { return j.landingRoot, true, nil }
	delivery.batchJoin = func(request batchJoinRequest) (batch.Record, error) { return executeBatchJoin(request, deps) }
	landOwners.delivery = delivery
	return landOwners, counts
}
