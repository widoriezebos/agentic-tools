package census

import (
	"fmt"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// The remaining small census verbs: alive (liveness), authentication-identity
// (start time + command from one source), and signature-check (adapter
// positive/lookalike contract).

// Alive is the `alive` verb: true iff the pid is live at expectedStart.
// The probe is the fixture authority (agnosticism B1) — nil refuses
// fixture identity and only the kernel answers.
func Alive(pid, expectedStart int64, probe identity.FixtureProbe) bool {
	return identityAlive(pid, expectedStart, probe)
}

// ProcIdentity is a live process's identity from the one authoritative
// source: its start second and command line. Command is never empty on
// success, so a caller's error check is its only absence check.
type ProcIdentity struct {
	Command      string `json:"command"`
	Pid          int64  `json:"pid"`
	PidStartedAt int64  `json:"pidStartedAt"`
}

// AuthIdentity returns a process's start time and command from ONE source —
// the fixture identity file when installed (its `started` and `command`), else
// the process table (start time + command). This one-source rule is why a main
// can recognize its own announcement.
func AuthIdentity(pid int64, probe identity.FixtureProbe) (ProcIdentity, error) {
	if entry, ok := probeFixture(probe, pid); ok && entry.HasStartedAt && entry.HasCommand && entry.Command != "" {
		return ProcIdentity{Pid: pid, PidStartedAt: entry.StartedAt, Command: entry.Command}, nil
	}
	return psIdentity(pid)
}

func psIdentity(pid int64) (ProcIdentity, error) {
	// Native: the kernel prober gives the start time (whole seconds) and argv.
	exact, state, err := identity.KernelProber{}.Probe(pid)
	if err != nil || state != identity.Alive {
		return ProcIdentity{}, fmt.Errorf("no such process: %d", pid)
	}
	command := strings.Join(exact.Argv, " ")
	if command == "" {
		return ProcIdentity{}, fmt.Errorf("unreadable command for pid %d", pid)
	}
	return ProcIdentity{Pid: pid, PidStartedAt: exact.StartedAt.Unix(), Command: command}, nil
}

// SignatureCheck is the `signature-check` verb: the positive argv must
// classify as the adapter's runtime and the lookalike must NOT — the
// adapters' self-test that their signatures are neither too loose nor too
// tight (KI-14). Returns an error when the contract fails.
func SignatureCheck(adapterPath, positive, lookalike string) error {
	text, err := SignatureText(adapterPath)
	if err != nil {
		return err
	}
	matches, excludes := ParseSignatureText(text)
	sig, err := CompileSignature("check", matches, excludes)
	if err != nil {
		return err
	}
	positiveOK := sig.matches(positive)
	lookalikeOK := sig.matches(lookalike)
	if !positiveOK || lookalikeOK {
		return fmt.Errorf("signature positive/lookalike contract failed for %s", adapterBase(adapterPath))
	}
	return nil
}

func adapterBase(path string) string {
	if i := strings.LastIndexByte(path, '/'); i >= 0 {
		return path[i+1:]
	}
	return path
}

// probeFixture is the nil-safe fixture read (a nil probe refuses).
func probeFixture(probe identity.FixtureProbe, pid int64) (identity.FixtureEntry, bool) {
	if probe == nil {
		return identity.FixtureEntry{}, false
	}
	return probe.FixtureEntry(pid)
}
