package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
)

// carriedDeliveryBed is the whole-owner landing bed prepared for a real
// carried delivery: the repository's own carried transaction scripts and a
// testing contract published on the code origin, a stamped engine with a
// fixture steward identity, and the fixture process identities and host
// admission a person's public test run uses (runFrozenPublicVersionOneCorpusWorker).
type carriedDeliveryBed struct {
	t           *testing.T
	f           *wholeOwnerLanding
	engine      string
	proof       proofBinaryFixture
	environment []string
	owners      intentOwners
	calls       [][]string
	lands       []intentProcessResult
	transport   string
	// unproven withholds the fixture person proof from the carry owner.
	unproven bool
}

func newCarriedDeliveryBed(t *testing.T) *carriedDeliveryBed {
	t.Helper()
	f := newWholeOwnerLanding(t)
	t.Setenv("METASYSTEM_OWNER_LINEAGE", "m1")
	writeFixtureEnrollment(t, f.mainRoot, "Wido")
	b := &carriedDeliveryBed{t: t, f: f}
	b.engine, b.proof = publishCarriedLandScripts(t, f)
	t.Setenv("METASYSTEM_BIN", b.engine)
	// The carry readback and land.sh read the canonical branch as origin, as
	// the batch admission bed provides it.
	goalSyncMutationGit(t, f.mainRoot, "remote", "add", "origin", f.upstream)
	goalSyncMutationGit(t, f.mainRoot, "config", "metasystem.steward.landing-ref", "refs/remotes/origin/main")
	// land.sh publishes to origin, so the ledger is synced there too, as
	// land-fixtures.sh configures its carried beds.
	goalSyncMutationGit(t, f.mainRoot, "config", "goal.sync-remote", "origin")
	// sync-transport.sh mirrors origin's main to the checkout's transport.
	b.transport = filepath.Join(t.TempDir(), "transport.git")
	goalSyncMutationGit(t, filepath.Dir(b.transport), "init", "-q", "--bare", b.transport)
	goalSyncMutationGit(t, f.mainRoot, "remote", "add", "transport", b.transport)
	goalSyncMutationGit(t, f.mainRoot, "fetch", "-q", "origin")
	b.environment = carriedProofEnvironment(t, f.mainRoot)
	// The person runs land from the same terminal: every owner the
	// delivery starts (hooks included) classifies this process as that
	// person, as land-fixtures.sh declares its fixture pid.
	for _, entry := range b.environment[len(b.environment)-4 : len(b.environment)-2] {
		name, value, _ := strings.Cut(entry, "=")
		t.Setenv(name, value)
	}
	b.owners = defaultIntentOwners()
	delivery := defaultIntentDeliveryOwners()
	delivery.executable = func() (string, error) { return b.engine, nil }
	delivery.process = func(process intentProcess) intentProcessResult {
		b.calls = append(b.calls, append([]string(nil), process.argv...))
		if len(process.argv) > 2 && process.argv[1] == "goal" && process.argv[2] == "carry" && !b.unproven {
			process.argv = append(process.argv, "--fixture-human-authority", "--lineage", "m1")
		}
		ran := runIntentOwnerProcess(process)
		if strings.HasSuffix(process.argv[0], "land.sh") {
			b.lands = append(b.lands, ran)
			t.Logf("land.sh exited %d:\n%s\n%s", ran.code, ran.stdout, ran.stderr)
		}
		return ran
	}
	b.owners.delivery = delivery
	return b
}

// land runs the public land command from the main installation.
func (b *carriedDeliveryBed) land(args ...string) (int, intentResult) {
	b.t.Helper()
	return b.landFrom(b.f.mainRoot, args...)
}

// landFrom runs the public land command from an installation.
func (b *carriedDeliveryBed) landFrom(root string, args ...string) (int, intentResult) {
	b.t.Helper()
	command, _ := findIntentCommand("land")
	var stdout, stderr bytes.Buffer
	if !slices.Contains(args, "--repo") {
		args = append(args, "--repo", root)
	}
	code := runIntentIn(command, append(args, "--json"), &stdout, &stderr, root, b.owners)
	var result intentResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		b.t.Fatalf("land printed no result: %v; stdout=%q stderr=%q", err, stdout.String(), stderr.String())
	}
	return code, result
}

// publishProduct lands one product change on origin main from a separate
// clone, as another seat's landing would.
func (b *carriedDeliveryBed) publishProduct(path, content string) {
	b.t.Helper()
	clone := filepath.Join(b.t.TempDir(), "product")
	goalSyncMutationGit(b.t, filepath.Dir(clone), "clone", "-q", "--branch", "main", b.f.upstream, clone)
	writeTestingFixtureFile(b.t, filepath.Join(clone, filepath.FromSlash(path)), []byte(content), 0o755)
	goalSyncMutationGit(b.t, clone, "add", "-A")
	goalSyncMutationGit(b.t, clone, "-c", "user.name=peer", "-c", "user.email=peer@example.invalid", "commit", "-qm", "peer product "+path)
	goalSyncMutationGit(b.t, clone, "push", "-q", "origin", "HEAD:main")
}

// addGoalFold pushes one more plan fold onto the goal's branch through the
// branch owners, as further goal work after an exception would.
func (b *carriedDeliveryBed) addGoalFold(path, content, op string) {
	b.t.Helper()
	root := b.f.goalRoot
	writeTestingFixtureFile(b.t, filepath.Join(root, filepath.FromSlash(path)), []byte(content), 0o644)
	goalSyncMutationGit(b.t, root, "add", path)
	claim := func() error { return nil }
	if _, err := branch.CommitStaged(branch.CommitRequest{Repo: root, Remote: "upstream", EndpointTip: b.f.base,
		GoalID: "standing-validation", OpID: op + "-fold", Kind: branch.Plan, CheckClaim: claim}); err != nil {
		b.t.Fatal(err)
	}
	if _, err := branch.Push(branch.PushRequest{Repo: root, Remote: "upstream", EndpointTip: b.f.base,
		GoalID: "standing-validation", OpID: op + "-push", CheckClaim: claim}); err != nil {
		b.t.Fatal(err)
	}
}

