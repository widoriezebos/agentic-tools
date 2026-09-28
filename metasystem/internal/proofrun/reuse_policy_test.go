package proofrun

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

func TestReusePolicyTableNamesEveryReasonToExecute(t *testing.T) {
	t.Parallel()
	plain := testpolicy.Group{ID: "g", Adapter: "go"}
	external := testpolicy.Group{ID: "g", Adapter: "go", ExternalInputs: []testpolicy.ExternalInput{{ID: "e", Path: "/x"}}}
	// A frontend that reads the execution record (schema 4).
	reads := TestRunRequest{Workers: 1, ResultSchemaVersion: TestResultSchemaVersion}
	fresh := reads
	fresh.FreshGroups = map[string]bool{"g": true}
	for _, row := range []struct {
		name     string
		request  TestRunRequest
		group    testpolicy.Group
		shard    int
		facts    goCacheFacts
		countOne bool
		reason   string
	}{
		{"ordinary group", reads, plain, 1, goCacheFacts{}, false, GoTestCacheAllowed},
		{"fresh request (episode, --no-reuse, cadence)", fresh, plain, 1, goCacheFacts{}, true, GoTestCountOneFresh},
		{"frontend without the execution record", TestRunRequest{Workers: 1}, plain, 1, goCacheFacts{}, true, GoTestCountOneResultSchema},
		{"diagnostic rerun", reads, plain, diagnosticRerunShard, goCacheFacts{}, true, GoTestCountOneRerun},
		{"coverage shard", reads, testpolicy.Group{ID: "g", Adapter: "go", Coverage: true}, 2, goCacheFacts{}, true, GoTestCountOneCoverage},
		{"external inputs never recorded", reads, external, 1, goCacheFacts{ExternalInputsDigest: "a"}, true, GoTestCountOneUnrecorded},
		{"external inputs changed", reads, external, 1, goCacheFacts{ExternalInputsDigest: "b", RecordedExternalDigest: "a"}, true, GoTestCountOneExternal},
		{"external inputs equal", reads, external, 1, goCacheFacts{ExternalInputsDigest: "a", RecordedExternalDigest: "a"}, false, GoTestCacheAllowed},
		{"undeclared child launches", reads, plain, 1, goCacheFacts{ChildLaunches: []string{"p/x_test.go"}}, true, GoTestCountOneUntrackedChild},
		{"declared external inputs cover children", reads, external, 1,
			goCacheFacts{ExternalInputsDigest: "a", RecordedExternalDigest: "a", ChildLaunches: []string{"p/x_test.go"}}, false, GoTestCacheAllowed},
	} {
		countOne, reason := reusePolicy(row.request, row.group, row.shard, row.facts)
		if countOne != row.countOne || reason != row.reason {
			t.Errorf("%s: countOne=%t reason=%q, want %t %q", row.name, countOne, reason, row.countOne, row.reason)
		}
	}
}

func TestGoPackageExecutionsReadTheCachedMarker(t *testing.T) {
	t.Parallel()
	stream := strings.Join([]string{
		`{"Action":"start","Package":"m/a"}`,
		`{"Action":"run","Package":"m/a","Test":"TestA"}`,
		`{"Action":"pass","Package":"m/a","Test":"TestA","Elapsed":0}`,
		`{"Action":"output","Package":"m/a","Output":"ok  \tm/a\t(cached)\n"}`,
		`{"Action":"pass","Package":"m/a","Elapsed":0.01}`,
		`{"Action":"start","Package":"m/b"}`,
		`{"Action":"output","Package":"m/b","Output":"ok  \tm/b\t1.250s\n"}`,
		`{"Action":"pass","Package":"m/b","Elapsed":1.25}`,
		`{"Action":"output","Package":"m/c","Output":"FAIL\tm/c\t0.300s\n"}`,
		`{"Action":"fail","Package":"m/c","Elapsed":0.3}`,
	}, "\n")
	executions := goPackageExecutions(2, GoTestCacheAllowed, []byte(stream))
	if len(executions) != 3 {
		t.Fatalf("executions = %+v", executions)
	}
	if a := executions[0]; a.Package != "m/a" || a.Mode != PackageGoTestCache || a.ElapsedMS != nil || a.Shard != 2 || a.Reason != GoTestCacheAllowed {
		t.Fatalf("cached package = %+v", a)
	}
	if b := executions[1]; b.Mode != PackageExecuted || b.ElapsedMS == nil || *b.ElapsedMS != 1250 {
		t.Fatalf("executed package = %+v", b)
	}
	if c := executions[2]; c.Mode != PackageExecuted {
		t.Fatalf("failed package = %+v", c)
	}
}

