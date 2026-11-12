package main

import (
	"fmt"
	"slices"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

// publicationRequirements names the executable roots; the runner owns their
// prerequisite closure and runs each group once.
func publicationRequirements(contract testpolicy.Contract) ([]string, error) {
	const root = "fast-static-build"
	if !slices.ContainsFunc(contract.Groups, func(group testpolicy.Group) bool { return group.ID == root }) {
		return nil, fmt.Errorf("the contract has no publication group %s; nothing was run", root)
	}
	return []string{root}, nil
}

func impactCheckCommand(command string) bool {
	words := shellWords(command)
	return len(words) >= 3 && words[0] == "metasystem" && words[1] == "test" && words[2] == "impact"
}

func publicationCheckCoversRequirements(directory, base string) bool {
	top, err := goalBranchGit(directory, "rev-parse", "--show-toplevel")
	if err != nil {
		return false
	}
	command, err := landingProofCommand(directory, top, base, "proof.cheap", goalBranchGit)
	return err == nil && impactCheckCommand(command)
}
