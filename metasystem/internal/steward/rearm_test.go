package steward

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/behaviorsurface"
	processidentity "github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testgit"
)

func rearmGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func writeRearmFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := testexec.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func commitRearmTree(t *testing.T, root, value string) string {
	t.Helper()
	writeRearmFile(t, filepath.Join(root, "cmd", "surface.txt"), value+"\n")
	rearmGit(t, root, "add", ".")
	rearmGit(t, root, "commit", "-q", "-m", value)
	return rearmGit(t, root, "rev-parse", "HEAD")
}

func initRearmRepo(t *testing.T) string {
	t.Helper()
	root := canonicalPath(t.TempDir())
	rearmGit(t, root, "init", "-q", "-b", "trunk")
	rearmGit(t, root, "config", "--local", "maintenance.auto", "false")
	rearmGit(t, root, "config", "user.name", "test")
	rearmGit(t, root, "config", "user.email", "test@example.invalid")
	rearmGit(t, root, "config", "metasystem.steward.notify-command", "true")
	writeRearmFile(t, filepath.Join(root, "go.mod"), "module fixture.invalid/rearm\n")
	return root
}

func TestGitProjectionAdapterPreservesArchivedKindsModesAndSymlinks(t *testing.T) {
	outer := canonicalPath(t.TempDir())
	rearmGit(t, outer, "init", "-q", "-b", "trunk")
	rearmGit(t, outer, "config", "user.name", "test")
	rearmGit(t, outer, "config", "user.email", "test@example.invalid")
	installation := filepath.Join(outer, "metasystem")
	writeRearmFile(t, filepath.Join(installation, "go.mod"), "module fixture.invalid/nested\n")
	writeRearmFile(t, filepath.Join(installation, "cmd", "surface.txt"), "source")
	writeRearmFile(t, filepath.Join(installation, "cmd", "kind"), "target")
	if err := os.Symlink("one", filepath.Join(installation, "cmd", "link")); err != nil {
		t.Fatal(err)
	}
	rearmGit(t, outer, "add", ".")
	rearmGit(t, outer, "commit", "-qm", "source")
	source := rearmGit(t, outer, "rev-parse", "HEAD")
	policy, err := behaviorsurface.Load()
	if err != nil {
		t.Fatal(err)
	}
	deps := defaultRearmResolverDeps()
	originalDiff, originalArchive := deps.projectionDiff, deps.archivedEngineDigest
	diffCalls, archiveCalls := 0, 0
	deps.projectionDiff = func(ctx context.Context, root, from, to string, progress func()) ([]byte, error) {
		diffCalls++
		return originalDiff(ctx, root, from, to, progress)
	}
	deps.archivedEngineDigest = func(ctx context.Context, root, commit string, loaded behaviorsurface.Policy, clock RearmClock, seconds int) (string, error) {
		archiveCalls++
		return originalArchive(ctx, root, commit, loaded, clock, seconds)
	}
	cases := []struct {
		name             string
		change           func()
		equal, indexOnly bool
		changedPath      string
	}{
		{"ledger-only move", func() { writeRearmFile(t, filepath.Join(installation, "memory", "receipts.log"), "ledger\n") }, true, false, ""},
		{"engine content", func() { writeRearmFile(t, filepath.Join(installation, "cmd", "surface.txt"), "changed") }, false, false, "cmd/surface.txt"},
		{"executable mode", func() { _ = os.Chmod(filepath.Join(installation, "cmd", "surface.txt"), 0o755) }, true, false, ""},
		{"symlink target", func() {
			_ = os.Remove(filepath.Join(installation, "cmd", "link"))
			_ = os.Symlink("two", filepath.Join(installation, "cmd", "link"))
		}, false, false, "cmd/link"},
		{"file to symlink", func() {
			_ = os.Remove(filepath.Join(installation, "cmd", "kind"))
			_ = os.Symlink("target", filepath.Join(installation, "cmd", "kind"))
		}, false, false, "cmd/kind"},
		{"added engine file", func() { writeRearmFile(t, filepath.Join(installation, "cmd", "added"), "added") }, false, false, "cmd/added"},
		{"outside installation", func() { writeRearmFile(t, filepath.Join(outer, "cmd", "outside"), "outside") }, true, false, ""},
		{"gitlink fallback", func() {
			rearmGit(t, outer, "update-index", "--add", "--cacheinfo", "160000,"+source+",metasystem/cmd/submodule")
		}, true, true, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rearmGit(t, outer, "checkout", "-q", "--detach", source)
			tc.change()
			if !tc.indexOnly {
				rearmGit(t, outer, "add", "-A")
			}
			rearmGit(t, outer, "commit", "-qm", tc.name)
			destination := rearmGit(t, outer, "rev-parse", "HEAD")
			tempRoot := t.TempDir()
			t.Setenv("TMPDIR", tempRoot)
			sourceDigest, sourceErr := archivedEngineDigestAtCommit(context.Background(), installation, source, policy)
			destinationDigest, destinationErr := archivedEngineDigestAtCommit(context.Background(), installation, destination, policy)
			if sourceErr != nil || destinationErr != nil || (sourceDigest == destinationDigest) != tc.equal {
				t.Fatalf("archived digest agreement setup: source=%v destination=%v equal=%t want=%t", sourceErr, destinationErr, sourceDigest == destinationDigest, tc.equal)
			}
			before, _ := os.ReadDir(tempRoot)
			diffCalls, archiveCalls = 0, 0
			err := compareEngineProjectionWithDeps(deps, context.Background(), installation, source, destination, policy, SystemRearmClock(), RearmResolveSeconds(installation))
			if (err == nil) != tc.equal || diffCalls != 1 || archiveCalls != map[bool]int{true: 2}[tc.indexOnly] {
				t.Fatalf("comparison: err=%v diffs=%d archives=%d equal=%t", err, diffCalls, archiveCalls, tc.equal)
			}
			if tc.changedPath != "" && (!strings.Contains(err.Error(), tc.changedPath) || !errors.Is(err, ErrProjectionDiffers) || !errors.Is(err, ErrNotOwned) || errors.Is(err, ErrJudgmentStalled)) {
				t.Fatalf("changed path or error class missing: %v", err)
			}
			if after, readErr := os.ReadDir(tempRoot); readErr != nil || !reflect.DeepEqual(before, after) {
				t.Fatalf("comparison changed the temporary directory: before=%v after=%v err=%v", before, after, readErr)
			}
		})
	}
	truncated := fmt.Sprintf(":100644 100644 %040x %040x M\x00cmd/x", 1, 2)
	if _, err := parseProjectionDiff([]byte(truncated)); err == nil {
		t.Fatal("truncated raw diff was accepted as equality")
	}
}

func buildFakeRunner(t *testing.T, output, stamp string) {
	t.Helper()
	cmd := exec.Command("go", "build", "-buildvcs=false",
		"-ldflags", "-X github.com/widoriezebos/agentic-tools/metasystem/internal/supervise.BuildStamp="+stamp,
		"-o", output, "./testdata/fakerunner")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build fake runner: %v\n%s", err, out)
	}
}

type rearmBed struct {
	root, engine, replacement string
	first, second             string
}

