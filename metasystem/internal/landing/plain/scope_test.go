package plain

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy/adapter"
)

type scopeBed struct {
	install, checkout string
	git               stubGit
	base              Result
	seams             ProveSeams
	contract          testpolicy.Contract
	calls             []*exec.Cmd
	output            *os.File
}

func newScopeBed(t *testing.T) *scopeBed {
	t.Helper()
	b := &scopeBed{checkout: t.TempDir()}
	b.install = filepath.Join(b.checkout, "metasystem")
	if err := os.MkdirAll(b.install, 0o755); err != nil {
		t.Fatal(err)
	}
	var err error
	b.output, err = os.Create(filepath.Join(b.install, "proof.log"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = b.output.Close() })
	if err := os.WriteFile(filepath.Join(b.install, "metasystem.conf"), []byte("testing.contract=testing.json\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	b.base = Result{Tree: "full-tree", Commit: "base-commit", Result: Green, Scope: "full", Attempt: "base",
		At: bedNow.Add(-10 * time.Minute).Format(time.RFC3339), Environment: "image toolchain"}
	b.base.FullTree, b.base.FullAt = b.base.Tree, b.base.At
	b.git = stubGit{commit: "head-commit", tree: "head-tree", installPrefix: "metasystem",
		batches: map[string]string{"head-commit": "batch-b\nbatch-a", "base-commit": "batch-a\nbatch-b"},
		changed: map[[2]string]string{{b.base.Tree, "head-tree"}: "metasystem/plans/page.md"}, shows: map[string]string{}}
	b.contract = testpolicy.Contract{SchemaVersion: testpolicy.ExecutionContractSchemaVersion,
		ProjectRisk: testpolicy.ProjectRisk{Severity: 1, Exposure: 1, Reversibility: "revert", Detection: "immediate", Recovery: "bounded"},
		Surfaces:    []testpolicy.Surface{{ID: "app", Paths: []string{"metasystem/**"}, Standard: []string{"plans"}}}, Unknown: []string{"plans"}}
	for i, id := range []string{"plans", "unrelated", "shared"} {
		input := []string{"metasystem/plans/**", "metasystem/records/**", "metasystem/plans/**"}[i]
		b.contract.Groups = append(b.contract.Groups, testpolicy.Group{ID: id, Kind: "unit", Adapter: "command", CWD: ".",
			Phase: "acceptance", EnvironmentMode: "inherit", Inputs: []string{input}, Platforms: []string{"any"}, TargetMS: 1,
			Argv: []string{"app-tests"}, Format: "exit-status"})
	}
	b.setContract(t)
	b.save(t, b.base)
	b.seams = ProveSeams{Git: b.git.run, Now: func() time.Time { return bedNow }, NewID: func() string { return "attempt" },
		Closure: func(root, base, tree string) (adapter.Closure, error) {
			if root != b.checkout || base != b.base.Tree || tree != b.git.tree {
				t.Fatalf("closure received %q %q %q", root, base, tree)
			}
			return adapter.Closure{Unowned: []string{"metasystem/plans/page.md"}}, nil
		}}
	b.seams.Command = func(command *exec.Cmd) error {
		b.calls = append(b.calls, command)
		fmt.Fprint(command.Stdout, "ordinary output\nlanding environ")
		fmt.Fprint(command.Stdout, "ment image toolchain\nlanding environment ignored\nlanding group plans passed 12\nlanding group shared passed 34")
		return nil
	}
	return b
}

func (b *scopeBed) setContract(t *testing.T) {
	t.Helper()
	data, err := json.Marshal(b.contract)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := testpolicy.Decode(data); err != nil {
		t.Fatal(err)
	}
	b.git.shows[b.git.tree+":metasystem/testing.json"] = string(data)
}

func (b *scopeBed) save(t *testing.T, result Result) {
	t.Helper()
	if err := withLock(b.install, func() error { return appendLine(resultsPath(b.install), result) }); err != nil {
		t.Fatal(err)
	}
}

func (b *scopeBed) run(t *testing.T) Result {
	t.Helper()
	b.seams.Git = b.git.run
	result, err := Run(b.install, b.checkout, "proof-command", "", b.output, b.seams)
	if err != nil {
		t.Fatal(err)
	}
	last, found, err := LastResult(b.install)
	if err != nil || !found || !reflect.DeepEqual(result, last) {
		t.Fatalf("recorded %+v, returned %+v: %v", last, result, err)
	}
	return result
}

func (b *scopeBed) record(t *testing.T) scopeRecord {
	t.Helper()
	data, err := os.ReadFile(scopePath(b.install, "attempt"))
	if err != nil {
		t.Fatal(err)
	}
	var record scopeRecord
	if err := json.Unmarshal(data, &record); err != nil {
		t.Fatal(err)
	}
	return record
}

func commandEnv(command *exec.Cmd, name string) string {
	for i := len(command.Env) - 1; i >= 0; i-- {
		if value, ok := strings.CutPrefix(command.Env[i], name+"="); ok {
			return value
		}
	}
	return "missing"
}

// Record-only tip moves preserve scoped proof; executable changes require full proof.
func TestATipMoveRunsOnlyTheGroupsItsPathsReach(t *testing.T) {
	t.Parallel()
	b := newScopeBed(t)
	result := b.run(t)
	if result.Result != Green || result.Scope != "scoped" || result.Base != b.base.Tree || result.FullAt != b.base.FullAt || result.FullTree != b.base.FullTree ||
		result.Environment != b.base.Environment || !reflect.DeepEqual(result.Ran, []string{"plans", "shared"}) || len(b.calls) != 1 {
		t.Fatalf("scoped result: %+v, calls %d", result, len(b.calls))
	}
	for key, want := range map[string]string{"LANDING_PROOF_SCOPE": "scoped", "LANDING_PROOF_BASE": b.base.Tree, "LANDING_PROOF_GROUPS": "plans shared", "LANDING_TREE": b.git.tree, "LANDING_COMMIT": b.git.commit} {
		if got := commandEnv(b.calls[0], key); got != want {
			t.Fatalf("%s=%q, want %q", key, got, want)
		}
	}
	output, err := os.ReadFile(b.output.Name())
	if err != nil {
		t.Fatal(err)
	}
	if b.calls[0].Dir != filepath.Join(proofTrees(b.install), "attempt", "metasystem") || string(output) != "ordinary output\nlanding environment image toolchain\nlanding environment ignored\nlanding group plans passed 12\nlanding group shared passed 34" {
		t.Fatalf("proof directory or output changed: %s, %q", b.calls[0].Dir, output)
	}
	got := b.record(t)
	paths := []string{"metasystem/plans/page.md"}
	want := []scopeGroup{{ID: "plans", State: "ran", Why: paths, From: b.git.tree, DurationMS: 12},
		{ID: "unrelated", State: "kept", From: b.base.Tree}, {ID: "shared", State: "ran", Why: paths, From: b.git.tree, DurationMS: 34}}
	if got.Scope != result.Scope || got.ScopeReason != result.ScopeReason || got.Base != b.base.Tree || !reflect.DeepEqual(got.ChangedPaths, paths) || !reflect.DeepEqual(got.Groups, want) {
		t.Fatalf("scope record: %+v, want groups %+v", got, want)
	}
	// A later scoped proof keeps the original source and duration of untouched groups.
	b.base = result
	b.git.batches[result.Commit] = "batch-a\nbatch-b"
	b.git.tree, b.git.commit = "next-tree", "next-commit"
	b.git.batches[b.git.commit] = "batch-b\nbatch-a"
	b.git.changed[[2]string{result.Tree, b.git.tree}] = "metasystem/records/test-image.json"
	b.setContract(t)
	b.seams.NewID = func() string { return "next-attempt" }
	b.seams.Command = func(command *exec.Cmd) error {
		fmt.Fprint(command.Stdout, "landing environment image toolchain\nlanding group unrelated passed 56\n")
		return nil
	}
	next := b.run(t)
	data, err := os.ReadFile(scopePath(b.install, next.Attempt))
	var record scopeRecord
	if err != nil || json.Unmarshal(data, &record) != nil || next.Scope != "scoped" || record.Groups[0].From != result.Tree || record.Groups[0].DurationMS != 12 || record.Groups[2].DurationMS != 34 {
		t.Fatalf("kept provenance: %+v %v", record, err)
	}
}

func TestABatchsFirstProofIsFull(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"no base", "other batch", "legacy", "red", "empty batch"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			b := newScopeBed(t)
			switch name {
			case "no base":
				if err := os.Remove(resultsPath(b.install)); err != nil {
					t.Fatal(err)
				}
			case "other batch":
				b.git.batches[b.base.Commit] = "different-batch"
			case "legacy", "red":
				if err := os.Remove(resultsPath(b.install)); err != nil {
					t.Fatal(err)
				}
				if name == "legacy" {
					b.base.Scope = ""
				} else {
					b.base.Result = Red
				}
				b.save(t, b.base)
			case "empty batch":
				b.git.batches[b.git.commit] = ""
			}
			result := b.run(t)
			if result.Scope != "full" || result.ScopeReason != "no green proof of this batch yet" || result.Base != "" || result.FullTree != result.Tree || result.FullAt != result.At || len(b.record(t).Groups) != 0 {
				t.Fatalf("first proof: %+v", result)
			}
			if len(b.calls) != 1 || commandEnv(b.calls[0], "LANDING_PROOF_SCOPE") != "full" || commandEnv(b.calls[0], "LANDING_PROOF_BASE") != "" || commandEnv(b.calls[0], "LANDING_PROOF_GROUPS") != "" {
				t.Fatal("first proof command did not receive full scope")
			}
		})
	}
}

