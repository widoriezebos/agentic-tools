package plain

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

const DrainDraining = "draining"
const DrainHeld = "held"

// DrainSource identifies the person's act or the helm signature that closed admission.
type DrainSource struct {
	Kind     string `json:"kind"`
	Checkout string `json:"checkout,omitempty"`
	By       string `json:"by,omitempty"`
	At       string `json:"at,omitempty"`
}

type Drain struct {
	By     string      `json:"by"`
	At     string      `json:"at"`
	Reason string      `json:"reason"`
	Source DrainSource `json:"source"`
	State  string      `json:"state"`
}

func DrainPath(install string) string { return filepath.Join(Dir(install), "drain.json") }

// ReadDrain takes no lock: home-locked admission must not acquire the queue lock.
func ReadDrain(install string) (*Drain, error) {
	data, err := os.ReadFile(DrainPath(install))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var drain Drain
	if err := json.Unmarshal(data, &drain); err != nil {
		return nil, err
	}
	if drain.By == "" || drain.At == "" || (drain.State != DrainDraining && drain.State != DrainHeld) {
		return nil, errors.New("the drain has no author, time or valid progress state")
	}
	return &drain, nil
}

func writeDrain(install string, drain Drain) error {
	data, err := json.Marshal(drain)
	if err != nil {
		return err
	}
	_, err = atomicfile.WriteFile(DrainPath(install), append(data, '\n'), 0o600, "")
	return err
}

// SetDrain preserves a standing act's attribution. An explicit act repairs unreadable bytes.
func SetDrain(install string, requested Drain) (drain Drain, changed bool, err error) {
	err = withLock(install, func() error {
		current, readErr := ReadDrain(install)
		if readErr == nil && current != nil {
			drain = *current
			return nil
		}
		requested.State = DrainDraining
		drain = requested
		if err := writeDrain(install, drain); err != nil {
			return err
		}
		changed = true
		return nil
	})
	return
}

func ClearDrain(install string) (changed bool, err error) {
	err = withLock(install, func() error {
		err := os.Remove(DrainPath(install))
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		changed = err == nil
		return err
	})
	return
}

// AdmissionClosed carries the fence's cause separately from queue write failures.
type AdmissionClosed struct {
	Drain      *Drain
	Unreadable error
}

func (e *AdmissionClosed) Error() string {
	if e.Unreadable != nil {
		return "admission is closed because the drain cannot be read: " + e.Unreadable.Error()
	}
	return "admission was closed by " + e.Drain.By
}

// queueMembership keeps valid work available while refusing to call damaged membership empty.
func queueMembership(install string) ([]Entry, error) {
	lines, skipped, err := countedLines[Line](queuePath(install))
	if err != nil {
		return nil, err
	}
	for _, line := range lines {
		if line.Goal == "" || line.SHA == "" || (line.Outcome != "" && line.Outcome != StateReturned && line.Outcome != StateWaiting) {
			skipped++
		}
	}
	if skipped > 0 {
		err = fmt.Errorf("%s: %d queue lines cannot be decoded as hand-ins; membership is unknown", queuePath(install), skipped)
	}
	return entriesOf(lines), err
}

type DrainProgress struct {
	Drain   *Drain `json:"drain,omitempty"`
	Waiting int    `json:"waiting"`
	Unknown string `json:"unknown,omitempty"`
}

