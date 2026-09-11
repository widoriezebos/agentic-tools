package proofrun

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

// A whole-package go group with shards runs its discovered tests as
// concurrent launches inside the one group: every test is observed once,
// each shard leaves its own log beside the group's, the group log is their
// concatenation, and whole-package coverage is merged from the shards'
// coverage data and judged against the floors.
func TestShardedGoGroupObservesEveryTestAndMergesCoverage(t *testing.T) {
	root := t.TempDir()
	runTestResultGit(t, root, "init", "-q", "-b", "main")
	runTestResultGit(t, root, "config", "user.name", "fixture")
	runTestResultGit(t, root, "config", "user.email", "fixture@example.invalid")
	writeTestResultFile(t, filepath.Join(root, "metasystem", "go.mod"), []byte("module github.com/widoriezebos/agentic-tools/metasystem\n\ngo 1.22\n"), 0o644)
	writeTestResultFile(t, filepath.Join(root, "metasystem", "internal", "sharded", "sharded.go"), []byte(`package sharded

func One() int   { return 1 }
func Two() int   { return 2 }
func Three() int { return 3 }
func Four() int  { return 4 }
`), 0o644)
	writeTestResultFile(t, filepath.Join(root, "metasystem", "internal", "sharded", "sharded_test.go"), []byte(`package sharded

import "testing"

func TestOne(t *testing.T)   { if One() != 1 { t.Fatal("one") } }
func TestTwo(t *testing.T)   { if Two() != 2 { t.Fatal("two") } }
func TestThree(t *testing.T) { if Three() != 3 { t.Fatal("three") } }
func TestFour(t *testing.T)  { if Four() != 4 { t.Fatal("four") } }
`), 0o644)
	writeTestResultFile(t, filepath.Join(root, "metasystem", "scripts", "agents", "coverage-ratchet.json"),
		[]byte(`{"note":"fixture","exempt":{},"floors":{"internal/sharded":90}}`+"\n"), 0o644)
	runTestResultGit(t, root, "add", ".")
	runTestResultGit(t, root, "commit", "-qm", "fixture")
	tree := runTestResultGit(t, root, "rev-parse", "HEAD^{tree}")
	group := testpolicy.Group{ID: "sharded-coverage", Kind: "unit", Adapter: "go", CWD: "metasystem",
		Inputs:      []string{"metasystem/go.mod", "metasystem/internal/sharded/**", "metasystem/scripts/agents/coverage-ratchet.json"},
		Tools:       []testpolicy.Tool{{ID: "go", Executable: "go", VersionArgs: []string{"version"}}},
		Obligations: []string{"sharded-package-coverage"}, Platforms: []string{"any"}, TargetMS: 60000,
		Packages: []string{"internal/sharded"}, Tests: []byte(`"all"`), Coverage: true, Shards: 2}
	logRoot := filepath.Join(root, "logs")
	result := runTestGroup(context.Background(), TestRunRequest{ProjectRoot: root, InstallationPrefix: "metasystem", CandidateTree: tree,
		Environment: append(gittree.ScrubbedEnviron(), "GOFLAGS=-buildvcs=false"), LogRoot: logRoot}, group)
	if result.Status != "passed" || !result.CollectionComplete {
		t.Fatalf("sharded group result=%+v", result)
	}
	observed := map[string]bool{}
	for _, identity := range result.Observed {
		observed[identity.Name] = true
	}
	for _, name := range []string{"TestOne", "TestTwo", "TestThree", "TestFour"} {
		if !observed[name] {
			t.Fatalf("shards lost %s: observed=%+v", name, result.Observed)
		}
	}
	if len(result.Missing) != 0 || len(result.Unexpected) != 0 {
		t.Fatalf("shards changed the expected set: missing=%+v unexpected=%+v", result.Missing, result.Unexpected)
	}
	for _, shard := range []string{"sharded-coverage.shard-1.log", "sharded-coverage.shard-2.log"} {
		if _, err := os.Stat(filepath.Join(logRoot, shard)); err != nil {
			t.Fatalf("shard log missing: %v", err)
		}
	}
	data, err := os.ReadFile(result.LogPath)
	if err != nil || strings.Count(string(data), `"Action":"pass","Package":"github.com/widoriezebos/agentic-tools/metasystem/internal/sharded","Test":`) != 4 {
		t.Fatalf("group log is not the shards' concatenation: err=%v\n%s", err, data)
	}
	if !result.NativeLaunched || result.CPUSeconds < 0 {
		t.Fatalf("merged supervision lost the launch: %+v", result)
	}
}

// A floor the merged coverage cannot meet fails the sharded group, so the
// merge is judged, not decorative.
func TestShardedGoGroupFailsAMissedFloor(t *testing.T) {
	root := t.TempDir()
	runTestResultGit(t, root, "init", "-q", "-b", "main")
	runTestResultGit(t, root, "config", "user.name", "fixture")
	runTestResultGit(t, root, "config", "user.email", "fixture@example.invalid")
	writeTestResultFile(t, filepath.Join(root, "metasystem", "go.mod"), []byte("module github.com/widoriezebos/agentic-tools/metasystem\n\ngo 1.22\n"), 0o644)
	writeTestResultFile(t, filepath.Join(root, "metasystem", "internal", "half", "half.go"), []byte(`package half

func Covered() int   { return 1 }
func Uncovered() int { return 2 }
`), 0o644)
	writeTestResultFile(t, filepath.Join(root, "metasystem", "internal", "half", "half_test.go"), []byte(`package half

import "testing"

func TestCovered(t *testing.T) { if Covered() != 1 { t.Fatal("covered") } }
func TestAlsoCovered(t *testing.T) { if Covered() != 1 { t.Fatal("covered") } }
`), 0o644)
	writeTestResultFile(t, filepath.Join(root, "metasystem", "scripts", "agents", "coverage-ratchet.json"),
		[]byte(`{"note":"fixture","exempt":{},"floors":{"internal/half":90}}`+"\n"), 0o644)
	runTestResultGit(t, root, "add", ".")
	runTestResultGit(t, root, "commit", "-qm", "fixture")
	tree := runTestResultGit(t, root, "rev-parse", "HEAD^{tree}")
	group := testpolicy.Group{ID: "sharded-half", Kind: "unit", Adapter: "go", CWD: "metasystem",
		Inputs:      []string{"metasystem/go.mod", "metasystem/internal/half/**", "metasystem/scripts/agents/coverage-ratchet.json"},
		Tools:       []testpolicy.Tool{{ID: "go", Executable: "go", VersionArgs: []string{"version"}}},
		Obligations: []string{"half-package-coverage"}, Platforms: []string{"any"}, TargetMS: 60000,
		Packages: []string{"internal/half"}, Tests: []byte(`"all"`), Coverage: true, Shards: 2}
	result := runTestGroup(context.Background(), TestRunRequest{ProjectRoot: root, InstallationPrefix: "metasystem", CandidateTree: tree,
		Environment: append(gittree.ScrubbedEnviron(), "GOFLAGS=-buildvcs=false"), LogRoot: filepath.Join(root, "logs")}, group)
	if result.Status != "failed" || !strings.Contains(result.NotRunReason, "internal/half") {
		t.Fatalf("a missed floor did not fail the sharded group: %+v", result)
	}
}
