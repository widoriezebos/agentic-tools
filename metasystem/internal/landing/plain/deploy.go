package plain

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/behaviorsurface"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/enginebuild"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
)

type DeployOptions struct {
	Now               bool
	RestartSupervisor func(install string) error
	MergeSettled      func(fix *Fix) (bool, error)
}

type DeployRun struct {
	Person              ActProvenance `json:"person"`
	At                  string        `json:"at"`
	Before              string        `json:"before"`
	After               string        `json:"after"`
	Steps               []string      `json:"steps"`
	RestartAttempt      string        `json:"restart_attempt,omitempty"`
	Unchanged           bool          `json:"unchanged"`
	SupervisorRestarted bool          `json:"supervisor_restarted"`
}

func installedEngineStamp(install string) (string, error) {
	file, err := os.Open(filepath.Join(install, "bin", "metasystem"))
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	defer file.Close()
	return enginebuild.ReadStamp(file)
}

// DeployEngine replaces only the registered installation's engine. Admission,
// the pause and the keeper remain owned by the lane.
func DeployEngine(home string, person ActProvenance, now time.Time, opts DeployOptions, seams ProveSeams) (run DeployRun, err error) {
	run = DeployRun{Person: person, At: now.UTC().Format(time.RFC3339Nano), Steps: []string{}}
	record, _, err := lane.Read(home)
	if err != nil {
		return run, err
	}
	layout, err := record.Layout()
	if err != nil {
		return run, err
	}
	install, checkout := string(layout.Install), string(layout.Checkout)
	for {
		wait := false
		err = withLock(install, func() error {
			_, _, alive, err := ReadRunning(install, seams)
			if err != nil {
				return err
			}
			wait = alive && !opts.Now
			if wait {
				return nil
			}
			run.Before, err = installedEngineStamp(install)
			if err != nil {
				return err
			}
			if _, err = seams.git(checkout, "fetch", "origin", "main"); err != nil {
				return err
			}
			run.Steps = append(run.Steps, "fetched origin main")
			main, err := seams.git(checkout, "rev-parse", "--verify", "origin/main^{commit}")
			if err != nil {
				return err
			}
			if run.Before == main {
				runs, err := readLines[DeployRun](filepath.Join(Dir(install), "deploys.jsonl"))
				if err != nil {
					return err
				}
				if len(runs) > 0 && runs[len(runs)-1].After == run.Before && !runs[len(runs)-1].SupervisorRestarted {
					run.After, run.RestartAttempt = run.Before, runs[len(runs)-1].RestartAttempt
					return nil
				}
				run.After, run.Unchanged = run.Before, true
				run.Steps = append(run.Steps, "nothing to deploy")
				return nil
			}
			if err := deployMerge(install, checkout, main, opts, seams, &run); err != nil {
				return err
			}
			if _, err = seams.git(checkout, "checkout", "--detach", main); err != nil {
				return err
			}
			run.Steps = append(run.Steps, "checked out origin main detached")
			run.After, err = deployEngineBuild(install, main, seams, os.Getenv("METASYSTEM_BUILD_STAMP"))
			if err != nil {
				return err
			}
			run.Steps = append(run.Steps, "built and verified "+filepath.Join(install, "bin", "metasystem"))
			if running, _, alive, err := ReadRunning(install, seams); err != nil {
				return err
			} else if alive && opts.Now {
				run.RestartAttempt = running.Attempt
				run.Steps = append(run.Steps, "left the proof's restart to the lane")
			}
			return appendLine(filepath.Join(Dir(install), "deploys.jsonl"), run)
		})
		if err != nil || !wait {
			break
		}
		if len(run.Steps) == 0 {
			run.Steps = append(run.Steps, "waited for the running proof")
		}
		if seams.WaitPoll != nil {
			seams.WaitPoll()
		} else {
			time.Sleep(time.Second)
		}
	}
	if err == nil && !run.Unchanged {
		if opts.RestartSupervisor == nil {
			return run, errors.New("the engine was built, but no supervisor restart was supplied")
		}
		err = opts.RestartSupervisor(install)
		if err == nil {
			run.SupervisorRestarted = true
			run.Steps = append(run.Steps, "restarted the supervisor; kept the lane's run state")
			err = appendLine(filepath.Join(Dir(install), "deploys.jsonl"), run)
		}
	}
	return run, err
}

