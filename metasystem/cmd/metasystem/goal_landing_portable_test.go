package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

// The application in this fixture has text inputs and command/JUnit checks.
// Its tracked build script only wraps the MetaSystem engine installed for the
// fixture; application acceptance never invokes a Go package gate.
type portableProofFixture struct {
	t                 *testing.T
	root              string
	engine            string
	admissionDir      string
	baselineSource    string
	buildCounter      string
	nativeCounter     string
	installedSnapshot string
	installedDigest   string
	contract          testpolicy.Contract
	proofCommand      proofBinaryFixture
}

func newPortableProofFixture(t *testing.T) *portableProofFixture {
	return newPortableProofFixtureWithSource(t, "")
}

func newPortableProofFixtureWithSource(t *testing.T, baselineSource string) *portableProofFixture {
	return newPortableProofFixtureWithSetup(t, baselineSource, nil)
}

// newPortableProofFixtureWithSetup lets setup edit the fixture's files and
// contract before the baseline commit and engine enrollment, so its edits
// belong to the trusted base rather than to a later candidate.
func newPortableProofFixtureWithSetup(t *testing.T, baselineSource string, setup func(*portableProofFixture)) *portableProofFixture {
	t.Helper()
	root := t.TempDir()
	fixture := &portableProofFixture{t: t, root: root, baselineSource: baselineSource,
		engine:       filepath.Join(root, "bin", "metasystem"),
		admissionDir: filepath.Join(t.TempDir(), "host-admission"),
		buildCounter: filepath.Join(t.TempDir(), "builds.log"), nativeCounter: filepath.Join(t.TempDir(), "native.log")}
	fixture.git("init", "-q", "-b", "main")
	fixture.git("config", "user.name", "portable-fixture")
	fixture.git("config", "user.email", "portable@example.invalid")
	fixture.git("config", "metasystem.goal.machine", "portable")
	fixture.git("config", "goal.sync-remote", "local")
	fixture.git("config", "goal.sync-branch", goal.LocalLedgerBranch)
	fixture.git("config", "metasystem.steward.landing-ref", "refs/remotes/origin/main")

	fixture.contract = testpolicy.Contract{SchemaVersion: 1,
		ProjectRisk: testpolicy.ProjectRisk{Severity: 1, Exposure: 1, Reversibility: "revert", Detection: "immediate", Recovery: "bounded"},
		Surfaces: []testpolicy.Surface{
			{ID: "app", Paths: []string{"app/**", "testing.json"}, Standard: []string{"app-a"}, Critical: []string{"app-a-observed"}},
			{ID: "docs", Paths: []string{"docs/**"}, Standard: []string{"app-a"}}},
		Groups: []testpolicy.Group{fixture.group("app-a", "a")},
		Always: testpolicy.Always{Canary: []string{"app-a"}}, Unknown: []string{"app-a"}, Cadence: []string{"app-a"}}
	fixture.writeContract()
	fixture.write("app/a.txt", "green a\n", 0o644)
	fixture.write("app/shared-b.txt", "shared b v1\n", 0o644)
	fixture.write("metasystem.conf", "metasystem.runtimes=fake\ntesting.contract=testing.json\n", 0o644)
	fixture.proofCommand = pinProofBinaryFixture(t, root)
	fixture.write(".gitignore", "artifacts/\nbin/\nreports-*/\nmetasystem.conf.local\n", 0o644)
	fixture.write("scripts/check.sh", portableCommandCheck, 0o755)
	buildScript := fmt.Sprintf(portableCandidateBuild, strconv.Quote(fixture.buildCounter), strconv.Quote(fixture.engine))
	fixture.write("scripts/agents/go-build.sh", buildScript, 0o755)
	if setup != nil {
		setup(fixture)
	}
	fixture.writeGoal()
	fixture.git("add", ".")
	fixture.git("commit", "-qm", "portable command application")
	base := fixture.git("rev-parse", "HEAD")
	fixture.git("update-ref", "refs/remotes/origin/main", base)
	fixture.git("update-ref", goal.LocalLedgerBranch, base)
	fixture.git("update-ref", goal.AcceptedRef, base)
	engine := &batchE2EEngine{path: fixture.engine}
	if baselineSource != "" {
		if got := fixture.gitSource("rev-parse", "HEAD"); got != "9498700a9e246f09395213f8539d22e0ea196731" {
			t.Fatalf("baseline source is %s, want frozen 9498700a9 source", got)
		}
		linker := "-X github.com/widoriezebos/agentic-tools/metasystem/internal/supervise.BuildStamp=" + base
		command := exec.Command("go", "build", "-buildvcs=false", "-ldflags", linker, "-o", fixture.engine, "./cmd/metasystem")
		command.Dir = baselineSource
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("build frozen baseline policy engine: %v: %s", err, output)
		}
		engine.commit = base
	}
	batchFixture := &batchE2EFixture{t: t, landing: root, now: time.Now().UTC()}
	batchFixture.enrollPolicyEngine(base, engine)
	if baselineSource != "" {
		pinned, err := steward.OpenEnrolledBinary(root)
		if err != nil {
			t.Fatal(err)
		}
		stamp := pinned.BuildStamp()
		if err := pinned.Close(); err != nil {
			t.Fatal(err)
		}
		if stamp != base {
			t.Fatalf("frozen baseline engine stamp %s does not bind app base %s", stamp, base)
		}
	}
	batchFixture.announce(root, "portable-lineage")
	holder, err := lease.RequireHolder(root, int64(os.Getpid()), nil)
	if err != nil || !holder.Holder {
		t.Fatalf("fixture checkout has no active lease holder: %+v %v", holder, err)
	}
	return fixture
}

func (fixture *portableProofFixture) writeGoal() {
	now := time.Now().UTC().Truncate(time.Second)
	opened, claimed, approved := now.Add(-10*time.Minute).Format(time.RFC3339), now.Add(-5*time.Minute).Format(time.RFC3339), now.Add(-4*time.Minute).Format(time.RFC3339)
	risk := &goal.RiskRecord{Severity: 1, Novelty: 1, Exposure: 1, Accumulation: 1, Basis: "The application has two small command checks."}
	budget := &goal.Budget{ElapsedLimit: "4h", AttemptLimit: 8, ReservedJobMinutesLimit: 1000, ActiveJobLimit: 2, ReviewRoundLimit: 2}
	root := &goal.RootRecord{Identity: "01ARZ3NDEKTSV4RRFFQ69G5FAV", FormatVersion: "1", SyncMode: goal.SyncLocal, Revision: 1}
	fixture.writeBytes("plans/goals/backlog.md", goal.RenderRoot(root), 0o644)
	for _, entry := range []struct{ id, prefix string }{{"portable", "B"}, {"goal-a", "C"}, {"goal-b", "D"}, {"goal-c", "E"}} {
		intent := "Prove the portable command application for " + entry.id + "."
		openOpid := goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5F"+entry.prefix+"1", "human", "terminal")
		claimOpid := goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5F"+entry.prefix+"2", "portable", "portable-lineage")
		approval := goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5F"+entry.prefix+"3", "human", "terminal")
		file := &goal.GoalFile{Id: entry.id, State: goal.StateClaimed, Tier: 1, Risk: risk, Intent: intent, Origin: goal.OriginMain,
			NextStep: "Run application acceptance.", OpenedAt: opened, Revision: 3, Budget: budget,
			Approved: &goal.ApprovalRecord{By: "human:portable", At: approved, Revision: 3, Opid: approval, Authority: goal.ApprovalAuthorityProven,
				Digest: goal.ApprovalDigest(intent, 1, *budget, risk)},
			Claimed:        &goal.ClaimRecord{Machine: "portable", Lineage: "portable-lineage", At: claimed, Revision: 2, AccountingRevision: 2},
			StopCapability: &goal.StopCapability{Generation: 2, Revision: 2, Machine: "portable", ClaimEpoch: 1},
			History: []goal.HistoryLine{
				{At: opened, Opid: openOpid, Verb: "open", Actor: "human:portable", Keep: -1},
				{At: claimed, Opid: claimOpid, Verb: "claim", Actor: "portable+portable-lineage", Keep: -1},
				{At: approved, Opid: approval, Verb: "approve", Actor: "human:portable", Keep: -1}}}
		fixture.writeBytes("plans/goals/"+entry.id+".md", goal.RenderFile(file), 0o644)
	}
}

