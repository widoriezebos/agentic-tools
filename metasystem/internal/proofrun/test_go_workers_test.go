package proofrun

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

func TestGoNativeArgumentsPinEveryNestedParallelismLayer(t *testing.T) {
	t.Parallel()
	group := testpolicy.Group{Race: true, Coverage: true, BuildTags: []string{"one", "two"}}
	arguments := goNativeTestArguments(group, true)
	for _, required := range []string{"-p=1", "-parallel=1", "-race", "-cover", "one,two"} {
		if !slices.Contains(arguments, required) {
			t.Fatalf("native Go arguments omit %s: %v", required, arguments)
		}
	}
	if slices.Contains(goNativeTestArguments(group, false), "-cover") {
		t.Fatalf("diagnostic rerun unexpectedly contributes coverage: %v", goNativeTestArguments(group, false))
	}
}

func TestGoGateNativeFailurePrintsCompleteLogBeforeLocalEvidenceMove(t *testing.T) {
	t.Parallel()
	source, err := os.ReadFile(filepath.Join("..", "..", "scripts", "agents", "go-gate.sh"))
	if err != nil {
		t.Fatal(err)
	}
	complete := bytes.Index(source, []byte("cat \"$coverage_log\" >&2"))
	move := bytes.Index(source, []byte("mv \"$coverage_log\" \"$keep\""))
	if complete < 0 || move < 0 || complete > move {
		t.Fatalf("native failure output is not copied to retained parent evidence before local move")
	}
}

func TestGoNativePlainOutputPreservesDiagnosticLargerThanScannerLimit(t *testing.T) {
	t.Parallel()
	diagnostic := strings.Repeat("native-diagnostic-", 8192)
	encoded, err := json.Marshal(goEvent{Action: "output", Package: "example.invalid/pkg", Output: diagnostic})
	if err != nil {
		t.Fatal(err)
	}
	encoded = append(encoded, '\n')
	if output := string(goNativePlainOutput(encoded)); output != diagnostic {
		t.Fatalf("large native diagnostic length=%d want=%d", len(output), len(diagnostic))
	}
}

func TestGoPartitionsKeepDuplicateNamesAndBuildOnlyPackagesComplete(t *testing.T) {
	t.Parallel()
	expected := []NativeTestIdentity{
		{Classname: "example.invalid/mod/a", Name: "TestSame"},
		{Classname: "example.invalid/mod/b", Name: "TestSame"},
		{Classname: "example.invalid/mod/a", Name: "TestOnlyA"},
		{Classname: "example.invalid/mod/c", Name: goPackageBuildIdentity},
	}
	inventory := []string{"a", "b", "c"}
	first := partitionGoTests(expected, inventory, "example.invalid/mod/", 4)
	second := partitionGoTests(expected, inventory, "example.invalid/mod/", 4)
	if !reflect.DeepEqual(first, second) || len(first) != 3 {
		t.Fatalf("partitions are not deterministic and capacity-bounded: first=%v second=%v", first, second)
	}
	var duplicate, buildOnly bool
	for _, partition := range first {
		if slices.Contains(partition.Names, "TestSame") {
			duplicate = slices.Contains(partition.Packages, "example.invalid/mod/a") &&
				slices.Contains(partition.Packages, "example.invalid/mod/b")
		}
		if slices.Contains(partition.Packages, "example.invalid/mod/c") {
			buildOnly = len(partition.Names) == 0
		}
	}
	if !duplicate || !buildOnly {
		t.Fatalf("duplicate-name or build-only ownership was split: %v", first)
	}
	if one := partitionGoTests(expected, inventory, "example.invalid/mod/", 1); len(one) != 1 || len(one[0].Packages) != 3 {
		t.Fatalf("worker one did not retain the complete selection: %v", one)
	}
}

func TestAutomaticGoPartitionsRunNamedSelectionsAtWorkerOneAndTwo(t *testing.T) {
	t.Parallel()
	root, tree := goWorkerNamedFixture(t)
	group := testpolicy.Group{ID: "named-workers", Kind: "unit", Adapter: "go", CWD: ".",
		Inputs: []string{"go.mod", "a/**", "b/**"}, Tools: []testpolicy.Tool{{ID: "go", Executable: "go", VersionArgs: []string{"version"}}},
		Obligations: []string{"named-workers"}, Platforms: []string{"any"}, TargetMS: 1000,
		Packages: []string{"a", "b"}, Tests: []byte(`["TestSame","TestOnlyA"]`)}
	for _, workers := range []int{1, 2} {
		logRoot := filepath.Join(root, "logs", strconv.Itoa(workers))
		request := TestRunRequest{ProjectRoot: root, CandidateTree: tree, Workers: workers, LogRoot: logRoot,
			Environment: append(gittree.ScrubbedEnviron(), "GOFLAGS=-buildvcs=false")}
		request.Contract.SchemaVersion = testpolicy.ExecutionContractSchemaVersion
		result := runTestGroup(context.Background(), request, group)
		if result.Status != "passed" || !result.CollectionComplete || len(result.Observed) != 3 || len(result.Missing) != 0 || len(result.Unexpected) != 0 {
			t.Fatalf("workers=%d named result=%+v", workers, result)
		}
		seen := map[string]int{}
		for _, identity := range result.Observed {
			seen[identity.Classname+"/"+identity.Name]++
			if identity.Name == "TestOnlyB" {
				t.Fatalf("workers=%d ran an unselected test: %+v", workers, result.Observed)
			}
		}
		for _, identity := range []string{
			"example.invalid/workers/a/TestOnlyA",
			"example.invalid/workers/a/TestSame",
			"example.invalid/workers/b/TestSame",
		} {
			if seen[identity] != 1 {
				t.Fatalf("workers=%d identity %s count=%d observed=%+v", workers, identity, seen[identity], result.Observed)
			}
		}
		for shard := 1; shard <= workers; shard++ {
			if _, err := os.Stat(filepath.Join(logRoot, "named-workers.shard-"+strconv.Itoa(shard)+".log")); err != nil {
				t.Fatalf("workers=%d shard=%d log: %v", workers, shard, err)
			}
		}
		if _, err := os.Stat(filepath.Join(logRoot, "named-workers.shard-"+strconv.Itoa(workers+1)+".log")); !os.IsNotExist(err) {
			t.Fatalf("workers=%d exceeded its partition ceiling: %v", workers, err)
		}
	}
}

