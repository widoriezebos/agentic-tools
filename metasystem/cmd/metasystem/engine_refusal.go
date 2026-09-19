package main

import (
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/enginecause"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
)

const engineRefusalCode = "TEST_POLICY_ENGINE_REQUIRED"

func engineRefusal(token string, facts []enginecause.Fact, detail string) error {
	return enginecause.Refuse(token, facts, detail)
}

func engineCheckoutFacts(checkout string) []enginecause.Fact {
	return []enginecause.Fact{enginecause.Path("checkout", checkout)}
}

func linkedWorktreeMainCheckout(installation string) (string, bool) {
	read := func(option string) string {
		command := exec.Command("git", "-C", installation, "rev-parse", "--path-format=absolute", option)
		command.Env = gittree.ScrubbedEnviron()
		output, err := command.Output()
		if err != nil {
			return ""
		}
		return filepath.Clean(strings.TrimSpace(string(output)))
	}
	common, gitDir := read("--git-common-dir"), read("--git-dir")
	if common == "" || gitDir == "" || common == gitDir {
		return "", false
	}
	return filepath.Dir(common), true
}

// linkedEnrollmentRoot names the installation inside a linked worktree's main
// checkout at the same relative path, so a nested installation keeps its
// subdirectory. Callers try the installation's own enrollment first: a proof
// worktree may carry one, while a batch tree worktree borrows its checkout's.
func linkedEnrollmentRoot(installation string) (string, bool) {
	mainCheckout, linked := linkedWorktreeMainCheckout(installation)
	if !linked {
		return "", false
	}
	command := exec.Command("git", "-C", installation, "rev-parse", "--show-toplevel")
	command.Env = gittree.ScrubbedEnviron()
	output, err := command.Output()
	if err != nil {
		return "", false
	}
	resolved, err := filepath.EvalSymlinks(installation)
	if err != nil {
		return "", false
	}
	relative, err := filepath.Rel(filepath.Clean(strings.TrimSpace(string(output))), resolved)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", false
	}
	return filepath.Join(mainCheckout, relative), true
}

func linkedWorktreeMainInstallation(installation string) (string, bool) {
	main, linked := linkedWorktreeMainCheckout(installation)
	if !linked {
		return "", false
	}
	prefixCommand := exec.Command("git", "-C", installation, "rev-parse", "--show-prefix")
	prefixCommand.Env = gittree.ScrubbedEnviron()
	prefix, err := prefixCommand.Output()
	if err != nil {
		return "", false
	}
	return filepath.Join(main, filepath.FromSlash(strings.TrimSuffix(strings.TrimSpace(string(prefix)), "/"))), true
}

func batchPrefixProofControlRoot(installation, requested string) (string, error) {
	controlRoot, err := canonicalProofRoot(requested)
	if err != nil {
		return "", fmt.Errorf("resolve batch prefix proof control root: %w", err)
	}
	mainInstallation, linked := linkedWorktreeMainInstallation(installation)
	if !linked {
		return "", fmt.Errorf("batch prefix proof execution root is not a linked worktree")
	}
	mainInstallation, err = canonicalProofRoot(mainInstallation)
	if err != nil {
		return "", fmt.Errorf("resolve batch prefix main installation: %w", err)
	}
	if controlRoot != mainInstallation {
		return "", fmt.Errorf("batch prefix proof control root %s does not own execution installation %s", controlRoot, installation)
	}
	return controlRoot, nil
}

func enrollmentRefusal(installation string, cause error) error {
	facts := engineCheckoutFacts(installation)
	if checkout, linked := linkedWorktreeMainCheckout(installation); linked {
		facts = append(facts, enginecause.Path("linked-worktree", checkout))
	}
	return engineRefusal("not-enrolled", facts, fmt.Sprintf("retained destination engine is not authenticated: %v", cause))
}

func judgmentRefusal(cause error, facts []enginecause.Fact, detail string) error {
	token := "judgment-failed"
	switch {
	case errors.Is(cause, steward.ErrJudgmentStalled):
		token = "judgment-stalled"
		if step, seconds, ok := steward.JudgmentStall(cause); ok {
			facts = append([]enginecause.Fact{enginecause.Value("step", step), enginecause.Value("seconds", fmt.Sprint(seconds))}, facts...)
		}
	case errors.Is(cause, steward.ErrNotOwned):
		token = "engine-behind-tip"
	}
	return engineRefusal(token, facts, detail+": "+cause.Error())
}

func decisionMismatchRefusal(candidateTree, policyBaseCommit, baseContractDigest string, decision testingPlanOutput) error {
	fields := []struct{ name, ours, engine string }{
		{"candidate-tree", candidateTree, decision.CandidateTree},
		{"policy-base-commit", policyBaseCommit, decision.PolicyBaseCommit},
		{"base-contract-digest", baseContractDigest, decision.BaseContractDigest},
	}
	for _, field := range fields {
		if field.ours != field.engine {
			return engineRefusal("decision-mismatch", []enginecause.Fact{
				enginecause.Value("field", field.name), enginecause.Value("ours", field.ours), enginecause.Value("engine", field.engine),
			}, "retained trusted-base engine returned a mismatched policy decision")
		}
	}
	return nil
}
