package plain

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"strings"
)

func engineCheckCommand(seams ProveSeams, args string) (string, error) {
	executable := seams.Executable
	if executable == nil {
		executable = os.Executable
	}
	engine, err := executable()
	if err != nil {
		return "", err
	}
	return "'" + strings.ReplaceAll(engine, "'", "'\\''") + "' test groups " + args, nil
}

// engineCheck runs a testing group or reads its environment from the serving engine.
func engineCheck(seams ProveSeams, dir, args string, running Running, decision scopeDecision, output io.Writer, observed *proofOutput) error {
	command, err := engineCheckCommand(seams, args)
	if err != nil {
		return err
	}
	_, err = runCheck(seams, dir, command, running, "fast-static-build", decision, output, observed)
	return err
}

// prepareGate always checks static inputs before deciding whether to select tests.
func prepareGate(seams ProveSeams, install, checkout, dir, command string, running Running, decision *scopeDecision, output io.Writer, observed *proofOutput, result, previous Result) (Result, bool) {
	staticCommand, err := engineCheckCommand(seams, "fast-static-build")
	var report checkReport
	if err == nil {
		report, err = runCheck(seams, dir, staticCommand, running, "fast-static-build", *decision, output, observed)
	}
	result.Static = observed.static
	if err != nil || result.Static != Green {
		result.Result, result.Static = Red, Red
		if err == nil {
			err = fmt.Errorf("fast-static-build failed")
		}
		report.failed = []FailedUnit{{Unit: "fast-static-build"}}
		result = classifyRed(seams, install, checkout, staticCommand, dir, running, *decision, output, observed, result, previous, report, err,
			func(red, prior Result) Result {
				return replayGate(seams, install, checkout, staticCommand, running, red, prior)
			})
		return result, true
	}
	if seams.gateBaseline {
		return result, false
	}
	parents, err := seams.git(checkout, "show", "-s", "--format=%P", running.Commit)
	if err == nil && len(strings.Fields(parents)) == 2 {
		decision.BaseCommit = strings.Fields(parents)[0]
		decision.Base, err = seams.git(checkout, "rev-parse", "--verify", decision.BaseCommit+"^{tree}")

		share, cheap := 0, false
		if err == nil {
			if seams.ImpactCost == nil {
				err = fmt.Errorf("the impact cost reader is unavailable")
			} else {
				plan, planErr := ReadImpactPlan(seams, dir, decision.BaseCommit)
				err = planErr
				if err == nil {
					decision.PlanHash = fmt.Sprintf("%x", sha256.Sum256([]byte(plan)))
					share, cheap, err = seams.ImpactCost(dir, plan)
				}
			}
		}
		if err != nil {
			result.Result, result.Reason = Red, err.Error()
			result.Cause = &Cause{Kind: "environment", Evidence: running.Log}
			result.allowEnvironmentRepeat(previous)
			return result, true
		}
		result.Depth = "impact"
		if !cheap {
			result.Result = Skipped
			result.Reason = fmt.Sprintf("impact covers %d%%; the batch proof follows", share)
			return result, true
		}
	}
	hash := decision.PlanHash
	baseline, ok, baseDecision := gateBaseline(seams, install, checkout, command, running, output)
	*decision = baseDecision
	decision.PlanHash = hash
	if !ok {
		return baseline, true
	}
	return result, false
}

// reuseGate requires complete impact evidence for the batch's exact comparison.
func reuseGate(seams ProveSeams, install, dir string, running Running, decision scopeDecision, output io.Writer, observed *proofOutput) bool {
	gate, found, err := LastGate(install)
	if err != nil || !found || gate.Result != Green || gate.Tree != running.Tree || gate.Depth != "impact" || gate.Static != Green || gate.Base != decision.Base || gate.BaseCommit != decision.BaseCommit || gate.Environment == "" {
		return false
	}
	environment := &proofOutput{output: io.Discard}
	if err := engineCheck(seams, dir, "--environment", running, decision, output, environment); err != nil || environment.environment != gate.Environment {
		return false
	}
	observed.environment, observed.envSeen = gate.Environment, true
	observed.ran = append([]string{}, gate.Ran...)
	return true
}
