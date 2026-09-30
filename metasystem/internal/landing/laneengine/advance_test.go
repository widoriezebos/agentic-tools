package laneengine

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/enginebuild"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
)

// advanceBed is a real lane: a bare file:// origin, a nested lane checkout
// cloned from it (checkout root != installation root), a host home and an
// enrollment of the engine built from the landed main.
type advanceBed struct {
	*enrollBed
	home, origin, main string
	enrolled           []byte
	builds, rearms     int
	// stamp is what the fake build links; empty links main.
	stamp    string
	rearmErr error
}

func gitIn(t *testing.T, dir string, args ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=lane", "-c", "user.email=lane@example.invalid", "-c", "commit.gpgsign=false"}, args...)...)
	out, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

func newAdvanceBed(t *testing.T) *advanceBed {
	t.Helper()
	bed := &advanceBed{enrollBed: newEnrollBed(t)}
	base := filepath.Dir(bed.checkout)
	bed.home, bed.origin = filepath.Join(base, "home"), filepath.Join(base, "origin.git")
	if err := os.MkdirAll(bed.home, 0o700); err != nil {
		t.Fatal(err)
	}
	gitIn(t, base, "init", "--quiet", "--bare", "-b", "main", bed.origin)
	gitIn(t, bed.checkout, "init", "--quiet", "-b", "main")
	if err := os.WriteFile(filepath.Join(bed.installation, "metasystem.conf"), []byte("metasystem.template=true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bed.checkout, ".gitignore"), []byte("metasystem/bin/\nmetasystem/artifacts/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitIn(t, bed.checkout, "add", "-A")
	gitIn(t, bed.checkout, "commit", "--quiet", "-m", "landed")
	gitIn(t, bed.checkout, "remote", "add", "origin", "file://"+bed.origin)
	gitIn(t, bed.checkout, "push", "--quiet", "origin", "main")
	bed.main = gitIn(t, bed.checkout, "rev-parse", "HEAD")
	return bed
}

// enrollOld enrolls an engine built from an older landed commit.
func (bed *advanceBed) enrollOld(t *testing.T) {
	t.Helper()
	bed.enrolled = []byte("engine @ " + enginebuild.StampRecord("0123456789abcdef0123456789abcdef01234567"))
	bed.enroll(t, digestOf(bed.enrolled), "0123456789abcdef0123456789abcdef01234567", bed.enrolled)
}

func (bed *advanceBed) request(t *testing.T) AdvanceRequest {
	t.Helper()
	enrolled, err := steward.VerifyIdentity(steward.RepoIdentityPath(bed.installation), bed.installation)
	if err != nil {
		t.Fatal(err)
	}
	return AdvanceRequest{Home: bed.home, Checkout: bed.checkout, Installation: bed.installation,
		Identity: Identity{Installation: bed.installation, Enrolled: enrolled, Running: enrolled.InstallDigest}}
}

// steps are the production Git steps with the build and the steward re-arm
// recorded: a build links the stamp the bed names; a re-arm sees the
// installed bytes.
func (bed *advanceBed) steps(t *testing.T) Steps {
	t.Helper()
	steps := ProductionSteps(bed.checkout, bed.installation)
	steps.Build = func(staging string) error {
		bed.builds++
		stamp := bed.stamp
		if stamp == "" {
			stamp = bed.main
		}
		return os.WriteFile(staging, []byte("engine @ "+enginebuild.StampRecord(stamp)), 0o755)
	}
	steps.ReArm = func(installPath string) (steward.ReArmOutcome, error) {
		bed.rearms++
		if installPath != bed.installPath {
			t.Errorf("re-armed %s, want the enrolled path %s", installPath, bed.installPath)
		}
		if bed.rearmErr != nil {
			return steward.ReArmOutcome{}, bed.rearmErr
		}
		// The steward mints the next generation of the bytes now installed.
		installed := bed.installed(t)
		id := steward.InstallIdentity{RepoIdentity: bed.installation, Generation: 4, InstallPath: bed.installPath,
			InstallDigest: digestOf(installed), MintedAt: "2026-09-30T18:05:00Z", Enrollment: steward.EnrollmentHumanTerminal,
			MintedBy: "machine-rebuild", EngineBuild: bed.main, LandedCommit: bed.main}
		if err := steward.MintIdentity(steward.RepoIdentityPath(bed.installation), id); err != nil {
			return steward.ReArmOutcome{}, err
		}
		return steward.ReArmOutcome{Status: "re-armed", Generation: 4, PreviousGeneration: 3, EngineBuild: bed.main}, nil
	}
	return steps
}

func (bed *advanceBed) advance(t *testing.T) (AdvanceOutcome, error) {
	t.Helper()
	return Advance(bed.request(t), ProductionConditions(bed.home, bed.checkout), bed.steps(t))
}

func (bed *advanceBed) installed(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile(bed.installPath)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// nothingLeft proves an advance left no staged or saved engine beside the
// enrolled one.
func (bed *advanceBed) nothingLeft(t *testing.T) {
	t.Helper()
	entries, err := os.ReadDir(filepath.Dir(bed.installPath))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		var names []string
		for _, entry := range entries {
			names = append(names, entry.Name())
		}
		t.Fatalf("bin holds %q; want only metasystem", names)
	}
}

func (bed *advanceBed) untouched(t *testing.T, err error, code string) {
	t.Helper()
	refusalOf(t, err, code)
	if bed.builds != 0 || bed.rearms != 0 || !bytes.Equal(bed.installed(t), bed.enrolled) {
		t.Fatalf("a refused advance built %d, re-armed %d or changed the engine", bed.builds, bed.rearms)
	}
	bed.nothingLeft(t)
}

// The engine moves only to landed main: the fetched origin main is built,
// installed at the enrolled path, and the steward re-enrolls it.
func TestAdvanceInstallsTheLandedMainBuildAndReArms(t *testing.T) {
	t.Parallel()
	bed := newAdvanceBed(t)
	bed.enrollOld(t)
	outcome, err := bed.advance(t)
	if err != nil || !outcome.Changed || outcome.Commit != bed.main || outcome.Generation != 4 {
		t.Fatalf("advance = %+v %v", outcome, err)
	}
	if bed.builds != 1 || bed.rearms != 1 {
		t.Fatalf("built %d, re-armed %d; want once each", bed.builds, bed.rearms)
	}
	if !bytes.Contains(bed.installed(t), []byte(enginebuild.StampRecord(bed.main))) {
		t.Fatalf("installed engine %q is not the landed main build", bed.installed(t))
	}
	bed.nothingLeft(t)
}

// A repeat whose effect holds is success: the second advance, admitted by
// the engine the first one enrolled, builds and re-arms nothing.
func TestAdvanceTwiceBuildsOnce(t *testing.T) {
	t.Parallel()
	bed := newAdvanceBed(t)
	bed.enrollOld(t)
	if outcome, err := bed.advance(t); err != nil || !outcome.Changed {
		t.Fatalf("first advance = %+v %v", outcome, err)
	}
	outcome, err := bed.advance(t)
	if err != nil || outcome.Changed || outcome.Generation != 4 || bed.builds != 1 || bed.rearms != 1 {
		t.Fatalf("repeated advance = %+v %v, built %d re-armed %d; want unchanged after one build", outcome, err, bed.builds, bed.rearms)
	}
}

// An engine already built from landed main, with its enrolled bytes in
// place, is unchanged: no build, no re-arm.
func TestAdvanceOnLandedMainChangesNothing(t *testing.T) {
	t.Parallel()
	bed := newAdvanceBed(t)
	bed.enrolled = []byte("engine @ " + enginebuild.StampRecord(bed.main))
	bed.enroll(t, digestOf(bed.enrolled), bed.main, bed.enrolled)
	outcome, err := bed.advance(t)
	if err != nil || outcome.Changed || bed.builds != 0 || bed.rearms != 0 {
		t.Fatalf("advance on landed main = %+v %v, built %d re-armed %d", outcome, err, bed.builds, bed.rearms)
	}
}

// A checkout whose HEAD is not the fetched origin main is refused before
// anything is built; line 2 checks out the landed commit by its SHA.
func TestAdvanceBuildsOnlyLandedMain(t *testing.T) {
	t.Parallel()
	bed := newAdvanceBed(t)
	bed.enrollOld(t)
	if err := os.WriteFile(filepath.Join(bed.checkout, "unlanded.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitIn(t, bed.checkout, "add", "unlanded.txt")
	gitIn(t, bed.checkout, "commit", "--quiet", "-m", "not landed")
	_, err := bed.advance(t)
	bed.untouched(t, err, CodeAdvanceNotLanded)
	var refusal *Refusal
	errors.As(err, &refusal)
	if want := []string{"git", "-C", bed.checkout, "checkout", "--detach", bed.main}; !slices.Equal(refusal.Argv, want) {
		t.Fatalf("argv %q, want %q", refusal.Argv, want)
	}
}

// A build whose stamp is not the landed commit (engine changes in the
// checkout that main does not have) is refused and never installed.
func TestAdvanceRefusesABuildNotStampedWithLandedMain(t *testing.T) {
	t.Parallel()
	bed := newAdvanceBed(t)
	bed.enrollOld(t)
	bed.stamp = enginebuild.DevelopmentStamp(bed.main[:12])
	_, err := bed.advance(t)
	refusalOf(t, err, CodeAdvanceNotLanded)
	if bed.builds != 1 || bed.rearms != 0 || !bytes.Equal(bed.installed(t), bed.enrolled) {
		t.Fatalf("a dev build was installed or re-armed (built %d re-armed %d)", bed.builds, bed.rearms)
	}
	bed.nothingLeft(t)
}

// A re-arm that fails puts the enrolled bytes back: the lane is never left
// with an installed engine its enrollment does not name.
func TestAdvanceRestoresTheEnrolledEngineWhenReArmFails(t *testing.T) {
	t.Parallel()
	bed := newAdvanceBed(t)
	bed.enrollOld(t)
	bed.rearmErr = errors.New("the enrolled engine changed or is missing: not landed")
	if _, err := bed.advance(t); err == nil {
		t.Fatal("a failed re-arm reported success")
	}
	if bed.rearms != 1 || !bytes.Equal(bed.installed(t), bed.enrolled) {
		t.Fatalf("after a failed re-arm the engine is %q; want the enrolled bytes back", bed.installed(t))
	}
	bed.nothingLeft(t)
}

// Only between batches: a begun batch that has not finished holds the
// engine, while a batch still collecting (nothing begun) does not.
func TestAdvanceOnlyBetweenBatches(t *testing.T) {
	t.Parallel()
	bed := newAdvanceBed(t)
	bed.enrollOld(t)
	store := batch.NewStore(bed.checkout, identity.KernelProber{})
	if err := store.Create(batch.Record{Schema: 1, BatchID: "01k6d2m3n4p5q6r7s8t9v0w1x2", State: batch.StateProving}); err != nil {
		t.Fatal(err)
	}
	_, err := bed.advance(t)
	bed.untouched(t, err, CodeAdvanceBatchInFlight)
	if !strings.Contains(err.Error(), "01k6d2m3n4p5q6r7s8t9v0w1x2") {
		t.Fatalf("refusal %v does not name the batch", err)
	}
	if err := store.Update("01k6d2m3n4p5q6r7s8t9v0w1x2", func(record *batch.Record) error { record.State = batch.StateLanded; return nil }); err != nil {
		t.Fatal(err)
	}
	if err := store.Create(batch.Record{Schema: 1, BatchID: "01k6d2m3n4p5q6r7s8t9v0w1x3", State: batch.StateOpen}); err != nil {
		t.Fatal(err)
	}
	if outcome, err := bed.advance(t); err != nil || !outcome.Changed {
		t.Fatalf("advance with only a landed and a collecting batch = %+v %v", outcome, err)
	}
}

// An unreadable batch record is not "between batches".
func TestAdvanceRefusesUnreadableBatches(t *testing.T) {
	t.Parallel()
	bed := newAdvanceBed(t)
	bed.enrollOld(t)
	dir := filepath.Join(bed.checkout, "artifacts", "agents", "landing-batches")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "01k6d2m3n4p5q6r7s8t9v0w1x2.json"), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := bed.advance(t)
	bed.untouched(t, err, CodeAdvanceBatchInFlight)
}

// The pause holds (K2): a paused lane, and a pause that cannot be read,
// refuse the advance.
func TestAdvanceHoldsWhilePaused(t *testing.T) {
	t.Parallel()
	bed := newAdvanceBed(t)
	bed.enrollOld(t)
	if _, err := lane.SetPause(bed.home, "Wido", fixedNow); err != nil {
		t.Fatal(err)
	}
	_, err := bed.advance(t)
	bed.untouched(t, err, CodeAdvancePaused)
	if _, err := lane.ClearPause(bed.home); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(lane.HostDir(bed.home), "landing-lane-paused.json"), 0o700); err != nil {
		t.Fatal(err)
	}
	_, err = bed.advance(t)
	bed.untouched(t, err, CodeAdvancePaused)
}

// Custody must be settled (K9): while a proof holds the host's proving
// flock the engine does not move.
func TestAdvanceWaitsForProofCustody(t *testing.T) {
	t.Parallel()
	bed := newAdvanceBed(t)
	bed.enrollOld(t)
	release, err := lane.HoldProving(bed.home)
	if err != nil {
		t.Fatal(err)
	}
	_, err = bed.advance(t)
	bed.untouched(t, err, CodeAdvanceCustodyLive)
	if err := release(); err != nil {
		t.Fatal(err)
	}
	if outcome, err := bed.advance(t); err != nil || !outcome.Changed {
		t.Fatalf("advance after custody settled = %+v %v", outcome, err)
	}
}
