package steward

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/behaviorsurface"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testgit"
)

type rearmExitStatus int

func (status rearmExitStatus) Error() string { return fmt.Sprintf("exit status %d", status) }
func (status rearmExitStatus) ExitCode() int { return int(status) }

func rearmExpected(root string, output string, err error, args ...string) testgit.Expectation {
	return testgit.Expectation{
		Call:   testgit.Call{Dir: root, Args: args},
		Result: testgit.Result{Stdout: []byte(output), Err: err},
	}
}

func rearmFailed(root string, status int, args ...string) testgit.Expectation {
	return testgit.Expectation{
		Call:   testgit.Call{Dir: root, Args: args},
		Result: testgit.Result{Stderr: []byte("fatal: not a git repository (or any of the parent directories)\n"), Err: rearmExitStatus(status)},
	}
}

func rearmID(n int) string { return fmt.Sprintf("%040x", n) }

func rearmAncestor(root, source, landed string, status int) testgit.Expectation {
	if status != 0 {
		return rearmFailed(root, status, "merge-base", "--is-ancestor", source, landed)
	}
	return rearmExpected(root, "", nil, "merge-base", "--is-ancestor", source, landed)
}

func rearmSkew(root, source, landed, output string) testgit.Expectation {
	args := []string{"log", "--format=", "--name-only", "--ancestry-path", "--diff-merges=first-parent", source + ".." + landed, "--"}
	args = append(args, enrollmentSkewPathspecs[:]...)
	return rearmExpected(root, output, nil, args...)
}

func rearmBuild(root, stamp, resolved string, status int) testgit.Expectation {
	if status != 0 {
		return rearmFailed(root, status, "rev-parse", "--verify", "--quiet", stamp+"^{commit}")
	}
	return rearmExpected(root, resolved+"\n", nil, "rev-parse", "--verify", "--quiet", stamp+"^{commit}")
}

func rearmDiff(root, source, destination, output string) testgit.Expectation {
	return rearmExpected(root, output, nil, "diff-tree", "-r", "-z", "--no-commit-id", "--no-abbrev", "--no-renames", "--no-ext-diff", "--ignore-submodules=none", "--relative", source, destination)
}

func rearmRawChange(path string, oldID, newID string) string {
	oldMode, newMode := "100644", "100644"
	if oldID == strings.Repeat("0", 40) {
		oldMode = "000000"
	}
	if newID == strings.Repeat("0", 40) {
		newMode = "000000"
	}
	return fmt.Sprintf("\n:%s %s %s %s M\x00%s\x00", oldMode, newMode, oldID, newID, path)
}

func rearmHistory(commit, tree, parent, changes string) string {
	parents := ""
	if parent != "" {
		parents = " " + parent
	}
	return fmt.Sprintf("\x01%s %s%s\x00%s", commit, tree, parents, changes)
}

func rearmOwnedRef(root, ref, commit string) []testgit.Expectation {
	return []testgit.Expectation{
		rearmExpected(root, ref+"\n", nil, "config", "--local", "--no-includes", "--get", landingRefConfigKey),
		rearmExpected(root, commit+"\n", nil, "rev-parse", "--verify", "--quiet", ref+"^{commit}"),
	}
}

func rearmBatch(root, input, output string) testgit.Expectation {
	return testgit.Expectation{
		Call:   testgit.Call{Dir: root, Args: []string{"cat-file", "--batch-check=%(objectname)"}, Stdin: []byte(input)},
		Result: testgit.Result{Stdout: []byte(output)},
	}
}

func rearmWitnessExpectations(root, ref, history string) []testgit.Expectation {
	return []testgit.Expectation{
		rearmExpected(root, root+"\n", nil, "rev-parse", "--show-toplevel"),
		rearmExpected(root, history, nil, "log", "--topo-order", "--no-abbrev", "--raw", "-z", "--no-renames",
			"--no-ext-diff", "--ignore-submodules=none", "--diff-merges=first-parent", "--format=%x01%H %T %P", ref),
	}
}