func TestGoTestChildLaunchesFindsEveryLaunchForm(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	files := map[string]string{
		"pure/p_test.go":     "package pure\nimport \"testing\"\nfunc TestP(t *testing.T) {}\n",
		"execs/e_test.go":    "package execs\nimport (\"os/exec\"; \"testing\")\nfunc TestE(t *testing.T) { _ = exec.Command(\"bash\") }\n",
		"starts/s_test.go":   "package starts\nimport (\"os\"; \"testing\")\nfunc TestS(t *testing.T) { _, _ = os.StartProcess(\"x\", nil, nil) }\n",
		"helpers/h_test.go":  "package helpers\nimport (\"testing\"; \"example/internal/testutil\")\nfunc TestH(t *testing.T) { _ = testutil.InstalledWaitBinary(t, \"\") }\n",
		"syscalls/x_test.go": "package syscalls\nimport (\"syscall\"; \"testing\")\nfunc TestX(t *testing.T) { _ = syscall.Exec(\"x\", nil, nil) }\n",
	}
	for name, source := range files {
		writeTestResultFile(t, filepath.Join(root, filepath.FromSlash(name)), []byte(source), 0o644)
	}
	found, err := goTestChildLaunches(root, []string{"pure", "execs", "starts", "helpers", "syscalls"})
	want := []string{"execs/e_test.go", "helpers/h_test.go", "starts/s_test.go", "syscalls/x_test.go"}
	if err != nil || !slices.Equal(found, want) {
		t.Fatalf("child launches = %v %v, want %v", found, err, want)
	}
}

// cacheWitnessRun runs one group through the real group runner against a
// fresh candidate directory, as a per-run worktree, with real go and the
// inherited shared GOCACHE.
type cacheWitness struct {
	t        *testing.T
	root     string
	nonce    string
	files    map[string]string
	recorded map[string]string
}

func newCacheWitness(t *testing.T, files map[string]string) *cacheWitness {
	t.Helper()
	nonce := make([]byte, 8)
	if _, err := rand.Read(nonce); err != nil {
		t.Fatal(err)
	}
	return &cacheWitness{t: t, root: t.TempDir(), nonce: hex.EncodeToString(nonce), files: files, recorded: map[string]string{}}
}

func (w *cacheWitness) run(group testpolicy.Group, request TestRunRequest, extraEnv ...string) GroupResult {
	w.t.Helper()
	sources := map[string]testSnapshotEntry{
		"metasystem/go.mod": testSnapshotFile("module github.com/widoriezebos/agentic-tools/metasystem\n\ngo 1.22\n", 0o644),
	}
	for name, source := range w.files {
		// The nonce makes every test binary new to the shared cache; init
		// reads it, since an unreferenced value would not reach the binary.
		sources["metasystem/internal/"+name] = testSnapshotFile(source+"\nvar cacheWitnessNonce = \""+w.nonce+"\"\n\nfunc init() {\n\tif cacheWitnessNonce == \"\" {\n\t\tpanic(\"nonce\")\n\t}\n}\n", 0o644)
	}
	tree := hex.EncodeToString([]byte(w.nonce + "treeid.."))[:40]
	snapshot := newTestSnapshotFactory(w.t, filepath.Join(w.root, tree), tree, sources, 1)
	var packages []string
	for name := range w.files {
		if dir := "internal/" + filepath.Dir(name); !slices.Contains(packages, dir) {
			packages = append(packages, dir)
		}
	}
	slices.Sort(packages)
	group.Kind, group.Adapter, group.CWD, group.Packages, group.Tests = "unit", "go", "metasystem", packages, []byte(`"all"`)
	group.Inputs = []string{"metasystem/go.mod", "metasystem/internal/**"}
	group.Tools = []testpolicy.Tool{{ID: "go", Executable: "go", VersionArgs: []string{"version"}}}
	group.Obligations, group.Platforms, group.TargetMS = []string{"cache"}, []string{"any"}, 60000
	environment := []string{"GOFLAGS=-mod=readonly -buildvcs=false -trimpath"}
	for _, entry := range gittree.ScrubbedEnviron() {
		if !strings.HasPrefix(entry, "TMPDIR=") && !strings.HasPrefix(entry, "GOFLAGS=") {
			environment = append(environment, entry)
		}
	}
	request.ProjectRoot, request.InstallationPrefix, request.CandidateTree = snapshot.root, "metasystem", tree
	request.Environment = append(environment, extraEnv...)
	request.LogRoot = filepath.Join(snapshot.root, "logs")
	request.openCandidate = snapshot.open
	request.recordedExternalDigest = func(_, id string) string { return w.recorded[id] }
	if request.ResultSchemaVersion == 0 {
		request.ResultSchemaVersion = TestResultSchemaVersion
	}
	result := runTestGroup(context.Background(), request, group)
	if result.Status == "passed" && result.allExecuted() && result.ExternalInputsDigest != "" {
		w.recorded[group.ID] = result.ExternalInputsDigest
	}
	return result
}

