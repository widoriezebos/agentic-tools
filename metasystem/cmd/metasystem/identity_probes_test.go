package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/janitor"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)

type processRefProber struct {
	exact identity.Exact
	state identity.Liveness
	err   error
}

func (p processRefProber) Probe(int64) (identity.Exact, identity.Liveness, error) {
	return p.exact, p.state, p.err
}

func TestProcessRefPrintsOneExactEncodedLineOrNothing(t *testing.T) {
	exact := identity.Exact{Pid: 42, StartedAt: time.UnixMicro(123_456_789)}
	if runtime.GOOS == "linux" {
		exact.StartTicks, exact.BootID = 73, "fixture-boot"
	}
	if !exact.Ref().NativeExact() {
		t.Skip("process refs require a supported native exact identity")
	}
	var output bytes.Buffer
	code := runIdentityRefWithProber([]string{"--pid", "42"}, processRefProber{exact: exact, state: identity.Alive}, &output)
	parsed, err := identity.ParseRef(strings.TrimSpace(output.String()))
	if code != 0 || err != nil || parsed != exact.Ref() || strings.Count(output.String(), "\n") != 1 {
		t.Fatalf("proc ref exit=%d output=%q parsed=%+v err=%v", code, output.String(), parsed, err)
	}
	output.Reset()
	code = runIdentityRefWithProber([]string{"--pid", "42"}, processRefProber{state: identity.Unknown, err: errors.New("denied")}, &output)
	if code == 0 || output.Len() != 0 {
		t.Fatalf("unprobeable proc ref exit=%d output=%q", code, output.String())
	}
}

func TestProcFixtureKeyPrintsAnEncodedKey(t *testing.T) {
	exact, state, err := (identity.KernelProber{}).Probe(int64(os.Getpid()))
	if err != nil || state != identity.Alive {
		t.Fatalf("probe fixture-key owner: state=%s err=%v", state, err)
	}
	owner, err := identity.EncodeRef(exact.Ref())
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	code := runFixtureKeyWithReader([]string{"--owner", owner, "--test", t.Name()}, strings.NewReader("\x01\x23\x45\x67"), &output)
	key, parseErr := identity.ParseKey(strings.TrimSpace(output.String()))
	if code != 0 || parseErr != nil || key.Owner != exact.Ref() || key.Test != t.Name() || key.Nonce != "01234567" {
		t.Fatalf("proc fixture-key exit=%d output=%q key=%+v err=%v", code, output.String(), key, parseErr)
	}
}

func TestProcessProbeReportsTerminalAndSessionLeader(t *testing.T) {
	output, code := captureStdout(t, func() int {
		return runIdentityProbe([]string{"--pid", fmt.Sprint(os.Getpid())})
	})
	var observed struct {
		Liveness           string `json:"liveness"`
		TerminalKnown      bool   `json:"terminalKnown"`
		TerminalID         string `json:"terminalId"`
		SessionLeaderPID   int64  `json:"sessionLeaderPid"`
		SessionLeaderError string `json:"sessionLeaderError"`
	}
	if err := json.Unmarshal([]byte(output), &observed); err != nil || code != 0 ||
		observed.Liveness != "alive" || !observed.TerminalKnown || observed.SessionLeaderPID < 1 || observed.SessionLeaderError != "" {
		t.Fatalf("process probe did not report its live terminal session: code=%d output=%s error=%v", code, output, err)
	}
}

func absentProcessGroup(t *testing.T) int64 {
	t.Helper()
	for pgid := int64(999999); pgid < 1009999; pgid++ {
		if errors.Is(unix.Kill(int(-pgid), 0), unix.ESRCH) {
			return pgid
		}
	}
	t.Fatal("could not find an absent process group for the empty-scan test")
	return 0
}

func startTestProcessGroup(t *testing.T, command *exec.Cmd) int64 {
	t.Helper()
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = command.Process.Kill()
		_, _ = command.Process.Wait()
	})
	return int64(command.Process.Pid)
}

func assertGroupOwnership(t *testing.T, pgid int64, tag string, want janitor.GroupOwnershipOutcome) {
	t.Helper()
	if got := janitor.GroupOwnership(pgid, tag); got != want {
		t.Fatalf("process group %d ownership outcome = %s, want %s", pgid, got, want)
	}
}

func TestGroupOwnedEmptyScanExitsIndeterminate(t *testing.T) {
	pgid := absentProcessGroup(t)
	if code := runIdentityGroupOwned([]string{"--pgid", fmt.Sprint(pgid), "--tag", "metasystem-job-empty-scan"}); code != 3 {
		t.Fatalf("empty group scan exit=%d, want 3 (INDETERMINATE), never 1 (NOT-OWNED)", code)
	}
}

func TestGroupOwnedLiveNonOwnerExitsNotOwned(t *testing.T) {
	if _, err := identity.AllPids(); err != nil {
		t.Skipf("process enumeration is unavailable: %v", err)
	}
	tag := fmt.Sprintf("metasystem-job-not-owned-%d", os.Getpid())
	command := exec.Command("/bin/sh", "-c", "printf x >&3; IFS= read -r _ || :")
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	held := testutil.StartHeldProcess(t, command)
	pgid := int64(held.Command.Process.Pid)
	assertGroupOwnership(t, pgid, tag, janitor.GroupNotOwned)
	if code := runIdentityGroupOwned([]string{"--pgid", fmt.Sprint(pgid), "--tag", tag}); code != 1 {
		t.Fatalf("not-owned live group scan exit=%d, want 1 (NOT-OWNED)", code)
	}
}

func TestGroupOwnedRecordedProofMismatchExitsIndeterminate(t *testing.T) {
	tag := fmt.Sprintf("metasystem-job-record-mismatch-%d", os.Getpid())
	pgid := startTestProcessGroup(t, exec.Command("sh", "-c", "exit 0"))
	if err := waitExitedWithoutReaping(int(pgid)); err != nil {
		t.Fatalf("wait for zombie-backed process group: %v", err)
	}
	assertGroupOwnership(t, pgid, tag, janitor.GroupIndeterminate)
	if err := unix.Kill(int(-pgid), 0); err != nil && err != unix.EPERM {
		t.Fatalf("zombie-backed process group %d is not signalable: %v", pgid, err)
	}

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.runtimes=fake\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	record := filepath.Join(root, "mismatched-record.json")
	if err := os.WriteFile(record, []byte(fmt.Sprintf(
		`{"runtime":"fake","instanceTag":"different-tag","pgid":%d}`, pgid)), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("METASYSTEM_FAKE_PROCESS_IDENTITY_FILE", "")
	if code := runIdentityGroupOwned([]string{
		"--pgid", fmt.Sprint(pgid), "--tag", tag, "--root", root, "--record", record,
	}); code != 3 {
		t.Fatalf("recorded-proof mismatch exit=%d, want 3 (INDETERMINATE)", code)
	}
}

func TestProcSetsidRefusesWithoutACommandAndAnAbsentOne(t *testing.T) {
	// The exec path replaces the test process and is proven by the goal-cli
	// bed's proof-grades scenario; only the refusals are checked here.
	if got := runProcSetsid([]string{"--"}); got != 2 {
		t.Fatalf("proc setsid without a command exit = %d, want 2", got)
	}
	if got := runProcSetsid([]string{"--", "/nonexistent/metasystem-no-such-command"}); got != 127 {
		t.Fatalf("proc setsid with an absent command exit = %d, want 127", got)
	}
}
