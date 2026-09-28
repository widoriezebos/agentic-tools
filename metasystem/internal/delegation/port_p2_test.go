package delegation_test

import (
	"os"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/delegation/fake"
)

// Fixture lines 1229-1260 (build-stamp skew): an unknown stamp is allowed
// silently, a stamp older than a checkout that changed an agent script
// refuses with both commits and the rebuild remedy, and nothing is refused
// when only files outside the engine and agent sources moved.
func TestPortP2EngineSkewPreflight(t *testing.T) {
	t.Parallel()
	const stamp = "abc1234"
	const head = "0123456789abcdef0123456789abcdef01234567"
	logKey := func(s string) string { return "*|log --format=commit %H --name-only --ancestry-path " + s + "..HEAD" }

	t.Run("unknown stamp is silent", func(t *testing.T) {
		t.Parallel()
		b := newBed(t)
		b.doubles.Git.Responses[logKey("fixture-unknown-stamp")] = fake.GitResponse{Code: 128, Stderr: "fatal: bad revision"}
		result := b.run("__engine-skew-preflight", "fixture-unknown-stamp")
		requireExit(t, result, 0, b.stderr.String())
		if b.stderr.Len() != 0 || len(result.Stdout) != 0 || len(result.Outcome) != 0 {
			t.Fatalf("an unknown stamp was not silent: stdout %q stderr %q outcome %q", result.Stdout, b.stderr.String(), result.Outcome)
		}
	})

	t.Run("agent script change refuses", func(t *testing.T) {
		t.Parallel()
		b := newBed(t)
		b.doubles.Git.Responses[logKey(stamp)] = fake.GitResponse{Stdout: "commit " + head + "\n\nscripts/agents/dispatch.sh\n"}
		result := b.run("__engine-skew-preflight", stamp)
		requireExit(t, result, 1, b.stderr.String())
		stderr := b.stderr.String()
		if !strings.Contains(stderr, "dispatch refused: engine commit "+stamp+" is older than checkout commit "+head) ||
			!strings.Contains(stderr, "run scripts/agents/go-build.sh, then steward arm") {
			t.Fatalf("the skew refusal did not name both commits and the rebuild remedy: %q", stderr)
		}
		if outcome := outcomeOf(t, result); outcome["outcome"] != "REFUSED-INTERNAL" {
			t.Fatalf("outcome %v", outcome)
		}
	})

	t.Run("unrelated change is admitted", func(t *testing.T) {
		t.Parallel()
		b := newBed(t)
		b.doubles.Git.Responses[logKey(stamp)] = fake.GitResponse{Stdout: "commit " + head + "\n\nREADME.md\ndocs/guide.md\n"}
		requireExit(t, b.run("__engine-skew-preflight", stamp), 0, b.stderr.String())
	})

	t.Run("dev stamp never reads git", func(t *testing.T) {
		t.Parallel()
		b := newBed(t)
		requireExit(t, b.run("__engine-skew-preflight", "dev"), 0, b.stderr.String())
		if calls := b.calls("git "); len(calls) != 0 {
			t.Fatalf("a dev stamp read git: %v", calls)
		}
	})

	t.Run("more than one stamp is usage", func(t *testing.T) {
		t.Parallel()
		b := newBed(t)
		requireExit(t, b.run("__engine-skew-preflight", "a", "b"), 2, b.stderr.String())
	})
}

// Fixture no-role-default (lines 1705-1706): with neither a role entry nor
// role.default.runtime the dispatch refuses with the typed roster refusal
// before any claim, record or launch.
func TestPortP2DispatchWithoutAnyRosterRefuses(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	b.writeFile("metasystem.conf", "metasystem.runtimes=fake\nevidence.root="+b.root+"/../evidence\n")
	b.writeFile("scripts/agents/roles/verifier.md", "# Verifier\n")
	b.writeFile("scripts/agents/roles/verifier.requirements.json", "{}\n")
	brief := b.writeFile("verifier.md", "Working Mode: verify\n")
	result := b.run("dispatch", "--role", "verifier", "--brief", brief, "--permissions", "none",
		"--destructive-reach", "MECHANICAL", "--job-id", "no-role-default")
	requireExit(t, result, 1, b.stderr.String())
	if !strings.Contains(b.stderr.String(), "role verifier has neither a runtime entry nor role.default.runtime") {
		t.Fatalf("stderr %q", b.stderr.String())
	}
	if outcome := outcomeOf(t, result); outcome["outcome"] != "REFUSED-ROSTER" {
		t.Fatalf("outcome %v", outcome)
	}
	if _, err := os.Stat(b.recordPath("no-role-default")); !os.IsNotExist(err) {
		t.Fatalf("a roster refusal left a job record: %v", err)
	}
	if len(b.calls("adapter.")) != 0 || len(b.calls("records.")) != 0 {
		t.Fatalf("a roster refusal reached the adapter or the record owner: %v", b.doubles.Log.Calls())
	}
}