func newRearmBed(t *testing.T, startRunner bool) rearmBed {
	t.Helper()
	t.Setenv("METASYSTEM_SUPERVISION_REGISTRY_HOME", t.TempDir())
	root := canonicalPath(t.TempDir())
	writeRearmFile(t, filepath.Join(root, "go.mod"), "module fixture.invalid/rearm\n")
	writeRearmFile(t, filepath.Join(root, "cmd", "surface.txt"), "second\n")
	reapStewardRunnerFixture(t, root)
	first, second := fmt.Sprintf("%040x", 1), fmt.Sprintf("%040x", 2)
	engine := filepath.Join(root, "metasystem")
	replacement := filepath.Join(root, "metasystem.next")
	buildFakeRunner(t, engine, first)
	buildFakeRunner(t, replacement, second)
	if startRunner {
		outcome, err := armWithRearmDeps(root, engine, false, false, false, "", humanMintDecision("human-terminal", "", "", EnrollmentHumanTerminal), rearmTestDeps(t))
		if message := outcome.Message; err != nil || !strings.Contains(message, "armed") {
			t.Fatalf("initial arm: %q %v", message, err)
		}
	} else {
		digest, err := installDigest(engine)
		if err != nil {
			t.Fatal(err)
		}
		if err := MintIdentity(RepoIdentityPath(root), InstallIdentity{
			RepoIdentity: root, Generation: 1, InstallPath: engine, InstallDigest: digest,
			MintedAt: time.Now().UTC().Format(time.RFC3339), MintedBy: "human-terminal",
			HumanWitnessedGeneration: 1, HumanWitnessedAt: time.Now().UTC().Format(time.RFC3339), EngineBuild: first,
		}); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Rename(replacement, engine); err != nil {
		t.Fatal(err)
	}
	return rearmBed{root: root, engine: engine, replacement: replacement, first: first, second: second}
}

func TestVerifySourceAtDestinationHumanEnrollmentAndMachineRearm(t *testing.T) {
	t.Setenv("METASYSTEM_SUPERVISION_REGISTRY_HOME", t.TempDir())
	root := canonicalPath(t.TempDir())
	source, destination, changed, unrelated := rearmID(1), rearmID(2), rearmID(3), rearmID(4)
	deps := rearmTestDeps(t,
		rearmBuild(root, source, source, 0), rearmAncestor(root, source, source, 0), rearmDiff(root, source, source, ""),
		rearmBuild(root, source, source, 0), rearmAncestor(root, source, destination, 0), rearmDiff(root, source, destination, ""),
		rearmBuild(root, source, source, 0), rearmAncestor(root, source, destination, 0),
		rearmBuild(root, source, source, 0), rearmAncestor(root, source, destination, 0), rearmDiff(root, source, destination, ""),
		rearmBuild(root, source, source, 0), rearmAncestor(root, source, destination, 0), rearmAncestor(root, source, destination, 0), rearmSkew(root, source, destination, ""), rearmDiff(root, source, destination, ""),
		rearmBuild(root, source, source, 0), rearmAncestor(root, source, changed, 0), rearmDiff(root, source, changed, strings.TrimPrefix(rearmRawChange("cmd/surface.txt", rearmID(11), rearmID(12)), "\n")),
		rearmBuild(root, source, source, 0), rearmAncestor(root, source, changed, 0), rearmAncestor(root, source, changed, 0), rearmSkew(root, source, changed, "cmd/surface.txt"),
		rearmBuild(root, source, source, 0), rearmAncestor(root, source, unrelated, 1),
	)
	originalDiff := deps.projectionDiff
	diffCalls, archiveCalls := 0, 0
	deps.projectionDiff = func(ctx context.Context, root, from, to string, progress func()) ([]byte, error) {
		diffCalls++
		return originalDiff(ctx, root, from, to, progress)
	}
	originalArchive := deps.archivedEngineDigest
	deps.archivedEngineDigest = func(ctx context.Context, root, commit string, policy behaviorsurface.Policy, clock RearmClock, seconds int) (string, error) {
		archiveCalls++
		return originalArchive(ctx, root, commit, policy, clock, seconds)
	}
	assertFast := func() {
		t.Helper()
		if diffCalls != 1 || archiveCalls != 0 {
			t.Fatalf("source comparison used %d diffs and %d archives", diffCalls, archiveCalls)
		}
		diffCalls, archiveCalls = 0, 0
	}
	engine := filepath.Join(canonicalPath(t.TempDir()), "metasystem")
	buildFakeRunner(t, engine, source)
	digest, err := installDigest(engine)
	if err != nil {
		t.Fatal(err)
	}
	human := InstallIdentity{RepoIdentity: root, Generation: 1, InstallPath: engine, InstallDigest: digest,
		MintedAt: time.Now().UTC().Format(time.RFC3339), MintedBy: "human-terminal",
		Enrollment: EnrollmentHumanTerminal, EngineBuild: source}
	if err := MintIdentity(RepoIdentityPath(root), human); err != nil {
		t.Fatal(err)
	}
	pinned, err := OpenEnrolledBinary(root)
	if err != nil {
		t.Fatal(err)
	}
	defer pinned.Close()
	for _, at := range []string{source, destination} {
		if err := pinned.verifySourceAtDestinationWithDeps(deps, SystemRearmClock(), root, at); err != nil {
			t.Fatalf("lawful human enrollment at %s: %v", at, err)
		}
		assertFast()
	}
	if pinned.Install.LandedCommit != "" || pinned.Install.LandingRef != "" {
		t.Fatal("source verification manufactured machine enrollment provenance")
	}
	pinned.Install.MintedBy = "machine-rebuild"
	if err := pinned.verifySourceAtDestinationWithDeps(deps, SystemRearmClock(), root, destination); err == nil || !strings.Contains(err.Error(), "missing its landed source") {
		t.Fatalf("incomplete machine provenance was accepted: %v", err)
	}
	pinned.Install.LandedCommit, pinned.Install.LandingRef = source, "refs/remotes/origin/trunk"
	if err := pinned.verifySourceAtDestinationWithDeps(deps, SystemRearmClock(), root, destination); err != nil {
		t.Fatalf("machine rearm lost record-only reuse: %v", err)
	}
	assertFast()
	pinned.Install.LandedCommit = destination
	if err := pinned.verifySourceAtDestinationWithDeps(deps, SystemRearmClock(), root, destination); err != nil {
		t.Fatalf("machine rearm lost a newer record-only landed source: %v", err)
	}
	assertFast()
	pinned.Install = human
	if err := pinned.verifySourceAtDestinationWithDeps(deps, SystemRearmClock(), root, changed); err == nil || !strings.Contains(err.Error(), "different ENGINE projections") || !errors.Is(err, ErrProjectionDiffers) || !errors.Is(err, ErrNotOwned) {
		t.Fatalf("human enrollment accepted changed engine source: %v", err)
	}
	assertFast()
	pinned.Install.MintedBy = "machine-rebuild"
	pinned.Install.LandedCommit, pinned.Install.LandingRef = changed, "refs/remotes/origin/trunk"
	if err := pinned.verifySourceAtDestinationWithDeps(deps, SystemRearmClock(), root, changed); err == nil || !strings.Contains(err.Error(), "engine or agent scripts changed") || !errors.Is(err, ErrNotOwned) {
		t.Fatalf("machine enrollment accepted changed engine source: %v", err)
	}
	pinned.Install = human
	if err := pinned.verifySourceAtDestinationWithDeps(deps, SystemRearmClock(), root, unrelated); err == nil || !strings.Contains(err.Error(), "not landed") || !errors.Is(err, ErrNotOwned) {
		t.Fatalf("human enrollment accepted unrelated destination: %v", err)
	}
}

func TestEnrollmentLandedSourceUsesEngineScriptSkewRule(t *testing.T) {
	for _, tc := range []struct {
		name    string
		status  int
		changed string
		want    string
	}{
		{"ledger-only movement binds", 0, "", ""},
		{"ledger-only second-parent movement binds", 0, "", ""},
		{"engine movement drifts", 0, "internal/skew.go", "engine or agent scripts changed"},
		{"non-ancestor stamp drifts", 1, "", "not its ancestor"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := canonicalPath(t.TempDir())
			source, landed := rearmID(1), rearmID(2)
			expected := []testgit.Expectation{rearmAncestor(root, source, landed, tc.status)}
			if tc.status == 0 {
				expected = append(expected, rearmSkew(root, source, landed, tc.changed))
			}
			err := verifyEnrollmentLandedSourceWithDeps(rearmTestDeps(t, expected...), SystemRearmClock(), root, source, landed)
			if tc.want == "" && err != nil || tc.want != "" && (err == nil || !strings.Contains(err.Error(), tc.want)) {
				t.Fatalf("enrollment skew rule: %v", err)
			}
		})
	}
}

func TestLandedBuildResolutionSeparatesAMissingObjectFromARepositoryFailure(t *testing.T) {
	root := canonicalPath(t.TempDir())
	landed, missing := rearmID(1), strings.Repeat("f", 40)
	ref := "refs/remotes/origin/trunk"
	deps := rearmTestDeps(t, rearmBuild(root, landed, landed, 0), rearmAncestor(root, landed, ref, 0), rearmBuild(root, missing, "", 1), rearmFailed(root, 128, "rev-parse", "--verify", "--quiet", landed+"^{commit}"))
	if resolved, err := resolveLandedBuildWithDeps(deps, SystemRearmClock(), root, root, ref, landed); err != nil || resolved != landed {
		t.Fatalf("known landed build did not resolve: resolved=%q err=%v", resolved, err)
	}
	_, missingErr := resolveLandedBuildWithDeps(deps, SystemRearmClock(), root, root, ref, missing)
	wantMissing := fmt.Sprintf("rebuilt engine carries unresolved build stamp %q; automatic re-arm is bounded to landed commits", missing)
	if missingErr == nil || missingErr.Error() != wantMissing || !errors.Is(missingErr, ErrNotOwned) || errors.Is(missingErr, ErrJudgmentStalled) {
		t.Fatalf("missing object changed its judgment: %v", missingErr)
	}
	_, failureErr := resolveLandedBuildWithDeps(deps, SystemRearmClock(), root, root, ref, landed)
	if failureErr == nil || !strings.Contains(failureErr.Error(), "git rev-parse --verify --quiet") ||
		!strings.Contains(failureErr.Error(), "fatal: not a git repository (or any of the parent directories)") ||
		errors.Is(failureErr, ErrNotOwned) || errors.Is(failureErr, ErrJudgmentStalled) {
		t.Fatalf("repository failure became an ownership judgment: %v", failureErr)
	}
}