// addGoalUnit pushes one more reviewed unit onto the goal's branch through
// the branch owners; path is repository-relative, so it may lie outside
// the installation.
func (b *carriedDeliveryBed) addGoalUnit(path, content, unit string) {
	b.t.Helper()
	root := b.f.goalRoot
	top := filepath.Dir(root)
	writeTestingFixtureFile(b.t, filepath.Join(top, filepath.FromSlash(path)), []byte(content), 0o755)
	goalSyncMutationGit(b.t, top, "add", path)
	claim := func() error { return nil }
	commit, err := branch.CommitStaged(branch.CommitRequest{Repo: root, Remote: "upstream", EndpointTip: b.f.base,
		GoalID: "standing-validation", Unit: unit, OpID: unit + "-unit", Kind: branch.Unit, CheckClaim: claim})
	if err != nil {
		b.t.Fatal(err)
	}
	digest, err := branch.UnitDigest(root, commit)
	if err != nil {
		b.t.Fatal(err)
	}
	record := "metasystem/records/misc/" + unit + "-read.md"
	writeTestingFixtureFile(b.t, filepath.Join(top, filepath.FromSlash(record)), []byte(commit+" "+digest+"\n"), 0o644)
	unitTree := goalSyncMutationGit(b.t, root, "rev-parse", commit+"^{tree}")
	if _, _, err := branch.CommitRead(branch.CommitReadRequest{Repo: root, Remote: "upstream", EndpointTip: b.f.base,
		GoalID: "standing-validation", Unit: unit, OpID: unit + "-read", ReaderRecord: record,
		GateRunID: "fast-clean", GateTree: unitTree, CheckClaim: claim}); err != nil {
		b.t.Fatal(err)
	}
	if _, err := branch.Push(branch.PushRequest{Repo: root, Remote: "upstream", EndpointTip: b.f.base,
		GoalID: "standing-validation", OpID: unit + "-push", CheckClaim: claim}); err != nil {
		b.t.Fatal(err)
	}
}

// carried is the one commit on origin main carrying the word.
func (b *carriedDeliveryBed) carried(opid string) string {
	b.t.Helper()
	main := b.f.remote(b.t, "refs/heads/main")
	commits := strings.Fields(goalSyncMutationGit(b.t, b.f.mainRoot, "log", "--format=%H", "--fixed-strings", "--grep=Carry: "+opid, main))
	if len(commits) != 1 {
		b.t.Fatalf("origin main %s has %d commits carrying %s", main, len(commits), opid)
	}
	if message := goalSyncMutationGit(b.t, b.f.mainRoot, "log", "-1", "--format=%B", commits[0]); !strings.Contains(message, "Landing-Provenance: carried opid="+opid) {
		b.t.Fatalf("carried commit %s has no carried provenance:\n%s", commits[0], message)
	}
	return commits[0]
}

func carriedResultData(result intentResult) map[string]any {
	data, _ := result.Data.(map[string]any)
	if data == nil {
		data = map[string]any{}
	}
	return data
}

// shown runs the continuation a result showed, exactly as printed.
func (b *carriedDeliveryBed) shown(result intentResult) (int, intentResult) {
	b.t.Helper()
	if result.Next == nil || len(result.Next.Argv) < 3 || result.Next.Argv[1] != "land" {
		b.t.Fatalf("no land continuation was shown: %+v", result)
	}
	return b.land(result.Next.Argv[2:]...)
}

func (b *carriedDeliveryBed) owner(verb ...string) int {
	count := 0
	for _, argv := range b.calls {
		if len(argv) > len(verb) && slices.Equal(argv[1:1+len(verb)], verb) {
			count++
		}
	}
	return count
}

// provePublicly runs the public test command a missing-proof stop showed,
// exactly as shown, through the stamped enrolled engine from the person's
// terminal; it proves the index the delivery staged and must leave it
// unchanged. It returns the stop with its shown after-proof continuation.
func (b *carriedDeliveryBed) provePublicly(stopped intentResult) intentResult {
	b.t.Helper()
	opid, _ := carriedResultData(stopped)["exception"].(string)
	want := []string{"metasystem", "test", "--goal", "standing-validation", "--repo", b.f.mainRoot}
	if stopped.Outcome != intentPartial || stopped.Next == nil || !slices.Equal(stopped.Next.Argv, want) ||
		!strings.Contains(stopped.Next.Reason, "metasystem land standing-validation --using-exception "+opid) {
		b.t.Fatalf("the missing-proof stop did not show the public test and the same-word continuation: %+v", stopped)
	}
	var after []string
	shown, _ := carriedResultData(stopped)["afterProof"].([]any)
	for _, word := range shown {
		text, _ := word.(string)
		after = append(after, text)
	}
	if len(after) < 5 || !slices.Equal(after[:5], []string{"metasystem", "land", "standing-validation", "--using-exception", opid}) {
		b.t.Fatalf("the after-proof continuation is not the same word: %v", after)
	}
	index := goalSyncMutationGit(b.t, b.f.mainRoot, "write-tree")
	run := b.proof.command(b.environment, b.engine, stopped.Next.Argv[1:]...)
	run.Dir = b.f.mainRoot
	output, err := run.CombinedOutput()
	if err != nil {
		b.t.Fatalf("the shown public test command: %v\n%s", err, output)
	}
	if staged := goalSyncMutationGit(b.t, b.f.mainRoot, "write-tree"); staged != index {
		b.t.Fatalf("the public test moved the staged index %s to %s", index, staged)
	}
	return intentResult{Outcome: stopped.Outcome, Data: stopped.Data, Next: &intentNext{Argv: after}}
}