func TestARecordsOnlyBatchIsProvedScopedOnMainsGreen(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"selected groups", "changed environment", "main moved"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			b := newScopeBed(t)
			b.git.batches[b.base.Commit] = ""
			b.git.onMain = map[string]bool{b.base.Commit: true}
			paths := "metasystem/plans/page.md\nmetasystem/plans/goals/fix.md"
			b.git.changed[[2]string{"origin/main", b.git.tree}] = paths
			if name == "main moved" {
				paths += "\nmetasystem/plans/another-page.md"
			}
			b.git.changed[[2]string{b.base.Tree, b.git.tree}] = paths
			// A newer full green that has not reached main cannot be the base.
			other := b.base
			other.Commit, other.Tree, other.At = "unlanded", "unlanded-tree", bedNow.Add(-time.Minute).Format(time.RFC3339)
			b.git.batches[other.Commit] = "other-batch"
			b.save(t, other)
			groups := "plans shared"
			b.seams.Command = func(command *exec.Cmd) error {
				b.calls = append(b.calls, command)
				environment := b.base.Environment
				if name == "changed environment" {
					environment = "new image toolchain"
				}
				fmt.Fprintln(command.Stdout, "landing environment "+environment)
				for _, group := range strings.Fields(commandEnv(command, "LANDING_PROOF_GROUPS")) {
					fmt.Fprintf(command.Stdout, "landing group %s green 12\n", group)
				}
				fmt.Fprintln(command.Stdout, "LANDING-CHECKED\t0")
				return nil
			}
			result := b.run(t)
			if len(b.calls) == 0 || commandEnv(b.calls[0], "LANDING_PROOF_SCOPE") != "scoped" || commandEnv(b.calls[0], "LANDING_PROOF_BASE") != b.base.Tree || commandEnv(b.calls[0], "LANDING_PROOF_GROUPS") != groups {
				t.Fatalf("records-only command: %+v, calls %d", result, len(b.calls))
			}
			if name == "changed environment" {
				if result.Result != Green || result.Scope != "full" || !strings.Contains(result.ScopeReason, "environment changed") || result.Base != "" || len(b.calls) != 2 || commandEnv(b.calls[1], "LANDING_PROOF_SCOPE") != "full" {
					t.Fatalf("environment rerun: %+v, calls %d", result, len(b.calls))
				}
				return
			}
			if result.Result != Green || result.Scope != "scoped" || !strings.Contains(result.ScopeReason, "records-only batch") || result.Base != b.base.Tree || result.FullTree != b.base.Tree || result.FullAt != b.base.At || strings.Join(result.Ran, " ") != groups || len(b.calls) != 1 {
				t.Fatalf("records-only proof: %+v, calls %d", result, len(b.calls))
			}
			if record := b.record(t); record.Base != b.base.Tree || !reflect.DeepEqual(record.ChangedPaths, strings.Split(paths, "\n")) {
				t.Fatalf("records-only scope record: %+v", record)
			}
			writePushRecords(t, b.install, Pushed{Tree: result.Tree, At: bedNow.Format(time.RFC3339)})
			if due, err := fullProofDue(b.install, bedNow.Add(51*time.Minute)); err != nil || !due {
				t.Fatalf("records-only push owes no full proof: %v, %v", due, err)
			}
		})
	}
}