func executionModes(result GroupResult) map[string]string {
	modes := map[string]string{}
	for _, execution := range result.Execution {
		name := filepath.Base(execution.Package)
		if previous, seen := modes[name]; seen && previous != execution.Mode {
			modes[name] = previous + "+" + execution.Mode
			continue
		}
		modes[name] = execution.Mode
	}
	return modes
}

const cacheWitnessPure = "package pure\n\nimport \"testing\"\n\nfunc TestPure(t *testing.T) {}\n"

// The hit witness: a package whose tests read no file and no variable runs
// twice through the real shard runner; the second launch is a replay whose
// time is unknown, and the group passes. The mixed witness: a sibling that
// reads a per-run variable executes again beside it.
func TestGoTestCacheHitAndMixedGroupThroughTheShardRunner(t *testing.T) {
	t.Parallel()
	witness := newCacheWitness(t, map[string]string{
		"pure/pure_test.go": cacheWitnessPure,
		"tmpreader/tmp_test.go": "package tmpreader\n\nimport (\"os\"; \"testing\")\n\n" +
			"func TestReadsTMPDIR(t *testing.T) { if os.Getenv(\"TMPDIR\") == \"\" { t.Fatal(\"unset\") } }\n",
	})
	group := testpolicy.Group{ID: "cache-mixed"}
	first := witness.run(group, TestRunRequest{Workers: 1}, "TMPDIR="+t.TempDir())
	second := witness.run(group, TestRunRequest{Workers: 1}, "TMPDIR="+t.TempDir())
	if first.Status != "passed" || second.Status != "passed" {
		t.Fatalf("statuses %s/%s: %s / %s", first.Status, second.Status, first.NotRunReason, second.NotRunReason)
	}
	if modes := executionModes(first); modes["pure"] != PackageExecuted || modes["tmpreader"] != PackageExecuted {
		t.Fatalf("first run modes = %v", modes)
	}
	if modes := executionModes(second); modes["pure"] != PackageGoTestCache || modes["tmpreader"] != PackageExecuted {
		t.Fatalf("second run modes = %v (%+v)", modes, second.Execution)
	}
	for _, execution := range second.Execution {
		if execution.Mode == PackageGoTestCache && execution.ElapsedMS != nil || execution.Reason != GoTestCacheAllowed {
			t.Fatalf("cached execution record = %+v", execution)
		}
	}
	if !second.NativeLaunched || second.PassedByGoTestCache() || !second.GoTestCacheReplayed() {
		t.Fatalf("a mixed group reads as launched, partly replayed, not a pass by cache: %+v", second)
	}
}

// A fresh request (episode freshness, --no-reuse, a cadence run) and a
// coverage group execute on every launch; a coverage shard's argv says so.
func TestGoTestCacheFreshAndCoverageGroupsAlwaysExecute(t *testing.T) {
	t.Parallel()
	witness := newCacheWitness(t, map[string]string{"pure/pure_test.go": cacheWitnessPure})
	fresh := TestRunRequest{Workers: 1, FreshGroups: map[string]bool{"cache-fresh": true}}
	for run := 0; run < 2; run++ {
		result := witness.run(testpolicy.Group{ID: "cache-fresh"}, fresh)
		if modes := executionModes(result); result.Status != "passed" || modes["pure"] != PackageExecuted || result.Execution[0].Reason != GoTestCountOneFresh {
			t.Fatalf("fresh run %d = %s %v %+v", run, result.Status, modes, result.Execution)
		}
	}
	// -count=1 also keeps go test from storing, so the next ordinary run
	// executes and stores, and only the one after it replays.
	for run, want := range []string{PackageExecuted, PackageGoTestCache} {
		ordinary := witness.run(testpolicy.Group{ID: "cache-fresh"}, TestRunRequest{Workers: 1})
		if modes := executionModes(ordinary); modes["pure"] != want {
			t.Fatalf("ordinary run %d after fresh runs = %v, want %s", run+1, modes, want)
		}
	}
	if countOne, reason := reusePolicy(TestRunRequest{Workers: 1, ResultSchemaVersion: TestResultSchemaVersion}, testpolicy.Group{Adapter: "go", Coverage: true}, 1, goCacheFacts{}); !countOne ||
		!slices.Contains(goNativeTestArguments(testpolicy.Group{Coverage: true}, true, countOne), "-count=1") || reason != GoTestCountOneCoverage {
		t.Fatalf("coverage shard policy = %t %q", countOne, reason)
	}
}

