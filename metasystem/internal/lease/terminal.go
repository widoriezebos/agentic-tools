package lease

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/fixtureauth"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
)

const (
	PersonTerminal  = "PERSON-TERMINAL"
	NotTerminal     = "NOT-TERMINAL"
	TerminalUnknown = "UNKNOWN"
)

type terminalObservations struct {
	humanauthority.Reader
	snapshots map[int64]humanauthority.Snapshot
}

func (r terminalObservations) Read(pid int64) (humanauthority.Snapshot, error) {
	snapshot, err := r.Reader.Read(pid)
	if err == nil {
		r.snapshots[pid] = snapshot
	}
	return snapshot, err
}

func terminalRefMatches(left, right humanauthority.ProcessRef) bool {
	if left.PID != right.PID {
		return false
	}
	if left.StartTicks > 0 && left.BootID != "" && right.StartTicks > 0 && right.BootID != "" {
		return left.StartTicks == right.StartTicks && left.BootID == right.BootID
	}
	return left.PIDStartedAt == right.PIDStartedAt
}

// PersonTerminalAt proves a terminal without granting a person-act authority.
// Only a complete enrollment walk with no recorded agent custody admits it.
func PersonTerminalAt(root, metasystemRoot string, target humanauthority.ProcessRef, reader humanauthority.Reader) string {
	if reader == nil {
		if fixtureauth.FixtureModeRoot(metasystemRoot) {
			return TerminalUnknown
		}
		reader = humanauthority.KernelReader{}
	}
	observations := terminalObservations{Reader: reader, snapshots: map[int64]humanauthority.Snapshot{}}
	proof, err := humanauthority.ProveTerminal(metasystemRoot, target.PID, observations, time.Time{})
	switch proof.Outcome {
	case humanauthority.OutcomeAgent, humanauthority.OutcomeTerminalMissing:
		return NotTerminal
	}
	if err != nil || proof.Outcome != humanauthority.OutcomeProven ||
		proof.InvokerRef.PIDStartedAt != target.PIDStartedAt || !terminalRefMatches(proof.InvokerRef, target) {
		return TerminalUnknown
	}
	// Glob-based readers cannot distinguish an absent store from a failed
	// enumeration. A hidden custody or announcement store proves nothing.
	for _, store := range []string{"mains", "jobs"} {
		if _, err := os.ReadDir(filepath.Join(root, "artifacts/agents", store)); err != nil && !os.IsNotExist(err) {
			return TerminalUnknown
		}
	}
	records, err := readAnnouncements(root, true)
	if err != nil {
		return TerminalUnknown
	}
	supervision, adapters, err := custodyIdentities(root)
	if err != nil {
		return TerminalUnknown
	}
	binaries := verifiedStewardBinaries(root)
	for _, node := range proof.Nodes {
		for _, record := range records {
			ann := record.Ann
			if terminalRefMatches(node.Ref, humanauthority.ProcessRef{PID: ann.Pid, PIDStartedAt: ann.PidStartedAt, StartTicks: ann.PidStartTicks, BootID: ann.BootID}) {
				return NotTerminal
			}
		}
		key := procKey{node.Ref.PID, node.Ref.PIDStartedAt}
		if supervision[key] || adapters[key] != "" || stewardPlumbing(strings.Join(observations.snapshots[node.Ref.PID].Exact.Argv, " "), binaries) {
			return NotTerminal
		}
	}
	before := observations.snapshots[target.PID]
	final, err := reader.Read(target.PID)
	if err != nil || final.Exact.Pid != before.Exact.Pid || !final.Exact.StartedAt.Equal(before.Exact.StartedAt) ||
		final.Exact.StartTicks != before.Exact.StartTicks || final.Exact.BootID != before.Exact.BootID ||
		final.Exact.ArgvKnown != before.Exact.ArgvKnown || !slices.Equal(final.Exact.Argv, before.Exact.Argv) ||
		final.ParentKnown != before.ParentKnown || final.ParentPID != before.ParentPID ||
		final.TerminalKnown != before.TerminalKnown || final.TerminalID != before.TerminalID ||
		final.OwnerKnown != before.OwnerKnown || final.OwnerUID != before.OwnerUID ||
		final.ExecutableKnown != before.ExecutableKnown || final.Executable != before.Executable {
		return TerminalUnknown
	}
	// The enrolled engine can run from a person's terminal without being
	// the terminal itself. Its executable identity survives the final read.
	// Lineage metadata does not establish an executable identity.
	for _, binary := range binaries {
		if before.ExecutableKnown && sameExecutable(before.Executable, binary) {
			return NotTerminal
		}
	}
	return PersonTerminal
}