func TestLandedBuildResolutionSeparatesNotAnAncestorFromARepositoryFailure(t *testing.T) {
	root := canonicalPath(t.TempDir())
	side := rearmID(2)
	ref := "refs/remotes/origin/trunk"
	deps := rearmTestDeps(t, rearmBuild(root, side, side, 0), rearmAncestor(root, side, ref, 1),
		rearmBuild(root, side, side, 0), rearmFailed(root, 128, "merge-base", "--is-ancestor", side, ref))
	_, notAncestorErr := resolveLandedBuildWithDeps(deps, SystemRearmClock(), root, root, ref, side)
	wantNotAncestor := fmt.Sprintf("rebuilt engine was built from %s, which is not landed on %s", side, ref)
	if notAncestorErr == nil || notAncestorErr.Error() != wantNotAncestor || !errors.Is(notAncestorErr, ErrNotOwned) || errors.Is(notAncestorErr, ErrJudgmentStalled) {
		t.Fatalf("non-ancestor changed its judgment: %v", notAncestorErr)
	}
	_, failureErr := resolveLandedBuildWithDeps(deps, SystemRearmClock(), root, root, ref, side)
	if failureErr == nil || !strings.Contains(failureErr.Error(), "git merge-base --is-ancestor") ||
		!strings.Contains(failureErr.Error(), "fatal: not a git repository (or any of the parent directories)") ||
		errors.Is(failureErr, ErrNotOwned) || errors.Is(failureErr, ErrJudgmentStalled) {
		t.Fatalf("repository failure became an ownership judgment: %v", failureErr)
	}
}

func TestEnrollmentBuildSourceCheckSeparatesNotAnAncestorFromARepositoryFailure(t *testing.T) {
	root := canonicalPath(t.TempDir())
	base, landed, side := rearmID(1), rearmID(2), rearmID(3)
	deps := rearmTestDeps(t,
		rearmAncestor(root, base, landed, 0), rearmSkew(root, base, landed, ""),
		rearmAncestor(root, side, landed, 1),
		rearmFailed(root, 128, "merge-base", "--is-ancestor", side, landed),
	)
	if err := verifyEnrollmentBuildSourceWithDeps(deps, SystemRearmClock(), root, base, base, landed); err != nil {
		t.Fatalf("known ancestor did not pass the enrollment check: %v", err)
	}
	notAncestorErr := verifyEnrollmentBuildSourceWithDeps(deps, SystemRearmClock(), root, side, side, landed)
	wantNotAncestor := fmt.Sprintf("enrollment records landed source %q but executable stamp source %q is not its ancestor", landed, side)
	if notAncestorErr == nil || notAncestorErr.Error() != wantNotAncestor || !errors.Is(notAncestorErr, ErrNotOwned) || errors.Is(notAncestorErr, ErrJudgmentStalled) {
		t.Fatalf("non-ancestor changed its enrollment judgment: %v", notAncestorErr)
	}
	t.Run("witness operational failure does not run the reverse comparison", func(t *testing.T) {
		root := canonicalPath(t.TempDir())
		landed, source := rearmID(2), rearmID(3)
		deps := rearmTestDeps(t, rearmFailed(root, 128, "merge-base", "--is-ancestor", source, landed))
		calls := 0
		original := deps.deadlineGit
		deps.deadlineGit = func(clock RearmClock, seconds int, step, root string, args ...string) (string, error) {
			calls++
			return original(clock, seconds, step, root, args...)
		}
		err := verifyEnrollmentBuildSourceWithDeps(deps, SystemRearmClock(), root, "witness-aaaaaaaaaaaa", source, landed)
		if err == nil || !strings.Contains(err.Error(), "git merge-base --is-ancestor") || !strings.Contains(err.Error(), "fatal: not a git repository (or any of the parent directories)") || errors.Is(err, ErrNotOwned) || errors.Is(err, ErrJudgmentStalled) {
			t.Fatalf("witness comparison replaced its repository failure with a reverse result: %v", err)
		}
		if calls != 1 {
			t.Fatalf("witness repository failure ran %d comparisons, want exactly one", calls)
		}
	})
	t.Run("witness stall does not run the reverse comparison", func(t *testing.T) {
		root := canonicalPath(t.TempDir())
		landed, source := rearmID(2), rearmID(3)
		stub := testgit.New(t, rearmExpected(root, "", ErrJudgmentStalled, "merge-base", "--is-ancestor", source, landed))
		clock := newManualRearmClock()
		calls := 0
		deps := rearmTestDeps(t)
		deps.deadlineGit = func(_ RearmClock, seconds int, step, gotRoot string, args ...string) (string, error) {
			calls++
			declared := stub.Run(testgit.Call{Dir: gotRoot, Args: args})
			if !errors.Is(declared.Err, ErrJudgmentStalled) {
				return "", declared.Err
			}
			return "", RunRearmStep(context.Background(), clock, time.Duration(seconds)*time.Second, step, func(ctx context.Context, progress func()) error { <-ctx.Done(); return ctx.Err() })
		}
		result := make(chan error, 1)
		go func() {
			result <- verifyEnrollmentBuildSourceWithDeps(deps, clock, root, "witness-aaaaaaaaaaaa", source, landed)
		}()
		timer := waitForRearmTimer(clock, 1)
		clock.advance(21 * time.Second)
		timer <- clock.Now()
		err := <-result
		if err == nil || err.Error() != "compare-enrollment-ancestry exceeded the configured 20-second bound (metasystem.steward.rearm-resolve-seconds)" || !errors.Is(err, ErrJudgmentStalled) || errors.Is(err, ErrNotOwned) {
			t.Fatalf("witness comparison replaced its stall with a reverse result: %v", err)
		}
		if calls != 1 {
			t.Fatalf("witness stall ran %d comparisons, want exactly one", calls)
		}
	})
	failureErr := verifyEnrollmentBuildSourceWithDeps(deps, SystemRearmClock(), root, side, side, landed)
	if failureErr == nil || !strings.Contains(failureErr.Error(), "git merge-base --is-ancestor") || !strings.Contains(failureErr.Error(), "fatal: not a git repository (or any of the parent directories)") || errors.Is(failureErr, ErrNotOwned) || errors.Is(failureErr, ErrJudgmentStalled) {
		t.Fatalf("repository failure became an enrollment judgment: %v", failureErr)
	}
}

func TestEnrollmentSkewPathspecsMatchDispatch(t *testing.T) {
	script, err := os.ReadFile(filepath.Join("..", "..", "scripts", "agents", "dispatch.sh"))
	if err != nil {
		t.Fatal(err)
	}
	matches := regexp.MustCompile(`"\$protected_prefix"([^|)]+)/\*`).FindAllStringSubmatch(string(script), -1)
	got := make([]string, 0, len(matches))
	for _, match := range matches {
		got = append(got, match[1])
	}
	want := enrollmentSkewPathspecs[:]
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("steward enrollment skew paths %v differ from dispatch paths %v", want, got)
	}
}

func TestWitnessResolverWalksPastTheSixtyFourthCandidate(t *testing.T) {
	root := t.TempDir()
	ref := "refs/remotes/origin/trunk"
	targetDigest := strings.Repeat("a", 64)
	otherDigest := strings.Repeat("b", 64)
	commits, trees, blobs := make([]string, 65), make([]string, 65), make([]string, 65)
	for candidate := range commits {
		commits[candidate] = fmt.Sprintf("%040x", candidate+1)
		trees[candidate] = fmt.Sprintf("%040x", candidate+101)
		blobs[candidate] = fmt.Sprintf("%040x", candidate+201)
	}
	var history strings.Builder
	for candidate := len(commits) - 1; candidate >= 0; candidate-- {
		fmt.Fprintf(&history, "\x01%s %s", commits[candidate], trees[candidate])
		if candidate > 0 {
			fmt.Fprintf(&history, " %s\x00\n:100644 100644 %s %s M\x00cmd/surface.txt\x00", commits[candidate-1], blobs[candidate-1], blobs[candidate])
		} else {
			fmt.Fprintf(&history, "\x00\n:000000 100644 %s %s A\x00cmd/surface.txt\x00", strings.Repeat("0", 40), blobs[candidate])
		}
	}
	logCalls, nulLogCalls, batchCalls := 0, 0, 0
	var archives []string
	deps := rearmTestDeps(t,
		rearmExpected(root, root+"\n", nil, "rev-parse", "--show-toplevel"),
		rearmExpected(root, history.String(), nil, "log", "--topo-order", "--no-abbrev", "--raw", "-z", "--no-renames", "--no-ext-diff", "--ignore-submodules=none", "--diff-merges=first-parent", "--format=%x01%H %T %P", ref),
	)
	originalRunner := deps.witnessGit
	deps.witnessGit = func(ctx context.Context, gotRoot string, input []byte, progress func(), args ...string) ([]byte, error) {
		if args[0] == "log" {
			logCalls++
			nulLogCalls++
		}
		if args[0] == "cat-file" {
			batchCalls++
		}
		return originalRunner(ctx, gotRoot, input, progress, args...)
	}
	deps.witnessTreeDigest = func(_ context.Context, toplevel, tree string, _ behaviorsurface.Policy, _ RearmClock, _ int) (string, error) {
		if toplevel != root {
			return "", fmt.Errorf("unexpected archive root: %q", toplevel)
		}
		archives = append(archives, tree)
		if tree == commits[0] {
			return targetDigest, nil
		}
		return otherDigest, nil
	}
	resolved, err := resolveWitnessStampWithDeps(deps, SystemRearmClock(), root, root, ref, targetDigest[:12])
	if err != nil || resolved != commits[0] {
		t.Fatalf("the sixty-fifth reachable candidate did not resolve: got=%s want=%s err=%v", resolved, commits[0], err)
	}
	if logCalls != 1 || nulLogCalls != 1 || batchCalls != 0 || len(archives) != 65 {
		t.Fatalf("65-candidate walk used log=%d NUL-log=%d batch=%d archives=%d, want 1, 1, 0, 65", logCalls, nulLogCalls, batchCalls, len(archives))
	}
	for index, archived := range archives {
		if want := commits[len(commits)-1-index]; archived != want {
			t.Fatalf("archive %d used %s, want %s", index, archived, want)
		}
	}
}