// AdvanceDrain derives completion under the queue lock from one captured main tip.
// Unknown membership is data: known eligible work can still finish.
func AdvanceDrain(install, checkout string, seams ProveSeams) (progress DrainProgress, err error) {
	err = withLock(install, func() error {
		progress.Drain, err = ReadDrain(install)
		if err != nil {
			return fmt.Errorf("read drain %s: %w; repair that record and observe the lane again", DrainPath(install), err)
		}
		if progress.Drain == nil {
			return nil
		}
		entries, memberErr := queueMembership(install)
		main, mainErr := seams.git(checkout, "rev-parse", "--verify", "--quiet", "refs/remotes/origin/main^{commit}")
		if mainErr == nil && main != "" {
			var deriveErr error
			entries, deriveErr = Landed(entries, func(sha string) (bool, error) { return checkoutGit(checkout, seams).contains(main, sha) })
			memberErr = errors.Join(memberErr, deriveErr)
		} else {
			if mainErr == nil {
				mainErr = errors.New("the captured main tip is empty")
			}
			memberErr = errors.Join(memberErr, fmt.Errorf("main tip cannot be read: %w", mainErr))
		}
		for _, entry := range entries {
			if entry.State == StateWaiting {
				progress.Waiting++
			}
		}
		unfinished, executionErr := drainExecution(install, seams)
		unknown := errors.Join(memberErr, executionErr)
		if unknown != nil {
			progress.Unknown = "drain membership/progress cannot be read: " + unknown.Error()
			return nil
		}
		state := DrainDraining
		if progress.Waiting == 0 && !unfinished {
			state = DrainHeld
		}
		if progress.Drain.State == state {
			return nil
		}
		next := *progress.Drain
		next.State = state
		if err := writeDrain(install, next); err != nil {
			return fmt.Errorf("write drain progress %s: %w; repair that record's write access and observe the lane again", DrainPath(install), err)
		}
		progress.Drain = &next
		return nil
	})
	if err != nil {
		err = fmt.Errorf("advance drain %s: %w; repair the named source and observe the lane again", DrainPath(install), err)
	}
	return
}

// drainExecution treats undecidable process records as unknown progress, never finished work.
func drainExecution(install string, seams ProveSeams) (bool, error) {
	running, recorded, alive, proofErr := ReadRunning(install, seams)
	if recorded && (running.Attempt == "" || seams.Alive == nil && !knownDrainProcess(running)) {
		proofErr = errors.Join(proofErr, fmt.Errorf("%s: test run process is undecidable", runningPath(install)))
	}
	regeneration, regenErr := ReadRunningRegeneration(install, seams)
	if regeneration != nil && seams.Alive == nil && !knownDrainProcess(Running{Pid: regeneration.Pid, Process: regeneration.Process}) {
		regenErr = errors.Join(regenErr, fmt.Errorf("%s: regeneration process is undecidable", regenerationRunningPath(install)))
	}
	if proofErr != nil {
		proofErr = fmt.Errorf("read test run progress %s: %w", runningPath(install), proofErr)
	}
	if regenErr != nil {
		regenErr = fmt.Errorf("read regeneration progress %s: %w", regenerationRunningPath(install), regenErr)
	}
	return recorded && alive || regeneration != nil && regeneration.State == "running", errors.Join(proofErr, regenErr)
}

func knownDrainProcess(running Running) bool {
	_, err := identity.ParseRef(running.Process)
	return running.Pid > 0 && err == nil
}

// KeeperDrainHold rechecks admission without nesting the queue lock under the home lock.
func KeeperDrainHold(install string) (string, error) {
	drain, err := ReadDrain(install)
	if err != nil {
		return "", fmt.Errorf("read drain %s: %w; a person reopens admission with metasystem landing start", DrainPath(install), err)
	}
	if drain == nil {
		return "", nil
	}
	if drain.State == DrainHeld {
		return "the landing lane is drained and holding; a person reopens admission with metasystem landing start", nil
	}
	return "", nil
}

// proofAdmission allows admitted work and incident recovery, but suppresses idle timer proofs.
func proofAdmission(install, checkout, commit string, seams ProveSeams) error {
	drain, err := ReadDrain(install)
	if seams.Person || drain == nil && err == nil {
		return nil
	}
	if err != nil {
		return &AdmissionClosed{Unreadable: err}
	}
	entries, memberErr := queueMembership(install)
	for _, entry := range entries {
		if entry.State != StateWaiting {
			continue
		}
		if !seams.Trunk {
			inside, err := checkoutGit(checkout, seams).contains(commit, entry.SHA)
			if err != nil {
				return err
			}
			if inside {
				return nil
			}
		} else {
			incidents, err := seams.incidents(install, checkout, commit)
			if err != nil {
				return err
			}
			held := HoldEntries([]Entry{entry}, incidents)
			if len(held) > 0 && held[0].Held {
				return nil
			}
		}
	}
	if memberErr != nil {
		return memberErr
	}
	return &AdmissionClosed{Drain: drain}
}

func (p DrainProgress) Words() string {
	if p.Unknown != "" {
		return "admission unknown; " + p.Unknown
	}
	if p.Drain == nil {
		return "admission open"
	}
	if p.Drain.State == DrainHeld {
		return "drained and holding"
	}
	return fmt.Sprintf("draining, %d waiting", p.Waiting)
}