// Declared external inputs are the engine's own equivalence: the first run
// executes (unrecorded), an equal digest lets Go decide, a changed input
// executes again with the new digest recorded.
func TestGoTestCacheExternalInputsForceExecutionWhenChanged(t *testing.T) {
	t.Parallel()
	witness := newCacheWitness(t, map[string]string{"pure/pure_test.go": cacheWitnessPure})
	external := filepath.Join(t.TempDir(), "external.txt")
	if err := os.WriteFile(external, []byte("one"), 0o600); err != nil {
		t.Fatal(err)
	}
	group := testpolicy.Group{ID: "cache-external", ExternalInputs: []testpolicy.ExternalInput{{ID: "ext", Path: external}}}
	// A -count=1 launch stores nothing in Go's cache, so the first equal
	// digest executes and stores and the next one replays.
	first := witness.run(group, TestRunRequest{Workers: 1})
	storing := witness.run(group, TestRunRequest{Workers: 1})
	second := witness.run(group, TestRunRequest{Workers: 1})
	if err := os.WriteFile(external, []byte("two"), 0o600); err != nil {
		t.Fatal(err)
	}
	third := witness.run(group, TestRunRequest{Workers: 1})
	for index, want := range []struct {
		result GroupResult
		mode   string
		reason string
	}{{first, PackageExecuted, GoTestCountOneUnrecorded}, {storing, PackageExecuted, GoTestCacheAllowed},
		{second, PackageGoTestCache, GoTestCacheAllowed}, {third, PackageExecuted, GoTestCountOneExternal}} {
		if want.result.Status != "passed" || len(want.result.Execution) == 0 || want.result.Execution[0].Mode != want.mode || want.result.Execution[0].Reason != want.reason {
			t.Fatalf("run %d = %s %+v, want %s/%s", index+1, want.result.Status, want.result.Execution, want.mode, want.reason)
		}
	}
	if first.ExternalInputsDigest == "" || first.ExternalInputsDigest != second.ExternalInputsDigest || third.ExternalInputsDigest == first.ExternalInputsDigest {
		t.Fatalf("external digests %q %q %q", first.ExternalInputsDigest, second.ExternalInputsDigest, third.ExternalInputsDigest)
	}
}

// A group whose tests start children, with no declared external inputs,
// executes every time with the reason naming why.
func TestGoTestCacheUntrackedChildInputsExecute(t *testing.T) {
	t.Parallel()
	witness := newCacheWitness(t, map[string]string{"child/child_test.go": "package child\n\nimport (\"os/exec\"; \"testing\")\n\n" +
		"func TestChild(t *testing.T) { _ = exec.Command(\"true\") }\n"})
	for run := 0; run < 2; run++ {
		result := witness.run(testpolicy.Group{ID: "cache-child"}, TestRunRequest{Workers: 1})
		if result.Status != "passed" || len(result.Execution) == 0 || result.Execution[0].Mode != PackageExecuted || result.Execution[0].Reason != GoTestCountOneUntrackedChild {
			t.Fatalf("child run %d = %s %+v", run, result.Status, result.Execution)
		}
	}
}