func TestGitWitnessAdapterPreservesNestedArchiveHistory(t *testing.T) {
	outer := canonicalPath(t.TempDir())
	rearmGit(t, outer, "init", "-q", "-b", "trunk")
	rearmGit(t, outer, "config", "user.name", "test")
	rearmGit(t, outer, "config", "user.email", "test@example.invalid")
	root := filepath.Join(outer, "metasystem")
	writeRearmFile(t, filepath.Join(root, "go.mod"), "module fixture.invalid/nested\n")
	writeRearmFile(t, filepath.Join(root, "cmd", "surface.txt"), "nested\n")
	rearmGit(t, outer, "add", ".")
	rearmGit(t, outer, "commit", "-q", "-m", "nested")
	commit := rearmGit(t, outer, "rev-parse", "HEAD")
	ref := "refs/remotes/origin/trunk"
	rearmGit(t, outer, "update-ref", ref, commit)
	policy, _ := behaviorsurface.Load()
	digest, err := policy.Digest(root, behaviorsurface.Engine)
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := resolveWitnessStamp(outer, root, ref, digest[:12])
	if err != nil || resolved != commit {
		t.Fatalf("nested witness resolution: got=%s want=%s err=%v", resolved, commit, err)
	}
	oldPath := "metasystem/cmd/space name.txt"
	newPath := "metasystem/cmd/renamed name.txt"
	newlinePath := "metasystem/cmd/line\nbreak.txt"
	writeRearmFile(t, filepath.Join(outer, filepath.FromSlash(oldPath)), "space\n")
	writeRearmFile(t, filepath.Join(outer, filepath.FromSlash(newlinePath)), "newline\n")
	rearmGit(t, outer, "add", ".")
	rearmGit(t, outer, "commit", "-qm", "paths")
	rearmGit(t, outer, "mv", oldPath, newPath)
	rearmGit(t, outer, "commit", "-qm", "rename")
	candidates, err := readWitnessCandidates(SystemRearmClock(), defaultRearmResolveSeconds, outer, "HEAD")
	if err != nil || len(candidates) != 3 {
		t.Fatalf("actual Git NUL history: candidates=%d err=%v", len(candidates), err)
	}
	paths := map[string]projectionDiffEntry{}
	for _, entry := range candidates[0].changes {
		paths[entry.path] = entry
	}
	if deleted, ok := paths[oldPath]; !ok || deleted.newMode != "000000" {
		t.Fatalf("actual Git rename source path: %+v", deleted)
	}
	if added, ok := paths[newPath]; !ok || added.oldMode != "000000" {
		t.Fatalf("actual Git rename destination path: %+v", added)
	}
	foundNewline := false
	for _, entry := range candidates[1].changes {
		foundNewline = foundNewline || entry.path == newlinePath
	}
	if !foundNewline {
		t.Fatal("actual Git NUL history lost the newline path")
	}
	remoteTip := rearmGit(t, outer, "rev-parse", "HEAD")
	rearmGit(t, outer, "update-ref", ref, remoteTip)
	commitRearmTree(t, outer, "ordinary branch movement")
	if got := rearmGit(t, outer, "rev-parse", ref); got != remoteTip {
		t.Fatalf("ordinary branch movement advanced the remote-tracking ref: got=%s want=%s", got, remoteTip)
	}
}

func TestWitnessResolverChoosesNewestMatchingCommit(t *testing.T) {
	root := canonicalPath(t.TempDir())
	writeRearmFile(t, filepath.Join(root, "go.mod"), "module fixture.invalid/rearm\n")
	writeRearmFile(t, filepath.Join(root, "cmd", "surface.txt"), "shared-engine\n")
	policy, err := behaviorsurface.Load()
	if err != nil {
		t.Fatal(err)
	}
	digest, err := policy.Digest(root, behaviorsurface.Engine)
	if err != nil {
		t.Fatal(err)
	}
	first, newest, tree := rearmID(1), rearmID(2), rearmID(101)
	ref := "refs/remotes/origin/trunk"
	history := rearmHistory(newest, tree, first, rearmRawChange("docs/first.txt", strings.Repeat("0", 40), rearmID(201))) + rearmHistory(first, tree, "", rearmRawChange("cmd/surface.txt", strings.Repeat("0", 40), rearmID(202)))
	deps := rearmTestDeps(t, rearmWitnessExpectations(root, ref, history)...)
	deps.witnessTreeDigest = func(_ context.Context, gotRoot, spec string, _ behaviorsurface.Policy, _ RearmClock, _ int) (string, error) {
		if gotRoot != root || spec != newest {
			t.Fatalf("archive request: root=%q spec=%q", gotRoot, spec)
		}
		return digest, nil
	}
	resolved, err := resolveWitnessStampWithDeps(deps, SystemRearmClock(), root, root, ref, digest[:12])
	if err != nil || resolved != newest {
		t.Fatalf("newest matching commit was not selected: got=%s want=%s err=%v", resolved, newest, err)
	}
}

func TestWitnessResolverRefusesUnlandedTreeDigest(t *testing.T) {
	root := canonicalPath(t.TempDir())
	writeRearmFile(t, filepath.Join(root, "go.mod"), "module fixture.invalid/rearm\n")
	writeRearmFile(t, filepath.Join(root, "cmd", "surface.txt"), "trunk\n")
	policy, err := behaviorsurface.Load()
	if err != nil {
		t.Fatal(err)
	}
	trunkDigest, err := policy.Digest(root, behaviorsurface.Engine)
	if err != nil {
		t.Fatal(err)
	}
	writeRearmFile(t, filepath.Join(root, "cmd", "surface.txt"), "unlanded-side\n")
	digest, err := policy.Digest(root, behaviorsurface.Engine)
	if err != nil {
		t.Fatal(err)
	}
	trunk, tree := rearmID(1), rearmID(101)
	ref := "refs/remotes/origin/trunk"
	history := rearmHistory(trunk, tree, "", rearmRawChange("cmd/surface.txt", strings.Repeat("0", 40), rearmID(201)))
	expected := append([]testgit.Expectation{}, rearmWitnessExpectations(root, ref, history)...)
	expected = append(expected, rearmWitnessExpectations(root, ref, history)...)
	expected = append(expected, rearmWitnessExpectations(root, ref, history)...)
	deps := rearmTestDeps(t, expected...)
	deps.witnessTreeDigest = func(_ context.Context, gotRoot, spec string, _ behaviorsurface.Policy, _ RearmClock, _ int) (string, error) {
		if gotRoot != root || spec != trunk {
			t.Fatalf("archive request: root=%q spec=%q", gotRoot, spec)
		}
		return trunkDigest, nil
	}
	resolved, err := resolveWitnessStampWithDeps(deps, SystemRearmClock(), root, root, ref, digest[:12])
	if err == nil || resolved != "" || !errors.Is(err, ErrNotOwned) {
		t.Fatalf("unlanded witness digest resolved to %q: %v", resolved, err)
	}
	if _, err := resolveLandedBuildWithDeps(deps, SystemRearmClock(), root, root, ref, "witness-"+digest[:12]); err == nil || !errors.Is(err, ErrNotOwned) {
		t.Fatalf("no-match witness verdict lost its class: %v", err)
	}
	_ = os.Remove(filepath.Join(root, "artifacts", "agents", "steward", witnessDigestCacheName))
	deps.witnessTreeDigest = func(context.Context, string, string, behaviorsurface.Policy, RearmClock, int) (string, error) {
		return "", errors.New("injected digest failure")
	}
	if _, err := resolveLandedBuildWithDeps(deps, SystemRearmClock(), root, root, ref, "witness-"+digest[:12]); err == nil || !strings.Contains(err.Error(), "injected digest failure") || errors.Is(err, ErrNotOwned) || errors.Is(err, ErrJudgmentStalled) {
		t.Fatalf("operational witness failure was misclassified: %v", err)
	}
}

