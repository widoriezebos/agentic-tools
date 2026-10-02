package census

import (
	"fmt"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// The remaining small census verbs: alive (liveness), authentication-identity
// (start time + command from one source), and the signature contract (a
// runtime's positive/lookalike vectors).

// Alive is the `alive` verb: true iff the pid is live at expectedStart.
// The probe is the fixture authority — nil refuses fixture identity and
// only the kernel answers.
func Alive(pid, expectedStart int64, probe identity.FixtureProbe) bool {
	return identityAlive(pid, expectedStart, probe)
}

// ProcIdentity is a live process's identity from the one authoritative
// source: its start second and command line. Command is never empty on
// success, so a caller's error check is its only absence check.
type ProcIdentity struct {
	Command       string `json:"command"`
	Pid           int64  `json:"pid"`
	PidStartedAt  int64  `json:"pidStartedAt"`
	PidStartTicks int64
	BootID        string
}

// AuthIdentity returns a process's start time and command from ONE source —
// the fixture identity file when installed (its `started` and `command`), else
// the process table (start time + command). This one-source rule is why a main
// can recognize its own announcement.
func AuthIdentity(pid int64, probe identity.FixtureProbe) (ProcIdentity, error) {
	if entry, ok := authFixture(probe, pid); ok {
		return ProcIdentity{Pid: pid, PidStartedAt: entry.StartedAt, Command: entry.Command}, nil
	}
	return kernelIdentity(pid)
}

// AuthStartedAt is AuthIdentity's start second without its command: the
// same source decides (a fixture row AuthIdentity would answer from, else
// the kernel), but the kernel read is the start record alone. A process's
// start is readable before its new image has published argv (Linux, right
// after exec) and after argv is gone (an unreaped exit), so a caller that
// needs only the start must not fail on an unreadable command.
func AuthStartedAt(pid int64, probe identity.FixtureProbe) (int64, error) {
	if entry, ok := authFixture(probe, pid); ok {
		return entry.StartedAt, nil
	}
	exact, state, err := identity.KernelProber{}.ReadStart(pid)
	if err != nil || state != identity.Alive {
		return 0, fmt.Errorf("no such process: %d", pid)
	}
	return exact.StartedAt.Unix(), nil
}

// authFixture is the fixture row the authentication identity answers from:
// one with both a start and a non-empty command.
func authFixture(probe identity.FixtureProbe, pid int64) (identity.FixtureEntry, bool) {
	entry, ok := probeFixture(probe, pid)
	return entry, ok && entry.HasStartedAt && entry.HasCommand && entry.Command != ""
}

func kernelIdentity(pid int64) (ProcIdentity, error) {
	// The kernel prober reads natively — sysctl on darwin, /proc on
	// linux; no subprocess is involved.
	exact, state, err := identity.KernelProber{}.Probe(pid)
	if err != nil || state != identity.Alive {
		return ProcIdentity{}, fmt.Errorf("no such process: %d", pid)
	}
	command := strings.Join(exact.Argv, " ")
	if command == "" {
		return ProcIdentity{}, fmt.Errorf("unreadable command for pid %d", pid)
	}
	return ProcIdentity{Pid: pid, PidStartedAt: exact.StartedAt.Unix(), Command: command,
		PidStartTicks: exact.StartTicks, BootID: exact.BootID}, nil
}

// probeFixture is the nil-safe fixture read (a nil probe refuses).
func probeFixture(probe identity.FixtureProbe, pid int64) (identity.FixtureEntry, bool) {
	if probe == nil {
		return identity.FixtureEntry{}, false
	}
	return probe.FixtureEntry(pid)
}