func TestAutomaticGoPartitionsRunTestMainAndNoTestPackageOnce(t *testing.T) {
	t.Parallel()
	root, _ := goWorkerNamedFixture(t)
	marker := filepath.Join(t.TempDir(), "testmain-ran")
	writeTestResultFile(t, filepath.Join(root, "empty", "empty.go"), []byte("package empty\nconst Value = 1\n"), 0o644)
	writeTestResultFile(t, filepath.Join(root, "empty", "empty_test.go"), []byte(`package empty
import (
 "os"
 "testing"
)
func TestMain(m *testing.M) {
 if err := os.WriteFile(os.Getenv("TESTMAIN_MARKER"), []byte("ran"), 0600); err != nil { os.Exit(97) }
 os.Exit(m.Run())
}
`), 0o644)
	runTestResultGit(t, root, "add", "empty")
	runTestResultGit(t, root, "commit", "-qm", "add empty package")
	tree := runTestResultGit(t, root, "rev-parse", "HEAD^{tree}")
	group := testpolicy.Group{ID: "whole-with-testmain", Kind: "unit", Adapter: "go", CWD: ".",
		Inputs: []string{"go.mod", "a/**", "empty/**"}, Tools: []testpolicy.Tool{{ID: "go", Executable: "go", VersionArgs: []string{"version"}}},
		Obligations: []string{"whole-workers"}, Platforms: []string{"any"}, TargetMS: 1000,
		Packages: []string{"a", "empty"}, Tests: []byte(`"all"`), Env: map[string]string{"TESTMAIN_MARKER": marker}}
	request := TestRunRequest{ProjectRoot: root, CandidateTree: tree, Workers: 2, LogRoot: filepath.Join(root, "whole-logs"),
		Environment: append(gittree.ScrubbedEnviron(), "GOFLAGS=-buildvcs=false")}
	request.Contract.SchemaVersion = testpolicy.ExecutionContractSchemaVersion
	result := runTestGroup(context.Background(), request, group)
	if result.Status != "passed" || !result.CollectionComplete {
		t.Fatalf("whole selection result=%+v", result)
	}
	buildTerminals := 0
	for _, identity := range result.Observed {
		if identity.Classname == "example.invalid/workers/empty" && identity.Name == goPackageBuildIdentity && identity.Status == "passed" {
			buildTerminals++
		}
	}
	if buildTerminals != 1 {
		t.Fatalf("TestMain package build terminals=%d observed=%+v", buildTerminals, result.Observed)
	}
	if data, err := os.ReadFile(marker); err != nil || string(data) != "ran" {
		t.Fatalf("TestMain did not run: data=%q err=%v", data, err)
	}
	if err := os.Remove(marker); err != nil {
		t.Fatal(err)
	}
	group.ID, group.Tests = "named-with-testmain", []byte(`["TestOnlyA"]`)
	request.LogRoot = filepath.Join(root, "named-testmain-logs")
	named := runTestGroup(context.Background(), request, group)
	if named.Status != "passed" || !named.CollectionComplete || len(named.Observed) != 2 {
		t.Fatalf("named selection lost an unselected package: %+v", named)
	}
	if data, err := os.ReadFile(marker); err != nil || string(data) != "ran" {
		t.Fatalf("named selection did not run TestMain: data=%q err=%v", data, err)
	}
}

func TestSchemaOnePartitionsNeverDropDeclaredPackages(t *testing.T) {
	t.Parallel()
	root, _ := goWorkerNamedFixture(t)
	marker := filepath.Join(t.TempDir(), "testmain-diagnostic")
	for path, source := range map[string]string{
		"mainonly/mainonly.go": "package mainonly\nconst Value = 1\n",
		"mainonly/mainonly_test.go": `package mainonly
import (
 "fmt"
 "os"
 "testing"
)
func TestMain(m *testing.M) {
 fmt.Fprintln(os.Stderr, "schema-one TestMain diagnostic")
 if err := os.WriteFile(os.Getenv("TESTMAIN_MARKER"), []byte("ran"), 0600); err != nil { os.Exit(97) }
 os.Exit(23)
}
`,
		"notests/notests.go": "package notests\nconst Value = 1\n",
		"broken/broken.go":   "package broken\nvar Value = doesNotCompile\n",
	} {
		writeTestResultFile(t, filepath.Join(root, filepath.FromSlash(path)), []byte(source), 0o644)
	}
	runTestResultGit(t, root, "add", ".")
	runTestResultGit(t, root, "commit", "-qm", "schema one package inventory")
	tree := runTestResultGit(t, root, "rev-parse", "HEAD^{tree}")
	environment := append(gittree.ScrubbedEnviron(), "GOFLAGS=-buildvcs=false")
	for _, workers := range []int{1, 2} {
		for _, selection := range []struct {
			name  string
			tests []byte
		}{
			{name: "whole", tests: []byte(`"all"`)},
			{name: "named", tests: []byte(`["TestOnlyA"]`)},
		} {
			t.Run(fmt.Sprintf("testmain-%s-workers-%d", selection.name, workers), func(t *testing.T) {
				_ = os.Remove(marker)
				group := testpolicy.Group{ID: "schema-one-testmain", Kind: "unit", Adapter: "go", CWD: ".",
					Inputs:      []string{"go.mod", "a/**", "mainonly/**", "notests/**"},
					Tools:       []testpolicy.Tool{{ID: "go", Executable: "go", VersionArgs: []string{"version"}}},
					Obligations: []string{"schema-one"}, Platforms: []string{"any"}, TargetMS: 1000,
					Packages: []string{"a", "mainonly", "notests"}, Tests: selection.tests,
					Env: map[string]string{"TESTMAIN_MARKER": marker}}
				request := TestRunRequest{ProjectRoot: root, CandidateTree: tree, Workers: workers,
					LogRoot: filepath.Join(root, "schema-one", selection.name, strconv.Itoa(workers)), Environment: environment}
				request.Contract.SchemaVersion = 1
				result := runTestGroup(context.Background(), request, group)
				if result.Status != "failed" || result.NativeExitStatus == nil || *result.NativeExitStatus == 0 {
					t.Fatalf("schema-one TestMain package vanished: %+v", result)
				}
				if data, err := os.ReadFile(marker); err != nil || string(data) != "ran" {
					t.Fatalf("schema-one TestMain did not execute: data=%q err=%v result=%+v", data, err, result)
				}
				log, err := os.ReadFile(result.LogPath)
				if err != nil || !strings.Contains(string(log), "example.invalid/workers/notests") {
					t.Fatalf("schema-one no-test package vanished: err=%v\n%s", err, log)
				}
			})

			t.Run(fmt.Sprintf("type-error-%s-workers-%d", selection.name, workers), func(t *testing.T) {
				group := testpolicy.Group{ID: "schema-one-type-error", Kind: "unit", Adapter: "go", CWD: ".",
					Inputs:      []string{"go.mod", "a/**", "broken/**"},
					Tools:       []testpolicy.Tool{{ID: "go", Executable: "go", VersionArgs: []string{"version"}}},
					Obligations: []string{"schema-one"}, Platforms: []string{"any"}, TargetMS: 1000,
					Packages: []string{"a", "broken"}, Tests: selection.tests}
				request := TestRunRequest{ProjectRoot: root, CandidateTree: tree, Workers: workers,
					LogRoot: filepath.Join(root, "schema-one-broken", selection.name, strconv.Itoa(workers)), Environment: environment}
				request.Contract.SchemaVersion = 1
				result := runTestGroup(context.Background(), request, group)
				if result.Status == "passed" || !strings.Contains(result.NotRunReason+readFileText(result.LogPath), "broken") {
					t.Fatalf("schema-one type-error package vanished: %+v", result)
				}
			})
		}
	}
}