func TestWitnessResolverPersistsTreeDigestsAcrossResolutions(t *testing.T) {
	root := canonicalPath(t.TempDir())
	writeRearmFile(t, filepath.Join(root, "go.mod"), "module fixture.invalid/rearm\n")
	writeRearmFile(t, filepath.Join(root, "cmd", "surface.txt"), "cached\n")
	policy, err := behaviorsurface.Load()
	if err != nil {
		t.Fatal(err)
	}
	digest, err := policy.Digest(root, behaviorsurface.Engine)
	if err != nil {
		t.Fatal(err)
	}
	commit, tree := rearmID(1), rearmID(101)
	ref := "refs/remotes/origin/trunk"
	history := rearmHistory(commit, tree, "", rearmRawChange("cmd/surface.txt", strings.Repeat("0", 40), rearmID(201)))
	expected := append([]testgit.Expectation{}, rearmWitnessExpectations(root, ref, history)...)
	expected = append(expected, rearmWitnessExpectations(root, ref, history)...)
	deps := rearmTestDeps(t, expected...)
	cachePath := filepath.Join(root, "artifacts", "agents", "steward", witnessDigestCacheName)
	writeRearmFile(t, cachePath, "not json\n")
	digestCalls, cacheWrites := 0, 0
	deps.witnessTreeDigest = func(_ context.Context, gotRoot, spec string, _ behaviorsurface.Policy, _ RearmClock, _ int) (string, error) {
		digestCalls++
		if gotRoot != root || spec != commit {
			t.Fatalf("archive request: root=%q spec=%q", gotRoot, spec)
		}
		return digest, nil
	}
	deps.writeWitnessCache = func(path, anchor string, entries map[string]string) {
		cacheWrites++
		writeWitnessDigestCache(path, anchor, entries)
	}
	resolved, err := resolveWitnessStampWithDeps(deps, SystemRearmClock(), root, root, ref, digest[:12])
	if err != nil || resolved != commit || digestCalls != 1 || cacheWrites != 1 {
		t.Fatalf("first resolution did not rebuild the unreadable cache once: got=%s digest-calls=%d cache-writes=%d err=%v", resolved, digestCalls, cacheWrites, err)
	}
	cacheBytes, err := os.ReadFile(cachePath)
	if err != nil || !json.Valid(cacheBytes) {
		t.Fatalf("persistent witness cache was not rewritten as JSON: %v %q", err, cacheBytes)
	}
	digestCalls, cacheWrites = 0, 0
	resolved, err = resolveWitnessStampWithDeps(deps, SystemRearmClock(), root, root, ref, digest[:12])
	if err != nil || resolved != commit || digestCalls != 0 || cacheWrites != 1 {
		t.Fatalf("second resolution repeated a digest or wrote more than once: got=%s digest-calls=%d cache-writes=%d err=%v", resolved, digestCalls, cacheWrites, err)
	}
}

func TestWitnessResolverWritesCacheOnceOnExpiry(t *testing.T) {
	root := canonicalPath(t.TempDir())
	older, newest := rearmID(1), rearmID(2)
	ref := "refs/remotes/origin/trunk"
	history := rearmHistory(newest, rearmID(102), older, rearmRawChange("cmd/surface.txt", rearmID(201), rearmID(202))) + rearmHistory(older, rearmID(101), "", rearmRawChange("cmd/surface.txt", strings.Repeat("0", 40), rearmID(201)))
	deps := rearmTestDeps(t, rearmWitnessExpectations(root, ref, history)...)
	deps.resolveSeconds = func(gotRoot string) int {
		if gotRoot != root {
			t.Fatalf("resolve cadence root %q", gotRoot)
		}
		return 1
	}
	digestCalls, cacheWrites := 0, 0
	stalled := make(chan chan time.Time, 1)
	deps.witnessTreeDigest = func(_ context.Context, gotRoot, spec string, _ behaviorsurface.Policy, _ RearmClock, seconds int) (string, error) {
		digestCalls++
		if gotRoot != root || (digestCalls == 1 && spec != newest) || (digestCalls == 2 && spec != older) || seconds != 1 {
			t.Fatalf("archive request root=%q spec=%q seconds=%d", gotRoot, spec, seconds)
		}
		if digestCalls == 1 {
			return strings.Repeat("f", 64), nil
		}
		timer := make(chan time.Time, 1)
		stalled <- timer
		<-timer
		return "", classifiedJudgment(fmt.Sprintf("digest exceeded the configured %d-second bound (%s)", seconds, rearmResolveSecondsConfig), ErrJudgmentStalled)
	}
	deps.writeWitnessCache = func(path, anchor string, entries map[string]string) {
		cacheWrites++
		writeWitnessDigestCache(path, anchor, entries)
	}
	clock := newManualRearmClock()
	type resolution struct {
		commit string
		err    error
	}
	result := make(chan resolution, 1)
	go func() {
		commit, err := resolveWitnessStampWithDeps(deps, clock, root, root, ref, "000000000000")
		result <- resolution{commit, err}
	}()
	timer := <-stalled
	clock.advance(2 * time.Second)
	timer <- clock.Now()
	resolved := <-result
	err := resolved.err
	if err == nil || resolved.commit != "" || !strings.Contains(err.Error(), "exceeded the configured 1-second bound") || !errors.Is(err, ErrJudgmentStalled) || errors.Is(err, ErrNotOwned) {
		t.Fatalf("resolver expiry was not loud: resolved=%q err=%v", resolved.commit, err)
	}
	if digestCalls < 2 || cacheWrites != 1 {
		t.Fatalf("expiry used %d digest calls and %d cache writes, want at least two calls and exactly one write", digestCalls, cacheWrites)
	}
}

func TestLandingRefReadIgnoresGlobalConfigurationAndOrdinaryBranchMovement(t *testing.T) {
	root := canonicalPath(t.TempDir())
	first := rearmID(1)
	ref := "refs/remotes/origin/trunk"
	global := filepath.Join(t.TempDir(), "gitconfig")
	writeRearmFile(t, global, "[metasystem \"steward\"]\n\tlanding-ref = "+ref+"\n")
	t.Setenv("GIT_CONFIG_GLOBAL", global)
	deps := rearmTestDeps(t,
		rearmFailed(root, 1, "config", "--local", "--no-includes", "--get", landingRefConfigKey),
		rearmFailed(root, 1, "config", "--local", "--no-includes", "--get", landingRefConfigKey),
		rearmExpected(root, "\n", nil, "config", "--local", "--no-includes", "--get", landingRefConfigKey),
		rearmExpected(root, "refs/heads/trunk\n", nil, "config", "--local", "--no-includes", "--get", landingRefConfigKey),
		rearmExpected(root, ref+"\n", nil, "config", "--local", "--no-includes", "--get", landingRefConfigKey),
		rearmExpected(root, first+"\n", nil, "rev-parse", "--verify", "--quiet", ref+"^{commit}"),
		rearmExpected(root, ref+"\n", nil, "config", "--local", "--no-includes", "--get", landingRefConfigKey),
		rearmExpected(root, first+"\n", nil, "rev-parse", "--verify", "--quiet", ref+"^{commit}"),
	)
	wantUnset := fmt.Sprintf("the installation owns no remote-tracking landing ref (%s is <unset>; expected refs/remotes/<remote>/<branch>)", landingRefConfigKey)
	if _, err := readOwnedLandingRefWithDeps(deps, root); err == nil || err.Error() != wantUnset {
		t.Fatalf("a global landing-ref value changed the absent local value result: %v", err)
	}
	included := filepath.Join(t.TempDir(), "included-config")
	writeRearmFile(t, included, "[metasystem \"steward\"]\n\tlanding-ref = "+ref+"\n")
	if _, err := readOwnedLandingRefWithDeps(deps, root); err == nil {
		t.Fatal("an included landing-ref value satisfied the installation")
	}
	if _, err := readOwnedLandingRefWithDeps(deps, root); err == nil || err.Error() != wantUnset {
		t.Fatalf("an empty local landing-ref value changed its result: %v", err)
	}
	localRef := "refs/heads/trunk"
	wantMalformed := fmt.Sprintf("the installation owns no remote-tracking landing ref (%s is %s; expected refs/remotes/<remote>/<branch>)", landingRefConfigKey, localRef)
	if _, err := readOwnedLandingRefWithDeps(deps, root); err == nil || err.Error() != wantMalformed {
		t.Fatalf("a local branch ref was not refused with the expected remote-tracking shape: %v", err)
	}
	if got, err := readOwnedLandingRefWithDeps(deps, root); err != nil || got != ref {
		t.Fatalf("owned remote-tracking ref: %q %v", got, err)
	}
	// A local branch moving does not change the owned remote-tracking answer.
	if got, err := readOwnedLandingRefWithDeps(deps, root); err != nil || got != ref {
		t.Fatalf("ordinary branch movement advanced the remote-tracking ref: got=%s want=%s err=%v", got, ref, err)
	}
}

