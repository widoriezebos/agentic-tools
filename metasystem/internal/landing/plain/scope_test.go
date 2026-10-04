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
	b.git = stubGit{commit: "head-commit", tree: "head-tree",
		batches: map[string]string{"head-commit": "batch-b\nbatch-a", "base-commit": "batch-a\nbatch-b"},
		changed: map[[2]string]string{{b.base.Tree, "head-tree"}: "metasystem/plans/page.md"}, shows: map[string]string{}}
	b.contract = testpolicy.Contract{SchemaVersion: testpolicy.ExecutionContractSchemaVersion,
		ProjectRisk: testpolicy.ProjectRisk{Severity: 1, Exposure: 1, Reversibility: "revert", Detection: "immediate", Recovery: "bounded"},
		Surfaces:    []testpolicy.Surface{{ID: "app", Paths: []string{"metasystem/**"}, Standard: []string{"plans"}}}, Unknown: []string{"plans"}}
	for i, id := range []string{"plans", "unrelated", "shared"} {
		input := []string{"metasystem/plans/**", "assets/**", "metasystem/**"}[i]
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
	b.git.changed[[2]string{result.Tree, b.git.tree}] = "assets/image.png"
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
				want = "testing contract changed"
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
				b.git.changed[[2]string{b.base.Tree, b.git.tree}] = "metasystem/plans/page.md\nREADME.md"
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
				if command.Stdout != output || command.Stderr != output {
					t.Fatal("proof did not receive the original output")
				}
			}
		})
	}
}