func deployMerge(install, checkout, main string, opts DeployOptions, seams ProveSeams, run *DeployRun) error {
	tip, err := seams.git(checkout, "rev-parse", "--verify", "--quiet", "MERGE_HEAD^{commit}")
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() == 1 {
		return nil
	}
	if err != nil {
		return err
	}
	fix, err := readFix(install, true)
	if err != nil {
		return err
	}
	head, err := seams.git(checkout, "rev-parse", "--verify", "HEAD^{commit}")
	if err != nil {
		return err
	}
	stale := fix != nil && fix.Commit == head && fix.Tip == tip && (fix.State == "abandoned" || fix.State == "rebuild")
	if fix != nil && fix.Commit == head && fix.Tip == tip && fix.State == "resolving" {
		stale, err = codeMoved(install, checkout, head, main, seams)
		if err != nil {
			return err
		}
	}
	if !stale {
		goal := "the pending merge"
		if fix != nil {
			goal = fix.Goal
		}
		return &Refusal{Code: "LANE_DEPLOY_MERGE", Reason: "the lane is resolving " + goal + "; give it back or wait", Next: "metasystem landing give-back " + goal + " --reason TEXT"}
	}
	if opts.MergeSettled == nil {
		return errors.New("the abandoned merge's writers cannot be checked; wait before deploying")
	}
	settled, err := opts.MergeSettled(fix)
	if err != nil {
		return err
	}
	if !settled {
		return &Refusal{Code: "LANE_DEPLOY_MERGE", Reason: "the lane is resolving " + fix.Goal + "; give it back or wait", Next: "metasystem landing give-back " + fix.Goal + " --reason TEXT"}
	}
	if fix.State != "abandoned" {
		fix.State, fix.Reason = "abandoned", "superseded by main's code move; rebuild"
		if err := WriteFix(install, fix); err != nil {
			return err
		}
	}
	if _, err := seams.git(checkout, "merge", "--abort"); err != nil {
		return err
	}
	run.Steps = append(run.Steps, "aborted the abandoned merge of "+fix.Goal)
	return nil
}

// RestartDeployedProof is the keeper's consumption of an engine-change request.
// Deploy never signals a proof; the lane settles it through its usual owner.
func RestartDeployedProof(install, checkout string, seams ProveSeams) error {
	var restart Running
	err := withLock(install, func() error {
		runs, err := readLines[DeployRun](filepath.Join(Dir(install), "deploys.jsonl"))
		if err != nil || len(runs) == 0 {
			return err
		}
		running, recorded, _, err := ReadRunning(install, seams)
		if err != nil || !recorded || runs[len(runs)-1].RestartAttempt != running.Attempt {
			return err
		}
		result := timedResult(Result{Result: "cancelled", Reason: "restart after the lane's engine changed", Attempt: running.Attempt, Tree: running.Tree, Commit: running.Commit, Log: running.Log, Fresh: running.Fresh, Trunk: running.Trunk, Person: running.Person, Executions: running.Executions, BatchID: running.BatchID, BatchMembers: running.BatchMembers, Goals: running.BatchMembers, ClassificationOf: running.ClassificationOf, ObservedIncidents: running.ObservedIncidents}, running.Since, seams.now())
		if cancel, err := readCancel(install, running); err != nil {
			return err
		} else if cancel == nil {
			data, err := json.Marshal(result)
			if err != nil {
				return err
			}
			if _, err := atomicfile.WriteFile(cancelPath(install, running.Attempt), data, 0600, install); err != nil {
				return err
			}
		}
		if !atomicfile.Confirm(cancelPath(install, running.Attempt), install) {
			return errors.New("the restart could not be made durable; the running check continues")
		}
		if err := seams.endProof(running); err != nil {
			return err
		}
		batch, err := ReadBatch(install)
		if err != nil {
			return err
		}
		if batch != nil && batch.ID == running.BatchID {
			batch.State, batch.Open, batch.SealedAt = BatchPrepared, true, ""
			batch.Restarts = append(batch.Restarts, Restart{At: seams.now().Format(time.RFC3339Nano), Reason: result.Reason})
			if err := writeBatch(install, batch); err != nil {
				return err
			}
		}
		restart = running
		return finishCancel(install, running, result)
	})
	if err == nil && restart.Trunk {
		seams.Trunk, seams.Fresh = true, true
		seams.Person, seams.ClassificationOf = restart.Person, restart.ClassificationOf
		_, _, err = Start(install, checkout, seams)
	}
	return err
}