// carriedProofEnvironment is the frozen corpus worker's fixture caller: this
// process as the terminal person, its ancestors neutral, an empty process
// census and an authenticated temporary host admission directory.
func carriedProofEnvironment(t *testing.T, root string) []string {
	t.Helper()
	identities := map[string]map[string]any{fmt.Sprint(os.Getpid()): {"terminal": true}}
	seen := map[int64]bool{int64(os.Getpid()): true}
	current, ok := identity.ParentPid(int64(os.Getpid()))
	for ok && !seen[current] {
		seen[current] = true
		identities[fmt.Sprint(current)] = map[string]any{"pidStartedAt": 1, "command": "fixture-neutral-ancestor"}
		current, ok = identity.ParentPid(current)
	}
	data, err := json.Marshal(identities)
	if err != nil {
		t.Fatal(err)
	}
	table := filepath.Join(t.TempDir(), "process-identities.json")
	writeTestingFixtureFile(t, table, data, 0o600)
	processes := filepath.Join(t.TempDir(), "processes.json")
	writeTestingFixtureFile(t, processes, []byte("[]\n"), 0o600)
	admission := filepath.Join(t.TempDir(), "host-admission")
	// The person runs it from the same shell that runs the landing: the
	// candidate engine's build identity digests the inherited Go settings.
	return append(os.Environ(),
		"METASYSTEM_FAKE_PROCESS_IDENTITY_FILE="+table,
		"METASYSTEM_CENSUS_PROCESS_FILE="+processes,
		"METASYSTEM_PROOF_ADMISSION_TEST_DIR="+admission,
		"METASYSTEM_PROOF_ADMISSION_FIXTURE_ROOT="+root)
}

