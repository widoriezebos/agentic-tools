package missionrunner

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/fixtureauth"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/janitor"
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
func processCommand(pid int, command fixtureauth.CommandProbe) string {
	exact, state, err := identity.KernelProber{}.Probe(int64(pid))
	if err == nil && state == identity.Alive && len(exact.Argv) > 0 {
		return strings.Join(exact.Argv, " ")
	}
	if err == nil && state == identity.Dead {
		// Kernel death VETOES the fixture: a
		// child that exited after publication must not recover its tag
		// from the just-written row.
		return ""
	}
	if fixtureCommand, ok := command.FixtureCommand(int64(pid)); ok {
		return fixtureCommand
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

// groupHasSubstantiveMember reports whether any member of the group is a
// live process with readable argv — the difference between running work
// and a zombie shell awaiting its parent's reap. A zombie-only group is
// finished work: SIGKILL can do no more to it, and holding the wind-down
// open for the parent's reaping debt reads as a leak that is not one
// (the VM sweep's Linux finding: zombies keep the pgid signalable).
func groupHasSubstantiveMember(pgid int) bool {
	pids, err := identity.AllPids()
	if err != nil {
		return true // unknown: stay conservative, keep waiting
	}
	for _, pid := range pids {
		memberGroup, err := unix.Getpgid(int(pid))
		if err != nil || memberGroup != pgid {
			continue
		}
		exact, state, err := identity.KernelProber{}.Probe(pid)
		if err == nil && state == identity.Alive && exact.ArgvKnown {
			return true
		}
	}
	return false
}

// groupOwnership is the runner's tri-state signal predicate, riding the
// janitor's positional shapes: a process that merely MENTIONS the tag
// anywhere in argv never proves ownership (the substring-kill hazard the
// shapes exist to close — this replaced a strings.Contains over the
// joined argv). INDETERMINATE stays distinguishable from NOT-OWNED so a
// wind-down that already holds proof can kill through a mid-death group
// whose argv went unreadable, while a provably foreign group is never
// signaled.
func groupOwnership(pgid int, tag string, grant fixtureauth.GroupOwnershipGrant) janitor.GroupOwnershipOutcome {
	outcome := janitor.GroupOwnership(int64(pgid), tag)
	if outcome == janitor.GroupOwned {
		return outcome
	}
	// The fake runtime's launcher records identities in the fixture
	// table; the grant is the root-checked authority for reading it.
	if fixturePgid, command, ok := grant.FixtureGroup(int64(pgid)); ok {
		if fixturePgid == int64(pgid) && strings.Contains(command, "fixture "+tag) {
			return janitor.GroupOwned
		}
	}
	return outcome
}

// publishFakeIdentity records a launched fake host's identity in the fixture
// identity file under its lock, so fixture liveness checks can see it.
func publishFakeIdentity(pid int, started int64, pgid int, tag string, grant fixtureauth.PublicationGrant) error {
	path, authorized := grant.TablePath()
	if !authorized {
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