func TestEngineRetainedAndCachedPassReadings(t *testing.T) {
	t.Parallel()
	ms := int64(5)
	group := GroupResult{Status: "passed", Execution: []PackageExecution{{Package: "a", Mode: PackageGoTestCache}, {Package: "b", Mode: PackageGoTestCache}}}
	if !group.PassedByGoTestCache() || !group.GoTestCacheReplayed() {
		t.Fatalf("a wholly cached pass reads otherwise: %+v", group)
	}
	group.Execution[1] = PackageExecution{Package: "b", Mode: PackageExecuted, ElapsedMS: &ms}
	if group.PassedByGoTestCache() || group.allExecuted() {
		t.Fatalf("a mixed pass reads as cached or executed: %+v", group)
	}
	group.markEngineRetained()
	for _, execution := range group.Execution {
		if execution.Mode != PackageEngineRetained {
			t.Fatalf("retained = %+v", group.Execution)
		}
	}
	if group.Execution[1].ElapsedMS == nil || *group.Execution[1].ElapsedMS != 5 || group.Execution[0].ElapsedMS != nil {
		t.Fatalf("retention changed the original measurements: %+v", group.Execution)
	}
	// A record from before Execution existed executed everything (-count=1).
	if legacy := (GroupResult{Status: "passed"}); !legacy.allExecuted() || legacy.GoTestCacheReplayed() || legacy.PassedByGoTestCache() {
		t.Fatalf("legacy record reads as not executed: %+v", legacy)
	}
	// A fresh consumer never takes credit from a replaying source.
	sources := []GroupResult{{ID: "s", Execution: []PackageExecution{{Mode: PackageGoTestCache}}}, {ID: "t"}}
	if kept := slices.DeleteFunc(slices.Clone(sources), GroupResult.GoTestCacheReplayed); len(kept) != 1 || kept[0].ID != "t" {
		t.Fatalf("fresh credit sources = %+v", kept)
	}
}

// Static witness over the committed contract: every go group whose test
// files start children either declares external inputs or is pinned to
// -count=1 by reusePolicy, so a new undeclared child launch can never serve
// a stale cached pass. The disposition of every group is logged.
func TestEveryGoGroupWithUntrackedChildInputsIsPinned(t *testing.T) {
	t.Parallel()
	moduleRoot := filepath.Join("..", "..")
	contract, err := testpolicy.Load(filepath.Join(moduleRoot, "testing.json"))
	if err != nil {
		t.Fatal(err)
	}
	pinned, cacheable := 0, 0
	for _, group := range contract.Groups {
		if group.Adapter != "go" || group.PackageSelection != "" {
			// A selection group's packages are scanned at launch.
			continue
		}
		var packages []string
		for _, pkg := range group.Packages {
			packages = append(packages, strings.TrimPrefix(pkg, "./"))
		}
		launches, err := goTestChildLaunches(moduleRoot, packages)
		if err != nil {
			t.Fatal(err)
		}
		facts := goCacheFacts{ChildLaunches: launches}
		countOne, reason := reusePolicy(TestRunRequest{Workers: 1, ResultSchemaVersion: TestResultSchemaVersion}, group, 1, facts)
		if len(launches) != 0 && len(group.ExternalInputs) == 0 && !countOne {
			t.Errorf("group %s starts children (%v) without declared external inputs but reusePolicy says %t %q", group.ID, launches, countOne, reason)
		}
		if countOne {
			pinned += len(packages)
		} else {
			cacheable += len(packages)
		}
		t.Logf("group %s: packages=%d child-launch files=%d countOne=%t reason=%s", group.ID, len(packages), len(launches), countOne, reason)
	}
	t.Logf("package memberships: %d pinned to -count=1, %d left to Go's cache", pinned, cacheable)
}

// A frontend that asked for schema 3 receives no execution record, and a
// schema-3 result carrying one does not validate.
func TestSchemaThreeResultsCarryNoExecutionRecord(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		request TestRunRequest
		want    int
	}{{TestRunRequest{}, PreviousTestResultSchemaVersion}, {TestRunRequest{Workers: 1}, WorkerPolicyTestResultSchemaVersion},
		{TestRunRequest{Workers: 1, ResultSchemaVersion: TestResultSchemaVersion}, TestResultSchemaVersion}} {
		if got := row.request.ResultSchema(); got != row.want {
			t.Fatalf("ResultSchema(%+v) = %d, want %d", row.request, got, row.want)
		}
	}
	result := TestResult{SchemaVersion: WorkerPolicyTestResultSchemaVersion, Groups: []GroupResult{{ID: "g",
		Execution: []PackageExecution{{Package: "p", Mode: PackageGoTestCache}}, ExternalInputsDigest: "d",
		Reruns: []RerunFinding{{Reason: GoTestCountOneRerun}}}}}
	stripExecutionRecord(&result)
	if group := result.Groups[0]; group.Execution != nil || group.ExternalInputsDigest != "" || group.Reruns[0].Reason != "" {
		t.Fatalf("stripped group = %+v", group)
	}
	if !slices.Equal(TestResultSchemaVersions, []int{WorkerPolicyTestResultSchemaVersion, TestResultSchemaVersion}) {
		t.Fatalf("written schemas = %v", TestResultSchemaVersions)
	}
}