func TestGoDiscoveryMatchesRaceAndUserBuildTags(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	runTestResultGit(t, root, "init", "-q", "-b", "main")
	runTestResultGit(t, root, "config", "user.name", "fixture")
	runTestResultGit(t, root, "config", "user.email", "fixture@example.invalid")
	for path, source := range map[string]string{
		"go.mod":                "module example.invalid/conditions\n\ngo 1.27\n",
		"pkg/pkg.go":            "package pkg\n",
		"pkg/ordinary_test.go":  "package pkg\nimport \"testing\"\nfunc TestOrdinary(t *testing.T) {}\n",
		"pkg/race_test.go":      "//go:build race\n\npackage pkg\nimport \"testing\"\nfunc TestRaceOnly(t *testing.T) { t.Fatal(\"race-only failure\") }\n",
		"pkg/nonrace_test.go":   "//go:build !race\n\npackage pkg\nimport \"testing\"\nfunc TestNonRace(t *testing.T) {}\n",
		"tagged/tagged.go":      "//go:build customtag\n\npackage tagged\n",
		"tagged/tagged_test.go": "//go:build customtag\n\npackage tagged\nimport \"testing\"\nfunc TestTagged(t *testing.T) {}\n",
	} {
		writeTestResultFile(t, filepath.Join(root, filepath.FromSlash(path)), []byte(source), 0o644)
	}
	runTestResultGit(t, root, "add", ".")
	runTestResultGit(t, root, "commit", "-qm", "native build conditions")
	tree := runTestResultGit(t, root, "rev-parse", "HEAD^{tree}")
	environment := append(gittree.ScrubbedEnviron(), "GOFLAGS=-buildvcs=false")
	cache := &goDiscoveryCache{catalogs: map[string]goPackageCatalog{}}
	identities := func(group testpolicy.Group) map[string]bool {
		t.Helper()
		_, expected, _, _, err := goArgumentsCachedForSchema(context.Background(), group, root, environment, cache, testpolicy.ExecutionContractSchemaVersion)
		if err != nil {
			t.Fatal(err)
		}
		found := map[string]bool{}
		for _, identity := range expected {
			found[identity.Classname+"/"+identity.Name] = true
		}
		return found
	}
	nonRace := testpolicy.Group{Adapter: "go", Packages: []string{"pkg"}, Tests: []byte(`"all"`)}
	race := nonRace
	race.Race = true
	nonRaceIDs, raceIDs := identities(nonRace), identities(race)
	if !nonRaceIDs["example.invalid/conditions/pkg/TestOrdinary"] || !nonRaceIDs["example.invalid/conditions/pkg/TestNonRace"] || nonRaceIDs["example.invalid/conditions/pkg/TestRaceOnly"] {
		t.Fatalf("non-race discovery inventory=%v", nonRaceIDs)
	}
	if !raceIDs["example.invalid/conditions/pkg/TestOrdinary"] || !raceIDs["example.invalid/conditions/pkg/TestRaceOnly"] || raceIDs["example.invalid/conditions/pkg/TestNonRace"] {
		t.Fatalf("race discovery reused the non-race inventory=%v", raceIDs)
	}
	tagged := testpolicy.Group{Adapter: "go", Packages: []string{"tagged"}, BuildTags: []string{"customtag"}, Tests: []byte(`"all"`)}
	if !identities(tagged)["example.invalid/conditions/tagged/TestTagged"] {
		t.Fatal("user-tag-only package was absent from discovery")
	}
	run := func(name string, group testpolicy.Group) GroupResult {
		group.ID, group.Kind = name, "unit"
		group.Inputs = []string{"go.mod", "pkg/**", "tagged/**"}
		group.Tools = []testpolicy.Tool{{ID: "go", Executable: "go", VersionArgs: []string{"version"}}}
		group.Obligations, group.Platforms, group.TargetMS = []string{"native-conditions"}, []string{"any"}, 1000
		request := TestRunRequest{ProjectRoot: root, CandidateTree: tree, Workers: 2,
			LogRoot: filepath.Join(root, "condition-logs", name), Environment: environment}
		request.Contract.SchemaVersion = testpolicy.ExecutionContractSchemaVersion
		return runTestGroup(context.Background(), request, group)
	}
	if result := run("ordinary", nonRace); result.Status != "passed" || !result.CollectionComplete {
		t.Fatalf("ordinary/!race execution=%+v", result)
	}
	if result := run("race", race); result.Status != "failed" || !nativeIdentityStatus(result.Observed, "TestRaceOnly", "failed") {
		t.Fatalf("race-only failure was not executed: %+v", result)
	}
	if result := run("tagged", tagged); result.Status != "passed" || !nativeIdentityStatus(result.Observed, "TestTagged", "passed") {
		t.Fatalf("tagged-only execution=%+v", result)
	}
}

func TestAutomaticGoPartitionsUseWorkerCapacityBetweenIsolatedProcesses(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	runTestResultGit(t, root, "init", "-q", "-b", "main")
	runTestResultGit(t, root, "config", "user.name", "fixture")
	runTestResultGit(t, root, "config", "user.email", "fixture@example.invalid")
	writeTestResultFile(t, filepath.Join(root, "go.mod"), []byte("module example.invalid/barrier\n\ngo 1.27\n"), 0o644)
	writeTestResultFile(t, filepath.Join(root, "pkg", "pkg.go"), []byte("package pkg\n"), 0o644)
	writeTestResultFile(t, filepath.Join(root, "pkg", "pkg_test.go"), []byte(`package pkg
import (
	"fmt"
 "os"
 "path/filepath"
 "testing"
 "time"
)
func barrier(t *testing.T, name string) {
 t.Helper()
 root := os.Getenv("BARRIER_ROOT")
 if err := os.WriteFile(filepath.Join(root, name+".ready"), []byte("ready"), 0600); err != nil { t.Fatal(err) }
	fmt.Println("barrier ready", name)
 for {
  if _, err := os.Stat(filepath.Join(root, "release")); err == nil { return } else if !os.IsNotExist(err) { t.Fatal(err) }
  time.Sleep(10 * time.Millisecond)
 }
}
func TestA(t *testing.T) { barrier(t, "a") }
func TestB(t *testing.T) { barrier(t, "b") }
func TestC(t *testing.T) { barrier(t, "c") }
func TestD(t *testing.T) { barrier(t, "d") }
`), 0o644)
	runTestResultGit(t, root, "add", ".")
	runTestResultGit(t, root, "commit", "-qm", "fixture")
	tree := runTestResultGit(t, root, "rev-parse", "HEAD^{tree}")
	for _, workers := range []int{2, 3} {
		t.Run(strconv.Itoa(workers), func(t *testing.T) {
			barrierRoot := t.TempDir()
			readyEvents := make([]chan struct{}, workers)
			for index := range readyEvents {
				readyEvents[index] = make(chan struct{})
			}
			group := testpolicy.Group{ID: "barrier-" + strconv.Itoa(workers), Kind: "unit", Adapter: "go", CWD: ".", Inputs: []string{"go.mod", "pkg/**"},
				Tools: []testpolicy.Tool{{ID: "go", Executable: "go", VersionArgs: []string{"version"}}}, Obligations: []string{"barrier"},
				Platforms: []string{"any"}, TargetMS: 1000, Packages: []string{"pkg"}, Tests: []byte(`"all"`),
				Env: map[string]string{"BARRIER_ROOT": barrierRoot}}
			request := TestRunRequest{ProjectRoot: root, CandidateTree: tree, Workers: workers,
				LogRoot: filepath.Join(root, "logs", strconv.Itoa(workers)), Environment: append(gittree.ScrubbedEnviron(), "GOFLAGS=-buildvcs=false")}
			request.Contract.SchemaVersion = testpolicy.ExecutionContractSchemaVersion
			ctx := context.WithValue(t.Context(), goShardLifecycleHooksKey{}, goShardLifecycleHooks{
				wrapOutput: func(index int, writer io.Writer) io.Writer {
					return newCollectedOutputWriter(writer, "barrier ready", readyEvents[index])
				},
			})
			ctx, cancel := context.WithCancel(ctx)
			done := make(chan GroupResult, 1)
			go func() { done <- runTestGroup(ctx, request, group) }()
			joined := false
			t.Cleanup(func() {
				cancel()
				if !joined {
					<-done
				}
			})
			ready := []string{"a", "b", "c"}[:workers]
			for index, readyEvent := range readyEvents {
				select {
				case <-readyEvent:
				case result := <-done:
					joined = true
					t.Fatalf("Go group ended before partition %d published readiness: %+v", index, result)
				case <-t.Context().Done():
					t.Fatalf("Go partition %d did not publish readiness before test cancellation: %v", index, context.Cause(t.Context()))
				}
			}
			for _, name := range ready {
				if !pathExists(filepath.Join(barrierRoot, name+".ready")) {
					t.Fatalf("ready partition %s did not publish its barrier file", name)
				}
			}
			if pathExists(filepath.Join(barrierRoot, "d.ready")) {
				t.Fatalf("more than %d isolated partitions passed the barrier: files=%v", workers, directoryNames(t, barrierRoot))
			}
			select {
			case result := <-done:
				t.Fatalf("group returned before the release event: %+v", result)
			default:
			}
			if err := os.WriteFile(filepath.Join(barrierRoot, "release"), []byte("release"), 0o600); err != nil {
				t.Fatal(err)
			}
			result := <-done
			joined = true
			cancel()
			if result.Status != "passed" || !result.CollectionComplete || len(result.Observed) != 4 {
				t.Fatalf("barrier result=%+v", result)
			}
		})
	}
}

