package plain

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// impactScope binds selection to the main commit from which the batch began.
func impactScope(install, checkout string, seams ProveSeams) scopeDecision {
	d := scopeDecision{scopeRecord: scopeRecord{Scope: "impact", ScopeReason: "impact proof against the batch base; static checks first"}}
	batch, err := ReadBatch(install)
	if err == nil && batch == nil {
		err = fmt.Errorf("no batch base is recorded")
	}
	if err == nil {
		d.BaseCommit = batch.Base
		d.Base, err = seams.git(checkout, "rev-parse", "--verify", batch.Base+"^{tree}")
		if err == nil && strings.TrimSpace(d.Base) == "" {
			err = fmt.Errorf("the batch base tree is empty")
		}
	}
	if err != nil {
		d.ScopeReason = "impact proof error: " + err.Error()
	}
	return d
}

func (d *scopeDecision) impactPlan(seams ProveSeams, dir string) error {
	plan, err := ReadImpactPlan(seams, dir, d.BaseCommit)
	if err == nil {
		d.PlanHash = fmt.Sprintf("%x", sha256.Sum256([]byte(plan)))
	}
	return err
}

// ReadImpactPlan asks the serving engine for its exact impact selection.
func ReadImpactPlan(seams ProveSeams, dir, base string) (string, error) {
	executable := seams.Executable
	if executable == nil {
		executable = os.Executable
	}
	engine, err := executable()
	if err != nil {
		return "", err
	}
	command := exec.Command(engine, "test", "impact", "--plan", "--base", base)
	command.Dir = dir
	command.Env = append(os.Environ(), "LANDING_ONLY=", "LANDING_PROOF_BASE="+base)
	var plan, problem bytes.Buffer
	command.Stdout, command.Stderr = &plan, &problem
	run := seams.Command
	if run == nil {
		run = (*exec.Cmd).Run
	}
	if err := run(command); err != nil {
		return "", fmt.Errorf("the impact plan could not be read: %w; %s", err, strings.TrimSpace(problem.String()))
	}
	if plan.Len() == 0 {
		return "", fmt.Errorf("the impact plan is empty")
	}
	return plan.String(), nil
}

func (s ProveSeams) acceptsGreen(install, checkout string, result Result) bool {
	if !result.reusableGreen(s.now()) {
		return false
	}
	if s.Impact {
		d := impactScope(install, checkout, s)
		return result.Scope == "impact" && d.Base != "" && result.Base == d.Base && result.BaseCommit == d.BaseCommit
	}
	return result.Scope != "impact"
}
