package goal

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

// BoundedCapture is one isolated canonical-tip fetch. OperationID names the
// temporary ref that the caller removes after validation and acceptance.
type BoundedCapture struct {
	Tip         string
	OperationID string
}

// LedgerChange is one consecutive first-parent ledger state. Consecutive is
// false when history cannot prove intermediate canonical states and the
// caller must treat the target as one direct accepted-world transition.
type LedgerChange struct {
	Tip         string
	Consecutive bool
}

const boundedCaptureGrace = 5 * time.Second

// AcceptedLedgerTip distinguishes an absent pre-bootstrap world from a broken
// accepted ref and from a readable migrated ledger.
func AcceptedLedgerTip(root string) (tip string, exists bool, err error) {
	tip, resolved, err := acceptedTipForGates(root)
	if err != nil || !resolved {
		return tip, false, err
	}
	hasLedger, err := tipCarriesLedger(Endpoint{Root: root}, tip)
	if err != nil {
		return "", false, err
	}
	if !hasLedger {
		return tip, false, nil
	}
	return tip, true, nil
}

// ProjectAt reads one already-identified commit without moving any ref.
func ProjectAt(root, tip string) (Projection, error) {
	tree, err := loadTree(root, tip)
	if err != nil {
		return Projection{}, err
	}
	return Projection{Tip: tip, Tree: tree}, nil
}

// LedgerChanges returns the proven consecutive first-parent states from
// before to after. A merge that reaches before through another parent, or a
// sanctioned accepted-ref rewind, is represented as one direct transition;
// inventing intermediate canonical states would surface changes that may
// never have occupied the shared branch.
func LedgerChanges(root, before, after string) ([]LedgerChange, error) {
	if before == after {
		return nil, nil
	}
	out, err := goalGit(root, nil, "rev-list", "--reverse", "--first-parent", before+".."+after)
	if err != nil {
		return nil, fmt.Errorf("walk accepted ledger changes: %w", err)
	}
	var tips []string
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if tip := strings.TrimSpace(line); tip != "" {
			tips = append(tips, tip)
		}
	}
	if len(tips) == 0 {
		return []LedgerChange{{Tip: after, Consecutive: false}}, nil
	}
	parent, parentErr := goalGit(root, nil, "rev-parse", "--verify", tips[0]+"^1")
	if parentErr != nil {
		return nil, fmt.Errorf("read first parent of accepted ledger change: %w", parentErr)
	}
	if strings.TrimSpace(parent) != before {
		return []LedgerChange{{Tip: after, Consecutive: false}}, nil
	}
	changes := make([]LedgerChange, 0, len(tips))
	for _, tip := range tips {
		changes = append(changes, LedgerChange{Tip: tip, Consecutive: true})
	}
	return changes, nil
}

// IsAncestor reports whether ancestor is equal to or precedes descendant.
func IsAncestor(root, ancestor, descendant string) (bool, error) {
	if ancestor == descendant {
		return true, nil
	}
	_, err := goalGit(root, nil, "merge-base", "--is-ancestor", ancestor, descendant)
	if err == nil {
		return true, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
		return false, nil
	}
	return false, err
}

// CaptureTipBounded fetches the canonical branch on an owned process group.
// A deadline kills the whole group before returning, so a transport helper
// cannot outlive the steward pass or advance anything beyond its temporary
// ref.
func CaptureTipBounded(e Endpoint, budget time.Duration) (BoundedCapture, error) {
	if budget <= 0 {
		return BoundedCapture{}, fmt.Errorf("goal ledger fetch budget must be positive")
	}
	ulid, err := NewOperationULID()
	if err != nil {
		return BoundedCapture{}, err
	}
	opid := "read-" + ulid
	fail := func(cause error) (BoundedCapture, error) {
		CleanupRefs(e, opid)
		return BoundedCapture{}, cause
	}
	if e.LocalMode() {
		tip, err := CaptureTip(e, opid)
		if err != nil {
			return fail(err)
		}
		return BoundedCapture{Tip: tip, OperationID: opid}, nil
	}

	args := []string{
		"-C", e.Root, "-c", "core.logAllRefUpdates=false",
		"fetch", "--no-tags", "--refmap=", e.Remote,
		"+" + e.Branch + ":" + fetchRefFor(opid),
	}
	cmd := exec.Command("git", args...)
	cmd.Env = environWithoutGitSteering()
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return fail(fmt.Errorf("start goal ledger fetch: %w", err))
	}
	waited := make(chan error, 1)
	go func() { waited <- cmd.Wait() }()
	timer := time.NewTimer(budget)
	defer timer.Stop()
	select {
	case waitErr := <-waited:
		if waitErr != nil {
			return fail(fmt.Errorf("git fetch --no-tags --refmap=: %v (%s)", waitErr, strings.TrimSpace(stderr.String())))
		}
	case <-timer.C:
		pgid := cmd.Process.Pid
		termErr := syscall.Kill(-pgid, syscall.SIGTERM)
		grace := time.NewTimer(boundedCaptureGrace)
		poll := time.NewTicker(10 * time.Millisecond)
		defer grace.Stop()
		defer poll.Stop()
		waitDone := false
		var waitErr error
		for {
			groupGone, probeErr := captureProcessGroupGone(pgid)
			if probeErr != nil {
				killErr := syscall.Kill(-pgid, syscall.SIGKILL)
				if !waitDone {
					waitErr = <-waited
				}
				return fail(fmt.Errorf("goal ledger fetch timed out and its process group could not be inspected: %v (TERM: %v, KILL: %v, wait: %v)", probeErr, termErr, killErr, waitErr))
			}
			if waitDone && groupGone {
				return fail(fmt.Errorf("goal ledger fetch timed out after %s", budget))
			}
			select {
			case waitErr = <-waited:
				waitDone = true
			case <-poll.C:
				// Recheck the process-group condition above; cooperative Git
				// transports return without spending the grace ceiling.
			case <-grace.C:
				killErr := syscall.Kill(-pgid, syscall.SIGKILL)
				if !waitDone {
					waitErr = <-waited
				}
				if killErr != nil && killErr != syscall.ESRCH {
					return fail(fmt.Errorf("goal ledger fetch timed out and its process group could not be killed after %s grace: %v (TERM: %v, wait: %v)", boundedCaptureGrace, killErr, termErr, waitErr))
				}
				return fail(fmt.Errorf("goal ledger fetch timed out after %s", budget))
			}
		}
	}

	tipOut, err := goalGit(e.Root, nil, "rev-parse", "--verify", fetchRefFor(opid))
	if err != nil {
		return fail(err)
	}
	return BoundedCapture{Tip: strings.TrimSpace(tipOut), OperationID: opid}, nil
}

func captureProcessGroupGone(pgid int) (bool, error) {
	err := syscall.Kill(-pgid, 0)
	if err == nil || errors.Is(err, syscall.EPERM) {
		return false, nil
	}
	if errors.Is(err, syscall.ESRCH) {
		return true, nil
	}
	return false, err
}