func TestPerformanceGoPartitionsExportOneWorkerToNestedRunners(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	runTestResultGit(t, root, "init", "-q", "-b", "main")
	runTestResultGit(t, root, "config", "user.name", "fixture")
	runTestResultGit(t, root, "config", "user.email", "fixture@example.invalid")
	writeTestResultFile(t, filepath.Join(root, "go.mod"), []byte("module example.invalid/nested\n\ngo 1.27\n"), 0o644)
	writeTestResultFile(t, filepath.Join(root, "pkg", "pkg.go"), []byte("package pkg\n"), 0o644)
	writeTestResultFile(t, filepath.Join(root, "pkg", "pkg_test.go"), []byte(`package pkg
import (
 "fmt"
 "os"
 "path/filepath"
 "strconv"
 "testing"
 "time"
)
func nestedRunner(t *testing.T, leaf string) {
 t.Helper()
 workers, err := strconv.Atoi(os.Getenv("METASYSTEM_TEST_WORKERS"))
 if err != nil || workers < 1 { t.Fatalf("nested worker allowance=%q err=%v", os.Getenv("METASYSTEM_TEST_WORKERS"), err) }
 root := os.Getenv("NESTED_RUNNER_ROOT")
 for worker := 1; worker <= workers; worker++ {
  path := filepath.Join(root, leaf+"-worker-"+strconv.Itoa(worker)+".ready")
  if err := os.WriteFile(path, []byte("ready"), 0600); err != nil { t.Fatal(err) }
 }
 if err := os.WriteFile(filepath.Join(root, leaf+".ready"), []byte("ready"), 0600); err != nil { t.Fatal(err) }
 fmt.Println("nested runner ready", leaf)
 for {
  if _, err := os.Stat(filepath.Join(root, "release")); err == nil { return } else if !os.IsNotExist(err) { t.Fatal(err) }
  time.Sleep(10 * time.Millisecond)
 }
}
func TestA(t *testing.T) { nestedRunner(t, "a") }
func TestB(t *testing.T) { nestedRunner(t, "b") }
`), 0o644)
	runTestResultGit(t, root, "add", ".")
	runTestResultGit(t, root, "commit", "-qm", "fixture")
	tree := runTestResultGit(t, root, "rev-parse", "HEAD^{tree}")
	nestedRoot := t.TempDir()
	group := testpolicy.Group{ID: "performance-go-nested", Kind: "performance", Adapter: "go", CWD: ".",
		Inputs: []string{"go.mod", "pkg/**"}, Tools: []testpolicy.Tool{{ID: "go", Executable: "go", VersionArgs: []string{"version"}}},
		Obligations: []string{"nested"}, Platforms: []string{"any"}, TargetMS: 1000, Packages: []string{"pkg"}, Tests: []byte(`"all"`),
		Env: map[string]string{"NESTED_RUNNER_ROOT": nestedRoot}}
	request := TestRunRequest{ProjectRoot: root, CandidateTree: tree, Workers: 2,
		LogRoot: filepath.Join(root, "logs"), Environment: append(gittree.ScrubbedEnviron(), "GOFLAGS=-buildvcs=false")}
	request.Contract.SchemaVersion = testpolicy.ExecutionContractSchemaVersion
	ready := []chan struct{}{make(chan struct{}), make(chan struct{})}
	ctx := context.WithValue(t.Context(), goShardLifecycleHooksKey{}, goShardLifecycleHooks{
		wrapOutput: func(index int, writer io.Writer) io.Writer {
			return newCollectedOutputWriter(writer, "nested runner ready", ready[index])
		},
	})
	ctx, cancel := context.WithCancel(ctx)
	done := make(chan GroupResult, 1)
	go func() { done <- runTestGroup(ctx, request, group) }()
	joined := false
	var release sync.Once
	releaseLeaves := func() {
		release.Do(func() { _ = os.WriteFile(filepath.Join(nestedRoot, "release"), []byte("release"), 0o600) })
	}
	t.Cleanup(func() {
		releaseLeaves()
		cancel()
		if !joined {
			<-done
		}
	})
	for index, event := range ready {
		select {
		case <-event:
		case result := <-done:
			joined = true
			t.Fatalf("performance Go group ended before leaf %d published readiness: %+v", index, result)
		case <-t.Context().Done():
			t.Fatalf("performance Go leaf %d did not reach its nested runner: %v", index, context.Cause(t.Context()))
		}
	}
	entries, err := os.ReadDir(nestedRoot)
	if err != nil {
		t.Fatal(err)
	}
	workers := 0
	for _, entry := range entries {
		if strings.Contains(entry.Name(), "-worker-") {
			workers++
		}
	}
	if workers != 2 {
		t.Fatalf("two concurrent Go leaves propagated %d nested worker allowances, want 2: %v", workers, directoryNames(t, nestedRoot))
	}
	releaseLeaves()
	result := <-done
	joined = true
	cancel()
	if result.Status != "passed" || !result.CollectionComplete || len(result.Observed) != 2 {
		t.Fatalf("performance Go nested-runner result=%+v", result)
	}
}

