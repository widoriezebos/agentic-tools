package missionrunner

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// Process identity for the runner's liveness and ownership decisions. All
// answers come from kernel facts (start times, argv, process groups); the
// fake-identity file is a fixture seam that stands in for processes the test
// harness simulates rather than runs.

// processStartedAt reads a pid's kernel start time in epoch seconds. A pid
// whose identity cannot be read is a pid the runner must not reason about.
func processStartedAt(pid int) (int64, error) {
	exact, state, err := identity.KernelProber{}.Probe(int64(pid))
	if err != nil || state != identity.Alive {
		return 0, failf(3, "cannot resolve process start identity for pid %d", pid)
	}
	return exact.StartedAt.Unix(), nil
}

// processCommand reads a live pid's command line, falling back to the
// fixture identity file when the caller allows fakes. An unreadable command
// is empty, never a guess.
func processCommand(pid int, allowFake bool) string {
	exact, state, err := identity.KernelProber{}.Probe(int64(pid))
	if err == nil && state == identity.Alive && len(exact.Argv) > 0 {
		return strings.Join(exact.Argv, " ")
	}
	if allowFake {
		if entry := fakeIdentity(pid); entry != nil {
			if command, ok := entry["command"].(string); ok {
				return command
			}
		}
	}
	return ""
}

// pidExists reports whether a pid exists; a permission denial proves
// existence, which a shell kill -0 cannot distinguish from no-such-process.
func pidExists(pid int) bool {
	if pid < 1 {
		return false
	}
	switch unix.Kill(pid, 0) {
	case nil, unix.EPERM:
		return true
	default:
		return false
	}
}

// groupAlive reports whether a process group exists, with the same
// permission-denial-proves-existence rule.
func groupAlive(pgid int) bool {
	if pgid < 1 {
		return false
	}
	switch unix.Kill(-pgid, 0) {
	case nil, unix.EPERM:
		return true
	default:
		return false
	}
}

// groupOwned reports whether a process group is provably ours: some live
// member's command line carries the instance tag this runner minted. Without
// that proof the group must never be signaled.
func groupOwned(pgid int, tag string, allowFake bool) bool {
	if pids, err := identity.AllPids(); err == nil {
		for _, pid := range pids {
			memberGroup, err := unix.Getpgid(int(pid))
			if err != nil || memberGroup != pgid {
				continue
			}
			exact, state, err := identity.KernelProber{}.Probe(pid)
			if err != nil || state != identity.Alive {
				continue
			}
			if strings.Contains(strings.Join(exact.Argv, " "), tag) {
				return true
			}
		}
	}
	if allowFake {
		entry := fakeIdentity(pgid)
		if entry == nil {
			return false
		}
		recorded, ok := jsonInt(entry["pgid"])
		return ok && recorded == int64(pgid) && strings.Contains(valueString(entry["command"]), tag)
	}
	return false
}

// fakeIdentity reads one pid's entry from the fixture identity file, when the
// harness provides one. Any read problem means no identity.
func fakeIdentity(pid int) map[string]any {
	path := os.Getenv("METASYSTEM_FAKE_PROCESS_IDENTITY_FILE")
	if path == "" {
		return nil
	}
	doc, err := readJSONDoc(path)
	if err != nil {
		return nil
	}
	entry, _ := doc[strconv.Itoa(pid)].(map[string]any)
	return entry
}

// publishFakeIdentity records a launched fake host's identity in the fixture
// identity file under its lock, so fixture liveness checks can see it.
func publishFakeIdentity(pid int, started int64, pgid int, tag string) error {
	path := os.Getenv("METASYSTEM_FAKE_PROCESS_IDENTITY_FILE")
	if path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	lock, err := os.OpenFile(path+".lock", os.O_CREATE|os.O_RDWR|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer lock.Close()
	if err := unix.Flock(int(lock.Fd()), unix.LOCK_EX); err != nil {
		return err
	}
	doc, err := readJSONDoc(path)
	if err != nil {
		doc = map[string]any{}
	}
	doc[strconv.Itoa(pid)] = map[string]any{
		"pidStartedAt": started,
		"pgid":         pgid,
		"command":      "fixture " + tag,
	}
	return atomicWriteJSON(path, doc)
}