func (fixture *portableProofFixture) group(id, input string) testpolicy.Group {
	report := "reports-" + input
	return testpolicy.Group{ID: id, Kind: "unit", Adapter: "command", CWD: ".",
		Inputs: []string{"app/" + input + ".txt", "scripts/check.sh"}, Outputs: []string{report},
		Tools:       []testpolicy.Tool{{ID: "shell", Executable: "sh", VersionArgs: []string{"-c", "printf portable-shell"}}},
		Obligations: []string{id + "-observed"}, Platforms: []string{"any"}, TargetMS: 1000,
		Env:     map[string]string{"PORTABLE_NATIVE_COUNTER": fixture.nativeCounter},
		Argv:    []string{"sh", "scripts/check.sh", input, "app/" + input + ".txt", report},
		Reports: []string{report}, Format: "junit-xml",
		ExpectedTests: []testpolicy.ExpectedTest{{Report: report + "/result.xml", Classname: "portable", Name: input}}}
}

func (fixture *portableProofFixture) writeContract() {
	fixture.t.Helper()
	if err := fixture.contract.Validate(); err != nil {
		fixture.t.Fatal(err)
	}
	encoded, err := json.Marshal(fixture.contract)
	if err != nil {
		fixture.t.Fatal(err)
	}
	var document map[string]any
	if err := json.Unmarshal(encoded, &document); err != nil {
		fixture.t.Fatal(err)
	}
	if fixture.baselineSource == "" {
		document["schemaVersion"] = 2
		for _, value := range document["groups"].([]any) {
			group := value.(map[string]any)
			if group["phase"] == nil || group["phase"] == "" {
				group["phase"] = "acceptance"
			}
			if group["environmentMode"] == nil || group["environmentMode"] == "" {
				group["environmentMode"] = "inherit"
			}
		}
	}
	data, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		fixture.t.Fatal(err)
	}
	fixture.writeBytes("testing.json", append(data, '\n'), 0o644)
}

func (fixture *portableProofFixture) gitSource(args ...string) string {
	fixture.t.Helper()
	command := exec.Command("git", append([]string{"-C", fixture.baselineSource}, args...)...)
	output, err := command.CombinedOutput()
	if err != nil {
		fixture.t.Fatalf("baseline git %s: %v: %s", strings.Join(args, " "), err, output)
	}
	return strings.TrimSpace(string(output))
}

func (fixture *portableProofFixture) write(path, body string, mode os.FileMode) {
	fixture.writeBytes(path, []byte(body), mode)
}

func (fixture *portableProofFixture) writeBytes(path string, body []byte, mode os.FileMode) {
	fixture.t.Helper()
	absolute := filepath.Join(fixture.root, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(absolute), 0o755); err != nil {
		fixture.t.Fatal(err)
	}
	if err := testexec.WriteFile(absolute, body, mode); err != nil {
		fixture.t.Fatal(err)
	}
}

func (fixture *portableProofFixture) git(args ...string) string {
	fixture.t.Helper()
	command := exec.Command("git", append([]string{"-C", fixture.root}, args...)...)
	output, err := command.CombinedOutput()
	if err != nil {
		fixture.t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, output)
	}
	return strings.TrimSpace(string(output))
}

func (fixture *portableProofFixture) gitBytes(args ...string) []byte {
	fixture.t.Helper()
	command := exec.Command("git", append([]string{"-C", fixture.root}, args...)...)
	output, err := command.CombinedOutput()
	if err != nil {
		fixture.t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, output)
	}
	return output
}

func portableGitAt(t *testing.T, root string, args ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", root}, args...)...)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, output)
	}
	return strings.TrimSpace(string(output))
}

func (fixture *portableProofFixture) commit(message string) string {
	fixture.git("add", "-A")
	fixture.git("commit", "-qm", message)
	return fixture.git("rev-parse", "HEAD^{tree}")
}

func (fixture *portableProofFixture) command(args ...string) (int, string) {
	fixture.t.Helper()
	command := fixture.proofCommand.command(fixture.commandEnvironment(), fixture.engine, args...)
	command.Dir = fixture.root
	output, err := command.CombinedOutput()
	if err == nil {
		return 0, string(output)
	}
	if exit, ok := err.(*exec.ExitError); ok {
		return exit.ExitCode(), string(output)
	}
	fixture.t.Fatalf("run metasystem %s: %v: %s", strings.Join(args, " "), err, output)
	return 1, ""
}

func (fixture *portableProofFixture) commandEnvironment() []string {
	return append(os.Environ(), "METASYSTEM_OWNER_LINEAGE=portable-lineage",
		"METASYSTEM_PROOF_ADMISSION_TEST_DIR="+fixture.admissionDir,
		"METASYSTEM_PROOF_ADMISSION_FIXTURE_ROOT="+fixture.root)
}

func (fixture *portableProofFixture) requireCommand(args ...string) string {
	fixture.t.Helper()
	status, output := fixture.command(args...)
	if status != 0 {
		fixture.t.Fatalf("metasystem %s exited %d: %s", strings.Join(args, " "), status, output)
	}
	return output
}

func (fixture *portableProofFixture) requireReusableRun(args ...string) string {
	fixture.t.Helper()
	status, output := fixture.command(args...)
	if status != proofrun.ExitReusableSuccess {
		fixture.t.Fatalf("metasystem %s exited %d, want reusable success %d: %s", strings.Join(args, " "), status, proofrun.ExitReusableSuccess, output)
	}
	return output
}

func (fixture *portableProofFixture) counts() (builds int, native map[string]int) {
	fixture.t.Helper()
	native = map[string]int{}
	if data, err := os.ReadFile(fixture.buildCounter); err == nil {
		builds = len(strings.Fields(string(data)))
	} else if !os.IsNotExist(err) {
		fixture.t.Fatal(err)
	}
	if data, err := os.ReadFile(fixture.nativeCounter); err == nil {
		for _, id := range strings.Fields(string(data)) {
			native[id]++
		}
	} else if !os.IsNotExist(err) {
		fixture.t.Fatal(err)
	}
	return builds, native
}

func (fixture *portableProofFixture) observe(scenario, tree string, started time.Time) {
	fixture.t.Helper()
	builds, native := fixture.counts()
	observation := struct {
		Scenario       string         `json:"scenario"`
		Tree           string         `json:"tree"`
		WallDurationMS int64          `json:"wallDurationMs"`
		Builds         int            `json:"builds"`
		Native         map[string]int `json:"native"`
	}{scenario, tree, time.Since(started).Milliseconds(), builds, native}
	encoded, err := json.Marshal(observation)
	if err != nil {
		fixture.t.Fatal(err)
	}
	fixture.t.Logf("PORTABLE_OBSERVATION %s", encoded)
	if evidenceRoot := os.Getenv("GOAL_LANDING_EVIDENCE_DIR"); evidenceRoot != "" {
		if err := os.MkdirAll(evidenceRoot, 0o700); err != nil {
			fixture.t.Fatal(err)
		}
		if err := testexec.WriteFile(filepath.Join(evidenceRoot, scenario+".observation.json"), append(encoded, '\n'), 0o600); err != nil {
			fixture.t.Fatal(err)
		}
		controlRoot, err := canonicalProofRoot(fixture.root)
		if err != nil {
			fixture.t.Fatal(err)
		}
		attempts, err := proofrun.ReadAttempts(controlRoot)
		if err != nil {
			fixture.t.Fatalf("retain raw %s attempt: %v", scenario, err)
		}
		var latest *proofrun.Attempt
		for index := range attempts {
			if attempts[index].TestResult == nil || attempts[index].TestResult.CandidateTree != tree {
				continue
			}
			if latest == nil || attempts[index].StartedAt > latest.StartedAt {
				latest = &attempts[index]
			}
		}
		if latest != nil {
			path, err := proofrun.AttemptPath(controlRoot, latest.AttemptID)
			if err != nil {
				fixture.t.Fatal(err)
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				fixture.t.Fatal(err)
			}
			if err := testexec.WriteFile(filepath.Join(evidenceRoot, scenario+".attempt.json"), raw, 0o600); err != nil {
				fixture.t.Fatal(err)
			}
		}
	}
}

