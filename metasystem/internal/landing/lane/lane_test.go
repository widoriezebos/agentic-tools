package lane

import (
	"bufio"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var laneNow = time.Date(2026, 9, 29, 18, 0, 0, 0, time.UTC)

func laneDirs(t *testing.T) (home, first, second string) {
	t.Helper()
	base := t.TempDir()
	home, first, second = filepath.Join(base, "home"), filepath.Join(base, "landing-a"), filepath.Join(base, "landing-b")
	for _, dir := range []string{home, first, second} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return home, first, second
}

// Two seats whose settings name the same checkout: the first registers it,
// the second uses the same record, and neither writes a second one.
func TestResolveRegistersFirstSeatAndUsesItAfter(t *testing.T) {
	t.Parallel()
	home, root, _ := laneDirs(t)
	first, err := Resolve(home, root, "m1e", laneNow, true)
	if err != nil || first.Root != resolved(root) || !first.Registered {
		t.Fatalf("first seat = %+v, %v; want %s registered", first, err, root)
	}
	info, err := os.Stat(RecordPath(home))
	if err != nil {
		t.Fatal(err)
	}
	second, err := Resolve(home, root+string(filepath.Separator), "ui", laneNow.Add(time.Hour), true)
	if err != nil || second.Root != resolved(root) || second.Registered {
		t.Fatalf("second seat = %+v, %v; want the same lane, not re-registered", second, err)
	}
	record, ok, err := Read(home)
	if err != nil || !ok || record.RegisteredBy != "m1e" || record.At != laneNow.Format(time.RFC3339) {
		t.Fatalf("record = %+v %v %v; want m1e's registration kept", record, ok, err)
	}
	after, _ := os.Stat(RecordPath(home))
	if !after.ModTime().Equal(info.ModTime()) {
		t.Fatalf("the second seat rewrote the record")
	}
}

// A seat whose setting names another checkout is refused in plain words
// naming both paths and both ways out; the record is unchanged.
func TestResolveRefusesAnotherSeatRoot(t *testing.T) {
	t.Parallel()
	home, first, second := laneDirs(t)
	if _, err := Resolve(home, first, "m1e", laneNow, true); err != nil {
		t.Fatal(err)
	}
	_, err := Resolve(home, second, "ui", laneNow, true)
	var refusal *Refusal
	if !errors.As(err, &refusal) || refusal.Code != CodeMismatch {
		t.Fatalf("err = %v; want %s", err, CodeMismatch)
	}
	text := err.Error() + " " + refusal.Fix
	for _, want := range []string{resolved(first), resolved(second), "registered by m1e", "metasystem settings set landing.batch-root " + resolved(first), "metasystem landing set " + resolved(second)} {
		if !strings.Contains(text, want) {
			t.Errorf("refusal %q lacks %q", text, want)
		}
	}
	record, _, _ := Read(home)
	if record.Root != resolved(first) {
		t.Fatalf("record moved to %s", record.Root)
	}
}

// A seat without a setting (the UI seat) uses the host's lane.
func TestResolveUnsetSeatUsesHostRecord(t *testing.T) {
	t.Parallel()
	home, root, _ := laneDirs(t)
	none, err := Resolve(home, "", "ui", laneNow, true)
	if err != nil || none.Root != "" {
		t.Fatalf("nothing registered = %+v, %v; want no lane", none, err)
	}
	if _, err := Resolve(home, root, "m1e", laneNow, true); err != nil {
		t.Fatal(err)
	}
	got, err := Resolve(home, "", "ui", laneNow, true)
	if err != nil || got.Root != resolved(root) || !got.FromHost {
		t.Fatalf("unset seat = %+v, %v; want the host's lane", got, err)
	}
}

// A registered checkout that is gone is reported, never replaced by a seat's
// other setting.
func TestResolveReportsGoneRoot(t *testing.T) {
	t.Parallel()
	home, first, second := laneDirs(t)
	if _, err := Resolve(home, first, "m1e", laneNow, true); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(first); err != nil {
		t.Fatal(err)
	}
	for _, seat := range []string{"", first, second} {
		_, err := Resolve(home, seat, "ui", laneNow, true)
		var refusal *Refusal
		if !errors.As(err, &refusal) || refusal.Code != CodeGone || !strings.Contains(err.Error(), first) || !strings.Contains(refusal.Fix, "metasystem landing set PATH") {
			t.Fatalf("seat %q: err = %v; want %s naming %s", seat, err, CodeGone, first)
		}
	}
	record, _, _ := Read(home)
	if record.Root != resolved(first) {
		t.Fatalf("a gone lane was replaced by %s", record.Root)
	}
}

// Register moves the lane on a person's word, is unchanged on a repeat, and
// refuses a path that is no directory.
func TestRegisterMovesUnchangedAndRefuses(t *testing.T) {
	t.Parallel()
	home, first, second := laneDirs(t)
	if _, changed, err := Register(home, first, "Wido", laneNow); err != nil || !changed {
		t.Fatalf("first register = %v, %v", changed, err)
	}
	if _, changed, err := Register(home, first, "Wido", laneNow.Add(time.Minute)); err != nil || changed {
		t.Fatalf("repeat = %v, %v; want unchanged", changed, err)
	}
	previous, changed, err := Register(home, second, "Wido", laneNow.Add(time.Hour))
	if err != nil || !changed || previous.Root != resolved(first) {
		t.Fatalf("move = %+v %v %v", previous, changed, err)
	}
	for _, bad := range []string{"relative/path", filepath.Join(second, "missing")} {
		var refusal *Refusal
		if _, _, err := Register(home, bad, "Wido", laneNow); !errors.As(err, &refusal) || refusal.Code != CodeRegisterInvalid {
			t.Fatalf("register %q = %v; want %s", bad, err, CodeRegisterInvalid)
		}
	}
}

// One batch proves at a time on the host: a proof child holds the proving
// flock for its life; a probe sees it busy and names the holder, and takes
// nothing itself.
func TestProvingLockIsHeldByTheProofAndProbedByTheOwner(t *testing.T) {
	t.Parallel()
	home, _, _ := laneDirs(t)
	if holder, busy, err := ProbeProving(home); err != nil || busy || holder != "" {
		t.Fatalf("free lock probe = %q %v %v", holder, busy, err)
	}
	if holder, busy, err := ProbeProving(home); err != nil || busy {
		t.Fatalf("a probe kept the lock: %q %v %v", holder, busy, err)
	}
	release, err := HoldProving(home)
	if err != nil {
		t.Fatal(err)
	}
	if holder, busy, err := ProbeProving(home); err != nil || !busy || holder != "pid "+itoa(os.Getpid()) {
		t.Fatalf("held lock probe = %q %v %v; want busy naming this pid", holder, busy, err)
	}
	if err := release(); err != nil {
		t.Fatal(err)
	}
	if _, busy, _ := ProbeProving(home); busy {
		t.Fatalf("the lock stayed held after its release")
	}
}

func startProvingHolder(t *testing.T, home string) *exec.Cmd {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	child := exec.Command(executable)
	child.Env = append(os.Environ(), provingHolderEnv+"="+home)
	stdout, err := child.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	line, err := bufio.NewReader(stdout).ReadString('\n')
	if err != nil || strings.TrimSpace(line) != "held" {
		_ = child.Process.Kill()
		_ = child.Wait()
		t.Fatalf("holder said %q, %v", line, err)
	}
	return child
}

// A proof child killed mid-proof releases the lock: the kernel drops the
// flock with the process.
func TestProvingLockReleasedWhenHolderDies(t *testing.T) {
	t.Parallel()
	home, _, _ := laneDirs(t)
	child := startProvingHolder(t, home)
	pid := child.Process.Pid
	if holder, busy, err := ProbeProving(home); err != nil || !busy || holder != "pid "+itoa(pid) {
		_ = child.Process.Kill()
		_ = child.Wait()
		t.Fatalf("while held: %q %v %v; want busy naming pid %d", holder, busy, err, pid)
	}
	if err := child.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = child.Wait()
	if _, busy, err := ProbeProving(home); err != nil || busy {
		t.Fatalf("after the holder died: busy=%v %v; want the lock free", busy, err)
	}
}

// F-1: the lock belongs to the proof, not to its lane's owner: pausing the
// lane and moving it elsewhere leave the running proof holding it, and the
// new lane can prove the moment that proof ends.
func TestProvingLockOutlivesAPauseAndAMoveUntilTheProofEnds(t *testing.T) {
	t.Parallel()
	home, first, second := laneDirs(t)
	if _, _, err := Register(home, first, "Wido", laneNow); err != nil {
		t.Fatal(err)
	}
	child := startProvingHolder(t, home)
	if _, err := SetPause(home, "Wido", laneNow); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Register(home, second, "Wido", laneNow); err != nil {
		t.Fatal(err)
	}
	if _, busy, _ := ProbeProving(home); !busy {
		_ = child.Process.Kill()
		_ = child.Wait()
		t.Fatalf("the moved lane's proof lost the lock while it runs")
	}
	if err := child.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = child.Wait()
	if _, busy, _ := ProbeProving(home); busy {
		t.Fatalf("the new lane cannot prove after the old proof ended")
	}
}