func TestQueuedCancellationOccursAfterConfirmedWaitAndDrainsCounter(t *testing.T) {
	t.Parallel()
	ctx := withTestWorkerPool(t.Context(), 1)
	held, err := acquireTestWorkers(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	waiting, cancel := context.WithCancel(ctx)
	t.Cleanup(cancel)
	root := t.TempDir()
	type nativeResult struct {
		outcome supervisorOutcome
		err     error
	}
	done := make(chan nativeResult, 1)
	go func() {
		var output synchronizedBuffer
		outcome, _, _, launchErr := runShardedGoGroup(waiting, TestRunRequest{Workers: 1},
			testpolicy.Group{ID: "queued-native", Adapter: "go"}, root, []string{"PATH=/does/not/run"},
			[]NativeTestIdentity{{Classname: "example.invalid/queued/pkg", Name: "TestQueued"}}, []string{"pkg"},
			"example.invalid/queued/", supervisorLimits{}, time.Millisecond, filepath.Join(root, "queued.log"), &output)
		done <- nativeResult{outcome: outcome, err: launchErr}
	}()
	pool := ctx.Value(testWorkerPoolContextKey{}).(*testWorkerPool)
	waitForTestWorkerPoolWaiters(pool, 1)
	if available, _ := testWorkerPoolState(pool); available != 0 {
		t.Fatalf("queued native waiter observed available capacity: %d", available)
	}
	cancel()
	result := <-done
	if !errors.Is(result.err, context.Canceled) || result.outcome.Started {
		t.Fatalf("queued native cancellation outcome=%+v error=%v", result.outcome, result.err)
	}
	pool.mu.Lock()
	if pool.waiters != 0 || pool.available != 0 {
		pool.mu.Unlock()
		t.Fatalf("cancelled native owner leaked queued accounting before held capacity release")
	}
	pool.mu.Unlock()
	held()
	pool.mu.Lock()
	defer pool.mu.Unlock()
	if pool.waiters != 0 || pool.available != 1 {
		t.Fatalf("cancelled queue leaked accounting: waiters=%d available=%d", pool.waiters, pool.available)
	}
}

func TestSupervisedGoStartFailureReleasesWorker(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	bin := filepath.Join(root, "bin")
	if err := os.MkdirAll(bin, 0o700); err != nil {
		t.Fatal(err)
	}
	goExecutable, err := exec.LookPath("go")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(goExecutable, filepath.Join(bin, "go")); err != nil {
		t.Fatal(err)
	}
	missingWorkingDirectory := filepath.Join(root, "missing-command-working-directory")
	if _, err := os.Stat(filepath.Join(bin, "go")); err != nil {
		t.Fatalf("explicit PATH executable is invalid: %v", err)
	}
	if _, err := os.Stat(missingWorkingDirectory); !os.IsNotExist(err) {
		t.Fatalf("command working directory unexpectedly exists: %v", err)
	}
	logPath := filepath.Join(root, "failure.log")
	ctx := withTestWorkerPool(context.Background(), 1)
	var output synchronizedBuffer
	outcome, _, _, launchErr := runShardedGoGroup(ctx, TestRunRequest{Workers: 1}, testpolicy.Group{ID: "start-failure", Adapter: "go"},
		missingWorkingDirectory, []string{"PATH=" + bin}, []NativeTestIdentity{{Classname: "example.invalid/failure/pkg", Name: "TestOne"}},
		[]string{"pkg"}, "example.invalid/failure/", supervisorLimits{}, time.Second, logPath, &output)
	if launchErr == nil || !strings.Contains(launchErr.Error(), "start Go test shard") ||
		!errors.Is(launchErr, os.ErrNotExist) || outcome.Started || outcome.WaitErr == nil {
		t.Fatalf("supervised native command.Start failure outcome=%+v error=%v", outcome, launchErr)
	}
	if _, err := os.Stat(logPath); err != nil {
		t.Fatalf("valid shard log was not created before command.Start: %v", err)
	}
	assertWorkerPoolFullyAvailable(t, ctx, 1)
}

func TestSupervisedGoExecFailureIsTerminalAndReleasesWorker(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	badBin := filepath.Join(root, "bin")
	if err := os.MkdirAll(badBin, 0o700); err != nil {
		t.Fatal(err)
	}
	badGo := filepath.Join(badBin, "go")
	writeTestResultFile(t, badGo, []byte("#!/definitely/missing/interpreter\n"), 0o700)
	ctx := withTestWorkerPool(WithResourceCustodyExecutable(t.Context(), buildResourceCustodyEngine(t)), 1)
	logPath := filepath.Join(root, "failure.log")
	var output synchronizedBuffer
	outcome, closeErr, _, launchErr := runShardedGoGroup(ctx, TestRunRequest{Workers: 1}, testpolicy.Group{ID: "exec-failure", Adapter: "go"},
		root, []string{"PATH=" + badBin}, []NativeTestIdentity{{Classname: "example.invalid/failure/pkg", Name: "TestOne"}},
		[]string{"pkg"}, "example.invalid/failure/", supervisorLimits{}, time.Second, logPath, &output)
	var exitErr *exec.ExitError
	if launchErr != nil || closeErr != nil || !outcome.Started || outcome.WaitErr == nil ||
		!errors.As(outcome.WaitErr, &exitErr) || exitErr.ExitCode() == 0 {
		t.Fatalf("supervised native exec failure outcome=%+v close=%v launch=%v", outcome, closeErr, launchErr)
	}
	diagnostic := output.String()
	if !strings.Contains(diagnostic, "proof-run custody-exec: no such file or directory") {
		t.Fatalf("supervised native exec failure omitted its diagnostic:\n%s", diagnostic)
	}
	if retained := readFileText(logPath); !strings.Contains(retained, "proof-run custody-exec: no such file or directory") {
		t.Fatalf("retained native exec failure omitted its diagnostic:\n%s", retained)
	}
	assertWorkerPoolFullyAvailable(t, ctx, 1)
}

func TestSupervisorOutcomeAssignmentDoesNotInventUnobservedExit(t *testing.T) {
	t.Parallel()
	result := GroupResult{ProgressRule: "base"}
	assignSupervisorOutcome(&result, supervisorOutcome{CPUSeconds: 1.25, LongestSilentSeconds: 2,
		LongestZeroCPUSeconds: 3, WaitErr: errors.New("start was not observed"), RuleSuffix: "native"})
	if result.NativeLaunched || result.NativeExitStatus != nil || result.Status != "unavailable" ||
		result.CPUSeconds != 1.25 || result.LongestSilentSeconds != 2 || result.LongestZeroCPUSeconds != 3 ||
		result.ProgressRule != "base+native" {
		t.Fatalf("unobserved native outcome was misrepresented: %+v", result)
	}
	started := GroupResult{ProgressRule: "base"}
	assignSupervisorOutcome(&started, supervisorOutcome{Started: true, WaitErr: errors.New("wait status unavailable")})
	if !started.NativeLaunched || started.NativeExitStatus != nil || started.Status != "unavailable" {
		t.Fatalf("started native outcome lost launch or invented exit: %+v", started)
	}
}

func TestGoPlanCountsStartedShardWhenLaterCoverageSetupFails(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	runTestResultGit(t, root, "init", "-q", "-b", "main")
	runTestResultGit(t, root, "config", "user.name", "fixture")
	runTestResultGit(t, root, "config", "user.email", "fixture@example.invalid")
	const diagnostic = "plan accounting diagnostic reached the parent collector"
	writeTestResultFile(t, filepath.Join(root, "go.mod"), []byte("module example.invalid/accounting\n\ngo 1.27\n"), 0o644)
	for _, pkg := range []string{"a", "b"} {
		writeTestResultFile(t, filepath.Join(root, pkg, pkg+".go"), []byte("package "+pkg+"\n"), 0o644)
		writeTestResultFile(t, filepath.Join(root, pkg, pkg+"_test.go"), []byte(`package `+pkg+`
import ("fmt"; "os"; "testing")
func TestDiagnostic`+strings.ToUpper(pkg)+`(t *testing.T) {
 fmt.Fprintln(os.Stderr, "`+diagnostic+`")
 select {}
}
`), 0o644)
	}
	runTestResultGit(t, root, "add", ".")
	runTestResultGit(t, root, "commit", "-qm", "fixture")
	tree := runTestResultGit(t, root, "rev-parse", "HEAD^{tree}")
	collected := make(chan struct{})
	hooks := goShardLifecycleHooks{
		prepareCoverage: func(index int, directory, groupID string) error {
			if index == 1 {
				<-collected
				return errors.New("injected plan coverage setup failure")
			}
			if err := os.MkdirAll(directory, 0o700); err != nil {
				return err
			}
			return os.WriteFile(filepath.Join(directory, ".pending"), []byte(groupID+"\n"), 0o600)
		},
		wrapOutput: func(index int, writer io.Writer) io.Writer {
			if index == 0 {
				return newCollectedOutputWriter(writer, diagnostic, collected)
			}
			return writer
		},
	}
	group := testpolicy.Group{ID: "accounting", Kind: "unit", Adapter: "go", CWD: ".", Coverage: true,
		Inputs: []string{"go.mod", "a/**", "b/**"}, Tools: []testpolicy.Tool{{ID: "go", Executable: "go", VersionArgs: []string{"version"}}},
		Obligations: []string{"accounting"}, Platforms: []string{"any"}, TargetMS: 1000,
		Packages: []string{"a", "b"}, Tests: []byte(`"all"`)}
	contract := testpolicy.Contract{SchemaVersion: 1, Groups: []testpolicy.Group{group}}
	plan := testpolicy.Plan{Purpose: testpolicy.PurposeDiagnostic, RequiredGroups: []string{group.ID}, SelectedGroups: []string{group.ID},
		Stages: []testpolicy.Stage{{ID: "standard", Groups: []string{group.ID}}}}
	ctx := context.WithValue(t.Context(), goShardLifecycleHooksKey{}, hooks)
	result, status, err := RunTestPlan(ctx, TestRunRequest{ProjectRoot: root, CandidateTree: tree, Contract: contract, Plan: plan,
		Workers: 2, LogRoot: filepath.Join(root, "logs"), Environment: append(gittree.ScrubbedEnviron(), "GOFLAGS=-buildvcs=false")})
	if err != nil || status == 0 || len(result.Groups) != 1 {
		t.Fatalf("plan setup failure status=%d error=%v result=%+v", status, err, result)
	}
	actual := result.Groups[0]
	if actual.Status != "unavailable" || !strings.Contains(actual.NotRunReason, "injected plan coverage setup failure") ||
		!actual.NativeLaunched || actual.NativeExitStatus == nil || result.LaunchCounts.Test != 1 {
		t.Fatalf("started native accounting was lost: counts=%+v group=%+v", result.LaunchCounts, actual)
	}
	if log := readFileText(actual.LogPath); !strings.Contains(string(goNativePlainOutput([]byte(log))), diagnostic) {
		t.Fatalf("plan setup failure log lost collected diagnostic:\n%s", log)
	}
}

func TestLaterCoverageSetupFailureCancelsAndJoinsStartedShard(t *testing.T) {
	t.Parallel()
	if runGoWorkerTestInOwnedProcess(t) {
		return
	}
	testActiveShardFailureDrain(t, "coverage setup", true)
	testPublicGoGateCoverageSetupFailureKeepsOutput(t)
}

func TestOutputFailureCancelsAndJoinsActiveShard(t *testing.T) {
	t.Parallel()
	if runGoWorkerTestInOwnedProcess(t) {
		return
	}
	testActiveShardFailureDrain(t, "output", false)
}

func TestGoGateOwnerRunsInternalAndCommandCoverageAndKeepsFirstRed(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	marker := filepath.Join(t.TempDir(), "first-red")
	for path, source := range map[string]string{
		"go.mod":              "module example.invalid/gate\n\ngo 1.27\n",
		"internal/app/app.go": "package app\nfunc Value() int { return 1 }\n",
		"internal/app/app_test.go": `package app
import (
 "os"
 "testing"
)
func TestFailsOnce(t *testing.T) {
 marker := os.Getenv("GATE_FLIP_MARKER")
 if _, err := os.Stat(marker); os.IsNotExist(err) {
  if err := os.WriteFile(marker, []byte("failed"), 0600); err != nil { t.Fatal(err) }
  t.Fatal("first verdict is red")
 } else if err != nil { t.Fatal(err) }
 if Value() != 1 { t.Fatal("value") }
}
`,
		"cmd/tool/main.go":      "package main\nfunc main() {}\nfunc value() int { return 2 }\n",
		"cmd/tool/main_test.go": "package main\nimport \"testing\"\nfunc TestCommand(t *testing.T) { if value() != 2 { t.Fatal(\"value\") } }\n",
	} {
		writeTestResultFile(t, filepath.Join(root, filepath.FromSlash(path)), []byte(source), 0o644)
	}
	environment := append(gittree.ScrubbedEnviron(), "GOFLAGS=-buildvcs=false", "GATE_FLIP_MARKER="+marker)
	result, status, err := RunGoGateTests(context.Background(), GoGateTestRequest{
		Root: root, LogRoot: filepath.Join(root, "logs"), Environment: environment, Workers: 2,
	})
	if status != 1 || err == nil || len(result.Reruns) != 1 || result.Reruns[0].First != "failed" || result.Reruns[0].Second != "passed" {
		t.Fatalf("gate red/rerun result status=%d err=%v result=%+v", status, err, result)
	}
	observed := map[string]string{}
	for _, identity := range result.Observed {
		observed[identity.Classname+"/"+identity.Name] = identity.Status
	}
	if observed["example.invalid/gate/internal/app/TestFailsOnce"] != "failed" ||
		observed["example.invalid/gate/cmd/tool/TestCommand"] != "passed" {
		t.Fatalf("gate dropped internal or command evidence: %v", observed)
	}
	if !strings.Contains(string(result.Output), "coverage:") {
		t.Fatalf("gate output omitted merged coverage:\n%s", result.Output)
	}
}

func TestDiagnosticRerunContendsForTheAttemptWorkerPool(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeTestResultFile(t, filepath.Join(root, "go.mod"), []byte("module example.invalid/rerunpool\n\ngo 1.27\n"), 0o644)
	writeTestResultFile(t, filepath.Join(root, "pkg", "pkg.go"), []byte("package pkg\n"), 0o644)
	writeTestResultFile(t, filepath.Join(root, "pkg", "pkg_test.go"), []byte("package pkg\nimport \"testing\"\nfunc TestPass(t *testing.T) {}\n"), 0o644)
	ctx := withTestWorkerPool(context.Background(), 1)
	held, err := acquireTestWorkers(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	request := TestRunRequest{Workers: 1, LogRoot: filepath.Join(root, "logs")}
	if err := os.MkdirAll(request.LogRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	group := testpolicy.Group{ID: "rerun-contention", Adapter: "go"}
	done := make(chan RerunFinding, 1)
	go func() {
		done <- runFailedTestAgain(ctx, request, group, root,
			append(gittree.ScrubbedEnviron(), "GOFLAGS=-buildvcs=false"), supervisorLimits{}, time.Second,
			NativeTestIdentity{Classname: "example.invalid/rerunpool/pkg", Name: "TestPass"}, 1, LoadSample{})
	}()
	pool := ctx.Value(testWorkerPoolContextKey{}).(*testWorkerPool)
	waitForTestWorkerPoolWaiters(pool, 1)
	if available, _ := testWorkerPoolState(pool); available != 0 {
		t.Fatalf("queued diagnostic waiter observed available capacity: %d", available)
	}
	select {
	case finding := <-done:
		t.Fatalf("diagnostic rerun bypassed the occupied pool: %+v", finding)
	default:
	}
	held()
	if finding := <-done; finding.First != "failed" || finding.Second != "passed" {
		t.Fatalf("diagnostic rerun after contention=%+v", finding)
	}
	assertWorkerPoolFullyAvailable(t, ctx, 1)
}

func testActiveShardFailureDrain(t *testing.T, failure string, coverage bool) {
	t.Helper()
	root := t.TempDir()
	bin := filepath.Join(root, "bin")
	if err := os.MkdirAll(bin, 0o700); err != nil {
		t.Fatal(err)
	}
	stateRoot := filepath.Join(root, "active-shards")
	if err := os.MkdirAll(stateRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	const diagnostic = "distinctive native diagnostic before later setup failure"
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(executable, filepath.Join(bin, "go")); err != nil {
		t.Fatal(err)
	}
	admissionRoot, conf := isolatedHostResources(t)
	custodyContext := WithResourceCustodyExecutable(t.Context(), buildResourceCustodyEngine(t))
	lease, err := AcquireHostResources(custodyContext, admissionRoot, conf, "heavy", nil)
	if err != nil {
		t.Fatal(err)
	}
	leaseOpen := true
	defer func() {
		if leaseOpen {
			_ = lease.Close()
		}
	}()
	ctx := withTestWorkerPool(WithHostResourceLease(custodyContext, lease), 2)
	hooks := goShardLifecycleHooks{}
	if coverage {
		collected := make(chan struct{})
		hooks.prepareCoverage = func(index int, directory, groupID string) error {
			if index == 1 {
				<-collected
				return errors.New("injected later coverage marker failure")
			}
			if err := os.MkdirAll(directory, 0o700); err != nil {
				return err
			}
			return os.WriteFile(filepath.Join(directory, ".pending"), []byte(groupID+"\n"), 0o600)
		}
		hooks.wrapOutput = func(index int, writer io.Writer) io.Writer {
			if index == 0 {
				return newCollectedOutputWriter(writer, diagnostic, collected)
			}
			return writer
		}
	} else {
		collected := []chan struct{}{make(chan struct{}), make(chan struct{})}
		allCollected := make(chan struct{})
		go func() {
			<-collected[0]
			<-collected[1]
			close(allCollected)
		}()
		hooks.wrapOutput = func(index int, writer io.Writer) io.Writer {
			collector := newCollectedOutputWriter(writer, diagnostic, collected[index])
			if index == 0 {
				return &failAfterCollectedWriter{collectedOutputWriter: collector, allCollected: allCollected,
					err: errors.New("injected active output failure")}
			}
			return collector
		}
	}
	ctx = context.WithValue(ctx, goShardLifecycleHooksKey{}, hooks)
	group := testpolicy.Group{ID: "active-failure", Adapter: "go", Coverage: coverage}
	var output synchronizedBuffer
	outcome, _, _, launchErr := runShardedGoGroup(ctx, TestRunRequest{Workers: 2}, group, root,
		[]string{"PATH=" + bin, "ACTIVE_SHARD_STATE_DIR=" + stateRoot,
			"ACTIVE_SHARD_DIAGNOSTIC=" + diagnostic, "GO_WANT_GO_SHARD_HELPER=1"},
		[]NativeTestIdentity{{Classname: "example.invalid/active/a", Name: "TestA"}, {Classname: "example.invalid/active/b", Name: "TestB"}},
		[]string{"a", "b"}, "example.invalid/active/", supervisorLimits{}, 10*time.Millisecond,
		filepath.Join(root, "active-failure.log"), &output)
	if launchErr == nil || !strings.Contains(launchErr.Error(), "injected") || !outcome.Started {
		t.Fatalf("%s outcome=%+v error=%v", failure, outcome, launchErr)
	}
	wantProcesses := 1
	if !coverage {
		wantProcesses = 2
	}
	processes := readAcknowledgedShardProcesses(t, stateRoot)
	if len(processes) != wantProcesses {
		t.Fatalf("%s acknowledged partitions=%d want=%d: %+v", failure, len(processes), wantProcesses, processes)
	}
	for _, process := range processes {
		if err := syscall.Kill(process.child, 0); !errors.Is(err, syscall.ESRCH) {
			t.Fatalf("%s returned before exact child %d drained: %v", failure, process.child, err)
		}
		if err := syscall.Kill(process.descendant, 0); !errors.Is(err, syscall.ESRCH) {
			t.Fatalf("%s returned before exact descendant %d drained: %v", failure, process.descendant, err)
		}
	}
	if coverage {
		if err := os.RemoveAll(filepath.Join(root, "active-failure.coverage")); err != nil {
			t.Fatal(err)
		}
		for shard := 1; shard <= 2; shard++ {
			_ = os.Remove(filepath.Join(root, fmt.Sprintf("active-failure.shard-%d.log", shard)))
		}
		_ = os.Remove(filepath.Join(root, "active-failure.log"))
	}
	if !strings.Contains(string(goNativePlainOutput(output.Bytes())), diagnostic) {
		t.Fatalf("%s lost collected native output:\n%s", failure, output.String())
	}
	assertWorkerPoolFullyAvailable(t, ctx, 2)
	if dirty, _, err := hostLeaseState(admissionRoot); err != nil || dirty != 0 {
		t.Fatalf("%s returned before host-resource custody drained: dirty=%d err=%v", failure, dirty, err)
	}
	if err := lease.Close(); err != nil {
		t.Fatal(err)
	}
	leaseOpen = false
	probe, err := AcquireHostResources(t.Context(), admissionRoot, conf, "heavy", nil)
	if err != nil {
		t.Fatalf("%s returned before host lease was reusable: %v", failure, err)
	}
	_ = probe.Close()
}

type collectedOutputWriter struct {
	mu         sync.Mutex
	writer     io.Writer
	diagnostic []byte
	observed   bytes.Buffer
	reached    chan struct{}
	once       sync.Once
}

func newCollectedOutputWriter(writer io.Writer, diagnostic string, reached chan struct{}) *collectedOutputWriter {
	return &collectedOutputWriter{writer: writer, diagnostic: []byte(diagnostic), reached: reached}
}

func (writer *collectedOutputWriter) Write(data []byte) (int, error) {
	n, err := writer.writer.Write(data)
	writer.mu.Lock()
	if n > 0 {
		_, _ = writer.observed.Write(data[:n])
		if bytes.Contains(writer.observed.Bytes(), writer.diagnostic) {
			writer.once.Do(func() { close(writer.reached) })
		}
	}
	writer.mu.Unlock()
	return n, err
}

type failAfterCollectedWriter struct {
	*collectedOutputWriter
	allCollected <-chan struct{}
	err          error
}

func (writer *failAfterCollectedWriter) Write(data []byte) (int, error) {
	n, err := writer.collectedOutputWriter.Write(data)
	if err != nil {
		return n, err
	}
	select {
	case <-writer.reached:
		<-writer.allCollected
		return 0, writer.err
	default:
		return n, nil
	}
}

type acknowledgedShardProcess struct {
	child      int
	descendant int
}

func readAcknowledgedShardProcesses(t *testing.T, root string) []acknowledgedShardProcess {
	t.Helper()
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	var processes []acknowledgedShardProcess
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".ready") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		var process acknowledgedShardProcess
		if _, err := fmt.Sscanf(string(data), "child=%d\ndescendant=%d\n", &process.child, &process.descendant); err != nil {
			t.Fatalf("parse complete shard readiness %s: %v", entry.Name(), err)
		}
		if entry.Name() != strconv.Itoa(process.child)+".ready" {
			t.Fatalf("shard readiness identity mismatch: file=%s child=%d", entry.Name(), process.child)
		}
		processes = append(processes, process)
	}
	return processes
}

func runGoWorkerTestInOwnedProcess(t *testing.T) bool {
	t.Helper()
	if os.Getenv("GO_WANT_GO_WORKER_OWNED_TEST") == "1" {
		return false
	}
	command := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^"+regexp.QuoteMeta(t.Name())+"$", "-test.v")
	command.Env = append(os.Environ(), "GO_WANT_GO_WORKER_OWNED_TEST=1")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("owned Go-worker test failed: %v\n%s", err, output)
	}
	return true
}

func testPublicGoGateCoverageSetupFailureKeepsOutput(t *testing.T) {
	t.Helper()
	root := t.TempDir()
	const diagnostic = "public native diagnostic before second coverage setup failure"
	for path, source := range map[string]string{
		"go.mod":              "module example.invalid/setupfailure\n\ngo 1.27\n",
		"internal/app/app.go": "package app\n",
		"internal/app/app_test.go": `package app
import ("fmt"; "os"; "testing"; "time")
func TestInternalDiagnostic(t *testing.T) {
 fmt.Fprintln(os.Stderr, "` + diagnostic + `")
 for { time.Sleep(time.Hour) }
}
`,
		"cmd/tool/main.go": "package main\nfunc main() {}\n",
		"cmd/tool/main_test.go": `package main
import ("fmt"; "os"; "testing"; "time")
func TestCommandDiagnostic(t *testing.T) {
 fmt.Fprintln(os.Stderr, "` + diagnostic + `")
 for { time.Sleep(time.Hour) }
}
`,
	} {
		writeTestResultFile(t, filepath.Join(root, filepath.FromSlash(path)), []byte(source), 0o644)
	}
	logRoot := filepath.Join(root, "logs")
	collected := make(chan struct{})
	hooks := goShardLifecycleHooks{prepareCoverage: func(index int, directory, groupID string) error {
		if index == 1 {
			<-collected
			return errors.New("injected public second coverage setup failure")
		}
		if err := os.MkdirAll(directory, 0o700); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(directory, ".pending"), []byte(groupID+"\n"), 0o600)
	}, wrapOutput: func(index int, writer io.Writer) io.Writer {
		if index == 0 {
			return newCollectedOutputWriter(writer, diagnostic, collected)
		}
		return writer
	}}
	ctx := context.WithValue(t.Context(), goShardLifecycleHooksKey{}, hooks)
	result, status, err := RunGoGateTests(ctx, GoGateTestRequest{Root: root, LogRoot: logRoot,
		Environment: append(gittree.ScrubbedEnviron(), "GOFLAGS=-buildvcs=false"), Workers: 2})
	if status != 1 || err == nil || !strings.Contains(err.Error(), "injected public second coverage setup failure") {
		t.Fatalf("public setup failure status=%d error=%v result=%+v", status, err, result)
	}
	if err := os.RemoveAll(logRoot); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(result.Output), diagnostic) {
		t.Fatalf("public result lost native diagnostic after snapshot cleanup:\n%s", result.Output)
	}
}