// The following v1-compatible changes are run on both source cohorts. They
// are command/JUnit observations, not batch receipts or schema-2 claims.
func (fixture *portableProofFixture) runMatchedSupplementaryCohort(args []string, label string) {
	fixture.t.Helper()
	args = append([]string(nil), args...)
	run := func(tree, scenario string) {
		args[5] = tree
		started := time.Now()
		fixture.requireCommand(append([]string{"test", "plan"}, args...)...)
		status, output := fixture.command(append([]string{"test", "run"}, args...)...)
		if status != 0 && status != proofrun.ExitReusableSuccess {
			fixture.t.Fatalf("%s command proof exited %d: %s", scenario, status, output)
		}
		fixture.requireCommand(append([]string{"test", "verify"}, args...)...)
		fixture.observe(label+"-"+scenario, tree, started)
	}
	fixture.write("docs/note.md", "portable documentation only\n", 0o644)
	run(fixture.commit("add documentation outside command inputs"), "docs-only")
	for index := range fixture.contract.Groups {
		if fixture.contract.Groups[index].ID == "app-a" || fixture.contract.Groups[index].ID == "app-b" {
			fixture.contract.Groups[index].Inputs = append(fixture.contract.Groups[index].Inputs, "app/shared-b.txt")
		}
	}
	fixture.writeContract()
	run(fixture.commit("declare shared command input"), "shared-declared")
	_, before := fixture.counts()
	fixture.write("app/shared-b.txt", "shared input changed\n", 0o644)
	run(fixture.commit("mutate shared command input"), "shared-mutated")
	_, after := fixture.counts()
	if after["a"] <= before["a"] || after["b"] <= before["b"] {
		fixture.t.Fatalf("shared input mutation failed to rerun both consuming native checks: before=%v after=%v", before, after)
	}
}

func TestCommandApplicationPublicProofReuse(t *testing.T) {
	f := newPortableFileProof(t)
	run := func(changed ...string) proofrun.TestResult {
		t.Helper()
		tree, files := f.snapshot()
		contract := f.loadedContract(files)
		plan := f.plan(contract, changed...)
		request := f.request(tree, files, plan)
		request.CandidateEngineBuildIdentity = f.engineIdentity(tree, request.Environment)
		result, _ := f.execute(request, true)
		verified, err := f.verify(request, files, result, time.Now().UTC())
		if err != nil || !verified.Delivery.Sufficient {
			t.Fatalf("retained proof verification sufficient=%t err=%v groups=%+v", verified.Delivery.Sufficient, err, verified.Groups)
		}
		return result
	}
	cold := run("app/a.txt")
	if groups := portableGroups(cold); groups["app-a"].Status != "passed" || !groups["app-a"].NativeLaunched {
		t.Fatalf("cold A did not run natively: %+v", groups)
	}
	requirePortableCounts(t, f.counts(), map[string]int{"a": 1})
	warm := run("app/a.txt")
	if groups := portableGroups(warm); groups["app-a"].Status != "reused" || groups["app-a"].ReuseAttempt == "" {
		t.Fatalf("unchanged A did not reuse its native producer: %+v", groups)
	}
	requirePortableCounts(t, f.counts(), map[string]int{"a": 1})
	f.contract.Groups = append(f.contract.Groups, f.group("app-b", "b"))
	f.contract.Surfaces[0].Standard = append(f.contract.Surfaces[0].Standard, "app-b")
	f.contract.Always.Standard = []string{"app-b"}
	f.contract.Cadence = append(f.contract.Cadence, "app-b")
	f.put("app/b.txt", "green b\n", 0o644)
	f.writeContract()
	added := run("app/b.txt")
	if groups := portableGroups(added); groups["app-a"].Status != "reused" || groups["app-b"].Status != "passed" || !groups["app-b"].NativeLaunched {
		t.Fatalf("B addition lost A reuse or B execution: %+v", groups)
	}
	requirePortableCounts(t, f.counts(), map[string]int{"a": 1, "b": 1})
	f.put("app/a.txt", "green a changed\n", 0o644)
	mutated := run("app/a.txt")
	if groups := portableGroups(mutated); groups["app-a"].Status != "passed" || groups["app-b"].Status != "reused" {
		t.Fatalf("A mutation changed the wrong native work: %+v", groups)
	}
	requirePortableCounts(t, f.counts(), map[string]int{"a": 2, "b": 1})
	f.put("docs/note.md", "portable documentation only\n", 0o644)
	documented := run("docs/note.md")
	if groups := portableGroups(documented); groups["app-a"].Status != "reused" || groups["app-b"].Status != "reused" {
		t.Fatalf("docs-only change lost reuse: %+v", groups)
	}
	requirePortableCounts(t, f.counts(), map[string]int{"a": 2, "b": 1})
	for index := range f.contract.Groups {
		if f.contract.Groups[index].ID == "app-a" || f.contract.Groups[index].ID == "app-b" {
			f.contract.Groups[index].Inputs = append(f.contract.Groups[index].Inputs, "app/shared-b.txt")
		}
	}
	f.writeContract()
	_ = run("testing.json")
	before := f.counts()
	f.put("app/shared-b.txt", "shared input changed\n", 0o644)
	shared := run("app/shared-b.txt")
	if groups := portableGroups(shared); groups["app-a"].Status != "passed" || groups["app-b"].Status != "passed" {
		t.Fatalf("shared input did not rerun both consumers: %+v", groups)
	}
	after := f.counts()
	if after["a"] != before["a"]+1 || after["b"] != before["b"]+1 {
		t.Fatalf("shared input native counts before=%v after=%v", before, after)
	}
}

