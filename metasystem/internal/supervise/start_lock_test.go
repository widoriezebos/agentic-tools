package supervise

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)

// The child sees the public caller's argv in both os.Args and the kernel.
// Its working directory selects an isolated checkout without adding a test flag.
func heldStartLock() error {
	root, err := os.Getwd()
	if err != nil {
		return err
	}
	release, err := acquireCapAuthorityLock(root, 1)
	if err != nil {
		return err
	}
	ready := os.NewFile(3, "claimed")
	_, err = ready.WriteString("x")
	ready.Close()
	if err == nil {
		_, err = io.Copy(io.Discard, os.Stdin)
	}
	return errors.Join(err, release())
}

func TestStartLockRecognizesEachCaller(t *testing.T) {
	for _, verb := range []string{"system start", "system start --if-down", "session start", "internal hook claude start", "helm return", "up"} {
		t.Run(verb, func(t *testing.T) {
			root, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command(os.Args[0], strings.Fields(verb)...)
			cmd.Args[0] = filepath.Join(root, "bin", "metasystem")
			if verb == "session start" {
				cmd.Args = append(cmd.Args, "--repo", root)
			}
			cmd.Dir = root
			cmd.Env = append(armingHelpers.env(os.Environ()), "GO_WANT_HELD_START_LOCK=1")
			holder := testutil.StartHeldProcess(t, cmd)
			var owner struct {
				Pid int
				Tag string `json:"instanceTag"`
			}
			data, err := os.ReadFile(filepath.Join(SupervisionDir(root), "cap-authority.lock.d", "owner.json"))
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(data, &owner); err != nil || owner.Pid != cmd.Process.Pid || owner.Tag != strings.Join(cmd.Args, " ") {
				t.Fatalf("owner record does not name the claiming command: %v", err)
			}
			operation := verb
			if strings.HasPrefix(verb, "internal hook") {
				operation = "runtime session-start hook"
			}
			requireBusyStart(t, root, cmd.Process.Pid, operation)
			if verb == "system start --if-down" {
				// A crash leaves this new writer's actual record on disk.
				if err := holder.Kill(); holder.Command.ProcessState == nil || err == nil {
					t.Fatal("failed to kill and reap the holder")
				}
			} else if err := holder.Release(); err != nil {
				t.Fatalf("holder release: %v", err)
			}
			requireFreeStart(t, root)
		})
	}
}

// Both the kernel reader and the legacy JSON codec must be able to
// reproduce the tag, even for a Unix filename with non-text bytes.
func TestStartLockRecognizesUnixArguments(t *testing.T) {
	for _, suffix := range []string{"", "path-\xff"} {
		root, err := filepath.EvalSymlinks(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command(os.Args[0], "up", "--repo", root, "--tag", suffix)
		cmd.Args[0] = filepath.Join(root, "bin", "metasystem")
		cmd.Dir = root
		cmd.Env = append(armingHelpers.env(os.Environ()), "GO_WANT_HELD_START_LOCK=1")
		holder := testutil.StartHeldProcess(t, cmd)
		requireBusyStart(t, root, cmd.Process.Pid, "metasystem up")
		if err := holder.Release(); err != nil {
			t.Fatal(err)
		}
	}
}

func requireBusyStart(t *testing.T, root string, pid int, command string) {
	t.Helper()
	start := time.Unix(100, 0)
	now := start
	priorNow, priorSleep := armingNow, armingSleep
	armingNow = func() time.Time { return now }
	armingSleep = func(d time.Duration) { now = now.Add(d) }
	defer func() { armingNow, armingSleep = priorNow, priorSleep }()
	release, err := acquireCapAuthorityLock(root, 1)
	if err == nil {
		_ = release()
		t.Fatal("contender acquired a live holder's lock")
	}
	wants := []string{"remained busy", command, "wait"}
	if pid > 0 {
		wants = append(wants, fmt.Sprintf("process %d ", pid))
	}
	for _, want := range wants {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("busy refusal omitted %q", want)
		}
	}
	if now.Sub(start) != scaledWait(10, 1) {
		t.Errorf("contender waited %v", now.Sub(start))
	}
}

func requireFreeStart(t *testing.T, root string) {
	t.Helper()
	release, err := acquireCapAuthorityLock(root, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := release(); err != nil {
		t.Fatal(err)
	}
}

func TestStartLockPreservesExistingOwnerRecords(t *testing.T) {
	for _, tag := range []string{"metasystem up", "metasystem system start --if-down", "metasystem delegate"} {
		t.Run(tag, func(t *testing.T) {
			root, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			dir := filepath.Join(SupervisionDir(root), "cap-authority.lock.d")
			if err := os.MkdirAll(filepath.Dir(dir), 0755); err != nil {
				t.Fatal(err)
			}
			process := startOwnedHeldProcess(t, false, tag)
			pid := process.command.Process.Pid
			if err := dispatch.OwnerLockClaim(dir, int64(pid), tag); err != nil {
				t.Fatal(err)
			}
			requireBusyStart(t, root, pid, tag)
			if tag == "metasystem delegate" {
				if err := dispatch.OwnerLockRelease(dir, int64(pid), tag); err != nil {
					t.Fatal(err)
				}
				requireFreeStart(t, root)
				return
			}
			if err := errors.Join(process.release(), process.join()); err != nil {
				t.Fatal(err)
			}
			requireFreeStart(t, root)
		})
	}
}

func TestLegacyStartLockWithReusedPIDIsTakenOver(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(SupervisionDir(root), "cap-authority.lock.d")
	if err := os.MkdirAll(filepath.Dir(dir), 0755); err != nil {
		t.Fatal(err)
	}
	stranger := startOwnedHeldProcess(t, false, "another command")
	if err := dispatch.OwnerLockClaim(dir, int64(stranger.command.Process.Pid), "metasystem up"); err != nil {
		t.Fatal(err)
	}
	requireFreeStart(t, root)
}

func TestStartLockWithUnreadableRecordWaits(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(SupervisionDir(root), "cap-authority.lock.d")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "owner.json"), []byte("{"), 0600); err != nil {
		t.Fatal(err)
	}
	requireBusyStart(t, root, 0, "unreadable")
}