func TestARecordsOnlyBatchWithoutARecentGreenIsFull(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"missing", "one hour old", "stale", "scoped", "red", "unreadable time", "not on main"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			b := newScopeBed(t)
			b.git.batches[b.base.Commit] = ""
			b.git.onMain = map[string]bool{b.base.Commit: name != "not on main"}
			b.git.changed[[2]string{"origin/main", b.git.tree}] = "metasystem/plans/page.md"
			if err := os.Remove(resultsPath(b.install)); err != nil {
				t.Fatal(err)
			}
			switch name {
			case "one hour old":
				b.base.At = bedNow.Add(-time.Hour).Format(time.RFC3339)
			case "stale":
				b.base.At = bedNow.Add(-61 * time.Minute).Format(time.RFC3339)
			case "scoped":
				b.base.Scope = "scoped"
			case "red":
				b.base.Result = Red
			case "unreadable time":
				b.base.At = "bad time"
			}
			if name != "missing" {
				b.save(t, b.base)
			}
			result := b.run(t)
			if result.Result != Green || result.Scope != "full" || result.Base != "" || !strings.Contains(result.ScopeReason, "records-only batch") || !strings.Contains(result.ScopeReason, "no full green on main under an hour") || result.FullTree != result.Tree || result.FullAt != result.At || len(b.calls) != 1 || commandEnv(b.calls[0], "LANDING_PROOF_SCOPE") != "full" {
				t.Fatalf("records without a recent green: %+v, calls %d", result, len(b.calls))
			}
		})
	}
}