// publishCarriedLandScripts lands the repository's own carried transaction
// scripts on the bed's code origin from a separate clone, the way any other
// product change reaches main; the main checkout stays behind origin until
// the delivery advances it.
func publishCarriedLandScripts(t *testing.T, f *wholeOwnerLanding) (string, proofBinaryFixture) {
	t.Helper()
	engine := filepath.Join(t.TempDir(), "carried-engine", "metasystem")
	clone := filepath.Join(t.TempDir(), "publisher")
	goalSyncMutationGit(t, filepath.Dir(clone), "clone", "-q", "--branch", "main", f.upstream, clone)
	prefix := goalSyncMutationGit(t, f.mainRoot, "rev-parse", "--show-prefix")
	// A real checkout tracks the design page the bed only writes as its
	// template marker beside the installation; main takes it first, so the
	// scripts below are the one product commit main is behind by.
	markerPath := filepath.Join(filepath.Dir(f.mainRoot), "development", "metasystem-design.md")
	marker, err := os.ReadFile(markerPath)
	if err != nil {
		t.Fatal(err)
	}
	writeTestingFixtureFile(t, filepath.Join(clone, "development", "metasystem-design.md"), marker, 0o644)
	// The bed's hand-written claim lacks the breach-stop capability a real
	// claim binds (goal bindClaim); a person's test run dispatches only a
	// goal that has it.
	pagePath := filepath.Join(clone, filepath.FromSlash(prefix), "plans", "goals", "standing-validation.md")
	pageData, err := os.ReadFile(pagePath)
	if err != nil {
		t.Fatal(err)
	}
	page, problems := goal.ParseFile(pageData)
	if len(problems) != 0 || page.Claimed == nil {
		t.Fatalf("parse goal page: %v", problems)
	}
	page.Revision++
	page.StopCapability = &goal.StopCapability{Generation: page.Claimed.Revision, Revision: page.Claimed.Revision, Machine: page.Claimed.Machine, ClaimEpoch: 1}
	writeTestingFixtureFile(t, pagePath, goal.RenderFile(page), 0o644)
	goalSyncMutationGit(t, clone, "add", "-A")
	goalSyncMutationGit(t, clone, "-c", "user.name=fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "track the design page")
	goalSyncMutationGit(t, clone, "push", "-q", "origin", "HEAD:main")
	accepted := goalSyncMutationGit(t, clone, "rev-parse", "HEAD")
	if err := os.Remove(markerPath); err != nil {
		t.Fatal(err)
	}
	goalSyncMutationGit(t, f.mainRoot, "pull", "-q", "--ff-only", f.upstream, "main")
	goalSyncMutationGit(t, f.mainRoot, "update-ref", goal.LocalLedgerBranch, accepted)
	goalSyncMutationGit(t, f.mainRoot, "update-ref", goal.AcceptedRef, accepted)
	for _, name := range []string{"land.sh", "commit.sh", "pre-commit-guard.sh", "coverage-delta.sh", "sync-transport.sh", "path-classes.txt", "landing-classes.json"} {
		data, err := os.ReadFile(filepath.Join("..", "..", "scripts", "agents", name))
		if err != nil {
			t.Fatal(err)
		}
		mode := os.FileMode(0o755)
		if !strings.HasSuffix(name, ".sh") {
			mode = 0o644
		}
		if name == "path-classes.txt" {
			// The goal's product file is classed as land-fixtures.sh
			// classes its payload.
			data = append(data, []byte("install:owned.go behavior\n")...)
		}
		writeTestingFixtureFile(t, filepath.Join(clone, filepath.FromSlash(prefix), "scripts", "agents", name), data, mode)
	}
	// The batteries the commit owner re-proves are land-fixtures.sh's own
	// carried stubs, and its proof engine is the bed's built engine, as
	// land-fixtures.sh's go-build.sh copies it: this bed proves the carried
	// transport, not the suites.
	for _, battery := range []string{"agents/dispatch-fixtures.sh", "agents/goal-cli-fixtures.sh", "audit-metasystem.sh"} {
		writeTestingFixtureFile(t, filepath.Join(clone, filepath.FromSlash(prefix), "scripts", filepath.FromSlash(battery)), []byte("#!/usr/bin/env bash\nexit 0\n"), 0o755)
	}
	gate := fmt.Sprintf("#!/usr/bin/env bash\nset -euo pipefail\n[[ \"${1:-}\" == --fast && \"${2:-}\" == --proof-out && -n \"${3:-}\" ]]\ncp %q \"$3\"\n", engine)
	writeTestingFixtureFile(t, filepath.Join(clone, filepath.FromSlash(prefix), "scripts", "agents", "go-gate.sh"), []byte(gate), 0o755)
	build := fmt.Sprintf("#!/usr/bin/env bash\nset -euo pipefail\n[[ \"${1:-}\" == --trimpath && \"${2:-}\" == --out && -n \"${3:-}\" ]]\ncp %q \"$3\"\nchmod +x \"$3\"\n", engine)
	writeTestingFixtureFile(t, filepath.Join(clone, filepath.FromSlash(prefix), "scripts", "agents", "go-build.sh"), []byte(build), 0o755)
	// The carried commit runs the contract's battery for real; this is
	// land-fixtures.sh's carried-prefixed contract over the bed's paths.
	conf := filepath.Join(clone, filepath.FromSlash(prefix), "metasystem.conf")
	confData, err := os.ReadFile(conf)
	if err != nil {
		t.Fatal(err)
	}
	writeTestingFixtureFile(t, conf, append(confData, []byte("testing.contract=testing.json\n")...), 0o644)
	writeTestingFixtureFile(t, filepath.Join(clone, filepath.FromSlash(prefix), "testing.json"), []byte(carriedDeliveryContract), 0o644)
	proof := pinProofBinaryFixture(t, filepath.Join(clone, filepath.FromSlash(prefix)))
	rulings, err := os.ReadFile(filepath.Join("..", "..", "memory", "rulings.md"))
	if err != nil {
		t.Fatal(err)
	}
	writeTestingFixtureFile(t, filepath.Join(clone, filepath.FromSlash(prefix), "memory", "rulings.md"), rulings, 0o644)
	goalSyncMutationGit(t, clone, "add", "-A")
	goalSyncMutationGit(t, clone, "-c", "user.name=fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "install carried landing scripts")
	goalSyncMutationGit(t, clone, "push", "-q", "origin", "HEAD:main")
	// The testing owner runs only an enrolled engine built from this
	// history: the batch e2e bed's stamped build and fixture enrollment.
	stamp := goalSyncMutationGit(t, clone, "rev-parse", "HEAD")
	stamped := exec.Command("go", "build", "-buildvcs=false", "-ldflags", "-X github.com/widoriezebos/agentic-tools/metasystem/internal/supervise.BuildStamp="+stamp, "-o", engine, ".")
	if output, err := stamped.CombinedOutput(); err != nil {
		t.Fatalf("build stamped fixture engine: %v: %s", err, output)
	}
	canonicalRoot, err := canonicalProofRoot(f.mainRoot)
	if err != nil {
		t.Fatal(err)
	}
	canonicalEngine, err := canonicalPath(engine)
	if err != nil {
		t.Fatal(err)
	}
	digest, err := fileSHA256(canonicalEngine)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(steward.RepoIdentityPath(canonicalRoot)), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := steward.MintIdentity(steward.RepoIdentityPath(canonicalRoot), steward.InstallIdentity{RepoIdentity: canonicalRoot, Generation: 1,
		InstallPath: canonicalEngine, InstallDigest: "sha256:" + digest, MintedAt: "2026-09-26T00:00:00Z",
		Enrollment: steward.EnrollmentFixture, EngineBuild: stamp}); err != nil {
		t.Fatal(err)
	}
	return engine, proof
}

const carriedDeliveryContract = `{
  "schemaVersion": 1,
  "projectRisk": {"severity": 1, "exposure": 1, "reversibility": "revert", "detection": "immediate", "recovery": "bounded"},
  "surfaces": [
    {"id": "fixture-carry", "paths": ["metasystem/owned.go"], "dependsOn": [], "standard": ["fixture-carry"], "deep": [], "critical": []},
    {"id": "fixture-support", "paths": ["metasystem/testing.json", "metasystem/metasystem.conf", "metasystem/plans/**", "metasystem/scripts/**", "metasystem/memory/**", "metasystem/records/**", "development/**", "benchmark/**"], "dependsOn": [], "standard": ["fixture-carry"], "deep": [], "critical": []}
  ],
  "groups": [
    {"id": "fixture-carry", "kind": "unit", "adapter": "command", "cwd": ".", "inputs": ["metasystem/owned.go"], "outputs": ["fixture-reports"], "tools": [], "obligations": [], "platforms": ["any"], "targetMs": 1000, "argv": ["sh", "-c", "mkdir -p fixture-reports; printf '%s\\n' '<testsuite><testcase classname=\"fixture\" name=\"carry\"/></testsuite>' >fixture-reports/result.xml"], "reports": ["fixture-reports"], "format": "junit-xml", "expectedTests": [{"report": "fixture-reports/result.xml", "classname": "fixture", "name": "carry"}]}
  ],
  "always": {"canary": [], "standard": []},
  "unknown": ["fixture-carry"],
  "cadence": ["fixture-carry"]
}
`

// TestIntentCarriedReplay: a carried landing interrupted before its push
// resumes under the same word and the same retained subject, although the
// goal's branch moved on meanwhile; the local carried commit is recovered
// without staging anything. A word whose consumption only the fetched
// origin knows resumes through recovery and stages nothing.
func TestIntentCarriedReplay(t *testing.T) {
	b := newCarriedDeliveryBed(t)
	f := b.f
	_, result := b.land("standing-validation", "--exception", "missing-declaration", "--reason", "flaky host", "--by", "Wido", "--upgrade-goals")
	opid, _ := carriedResultData(result)["exception"].(string)
	if result.Outcome != intentPartial || opid == "" {
		t.Fatalf("first request: %+v", result)
	}
	proved := b.provePublicly(result)
	b.addGoalFold("plans/later.md", "later goal work\n", "later")
	t.Setenv("METASYSTEM_LAND_FIXTURE_CRASH", "before-push")
	code, crashed := b.shown(proved)
	os.Unsetenv("METASYSTEM_LAND_FIXTURE_CRASH")
	if code == 0 || crashed.Outcome != intentPartial || !strings.Contains(string(b.lands[len(b.lands)-1].stdout), "FIXTURE-CRASH before-push") {
		t.Fatalf("the crash seam did not stop the landing before its push: %d %+v", code, crashed)
	}
	head := goalSyncMutationGit(t, f.mainRoot, "log", "-1", "--format=%B", "HEAD")
	if !strings.Contains(head, "Carry: "+opid) {
		t.Fatalf("no local carried commit was retained:\n%s", head)
	}
	code, resumed := b.shown(crashed)
	data := carriedResultData(resumed)
	if consumption, _ := data["consumption"].(string); code != 0 || resumed.Outcome != intentConfirmed || !strings.HasPrefix(consumption, "local:") || data["staged"] != nil {
		t.Fatalf("the local carried commit was not recovered without staging: %d %+v", code, resumed)
	}
	if b.owner("goal", "carry") != 1 {
		t.Fatalf("replay recorded another exception: %v", b.calls)
	}
	carried := b.carried(opid)
	if out := goalSyncMutationGit(t, f.mainRoot, "ls-tree", "-r", "--name-only", carried, "--", "metasystem/plans/later.md"); out != "" {
		t.Fatalf("the replay landed goal work newer than its exception: %s", out)
	}
	// A seat that never saw the landing: its main and origin ref are from
	// before the push. The fetch discovers the consumption, nothing stages.
	goalSyncMutationGit(t, f.mainRoot, "reset", "-q", "--keep", carried+"^")
	goalSyncMutationGit(t, f.mainRoot, "update-ref", "refs/remotes/origin/main", carried+"^")
	before := goalSyncMutationGit(t, f.mainRoot, "write-tree")
	code, consumed := b.land("standing-validation", "--using-exception", opid)
	data = carriedResultData(consumed)
	if consumption, _ := data["consumption"].(string); code != 0 || consumed.Outcome != intentConfirmed || consumption == "none" || strings.HasPrefix(consumption, "local:") || data["staged"] != nil {
		t.Fatalf("the fetched origin consumption did not resume without staging: %d %+v", code, consumed)
	}
	if after := goalSyncMutationGit(t, f.mainRoot, "write-tree"); after != before {
		t.Fatalf("recovery of an origin-consumed word staged %s over %s", after, before)
	}
}

// TestIntentCarriedReplacement: a known unrelated path beside the
// installation refuses the delivery before anything is staged, the word
// stays; after origin's product moves, the same word shows the explicit
// replacement, which really supersedes it through the carry owner and lands
// under the new word on the current main.
func TestIntentCarriedReplacement(t *testing.T) {
	b := newCarriedDeliveryBed(t)
	f := b.f
	top := filepath.Dir(f.mainRoot)
	scratch := filepath.Join(top, "notes.txt")
	writeTestingFixtureFile(t, scratch, []byte("a person's scratch\n"), 0o644)
	indexBefore := goalSyncMutationGit(t, f.mainRoot, "write-tree")
	_, result := b.land("standing-validation", "--exception", "missing-declaration", "--reason", "flaky host", "--by", "Wido", "--upgrade-goals")
	first, _ := carriedResultData(result)["exception"].(string)
	if result.Outcome != intentPartial || first == "" || !strings.Contains(result.Summary, "carried-checkout-dirty") || !strings.Contains(result.Summary, "notes.txt") || len(b.lands) != 0 {
		t.Fatalf("an untracked top-level path did not refuse before staging: %+v", result)
	}
	if index := goalSyncMutationGit(t, f.mainRoot, "write-tree"); index != goalSyncMutationGit(t, f.mainRoot, "rev-parse", "HEAD^{tree}") && index != indexBefore {
		t.Fatalf("the refused delivery left a staged product: %s", index)
	}
	if err := os.Remove(scratch); err != nil {
		t.Fatal(err)
	}
	// A peer's product outside the engine: the enrolled engine stays current.
	b.publishProduct("benchmark/peer.sh", "#!/usr/bin/env bash\n# moved\nexit 0\n")
	_, moved := b.shown(result)
	if moved.Outcome != intentPartial || !strings.Contains(moved.Summary, "carried-origin-moved") || moved.Next == nil ||
		!slices.Contains(moved.Next.Argv, "--replace-exception") || !slices.Contains(moved.Next.Argv, first) {
		t.Fatalf("moved product did not show the explicit replacement: %+v", moved)
	}
	_, replaced := b.shown(moved)
	second, _ := carriedResultData(replaced)["exception"].(string)
	if second == "" || second == first || b.owner("goal", "carry") != 2 {
		t.Fatalf("the replacement did not record a new word: %+v", replaced)
	}
	last := b.calls[len(b.calls)-1]
	for _, argv := range b.calls {
		if len(argv) > 2 && argv[1] == "goal" && argv[2] == "carry" {
			last = argv
		}
	}
	if !slices.Contains(last, "--supersede") || !slices.Contains(last, first) {
		t.Fatalf("the replacement did not reach the carry owner's supersede: %v", last)
	}
	code, landed := b.shown(b.provePublicly(replaced))
	if code != 0 || landed.Outcome != intentConfirmed {
		t.Fatalf("the replacement word did not land: %d %+v", code, landed)
	}
	carried := b.carried(second)
	if got := goalSyncMutationGit(t, f.mainRoot, "show", carried+":benchmark/peer.sh"); !strings.Contains(got, "# moved") {
		t.Fatalf("the replacement rewound the newer product: %q", got)
	}
	// The replaced word never delivers again: its owner reports it
	// superseded, and nothing is staged for it.
	head := goalSyncMutationGit(t, f.mainRoot, "rev-parse", "HEAD")
	lands := len(b.lands)
	code, old := b.land("standing-validation", "--using-exception", first)
	data := carriedResultData(old)
	if consumption, _ := data["consumption"].(string); code == 0 || old.Outcome == intentConfirmed || consumption != "superseded:"+second || data["staged"] != nil ||
		goalSyncMutationGit(t, f.mainRoot, "write-tree") != goalSyncMutationGit(t, f.mainRoot, "rev-parse", "HEAD^{tree}") ||
		goalSyncMutationGit(t, f.mainRoot, "rev-parse", "HEAD") != head || b.owner("goal", "carry") != 2 {
		t.Fatalf("the replaced word was reused: %d %+v (land.sh runs %d -> %d)", code, old, lands, len(b.lands))
	}
}

// TestIntentCarriedFromTheLinkedGoalCheckout: requested from the goal's
// linked checkout, the delivery lands into the main checkout it belongs to,
// including goal code outside the installation, and keeps the rows hooks
// already appended to main's receipt ledger.
func TestIntentCarriedFromTheLinkedGoalCheckout(t *testing.T) {
	b := newCarriedDeliveryBed(t)
	f := b.f
	b.addGoalUnit("benchmark/carried.sh", "#!/usr/bin/env bash\nexit 0\n", "u2")
	// The goal worktree tracks the design page a template checkout has; its
	// branch predates the bed's tracking commit.
	writeTestingFixtureFile(t, filepath.Join(filepath.Dir(f.goalRoot), "development", "metasystem-design.md"), []byte("# fixture\n"), 0o644)
	ledger := filepath.Join(f.mainRoot, "memory", "receipts.log")
	existing, err := os.ReadFile(ledger)
	if err != nil {
		t.Fatal(err)
	}
	hookRow := "2|2026-09-26T10:00:00Z|RECEIPT|type=other|outcome=shipped|note=hook row\n"
	writeTestingFixtureFile(t, ledger, append(existing, []byte(hookRow)...), 0o644)
	goalBefore := goalSyncMutationGit(t, f.goalRoot, "rev-parse", "HEAD") + goalSyncMutationGit(t, f.goalRoot, "status", "--porcelain=v1")
	_, result := b.landFrom(f.goalRoot, "standing-validation", "--exception", "missing-declaration", "--reason", "flaky host", "--by", "Wido", "--upgrade-goals")
	opid, _ := carriedResultData(result)["exception"].(string)
	if opid == "" || len(b.lands) != 1 || !strings.Contains(b.calls[len(b.calls)-1][0], filepath.Join(f.mainRoot, "scripts", "agents", "land.sh")) {
		t.Fatalf("the linked request did not deliver through the main checkout: %+v %v", result, b.calls)
	}
	code, landed := b.shown(b.provePublicly(result))
	if code != 0 || landed.Outcome != intentConfirmed {
		t.Fatalf("the linked delivery did not land: %d %+v", code, landed)
	}
	carried := b.carried(opid)
	if got := goalSyncMutationGit(t, f.mainRoot, "show", carried+":benchmark/carried.sh"); !strings.Contains(got, "exit 0") {
		t.Fatalf("the goal's top-level code did not land: %q", got)
	}
	if got := goalSyncMutationGit(t, f.mainRoot, "show", carried+":metasystem/memory/receipts.log"); !strings.Contains(got, "note=hook row") || !strings.Contains(got, "type=seed") {
		t.Fatalf("the landing lost main's existing receipt rows:\n%s", got)
	}
	if after := goalSyncMutationGit(t, f.goalRoot, "rev-parse", "HEAD") + goalSyncMutationGit(t, f.goalRoot, "status", "--porcelain=v1"); after != goalBefore {
		t.Fatalf("the delivery changed the goal checkout:\nbefore=%s\nafter=%s", goalBefore, after)
	}
}

// TestIntentCarriedCarryOwnerDecidesThePerson: after the code and ledger
// refresh the adapter hands the exception to the real carry owner with its
// own argv, and that owner's refusal of a caller it cannot prove to be a
// person stands: nothing is recorded, staged or landed.
func TestIntentCarriedCarryOwnerDecidesThePerson(t *testing.T) {
	b := newCarriedDeliveryBed(t)
	b.unproven = true
	head := goalSyncMutationGit(t, b.f.mainRoot, "rev-parse", "HEAD")
	code, result := b.land("standing-validation", "--exception", "missing-declaration", "--reason", "flaky host", "--by", "Wido")
	if code == 0 || result.Outcome == intentConfirmed || carriedResultData(result)["exception"] != nil || len(b.lands) != 0 {
		t.Fatalf("an unproven caller's exception was accepted: %d %+v", code, result)
	}
	if len(b.calls) != 2 || !slices.Equal(b.calls[0][1:], []string{"goal", "fetch", "--root", b.f.mainRoot}) ||
		!slicesHasPrefix(b.calls[1][1:], []string{"goal", "carry", "--root", b.f.mainRoot, "--id", "standing-validation", "--by", "Wido"}) {
		t.Fatalf("owner routing = %v", b.calls)
	}
	if index := goalSyncMutationGit(t, b.f.mainRoot, "write-tree"); index != goalSyncMutationGit(t, b.f.mainRoot, "rev-parse", "HEAD^{tree}") ||
		goalSyncMutationGit(t, b.f.mainRoot, "rev-parse", "HEAD") != head {
		t.Fatalf("a refused exception changed the main checkout")
	}
}

// answerCarry records a channel carry word through the goal ledger's own
// answer owner, exactly as the channel poller records an authenticated
// reply to a carry question (channel/poll.go advanceAnswer): the reply text
// is the question's wanted carry token. The provider's authentication of
// the reply is the poller's and is not exercised here.
func (b *carriedDeliveryBed) answerCarry(workspace, ulid string) string {
	b.t.Helper()
	endpoint, err := goal.ResolveEndpoint(b.f.mainRoot)
	if err != nil {
		b.t.Fatal(err)
	}
	machine, err := goal.ResolveMachine(b.f.mainRoot)
	if err != nil {
		b.t.Fatal(err)
	}
	request := goal.VerbRequest{Endpoint: endpoint, Actor: goal.Actor{Machine: machine, Lineage: "m1"}, Ulid: ulid, Now: time.Now().UTC()}
	text := "carry workspace=" + workspace + " goal=standing-validation past=missing-declaration"
	published, err := goal.Answer(request, "standing-validation", "q-carry-"+ulid, text, "",
		goal.AnswerProof{Provider: "fixture-channel", User: "wido", Ref: "thread/" + ulid, Step: 1})
	if err != nil || published.Outcome != goal.OutcomeConfirmed {
		b.t.Fatalf("channel carry answer: %+v %v", published, err)
	}
	return goal.Opid(ulid, machine, "m1")
}

// raiseFormatThroughAnExpiredExercise is a person's earlier exception that
// raised the ledger to the carry format, recorded by the real carry owner
// on the ledger's fixture clock two hours ago with a one-hour life: the
// ledger is at format 2 and no other word is open.
func (b *carriedDeliveryBed) raiseFormatThroughAnExpiredExercise() {
	b.t.Helper()
	tree := goalSyncMutationGit(b.t, b.f.mainRoot, "rev-parse", "refs/remotes/origin/main^{tree}")
	carry := exec.Command(b.engine, "goal", "carry", "--root", b.f.mainRoot, "--id", "standing-validation", "--by", "Wido", "--tree", tree,
		"--past", "missing-declaration", "--why", "an earlier exception", "--expires", "1h", "--raise-format", "--fixture-human-authority", "--lineage", "m1")
	carry.Env = append(os.Environ(), "METASYSTEM_GOAL_NOW="+time.Now().UTC().Add(-2*time.Hour).Format(time.RFC3339))
	if output, err := carry.CombinedOutput(); err != nil {
		b.t.Fatalf("raise the ledger format through an earlier exception: %v\n%s", err, output)
	}
}

// composed is the workspace the goal's canonical candidate owner composes
// on the live endpoint now.
func (b *carriedDeliveryBed) composed() string {
	b.t.Helper()
	candidate, _, err := b.owners.delivery.landCandidate([]string{"--root", b.f.mainRoot, "--goal", "standing-validation", "--last"})
	if err != nil {
		b.t.Fatal(err)
	}
	return candidate.Result.Candidate
}

// TestIntentCarriedChannelWordAdoption: a carry word the channel recorded
// for the exact workspace the goal composes is adopted by the public land
// under that word; no carry owner runs, the composition is bound to the
// word, and the carried transaction lands it.
func TestIntentCarriedChannelWordAdoption(t *testing.T) {
	b := newCarriedDeliveryBed(t)
	b.raiseFormatThroughAnExpiredExercise()
	opid := b.answerCarry(b.composed(), "01K6CARRYCHANNEXWKRD00000A")
	_, result := b.land("standing-validation", "--using-exception", opid)
	if data := carriedResultData(result); data["staged"] == nil || b.owner("goal", "carry") != 0 {
		t.Fatalf("the channel word was not adopted for its exact composition: %+v %v", result, b.calls)
	}
	bound := filepath.Join(b.f.mainRoot, "artifacts", "agents", "intent-land", "standing-validation", "exception-"+opid+".json")
	if _, err := os.Stat(bound); err != nil {
		t.Fatalf("the adopted composition was not bound to the word: %v", err)
	}
	code, landed := b.shown(b.provePublicly(result))
	if code != 0 || landed.Outcome != intentConfirmed || b.owner("goal", "carry") != 0 {
		t.Fatalf("the adopted channel word did not land: %d %+v", code, landed)
	}
	b.carried(opid)
}

// TestIntentCarriedChannelWordMismatchRefuses: a channel word for a
// workspace the goal does not compose to is refused before anything is
// bound or staged, and no other authorization is minted.
func TestIntentCarriedChannelWordMismatchRefuses(t *testing.T) {
	b := newCarriedDeliveryBed(t)
	endpoint := goalSyncMutationGit(t, b.f.mainRoot, "rev-parse", "refs/remotes/origin/main^{tree}")
	opid := b.answerCarry(endpoint, "01K6CARRYCHANNEXWKRD00000B")
	head := goalSyncMutationGit(t, b.f.mainRoot, "rev-parse", "HEAD")
	code, result := b.land("standing-validation", "--using-exception", opid)
	if code == 0 || result.Outcome != intentPartial || !strings.Contains(result.Summary, "not the exception's "+endpoint) ||
		carriedResultData(result)["staged"] != nil || len(b.lands) != 0 || b.owner("goal", "carry") != 0 {
		t.Fatalf("a mismatched channel word was not refused: %d %+v", code, result)
	}
	if _, err := os.Stat(filepath.Join(b.f.mainRoot, "artifacts", "agents", "intent-land", "standing-validation", "exception-"+opid+".json")); !os.IsNotExist(err) {
		t.Fatalf("a mismatched composition was bound: %v", err)
	}
	if goalSyncMutationGit(t, b.f.mainRoot, "write-tree") != goalSyncMutationGit(t, b.f.mainRoot, "rev-parse", "HEAD^{tree}") ||
		goalSyncMutationGit(t, b.f.mainRoot, "rev-parse", "HEAD") != head {
		t.Fatal("a refused channel word changed the main checkout")
	}
}

// TestIntentCarriedAmbiguousRetainedBaseNeedsReplacement: a word whose
// binding was lost while two retained compositions share its workspace
// (the carry itself moved origin, so the identical request composed on a
// second endpoint) is never delivered on a guessed base. The identical
// request and the same-word continuation both refuse before any carry,
// binding, staging or delivery and show the public replacement from the
// word's own facts; that replacement supersedes the word and lands.
func TestIntentCarriedAmbiguousRetainedBaseNeedsReplacement(t *testing.T) {
	b := newCarriedDeliveryBed(t)
	f := b.f
	request := []string{"standing-validation", "--exception", "missing-declaration", "--reason", "flaky host", "--by", "Wido", "--upgrade-goals"}
	_, result := b.land(request...)
	first, _ := carriedResultData(result)["exception"].(string)
	if first == "" || result.Outcome != intentPartial {
		t.Fatalf("first request: %+v", result)
	}
	subjects := filepath.Join(f.mainRoot, "artifacts", "agents", "intent-land", "standing-validation")
	if err := os.Remove(filepath.Join(subjects, "exception-"+first+".json")); err != nil {
		t.Fatal(err)
	}
	want := []string{"metasystem", "land", "standing-validation", "--exception", "missing-declaration", "--replace-exception", first}
	for _, args := range [][]string{request, {"standing-validation", "--using-exception", first}} {
		index := goalSyncMutationGit(t, f.mainRoot, "write-tree")
		head := goalSyncMutationGit(t, f.mainRoot, "rev-parse", "HEAD")
		carries, lands := b.owner("goal", "carry"), len(b.lands)
		code, refused := b.land(args...)
		if code == 0 || !strings.Contains(refused.Summary, "carried-subject-ambiguous") || refused.Next == nil ||
			!slicesHasPrefix(refused.Next.Argv, want) || !slices.Contains(refused.Next.Argv, "Wido") {
			t.Fatalf("%v did not refuse the ambiguous composition with the public replacement: %d %+v", args, code, refused)
		}
		if b.owner("goal", "carry") != carries || len(b.lands) != lands || carriedResultData(refused)["staged"] != nil ||
			goalSyncMutationGit(t, f.mainRoot, "write-tree") != index || goalSyncMutationGit(t, f.mainRoot, "rev-parse", "HEAD") != head {
			t.Fatalf("%v had effects before refusing", args)
		}
		if _, err := os.Stat(filepath.Join(subjects, "exception-"+first+".json")); !os.IsNotExist(err) {
			t.Fatalf("%v bound a guessed composition: %v", args, err)
		}
		result = refused
	}
	_, replaced := b.shown(result)
	second, _ := carriedResultData(replaced)["exception"].(string)
	if second == "" || second == first || b.owner("goal", "carry") != 2 {
		t.Fatalf("the replacement did not record a new word: %+v", replaced)
	}
	for _, argv := range b.calls {
		if len(argv) > 2 && argv[1] == "goal" && argv[2] == "carry" && slices.Contains(argv, "--supersede") && !slices.Contains(argv, first) {
			t.Fatalf("the replacement superseded another word: %v", argv)
		}
	}
	code, landed := b.shown(b.provePublicly(replaced))
	if code != 0 || landed.Outcome != intentConfirmed {
		t.Fatalf("the replacement did not land: %d %+v", code, landed)
	}
	b.carried(second)
}

// TestIntentCarriedChannelWordAdoptionStopsWhenBindingFails: when the
// composition of a channel word cannot be persisted, the landing stops there:
// nothing is staged, no composition is bound to the word and no carry act
// is recorded. The failure is the real subject store's (a directory the
// owner cannot write).
func TestIntentCarriedChannelWordAdoptionStopsWhenBindingFails(t *testing.T) {
	b := newCarriedDeliveryBed(t)
	b.raiseFormatThroughAnExpiredExercise()
	opid := b.answerCarry(b.composed(), "01K6CARRYCHANNEXWKRD00000B")
	subjects := filepath.Join(b.f.mainRoot, "artifacts", "agents", "intent-land", "standing-validation")
	// The subject store becomes a file at the adoption's own composition,
	// after land has read that no subject is bound or retained and before
	// it retains the composed one: the real retain then fails.
	aside := subjects + ".aside"
	obstructed := 0
	compose := b.owners.delivery.landCandidate
	b.owners.delivery.landCandidate = func(args []string) (goalBranchLandPrepOutcome, int, error) {
		outcome, code, err := compose(args)
		if slices.Contains(args, "--last") {
			obstructed++
			if renameErr := os.Rename(subjects, aside); renameErr != nil && !os.IsNotExist(renameErr) {
				t.Fatalf("setting the subject store aside: %v", renameErr)
			}
			if mkdirErr := os.MkdirAll(filepath.Dir(subjects), 0o755); mkdirErr != nil {
				t.Fatalf("the subject store's parent: %v", mkdirErr)
			}
			if writeErr := os.WriteFile(subjects, []byte("not a directory\n"), 0o644); writeErr != nil {
				t.Fatalf("obstructing the subject store: %v", writeErr)
			}
		}
		return outcome, code, err
	}
	t.Cleanup(func() {
		if info, err := os.Lstat(subjects); err == nil && info.Mode().IsRegular() {
			if err := os.Remove(subjects); err != nil {
				t.Errorf("removing the obstruction: %v", err)
			}
		}
		if err := os.Rename(aside, subjects); err != nil && !os.IsNotExist(err) {
			t.Errorf("restoring the subject store: %v", err)
		}
	})
	code, result := b.land("standing-validation", "--using-exception", opid)
	if code == 0 || result.Outcome == intentConfirmed || carriedResultData(result)["staged"] != nil ||
		!strings.Contains(result.Summary, "cannot be bound") || b.owner("goal", "carry") != 0 {
		t.Fatalf("a composition that cannot be bound was adopted: %d %+v %v", code, result, b.calls)
	}
	if obstructed != 1 {
		t.Fatalf("the adoption composed %d times, want once", obstructed)
	}
	if _, err := os.Stat(filepath.Join(subjects, "exception-"+opid+".json")); err == nil {
		t.Fatalf("a composition was bound although persisting it failed: %v", err)
	}
}