func runPortablePublicProofReuse(fixture *portableProofFixture) {
	t := fixture.t
	base := fixture.git("rev-parse", "HEAD^{tree}")
	args := []string{"--root", fixture.root, "--goal", "portable", "--tree", base, "--mode", "auto", "--purpose", "delivery"}
	started := time.Now()
	fixture.requireCommand(append([]string{"test", "plan"}, args...)...)
	fixture.requireCommand(append([]string{"test", "run"}, args...)...)
	fixture.requireCommand(append([]string{"test", "verify"}, args...)...)
	fixture.observe(fixture.portableScenario("fixed-a-cold"), base, started)
	builds, native := fixture.counts()
	if native["a"] != 1 || builds == 0 || (fixture.baselineSource != "" && builds != 1) {
		t.Fatalf("cold proof counts: builds=%d native=%v", builds, native)
	}
	started = time.Now()
	fixture.requireReusableRun(append([]string{"test", "run"}, args...)...)
	fixture.requireCommand(append([]string{"test", "verify"}, args...)...)
	fixture.observe(fixture.portableScenario("fixed-a-warm"), base, started)
	warmBuilds, warmNative := fixture.counts()
	if (fixture.baselineSource == "" && warmBuilds != builds) || (fixture.baselineSource != "" && warmBuilds < builds) || !reflect.DeepEqual(warmNative, native) {
		t.Errorf("unchanged proof launched native work: cold=(%d,%v) warm=(%d,%v)", builds, native, warmBuilds, warmNative)
	}

	fixture.contract.Groups = append(fixture.contract.Groups, fixture.group("app-b", "b"))
	fixture.contract.Surfaces[0].Standard = append(fixture.contract.Surfaces[0].Standard, "app-b")
	fixture.contract.Always.Standard = []string{"app-b"}
	fixture.contract.Cadence = append(fixture.contract.Cadence, "app-b")
	fixture.write("app/b.txt", "green b\n", 0o644)
	fixture.writeContract()
	addedTree := fixture.commit("add independent command group")
	args[5] = addedTree
	started = time.Now()
	fixture.requireCommand(append([]string{"test", "plan"}, args...)...)
	fixture.requireCommand(append([]string{"test", "run"}, args...)...)
	fixture.requireCommand(append([]string{"test", "verify"}, args...)...)
	fixture.observe(fixture.portableScenario("a-plus-b"), addedTree, started)
	_, afterAddition := fixture.counts()
	if (fixture.baselineSource == "" && (afterAddition["a"] != 1 || afterAddition["b"] != 1)) || (fixture.baselineSource != "" && afterAddition["b"] == 0) {
		t.Errorf("unrelated policy addition failed to reuse A or run B: %v", afterAddition)
	}

	fixture.write("app/a.txt", "green a changed\n", 0o644)
	mutatedTree := fixture.commit("change A input")
	args[5] = mutatedTree
	started = time.Now()
	fixture.requireCommand(append([]string{"test", "run"}, args...)...)
	fixture.requireCommand(append([]string{"test", "verify"}, args...)...)
	fixture.observe(fixture.portableScenario("a-input-changed"), mutatedTree, started)
	_, afterMutation := fixture.counts()
	if (fixture.baselineSource == "" && (afterMutation["a"] != 2 || afterMutation["b"] != 1)) || (fixture.baselineSource != "" && afterMutation["a"] <= afterAddition["a"]) {
		t.Errorf("input mutation did not rerun only A: %v", afterMutation)
	}

	controlRoot, err := canonicalProofRoot(fixture.root)
	if err != nil {
		t.Fatal(err)
	}
	attempts, err := proofrun.ReadAttempts(controlRoot)
	if err != nil || len(attempts) == 0 {
		t.Fatalf("public runs retained no attempt: count=%d err=%v", len(attempts), err)
	}
	fixture.runMatchedSupplementaryCohort(args, fixture.portableCohort())
}

func TestCommandApplicationPrerequisiteAndIndependentFailures(t *testing.T) {
	f := newPortableFileProof(t)
	f.contract.SchemaVersion = testpolicy.ExecutionContractSchemaVersion
	harness := f.group("harness", "h")
	harness.Phase = "admission"
	dependent := f.contract.Groups[0]
	dependent.Requires = []string{"harness"}
	independentRed := f.group("app-b", "b")
	independentPass := f.group("app-c", "c")
	f.contract.Groups = []testpolicy.Group{harness, dependent, independentRed, independentPass}
	for index := range f.contract.Groups {
		if f.contract.Groups[index].Phase == "" {
			f.contract.Groups[index].Phase = "acceptance"
		}
		f.contract.Groups[index].EnvironmentMode = "inherit"
	}
	f.contract.Always.Canary = []string{"harness", "app-a", "app-b", "app-c"}
	f.contract.Unknown = []string{"harness", "app-a", "app-b", "app-c"}
	f.contract.Cadence = []string{"harness", "app-a", "app-b", "app-c"}
	f.put("app/h.txt", "red harness\n", 0o644)
	f.put("app/b.txt", "red independent\n", 0o644)
	f.put("app/c.txt", "green independent\n", 0o644)
	f.writeContract()
	run := func(changed string, green bool) map[string]proofrun.GroupResult {
		t.Helper()
		tree, files := f.snapshot()
		contract := f.loadedContract(files)
		plan := f.plan(contract, changed)
		request := f.request(tree, files, plan)
		if green {
			request.CandidateEngineBuildIdentity = f.engineIdentity(tree, request.Environment)
		}
		result, _ := f.execute(request, green)
		if green {
			verified, err := f.verify(request, files, result, time.Now().UTC())
			if err != nil || !verified.Delivery.Sufficient {
				t.Fatalf("repaired retained proof verification sufficient=%t err=%v groups=%+v", verified.Delivery.Sufficient, err, verified.Groups)
			}
		}
		if err := proofrun.ValidateTestResult(result); err != nil {
			t.Fatal(err)
		}
		groups := portableGroups(result)
		if len(groups) != 4 || result.Delivery.Sufficient != green {
			t.Fatalf("selected groups=%+v delivery=%+v want green=%t", groups, result.Delivery, green)
		}
		return groups
	}
	first := run("app/h.txt", false)
	if first["harness"].Status != "failed" || !first["harness"].NativeLaunched ||
		first["app-b"].Status != "failed" || !first["app-b"].NativeLaunched ||
		first["app-c"].Status != "passed" || !first["app-c"].NativeLaunched ||
		first["app-a"].Status != "blocked" || first["app-a"].NativeLaunched ||
		!reflect.DeepEqual(first["app-a"].BlockingGroups, []string{"harness"}) {
		t.Fatalf("first pass did not collect independent failures and block only dependent: %+v", first)
	}
	requirePortableCounts(t, f.counts(), map[string]int{"h": 1, "b": 1, "c": 1})
	f.put("app/h.txt", "green harness repaired\n", 0o644)
	f.put("app/b.txt", "green independent repaired\n", 0o644)
	second := run("app/h.txt", true)
	if second["harness"].Status != "passed" || second["app-a"].Status != "passed" ||
		second["app-b"].Status != "passed" || second["app-c"].Status != "reused" ||
		second["app-c"].ReuseAttempt == "" {
		t.Fatalf("repair lost independent pass or dependent execution: %+v", second)
	}
	requirePortableCounts(t, f.counts(), map[string]int{"h": 2, "b": 2, "c": 1, "a": 1})
	f.put("app/a.txt", "green dependent changed\n", 0o644)
	third := run("app/a.txt", true)
	for _, id := range []string{"harness", "app-b", "app-c"} {
		if third[id].Status != "reused" || third[id].ReuseAttempt == "" {
			t.Fatalf("dependent change lost %s reuse: %+v", id, third)
		}
	}
	if third["app-a"].Status != "passed" || !third["app-a"].NativeLaunched {
		t.Fatalf("dependent change did not run A: %+v", third["app-a"])
	}
	requirePortableCounts(t, f.counts(), map[string]int{"h": 2, "b": 2, "c": 1, "a": 2})
}

func (fixture *portableProofFixture) portableScenario(label string) string {
	if fixture.baselineSource != "" {
		return "legacy-" + label
	}
	return label
}
func (fixture *portableProofFixture) portableCohort() string {
	if fixture.baselineSource != "" {
		return "legacy"
	}
	return "integration"
}

func TestCommandApplicationLegacyBaselineCohort(t *testing.T) {
	source := os.Getenv("GOAL_LANDING_BASELINE_SOURCE")
	if source == "" {
		t.Skip("set GOAL_LANDING_BASELINE_SOURCE to the frozen 9498700a9 metasystem source")
	}
	t.Run("current", func(t *testing.T) { runPortablePublicProofReuse(newPortableProofFixture(t)) })
	t.Run("frozen", func(t *testing.T) { runPortablePublicProofReuse(newPortableProofFixtureWithSource(t, source)) })
}

