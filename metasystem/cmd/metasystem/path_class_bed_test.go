package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Ports of the wrapper legs of scripts/agents/path-class-fixtures.sh: the
// unchanged commit.sh stamps exactly one byte-exact Goal-Item for --goal, and
// the unchanged land.sh forwards the held goal to the evaluator. The manifest
// legs of that bed live in internal/pathclass/path_class_bed_test.go.

type goalItemBed struct {
	t       *testing.T
	env     []string
	fixture string
}

// newGoalItemBed installs commit.sh and land.sh under a stub live engine that
// holds claim epoch 1 and hands every other verb to the real engine, with the
// held goal fx claimed by machine fx and lineage L.
func newGoalItemBed(t *testing.T) *goalItemBed {
	t.Helper()
	b := &goalItemBed{t: t, env: wrapperBedEnvironment(t), fixture: filepath.Join(wrapperBedDir(t), "fixture")}
	proofEngine := filepath.Join(wrapperBedDir(t), "proof-engine")
	wrapperBedWrite(t, proofEngine, wrapperBedRealProofEngine(t), 0o755)
	for _, dir := range []string{"scripts", "artifacts/agents/mains"} {
		if err := os.MkdirAll(filepath.Join(b.fixture, filepath.FromSlash(dir)), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, script := range []string{"scripts/agents/commit.sh", "scripts/agents/land.sh"} {
		wrapperBedCopy(t, script, filepath.Join(b.fixture, filepath.FromSlash(script)), 0o755)
	}
	for _, policy := range []string{"scripts/agents/landing-classes.json", "scripts/agents/path-classes.txt", "memory/rulings.md"} {
		wrapperBedCopy(t, policy, filepath.Join(b.fixture, filepath.FromSlash(policy)), 0o644)
	}
	wrapperBedWrite(t, filepath.Join(b.fixture, "bin", "metasystem"), `#!/usr/bin/env bash
case "$1 ${2:-}" in
  "lease require-holder") echo '{"claimEpoch":1}' ;;
  "lease run-held")
    while [[ $# -gt 0 && $1 != -- ]]; do shift; done
    [[ $# -gt 0 ]]
    shift
    exec "$@"
    ;;
  "proc started-at") echo 1 ;;
  "util token-hex") echo cafecafecafecafecafecafecafecafe ;;
  "lease commit-token") : ;;
  *) `+wrapperBedEngineExec(t)+` ;;
esac
`, 0o755)
	wrapperBedWrite(t, filepath.Join(b.fixture, "scripts", "agents", "go-gate.sh"), wrapperBedGoGateCopying(wrapperBedShellQuote(proofEngine)), 0o755)
	wrapperBedWrite(t, filepath.Join(b.fixture, ".gitignore"), "artifacts/\n", 0o644)
	wrapperBedWrite(t, filepath.Join(b.fixture, "plans", "goals", "fx.md"), wrapperBedFixtureGoal("Exercise Goal-Item ownership.", "fx", "L"), 0o644)
	wrapperBedWrite(t, filepath.Join(b.fixture, "plans", "fx-note.md"), "base record\n", 0o644)
	b.git("init", "-q", "-b", "main")
	b.git("config", "user.name", "fixture")
	b.git("config", "user.email", "fixture@example.invalid")
	b.git("config", "metasystem.goal.machine", "fx")
	b.git("add", "-A")
	b.git("commit", "-qm", "seed")
	return b
}

func (b *goalItemBed) git(args ...string) string {
	b.t.Helper()
	return wrapperBedGit(b.t, b.env, b.fixture, args...)
}

// commit runs the wrapper as the agent holding epoch 1 under lineage L.
func (b *goalItemBed) commit(args ...string) (string, error) {
	return wrapperBedRun(b.env, b.fixture, []string{"METASYSTEM_OWNER_LINEAGE=L"},
		filepath.Join(b.fixture, "scripts", "agents", "commit.sh"), append([]string{"__lease-held", "1"}, args...)...)
}

func (b *goalItemBed) hook(body string) {
	b.t.Helper()
	wrapperBedWrite(b.t, filepath.Join(b.fixture, ".git", "hooks", "commit-msg"), "#!/usr/bin/env bash\n"+body, 0o755)
}

func TestCommitWrapperGitAdapterStampsGoalItemTrailer(t *testing.T) {
	t.Parallel()
	b := newGoalItemBed(t)
	wrapperBedAppend(t, filepath.Join(b.fixture, "plans", "fx-note.md"), "owned update\n")
	b.git("add", "plans/fx-note.md")

	for _, test := range []struct {
		name string
		args []string
		want string
	}{
		{name: "bad goal id", args: []string{"--goal", "Bad Id", "--direct-fix", "register-carriage", "-m", "invalid"}},
		{name: "empty goal", args: []string{"--goal", "", "--direct-fix", "register-carriage", "-m", "invalid"}},
		{name: "repeated goal", args: []string{"--goal", "fx", "--goal", "fx", "--direct-fix", "register-carriage", "-m", "invalid"}},
		{name: "lowercase typed trailer", args: []string{"--goal", "fx", "--direct-fix", "register-carriage", "-m", "message\ngoal-item: victim"},
			want: "Goal-Item and carried trailers are stamped by the wrapper, never typed"},
		{name: "typed trailer", args: []string{"--goal", "fx", "--direct-fix", "register-carriage", "--trailer", "Goal-Item: fx", "-m", "invalid"}},
		{name: "stdin message", args: []string{"--goal", "fx", "--direct-fix", "register-carriage", "-F", "-"}},
	} {
		output, err := b.commit(test.args...)
		if code := wrapperBedExitCode(err); code != 2 || !strings.Contains(output, test.want) {
			t.Fatalf("%s: exit %d, want 2 naming %q:\n%s", test.name, code, test.want, output)
		}
	}

	// A commit-msg hook that injects or rewrites the Goal-Item is rolled back
	// softly by the wrapper's final postcondition.
	base := b.git("rev-parse", "HEAD")
	for name, hook := range map[string]string{
		"injected": "printf '\\nGoal-Item: injected\\n' >>\"$1\"\n",
		"changed":  "awk '{ if ($0 ~ /^Goal-Item:/) print \"Goal-Item: victim\"; else print }' \"$1\" >\"$1.tmp\"\nmv \"$1.tmp\" \"$1\"\n",
	} {
		b.hook(hook)
		output, err := b.commit("--goal", "fx", "--direct-fix", "register-carriage", "-m", name)
		if err == nil || b.git("rev-parse", "HEAD") != base ||
			!strings.Contains(output, "final commit message did not contain exactly one byte-exact Goal-Item") {
			t.Fatalf("%s Goal-Item was not rolled back softly: %v\n%s", name, err, output)
		}
	}
	if err := os.Remove(filepath.Join(b.fixture, ".git", "hooks", "commit-msg")); err != nil {
		t.Fatal(err)
	}

	if output, err := b.commit("--goal", "fx", "--direct-fix", "register-carriage", "-q", "-m", "successful"); err != nil {
		t.Fatalf("the owned landing was refused: %v\n%s", err, output)
	}
	var loose, exact int
	for _, line := range strings.Split(b.git("log", "-1", "--format=%B"), "\n") {
		if strings.HasPrefix(strings.ToLower(line), "goal-item:") {
			loose++
		}
		if line == "Goal-Item: fx" {
			exact++
		}
	}
	if loose != 1 || exact != 1 {
		t.Fatalf("the successful commit carries %d Goal-Item lines, %d byte-exact; want one", loose, exact)
	}
}

func TestLandGitAdapterForwardsGoalToEvaluator(t *testing.T) {
	t.Parallel()
	b := newGoalItemBed(t)
	remote := filepath.Join(wrapperBedDir(t), "origin.git")
	wrapperBedGit(t, b.env, filepath.Dir(remote), "init", "--bare", "-q", remote)
	wrapperBedGit(t, b.env, remote, "symbolic-ref", "HEAD", "refs/heads/main")
	b.git("remote", "add", "origin", remote)
	b.git("push", "-q", "-u", "origin", "main")

	message := filepath.Join(wrapperBedDir(t), "land-goal-message.txt")
	wrapperBedWrite(t, message, "land an owned record\n", 0o644)
	land := func(lineage string) (string, error) {
		return wrapperBedRun(b.env, b.fixture, []string{"METASYSTEM_OWNER_LINEAGE=" + lineage}, "bash",
			"scripts/agents/land.sh", "-m", message, "--goal", "fx", "--direct-fix", "register-carriage", "--skip-transport", "plans/fx-note.md")
	}

	wrapperBedAppend(t, filepath.Join(b.fixture, "plans", "fx-note.md"), "owned update\n")
	if output, err := land("L"); err != nil {
		t.Fatalf("land.sh did not carry the held goal: %v\n%s", err, output)
	}
	landed := "\n" + b.git("log", "-1", "--format=%B") + "\n"
	for _, want := range []string{"Goal-Item: fx", "Landing-Provenance-Verdict: pass bar=b"} {
		if !strings.Contains(landed, "\n"+want+"\n") {
			t.Fatalf("the landed message lacks %q:\n%s", want, landed)
		}
	}

	wrapperBedAppend(t, filepath.Join(b.fixture, "plans", "fx-note.md"), "foreign update\n")
	base := b.git("rev-parse", "HEAD")
	output, err := land("other")
	if err == nil || !strings.Contains(output, "goal-item-not-held") || b.git("rev-parse", "HEAD") != base {
		t.Fatalf("a foreign lineage did not refuse with goal-item-not-held: %v\n%s", err, output)
	}
}