func TestABatchWithACodePathKeepsItsFullFirstProof(t *testing.T) {
	t.Parallel()
	for _, path := range []string{"metasystem/internal/app.go", "metasystem/plans/README.md", "metasystem/records/README.md", "unknown/file"} {
		t.Run(path, func(t *testing.T) {
			t.Parallel()
			b := newScopeBed(t)
			b.git.batches[b.base.Commit] = ""
			b.git.onMain = map[string]bool{b.base.Commit: true}
			paths := "metasystem/plans/page.md\n" + path
			b.git.changed[[2]string{"origin/main", b.git.tree}] = paths
			b.git.changed[[2]string{b.base.Tree, b.git.tree}] = paths
			result := b.run(t)
			if result.Result != Green || result.Scope != "full" || result.ScopeReason != "no green proof of this batch yet" || result.Base != "" || len(b.calls) != 1 || commandEnv(b.calls[0], "LANDING_PROOF_SCOPE") != "full" || commandEnv(b.calls[0], "LANDING_PROOF_GROUPS") != "" {
				t.Fatalf("mixed batch's first proof: %+v, calls %d", result, len(b.calls))
			}
		})
	}
}

func TestAScopedChainOlderThanAnHourIsFull(t *testing.T) {
	t.Parallel()
	for _, age := range []time.Duration{time.Hour, 61 * time.Minute} {
		t.Run(age.String(), func(t *testing.T) {
			t.Parallel()
			b := newScopeBed(t)
			b.base.Scope, b.base.At = "scoped", bedNow.Add(-time.Minute).Format(time.RFC3339)
			b.base.FullAt = bedNow.Add(-age).Format(time.RFC3339)
			b.save(t, b.base)
			result := b.run(t)
			if age > time.Hour {
				if result.Scope != "full" || !strings.Contains(result.ScopeReason, "1h1m0s") {
					t.Fatalf("old full proof: %+v", result)
				}
			} else if result.Scope != "scoped" {
				t.Fatalf("one-hour boundary: %+v", result)
			}
		})
	}
}

func TestAStaleScopedGreenDoesNotSettleItsTree(t *testing.T) {
	t.Parallel()
	for _, each := range []struct {
		name, scope, fullAt string
		settled             bool
	}{
		{"61 minutes", "scoped", bedNow.Add(-61 * time.Minute).Format(time.RFC3339), false},
		{"59 minutes", "scoped", bedNow.Add(-59 * time.Minute).Format(time.RFC3339), true},
		{"exactly one hour", "scoped", bedNow.Add(-time.Hour).Format(time.RFC3339), true},
		{"unreadable time", "scoped", "bad time", false},
		{"full green", "full", bedNow.Add(-2 * time.Hour).Format(time.RFC3339), true},
		{"legacy green", "", "", true},
	} {
		t.Run(each.name, func(t *testing.T) {
			t.Parallel()
			b := newScopeBed(t)
			// HEAD is origin/main: rev-list names no batch to keep green.
			b.git.batches[b.git.commit] = ""
			green := Result{Tree: b.git.tree, Commit: b.git.commit, Result: Green, Scope: each.scope,
				At: bedNow.Add(-time.Minute).Format(time.RFC3339), FullAt: each.fullAt, FullTree: b.base.Tree}
			data, err := json.Marshal(green)
			if err != nil {
				t.Fatal(err)
			}
			writeProofRecords(t, b.install, []string{string(data)}, "")
			got, settled, err := Settled(b.install, b.checkout, b.seams)
			if err != nil || settled != each.settled || !reflect.DeepEqual(got, green) {
				t.Fatalf("settled = %+v, %v, want %v: %v", got, settled, each.settled, err)
			}
			if settled {
				return
			}
			result := b.run(t)
			if result.Result != Green || result.Scope != "full" || result.FullTree != b.git.tree || result.FullAt != result.At ||
				len(b.calls) != 1 || commandEnv(b.calls[0], "LANDING_PROOF_SCOPE") != "full" {
				t.Fatalf("main's owed proof = %+v, commands %d", result, len(b.calls))
			}
			if paid, ok, err := Settled(b.install, b.checkout, b.seams); err != nil || !ok || paid.Scope != "full" {
				t.Fatalf("the full green did not settle main: %+v, %v, %v", paid, ok, err)
			}
		})
	}
}

