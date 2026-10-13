package humanauthority

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"
)

func sharedEnrollmentRoot(t *testing.T) string {
	t.Helper()
	root := authorityRoot(t)
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestEnrollmentReadOnlyOutsideInstall(t *testing.T) {
	t.Parallel()
	if os.Getenv("METASYSTEM_TEST_ENROLLMENT_READ") == "1" {
		enrollment, err := ReadEnrollment(os.Getenv("METASYSTEM_TEST_ENROLLMENT_ROOT"))
		if err != nil || enrollment.Human != "Wido" || enrollment.Generation != 1 {
			t.Fatalf("read machine enrollment without an install: %+v %v", enrollment, err)
		}
		return
	}
	if testenv.IsolateMachine(t) {
		return
	}
	enrollTestTerminal(t, sharedEnrollmentRoot(t), enrolledReader())
	for _, shape := range []string{"empty", "missing-layout", "relative"} {
		cwd := t.TempDir()
		root := ""
		switch shape {
		case "missing-layout":
			root = cwd
		case "relative":
			root = "."
		}
		// Even a current directory with an install marker must not become
		// the cache destination when the caller supplied no resolved root.
		if shape != "missing-layout" {
			if err := os.WriteFile(filepath.Join(cwd, "metasystem.conf"), nil, 0600); err != nil {
				t.Fatal(err)
			}
		}
		before, err := os.ReadDir(cwd)
		if err != nil {
			t.Fatal(err)
		}
		command := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestEnrollmentReadOnlyOutsideInstall$", "-test.count=1", "-test.timeout=30m")
		command.Dir = cwd
		command.Env = append(os.Environ(), "METASYSTEM_TEST_ENROLLMENT_READ=1", "METASYSTEM_TEST_ENROLLMENT_ROOT="+root)
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("%s: enrollment read: %v\n%s", shape, err, output)
		}
		after, err := os.ReadDir(cwd)
		if err != nil || len(after) != len(before) {
			t.Fatalf("%s: read changed the current directory: before=%v after=%v err=%v", shape, before, after, err)
		}
		if _, err := os.Stat(enrollmentPath(cwd)); !os.IsNotExist(err) {
			t.Fatalf("%s: read created an install cache: %v", shape, err)
		}
	}
}