func rearmTestDeps(t *testing.T, expectations ...testgit.Expectation) rearmResolverDeps {
	t.Helper()
	stub := testgit.New(t, expectations...)
	run := func(root string, input []byte, args ...string) ([]byte, error) {
		result := stub.Run(testgit.Call{Dir: root, Args: args, Stdin: input})
		if result.Err != nil {
			return nil, fmt.Errorf("git %s: %w (%s)", strings.Join(args, " "), result.Err, strings.TrimSpace(string(result.Stderr)))
		}
		return result.Stdout, nil
	}
	deps := defaultRearmResolverDeps()
	deps.resolveSeconds = func(string) int { return defaultRearmResolveSeconds }
	deps.localLandingRef = func(root string) (string, error) {
		out, err := run(root, nil, "config", "--local", "--no-includes", "--get", landingRefConfigKey)
		return strings.TrimSpace(string(out)), err
	}
	deps.resolvingRef = func(root, ref string) (string, error) {
		out, err := run(root, nil, "rev-parse", "--verify", "--quiet", ref+"^{commit}")
		return strings.TrimSpace(string(out)), err
	}
	deps.checkoutHead = func(root string) (string, error) {
		out, err := run(root, nil, "rev-parse", "--verify", "HEAD^{commit}")
		return strings.TrimSpace(string(out)), err
	}
	deps.deadlineGit = func(_ RearmClock, _ int, _, root string, args ...string) (string, error) {
		out, err := run(root, nil, args...)
		return strings.TrimSpace(string(out)), err
	}
	deps.witnessGit = func(_ context.Context, root string, input []byte, progress func(), args ...string) ([]byte, error) {
		out, err := run(root, input, args...)
		if len(out) > 0 {
			progress()
		}
		return out, err
	}
	deps.witnessTreeDigest = func(context.Context, string, string, behaviorsurface.Policy, RearmClock, int) (string, error) {
		return "", fmt.Errorf("unexpected witness tree digest")
	}
	deps.archivedEngineDigest = func(context.Context, string, string, behaviorsurface.Policy, RearmClock, int) (string, error) {
		return "", fmt.Errorf("unexpected archived ENGINE digest")
	}
	deps.projectionDiff = func(_ context.Context, root, from, to string, progress func()) ([]byte, error) {
		out, err := run(root, nil, "diff-tree", "-r", "-z", "--no-commit-id", "--no-abbrev", "--no-renames",
			"--no-ext-diff", "--ignore-submodules=none", "--relative", from, to)
		if len(out) > 0 {
			progress()
		}
		return out, err
	}
	deps.notifyAvailable = func(string) bool { return true }
	deps.runnerExcluded = func(string, bool) (string, bool) { return "", false }
	return deps
}

func (bed rearmBed) successDeps(t *testing.T) rearmResolverDeps {
	t.Helper()
	ref := "refs/remotes/origin/trunk"
	return rearmTestDeps(t,
		rearmExpected(bed.root, ref+"\n", nil, "config", "--local", "--no-includes", "--get", landingRefConfigKey),
		rearmExpected(bed.root, bed.second+"\n", nil, "rev-parse", "--verify", "--quiet", ref+"^{commit}"),
		rearmExpected(bed.root, bed.second+"\n", nil, "rev-parse", "--verify", "--quiet", bed.second+"^{commit}"),
		rearmExpected(bed.root, "", nil, "merge-base", "--is-ancestor", bed.second, ref),
		rearmExpected(bed.root, bed.second+"\n", nil, "rev-parse", "--verify", "HEAD^{commit}"),
	)
}

func (bed rearmBed) rearm(t *testing.T) (ReArmOutcome, error) {
	t.Helper()
	return reArmRebuiltEngineWithDeps(bed.successDeps(t), bed.root, bed.root, bed.engine)
}

func (bed rearmBed) alreadyCurrent(t *testing.T) (ReArmOutcome, error) {
	t.Helper()
	return reArmRebuiltEngineWithDeps(rearmTestDeps(t), bed.root, bed.root, bed.engine)
}

func rearmHumanArm(t *testing.T, root, binary string, replace bool, word, reviewBy, enrollment string) (string, error) {
	t.Helper()
	mintedBy := "human-terminal"
	if word != "" {
		mintedBy = "human-word"
	}
	outcome, err := armWithRearmDeps(root, binary, replace, false, false,
		humanMintDecision(mintedBy, word, reviewBy, enrollment), rearmTestDeps(t))
	return outcome.Message, err
}