func TestCommandApplicationThreePrefixReceiptConsumer(t *testing.T) {
	batchE2EProcessEnvironment.Lock()
	t.Cleanup(batchE2EProcessEnvironment.Unlock)
	fixture := newPortableProofFixture(t)
	previousPlannerExecutable := batchTreePlanExecutable
	batchTreePlanExecutable = func() (string, error) { return fixture.engine, nil }
	t.Cleanup(func() { batchTreePlanExecutable = previousPlannerExecutable })
	baseTree := fixture.git("rev-parse", "HEAD^{tree}")
	baseCommit := fixture.git("rev-parse", "HEAD")
	origin := filepath.Join(t.TempDir(), "origin.git")
	if output, err := exec.Command("git", "init", "-q", "--bare", origin).CombinedOutput(); err != nil {
		t.Fatalf("create disposable origin: %v: %s", err, output)
	}
	fixture.git("remote", "add", "origin", origin)
	fixture.git("push", "-q", "origin", baseCommit+":refs/heads/main")
	fixture.write("app/a.txt", "green a first\n", 0o644)
	firstTree := fixture.commit("first portable unit")
	firstCommit := fixture.git("rev-parse", "HEAD")
	groupB := fixture.group("app-b", "b")
	groupB.Inputs = append(groupB.Inputs, "app/shared-b.txt")
	fixture.contract.Groups = append(fixture.contract.Groups, groupB)
	fixture.contract.Surfaces[0].Standard = append(fixture.contract.Surfaces[0].Standard, "app-b")
	fixture.contract.Cadence = append(fixture.contract.Cadence, "app-b")
	fixture.write("app/b.txt", "green b\n", 0o644)
	fixture.writeContract()
	secondTree := fixture.commit("second portable unit")
	secondCommit := fixture.git("rev-parse", "HEAD")
	fixture.contract.Groups = append(fixture.contract.Groups, fixture.group("app-c", "c"))
	fixture.contract.Surfaces[0].Standard = append(fixture.contract.Surfaces[0].Standard, "app-c")
	fixture.contract.Cadence = append(fixture.contract.Cadence, "app-c")
	fixture.write("app/c.txt", "green c\n", 0o644)
	fixture.writeContract()
	thirdTree := fixture.commit("third portable unit")
	thirdCommit := fixture.git("rev-parse", "HEAD")
	for _, member := range []struct{ chain, from, to string }{
		{"portable-goal-a", baseCommit, firstCommit},
		{"portable-goal-b", firstCommit, secondCommit},
		{"portable-goal-c", secondCommit, thirdCommit},
	} {
		fixture.writeBytes("artifacts/agents/landing-batches/chains/"+member.chain+"/diff.patch",
			fixture.gitBytes("diff", "--binary", member.from, member.to), 0o644)
	}
	trees := []string{firstTree, secondTree, thirdTree}
	units := []batch.Unit{}
	seal := map[string]batch.Claim{}
	for index, id := range []string{"goal-a", "goal-b", "goal-c"} {
		claim := batch.Claim{Machine: "portable", Lineage: "portable-lineage", Epoch: 1, Revision: 2, AccountingRevision: 2}
		selected := []string{"app-a", "app-b", "app-c"}[index]
		units = append(units, batch.Unit{GoalID: id, Chain: "portable-" + id, Claim: claim, State: batch.UnitJoined, SelectedGroups: []string{selected}})
		seal[id] = claim
	}
	tipResultPath := filepath.Join(t.TempDir(), "tip-result.json")
	tipArgs := []string{"test", "run", "--root", fixture.root, "--goal", "goal-c", "--tree", thirdTree,
		"--mode", "auto", "--purpose", "delivery", "--batch-prefix", "--batch-requirements", batchRequirementsArgument([]string{"app-a", "app-b", "app-c"}), "--result", tipResultPath}
	started := time.Now()
	fixture.requireCommand(tipArgs...)
	tipOutput, err := os.ReadFile(tipResultPath)
	if err != nil {
		t.Fatal(err)
	}
	var tip proofrun.TestResult
	if err := json.Unmarshal(tipOutput, &tip); err != nil || !tip.Delivery.Sufficient {
		t.Fatalf("real native tip proof is incomplete: %+v err=%v", tip.Delivery, err)
	}
	reportOutput := fixture.requireCommand("test", "report", "--result", tipResultPath, "--expensive-ms", "1")
	var cost proofrun.TestCostSummary
	if err := json.Unmarshal([]byte(reportOutput), &cost); err != nil || cost.NativeTestGroups != 3 || cost.LaunchCounts.Test != 3 {
		t.Fatalf("read-only public cost report=%+v err=%v", cost, err)
	}
	fixture.observe("three-prefix-tip", thirdTree, started)
	inputManifests := map[string][]string{}
	for _, group := range tip.Groups {
		inputManifests[group.ID] = append([]string(nil), group.InputManifest...)
	}
	record := batch.Record{Schema: 1, BatchID: "01j5x00000000000000000ba01", State: batch.StateLanding, BaseTree: baseTree, TipTree: thirdTree,
		PrefixTrees: trees, Units: units, Seal: seal, SelectedGroups: []string{"app-a", "app-b", "app-c"},
		Proof: &batch.Proof{Status: "green", Tree: thirdTree, AttemptID: tip.AttemptID, BaseCommit: baseCommit, BaseTree: baseTree,
			SelectedGroups: []string{"app-a", "app-b", "app-c"}, InputManifests: inputManifests}}
	store := batch.NewStore(fixture.root, nil)
	if err := store.Create(record); err != nil {
		t.Fatal(err)
	}
	seams := batch.PrefixReceiptSeams{
		Plan: func(prefix []batch.Unit, tree string) (batch.PrefixDecision, error) {
			return productionPrefixDecision(fixture.root, prefix, tree)
		},
		ExecuteDecision: func(goalID, tree string, decision batch.PrefixDecision) (batch.PrefixRunResult, error) {
			var unit batch.Unit
			for _, candidate := range units {
				if candidate.GoalID == goalID {
					unit = candidate
					break
				}
			}
			return fixture.runPrefixCommand(unit, tree, decision)
		},
		Verify: func(unit batch.Unit, tree string, decision batch.PrefixDecision) error {
			return batchVerifyPrefixEvidence(fixture.root, unit, tree, decision)
		},
	}
	started = time.Now()
	if err := batch.ComposePrefixReceipts(store, record.BatchID, "portable", time.Now().UTC(), seams); err != nil {
		t.Fatalf("compose real prefix receipts: %v", err)
	}
	fixture.observe("three-prefix-receipts", thirdTree, started)
	retained, err := store.Load(record.BatchID)
	if err != nil || len(retained.Receipts) != 2 || retained.Receipts["goal-a"].Reused["app-a"] == "" ||
		retained.Receipts["goal-b"].Reused["app-b"] == "" || len(retained.Receipts["goal-a"].Executed) != 0 ||
		len(retained.Receipts["goal-b"].Executed) != 0 {
		t.Fatalf("prefix receipts=%+v err=%v", retained.Receipts, err)
	}
	if err := verifyBatchSeries(fixture.root, retained, trees); err != nil {
		t.Fatalf("final three-prefix consumer refused real evidence: %v", err)
	}
	_, native := fixture.counts()
	if native["a"] != 1 || native["b"] != 1 || native["c"] != 1 {
		t.Fatalf("unchanged prefix evidence was reexecuted: %v", native)
	}
	buildsBeforeRestart, nativeBeforeRestart := fixture.counts()
	restartedStore := batch.NewStore(fixture.root, nil)
	restarted, err := restartedStore.Load(record.BatchID)
	if err != nil {
		t.Fatal(err)
	}
	// A fresh CLI process must recover the exact retained green tip before
	// the reconstructed receipt consumer examines its own durable state.
	fixture.requireReusableRun(tipArgs...)
	restartedSeams := batch.PrefixReceiptSeams{
		Plan: func(prefix []batch.Unit, tree string) (batch.PrefixDecision, error) {
			return productionPrefixDecision(fixture.root, prefix, tree)
		},
		ExecuteDecision: func(goalID, tree string, decision batch.PrefixDecision) (batch.PrefixRunResult, error) {
			for _, unit := range restarted.Units {
				if unit.GoalID == goalID {
					return fixture.runPrefixCommand(unit, tree, decision)
				}
			}
			return batch.PrefixRunResult{}, fmt.Errorf("unknown restarted prefix member %s", goalID)
		},
		Verify: func(unit batch.Unit, tree string, decision batch.PrefixDecision) error {
			return batchVerifyPrefixEvidence(fixture.root, unit, tree, decision)
		},
	}
	started = time.Now()
	if err := batch.ComposePrefixReceipts(restartedStore, record.BatchID, "portable-restart", time.Now().UTC(), restartedSeams); err != nil {
		t.Fatalf("restarted receipt consumer refused retained evidence: %v", err)
	}
	restarted, err = restartedStore.Load(record.BatchID)
	if err != nil {
		t.Fatal(err)
	}
	verifyErr := verifyBatchSeries(fixture.root, restarted, trees)
	if !reflect.DeepEqual(restarted.Receipts, retained.Receipts) || verifyErr != nil {
		t.Fatalf("restarted receipt consumer changed durable receipts: %+v err=%v", restarted.Receipts, verifyErr)
	}
	fixture.observe("three-prefix-restart", thirdTree, started)
	buildsAfterRestart, nativeAfterRestart := fixture.counts()
	if buildsAfterRestart != buildsBeforeRestart || !reflect.DeepEqual(nativeAfterRestart, nativeBeforeRestart) {
		t.Fatalf("restarted consumer relaunched work: before=(%d,%v) after=(%d,%v)",
			buildsBeforeRestart, nativeBeforeRestart, buildsAfterRestart, nativeAfterRestart)
	}

	// The disposable remote advances only the declared shared input consumed
	// by B. Replay the original member patches through the production batch
	// assembler, then require new proof and receipts on every moved prefix.
	movedWorktree := filepath.Join(t.TempDir(), "moved-base")
	fixture.git("worktree", "add", "--detach", movedWorktree, baseCommit)
	t.Cleanup(func() { fixture.git("worktree", "remove", "--force", movedWorktree) })
	if err := testexec.WriteFile(filepath.Join(movedWorktree, "app", "shared-b.txt"), []byte("shared b v2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	portableGitAt(t, movedWorktree, "add", "app/shared-b.txt")
	portableGitAt(t, movedWorktree, "commit", "-qm", "move shared B input on trunk")
	movedBaseCommit := portableGitAt(t, movedWorktree, "rev-parse", "HEAD")
	movedBaseTree := portableGitAt(t, movedWorktree, "rev-parse", "HEAD^{tree}")
	fixture.git("push", "-q", "origin", movedBaseCommit+":refs/heads/main")
	fetchedCommit, fetchedTree, err := fetchBatchOrigin(fixture.root)
	if err != nil || fetchedCommit != movedBaseCommit || fetchedTree != movedBaseTree {
		t.Fatalf("disposable endpoint did not move to shared input: fetched=(%s,%s) want=(%s,%s) err=%v",
			fetchedCommit, fetchedTree, movedBaseCommit, movedBaseTree, err)
	}
	changed := strings.Fields(fixture.git("diff", "--name-only", baseCommit, movedBaseCommit))
	if !batchProofInputsMoved(restarted, changed, "") {
		t.Fatalf("declared input move did not invalidate tip proof: changed=%v manifests=%v", changed, restarted.Proof.InputManifests)
	}
	movedTrees, err := batch.AssembleUnits(fixture.root, movedBaseTree, restarted.Units)
	if err != nil || len(movedTrees) != 3 || reflect.DeepEqual(movedTrees, trees) {
		t.Fatalf("production reassembly did not produce three moved prefixes: trees=%v err=%v", movedTrees, err)
	}
	if err := verifyBatchSeries(fixture.root, restarted, movedTrees); err == nil || !strings.Contains(err.Error(), "BATCH_PREFIX_DECISION_MOVED") {
		t.Fatalf("old receipts authorized moved series, or refused for wrong reason: %v", err)
	}
	started = time.Now()
	reopenedForMove, err := reopenBatchBeforeReceiptsWhenProofBaseMoved(fixture.root, restartedStore,
		record.BatchID, "portable", time.Now().UTC(), restarted)
	if err != nil || !reopenedForMove {
		t.Fatalf("production moved-trunk guard did not reopen declared input move: reopened=%t err=%v", reopenedForMove, err)
	}
	reopened, err := restartedStore.Load(record.BatchID)
	if err != nil || reopened.State != batch.StateOpen || reopened.Proof != nil || len(reopened.Receipts) != 0 ||
		reopened.BaseTree != movedBaseTree || !reflect.DeepEqual(reopened.PrefixTrees, movedTrees) {
		t.Fatalf("moved-trunk reopen retained stale evidence or wrong series: %+v err=%v", reopened, err)
	}
	fixture.observe("moved-trunk-reassembled", movedTrees[2], started)
	var tipDecision batch.PrefixDecision
	for index := range reopened.Units {
		decision, planErr := productionPrefixDecision(fixture.root, reopened.Units[:index+1], movedTrees[index])
		if planErr != nil || len(decision.Groups) == 0 {
			t.Fatalf("moved prefix %d did not replan: %+v err=%v", index, decision, planErr)
		}
		tipDecision = decision
	}
	started = time.Now()
	movedTip, err := fixture.runPrefixCommand(reopened.Units[2], movedTrees[2], tipDecision)
	if err != nil || len(movedTip.Red) != 0 || movedTip.AttemptID == "" {
		t.Fatalf("moved tip did not produce native green proof: %+v err=%v", movedTip, err)
	}
	if err := batchVerifyPrefixEvidence(fixture.root, reopened.Units[2], movedTrees[2], tipDecision); err != nil {
		t.Fatalf("moved tip evidence refused: %v", err)
	}
	fixture.observe("moved-trunk-tip", movedTrees[2], started)
	_, movedNative := fixture.counts()
	if movedNative["a"] != 1 || movedNative["b"] != 2 || movedNative["c"] != 1 {
		t.Fatalf("moved shared input did not rerun only B: %v", movedNative)
	}
	if err := restartedStore.Update(record.BatchID, func(current *batch.Record) error {
		current.Seal = seal
		current.SelectedGroups = append([]string(nil), tipDecision.Groups...)
		current.Proof = &batch.Proof{Status: "green", Tree: movedTrees[2], AttemptID: movedTip.AttemptID,
			BaseCommit: movedBaseCommit, BaseTree: movedBaseTree, SelectedGroups: append([]string(nil), tipDecision.Groups...)}
		current.Transition(batch.StateLanding, time.Now().UTC(), "portable-tip-proof", "portable", "")
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	started = time.Now()
	if err := batch.ComposePrefixReceipts(restartedStore, record.BatchID, "portable-moved", time.Now().UTC(), restartedSeams); err != nil {
		t.Fatalf("moved prefix receipt consumer: %v", err)
	}
	movedRecord, err := restartedStore.Load(record.BatchID)
	if err != nil || len(movedRecord.Receipts) != 2 {
		t.Fatalf("moved series retained no full receipts: %+v err=%v", movedRecord.Receipts, err)
	}
	if err := verifyBatchSeries(fixture.root, movedRecord, movedTrees); err != nil {
		t.Fatalf("moved final series refused: %v", err)
	}
	fixture.observe("moved-trunk-receipts", movedTrees[2], started)
	_, movedNative = fixture.counts()
	if movedNative["a"] != 1 || movedNative["b"] != 2 || movedNative["c"] != 1 {
		t.Fatalf("moved receipt consumer relaunched unaffected checks: %v", movedNative)
	}
}

func TestCommandApplicationRedEarlierPrefixBlocksReceipt(t *testing.T) {
	batchE2EProcessEnvironment.Lock()
	t.Cleanup(batchE2EProcessEnvironment.Unlock)
	fixture := newPortableProofFixture(t)
	previousPlannerExecutable := batchTreePlanExecutable
	batchTreePlanExecutable = func() (string, error) { return fixture.engine, nil }
	t.Cleanup(func() { batchTreePlanExecutable = previousPlannerExecutable })
	baseTree := fixture.git("rev-parse", "HEAD^{tree}")
	fixture.write("app/a.txt", "green a first\n", 0o644)
	firstTree := fixture.commit("first portable unit")
	fixture.contract.Groups = append(fixture.contract.Groups, fixture.group("app-b", "b"))
	fixture.contract.Surfaces[0].Standard = append(fixture.contract.Surfaces[0].Standard, "app-b")
	fixture.write("app/b.txt", "red b\n", 0o644)
	fixture.writeContract()
	redTree := fixture.commit("second unit is red")
	fixture.contract.Groups = append(fixture.contract.Groups, fixture.group("app-c", "c"))
	fixture.contract.Surfaces[0].Standard = append(fixture.contract.Surfaces[0].Standard, "app-c")
	fixture.write("app/b.txt", "green b repaired\n", 0o644)
	fixture.write("app/c.txt", "green c\n", 0o644)
	fixture.writeContract()
	tipTree := fixture.commit("third unit repairs the tip")
	claim := batch.Claim{Machine: "portable", Lineage: "portable-lineage", Epoch: 1, Revision: 2, AccountingRevision: 2}
	units := []batch.Unit{}
	seal := map[string]batch.Claim{}
	for index, id := range []string{"goal-a", "goal-b", "goal-c"} {
		units = append(units, batch.Unit{GoalID: id, Chain: "portable-" + id, Claim: claim, State: batch.UnitJoined,
			SelectedGroups: []string{[]string{"app-a", "app-b", "app-c"}[index]}})
		seal[id] = claim
	}
	tipResultPath := filepath.Join(t.TempDir(), "tip-result.json")
	started := time.Now()
	fixture.requireCommand("test", "run", "--root", fixture.root, "--goal", "goal-c", "--tree", tipTree,
		"--mode", "auto", "--purpose", "delivery", "--batch-prefix", "--batch-requirements", batchRequirementsArgument([]string{"app-a", "app-b", "app-c"}), "--result", tipResultPath)
	data, err := os.ReadFile(tipResultPath)
	if err != nil {
		t.Fatal(err)
	}
	var tip proofrun.TestResult
	if err := json.Unmarshal(data, &tip); err != nil || !tip.Delivery.Sufficient {
		t.Fatalf("repaired native tip is not green: %+v err=%v", tip.Delivery, err)
	}
	fixture.observe("red-earlier-green-tip", tipTree, started)
	record := batch.Record{Schema: 1, BatchID: "01j5x00000000000000000ba02", State: batch.StateLanding, BaseTree: baseTree, TipTree: tipTree,
		PrefixTrees: []string{firstTree, redTree, tipTree}, Units: units, Seal: seal, SelectedGroups: []string{"app-a", "app-b", "app-c"},
		Proof: &batch.Proof{Status: "green", Tree: tipTree, AttemptID: tip.AttemptID, SelectedGroups: []string{"app-a", "app-b", "app-c"}}}
	store := batch.NewStore(fixture.root, nil)
	if err := store.Create(record); err != nil {
		t.Fatal(err)
	}
	seams := batch.PrefixReceiptSeams{
		Plan: func(prefix []batch.Unit, tree string) (batch.PrefixDecision, error) {
			return productionPrefixDecision(fixture.root, prefix, tree)
		},
		ExecuteDecision: func(goalID, tree string, decision batch.PrefixDecision) (batch.PrefixRunResult, error) {
			for _, unit := range units {
				if unit.GoalID == goalID {
					return fixture.runPrefixCommand(unit, tree, decision)
				}
			}
			return batch.PrefixRunResult{}, fmt.Errorf("unknown prefix member %s", goalID)
		},
		Verify: func(unit batch.Unit, tree string, decision batch.PrefixDecision) error {
			return batchVerifyPrefixEvidence(fixture.root, unit, tree, decision)
		},
	}
	started = time.Now()
	err = batch.ComposePrefixReceipts(store, record.BatchID, "portable", time.Now().UTC(), seams)
	fixture.observe("red-earlier-prefix-refusal", redTree, started)
	var red *batch.PrefixRedError
	if !errors.As(err, &red) || red.GoalID != "goal-b" {
		t.Fatalf("green tip hid the red earlier prefix: %v", err)
	}
	retained, err := store.Load(record.BatchID)
	if err != nil || retained.State != batch.StateDiagnosing || retained.Proof == nil || retained.Proof.Status != "prefix-red" ||
		retained.Proof.PrefixGoal != "goal-b" || retained.Receipts["goal-b"].Tree != "" {
		t.Fatalf("red prefix did not stop receipt publication: record=%+v err=%v", retained, err)
	}
	controlRoot, err := canonicalProofRoot(fixture.root)
	if err != nil {
		t.Fatal(err)
	}
	attempts, err := proofrun.ReadAttempts(controlRoot)
	if err != nil {
		t.Fatal(err)
	}
	completeRed := false
	for _, attempt := range attempts {
		if attempt.TestResult == nil || attempt.TestResult.CandidateTree != redTree {
			continue
		}
		for _, group := range attempt.TestResult.Groups {
			completeRed = completeRed || group.ID == "app-b" && group.Status == "failed" && group.NativeLaunched && group.CollectionComplete
		}
	}
	if !completeRed {
		t.Fatal("red earlier prefix has no complete native JUnit failure")
	}
}

func (fixture *portableProofFixture) runPrefixCommand(unit batch.Unit, tree string, decision batch.PrefixDecision) (batch.PrefixRunResult, error) {
	fixture.t.Helper()
	detached, err := (gittree.Workspace{Dir: fixture.root}).NewDetachedWorktree(tree)
	if err != nil {
		return batch.PrefixRunResult{}, err
	}
	defer detached.Close()
	resultPath := filepath.Join(fixture.t.TempDir(), unit.GoalID+"-result.json")
	args := batchPrefixReceiptArgsWithFresh(detached.Workspace().Dir, fixture.root, unit.GoalID, tree, resultPath,
		decision.Groups, unit.Claim, decision.FreshEpisode, decision.FreshExpiresAt)
	command := fixture.proofCommand.command(fixture.commandEnvironment(), fixture.engine, args...)
	command.Dir = detached.Workspace().Dir
	output, runErr := command.CombinedOutput()
	reusedSuccess := false
	if exit, ok := runErr.(*exec.ExitError); ok && exit.ExitCode() == proofrun.ExitReusableSuccess {
		reusedSuccess, runErr = true, nil
	}
	data, readErr := os.ReadFile(resultPath)
	if readErr != nil {
		return batch.PrefixRunResult{}, fmt.Errorf("prefix command %s: %v; output=%s", unit.GoalID, readErr, output)
	}
	var result proofrun.TestResult
	if err := json.Unmarshal(data, &result); err != nil {
		return batch.PrefixRunResult{}, err
	}
	fixture.t.Logf("PREFIX_RESULT goal=%s reusable=%t attempt=%q groups=%d", unit.GoalID, reusedSuccess, result.AttemptID, len(result.Groups))
	out := batch.PrefixRunResult{AttemptID: result.AttemptID, ResultPath: resultPath, Reused: map[string]string{}}
	for _, group := range result.Groups {
		if group.NativeLaunched && !reusedSuccess {
			out.Executed = append(out.Executed, group.ID)
		}
		if group.ReuseAttempt != "" {
			out.Reused[group.ID] = group.ReuseAttempt
		}
		if group.Status != "passed" && group.Status != "reused" {
			out.Red = append(out.Red, batch.RedGroup{ID: group.ID, Status: group.Status, NotRunReason: group.NotRunReason})
		}
	}
	if len(out.Red) != 0 {
		return out, nil
	}
	if runErr != nil {
		return out, fmt.Errorf("prefix command %s: %v; output=%s", unit.GoalID, runErr, output)
	}
	if !result.Delivery.Sufficient {
		return out, fmt.Errorf("prefix command %s retained insufficient evidence", unit.GoalID)
	}
	return out, nil
}

func TestCommandApplicationGreenTipCannotHideRedPrefix(t *testing.T) {
	f := newPortableFileProof(t)
	f.put("app/a.txt", "red a\n", 0o644)
	redTree, redFiles := f.snapshot()
	redContract := f.loadedContract(redFiles)
	redRequest := f.request(redTree, redFiles, f.plan(redContract, "app/a.txt"))
	redRequest.CandidateEngineBuildIdentity = f.engineIdentity(redTree, redRequest.Environment)
	red, _ := f.execute(redRequest, false)
	redGroup := portableGroups(red)["app-a"]
	if redGroup.Status != "failed" || !redGroup.NativeLaunched || !redGroup.CollectionComplete || red.Delivery.Sufficient {
		t.Fatalf("earlier red candidate lacks complete failed native evidence: %+v", red)
	}
	attempts, err := proofrun.ReadAttempts(f.root)
	if err != nil {
		t.Fatal(err)
	}
	retainedRed := false
	for _, attempt := range attempts {
		if attempt.TestResult == nil || attempt.TestResult.CandidateTree != redTree || attempt.TestResult.AttemptID != red.AttemptID {
			continue
		}
		for _, group := range attempt.TestResult.Groups {
			if group.ID == "app-a" && group.Status == "failed" && group.NativeLaunched && group.CollectionComplete {
				retainedRed = true
			}
		}
	}
	if !retainedRed {
		t.Fatalf("red candidate has no retained complete native failure: %+v", attempts)
	}
	requirePortableCounts(t, f.counts(), map[string]int{"a": 1})
	f.put("app/a.txt", "green a repaired\n", 0o644)
	greenTree, greenFiles := f.snapshot()
	greenContract := f.loadedContract(greenFiles)
	greenRequest := f.request(greenTree, greenFiles, f.plan(greenContract, "app/a.txt"))
	greenRequest.CandidateEngineBuildIdentity = f.engineIdentity(greenTree, greenRequest.Environment)
	green, _ := f.execute(greenRequest, true)
	if group := portableGroups(green)["app-a"]; group.Status != "passed" || !group.NativeLaunched || !green.Delivery.Sufficient {
		t.Fatalf("repaired candidate did not produce green native evidence: %+v", green)
	}
	now := time.Now().UTC()
	verifiedGreen, err := f.verify(greenRequest, greenFiles, green, now)
	if err != nil || !verifiedGreen.Delivery.Sufficient {
		t.Fatalf("repaired candidate verification: sufficient=%t err=%v source=%+v groups=%+v", verifiedGreen.Delivery.Sufficient, err, green.Groups, verifiedGreen.Groups)
	}
	verifiedRed, err := f.verify(redRequest, redFiles, red, now)
	if err != nil || verifiedRed.Delivery.Sufficient {
		t.Fatalf("later green candidate authorized earlier red candidate: sufficient=%t err=%v groups=%+v", verifiedRed.Delivery.Sufficient, err, verifiedRed.Groups)
	}
}

func TestCommandApplicationFreshEpisodeResumesAndRenews(t *testing.T) {
	f := newPortableFileProof(t)
	f.contract.SchemaVersion = testpolicy.ExecutionContractSchemaVersion
	for index := range f.contract.Groups {
		f.contract.Groups[index].Phase = "acceptance"
		f.contract.Groups[index].EnvironmentMode = "inherit"
		f.contract.Groups[index].Freshness = "episode"
	}
	f.writeContract()
	tree, files := f.snapshot()
	contract := f.loadedContract(files)
	if contract.SchemaVersion != testpolicy.ExecutionContractSchemaVersion || contract.Groups[0].Freshness != "episode" {
		t.Fatalf("freshness declaration was not decoded from literal JSON: %+v", contract)
	}
	plan := f.plan(contract, "app/a.txt")
	before := time.Now().UTC().Truncate(time.Second)
	expires := before.Add(time.Hour).Format(time.RFC3339Nano)
	episode := func(digit string) string { return strings.Repeat(digit, 64) }
	args := []string{"--root", f.root, "--goal", "portable", "--tree", tree, "--mode", "auto", "--purpose", "delivery",
		"--fresh-episode", episode("1"), "--fresh-expires-at", expires}
	parsed, _, status := parseTestingSelection("test run", args, true)
	if status != 0 || parsed.FreshEpisode != episode("1") || parsed.FreshExpiresAt != expires {
		t.Fatalf("public freshness flags status=%d selection=%+v", status, parsed)
	}
	request := f.request(tree, files, plan)
	request.CandidateEngineBuildIdentity = f.engineIdentity(tree, request.Environment)
	request.FreshnessEpisode, request.FreshnessExpiresAt = parsed.FreshEpisode, parsed.FreshExpiresAt
	request.FreshGroups = map[string]bool{"app-a": true}
	f.bindFreshness(&request)
	first, _ := f.execute(request, true)
	if verified, err := f.verify(request, files, first, before.Add(time.Minute)); err != nil || !verified.Delivery.Sufficient {
		t.Fatalf("first episode verify sufficient=%t err=%v groups=%+v", verified.Delivery.Sufficient, err, verified.Groups)
	}
	if group := portableGroups(first)["app-a"]; group.Status != "passed" || !group.NativeLaunched || first.FreshnessEpisode != episode("1") {
		t.Fatalf("first episode lacked fresh native observation: %+v", first)
	}
	requirePortableCounts(t, f.counts(), map[string]int{"a": 1})
	same, _ := f.execute(request, true)
	if verified, err := f.verify(request, files, same, before.Add(time.Minute)); err != nil || !verified.Delivery.Sufficient {
		t.Fatalf("same episode verify sufficient=%t err=%v groups=%+v", verified.Delivery.Sufficient, err, verified.Groups)
	}
	if group := portableGroups(same)["app-a"]; group.Status != "reused" || group.ReuseAttempt == "" {
		t.Fatalf("same episode did not reuse prior native observation: %+v", same)
	}
	requirePortableCounts(t, f.counts(), map[string]int{"a": 1})
	newEpisode := request
	newEpisode.FreshnessEpisode = episode("2")
	newEpisode.FreshnessBinding = ""
	f.bindFreshness(&newEpisode)
	renewed, _ := f.execute(newEpisode, true)
	if group := portableGroups(renewed)["app-a"]; group.Status != "passed" || !group.NativeLaunched || renewed.FreshnessEpisode != episode("2") {
		t.Fatalf("new episode reused old observation: %+v", renewed)
	}
	requirePortableCounts(t, f.counts(), map[string]int{"a": 2})
	verified, err := f.verify(newEpisode, files, renewed, before.Add(time.Minute))
	if err != nil || !verified.Delivery.Sufficient {
		t.Fatalf("live episode verify sufficient=%t err=%v groups=%+v", verified.Delivery.Sufficient, err, verified.Groups)
	}
	for _, boundary := range []struct {
		name string
		at   time.Time
	}{{"at-expiry", before.Add(time.Hour)}, {"after-expiry", before.Add(time.Hour).Add(time.Nanosecond)}} {
		t.Run(boundary.name, func(t *testing.T) {
			_, err := f.verify(newEpisode, files, renewed, boundary.at)
			if err == nil || !strings.Contains(err.Error(), "expired") {
				t.Fatalf("freshness verification at %s err=%v", boundary.at, err)
			}
		})
	}
}

const portableCommandCheck = `#!/bin/sh
set -eu
id="$1"
input="$2"
report="$3"
printf '%s\n' "$id" >> "$PORTABLE_NATIVE_COUNTER"
mkdir -p "$report"
if grep -q '^red' "$input"; then
  printf '<testsuite><testcase classname="portable" name="%s"><failure message="red input"/></testcase></testsuite>\n' "$id" > "$report/result.xml"
  exit 9
fi
printf '<testsuite><testcase classname="portable" name="%s"/></testsuite>\n' "$id" > "$report/result.xml"
`

const portableCandidateBuild = `#!/bin/sh
set -eu
if [ "${1:-}" = "--trimpath" ]; then
  out="$3"
else
  out="bin/metasystem"
fi
mkdir -p "$(dirname "$out")"
printf 'build\n' >> %s
printf '#!/bin/sh\n# candidate %%s\nexec %%s "$@"\n' "$METASYSTEM_BUILD_STAMP" %s > "$out"
chmod +x "$out"
`
