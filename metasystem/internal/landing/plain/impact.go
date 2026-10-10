package plain

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/verbresult"
)

// ImpactPlan is the selection returned by test impact --plan --json.
type ImpactPlan struct {
	Base       string   `json:"base"`
	Subject    string   `json:"subject"`
	Selections []string `json:"selections"`
}

// Text renders the plan content independently of its envelope's formatting.
func (p ImpactPlan) Text() string {
	var text strings.Builder
	fmt.Fprintf(&text, "plan: base %s (%s)\n", p.Base, p.Subject)
	for _, selection := range p.Selections {
		fmt.Fprintln(&text, "selection: "+selection)
	}
	return text.String()
}

// impactScope binds selection to the main commit from which the batch began.
func impactScope(install, checkout string, seams ProveSeams) scopeDecision {
	d := scopeDecision{scopeRecord: scopeRecord{Scope: "impact", ScopeReason: "impact check against the batch base; static checks first"}}
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
		d.ScopeReason = "impact check error: " + err.Error()
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
	command := exec.Command(engine, "test", "impact", "--plan", "--json", "--base", base)
	command.Dir = dir
	command.Env = append(os.Environ(), "LANDING_ONLY=", "LANDING_PROOF_BASE="+base)
	read := verbresult.Capture(command, "test impact")
	run := seams.Command
	if run == nil {
		run = (*exec.Cmd).Run
	}
	result, err := read(run(command))
	if err != nil {
		return "", fmt.Errorf("the impact plan could not be read: %w", err)
	}
	if result.Outcome != verbresult.Confirmed {
		return "", result.Err()
	}
	var plan ImpactPlan
	if err := result.DecodeData(&plan); err != nil {
		return "", fmt.Errorf("the impact plan could not be read: %w", err)
	}
	if plan.Base == "" {
		return "", fmt.Errorf("the impact plan's base is empty")
	}
	return plan.Text(), nil
}

func (s ProveSeams) acceptsGreen(install, checkout string, result Result) bool {
	if !result.reusableGreen(s.now()) {
		return false
	}
	if s.Gate {
		commit, _, err := s.subject(checkout)
		return err == nil && result.Requested == commit && result.Static == Green
	}
	if s.Impact {
		d := impactScope(install, checkout, s)
		return result.Scope == "impact" && d.Base != "" && result.Base == d.Base && result.BaseCommit == d.BaseCommit
	}
	return result.Scope != "impact"
}
