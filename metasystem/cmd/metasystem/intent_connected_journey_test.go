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

	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/readsubject"
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
	baseline     string
}

func journeyCleanFinding() readsubject.Finding {
	finding := stopFinding("regression", "connect.txt")
	finding.ID, finding.Material = "F1", false
	return finding
}

func newJourneyBed(t *testing.T) *journeyBed {
	return newJourneyBedWith(t, workApprovedBox)
}

// newJourneyBedWith is the journey bed with its goal record shaped by amend.
func newJourneyBedWith(t *testing.T, amend func(*goal.GoalFile)) *journeyBed {
	c := newConnectionBedWith(t, amend)
	owners := c.connectionOwners()
	// The fixture checkout has no cmd/devgate, so the static gate a carried
	// review records is the bed's declared fixture effect, as in the carry
	// tests; the carry itself, its read commit and its attestation are real.
	owners.connection.rebaseGate = func(string) (string, error) { return "checks passed", nil }
	b := &deliveryBed{intentBed: c.intentBed, install: c.root(), owners: owners.delivery}
	realCloseOwner(t, b)
	conf := filepath.Join(c.root(), "metasystem.conf")
	confBytes, _ := os.ReadFile(conf)
	os.WriteFile(conf, append(confBytes, []byte("\nevidence.root="+t.TempDir()+"\n")...), 0o644)
	// The close scripts are part of the baseline every goal checkout starts
	// from, so the critic's own checkout closes its own chain.
	// The same native goal bytes the bed projects become a physical
	// accepted goal at the baseline: the backlog root syncs with origin, the
	// goal configuration names this machine and the approver, and a module
	// and a tiny committed testing contract are found.
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
	do := func(args ...string) (int, intentResult) {
		t.Helper()
		command, rest, ok := resolveIntentArgv(args)
		if !ok {
			t.Fatalf("no public command %q", args)
		}
		var stdout, stderr bytes.Buffer
		code := runIntentIn(command, append([]string{"--json"}, rest...), &stdout, &stderr, c.root(), owners)
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
		finishWith(install, id, commit, []any{journeyCleanFinding()})
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
		material := 0
		data, err := json.Marshal(findings)
		if err != nil {
			t.Fatal(err)
		}
		var counted []struct {
			Material bool `json:"material"`
		}
		if err := json.Unmarshal(data, &counted); err != nil {
			t.Fatal(err)
		}
		for _, finding := range counted {
			if finding.Material {
				material++
			}
		}
		c.writeJSON(filepath.Join(rounds, "return.json"),
			map[string]any{"jobId": id, "round": round, "findings": findings, "verdictMaterialCount": material, "verdict": fmt.Sprintf("%d finding(s)", len(findings)), "reviewedTree": subject.Tree})
		verdict := "VERDICT: LAND\n"
		if material > 0 {
			verdict = fmt.Sprintf("VERDICT: REVISE material=%d\n", material)
		}
		if err := os.WriteFile(filepath.Join(rounds, "return.md"), []byte(verdict), 0600); err != nil {
			t.Fatal(err)
		}
		c.writeJSON(filepath.Join(agents, "capabilities", "close.json"), map[string]any{"ok": true})
		os.MkdirAll(filepath.Join(agents, "record-locks"), 0o700)
		record := job(install, id)
		var parent any
		if round > 1 {
			parent = root
		}
		for key, value := range map[string]any{"status": "completed", "destructiveReach": "DESIGN-BEARING", "dispatchMode": "fresh",
			"sessionId": root + "-session", "parentJob": parent, "capabilitySnapshot": "artifacts/agents/capabilities/close.json",
			"endedAt": "2026-09-25T12:00:00Z", "reviewRoundLimit": 3,
			"engineBuild": "fixture-engine", "effectiveModel": "fixture-critic", "reviews": "commit:" + commit} {
			record[key] = value
		}
		c.writeJSON(filepath.Join(agents, "jobs", id+".json"), record)
		if outcome, err := dispatchcore.CritiqueRegisterAdvance(install, root, id); err != nil || outcome != "advanced" {
			t.Fatalf("register advance of %s = %q, %v", id, outcome, err)
		}
	}
	return &journeyBed{c: c, owners: owners, b: b, do: do, dispositions: dispositions, job: job, finish: finish, finishWith: finishWith, finishRound: finishRound, baseline: baseline}
}

