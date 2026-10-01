package main

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/boundedexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testgit"
)

func TestLandingReceiptForwardsBothExpectedRevisions(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	controlRoot, err := canonicalProofRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("exec.local-timeout-sec=7\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "file"), []byte("one\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	const tree = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	pins := []string{"-C", controlRoot, "-c", "core.fileMode=true", "-c", "diff.noprefix=false",
		"-c", "diff.mnemonicPrefix=false", "-c", "apply.ignoreWhitespace=no",
		"-c", "core.logAllRefUpdates=false", "-c", "core.useReplaceRefs=false",
		"-c", "gc.auto=0", "-c", "maintenance.auto=false"}
	expect := func(output string, args ...string) testgit.Expectation {
		return testgit.Expectation{Call: testgit.Call{
			Dir: controlRoot, Args: append(slices.Clone(pins), args...), Env: gittree.ScrubbedEnviron(),
		}, Result: testgit.Result{Stdout: []byte(output)}}
	}
	stub := testgit.New(t,
		expect(controlRoot+"\n", "rev-parse", "--show-toplevel"),
		expect(tree+"\n", "rev-parse", "--verify", tree),
		expect("tree\n", "cat-file", "-t", tree),
	)
	operations := []string{"git rev-parse --show-toplevel", "git rev-parse --verify " + tree, "git cat-file -t " + tree}
	wantTimeout := boundedexec.Bound{Limit: 7 * time.Second, Key: "exec.local-timeout-sec"}
	raw := func(request gittree.RawRequest) gittree.RawResult {
		t.Helper()
		index := len(stub.Calls())
		if index >= len(operations) {
			t.Fatalf("unexpected raw Git request: %+v", request)
		}
		if request.Stdin != nil || request.Operation != operations[index] || request.Timeout != wantTimeout {
			t.Fatalf("raw Git request %d has stdin=%q operation=%q timeout=%+v", index+1, request.Stdin, request.Operation, request.Timeout)
		}
		result := stub.Run(testgit.Call{Dir: request.Dir, Args: request.Args, Env: request.Env, Stdin: request.Stdin})
		return gittree.RawResult{Stdout: result.Stdout, Stderr: result.Stderr, Err: result.Err}
	}
	var forwarded []string
	testRun := func(args []string, _, _ io.Writer) int {
		forwarded = append([]string(nil), args...)
		return proofrun.ExitAdmissionRefused
	}
	code := runLandingTestReceiptWithDependencies(t.Output(), t.Output(), context.Background(), goalCommandClock, raw, testRun,
		[]string{"--root", root, "--tree", tree, "--mode", "auto", "--goal", "goal-a", "--expected-goal-revision", "7", "--expected-accounting-revision", "5"})
	resultPath := filepath.Join(controlRoot, "artifacts", "agents", "proof-runs", "delivery", "testing-result-"+tree+".json")
	wantForwarded := []string{"--root", controlRoot, "--tree", tree, "--mode", "auto", "--purpose", "delivery",
		"--result", resultPath, "--goal", "goal-a", "--expected-goal-revision", "7", "--expected-accounting-revision", "5"}
	if code != proofrun.ExitAdmissionRefused || !slices.Equal(forwarded, wantForwarded) {
		t.Fatalf("code=%d forwarded=%v, want code=%d forwarded=%v", code, forwarded, proofrun.ExitAdmissionRefused, wantForwarded)
	}
	if calls := stub.Calls(); len(calls) != 3 {
		t.Fatalf("raw Git calls=%d, want 3", len(calls))
	}
	if _, err := os.Stat(filepath.Join(root, "artifacts")); !os.IsNotExist(err) {
		t.Fatalf("refused run created receipt artifacts: %v", err)
	}
}
