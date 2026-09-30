package gaterun

import (
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// makeFifo creates one named pipe under dir; the launched shells below use
// fifos as their only synchronization so no assertion depends on timing.
func makeFifo(t *testing.T, dir, name string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := syscall.Mkfifo(path, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// readFifo opens the fifo for reading (blocking until the writer opens it)
// and returns everything written before the writer closed it.
func readFifo(t *testing.T, path string) string {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	data, err := io.ReadAll(file)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// releaseFifo opens the fifo for writing (blocking until the reader opens it)
// and closes it at once, handing the reader EOF.
func releaseFifo(t *testing.T, path string) {
	t.Helper()
	file, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestLaunchDetachedRefusesAnIncompleteLaunch(t *testing.T) {
	t.Parallel()
	if pid, err := LaunchDetached(DetachedLaunch{}); err == nil || pid != 0 || !strings.Contains(err.Error(), "a command is required") {
		t.Fatalf("empty argv: pid=%d err=%v", pid, err)
	}
	for _, launch := range []DetachedLaunch{
		{Argv: []string{"/bin/true"}, GuardRoot: t.TempDir()},
		{Argv: []string{"/bin/true"}, GuardOwner: "dispatch"},
	} {
		if pid, err := LaunchDetached(launch); !errors.Is(err, ErrGuardPairIncomplete) || pid != 0 {
			t.Fatalf("guard pair %+v: pid=%d err=%v", launch, pid, err)
		}
	}
	dir := t.TempDir()
	if pid, err := LaunchDetached(DetachedLaunch{Argv: []string{"/bin/true"}, Log: filepath.Join(dir, "missing", "launch.log")}); err == nil || pid != 0 {
		t.Fatalf("unopenable log: pid=%d err=%v", pid, err)
	}
	if pid, err := LaunchDetached(DetachedLaunch{Argv: []string{filepath.Join(dir, "no-such-program")}}); err == nil || pid != 0 {
		t.Fatalf("unstartable program: pid=%d err=%v", pid, err)
	}
}

// A detached launch runs in its own session and process group (pid ==
// pgid == sid), in Dir, with Env added, stdin from /dev/null, and both output
// streams APPENDED to Log.
func TestLaunchDetachedStartsItsOwnSessionAndAppendsOutputToTheLog(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	work := t.TempDir()
	ready := makeFifo(t, dir, "ready")
	release := makeFifo(t, dir, "release")
	logPath := filepath.Join(dir, "launch.log")
	if err := os.WriteFile(logPath, []byte("earlier line\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	script := `read -r stdin_line || stdin_line="<eof>"
echo "out $LAUNCH_MARK $(pwd -P) $stdin_line"
echo "err line" >&2
printf ready > "$1"
read -r _ < "$2"
`
	pid, err := LaunchDetached(DetachedLaunch{
		Argv: []string{"/bin/sh", "-c", script, "launched", ready, release},
		Dir:  work,
		Log:  logPath,
		Env:  []string{"LAUNCH_MARK=marked"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if pid <= 0 {
		t.Fatalf("pid = %d", pid)
	}
	if got := readFifo(t, ready); got != "ready" {
		t.Fatalf("ready fifo = %q", got)
	}
	// The child is parked on the release fifo: its group and session are
	// its own.
	pgid, err := syscall.Getpgid(int(pid))
	if err != nil || int64(pgid) != pid {
		t.Fatalf("pgid = %d err=%v, want the pid %d", pgid, err, pid)
	}
	sid, err := unix.Getsid(int(pid))
	if err != nil || int64(sid) != pid {
		t.Fatalf("sid = %d err=%v, want the pid %d", sid, err, pid)
	}
	releaseFifo(t, release)

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	resolvedWork, err := filepath.EvalSymlinks(work)
	if err != nil {
		t.Fatal(err)
	}
	want := "earlier line\nout marked " + resolvedWork + " <eof>\nerr line\n"
	if string(data) != want {
		t.Fatalf("log = %q, want %q", data, want)
	}
}

// With a guard pair the started process joins the checkout execution guard
// held by its launcher, so the guard outlives the launcher's own membership.
func TestLaunchDetachedRegistersTheChildInTheExecutionGuard(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	dir := t.TempDir()
	self := int64(os.Getpid())
	if result, err := AcquireExecutionGuard(root, self, "suite", time.Second, time.Second, &bytes.Buffer{}); err != nil || result != GuardAcquired {
		t.Fatalf("acquire: result=%v err=%v", result, err)
	}
	release := makeFifo(t, dir, "release")
	pid, err := LaunchDetached(DetachedLaunch{
		Argv:       []string{"/bin/sh", "-c", `read -r _ < "$1"`, "member", release},
		GuardRoot:  root,
		GuardOwner: "dispatch supervisor",
	})
	if err != nil {
		t.Fatal(err)
	}
	defer releaseFifo(t, release)
	record, err := readExecutionGuardRecord(root)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, member := range record.Members {
		if member.Pid == pid && member.Owner == "dispatch supervisor" {
			found = true
		}
	}
	if !found {
		t.Fatalf("launched pid %d is not a guard member: %+v", pid, record.Members)
	}
	if err := ReleaseExecutionGuard(root, self); err != nil {
		t.Fatal(err)
	}
	record, err = readExecutionGuardRecord(root)
	if err != nil || len(record.Members) != 1 || record.Members[0].Pid != pid {
		t.Fatalf("the launched member must hold the guard alone: %+v err=%v", record.Members, err)
	}
	if err := ReleaseExecutionGuard(root, pid); err != nil {
		t.Fatal(err)
	}
	if _, err := readExecutionGuardRecord(root); !os.IsNotExist(err) {
		t.Fatalf("guard still held after the last member released: %v", err)
	}
}

// A launcher outside any held guard is refused: no pid is returned and no
// guard springs into existence.
func TestLaunchDetachedRefusesRegistrationOutsideAHeldGuard(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	dir := t.TempDir()
	parked := makeFifo(t, dir, "parked")
	pid, err := LaunchDetached(DetachedLaunch{
		Argv:       []string{"/bin/sh", "-c", `read -r _ < "$1"`, "unguarded", parked},
		GuardRoot:  root,
		GuardOwner: "dispatch supervisor",
	})
	if err == nil || pid != 0 || !strings.Contains(err.Error(), "descends from no live member") {
		t.Fatalf("pid=%d err=%v", pid, err)
	}
	if _, err := readExecutionGuardRecord(root); !os.IsNotExist(err) {
		t.Fatalf("a refused registration created a guard: %v", err)
	}
}

func TestRunGuardMemberRequiresRootAndCommand(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		root string
		argv []string
	}{{"", []string{"/bin/true"}}, {t.TempDir(), nil}} {
		var stdout, stderr bytes.Buffer
		if status := RunGuardMember(tc.root, tc.argv, &stdout, &stderr); status != 2 {
			t.Fatalf("status = %d", status)
		}
		if !strings.Contains(stderr.String(), "root and command are required") || stdout.Len() != 0 {
			t.Fatalf("stdout=%q stderr=%q", stdout.String(), stderr.String())
		}
	}
}

// The wrapper passes the child's output and exit status through and releases
// its own membership when the child exits, freeing a guard it held alone.
func TestRunGuardMemberReturnsTheChildStatusAndReleasesTheGuard(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		script string
		status int
		stdout string
		stderr string
	}{
		{"success", `echo hello`, 0, "hello\n", ""},
		{"exit code", `echo out; echo err >&2; exit 3`, 3, "out\n", "err\n"},
		{"signalled", `kill -TERM $$`, 128 + int(syscall.SIGTERM), "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			self := int64(os.Getpid())
			if result, err := AcquireExecutionGuard(root, self, "suite", time.Second, time.Second, &bytes.Buffer{}); err != nil || result != GuardAcquired {
				t.Fatalf("acquire: result=%v err=%v", result, err)
			}
			var stdout, stderr bytes.Buffer
			status := RunGuardMember(root, []string{"/bin/sh", "-c", tc.script}, &stdout, &stderr)
			if status != tc.status || stdout.String() != tc.stdout || stderr.String() != tc.stderr {
				t.Fatalf("status=%d stdout=%q stderr=%q", status, stdout.String(), stderr.String())
			}
			if _, err := readExecutionGuardRecord(root); !os.IsNotExist(err) {
				t.Fatalf("the wrapper's membership outlived its child: %v", err)
			}
		})
	}
}

func TestRunGuardMemberReportsAnUnstartableCommandAndReleases(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	self := int64(os.Getpid())
	if result, err := AcquireExecutionGuard(root, self, "suite", time.Second, time.Second, &bytes.Buffer{}); err != nil || result != GuardAcquired {
		t.Fatalf("acquire: result=%v err=%v", result, err)
	}
	missing := filepath.Join(t.TempDir(), "no-such-program")
	var stdout, stderr bytes.Buffer
	if status := RunGuardMember(root, []string{missing}, &stdout, &stderr); status != 127 {
		t.Fatalf("status = %d", status)
	}
	if !strings.Contains(stderr.String(), missing) {
		t.Fatalf("stderr = %q", stderr.String())
	}
	if _, err := readExecutionGuardRecord(root); !os.IsNotExist(err) {
		t.Fatalf("an unstartable command kept the guard: %v", err)
	}
}

func TestExitStatusMapsWaitErrors(t *testing.T) {
	t.Parallel()
	if got := exitStatus(nil); got != 0 {
		t.Fatalf("nil = %d", got)
	}
	if got := exitStatus(errors.New("wait failed")); got != 1 {
		t.Fatalf("non-exit error = %d", got)
	}
	err := exec.Command("/bin/sh", "-c", "exit 7").Run()
	if got := exitStatus(err); got != 7 {
		t.Fatalf("exit 7 = %d (%v)", got, err)
	}
	err = exec.Command("/bin/sh", "-c", "kill -KILL $$").Run()
	if got := exitStatus(err); got != 128+int(syscall.SIGKILL) {
		t.Fatalf("killed = %d (%v)", got, err)
	}
}
