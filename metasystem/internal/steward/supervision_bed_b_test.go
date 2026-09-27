package steward

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	processidentity "github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testgit"
)

// supBFixtureEnrolledBed is newRearmBed with the standing enrollment the
// supervision bed writes: generation 1, no minting provenance and no human
// witness, whose engine bytes were since rebuilt from the landed commit.
func supBFixtureEnrolledBed(t *testing.T) rearmBed {
	t.Helper()
	bed := newRearmBed(t, false)
	installed, err := VerifyIdentity(RepoIdentityPath(bed.root), bed.root)
	if err != nil {
		t.Fatal(err)
	}
	installed.MintedBy, installed.HumanWitnessedGeneration, installed.HumanWitnessedAt, installed.EngineBuild = "", 0, "", ""
	if err := MintIdentity(RepoIdentityPath(bed.root), installed); err != nil {
		t.Fatal(err)
	}
	return bed
}

func supBGenerationPins(t *testing.T, root string, generation string) []string {
	t.Helper()
	pins, err := filepath.Glob(filepath.Join(root, "artifacts", "agents", "steward", "engine-pins", "generation-"+generation+"-*"))
	if err != nil {
		t.Fatal(err)
	}
	return pins
}

// TestSupBLandedRebuildReArmsWithMachineProvenance ports rearm-rebuild of
// supervision-fixtures part B: a rebuilt engine whose stamp is landed on the
// owned remote-tracking ref re-arms at generation 2 before session work,
// records machine-rebuild provenance without a human witness, pins the
// generation-2 engine, confirms a live runner, and health names the
// machine-minted provenance.
func TestSupBLandedRebuildReArmsWithMachineProvenance(t *testing.T) {
	bed := supBFixtureEnrolledBed(t)
	reapStewardRunnerFixture(t, bed.root)
	ref := "refs/remotes/origin/trunk"
	outcome, err := bed.rearm(t)
	if err != nil || outcome.Status != "re-armed" || outcome.Stage != StageMinted || outcome.Generation != 2 || outcome.PreviousGeneration != 1 ||
		outcome.EngineBuild != bed.second || outcome.LandedCommit != bed.second || outcome.LandingRef != ref || outcome.StoppedRunnerPid != 0 {
		t.Fatalf("landed rebuild re-arm = %+v err=%v", outcome, err)
	}
	installed, err := VerifyIdentity(RepoIdentityPath(bed.root), bed.root)
	if err != nil || installed.Generation != 2 || installed.MintedBy != "machine-rebuild" || installed.HumanWitnessedGeneration != 0 ||
		installed.EngineBuild != bed.second || installed.LandedCommit != bed.second || installed.LandingRef != ref {
		t.Fatalf("landed rebuild identity = %+v err=%v", installed, err)
	}
	if pins := supBGenerationPins(t, bed.root, "2"); len(pins) == 0 {
		t.Fatal("the landed rebuild prepared no generation-2 execution pin")
	}
	runner, alive := liveRunner(bed.root)
	if !alive || runner.Pid != outcome.RunnerPid || outcome.RunnerPid < 1 {
		t.Fatalf("the landed rebuild left no live runner: record=%+v alive=%t outcome=%+v", runner, alive, outcome)
	}
	wantProvenance := "enrollment generation 2 machine-minted (rebuild, engine " + bed.second + ", landed " + bed.second[:7] + " on " + ref + "); no human witness is recorded"
	if got := EnrollmentProvenance(installed); got != wantProvenance {
		t.Fatalf("enrollment provenance = %q, want %q", got, wantProvenance)
	}
	verdict := checkStewardRunnerWithCadence(bed.root, time.Now(), processidentity.KernelProber{}, func(string) int { return 600 })
	if !strings.Contains(verdict.Reason, wantProvenance) {
		t.Fatalf("health hid the machine-minted provenance: %+v", verdict)
	}
}

// TestSupBPostMintLaunchFailureKeepsGenerationTwoAndTheNextArmIsCurrent ports
// rearm-launch-fails: a runner launch that fails after the mint still reports
// the minted generation, keeps the generation-2 enrollment and pin, and the
// next re-arm finds the engine current instead of minting again.
func TestSupBPostMintLaunchFailureKeepsGenerationTwoAndTheNextArmIsCurrent(t *testing.T) {
	bed := supBFixtureEnrolledBed(t)
	blocked := runnerLogPath(bed.root)
	if err := os.MkdirAll(blocked, 0o755); err != nil {
		t.Fatal(err)
	}
	outcome, err := bed.rearm(t)
	if err == nil || outcome.Stage != StageMinted || outcome.Status != "re-armed" || outcome.Generation != 2 || outcome.PreviousGeneration != 1 || outcome.RunnerPid != 0 {
		t.Fatalf("post-mint launch failure = %+v err=%v", outcome, err)
	}
	installed, readErr := VerifyIdentity(RepoIdentityPath(bed.root), bed.root)
	if readErr != nil || installed.Generation != 2 || installed.MintedBy != "machine-rebuild" {
		t.Fatalf("post-mint launch failure lost generation two: %+v %v", installed, readErr)
	}
	if pins := supBGenerationPins(t, bed.root, "2"); len(pins) == 0 {
		t.Fatal("the minted generation has no execution pin")
	}
	if _, alive := liveRunner(bed.root); alive {
		t.Fatal("a failed launch published a live runner")
	}
	if err := os.Remove(blocked); err != nil {
		t.Fatal(err)
	}
	again, err := bed.alreadyCurrent(t)
	if err != nil || again.Status != "already-current" || again.Generation != 2 || again.Stage != StageBeforeMint {
		t.Fatalf("the next re-arm after the failed launch = %+v err=%v", again, err)
	}
	current, readErr := VerifyIdentity(RepoIdentityPath(bed.root), bed.root)
	if readErr != nil || current.Generation != 2 || current.InstallDigest != installed.InstallDigest || current.InstallPath != installed.InstallPath {
		t.Fatalf("the next re-arm changed the generation-2 enrollment: %+v %v", current, readErr)
	}
}

