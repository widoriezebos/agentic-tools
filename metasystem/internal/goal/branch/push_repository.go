package branch

import (
	"errors"
	"os/exec"
	"strings"
)

// pushRepository is the local Git boundary for one Push operation. The
// transport remains on PushRequest because it owns remote observations.
type pushRepository struct {
	TxnRefs      func(string) ([]string, error)
	Txn          func(string, string) (pushTxn, error)
	Tip          func(string, string) (string, bool, error)
	Ancestor     func(string, string, string) (bool, error)
	Range        func(string, string, string, string) ([]Commit, error)
	HeadRef      func(string) string
	TrackedClean func(string, bool) (bool, error)
	WriteTxn     func(string, string, pushTxn) error
	ClearRef     func(string, string) error
	RecordOrigin func(string, string, string) error
	Detach       func(string, string) error
	RestoreHead  func(string, string) error
	SwitchGoal   func(string, string) error
	MoveRefs     func(string, string, string, string, string) error
}

func gitPushRepository() pushRepository {
	return pushRepository{
		TxnRefs: func(repo string) ([]string, error) {
			out, err := gitOutput(repo, "for-each-ref", "--format=%(refname)", "refs/metasystem/goals/txn/")
			return strings.Fields(string(out)), err
		},
		Txn: readPushTxn, Tip: localBranchTip, Ancestor: ancestor, Range: ValidateRange,
		HeadRef: func(repo string) string {
			out, _ := gitOutput(repo, "symbolic-ref", "-q", "HEAD")
			return strings.TrimSpace(string(out))
		},
		TrackedClean: func(repo string, cached bool) (bool, error) {
			args := []string{"diff"}
			if cached {
				args = append(args, "--cached")
			}
			_, err := gitOutput(repo, append(args, "--quiet")...)
			if err == nil {
				return true, nil
			}
			var exit *exec.ExitError
			if errors.As(err, &exit) && exit.ExitCode() == 1 {
				return false, nil
			}
			return false, err
		},
		WriteTxn: writePushTxn, ClearRef: clearPushTxn, RecordOrigin: recordOriginTip,
		Detach: func(repo, tip string) error {
			_, err := gitOutput(repo, "switch", "--quiet", "--detach", tip)
			return err
		},
		RestoreHead: func(repo, ref string) error {
			_, err := gitOutput(repo, "symbolic-ref", "HEAD", ref)
			return err
		},
		SwitchGoal: func(repo, goal string) error {
			_, err := gitOutput(repo, "switch", "--quiet", "goal/"+goal)
			return err
		},
		MoveRefs: updateBranchAndOrigin,
	}
}