func TestSelectionFailureIsAFullProof(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"diff error", "no paths", "contract unreadable", "contract invalid", "contract changed", "adapter error", "code unit", "dependent unit", "uncovered path", "template path"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			b := newScopeBed(t)
			want := ""
			switch name {
			case "diff error":
				b.git.diffErr = errors.New("diff failed")
				want = "changed paths cannot be read"
			case "no paths":
				b.git.changed[[2]string{b.base.Tree, b.git.tree}] = ""
				want = "no changed paths"
			case "contract unreadable":
				b.git.showErr = errors.New("show failed")
				want = "testing contract cannot be read"
			case "contract invalid":
				b.git.shows[b.git.tree+":metasystem/testing.json"] = "{}"
				want = "testing contract is invalid"
			case "contract changed":
				b.git.changed[[2]string{b.base.Tree, b.git.tree}] = "metasystem/testing.json"
				want = "an executable input changed: metasystem/testing.json"
			case "adapter error":
				b.seams.Closure = func(string, string, string) (adapter.Closure, error) {
					return adapter.Closure{}, errors.New("no adapter")
				}
				want = "language closure cannot be read"
			case "code unit", "dependent unit":
				b.seams.Closure = func(string, string, string) (adapter.Closure, error) {
					if name == "code unit" {
						return adapter.Closure{Changed: []string{"app"}}, nil
					}
					return adapter.Closure{Dependents: []string{"app"}}, nil
				}
				want = "affect code units"
			case "uncovered path":
				b.contract.Groups[1].Inputs = []string{"nothing/**"}
				b.contract.Groups[2].Inputs = []string{"metasystem/plans/**"}
				b.setContract(t)
				b.git.changed[[2]string{b.base.Tree, b.git.tree}] = "metasystem/plans/page.md\nmetasystem/records/uncovered.json"
				want = "no declared test inputs"
			case "template path":
				b.contract.Groups = append(b.contract.Groups, testpolicy.Group{ID: "template", Adapter: "go", PackageSelection: "changed-and-consumers", Kind: "unit", CWD: ".", Phase: "acceptance", EnvironmentMode: "inherit", Inputs: []string{"metasystem/plans/**"}, Platforms: []string{"any"}, TargetMS: 1, Tests: json.RawMessage(`"all"`)})
				b.setContract(t)
				want = "test group template"
			}
			result := b.run(t)
			if result.Scope != "full" || !strings.Contains(result.ScopeReason, want) || commandEnv(b.calls[0], "LANDING_PROOF_SCOPE") != "full" || len(b.record(t).Groups) != 0 {
				t.Fatalf("%s: %+v, want full because %s", name, result, want)
			}
		})
	}
}

func TestAChangedEnvironmentProvesAgainInFull(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"changed", "base absent", "output absent", "both absent", "full red"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			b := newScopeBed(t)
			if name == "base absent" || name == "both absent" {
				b.base.Environment = ""
				b.save(t, b.base)
			}
			b.seams.Command = func(command *exec.Cmd) error {
				b.calls = append(b.calls, command)
				if len(b.calls) == 1 {
					if name != "output absent" && name != "both absent" {
						fmt.Fprint(command.Stdout, "landing environment new-image\nlanding group plans passed 1\n")
					}
				} else {
					fmt.Fprint(command.Stdout, "landing environment full-image\nlanding group unrelated passed 90\n")
					if name == "full red" {
						return errors.New("full failed")
					}
				}
				return nil
			}
			result := b.run(t)
			if len(b.calls) != 2 || commandEnv(b.calls[0], "LANDING_PROOF_SCOPE") != "scoped" || commandEnv(b.calls[1], "LANDING_PROOF_SCOPE") != "full" || commandEnv(b.calls[1], "LANDING_PROOF_BASE") != "" || commandEnv(b.calls[1], "LANDING_PROOF_GROUPS") != "" ||
				result.Scope != "full" || !strings.Contains(result.ScopeReason, "environment changed") || result.Attempt != "attempt" || result.Base != "" || result.Environment != "full-image" || !reflect.DeepEqual(result.Ran, []string{"unrelated"}) || len(b.record(t).Groups) != 0 {
				t.Fatalf("environment fallback: %+v, calls %d", result, len(b.calls))
			}
			if name == "full red" {
				if result.Result != Red || result.FullAt != "" {
					t.Fatalf("failed full proof: %+v", result)
				}
			} else if result.Result != Green || result.FullAt != result.At || result.FullTree != result.Tree {
				t.Fatalf("full green: %+v", result)
			}
			results, err := Results(b.install)
			if err != nil || results[len(results)-2].Attempt == result.Attempt {
				t.Fatal("fallback wrote a separate scoped result")
			}
		})
	}
}