// TestSupBIneligibleRebuildsRefuseBeforeTouchingTheEnrollment ports
// rearm-provenance's refusals: a development stamp, a stamp not landed on the
// owned ref, an unset landing ref and an unqualified landing ref each refuse
// as enrollment drift that names its cause, before any runner stop or mint.
func TestSupBIneligibleRebuildsRefuseBeforeTouchingTheEnrollment(t *testing.T) {
	ref := "refs/remotes/origin/trunk"
	configArgs := []string{"config", "--local", "--no-includes", "--get", landingRefConfigKey}
	for _, test := range []struct {
		name  string
		stamp string
		git   func(bed rearmBed) []testgit.Expectation
		want  string
	}{
		{
			name: "development stamp", stamp: "dev",
			git:  func(bed rearmBed) []testgit.Expectation { return rearmOwnedRef(bed.root, ref, bed.second) },
			want: "rebuilt engine carries build stamp dev; automatic re-arm is bounded to landed commits",
		},
		{
			name: "unlanded commit",
			git: func(bed rearmBed) []testgit.Expectation {
				return append(rearmOwnedRef(bed.root, ref, bed.first),
					rearmBuild(bed.root, bed.second, bed.second, 0), rearmAncestor(bed.root, bed.second, ref, 1))
			},
			want: "rebuilt engine was built from " + strings.Repeat("0", 39) + "2, which is not landed on " + ref,
		},
		{
			name: "no landing ref",
			git: func(bed rearmBed) []testgit.Expectation {
				return []testgit.Expectation{rearmFailed(bed.root, 1, configArgs...)}
			},
			want: "the installation owns no remote-tracking landing ref (" + landingRefConfigKey + " is <unset>; expected refs/remotes/<remote>/<branch>)",
		},
		{
			name: "unqualified landing ref",
			git: func(bed rearmBed) []testgit.Expectation {
				return []testgit.Expectation{rearmExpected(bed.root, "main\n", nil, configArgs...)}
			},
			want: "the installation owns no remote-tracking landing ref (" + landingRefConfigKey + " is main; expected refs/remotes/<remote>/<branch>)",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			bed := supBFixtureEnrolledBed(t)
			if test.stamp != "" {
				buildFakeRunner(t, bed.engine, test.stamp)
			}
			before, err := os.ReadFile(RepoIdentityPath(bed.root))
			if err != nil {
				t.Fatal(err)
			}
			outcome, err := reArmRebuiltEngineWithDeps(rearmTestDeps(t, test.git(bed)...), bed.root, bed.root, bed.engine)
			if err == nil || !errors.Is(err, ErrEnrollmentDrift) || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("%s re-arm error = %v, want enrollment drift naming %q", test.name, err, test.want)
			}
			if outcome.Stage != StageBeforeMint || outcome.StoppedRunnerPid != 0 || outcome.Generation != 0 || outcome.Status != "" {
				t.Fatalf("%s refusal reached past the eligibility decision: %+v", test.name, outcome)
			}
			after, err := os.ReadFile(RepoIdentityPath(bed.root))
			if err != nil || string(after) != string(before) {
				t.Fatalf("%s refusal changed the enrollment: %v", test.name, err)
			}
			if pins := supBGenerationPins(t, bed.root, "2"); len(pins) != 0 {
				t.Fatalf("%s refusal pinned a new generation: %v", test.name, pins)
			}
		})
	}
}

// TestSupBStoppedRunnerLoopNamesTheAgentFreeStartRemedy ports the stop-fence
// steward-run row: the runner loop refuses a completed stop with exactly the
// stopped sentence and the agent-free start remedy.
func TestSupBStoppedRunnerLoopNamesTheAgentFreeStartRemedy(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	closeProcessFence(t, root, 3)
	err := RunLoop(root, fakeCensus{}, nil, time.Hour, TickConfig{})
	want := "the metasystem is stopped for " + root + " since 2026-09-07T08:00:00Z, by stop pid 75\n" +
		"at an agent-free terminal, run: metasystem system start --repo " + root
	var stopped *StoppedError
	if !errors.As(err, &stopped) || err.Error() != want {
		t.Fatalf("stopped runner loop = %v, want %q", err, want)
	}
	if _, statErr := os.Stat(runnerDir(root)); !os.IsNotExist(statErr) {
		t.Fatalf("the stopped runner loop created runner state: %v", statErr)
	}
}