func TestEnrollmentSharedAcrossInstalls(t *testing.T) {
	t.Parallel()
	if testenv.IsolateMachine(t) {
		return
	}
	seat, lane := sharedEnrollmentRoot(t), sharedEnrollmentRoot(t)
	reader := enrolledReader()
	enrollTestTerminal(t, seat, reader)
	proof, err := Prove(lane, 30, reader, time.Unix(1400, 0))
	if err != nil || !proof.EnrolledTerminalFor(lane) {
		t.Fatalf("one enrollment must authorize the other install: %+v %v", proof, err)
	}
	seatCache, err := os.ReadFile(enrollmentPath(seat))
	if err != nil {
		t.Fatal(err)
	}
	laneCache, err := os.ReadFile(enrollmentPath(lane))
	if err != nil || string(seatCache) != string(laneCache) {
		t.Fatalf("the second install did not refresh its cache: %v", err)
	}
	if err := os.WriteFile(enrollmentPath(lane), []byte("stale cache"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadEnrollment(lane); err != nil {
		t.Fatalf("a stale cache defeated machine authority: %v", err)
	}
	refreshed, _ := os.ReadFile(enrollmentPath(lane))
	if string(refreshed) != string(seatCache) {
		t.Fatal("a stale cache was not replaced")
	}
	if repeated, err := Enroll(lane, 20, reader, "Wido", time.Unix(1420, 0)); err != nil || !repeated.Repeat || repeated.Generation != 1 {
		t.Fatalf("another install advanced an unchanged enrollment: %+v %v", repeated, err)
	}
	updated, err := Enroll(lane, 20, reader, "Another person", time.Unix(1450, 0))
	if err != nil || updated.Generation != 2 {
		t.Fatalf("machine enrollment did not advance: %+v %v", updated, err)
	}
	if current, err := ReadEnrollment(seat); err != nil || current.Human != "Another person" || current.Generation != 2 {
		t.Fatalf("the first install retained the old person: %+v %v", current, err)
	}
	agent := agentShellReader()
	for _, root := range []string{seat, lane} {
		p, err := Prove(root, 80, agent, time.Unix(1500, 0))
		if err == nil || p.ValidFor(root) || p.Outcome != OutcomeAgent {
			t.Fatalf("agent inherited the person's machine enrollment at %s: %v", root, err)
		}
	}
	if _, err := os.Stat(filepath.Join(lane, "artifacts", "agents", "authority", "proofs")); !os.IsNotExist(err) {
		t.Fatal("reading enrollment copied another install's custody")
	}
}

func TestEnrollmentCacheFailureLeavesMachineProof(t *testing.T) {
	t.Parallel()
	if testenv.IsolateMachine(t) {
		return
	}
	root := sharedEnrollmentRoot(t)
	if err := os.MkdirAll(enrollmentPath(root), 0700); err != nil {
		t.Fatal(err)
	}
	reader := enrolledReader()
	enrollment, err := Enroll(root, 20, reader, "Wido", time.Unix(1400, 0))
	if err == nil || !strings.Contains(err.Error(), "machine enrollment stands") || enrollment.Generation != 1 {
		t.Fatalf("cache failure concealed the published machine proof: %+v %v", enrollment, err)
	}
	other := sharedEnrollmentRoot(t)
	if proof, err := Prove(other, 30, reader, time.Unix(1500, 0)); err != nil || !proof.EnrolledTerminalFor(other) {
		t.Fatalf("a broken install cache damaged machine authority: %+v %v", proof, err)
	}
	if _, err := ReadEnrollment(root); err == nil {
		t.Fatal("an unrefreshable cache silently claimed to be refreshed")
	}
}

func TestEnrollmentBrokenMachineProofRefusesStaleCache(t *testing.T) {
	t.Parallel()
	if testenv.IsolateMachine(t) {
		return
	}
	root := sharedEnrollmentRoot(t)
	reader := enrolledReader()
	enrollTestTerminal(t, root, reader)
	path, err := machineEnrollmentPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{broken proof"), 0600); err != nil {
		t.Fatal(err)
	}
	proof, err := Prove(root, 30, reader, time.Unix(1500, 0))
	if err == nil || proof.ValidFor(root) || proof.Outcome != OutcomeNotEnrolled {
		t.Fatalf("a stale install cache hid a broken machine proof: %+v %v", proof, err)
	}
}

func TestEnrollmentPromotesLegacyProofOnEnroll(t *testing.T) {
	t.Parallel()
	if testenv.IsolateMachine(t) {
		return
	}
	root := sharedEnrollmentRoot(t)
	reader := enrolledReader()
	enrollTestTerminal(t, root, reader)
	if updated, err := Enroll(root, 20, reader, "Another person", time.Unix(1400, 0)); err != nil || updated.Generation != 2 {
		t.Fatalf("advance the recorded install generation: %+v %v", updated, err)
	}
	path, err := machineEnrollmentPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	recorded, err := Enroll(root, 20, reader, "Another person", time.Unix(1600, 0))
	if err != nil || recorded.Generation != 2 || recorded.Repeat {
		t.Fatalf("legacy promotion lost its generation or skipped publication: %+v %v", recorded, err)
	}
	other := sharedEnrollmentRoot(t)
	if proof, err := Prove(other, 30, reader, time.Unix(1700, 0)); err != nil || !proof.ValidFor(other) {
		t.Fatalf("the promoted proof did not reach another install: %+v %v", proof, err)
	}
}

func TestEnrollmentPromotesLegacyProofOnPersonAct(t *testing.T) {
	t.Parallel()
	if testenv.IsolateMachine(t) {
		return
	}
	root := sharedEnrollmentRoot(t)
	reader := enrolledReader()
	enrollTestTerminal(t, root, reader)
	if updated, err := Enroll(root, 20, reader, "Another person", time.Unix(1400, 0)); err != nil || updated.Generation != 2 {
		t.Fatalf("advance the recorded install generation: %+v %v", updated, err)
	}
	path, err := machineEnrollmentPath()
	if err != nil {
		t.Fatal(err)
	}
	legacy, err := os.ReadFile(enrollmentPath(root))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	proof, err := Prove(root, 30, reader, time.Unix(1500, 0))
	if err != nil || !proof.EnrolledTerminalFor(root) || proof.TerminalGeneration != 2 {
		t.Fatalf("legacy enrollment did not authorize the person without re-enrollment: %+v %v", proof, err)
	}
	promoted, err := os.ReadFile(path)
	if err != nil || string(promoted) != string(legacy) {
		t.Fatalf("promotion did not preserve the exact legacy proof: %q %v", promoted, err)
	}
	if repeated, err := Enroll(root, 20, reader, "Another person", time.Unix(1600, 0)); err != nil || !repeated.Repeat || repeated.Generation != 2 {
		t.Fatalf("the promoted terminal was re-enrolled: %+v %v", repeated, err)
	}
	other := sharedEnrollmentRoot(t)
	if proof, err := Prove(other, 30, reader, time.Unix(1700, 0)); err != nil || !proof.EnrolledTerminalFor(other) {
		t.Fatalf("promotion did not authorize another install: %+v %v", proof, err)
	}
}

func TestEnrollmentRefusesInvalidLegacyProof(t *testing.T) {
	t.Parallel()
	if testenv.IsolateMachine(t) {
		return
	}
	root := sharedEnrollmentRoot(t)
	reader := enrolledReader()
	enrollTestTerminal(t, root, reader)
	path, err := machineEnrollmentPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(enrollmentPath(root), []byte("{broken legacy proof"), 0600); err != nil {
		t.Fatal(err)
	}
	proof, err := Prove(root, 30, reader, time.Unix(1500, 0))
	if err == nil || proof.ValidFor(root) || proof.Outcome != OutcomeNotEnrolled {
		t.Fatalf("invalid legacy proof authorized the person: %+v %v", proof, err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("invalid legacy proof was published to the machine: %v", err)
	}
}