func TestProofStreamsStayFilesWithABackgroundChild(t *testing.T) {
	t.Parallel()
	for _, scope := range []string{"full", "scoped", "rerun"} {
		t.Run(scope, func(t *testing.T) {
			t.Parallel()
			b := newScopeBed(t)
			if scope == "full" {
				if err := os.Remove(resultsPath(b.install)); err != nil {
					t.Fatal(err)
				}
			}
			var descendantOutput *os.File
			b.seams.Command = func(command *exec.Cmd) error {
				b.calls = append(b.calls, command)
				stdout, outOK := command.Stdout.(*os.File)
				stderr, errOK := command.Stderr.(*os.File)
				if !outOK || !errOK || stdout != b.output || stderr != b.output || command.WaitDelay != 0 {
					t.Fatalf("proof streams: stdout %T, stderr %T; want the log file without WaitDelay", command.Stdout, command.Stderr)
				}
				// A descendant retains the file after the shell exits; no copy goroutine waits for it.
				descendantOutput = stdout
				environment := b.base.Environment
				if scope == "rerun" {
					environment = "new-image"
				}
				_, err := fmt.Fprintln(stdout, "landing environment "+environment)
				return err
			}
			result := b.run(t)
			wantCalls := 1
			if scope == "rerun" {
				wantCalls = 2
			}
			if result.Result != Green || len(b.calls) != wantCalls || descendantOutput != b.output {
				t.Fatalf("proof with retained descendant stream: %+v, calls %d", result, len(b.calls))
			}
		})
	}
}

func TestProofObservesOnlyTheCurrentLogSegment(t *testing.T) {
	t.Parallel()
	b := newScopeBed(t)
	if _, err := fmt.Fprint(b.output, "landing environment stale\nlanding group stale passed 999\n"); err != nil {
		t.Fatal(err)
	}
	result := b.run(t)
	if result.Scope != "scoped" || result.Environment != b.base.Environment || !reflect.DeepEqual(result.Ran, []string{"plans", "shared"}) || len(b.calls) != 1 {
		t.Fatalf("proof read older log bytes: %+v, calls %d", result, len(b.calls))
	}
}

func TestProofWithoutASeekableLogReprovesInFull(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"pipe", "writer"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			b := newScopeBed(t)
			var output io.Writer = new(bytes.Buffer)
			if kind == "pipe" {
				reader, writer, err := os.Pipe()
				if err != nil {
					t.Fatal(err)
				}
				defer reader.Close()
				defer writer.Close()
				output = writer
			}
			result, err := Run(b.install, b.checkout, "proof-command", "", output, b.seams)
			if err != nil || result.Result != Green || result.Scope != "full" || result.Environment != "" || len(result.Ran) != 0 || len(b.calls) != 2 || commandEnv(b.calls[1], "LANDING_PROOF_SCOPE") != "full" {
				t.Fatalf("nonseekable output: %+v, calls %d, error %v", result, len(b.calls), err)
			}
			for _, command := range b.calls {
				log, ok := command.Stdout.(*os.File)
				if !ok || command.Stderr != log || command.WaitDelay != 0 {
					t.Fatal("nonseekable output did not use a file for the shell")
				}
			}
		})
	}
}

