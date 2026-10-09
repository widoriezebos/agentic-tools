package dispatch

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
)

// ExaminationRetryAdmissible admits an environment failure without a return,
// or one fresh critic examination when a completed return has unavailable stop
// inputs. Readable completed findings are decided instead of retried. Live
// processes, cancellation and an already reserved unknown-input retry hold.
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
		state := loadCritiqueState(repoRoot)
		persisted, present := state.records[asString(latest["jobId"])]
		if !present || asString(persisted["status"]) != "completed" {
			return fmt.Errorf("completed examination has no matching retained completed record")
		}
		if _, err := CollectExamination(repoRoot, asString(latest["jobId"])); err == nil {
			return fmt.Errorf("examination round %d completed with readable stop inputs; its findings are decided, not retried", round)
		}
		root := state.records[state.chainRoot(asString(latest["jobId"]))]
		if asString(root["unknownExaminationRetryFrom"]) != "" {
			return fmt.Errorf("the fresh examination has already been reserved; unavailable stop inputs hold the critique")
		}
		if latest["pid"] != nil && asString(latest["groupDeathProvenAt"]) == "" {
			if death := ProveCustodyDeath(repoRoot, latest, custody); death.Outcome != CustodyDeathProven {
				return fmt.Errorf("completed examination round %d is not proven quiescent", round)
			}
		}
		return nil
	case status == "cancelled":
		return fmt.Errorf("examination round %d was cancelled; a cancellation is not retried", round)
	case role == "design-critic" && status == "timeout":
		return fmt.Errorf("design examination round %d reached its deadline; a deadline is not retried", round)
	}
	if latest["pid"] != nil && asString(latest["groupDeathProvenAt"]) == "" {
		// Without the reaper's recorded group-death proof, the custody owner
		// must prove the recorded processes dead now.
		if death := ProveCustodyDeath(repoRoot, latest, custody); death.Outcome != CustodyDeathProven {
			return fmt.Errorf("examination round %d may still be running, so it is not retried (%s: %s)", round, death.Outcome, death.Reason)
		}
	}
	root := asString(latest["jobId"])
	if parent := asString(latest["parentJob"]); parent != "" {
		root = parent
		if chainRoot, err := ChainRootOf(repoRoot, parent); err == nil {
			root = chainRoot
		}
	}
	if role == "design-critic" && asString(loadCritiqueState(repoRoot).records[root]["unknownExaminationRetryFrom"]) != "" {
		return fmt.Errorf("the one fresh design examination is already reserved")
	}
	returnPath := filepath.Join(repoRoot, "artifacts", "agents", root, "rounds", strconv.FormatInt(round, 10), "return.json")
	if _, err := os.Stat(returnPath); err == nil {
		return fmt.Errorf("examination round %d wrote a return; its findings are decided, not retried", round)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

// ReserveUnknownExaminationRetry consumes the one fresh examination before
// dispatch. Failure after this record leaves the unit held, never granting a
// second automatic launch.
func ReserveUnknownExaminationRetry(repoRoot, jobID string) error {
	_, err := withFindingRegisterLock(repoRoot, func() (string, error) {
		state := loadCritiqueState(repoRoot)
		latest, present := state.records[jobID]
		if !present || asString(latest["status"]) != "completed" && !(asString(latest["role"]) == "design-critic" && asString(latest["status"]) == "failed") {
			return "", fmt.Errorf("unknown-input retry needs a terminal critic examination")
		}
		if err := ExaminationRetryAdmissible(repoRoot, latest); err != nil {
			return "", err
		}
		rootID := state.chainRoot(jobID)
		err := withRecordLock(repoRoot, rootID, func(path string) error {
			root, err := readObject(path)
			if err != nil {
				return err
			}
			if asString(root["unknownExaminationRetryFrom"]) != "" {
				return fmt.Errorf("the one fresh examination is already reserved")
			}
			root["unknownExaminationRetryFrom"] = jobID
			round, _ := numInt(latest["round"])
			folded, _ := numInt(root[findingRegisterRoundField])
			if round == folded+1 {
				root[findingRegisterRoundField] = round
				delete(root, findingRegisterSubjectDigestField)
			}
			return writeRecord(path, root)
		})
		return "", err
	})
	return err
}