func TestOwnedLandingRefReadSeparatesAnUnresolvedRefFromARepositoryFailure(t *testing.T) {
	root := canonicalPath(t.TempDir())
	landed := rearmID(1)
	ref := "refs/remotes/origin/trunk"
	config := func() testgit.Expectation {
		return rearmExpected(root, ref+"\n", nil, "config", "--local", "--no-includes", "--get", landingRefConfigKey)
	}
	deps := rearmTestDeps(t, config(), rearmFailed(root, 1, "rev-parse", "--verify", "--quiet", ref+"^{commit}"),
		config(), rearmExpected(root, landed+"\n", nil, "rev-parse", "--verify", "--quiet", ref+"^{commit}"),
		config(), rearmFailed(root, 128, "rev-parse", "--verify", "--quiet", ref+"^{commit}"))
	_, unresolvedErr := readOwnedLandingRefWithDeps(deps, root)
	wantUnresolved := fmt.Sprintf("the installation owns no resolving remote-tracking landing ref (%s is %s)", landingRefConfigKey, ref)
	if unresolvedErr == nil || unresolvedErr.Error() != wantUnresolved || errors.Is(unresolvedErr, ErrNotOwned) || errors.Is(unresolvedErr, ErrJudgmentStalled) {
		t.Fatalf("unresolved ref changed its result: %v", unresolvedErr)
	}
	if resolved, err := readOwnedLandingRefWithDeps(deps, root); err != nil || resolved != ref {
		t.Fatalf("resolving owned ref was not returned: resolved=%q err=%v", resolved, err)
	}
	_, failureErr := readOwnedLandingRefWithDeps(deps, root)
	if failureErr == nil || !strings.Contains(failureErr.Error(), "git rev-parse --verify --quiet") || !strings.Contains(failureErr.Error(), "fatal: not a git repository (or any of the parent directories)") || errors.Is(failureErr, ErrNotOwned) || errors.Is(failureErr, ErrJudgmentStalled) {
		t.Fatalf("repository failure became an unresolved-ref result: %v", failureErr)
	}
}

func TestMintPlanReportsARepositoryFailureAsAFailureNotAsDrift(t *testing.T) {
	ref := "refs/remotes/origin/trunk"
	t.Run("unresolved landing ref remains drift", func(t *testing.T) {
		bed := newRearmBed(t, false)
		deps := rearmTestDeps(t, rearmExpected(bed.root, ref+"\n", nil, "config", "--local", "--no-includes", "--get", landingRefConfigKey), rearmFailed(bed.root, 1, "rev-parse", "--verify", "--quiet", ref+"^{commit}"))
		_, err := reArmRebuiltEngineWithDeps(deps, bed.root, bed.root, bed.engine)
		want := fmt.Sprintf("%s: the installation owns no resolving remote-tracking landing ref (%s is %s)", ErrEnrollmentDrift, landingRefConfigKey, ref)
		if err == nil || err.Error() != want || !errors.Is(err, ErrEnrollmentDrift) {
			t.Fatalf("unresolved ref changed its mint-plan drift: %v", err)
		}
	})
	t.Run("repository failure is a failed step", func(t *testing.T) {
		bed := newRearmBed(t, false)
		expected := append(rearmOwnedRef(bed.root, ref, bed.second), rearmBuild(bed.root, bed.second, bed.second, 0), rearmFailed(bed.root, 128, "merge-base", "--is-ancestor", bed.second, ref))
		_, err := reArmRebuiltEngineWithDeps(rearmTestDeps(t, expected...), bed.root, bed.root, bed.engine)
		if err == nil || !strings.Contains(err.Error(), "resolve landed build") || !strings.Contains(err.Error(), "git merge-base --is-ancestor") || !strings.Contains(err.Error(), "fatal: not a git repository (or any of the parent directories)") || errors.Is(err, ErrEnrollmentDrift) || errors.Is(err, ErrNotOwned) || errors.Is(err, ErrJudgmentStalled) {
			t.Fatalf("repository failure became mint-plan drift: %v", err)
		}
	})
	t.Run("landing ref config failure is a failed step", func(t *testing.T) {
		bed := newRearmBed(t, false)
		deps := rearmTestDeps(t, rearmFailed(bed.root, 128, "config", "--local", "--no-includes", "--get", landingRefConfigKey))
		_, err := reArmRebuiltEngineWithDeps(deps, bed.root, bed.root, bed.engine)
		if err == nil || !strings.Contains(err.Error(), "read owned landing ref") || !strings.Contains(err.Error(), "git config") || !strings.Contains(err.Error(), "fatal: not a git repository (or any of the parent directories)") || errors.Is(err, ErrEnrollmentDrift) || errors.Is(err, ErrNotOwned) || errors.Is(err, ErrJudgmentStalled) {
			t.Fatalf("landing-ref config failure became mint-plan drift: %v", err)
		}
	})
	for _, tc := range []struct {
		name   string
		status int
	}{{"checkout HEAD failure is a failed step", 128}, {"checkout HEAD negative answer remains drift", 1}} {
		t.Run(tc.name, func(t *testing.T) {
			bed := newRearmBed(t, false)
			expected := append(rearmOwnedRef(bed.root, ref, bed.second), rearmBuild(bed.root, bed.second, bed.second, 0), rearmAncestor(bed.root, bed.second, ref, 0), rearmFailed(bed.root, tc.status, "rev-parse", "--verify", "HEAD^{commit}"))
			_, err := reArmRebuiltEngineWithDeps(rearmTestDeps(t, expected...), bed.root, bed.root, bed.engine)
			if tc.status == 1 {
				want := "ENROLLMENT_DRIFT: resolve landed source at checkout HEAD: git rev-parse --verify HEAD^{commit}: exit status 1 (fatal: not a git repository (or any of the parent directories))"
				if err == nil || err.Error() != want || !errors.Is(err, ErrEnrollmentDrift) {
					t.Fatalf("checkout HEAD negative answer changed its mint-plan drift: %v", err)
				}
			} else if err == nil || !strings.Contains(err.Error(), "resolve landed source at checkout HEAD") || !strings.Contains(err.Error(), "git rev-parse") || !strings.Contains(err.Error(), "fatal: not a git repository (or any of the parent directories)") || errors.Is(err, ErrEnrollmentDrift) || errors.Is(err, ErrNotOwned) {
				t.Fatalf("checkout HEAD repository failure became mint-plan drift: %v", err)
			}
		})
	}
}

