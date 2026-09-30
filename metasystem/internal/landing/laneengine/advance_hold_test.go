package laneengine

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/enginebuild"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lock"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

// F-2: while the engine changes, a proof cannot take the host's proving
// lock and a begin cannot take the host flock it reads the pause under:
// both are refused (their non-blocking tries are busy) until the re-arm has
// finished, and both are free afterwards.
func TestAdvanceHoldsTheLanesLocksWhileTheEngineChanges(t *testing.T) {
	t.Parallel()
	bed := newAdvanceBed(t)
	bed.enrollOld(t)
	steps := bed.steps(t)
	rearm := steps.ReArm
	tries := func() (proving, host bool) {
		for _, path := range []string{lane.ProvingPath(bed.home), lane.LockPath(bed.home)} {
			held, err := lock.File(path, 0o600, lock.TryExclusive)
			busy := lock.Busy(err)
			if err == nil {
				_ = held.Release()
			} else if !busy {
				t.Fatalf("try %s: %v", path, err)
			}
			if path == lane.ProvingPath(bed.home) {
				proving = busy
			} else {
				host = busy
			}
		}
		return proving, host
	}
	during := [2]bool{}
	steps.ReArm = func(installPath string) (steward.ReArmOutcome, error) {
		during[0], during[1] = tries()
		return rearm(installPath)
	}
	outcome, err := Advance(bed.request(t), ProductionConditions(bed.home, bed.checkout), steps)
	if err != nil || !outcome.Changed {
		t.Fatalf("advance = %+v %v", outcome, err)
	}
	if !during[0] || !during[1] {
		t.Fatalf("during the re-arm: proving lock busy %v, host flock busy %v; want a concurrent proof and begin refused", during[0], during[1])
	}
	if proving, host := tries(); proving || host {
		t.Fatalf("after the advance: proving busy %v, host busy %v; want both free", proving, host)
	}
}

// F-1: a re-arm that fails after the steward wrote the new enrollment keeps
// the new bytes (the enrollment names them) and says the steward didn't
// start, with the person's command; the old bytes are not put back.
func TestAdvanceKeepsTheNewEngineWhenReArmFailsAfterEnrolling(t *testing.T) {
	t.Parallel()
	bed := newAdvanceBed(t)
	bed.enrollOld(t)
	steps := bed.steps(t)
	rearm := steps.ReArm
	steps.ReArm = func(installPath string) (steward.ReArmOutcome, error) {
		minted, err := rearm(installPath)
		if err != nil {
			return minted, err
		}
		return steward.ReArmOutcome{Stage: steward.StageMinted, Generation: 4}, errors.New("the steward runner died before guarding the repository")
	}
	_, err := Advance(bed.request(t), ProductionConditions(bed.home, bed.checkout), steps)
	refusal := refusalOf(t, err, "LANE_ENGINE_ADVANCE_RUNNER_DOWN")
	if want := []string{"metasystem", "system", "start", "--repo", bed.checkout}; !slices.Equal(refusal.Argv, want) {
		t.Fatalf("argv %q, want %q", refusal.Argv, want)
	}
	installed := bed.installed(t)
	enrolled, readErr := Enrollment(bed.installation)
	if readErr != nil || bytes.Equal(installed, bed.enrolled) || enrolled.InstallDigest != digestOf(installed) {
		t.Fatalf("installed %q under enrollment %+v %v; want the new engine the enrollment names", installed, enrolled, readErr)
	}
	bed.nothingLeft(t)
}