func TestALedgerInheritanceKeepsScopeAndOwesItsFullProof(t *testing.T) {
	t.Parallel()
	for _, entry := range []string{"run", "settled"} {
		t.Run(entry, func(t *testing.T) {
			t.Parallel()
			b := newScopeBed(t)
			base := b.run(t)
			b.base = base
			b.git.tree, b.git.commit = "ledger-tree", "ledger-commit"
			b.git.changed[[2]string{base.Tree, b.git.tree}] = "metasystem/plans/goals/fix.md"
			b.seams.Git = b.git.run
			b.seams.NewID = func() string { return "inherited" }
			var inherited Result
			if entry == "run" {
				inherited = b.run(t)
			} else {
				var ok bool
				var err error
				inherited, ok, err = Settled(b.install, b.checkout, b.seams)
				if err != nil || !ok {
					t.Fatalf("ledger settlement: %+v %v %v", inherited, ok, err)
				}
			}
			if inherited.Result != Green || inherited.Scope != "scoped" || inherited.Base != base.Tree || inherited.FullTree != base.FullTree || inherited.FullAt != base.FullAt || inherited.Environment != base.Environment || len(inherited.Ran) != 0 || len(b.calls) != 1 || inherited.Reason != inheritedReason(base) {
				t.Fatalf("inherited proof lost its source or ran again: %+v, calls %d", inherited, len(b.calls))
			}
			data, err := os.ReadFile(scopePath(b.install, inherited.Attempt))
			var record scopeRecord
			if err != nil || json.Unmarshal(data, &record) != nil || len(record.Groups) != 3 || record.Groups[0].State != "kept" || record.Groups[0].From != base.Tree || record.Groups[0].DurationMS != 12 {
				t.Fatalf("inherited group provenance: %+v %v", record, err)
			}
			writePushRecords(t, b.install, Pushed{Tree: inherited.Tree, At: bedNow.Format(time.RFC3339)})
			now := bedNow.Add(51 * time.Minute)
			b.seams.Now = func() time.Time { return now }
			if reasons, err := WakeReasons(b.install, b.checkout, bedNow, now); err != nil || !reflect.DeepEqual(reasons, []string{WakeFullDue}) {
				t.Fatalf("idle inherited push did not wake: %v %v", reasons, err)
			}
			if _, ok, err := Settled(b.install, b.checkout, b.seams); err != nil || ok {
				t.Fatalf("aged inherited proof settled: %v %v", ok, err)
			}
			launches := 0
			b.seams.Executable = func() (string, error) { return "engine", nil }
			b.seams.Launch = func([]string, string, string) (int64, error) { launches++; return int64(os.Getpid()), nil }
			b.seams.NewID = func() string { return "owed" }
			b.seams.Alive = func(Running) bool { return true }
			running, already, err := Start(b.install, b.checkout, b.seams)
			if err != nil || already || launches != 1 {
				t.Fatalf("owed proof did not start: %+v %v %v, launches %d", running, already, err, launches)
			}
			paid, err := Run(b.install, b.checkout, "proof-command", running.Attempt, b.output, b.seams)
			if err != nil || paid.Result != Green || paid.Scope != "full" || paid.FullTree != paid.Tree || paid.FullAt != now.Format(time.RFC3339) || len(b.calls) != 2 || commandEnv(b.calls[1], "LANDING_PROOF_SCOPE") != "full" {
				t.Fatalf("owed full proof: %+v %v, calls %d", paid, err, len(b.calls))
			}
			if reasons, err := WakeReasons(b.install, b.checkout, bedNow, now); err != nil || !reflect.DeepEqual(reasons, []string{WakeFullDue}) {
				t.Fatalf("batch proof paid the hourly debt but must still wake main's independent clock: %v %v", reasons, err)
			}
		})
	}
}

