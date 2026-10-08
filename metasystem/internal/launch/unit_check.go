package launch

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/boundedexec"
)

type UnitCheck struct {
	SelectedBy  string   `json:"selectedBy,omitempty"`
	Reason      string   `json:"reason,omitempty"`
	SourceTree  string   `json:"sourceTree"`
	Cheap       string   `json:"cheap"`
	Audits      string   `json:"audits"`
	Minutes     int      `json:"minutes"`
	Directory   string   `json:"directory"`
	Environment []string `json:"environment"`
}
type CheckExit struct {
	Name    string  `json:"name"`
	Exit    int     `json:"exit"`
	Output  string  `json:"output"`
	Error   string  `json:"error,omitempty"`
	Minutes float64 `json:"minutes"`
}

func (check UnitCheck) Run(directory, records string) (string, []CheckExit, error) {
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
	var exits []CheckExit
	var failures []error
	for _, command := range []struct{ name, text string }{{"cheap", check.Cheap}, {"audits", check.Audits}} {
		process := exec.Command("/bin/sh", "-c", command.text)
		process.Dir, process.Env = directory, check.Environment
		var output bytes.Buffer
		process.Stdout, process.Stderr = &output, &output
		started := time.Now()
		err := boundedexec.Run(process, boundedexec.FixedBound(time.Duration(check.Minutes)*time.Minute, "proof.deadline"), command.name)
		result := CheckExit{Name: command.name, Output: output.String(), Minutes: time.Since(started).Minutes()}
		if err != nil {
			result.Exit, result.Error = -1, err.Error()
			var exit *exec.ExitError
			if errors.As(err, &exit) {
				result.Exit = exit.ExitCode()
			}
			failures = append(failures, err)
		}
		exits = append(exits, result)
	}
	err = writeUnitJSON(filepath.Join(execution, "result.json"), struct {
		ExecutionID string      `json:"executionId"`
		Directory   string      `json:"runDirectory"`
		Check       UnitCheck   `json:"check"`
		Exits       []CheckExit `json:"exits"`
	}{filepath.Base(execution), directory, check, exits}, execution)
	return execution, exits, errors.Join(append(failures, err)...)
}
