package launch

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"syscall"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/boundedexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/processmeasure"
)

type UnitDeclaration struct {
	Values                                                                    [4]string
	Content, Commit, Tree, Branch, Directory, EnvironmentSHA256, SourceSHA256 string
	FullArgv                                                                  []string
}

type UnitCheck struct {
	FullArgv    []string `json:"fullArgv,omitempty"`
	ProcessAct  string   `json:"processAct,omitempty"`
	SelectedBy  string   `json:"selectedBy,omitempty"`
	Reason      string   `json:"reason,omitempty"`
	Base        string   `json:"base,omitempty"`
	SourceTree  string   `json:"sourceTree"`
	Cheap       string   `json:"cheap"`
	Audits      string   `json:"audits"`
	Minutes     int      `json:"minutes"`
	Directory   string   `json:"directory"`
	Environment []string `json:"environment"`

	Declaration *UnitDeclaration `json:"declaration,omitempty"`
}
type CheckExit struct {
	Name    string  `json:"name"`
	Exit    int     `json:"exit"`
	Output  string  `json:"output"`
	Error   string  `json:"error,omitempty"`
	Minutes float64 `json:"minutes"`
}

type CheckExecution struct {
	ExecutionID, Goal, Run, Round, Role, BriefSHA256, SourceSHA256 string
	Steps                                                          []processmeasure.Step
}
type CheckContext struct {
	CheckExecution
	Parent string
	Now    func() time.Time
}

func (check UnitCheck) Run(directory, records string, context CheckContext) (string, []CheckExit, error) {
	if check.Cheap == "" || check.Audits == "" || check.Minutes <= 0 || check.Directory == "" || check.Environment == nil {
		return "", nil, errors.New("the frozen unit check is incomplete")
	}
	if err := os.MkdirAll(records, 0700); err != nil {
		return "", nil, err
	}
	execution, err := os.MkdirTemp(records, "check-")
	if err != nil {
		return "", nil, err
	}
	if context.Now == nil {
		context.Now = time.Now
	}
	observation := context.CheckExecution
	observation.ExecutionID = filepath.Base(execution)
	if check.Declaration != nil {
		observation.SourceSHA256 = check.Declaration.SourceSHA256
	}
	save := func() error {
		return writeUnitJSON(filepath.Join(execution, "observation.json"), observation, execution)
	}
	var exits []CheckExit
	var failures []error
	for _, command := range []struct{ name, text string }{{"cheap", check.Cheap}, {"audits", check.Audits}} {
		process := exec.Command("/bin/sh", "-c", command.text)
		process.Dir, process.Env = directory, check.Environment
		if command.name == "cheap" && check.Base != "" {
			process.Env = append(slices.Clone(check.Environment), "LANDING_PROOF_BASE="+check.Base)
		}
		var output bytes.Buffer
		process.Stdout, process.Stderr = &output, &output
		started := context.Now()
		step := processmeasure.Step{ID: context.Run + "/" + context.Round + "/" + observation.ExecutionID + ":" + command.name, Kind: "attest", Parent: context.Parent, Act: check.ProcessAct, Start: started.UTC().Format(time.RFC3339Nano), Argv: []string{"/bin/sh", "-c", command.text}, FullArgv: check.FullArgv, Coverage: "unknown"}
		if check.Declaration != nil {
			step.FullArgv = check.Declaration.FullArgv
		}
		observation.Steps = append(observation.Steps, step)
		if err := save(); err != nil {
			return execution, exits, err
		}
		err := boundedexec.Run(process, boundedexec.FixedBound(time.Duration(check.Minutes)*time.Minute, "proof.deadline"), command.name)
		ended := context.Now()
		result := CheckExit{Name: command.name, Output: output.String(), Minutes: ended.Sub(started).Minutes()}
		step.End, step.Terminal, step.Outcome = ended.UTC().Format(time.RFC3339Nano), true, "passed"
		if err != nil {
			step.Outcome = "failed"
			if errors.Is(err, boundedexec.ErrTimedOut) {
				step.Outcome = "deadline"
			}
			result.Exit, result.Error = -1, err.Error()
			var exit *exec.ExitError
			if errors.As(err, &exit) {
				result.Exit = exit.ExitCode()
				if status, ok := exit.Sys().(syscall.WaitStatus); ok && status.Signaled() {
					step.Outcome = "cancelled"
				}
			}
			failures = append(failures, err)
		}
		exits = append(exits, result)
		observation.Steps[len(observation.Steps)-1] = step
		if err := save(); err != nil {
			return execution, exits, errors.Join(append(failures, err)...)
		}
	}
	err = writeUnitJSON(filepath.Join(execution, "result.json"), struct {
		ExecutionID string      `json:"executionId"`
		Directory   string      `json:"runDirectory"`
		Check       UnitCheck   `json:"check"`
		Exits       []CheckExit `json:"exits"`
	}{filepath.Base(execution), directory, check, exits}, execution)
	return execution, exits, errors.Join(append(failures, err)...)
}