func deployEngineBuild(install, main string, seams ProveSeams, stampOverride string) (string, error) {
	if stampOverride != "" {
		return "", errors.New("a build stamp override is set; unset the override before deploying origin main")
	}
	prefix, err := seams.git(install, "rev-parse", "--show-prefix")
	if err != nil {
		return "", err
	}
	status, err := deployStatus(install, seams)
	if err != nil {
		return "", err
	}
	policy, err := behaviorsurface.Load()
	if err != nil {
		return "", err
	}
	for _, entry := range strings.Split(status, "\x00") {
		if entry == "" {
			continue
		}
		if len(entry) < 4 || entry[2] != ' ' {
			return "", fmt.Errorf("unreadable git status record %q", entry)
		}
		included, err := policy.Includes(behaviorsurface.Engine, entry[3:], strings.TrimRight(prefix, "\r\n"))
		if err != nil {
			return "", err
		}
		if included {
			return "", fmt.Errorf("the lane checkout is dirty on engine path %q; settle its edits before deploying", entry[3:])
		}
	}
	target := filepath.Join(install, "bin", "metasystem")
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return "", err
	}
	file, err := os.CreateTemp(filepath.Dir(target), ".metasystem.deploy-*")
	if err != nil {
		return "", err
	}
	staging := file.Name()
	defer os.Remove(staging)
	if err := file.Close(); err != nil {
		return "", err
	}
	if err := os.Remove(staging); err != nil {
		return "", err
	}
	command := exec.Command("go", "run", "./cmd/devgate", "build", "--out", staging)
	command.Dir = install
	if seams.Command != nil {
		err = seams.Command(command)
	} else {
		err = command.Run()
	}
	if err != nil {
		return "", fmt.Errorf("build the lane's engine: %w", err)
	}
	file, err = os.Open(staging)
	if err != nil {
		return "", err
	}
	stamp, readErr := enginebuild.ReadStamp(file)
	if err := errors.Join(readErr, file.Close()); err != nil {
		return "", err
	}
	if stamp != main {
		return "", errors.New("the staged engine does not carry origin main's stamp; the installed engine is untouched")
	}
	if err := os.Rename(staging, target); err != nil {
		return "", err
	}
	return stamp, nil
}

// Porcelain columns and NUL-delimited paths must survive the Git read intact.
func deployStatus(install string, seams ProveSeams) (string, error) {
	args := []string{"status", "--porcelain=v1", "-z", "--no-renames", "--untracked-files=all"}
	if seams.Git != nil {
		return seams.Git(install, args...)
	}
	command := exec.Command("git", append([]string{"-C", install}, args...)...)
	command.Env = gittree.ScrubbedEnviron()
	var output bytes.Buffer
	command.Stdout = &output
	var err error
	if seams.Command != nil {
		err = seams.Command(command)
	} else {
		err = command.Run()
	}
	return output.String(), err
}
