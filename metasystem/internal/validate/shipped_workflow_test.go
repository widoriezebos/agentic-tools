package validate

// The workflow-tooling-fixtures section of the retired validate-metasystem.sh
// (verbs-object-action U7b), where existing unit tests did not already prove
// the same behaviour: the shipped documentation examples against their gates,
// the debug-java preflight helper, the stop-loss no-gain opt-out, and the
// refactor baseline's --file normalization with Git stubbed.

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testgit"
)

// The shipped example matrix still carries a READY_FOR_RUNTIME row: it passes
// the default gate and is refused under --runtime-required, which keeps the
// negative half of the gate honest against a real document.
func TestShippedDesignObligationExampleHoldsBothGateModes(t *testing.T) {
	t.Parallel()
	example := filepath.Join("..", "..", "docs", "examples", "design-obligation-matrix.md")
	if out, errs, code := DesignObligations(".", []string{example}, false); code != 0 {
		t.Fatalf("example matrix refused by the default gate: %v %v", out, errs)
	}
	if _, _, code := DesignObligations(".", []string{example}, true); code == 0 {
		t.Fatal("example matrix with READY_FOR_RUNTIME passed --runtime-required; the negative example is broken")
	}
}

func TestShippedStepBackLedgerExamplePassesStopLoss(t *testing.T) {
	t.Parallel()
	if out, errs, code := StopLoss(filepath.Join("..", "..", "docs", "examples", "step-back-ledger.md")); code != 0 {
		t.Fatalf("shipped step-back ledger example tripped stop-loss: %v %v", out, errs)
	}
}

// Without a declared no-gain budget, unresolved cycles never trip the no-gain
// rule, however many there are.
func TestStopLossNoGainRuleIsOptIn(t *testing.T) {
	t.Parallel()
	ledger := writeLedger(t, "### Cycle E1\n- Classification: unresolved\n### Cycle E2\n- Classification: unresolved\n### Cycle E3\n- Classification: unresolved\n")
	if out, errs, code := StopLoss(ledger); code != 0 {
		t.Fatalf("stop-loss blocked unresolved cycles without a declared no-gain budget: %v %v", out, errs)
	}
}

// The optional debug-java skill's preflight refuses an artifact older than
// its source. The helper is a skill extension point and stays a script.
func TestShippedDebugJavaPreflightRefusesAStaleArtifact(t *testing.T) {
	t.Parallel()
	preflight, err := filepath.Abs(filepath.Join("..", "..", "optional-skills", "debug-java", "scripts", "preflight.sh"))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	source, artifact := filepath.Join(dir, "source"), filepath.Join(dir, "artifact")
	for _, path := range []string{source, artifact} {
		if err := os.WriteFile(path, []byte("x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	built, edited := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC), time.Date(2026, 1, 1, 12, 5, 0, 0, time.UTC)
	run := func() (string, error) {
		command := exec.Command("bash", preflight, "--source", source, "--artifact", artifact)
		var output bytes.Buffer
		command.Stdout, command.Stderr = &output, &output
		err := command.Run()
		return output.String(), err
	}
	if err := os.Chtimes(source, built, built); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(artifact, edited, edited); err != nil {
		t.Fatal(err)
	}
	if output, err := run(); err != nil || !strings.Contains(output, "debug preflight passed") {
		t.Fatalf("fresh artifact refused: %v %s", err, output)
	}
	if err := os.Chtimes(artifact, built.Add(-time.Hour), built.Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	if output, err := run(); err == nil || !strings.Contains(output, "stale artifact") {
		t.Fatalf("debug preflight accepted a stale artifact: %v %s", err, output)
	}
}

// A baseline recorded at a custom relative, an in-repository absolute, a
// non-ASCII or a space-containing --file is checked right after record
// without the file's own dirt blocking: the path normalizes to the
// repository root and compares literally against NUL-delimited porcelain,
// which Git never C-quotes. Git is stubbed; its transcript is the gate's
// whole external surface.
func TestRefactorBaselineNormalizesEveryFileForm(t *testing.T) {
	t.Parallel()
	sha := strings.Repeat("c", 40)
	now := time.Unix(1_790_000_000, 0)
	for _, test := range []struct {
		name, file, porcelain string
	}{
		{"custom relative", "plans/custom-baseline", "plans/custom-baseline"},
		{"in-repository absolute", "", "plans/abs-baseline"},
		{"non-ASCII", "plans/bäseline", "plans/bäseline"},
		{"space-containing", "plans/my baseline", "plans/my baseline"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			repo, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			file := test.file
			if file == "" {
				file = filepath.Join(repo, "plans", "abs-baseline")
			}
			call := func(dir string, args ...string) testgit.Expectation {
				return testgit.Expectation{Call: testgit.Call{Dir: dir, Args: args}}
			}
			returning := func(expectation testgit.Expectation, stdout string) testgit.Expectation {
				expectation.Result.Stdout = []byte(stdout)
				return expectation
			}
			stub := testgit.New(t,
				returning(call(repo, "rev-parse", "--is-inside-work-tree"), "true\n"),
				returning(call(repo, "rev-parse", "--show-toplevel"), repo+"\n"),
				returning(call(repo, "status", "--porcelain"), ""),
				returning(call(repo, "rev-parse", "HEAD"), sha+"\n"),
				returning(call(repo, "rev-parse", "--is-inside-work-tree"), "true\n"),
				returning(call(repo, "rev-parse", "--show-toplevel"), repo+"\n"),
				returning(call(repo, "rev-parse", "--verify", "--quiet", sha+"^{commit}"), sha+"\n"),
				returning(call(repo, "status", "--porcelain", "--untracked-files=all", "-z"), "?? "+test.porcelain+"\x00"),
				call(repo, "merge-base", "--is-ancestor", sha, "HEAD"),
				returning(call(repo, "rev-list", "--count", sha+"..HEAD"), "0\n"),
			)
			git := func(dir string, args ...string) (string, error) {
				result := stub.Run(testgit.Call{Dir: dir, Args: args})
				return string(result.Stdout), result.Err
			}
			params := RefactorBaselineParams{Cwd: repo, File: file, Gate: "declared acceptance gate",
				MaxAgeMinutes: 1440, MaxCommits: 40, Git: git, Now: func() time.Time { return now }}
			var out, errOut strings.Builder
			params.Command = "record"
			if code := RefactorBaseline(params, &out, &errOut); code != 0 {
				t.Fatalf("record at %q: code=%d err=%q", file, code, errOut.String())
			}
			recorded, err := os.ReadFile(filepath.Join(repo, filepath.FromSlash(test.porcelain)))
			if err != nil || !strings.HasPrefix(string(recorded), "sha="+sha+"\n") {
				t.Fatalf("record did not write the baseline at %s: %q %v", test.porcelain, recorded, err)
			}
			params.Command = "check"
			out.Reset()
			errOut.Reset()
			if code := RefactorBaseline(params, &out, &errOut); code != 0 || !strings.Contains(out.String(), "refactor baseline safe: "+sha) {
				t.Fatalf("check right after record at %q blocked on the baseline file's own dirt: code=%d out=%q err=%q", file, code, out.String(), errOut.String())
			}
		})
	}
}