func assertWorkerPoolFullyAvailable(t *testing.T, ctx context.Context, capacity int) {
	t.Helper()
	pool := ctx.Value(testWorkerPoolContextKey{}).(*testWorkerPool)
	pool.mu.Lock()
	defer pool.mu.Unlock()
	if pool.available != capacity || pool.waiters != 0 {
		t.Fatalf("worker pool did not drain: capacity=%d available=%d waiters=%d", capacity, pool.available, pool.waiters)
	}
}

func nativeIdentityStatus(identities []NativeTestIdentity, name, status string) bool {
	for _, identity := range identities {
		if identity.Name == name && identity.Status == status {
			return true
		}
	}
	return false
}

func readFileText(path string) string {
	data, _ := os.ReadFile(path)
	return string(data)
}

func goWorkerNamedFixture(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	runTestResultGit(t, root, "init", "-q", "-b", "main")
	runTestResultGit(t, root, "config", "user.name", "fixture")
	runTestResultGit(t, root, "config", "user.email", "fixture@example.invalid")
	writeTestResultFile(t, filepath.Join(root, "go.mod"), []byte("module example.invalid/workers\n\ngo 1.27\n"), 0o644)
	for _, pkg := range []string{"a", "b"} {
		writeTestResultFile(t, filepath.Join(root, pkg, pkg+".go"), []byte("package "+pkg+"\n"), 0o644)
	}
	writeTestResultFile(t, filepath.Join(root, "a", "a_test.go"), []byte(goWorkerTestSource("a", "TestOnlyA")), 0o644)
	writeTestResultFile(t, filepath.Join(root, "b", "b_test.go"), []byte(goWorkerTestSource("b", "TestOnlyB")), 0o644)
	runTestResultGit(t, root, "add", ".")
	runTestResultGit(t, root, "commit", "-qm", "fixture")
	return root, runTestResultGit(t, root, "rev-parse", "HEAD^{tree}")
}

func goWorkerTestSource(pkg, only string) string {
	return `package ` + pkg + `
import (
 "flag"
 "os"
 "runtime"
 "testing"
)
func assertWorkerLimits(t *testing.T) {
 t.Helper()
 if runtime.GOMAXPROCS(0) != 1 || os.Getenv("GOMAXPROCS") != "1" || os.Getenv("METASYSTEM_TEST_WORKERS") != "1" {
  t.Fatalf("GOMAXPROCS=%d env=%q workers=%q", runtime.GOMAXPROCS(0), os.Getenv("GOMAXPROCS"), os.Getenv("METASYSTEM_TEST_WORKERS"))
 }
 parallel := flag.Lookup("test.parallel")
 if parallel == nil || parallel.Value.String() != "1" { t.Fatalf("test.parallel=%v", parallel) }
}
func TestSame(t *testing.T) { assertWorkerLimits(t) }
func ` + only + `(t *testing.T) { assertWorkerLimits(t) }
`
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func directoryNames(t *testing.T, root string) []string {
	t.Helper()
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names
}
