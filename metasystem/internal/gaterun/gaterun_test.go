package gaterun

import (
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestRegisterThenRunningIsTrueForLiveGate(t *testing.T) {
	root := t.TempDir()
	path, err := Register(root, int64(os.Getpid()), "go-gate")
	if err != nil {
		t.Fatal(err)
	}
	if path == "" {
		t.Fatal("registering a live process should write a marker")
	}
	if !Running(root) {
		t.Fatal("a live gate marker should read as running")
	}
}

func TestRunningPrunesDeadMarker(t *testing.T) {
	root := t.TempDir()
	// A dead pid: spawn, reap, then hand-write a marker for it.
	cmd := exec.Command("/bin/sleep", "1")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	deadPid := cmd.Process.Pid
	_ = cmd.Process.Kill()
	_ = cmd.Wait()

	dir := markerDir(root)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(dir, "dead.json")
	if err := os.WriteFile(marker, []byte(`{"pid":`+itoa(deadPid)+`,"pidStartedAt":111}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if Running(root) {
		t.Fatal("a dead gate marker must not read as running")
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("a dead gate marker should be pruned")
	}
}

func TestRunningDropsUnparsableMarker(t *testing.T) {
	root := t.TempDir()
	dir := markerDir(root)
	_ = os.MkdirAll(dir, 0o755)
	marker := filepath.Join(dir, "junk.json")
	_ = os.WriteFile(marker, []byte(`{not json`), 0o644)
	if Running(root) {
		t.Fatal("an unparsable marker is not a running gate")
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("an unparsable marker should be pruned")
	}
}

func TestRegisterSkipsDeadProcess(t *testing.T) {
	root := t.TempDir()
	cmd := exec.Command("/bin/sleep", "1")
	_ = cmd.Start()
	deadPid := int64(cmd.Process.Pid)
	_ = cmd.Process.Kill()
	_ = cmd.Wait()

	// Nonempty-or-error (goal-system GOAL-17): an unverifiable identity is
	// an error the caller must surface, never a silent success.
	path, err := Register(root, deadPid, "go-gate")
	if err == nil {
		t.Fatal("registering a dead process reported success")
	}
	if path != "" {
		t.Fatal("a dead process has no verifiable start, so no marker should be written")
	}
	if Running(root) {
		t.Fatal("nothing was registered, so nothing runs")
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// GOAL-17's marker edges: a live process's unparsable marker SURFACES
// instead of vanishing; a live well-formed marker is a fact; Register is
// atomic (no partial file is ever visible beside the final name).
func TestGateMarkerEdges(t *testing.T) {
	root := t.TempDir()

	// Our own pid: a well-formed live marker registers and surveys live.
	path, err := Register(root, int64(os.Getpid()), "edge-fixture")
	if err != nil || path == "" {
		t.Fatalf("register self: %v %q", err, path)
	}
	survey := Survey(root)
	if len(survey.Live) != 1 || survey.Live[0].Gate != "edge-fixture" || len(survey.Unreadable) != 0 {
		t.Fatalf("live marker not surveyed: %+v", survey)
	}

	// An unparsable marker NAMED FOR a live pid (ours) surfaces — never
	// deleted, never believed.
	garbled := filepath.Join(markerDir(root), itoa(os.Getpid())+".json")
	_ = os.WriteFile(garbled, []byte("{half a rec"), 0o644)
	survey = Survey(root)
	if len(survey.Unreadable) != 1 {
		t.Fatalf("live-process unparsable marker did not surface: %+v", survey)
	}
	if _, err := os.Stat(garbled); err != nil {
		t.Fatal("live-process unparsable marker was deleted")
	}

	// No temp droppings from atomic registration.
	leftovers, _ := filepath.Glob(filepath.Join(markerDir(root), ".gate-*"))
	if len(leftovers) != 0 {
		t.Fatalf("atomic register leaked temp files: %v", leftovers)
	}
}

// Survey's remaining classifications: a well-formed dead marker is
// pruned; an identity mismatch (pid reused at a different start) reads
// dead and is pruned; a non-pid filename was never a Register-written
// marker and is pruned regardless.
func TestSurveyPrunesProvablyDead(t *testing.T) {
	root := t.TempDir()
	dir := markerDir(root)
	_ = os.MkdirAll(dir, 0o755)

	dead := exec.Command("/bin/sleep", "1")
	_ = dead.Start()
	deadPid := dead.Process.Pid
	_ = dead.Process.Kill()
	_, _ = dead.Process.Wait()

	deadMarker := filepath.Join(dir, itoa(deadPid)+".json")
	_ = os.WriteFile(deadMarker, []byte(`{"pid":`+itoa(deadPid)+`,"pidStartedAt":123,"gate":"x"}`+"\n"), 0o644)
	mismatch := filepath.Join(dir, itoa(os.Getpid())+".json")
	_ = os.WriteFile(mismatch, []byte(`{"pid":`+itoa(os.Getpid())+`,"pidStartedAt":1,"gate":"y"}`+"\n"), 0o644)
	nonPid := filepath.Join(dir, "not-a-pid.json")
	_ = os.WriteFile(nonPid, []byte("junk\n"), 0o644)

	survey := Survey(root)
	if len(survey.Live) != 0 || len(survey.Unreadable) != 0 {
		t.Fatalf("dead/mismatch/non-pid markers classified wrong: %+v", survey)
	}
	for _, path := range []string{deadMarker, mismatch, nonPid} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("provably-dead marker survived: %s", path)
		}
	}
}

// B2's same-user invariant at this package's edge: another user's LIVE
// process (pid 1, init) with its true recorded start must never be
// misread as dead — the existence probe answers EPERM, which proves
// life, and the marker stays.
func TestForeignLiveProcessNeverReadsDead(t *testing.T) {
	exact, state, err := identity.KernelProber{}.Probe(1)
	if err != nil || state != identity.Alive {
		t.Skip("pid 1 identity unreadable on this platform; the EPERM leg needs it")
	}
	root := t.TempDir()
	dir := markerDir(root)
	_ = os.MkdirAll(dir, 0o755)
	marker := filepath.Join(dir, "1.json")
	body := `{"pid":1,"pidStartedAt":` + itoa(int(exact.StartedAt.Unix())) + `,"gate":"foreign"}` + "\n"
	_ = os.WriteFile(marker, []byte(body), 0o644)

	survey := Survey(root)
	if len(survey.Live) != 1 || survey.Live[0].Pid != 1 {
		t.Fatalf("a foreign live process was not read alive: %+v", survey)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatal("a foreign live process's marker was deleted")
	}
}

// Register surfaces filesystem failures instead of recording nothing
// silently: an unwritable marker directory is an error.
func TestRegisterSurfacesWriteFailure(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	root := t.TempDir()
	dir := markerDir(root)
	_ = os.MkdirAll(dir, 0o555)
	defer os.Chmod(dir, 0o755)
	if _, err := Register(root, int64(os.Getpid()), "unwritable"); err == nil {
		t.Fatal("an unwritable marker dir registered silently")
	}
}

// Enumeration failure surfaces instead of collapsing to idle: a root whose
// path breaks the glob pattern reports itself unreadable.
func TestSurveySurfacesEnumerationFailure(t *testing.T) {
	root := filepath.Join(t.TempDir(), "br[oken")
	_ = os.MkdirAll(markerDir(root), 0o755)
	survey := Survey(root)
	if len(survey.Unreadable) != 1 {
		t.Fatalf("glob failure did not surface: %+v", survey)
	}
}