func TestARedScopedProofNamesItsGroupsAndAllowsOneWholeRepeat(t *testing.T) {
	t.Parallel()
	b := newScopeBed(t)
	b.git.changed[[2]string{b.base.Tree, b.git.tree}] = "metasystem/records/test-image.json"
	// The repeat rule requires isolated greens before a whole retry.
	b.seams.Command = func(command *exec.Cmd) error {
		b.calls = append(b.calls, command)
		if commandEnv(command, "LANDING_ONLY") != "" {
			fmt.Fprint(command.Stdout, "LANDING-CHECKED\t0\n")
			return nil
		}
		fmt.Fprint(command.Stdout, "landing environment image toolchain\nlanding group unrelated failed 7\nLANDING-FAILED\tu/a\tTestA\nLANDING-LOAD\t2.75\nLANDING-CHECKED\t1\n")
		return errors.New("check failed")
	}
	b.seams.Judge = func(string, string, []FailedUnit) (map[string]UnitJudgement, error) {
		return map[string]UnitJudgement{"u/a": {Surfaces: []string{"records"}}}, nil
	}
	red := b.run(t)
	if red.Result != Red || red.Scope != "scoped" || red.Base != b.base.Tree || red.FullAt != b.base.FullAt || !reflect.DeepEqual(red.Ran, []string{"unrelated"}) || len(red.Failed) != 1 || red.Failed[0].Unit != "u/a" || red.Load != 2.75 || red.Repeat != "allowed" || commandEnv(b.calls[0], "LANDING_PROOF_GROUPS") != "unrelated" {
		t.Fatalf("scoped failure: %+v", red)
	}
	if record := b.record(t); record.Groups[1].State != "ran" || !reflect.DeepEqual(record.Groups[1].Why, []string{"metasystem/records/test-image.json"}) {
		t.Fatalf("failed group's scope record: %+v", record)
	}
	b.seams.NewID = func() string { return "whole-repeat" }
	b.seams.Command = func(command *exec.Cmd) error {
		b.calls = append(b.calls, command)
		fmt.Fprint(command.Stdout, "landing environment image toolchain\nlanding group unrelated passed 8\n")
		return nil
	}
	var records []FlakeRecord
	b.seams.RecordFlake = func(record FlakeRecord) (FlakeRecorded, error) {
		records = append(records, record)
		return FlakeRecorded{Goal: "fix-flaky-a", Seen: 1}, nil
	}
	green := b.run(t)
	if green.Result != Green || green.Scope != "scoped" || green.FullAt != b.base.FullAt || len(records) != 1 || records[0].Repeat != "whole" || records[0].RepeatAttempt != green.Attempt || !strings.Contains(green.Reason, "goal fix-flaky-a") || len(b.calls) != 3 {
		t.Fatalf("scoped whole repeat: %+v, records %+v", green, records)
	}
}

func TestAScopedFlakeRepeatsWithTheEffectiveScope(t *testing.T) {
	t.Parallel()
	for _, changedEnvironment := range []bool{false, true} {
		t.Run(fmt.Sprint(changedEnvironment), func(t *testing.T) {
			t.Parallel()
			b := newScopeBed(t)
			b.seams.Command = func(command *exec.Cmd) error {
				b.calls = append(b.calls, command)
				log, ok := command.Stdout.(*os.File)
				if !ok || command.Stderr != log || command.WaitDelay != 0 {
					t.Fatal("flake check did not receive a log file")
				}
				environment := b.base.Environment
				if changedEnvironment {
					environment = "new-image"
				}
				fmt.Fprintln(log, "landing environment "+environment)
				if commandEnv(command, "LANDING_ONLY") != "" {
					return nil
				}
				fmt.Fprint(log, "landing group plans failed 9\nLANDING-FAILED\tu/a\tTestA\nLANDING-LOAD\t1.5\nLANDING-CHECKED\t1\n")
				return errors.New("failed check")
			}
			judgements := 0
			b.seams.Judge = func(string, string, []FailedUnit) (map[string]UnitJudgement, error) {
				judgements++
				return map[string]UnitJudgement{"u/a": {Known: true, Surfaces: []string{"surface-a"}}}, nil
			}
			var records []FlakeRecord
			b.seams.RecordFlake = func(record FlakeRecord) (FlakeRecorded, error) {
				records = append(records, record)
				return FlakeRecorded{Goal: "fix-flaky-a", Seen: 1}, nil
			}
			result := b.run(t)
			wantScope, wantCalls, wantGroups := "scoped", 2, "plans shared"
			if changedEnvironment {
				wantScope, wantCalls, wantGroups = "full", 3, ""
			}
			if result.Result != Green || result.Scope != wantScope || len(b.calls) != wantCalls || judgements != 1 || len(records) != 1 || records[0].Repeat != "alone" || records[0].Load != 1.5 || !strings.Contains(result.Reason, "goal fix-flaky-a") {
				t.Fatalf("effective flake proof: %+v, calls %d, judgements %d, records %+v", result, len(b.calls), judgements, records)
			}
			last := b.calls[len(b.calls)-1]
			if commandEnv(last, "LANDING_ONLY") != "u/a" || commandEnv(last, "LANDING_PROOF_SCOPE") != wantScope || commandEnv(last, "LANDING_PROOF_GROUPS") != wantGroups {
				t.Fatal("unit repeat lost the effective scope")
			}
			lines, err := Results(b.install)
			if err != nil || len(lines) != 3 || lines[1].Scope != wantScope || lines[1].Result != Red || lines[1].Repeat != "started" {
				t.Fatalf("repeat allowance did not record scope: %+v %v", lines, err)
			}
			if changedEnvironment && (result.FullAt != result.At || result.FullTree != result.Tree) {
				t.Fatalf("full fallback did not reset the full proof clock: %+v", result)
			}
		})
	}
}
