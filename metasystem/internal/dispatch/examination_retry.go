package dispatch

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
)

// ExaminationRetryAdmissible decides whether a critic chain's newest round,
// which ended without a findings return, may be followed by one fresh
// examination round of the same chain. The chain's round cap, subject and
// records stay the follow-up's own; this only admits the retry. A round that
// is still live, that completed, that was cancelled, or that wrote a return
// is refused, and so is one whose recorded process is not proven dead.
func ExaminationRetryAdmissible(repoRoot string, latest map[string]any) error {
	return examinationRetryAdmissible(repoRoot, latest, CustodyDeathDependencies{})
}

// ExaminationRetryAdmissibleWith is ExaminationRetryAdmissible with the
// custody owner's process-tag matcher, for callers that have it.
func ExaminationRetryAdmissibleWith(repoRoot string, latest map[string]any, custody CustodyDeathDependencies) error {
	return examinationRetryAdmissible(repoRoot, latest, custody)
}

// examinationRetryAdmissible also requires a recorded process to be proven
// dead through the custody owner: a terminal status alone is not proof that
// the old examination stopped.
func examinationRetryAdmissible(repoRoot string, latest map[string]any, custody CustodyDeathDependencies) error {
	role, status := asString(latest["role"]), asString(latest["status"])
	if role != "code-critic" && role != "design-critic" {
		return fmt.Errorf("an examination retry follows a critic round, not a %s round", role)
	}
	round, err := strconv.ParseInt(fmt.Sprint(latest["round"]), 10, 64)
	if err != nil || round < 1 {
		return fmt.Errorf("the critic round has no round number")
	}
	switch {
	case !TerminalStatus(status):
		return fmt.Errorf("examination round %d is still %s; a live examination is never retried", round, status)
	case status == "completed":
		return fmt.Errorf("examination round %d completed; its findings are decided, not retried", round)
	case status == "cancelled":
		return fmt.Errorf("examination round %d was cancelled; a cancellation is not retried", round)
	}
	if latest["pid"] != nil && asString(latest["groupDeathProvenAt"]) == "" {
		// Without the reaper's recorded group-death proof, the custody owner
		// must prove the recorded processes dead now.
		if death := ProveCustodyDeath(repoRoot, latest, custody); death.Outcome != CustodyDeathProven {
			return fmt.Errorf("examination round %d's process is not proven stopped (%s: %s); it is never retried while it may still run", round, death.Outcome, death.Reason)
		}
	}
	root := asString(latest["jobId"])
	if parent := asString(latest["parentJob"]); parent != "" {
		root = parent
		if chainRoot, err := ChainRootOf(repoRoot, parent); err == nil {
			root = chainRoot
		}
	}
	returnPath := filepath.Join(repoRoot, "artifacts", "agents", root, "rounds", strconv.FormatInt(round, 10), "return.json")
	if _, err := os.Stat(returnPath); err == nil {
		return fmt.Errorf("examination round %d wrote a return; its findings are decided, not retried", round)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}