func TestIdentityPublicationKeepsAndClearsDurabilityDoubt(t *testing.T) {
	root := canonicalPath(t.TempDir())
	path := RepoIdentityPath(root)
	original := identityWriter
	t.Cleanup(func() { identityWriter = original })
	id := InstallIdentity{RepoIdentity: root, Generation: 1, InstallPath: "/bin/true", MintedAt: "2026-09-06T00:00:00Z"}
	identityWriter = func(target, text, anchor string) (bool, error) {
		durable, err := original(target, text, anchor)
		if target == path && err == nil {
			return false, nil
		}
		return durable, err
	}
	if err := MintIdentity(path, id); err == nil || !strings.Contains(err.Error(), "durability is pending") {
		t.Fatalf("post-rename doubt was reported as durable success: %v", err)
	}
	if got, err := VerifyIdentity(path, root); err != nil || got.Generation != 1 {
		t.Fatalf("the visible generation was lost: %+v %v", got, err)
	}
	if _, err := os.Stat(identityDurabilityPendingPath(path)); err != nil {
		t.Fatalf("durability marker is absent: %v", err)
	}
	identityWriter = original
	if err := MintIdentity(path, id); err != nil {
		t.Fatalf("re-publication did not confirm durability: %v", err)
	}
	if _, err := os.Stat(identityDurabilityPendingPath(path)); !os.IsNotExist(err) {
		t.Fatalf("durability marker survived a confirmed publication: %v", err)
	}
	identityWriter = func(target, text, anchor string) (bool, error) {
		durable, err := original(target, text, anchor)
		if target == identityDurabilityPendingPath(path) && err == nil {
			return false, nil
		}
		return durable, err
	}
	if durable, err := publishIdentity(path, id); err != nil || durable {
		t.Fatalf("doubted marker publication did not become a pending outcome: durable=%v err=%v", durable, err)
	}
	if got, err := VerifyIdentity(path, root); err != nil || got.Generation != 1 {
		t.Fatalf("marker durability doubt stopped identity publication: %+v %v", got, err)
	}
	if _, err := os.Stat(identityDurabilityPendingPath(path)); err != nil {
		t.Fatalf("doubted durability marker is absent: %v", err)
	}
	identityWriter = original
	if err := MintIdentity(path, id); err != nil {
		t.Fatalf("marker-doubt re-publication did not confirm durability: %v", err)
	}
	if err := os.WriteFile(identityDurabilityPendingPath(path), []byte("pending\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyIdentity(path, root); err == nil || !strings.Contains(err.Error(), "absent") {
		t.Fatalf("missing identity did not use the unenrolled path: %v", err)
	}
	if _, err := os.Stat(identityDurabilityPendingPath(path)); err != nil {
		t.Fatalf("a verifier removed a marker owned by the arm path: %v", err)
	}
	_, armErr := armWithRearmDeps(root, "/bin/true", false, false, false, "",
		func(_ InstallIdentity, priorErr error, _ enrolledBytes) (mintPlan, error) {
			return mintPlan{}, priorErr
		}, rearmTestDeps(t))
	if armErr == nil || !strings.Contains(armErr.Error(), "identity absent") {
		t.Fatalf("orphan marker did not retain the existing no-identity outcome: %v", armErr)
	}
	if _, err := os.Stat(identityDurabilityPendingPath(path)); !os.IsNotExist(err) {
		t.Fatalf("locked arm path did not remove the orphan durability marker: %v", err)
	}
}

func TestRearmStageTransitionsAndDurability(t *testing.T) {
	t.Run("BeforeMint to Minted", func(t *testing.T) {
		bed := newRearmBed(t, false)
		outcome, err := bed.rearm(t)
		if err != nil || outcome.Stage != StageMinted || outcome.Status != "re-armed" || outcome.StoppedRunnerPid != 0 {
			t.Fatalf("direct mint transition: %+v %v", outcome, err)
		}
	})

	t.Run("BeforeMint to StopAttempted", func(t *testing.T) {
		bed := newRearmBed(t, true)
		original := runnerStopWriter
		runnerStopWriter = func(string, []byte, os.FileMode) error { return errors.New("injected marker write failure") }
		t.Cleanup(func() { runnerStopWriter = original })
		outcome, err := bed.rearm(t)
		if err == nil || outcome.Stage != StageStopAttempted || !strings.Contains(err.Error(), "write restart marker") {
			t.Fatalf("marker-write transition: %+v %v", outcome, err)
		}
		installed, _ := VerifyIdentity(RepoIdentityPath(bed.root), bed.root)
		if installed.Generation != 1 {
			t.Fatalf("stop-attempt failure minted generation %d", installed.Generation)
		}
	})

	t.Run("StopAttempted to Stopped then mint failure", func(t *testing.T) {
		bed := newRearmBed(t, true)
		original := identityWriter
		identityWriter = func(target, text, anchor string) (bool, error) {
			if target == RepoIdentityPath(bed.root) {
				return false, errors.New("injected identity publication failure")
			}
			return original(target, text, anchor)
		}
		t.Cleanup(func() { identityWriter = original })
		outcome, err := bed.rearm(t)
		if err == nil || outcome.Stage != StageStopped || !strings.Contains(err.Error(), "identity publication") {
			t.Fatalf("stopped mint failure: %+v %v", outcome, err)
		}
		installed, _ := VerifyIdentity(RepoIdentityPath(bed.root), bed.root)
		if installed.Generation != 1 {
			t.Fatalf("failed mint changed generation to %d", installed.Generation)
		}
	})

	t.Run("Stopped to Minted with durability pending", func(t *testing.T) {
		bed := newRearmBed(t, true)
		original := identityWriter
		identityWriter = func(target, text, anchor string) (bool, error) {
			durable, err := original(target, text, anchor)
			if target == RepoIdentityPath(bed.root) && err == nil {
				return false, nil
			}
			return durable, err
		}
		t.Cleanup(func() { identityWriter = original })
		outcome, err := bed.rearm(t)
		if err != nil || outcome.Stage != StageMinted || !outcome.DurabilityPending || outcome.Status != "re-armed" {
			t.Fatalf("doubted mint transition: %+v %v", outcome, err)
		}
		cadenceReads := 0
		verdict := checkStewardRunnerWithCadence(bed.root, time.Now(), processidentity.KernelProber{}, func(root string) int {
			cadenceReads++
			if root != bed.root {
				t.Fatalf("cadence read used root %q, want %q", root, bed.root)
			}
			return 600
		})
		if cadenceReads != 1 {
			t.Fatalf("cadence read ran %d times, want once", cadenceReads)
		}
		if !strings.Contains(verdict.Reason, "(durability pending)") {
			t.Fatalf("health hid identity durability doubt: %+v", verdict)
		}
	})
}

func TestConcurrentRearmsMintOneGeneration(t *testing.T) {
	bed := newRearmBed(t, true)
	entered := make(chan struct{})
	release := make(chan struct{})
	calls := 0
	afterArmDecision = func() {
		calls++
		if calls == 1 {
			close(entered)
			<-release
		}
	}
	t.Cleanup(func() { afterArmDecision = nil })
	type result struct {
		outcome ReArmOutcome
		err     error
	}
	results := make(chan result, 2)
	go func() {
		outcome, err := bed.rearm(t)
		results <- result{outcome, err}
	}()
	<-entered
	go func() {
		outcome, err := bed.alreadyCurrent(t)
		results <- result{outcome, err}
	}()
	close(release)
	first, second := <-results, <-results
	if first.err != nil || second.err != nil {
		t.Fatalf("concurrent rearm: first=%+v second=%+v", first, second)
	}
	statuses := map[string]int{first.outcome.Status: 1}
	statuses[second.outcome.Status]++
	if statuses["re-armed"] != 1 || statuses["already-current"] != 1 {
		t.Fatalf("concurrent statuses: %+v %+v", first.outcome, second.outcome)
	}
	installed, _ := VerifyIdentity(RepoIdentityPath(bed.root), bed.root)
	if installed.Generation != 2 {
		t.Fatalf("concurrent re-arms minted generation %d", installed.Generation)
	}
}

func TestRearmAndTemporaryHumanArmPreserveTheHumanWord(t *testing.T) {
	t.Run("temporary human arm wins the first lock", func(t *testing.T) {
		bed := newRearmBed(t, true)
		entered := make(chan struct{})
		release := make(chan struct{})
		calls := 0
		beforeArmLock = func() {
			calls++
			if calls == 1 {
				close(entered)
				<-release
			}
		}
		t.Cleanup(func() { beforeArmLock = nil })
		type result struct {
			outcome ReArmOutcome
			err     error
		}
		resultChannel := make(chan result, 1)
		go func() {
			outcome, err := bed.alreadyCurrent(t)
			resultChannel <- result{outcome: outcome, err: err}
		}()
		<-entered
		if _, err := rearmHumanArm(t, bed.root, bed.engine, true, "approved-word", "2026-09-30", EnrollmentTemporaryWord); err != nil {
			t.Fatal(err)
		}
		close(release)
		got := <-resultChannel
		if got.err != nil || got.outcome.Status != "already-current" {
			t.Fatalf("re-arm did not observe the completed human arm: %+v", got)
		}
		installed, err := VerifyIdentity(RepoIdentityPath(bed.root), bed.root)
		if err != nil || installed.MintedBy != "human-word" || installed.TemporaryHumanWord != "approved-word" || installed.ReviewBy != "2026-09-30" || installed.Enrollment != EnrollmentTemporaryWord {
			t.Fatalf("the human word was not preserved: %+v %v", installed, err)
		}
	})

	t.Run("temporary human arm follows the machine mint", func(t *testing.T) {
		bed := newRearmBed(t, true)
		machine, err := bed.rearm(t)
		if err != nil || machine.Status != "re-armed" {
			t.Fatalf("machine re-arm: %+v %v", machine, err)
		}
		if _, err := rearmHumanArm(t, bed.root, bed.engine, true, "approved-word", "2026-09-30", EnrollmentTemporaryWord); err != nil {
			t.Fatal(err)
		}
		installed, err := VerifyIdentity(RepoIdentityPath(bed.root), bed.root)
		if err != nil || installed.Generation != machine.Generation+1 || installed.MintedBy != "human-word" || installed.Enrollment != EnrollmentTemporaryWord ||
			installed.HumanWitnessedGeneration != installed.Generation || installed.TemporaryHumanWord != "approved-word" {
			t.Fatalf("the later human arm did not witness its own generation: %+v %v", installed, err)
		}
	})
}

func TestMachineRebuildCarriesFixtureEnrollmentForward(t *testing.T) {
	bed := newRearmBed(t, false)
	remoteTip := rearmID(3)
	ref := "refs/remotes/origin/trunk"
	installed, err := VerifyIdentity(RepoIdentityPath(bed.root), bed.root)
	if err != nil {
		t.Fatal(err)
	}
	installed.Enrollment = EnrollmentFixture
	if err := MintIdentity(RepoIdentityPath(bed.root), installed); err != nil {
		t.Fatal(err)
	}
	expected := append(rearmOwnedRef(bed.root, ref, remoteTip), rearmBuild(bed.root, bed.second, bed.second, 0), rearmAncestor(bed.root, bed.second, ref, 0), rearmExpected(bed.root, bed.second+"\n", nil, "rev-parse", "--verify", "HEAD^{commit}"))
	if remoteTip == bed.second {
		t.Fatal("remote fixture did not move ahead of checkout HEAD")
	}
	outcome, err := reArmRebuiltEngineWithDeps(rearmTestDeps(t, expected...), bed.root, bed.root, bed.engine)
	if err != nil || outcome.Status != "re-armed" {
		t.Fatalf("machine re-arm with newer remote engine landing: %+v %v", outcome, err)
	}
	installed, err = VerifyIdentity(RepoIdentityPath(bed.root), bed.root)
	if err != nil || installed.Enrollment != EnrollmentFixture || installed.EngineBuild != bed.second || installed.LandedCommit != bed.second {
		t.Fatalf("machine rebuild did not retain checkout landing provenance: %+v %v", installed, err)
	}
}

func TestWitnessStampedMachineEnrollmentSurvivesLedgerOnlyLanding(t *testing.T) {
	bed := newRearmBed(t, false)
	policy, err := behaviorsurface.Load()
	if err != nil {
		t.Fatal(err)
	}
	digest, err := policy.Digest(bed.root, behaviorsurface.Engine)
	if err != nil {
		t.Fatal(err)
	}
	stamp := "witness-" + digest[:12]
	buildFakeRunner(t, bed.engine, stamp)
	ref := "refs/remotes/origin/trunk"
	landed := rearmID(3)
	tree := rearmID(101)
	baseHistory := rearmHistory(bed.second, tree, "", rearmRawChange("cmd/surface.txt", strings.Repeat("0", 40), rearmID(201)))
	nextHistory := rearmHistory(landed, tree, bed.second, rearmRawChange("plans/goals/peer.md", strings.Repeat("0", 40), rearmID(202))) + baseHistory
	expected := append(rearmOwnedRef(bed.root, ref, bed.second), rearmWitnessExpectations(bed.root, ref, baseHistory)...)
	expected = append(expected, rearmAncestor(bed.root, bed.second, ref, 0), rearmExpected(bed.root, bed.second+"\n", nil, "rev-parse", "--verify", "HEAD^{commit}"))
	expected = append(expected, rearmWitnessExpectations(bed.root, landed, nextHistory)...)
	expected = append(expected, rearmAncestor(bed.root, landed, landed, 0), rearmAncestor(bed.root, landed, bed.second, 1), rearmAncestor(bed.root, bed.second, landed, 0), rearmSkew(bed.root, bed.second, landed, ""), rearmDiff(bed.root, landed, landed, ""))
	deps := rearmTestDeps(t, expected...)
	deps.witnessTreeDigest = func(_ context.Context, root, spec string, _ behaviorsurface.Policy, _ RearmClock, _ int) (string, error) {
		if root != bed.root || spec != bed.second {
			t.Fatalf("witness archive used root=%q spec=%q", root, spec)
		}
		return digest, nil
	}
	if outcome, err := reArmRebuiltEngineWithDeps(deps, bed.root, bed.root, bed.engine); err != nil || outcome.Status != "re-armed" {
		t.Fatalf("arm witness-stamped machine engine: %+v %v", outcome, err)
	}
	pinned, err := OpenEnrolledBinary(bed.root)
	if err != nil {
		t.Fatal(err)
	}
	defer pinned.Close()
	if pinned.Install.LandedCommit != bed.second || pinned.BuildStamp() != stamp {
		t.Fatalf("witness enrollment was not valid before ledger movement: %+v stamp=%s", pinned.Install, pinned.BuildStamp())
	}
	if err := pinned.verifySourceAtDestinationWithDeps(deps, SystemRearmClock(), bed.root, landed); err != nil {
		t.Fatalf("witness-stamped enrollment drifted after ledger-only landing: %v", err)
	}
}

func TestHumanArmBesideALiveLegacyRunnerNamesRestart(t *testing.T) {
	bed := newRearmBed(t, false)
	digest, err := installDigest(bed.engine)
	if err != nil {
		t.Fatal(err)
	}
	legacy := InstallIdentity{
		RepoIdentity: bed.root, Generation: 1, InstallPath: bed.engine, InstallDigest: digest,
		MintedAt: time.Now().UTC().Format(time.RFC3339),
	}
	if err := MintIdentity(RepoIdentityPath(bed.root), legacy); err != nil {
		t.Fatal(err)
	}
	pinned, err := OpenEnrolledBinary(bed.root)
	if err != nil {
		t.Fatal(err)
	}
	if err := pinned.PrepareForExecution(); err != nil {
		pinned.Close()
		t.Fatal(err)
	}
	if _, err := launchRunner(bed.root, pinned, ""); err != nil {
		pinned.Close()
		t.Fatal(err)
	}
	pinned.Close()
	message, err := rearmHumanArm(t, bed.root, bed.engine, false, "", "", EnrollmentHumanTerminal)
	if err != nil || !strings.Contains(message, "already armed") || !strings.Contains(message, "steward restart") || !strings.Contains(message, "LEGACY") {
		t.Fatalf("plain arm hid the clearing verb: %q %v", message, err)
	}
	installed, _ := VerifyIdentity(RepoIdentityPath(bed.root), bed.root)
	if installed.Generation != 1 {
		t.Fatalf("plain arm minted generation %d", installed.Generation)
	}
	if _, err := rearmHumanArm(t, bed.root, bed.engine, true, "", "", EnrollmentHumanTerminal); err != nil {
		t.Fatal(err)
	}
	installed, err = VerifyIdentity(RepoIdentityPath(bed.root), bed.root)
	if err != nil || installed.Generation != 2 || installed.MintedBy != "human-terminal" || installed.HumanWitnessedGeneration != 2 || installed.Enrollment != EnrollmentHumanTerminal {
		t.Fatalf("restart did not witness the replacement generation: %+v %v", installed, err)
	}
}

func TestHumanArmWithChangedEnrolledBytesReplacesLiveRunnerAndWitnessesGeneration(t *testing.T) {
	bed := newRearmBed(t, true)
	before, alive := liveRunner(bed.root)
	if !alive {
		t.Fatal("fixture runner was not alive before the human arm")
	}

	message, err := rearmHumanArm(t, bed.root, bed.engine, false, "", "", EnrollmentHumanTerminal)
	if err != nil || !strings.Contains(message, fmt.Sprintf("replaced live runner pid %d", before.Pid)) ||
		!strings.Contains(message, "human-terminal generation 2") {
		t.Fatalf("human arm did not report the witnessed replacement: %q %v", message, err)
	}
	installed, err := VerifyIdentity(RepoIdentityPath(bed.root), bed.root)
	if err != nil || installed.Generation != 2 || installed.MintedBy != "human-terminal" ||
		installed.HumanWitnessedGeneration != 2 || installed.EngineBuild != bed.second {
		t.Fatalf("human arm did not mint its own witnessed generation: %+v %v", installed, err)
	}
	after, alive := liveRunner(bed.root)
	if !alive || after.Pid == before.Pid {
		t.Fatalf("human arm did not replace the live runner: before=%+v after=%+v alive=%t", before, after, alive)
	}
}

func TestCommandTimeDriftSurvivesTheStewardLaunchChain(t *testing.T) {
	bed := newRearmBed(t, false)
	// Restore the first engine so the staged enrollment is current.
	buildFakeRunner(t, bed.engine, bed.first)
	pinned, err := OpenEnrolledBinary(bed.root)
	if err != nil {
		t.Fatal(err)
	}
	defer pinned.Close()
	if err := pinned.PrepareForExecution(); err != nil {
		t.Fatal(err)
	}
	replacement := pinned.execPath + ".changed"
	if err := testexec.WriteFile(replacement, []byte("changed\n"), 0o500); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(replacement, pinned.execPath); err != nil {
		t.Fatal(err)
	}
	_, err = repairPinnedRunner(bed.root, pinned, nil, time.Second)
	if !errors.Is(err, ErrEnrollmentDrift) {
		t.Fatalf("command-time drift lost its typed cause: %v", err)
	}
}

func TestSignalFailureNamesStopAttempted(t *testing.T) {
	bed := newRearmBed(t, true)
	original := runnerSignal
	runnerSignal = func(pid int, signal syscall.Signal) error {
		if signal == syscall.SIGTERM {
			return errors.New("injected termination failure")
		}
		return original(pid, signal)
	}
	t.Cleanup(func() { runnerSignal = original })
	outcome, err := bed.rearm(t)
	if err == nil || outcome.Stage != StageStopAttempted || !strings.Contains(err.Error(), "stop runner pid") {
		t.Fatalf("signal failure transition: %+v %v", outcome, err)
	}
}