// F-5: a staged build or kept enrolled copy left by an advance whose
// process died is cleared by the next advance; a live process's is kept.
func TestAdvanceClearsWhatADeadAdvanceLeft(t *testing.T) {
	t.Parallel()
	bed := newAdvanceBed(t)
	bed.enrollOld(t)
	dir := filepath.Dir(bed.installPath)
	dead := []string{filepath.Join(dir, ".metasystem.enrolled.99999999"), filepath.Join(dir, ".metasystem.advance.99999999")}
	live := filepath.Join(dir, ".metasystem.advance."+strconv.Itoa(os.Getppid()))
	for _, path := range append(dead, live) {
		if err := testexec.WriteFile(path, []byte("left"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if outcome, err := bed.advance(t); err != nil || !outcome.Changed {
		t.Fatalf("advance = %+v %v", outcome, err)
	}
	for _, path := range dead {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("%s survived the advance: %v", path, err)
		}
	}
	if _, err := os.Stat(live); err != nil {
		t.Errorf("a live process's staging was removed: %v", err)
	}
}

// F-5: the build's stamp is the commit it classifies; an inherited
// METASYSTEM_BUILD_STAMP never reaches it.
func TestAdvanceBuildIgnoresAnInheritedStamp(t *testing.T) {
	t.Parallel()
	got := buildEnvironment([]string{"HOME=/h", "METASYSTEM_BUILD_STAMP=witness-abcdef012345", "PATH=/bin"})
	if want := []string{"HOME=/h", "PATH=/bin"}; !slices.Equal(got, want) {
		t.Fatalf("environment %q, want %q", got, want)
	}
}

// F-4, Skip path: the landed build is byte-identical to the enrolled engine;
// the real steward re-arm finds it already current and changes nothing.
func TestAdvanceThroughTheRealStewardAlreadyCurrent(t *testing.T) {
	t.Parallel()
	bed := newAdvanceBed(t)
	landed := []byte("engine @ " + enginebuild.StampRecord(bed.main))
	bed = realStewardBedFrom(t, bed, landed)
	steps := bed.steps(t)
	steps.ReArm = ProductionSteps(bed.checkout, bed.installation).ReArm
	outcome, err := Advance(bed.request(t), ProductionConditions(bed.home, bed.checkout), steps)
	if err != nil || outcome.Changed || outcome.Generation != 3 || !bytes.Equal(bed.installed(t), landed) {
		t.Fatalf("advance through the real steward = %+v %v; want already current, generation 3 kept", outcome, err)
	}
	bed.nothingLeft(t)
}

// F-4 with F-1's case, through the real steward: the new landed engine is
// enrolled as generation 4, then its runner cannot start (the fixture bytes
// are not a program); the new bytes stay installed and enrolled.
func TestAdvanceThroughTheRealStewardRunnerFailsAfterEnrolling(t *testing.T) {
	t.Parallel()
	bed := newAdvanceBed(t)
	bed = realStewardBedFrom(t, bed, []byte("engine @ "+enginebuild.StampRecord("0123456789abcdef0123456789abcdef01234567")))
	steps := bed.steps(t)
	steps.ReArm = ProductionSteps(bed.checkout, bed.installation).ReArm
	_, err := Advance(bed.request(t), ProductionConditions(bed.home, bed.checkout), steps)
	t.Logf("refusal: %v", refusalOf(t, err, "LANE_ENGINE_ADVANCE_RUNNER_DOWN"))
	enrolled, readErr := Enrollment(bed.installation)
	installed := bed.installed(t)
	if readErr != nil || enrolled.Generation != 4 || enrolled.InstallDigest != digestOf(installed) || enrolled.LandedCommit != bed.main ||
		!bytes.Contains(installed, []byte(enginebuild.StampRecord(bed.main))) {
		t.Fatalf("enrollment %+v %v over installed %q; want generation 4 naming landed main's build", enrolled, readErr, installed)
	}
	bed.nothingLeft(t)
}

func realStewardBedFrom(t *testing.T, bed *advanceBed, enrolled []byte) *advanceBed {
	t.Helper()
	gitIn(t, bed.installation, "config", "--local", "metasystem.steward.landing-ref", "refs/remotes/origin/main")
	bed.enrolled = enrolled
	if err := testexec.WriteFile(bed.installPath, enrolled, 0o755); err != nil {
		t.Fatal(err)
	}
	id := steward.InstallIdentity{RepoIdentity: bed.installation, Generation: 3, InstallPath: bed.installPath,
		InstallDigest: digestOf(enrolled), MintedAt: "2026-09-30T18:00:00Z", Enrollment: steward.EnrollmentFixture,
		MintedBy: "human-terminal", HumanWitnessedGeneration: 3, EngineBuild: "0123456789abcdef0123456789abcdef01234567"}
	if err := steward.MintIdentity(steward.RepoIdentityPath(bed.installation), id); err != nil {
		t.Fatal(err)
	}
	return bed
}

// N-1: the steward stopped the old runner and then failed before the mint:
// the old bytes go back, but no runner guards the lane, so the advance
// refuses with the person's system start rather than claiming all is well.
func TestAdvanceSaysTheRunnerIsDownWhenReArmFailsAfterStopping(t *testing.T) {
	t.Parallel()
	bed := newAdvanceBed(t)
	bed.enrollOld(t)
	steps := bed.steps(t)
	steps.ReArm = func(string) (steward.ReArmOutcome, error) {
		bed.rearms++
		return steward.ReArmOutcome{Stage: steward.StageStopped}, errors.New("publish the identity: disk full")
	}
	_, err := Advance(bed.request(t), ProductionConditions(bed.home, bed.checkout), steps)
	refusal := refusalOf(t, err, "LANE_ENGINE_ADVANCE_RUNNER_DOWN")
	if want := []string{"metasystem", "system", "start", "--repo", bed.checkout}; !slices.Equal(refusal.Argv, want) {
		t.Fatalf("argv %q, want %q", refusal.Argv, want)
	}
	if !bytes.Equal(bed.installed(t), bed.enrolled) {
		t.Fatalf("the enrolled bytes were not put back: %q", bed.installed(t))
	}
	bed.nothingLeft(t)
}
