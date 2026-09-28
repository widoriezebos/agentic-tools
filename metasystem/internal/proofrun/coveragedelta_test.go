package proofrun

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

// coverageDeltaBed is one coverage delta invocation over a synthetic
// installation, with every process and repository effect stubbed per test.
type coverageDeltaBed struct {
	options  CoverageDeltaOptions
	out, err bytes.Buffer
	tests    []string
	launches [][]string
	reused   [][]string
}

func newCoverageDeltaBed(t *testing.T) *coverageDeltaBed {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "scripts", "agents"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.invalid/metasystem\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"testing-coverage-floors.json", "testing-coverage-floors-linux.json"} {
		ratchet := `{"floors":{"internal/proofrun":80,"internal/low":90.5},"exempt":{}}`
		if name == "testing-coverage-floors-linux.json" {
			ratchet = `{"floors":{"internal/proofrun":70},"exempt":{}}`
		}
		if err := os.WriteFile(filepath.Join(root, name), []byte(ratchet), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	engine := filepath.Join(root, "bin", "metasystem")
	if err := os.MkdirAll(filepath.Dir(engine), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := testexec.WriteFile(engine, []byte("#!/bin/sh\nexit 97\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	bed := &coverageDeltaBed{}
	bed.options = CoverageDeltaOptions{
		Root: root, InvocationDir: root, Engine: engine, GOOS: "darwin",
		Git: func(args ...string) (string, error) { return "", errors.New("unexpected git call") },
		Reuse: func(ratchet string, packages []string) ([]string, bool, error) {
			bed.reused = append(bed.reused, append([]string{ratchet}, packages...))
			return nil, false, nil
		},
		WorkerAuthorized: func() bool { return true },
		Launch: func(ratchet string, packages []string) int {
			bed.launches = append(bed.launches, append([]string{ratchet}, packages...))
			return 23
		},
		GoTest: func(pkg string) (string, int) {
			bed.tests = append(bed.tests, pkg)
			return "ok  example.invalid/metasystem/" + strings.TrimPrefix(pkg, "./") + " 0.1s coverage: 85.0% of statements\n", 0
		},
	}
	bed.options.Out, bed.options.Err = &bed.out, &bed.err
	return bed
}

func (bed *coverageDeltaBed) run() int { return CoverageDelta(bed.options) }

func TestCoverageDeltaReusesMatchingRetainedEvidenceWithoutTesting(t *testing.T) {
	t.Parallel()
	bed := newCoverageDeltaBed(t)
	bed.options.Packages = []string{"internal/proofrun"}
	bed.options.Reuse = func(ratchet string, packages []string) ([]string, bool, error) {
		return []string{"coverage reuse: ./internal/proofrun: 85.0%"}, true, nil
	}
	if code := bed.run(); code != 0 {
		t.Fatalf("code=%d stderr=%s", code, bed.err.String())
	}
	if len(bed.tests) != 0 || len(bed.launches) != 0 {
		t.Fatalf("reuse still tested=%v launched=%v", bed.tests, bed.launches)
	}
	if !strings.Contains(bed.out.String(), "coverage reuse: ./internal/proofrun: 85.0%") ||
		!strings.Contains(bed.out.String(), "no coverage test launched") {
		t.Fatalf("stdout=%q", bed.out.String())
	}
}

func TestCoverageDeltaWorkerMeasuresEachPackageAgainstItsFloor(t *testing.T) {
	t.Parallel()
	bed := newCoverageDeltaBed(t)
	bed.options.Packages = []string{"internal/proofrun"}
	if code := bed.run(); code != 0 {
		t.Fatalf("code=%d stderr=%s", code, bed.err.String())
	}
	if !reflect.DeepEqual(bed.tests, []string{"./internal/proofrun"}) {
		t.Fatalf("tests=%v", bed.tests)
	}
	wantReuse := [][]string{{filepath.Join(bed.options.Root, "testing-coverage-floors.json"), "internal/proofrun"}}
	if !reflect.DeepEqual(bed.reused, wantReuse) {
		t.Fatalf("reuse=%v want %v", bed.reused, wantReuse)
	}
	for _, want := range []string{"coverage delta: ./internal/proofrun: 85.0% (floor 80.0%)", "coverage delta: passed (1 package(s) considered)"} {
		if !strings.Contains(bed.out.String(), want) {
			t.Fatalf("stdout lacks %q: %q", want, bed.out.String())
		}
	}
}

func TestCoverageDeltaOutsideAProofLaunchesOneAdmittedProof(t *testing.T) {
	t.Parallel()
	bed := newCoverageDeltaBed(t)
	bed.options.Packages = []string{"./internal/proofrun/", "example.invalid/metasystem/internal/low", "internal/proofrun"}
	bed.options.WorkerAuthorized = func() bool { return false }
	if code := bed.run(); code != 23 {
		t.Fatalf("code=%d want the launch's status; stderr=%s", code, bed.err.String())
	}
	want := [][]string{{filepath.Join(bed.options.Root, "testing-coverage-floors.json"), "internal/proofrun", "internal/low"}}
	if !reflect.DeepEqual(bed.launches, want) || len(bed.tests) != 0 {
		t.Fatalf("launches=%v tests=%v", bed.launches, bed.tests)
	}
}

func TestCoverageDeltaRefusesAnUnauthorizedRelaunchedChild(t *testing.T) {
	t.Parallel()
	bed := newCoverageDeltaBed(t)
	bed.options.Packages = []string{"internal/proofrun"}
	bed.options.Relaunched = true
	bed.options.WorkerAuthorized = func() bool { return false }
	if code := bed.run(); code != 1 || !strings.Contains(bed.err.String(), "relaunched child is not an authorized proof worker") {
		t.Fatalf("code=%d stderr=%q", code, bed.err.String())
	}
	if len(bed.launches) != 0 || len(bed.tests) != 0 {
		t.Fatalf("unauthorized child launched=%v tested=%v", bed.launches, bed.tests)
	}
}

func TestCoverageDeltaReportsEveryShortfallBeforeRefusing(t *testing.T) {
	t.Parallel()
	bed := newCoverageDeltaBed(t)
	bed.options.Packages = []string{"internal/low", "internal/proofrun", "internal/unregistered"}
	bed.options.GoTest = func(pkg string) (string, int) {
		bed.tests = append(bed.tests, pkg)
		if pkg == "./internal/proofrun" {
			return "--- FAIL: TestX\nFAIL\n", 1
		}
		return "coverage: 50.0% of statements\ncoverage: 60.0% of statements\n", 0
	}
	if code := bed.run(); code != 1 {
		t.Fatalf("code=%d", code)
	}
	for _, want := range []string{
		"coverage delta: packages below floor:",
		"  ./internal/low: measured 60.0%, floor 90.5%",
		"coverage delta: package test failures:",
		"  ./internal/proofrun (go test exited 1)",
	} {
		if !strings.Contains(bed.err.String(), want) {
			t.Fatalf("stderr lacks %q: %q", want, bed.err.String())
		}
	}
	if !strings.Contains(bed.out.String(), "coverage delta: ./internal/unregistered: no floor registered") {
		t.Fatalf("stdout=%q", bed.out.String())
	}
	if len(bed.tests) != 2 {
		t.Fatalf("tests=%v", bed.tests)
	}
}

func TestCoverageDeltaSelectionAndSkips(t *testing.T) {
	t.Parallel()
	bed := newCoverageDeltaBed(t)
	if code := bed.run(); code != 2 {
		t.Fatalf("no selection code=%d", code)
	}
	bed = newCoverageDeltaBed(t)
	bed.options.Staged, bed.options.Packages = true, []string{"internal/proofrun"}
	if code := bed.run(); code != 2 || !strings.Contains(bed.err.String(), "mutually exclusive") {
		t.Fatalf("exclusive code=%d stderr=%q", code, bed.err.String())
	}

	bed = newCoverageDeltaBed(t)
	bed.options.Staged = true
	if err := os.Remove(filepath.Join(bed.options.Root, "testing-coverage-floors.json")); err != nil {
		t.Fatal(err)
	}
	if code := bed.run(); code != 0 || !strings.Contains(bed.out.String(), "no ratchet registry at this root; skipped") {
		t.Fatalf("registry-less code=%d stdout=%q", code, bed.out.String())
	}

	bed = newCoverageDeltaBed(t)
	bed.options.Staged = true
	var gitArgs []string
	bed.options.Git = func(args ...string) (string, error) { gitArgs = args; return "", nil }
	if code := bed.run(); code != 0 || !strings.Contains(bed.out.String(), "no Go files staged; skipped") {
		t.Fatalf("nothing staged code=%d stdout=%q", code, bed.out.String())
	}
	if !reflect.DeepEqual(gitArgs, []string{"diff", "--cached", "--name-only", "--relative", "--", "*.go"}) {
		t.Fatalf("staged diff args=%v", gitArgs)
	}

	bed = newCoverageDeltaBed(t)
	bed.options.Base = "origin/main"
	bed.options.Git = func(args ...string) (string, error) {
		gitArgs = args
		return "internal/proofrun/a.go\ninternal/proofrun/b.go\nroot.go\n", nil
	}
	if code := bed.run(); code != 0 {
		t.Fatalf("base code=%d stderr=%q", code, bed.err.String())
	}
	if !reflect.DeepEqual(gitArgs, []string{"diff", "--name-only", "--relative", "origin/main", "--", "*.go"}) ||
		!reflect.DeepEqual(bed.tests, []string{"./internal/proofrun"}) {
		t.Fatalf("base diff args=%v tests=%v stdout=%q", gitArgs, bed.tests, bed.out.String())
	}
	if !strings.Contains(bed.out.String(), "coverage delta: .: no floor registered") {
		t.Fatalf("root package not considered: %q", bed.out.String())
	}

	bed = newCoverageDeltaBed(t)
	bed.options.Base = "missing"
	bed.options.Git = func(args ...string) (string, error) { return "", errors.New("exit 128") }
	if code := bed.run(); code != 1 || !strings.Contains(bed.err.String(), "could not derive packages from base missing") {
		t.Fatalf("bad base code=%d stderr=%q", code, bed.err.String())
	}
}

func TestCoverageDeltaResolvesRatchetEngineAndPackages(t *testing.T) {
	t.Parallel()
	bed := newCoverageDeltaBed(t)
	bed.options.Packages = []string{"internal/proofrun"}
	bed.options.GOOS = "linux"
	if code := bed.run(); code != 0 || !strings.Contains(bed.out.String(), "(floor 70.0%)") {
		t.Fatalf("linux ratchet code=%d stdout=%q", code, bed.out.String())
	}

	bed = newCoverageDeltaBed(t)
	bed.options.Packages = []string{"internal/proofrun"}
	bed.options.InvocationDir = filepath.Join(bed.options.Root, "internal")
	bed.options.Ratchet = "../testing-coverage-floors-linux.json"
	if code := bed.run(); code != 0 || !strings.Contains(bed.out.String(), "(floor 70.0%)") {
		t.Fatalf("relative ratchet code=%d stdout=%q stderr=%q", code, bed.out.String(), bed.err.String())
	}

	bed = newCoverageDeltaBed(t)
	bed.options.Packages = []string{"internal/..."}
	if code := bed.run(); code != 2 || !strings.Contains(bed.err.String(), "expected one concrete package") {
		t.Fatalf("pattern code=%d stderr=%q", code, bed.err.String())
	}

	bed = newCoverageDeltaBed(t)
	bed.options.Packages = []string{"internal/proofrun"}
	bed.options.Engine = filepath.Join(bed.options.Root, "bin", "absent")
	if code := bed.run(); code != 1 || !strings.Contains(bed.err.String(), "engine is unavailable") {
		t.Fatalf("absent engine code=%d stderr=%q", code, bed.err.String())
	}

	bed = newCoverageDeltaBed(t)
	bed.options.Packages = []string{"internal/proofrun"}
	bed.options.Ratchet = filepath.Join(bed.options.Root, "empty.json")
	if err := os.WriteFile(bed.options.Ratchet, []byte(`{"floors":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if code := bed.run(); code != 1 || !strings.Contains(bed.err.String(), "ratchet floors are unreadable") {
		t.Fatalf("empty floors code=%d stderr=%q", code, bed.err.String())
	}

	bed = newCoverageDeltaBed(t)
	bed.options.Packages = []string{"internal/proofrun"}
	bed.options.Reuse = func(string, []string) ([]string, bool, error) { return nil, false, errors.New("attempts unreadable") }
	if code := bed.run(); code != 1 || !strings.Contains(bed.err.String(), "retained coverage authority was unreadable") {
		t.Fatalf("unreadable reuse code=%d stderr=%q", code, bed.err.String())
	}
}
