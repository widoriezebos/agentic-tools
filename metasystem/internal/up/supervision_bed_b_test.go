package up

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stopfence"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/supervise"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

const (
	supBLanded     = "0123456789abcdef0123456789abcdef01234567"
	supBStamp      = "0123456789ab"
	supBLandingRef = "refs/remotes/origin/trunk"
)

// supBRebuiltEnrollment stages a generation-1 enrollment whose engine bytes
// were since rebuilt, the state a landed rebuild leaves before up runs.
func supBRebuiltEnrollment(t *testing.T) (root, binary string) {
	t.Helper()
	root = canonicalRuntimePath(t.TempDir())
	binary = filepath.Join(root, "metasystem")
	if err := testexec.WriteFile(binary, []byte("accepted\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	stageEnrollment(t, root, binary, 1)
	if err := testexec.WriteFile(binary, []byte("rebuilt\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root, binary
}

// supBMintingReArm is the steward re-arm owner for one call: it mints
// generation 2 for the rebuilt bytes and reports the landed provenance, then
// returns launchErr as the post-mint runner launch result.
func supBMintingReArm(t *testing.T, root, binary string, calls *int, launchErr error) func(string, string, string) (steward.ReArmOutcome, error) {
	return func(repoRoot, installationRoot, invokingBinary string) (steward.ReArmOutcome, error) {
		*calls++
		if repoRoot != root || installationRoot != root || invokingBinary != binary {
			t.Fatalf("re-arm asked for repo=%q installation=%q binary=%q", repoRoot, installationRoot, invokingBinary)
		}
		stageEnrollment(t, root, binary, 2)
		outcome := steward.ReArmOutcome{
			Status: "re-armed", Stage: steward.StageMinted, Generation: 2, PreviousGeneration: 1,
			EngineBuild: supBStamp, LandedCommit: supBLanded, LandingRef: supBLandingRef,
		}
		if launchErr == nil {
			outcome.RunnerPid = 4242
		}
		return outcome, launchErr
	}
}

func supBArmingLog(t *testing.T, root string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(supervise.SupervisionDir(root), "arming.log"))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// TestSupBUpRendersALandedReArmAndLogsIt ports the rendering half of
// rearm-rebuild (supervision-fixtures part B): up reports the accepted engine
// as re-armed with its runner, generation, stamp, landed commit and ref,
// carries the re-armed fact on its aggregate line, and appends the
// engine-re-armed line to the arming log before session work.
func TestSupBUpRendersALandedReArmAndLogsIt(t *testing.T) {
	t.Parallel()
	root, binary := supBRebuiltEnrollment(t)
	calls := 0
	result := ordinary(Options{
		Root: root, MetasystemRoot: root, Scope: root, Binary: binary, WaitScaleMilli: 1,
		// A pid without its start time stops the call at session identity,
		// after the re-arm, without reading any live process.
		Pid:                1,
		ReArmRebuiltEngine: supBMintingReArm(t, root, binary, &calls, nil),
	})
	if calls != 1 {
		t.Fatalf("up asked the steward to re-arm %d times, want once", calls)
	}
	wantFact := "generation=2 previous=1 engine=" + supBStamp + " landed=0123456"
	wantEngine := "component=accepted-engine outcome=re-armed detail=\"the enrolled engine was rebuilt; armed (runner pid 4242) (generation=2 previous=1 engine=" +
		supBStamp + " landed=0123456 ref=" + supBLandingRef + " path=" + binary + ")\""
	lines := result.Lines()
	if result.ReArmed != wantFact || len(lines) < 2 || lines[1] != wantEngine ||
		!strings.Contains(lines[len(lines)-1], " re-armed=\""+wantFact+"\"") || result.Failed != "session-identity" {
		t.Fatalf("re-armed up = %#v lines=%q", result, lines)
	}
	if log := supBArmingLog(t, root); !strings.Contains(log, " engine-re-armed generation=2 previous=1 engine="+supBStamp+" landed=0123456 ref="+supBLandingRef+"\n") {
		t.Fatalf("the arming log omitted the re-arm: %q", log)
	}
}

// TestSupBUpReportsAPostMintLaunchFailureAndTheNextUpConverges ports the
// rendering half of rearm-launch-fails: a runner launch that fails after the
// mint reports both the re-arm and the failed runner, and the next up
// verifies the generation-2 enrollment without minting and hands that
// generation to the runner owner.
func TestSupBUpReportsAPostMintLaunchFailureAndTheNextUpConverges(t *testing.T) {
	t.Parallel()
	root, binary := supBRebuiltEnrollment(t)
	calls := 0
	options := Options{
		Root: root, MetasystemRoot: root, Scope: root, Binary: binary, WaitScaleMilli: 1, Pid: 1,
		ReArmRebuiltEngine: supBMintingReArm(t, root, binary, &calls, errors.New("open runner log: is a directory")),
	}
	failed := ordinary(options)
	wantFact := "generation=2 previous=1 engine=" + supBStamp + " landed=0123456"
	lines := failed.Lines()
	joined := strings.Join(lines, "\n")
	if failed.ExitCode() != 1 || failed.Failed != "steward-runner" || failed.ReArmed != wantFact ||
		!strings.Contains(joined, "component=accepted-engine outcome=re-armed detail=\"generation=2 previous=1 engine="+supBStamp+" landed=0123456 ref="+supBLandingRef+"\"") ||
		!strings.Contains(joined, "component=steward-runner outcome=failed detail=\"after re-arm: open runner log: is a directory\"") ||
		!strings.HasPrefix(lines[len(lines)-1], "up outcome=failed re-armed=\""+wantFact+"\" component=steward-runner") {
		t.Fatalf("post-mint launch failure = %#v\n%s", failed, joined)
	}
	if log := supBArmingLog(t, root); !strings.Contains(log, " engine-re-armed generation=2 previous=1 ") {
		t.Fatalf("the arming log omitted the minted re-arm: %q", log)
	}

	options.ReArmRebuiltEngine = func(string, string, string) (steward.ReArmOutcome, error) {
		t.Fatal("the next up re-armed an engine that is already enrolled")
		return steward.ReArmOutcome{}, nil
	}
	next := ordinary(options)
	if len(next.Components) < 2 || next.Components[1] != (ComponentOutcome{Component: "accepted-engine", Outcome: "verified", Detail: "generation=2 path=" + binary}) || next.ReArmed != "" {
		t.Fatalf("the next up = %#v", next)
	}
	installed, err := steward.VerifyIdentity(steward.RepoIdentityPath(root), root)
	if err != nil || installed.Generation != 2 || calls != 1 {
		t.Fatalf("the next up minted again: %+v err=%v calls=%d", installed, err, calls)
	}

	enrolled, err := steward.OpenEnrolledBinary(root)
	if err != nil {
		t.Fatal(err)
	}
	defer enrolled.Close()
	components, runnerFailed := ensureStewardRunner(Options{
		Root: root, WaitScaleMilli: 1,
		EnsureStewardRunner: func(repoRoot string, got *steward.EnrolledBinary) (steward.EnsureRunnerResult, error) {
			if repoRoot != root || got != enrolled {
				t.Fatalf("runner owner asked for repo=%q enrolled=%p", repoRoot, got)
			}
			return steward.EnsureRunnerResult{Action: "started", Pid: 5151, Generation: got.Install.Generation}, nil
		},
	}, enrolled, nil)
	if runnerFailed != nil || len(components) != 1 ||
		(Result{Components: components}).Lines()[0] != "component=steward-runner outcome=started detail=\"pid=5151 generation=2\"" {
		t.Fatalf("repaired runner = %#v failure=%#v", components, runnerFailed)
	}
}

// TestSupBSurvivingMainJoinsTheReArmedSupervisionAsVerified ports the join
// half of arm-again: after arm opened the fence at a new generation, the
// surviving main's up asks supervision for that generation and reports the
// owner, watcher and reaper as verified, closing its creation claim.
func TestSupBSurvivingMainJoinsTheReArmedSupervisionAsVerified(t *testing.T) {
	t.Parallel()
	root := canonicalRuntimePath(t.TempDir())
	binary := filepath.Join(root, "metasystem")
	if err := testexec.WriteFile(binary, []byte("engine\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	stageEnrollment(t, root, binary, 1)
	enrolled, err := steward.OpenEnrolledBinary(root)
	if err != nil {
		t.Fatal(err)
	}
	defer enrolled.Close()
	if err := stopfence.Write(root, stopfence.Record{
		State: stopfence.StateOpen, Phase: stopfence.PhaseArmed, Generation: 5,
		ChangedAt: "2026-09-27T13:00:00Z", Checkout: root, NotStopped: []stopfence.Survivor{},
		By: stopfence.Actor{Verb: "arm", Process: stopfence.Process{Pid: 10, PidStartedAt: 20}},
	}); err != nil {
		t.Fatal(err)
	}
	options := Options{
		Root: root, MetasystemRoot: syntheticFingerprintRoot(t), Scope: root, Binary: binary, WaitScaleMilli: 1,
		EnsureArmed: func(armed supervise.EnsureOptions) (supervise.EnsureResult, error) {
			if armed.Root != root || armed.FenceGeneration != 5 || armed.OnlyIfDown {
				t.Fatalf("supervision arming options = %+v", armed)
			}
			claims, err := stopfence.Claims(root, 5)
			if err != nil || len(claims) != 1 || claims[0].Verb != "supervision-owner" {
				t.Fatalf("supervision arming ran outside one creation claim: %+v %v", claims, err)
			}
			return supervise.EnsureResult{Action: "verified", Owner: supervise.ArmingOwner{Pid: 6161}, Generation: 5}, nil
		},
	}
	components, _, failed := ensureSupervision(options, enrolled, nil)
	if failed != nil {
		t.Fatalf("join failed: %#v", failed)
	}
	want := []string{
		"component=supervision-owner outcome=verified detail=\"pid=6161 generation=5\"",
		"component=repo-watcher outcome=verified detail=\"generation=5\"",
		"component=job-reaper outcome=verified detail=\"generation=5\"",
		"up outcome=armed authority=writer",
	}
	if got := (Result{Components: components, Outcome: "armed", Authority: "writer"}).Lines(); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("join lines = %q, want %q", got, want)
	}
	if claims, err := stopfence.Claims(root, 5); err != nil || len(claims) != 0 {
		t.Fatalf("the join left its creation claim: %+v %v", claims, err)
	}
}

// TestSupBClosedFenceUpAnswersStoppedWithTheStartRemedy ports the stop-fence
// up rows: ordinary and recovery-only up under a completed stop both succeed
// with the stopped outcome and name the start command, exactly.
func TestSupBClosedFenceUpAnswersStoppedWithTheStartRemedy(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := stopfence.Write(root, stopfence.Record{
		State: stopfence.StateClosed, Phase: stopfence.PhaseStopped, Generation: 6,
		ChangedAt: "2026-09-27T12:30:00Z", Checkout: root, NotStopped: []stopfence.Survivor{},
		By: stopfence.Actor{Verb: "stop", Process: stopfence.Process{Pid: 72, PidStartedAt: 70}},
	}); err != nil {
		t.Fatal(err)
	}
	want := "component=stopped outcome=standing detail=\"since 2026-09-27T12:30:00Z\"\n" +
		"up outcome=stopped remedy=\"metasystem system start --repo " + root + "\""
	for _, options := range []Options{
		{Root: root, MetasystemRoot: root, Scope: root},
		{Root: root, MetasystemRoot: root, Scope: root, RecoverOnly: true, IfDown: true},
	} {
		result, closed := closedFenceResult(options)
		if !closed || result.ExitCode() != 0 || strings.Join(result.Lines(), "\n") != want {
			t.Fatalf("closed-fence up (recover-only=%t) = %#v closed=%t, want %q", options.RecoverOnly, result, closed, want)
		}
	}
	if entries, err := os.ReadDir(filepath.Join(root, "artifacts", "agents")); err != nil || len(entries) != 1 || entries[0].Name() != "supervision" {
		t.Fatalf("closed-fence up created state beside the fence: %v %v", entries, err)
	}
}