// TestIntentConnectedJourneyRealClose (VMI-10) drives one goal's units
// through the public surface on one physical repository with a bare origin:
// build A, review unit A, real close, collection; build B and C the same way;
// fold A and review it again: B's unchanged change has its review carried
// onto its replayed commit, while C, whose change now differs, is reviewed
// as the rewritten C commit, on the same physical accepted goal. Each committed critic is closed by
// the real delegate lifecycle's close over the real owners, after the
// actual register advance of a subject-bound fake model return; nothing here
// stamps a closure. Model launches, proof, the fast gate, the claim and the
// commit token stay the connection bed's declared fixture effects; the lease
// is realCloseOwner's. It sets METASYSTEM_BIN and the connection bed's Git
// configuration environment, so it runs serially.
func TestIntentConnectedJourneyRealClose(t *testing.T) {
	j := newJourneyBed(t)
	c, owners, b, do, dispositions, job, finish := j.c, j.owners, j.b, j.do, j.dispositions, j.job, j.finish
	_ = owners
	// reviewed takes run through review unit, the fake model's completion,
	// the printed close run by the real owner, and the collecting review.
	reviewed := func(run string, extra ...string) (critic, commit string) {
		t.Helper()
		code, result := do(append([]string{"work", "review", "run:" + run}, extra...)...)
		if result.Outcome != "in-progress" || len(c.delegates) == 0 {
			t.Fatalf("review unit %s: code=%d %+v", run, code, result)
		}
		critic, commit = "crit"+strconv.Itoa(len(c.delegates)), c.delegates[len(c.delegates)-1]
		finish(c.worktree, critic, commit)
		code, result = do(append([]string{"work", "review", "run:" + run}, extra...)...)
		route := ""
		if result.Next != nil {
			route = shellCommand(result.Next.Argv)
		}
		if result.Outcome != "in-progress" || !strings.Contains(route, "work review run:"+run) || !strings.Contains(route, "--dispositions "+resultData(t, result)["template"].(string)) ||
			strings.Contains(route, "metasystem close") {
			t.Fatalf("an unclosed critic must print its public decision route: code=%d %+v", code, result)
		}
		calls := len(b.calls)
		if code, result = do("work", "finish", "j2:"+critic, "--repo", c.worktree, "--dispositions", dispositions); code != 0 || result.Outcome != intentConfirmed {
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
		if code, result = do("work", "finish", "j2:"+critic, "--repo", c.worktree, "--dispositions", dispositions); code != 0 || len(b.calls) != calls {
			t.Fatalf("a repeated close must change nothing: code=%d %+v calls=%v", code, result, b.calls[calls:])
		}
		publications := c.publications
		if code, result = do(append([]string{"work", "review", "run:" + run}, extra...)...); code != 0 || result.Outcome != intentConfirmed || c.publications != publications+1 {
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
		code, result := do(append([]string{"work", "build", c.id, unit, "--brief", brief, "--lines", "5"}, workCheck...)...)
		if code != 0 || result.Outcome != intentConfirmed {
			t.Fatalf("build %s: code=%d %+v", unit, code, result)
		}
		return resultData(t, result)["run"].(string)
	}
	// A's file has room for a later unit to change a line whose diff context
	// holds A's first line, so folding A changes that later unit's change.
	runA := build("unit-a", map[string]string{"a.txt": "first A\nl2\nl3\nl4\nl5\n"})
	criticA, commitA := reviewed(runA)
	// The default path names no critic model; a caller's --model reaches the
	// committed read's dispatch through the branch read owner.
	for _, args := range c.reads {
		if slices.Contains(args, "requested-critic") {
			t.Fatalf("the default review named a caller model: %v", args)
		}
	}
	originTip := "refs/metasystem/goals/origin/" + c.id
	published := connectionGit(t, c.root(), "rev-parse", originTip)
	connectionGit(t, c.root(), "update-ref", originTip, commitA)
	if code, result := do("work", "review", c.id, "--work", "unit-a"); code != 0 || (result.Outcome != intentConfirmed && result.Outcome != intentUnchanged) || resultData(t, result)["state"] != "already-collected" {
		t.Fatalf("review republishes the same collected read: code=%d %+v", code, result)
	}
	if got := connectionGit(t, c.root(), "rev-parse", originTip); got != published {
		t.Fatalf("republished tip=%s want=%s", got, published)
	}
	runB := build("unit-b", map[string]string{"b.txt": "the B bytes\n"})
	// Coexistence: A's read is collected and published while B is built and
	// unread. A publication the push owner has not recorded makes A's read
	// collected but unpublished; publication waits for B's owning round.
	stages := func() map[string]string {
		t.Helper()
		code, result := do("status", c.id)
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
	connectionGit(t, c.root(), "update-ref", originTip, commitA)
	if found := stages(); !strings.HasPrefix(found["unit-a"], "reviewed; its read is collected but not yet published") || found["unit-b"] != "built, ready for review" {
		t.Fatalf("collected-unpublished beside built-unread: %v", found)
	}
	if code, result := do("status", c.id, "--work", "unit-a"); code != 0 || result.Next == nil || !slices.Equal(result.Next.Argv[1:], []string{"work", "review", c.id, "--work", "unit-a"}) {
		t.Fatalf("an unpublished read continues with its review: code=%d %+v", code, result)
	}
	publications := c.publications
	if code, result := do("work", "review", c.id, "--work", "unit-a"); code != 1 || result.Outcome != intentRefused || !strings.Contains(result.Summary, "worktree belongs to run "+runB) || c.publications != publications {
		t.Fatalf("publication waits for the built unit: code=%d %+v", code, result)
	}
	if got := connectionGit(t, c.root(), "rev-parse", originTip); got != commitA {
		t.Fatalf("waiting publication moved its origin tip: %s", got)
	}
	connectionGit(t, c.root(), "update-ref", originTip, published)
	reads := len(c.reads)
	criticB, commitB := reviewed(runB, "--model", "requested-critic")
	if model := flagValue(c.reads[reads], "--model"); model != "requested-critic" || flagValue(c.reads[reads], "--unit") != commitB {
		t.Fatalf("review unit --model must reach the read owner for B's commit: %v", c.reads[reads])
	}
	if code, result := do("work", "review", "run:"+runB, "--effort", "high"); code != 2 || result.Outcome != intentRefused {
		t.Fatalf("a review effort override stays refused: code=%d %+v", code, result)
	}
	if status := c.landAdmission(); status.Prefix != 2 || status.Units[0].Commit != commitA || status.Units[1].Commit != commitB {
		t.Fatalf("both units must be read on the published branch: %+v", status)
	}

	// C changes A's file four lines down, so its diff context holds A's
	// first line: folding A leaves C's own bytes but changes C's change.
	runC := build("unit-c", map[string]string{"a.txt": "first A\nl2\nl3\nthe C line\nl5\n"})
	criticC, commitC := reviewed(runC)
	if status := c.landAdmission(); status.Prefix != 3 || status.Units[0].Commit != commitA || status.Units[1].Commit != commitB || status.Units[2].Commit != commitC {
		t.Fatalf("all three units must be read on the published branch: %+v", status)
	}

	// A clean read ends automatic corrections, so a person requests this amend.
	// A same-unit correction: fold A, review it again. B's unit survives
	// with its exact bytes and its exact change; A's old read does not carry
	// over.
	c.edits = map[string]string{"a.txt": "amended A\nl2\nl3\nthe C line\nl5\n"}
	if code, result := do("work", "revise", "run:"+runA, "--brief", c.brief("fix.md", "Amend A.\n"), "--reason", "Amend the already reviewed unit", "--by", "Wido"); code != 0 || result.Outcome != intentConfirmed {
		t.Fatalf("fold unit A: code=%d %+v", code, result)
	}
	criticA2, commitA2 := reviewed(runA)
	if commitA2 == commitA || criticA2 == criticA {
		t.Fatalf("the fold must be read as a new subject by a new critic: %s/%s", criticA2, commitA2)
	}
	if originFile("a.txt") != "amended A\nl2\nl3\nthe C line\nl5" || originFile("b.txt") != "the B bytes" {
		t.Fatalf("the published branch must carry amended A, C's line and B's exact bytes")
	}
	status := c.landAdmission()
	var subjects []string
	for _, unit := range status.Units {
		subjects = append(subjects, unit.Commit)
	}
	if len(status.Units) != 3 || status.Units[0].Commit != commitA2 || slices.Contains(subjects, commitA) {
		t.Fatalf("admission must see the replacement A first and never the old A: %+v", status)
	}
	// The correction carries B's review: B's change is unchanged, so its
	// replayed commit has a review naming it, carried from the read of the
	// old B, and no critic ever reads it.
	currentB := status.Units[1].Commit
	if status.Prefix != 2 || currentB == commitB || status.Units[1].ReadState != "read clean" {
		t.Fatalf("the replayed B must carry its review: %+v", status)
	}
	publishedTip := strings.Fields(connectionGit(t, c.root(), "ls-remote", c.origin, "refs/heads/goal/"+c.id))[0]
	if att, err := branch.ValidateAttestationAt(c.root(), publishedTip, c.endpointTip(), c.id, "unit-b", currentB); err != nil || att.Carry == nil ||
		att.Carry.FromCommit != commitB || att.Carry.ToCommit != currentB {
		t.Fatalf("B's carried review must name its replayed commit: %+v, %v", att, err)
	}
	if slices.Contains(c.delegates, currentB) {
		t.Fatalf("a carried review must not start a critic: %v", c.delegates)
	}
	// Rewriting A's prefix changed C's change, so C's replayed commit has no
	// accepted read; the current C commit gets its own committed review,
	// closed by the real owner and collected, never a copied closure.
	currentC := status.Units[2].Commit
	if currentC == commitC || status.Units[2].ReadState != "built" {
		t.Fatalf("the replayed C must await its own read: %+v", status)
	}
	review := []string{"work", "review", "--commit", currentC, "--goal", c.id, "--brief", brief, "--repo", c.worktree}
	if code, result := do(review...); result.Outcome != "in-progress" || c.delegates[len(c.delegates)-1] != currentC {
		t.Fatalf("review commit of the current C: code=%d %+v", code, result)
	}
	criticC2 := "crit" + strconv.Itoa(len(c.delegates))
	finish(c.worktree, criticC2, currentC)
	// One public review with the author's decisions: the real whole close
	// owner closes and mirrors the chain, then the read is collected and
	// published.
	if code, result := do(append(review, "--dispositions", dispositions)...); code != 0 || result.Outcome != intentConfirmed || job(c.worktree, criticC2)["chainClosed"] != true {
		t.Fatalf("decide, close and collect the current C read: code=%d %+v", code, result)
	}
	status = c.landAdmission()
	if status.Prefix != 3 || status.Units[0].Commit != commitA2 || status.Units[1].Commit != currentB || status.Units[2].Commit != currentC {
		t.Fatalf("admission must see replacement A, carried B and current C read: %+v", status)
	}
	if originFile("b.txt") != "the B bytes" {
		t.Fatal("B's bytes changed")
	}
	if len(c.delegates) != 5 || criticB == criticC2 || criticC == criticC2 {
		t.Fatalf("exactly five committed critics, none for the carried B: %v", c.delegates)
	}
}
