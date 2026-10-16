package project

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

func PublicationEndpoint(root string) (goal.Endpoint, error) {
	return CheckedEndpoint(goal.ResolveEndpoint(root))
}
func CheckedEndpoint(endpoint goal.Endpoint, err error) (goal.Endpoint, error) {
	if err == nil {
		endpoint.CheckCandidate = func(commit string) error { return CheckCommit(endpoint.Root, commit) }
	}
	return endpoint, err
}
func CheckCommit(root, commit string) (err error) {
	candidate, err := (gittree.Workspace{Dir: root}).NewDetachedCommitWorktree(commit)
	if err != nil {
		return fmt.Errorf("the records could not be read: %v; the push waits — retry the publication", err)
	}
	defer func() {
		if cause := candidate.Close(); cause != nil && err == nil {
			err = fmt.Errorf("the temporary records could not be removed: %v; the push waits — retry the publication", cause)
		}
	}()
	roots, err := candidateRoots(candidate.Workspace())
	var read *Project
	if err == nil {
		read, err = Read(roots)
	}
	if err != nil {
		return fmt.Errorf("the records could not be read: %v; the push waits — retry the publication", err)
	}
	if len(read.Problems) == 0 {
		return nil
	}
	problem := read.Problems[0]
	for _, record := range read.queried() {
		if record.Path == problem.Path && strings.HasPrefix(problem.Message, "the goal ") {
			id := strings.TrimSuffix(strings.TrimPrefix(problem.Message, "the goal "), " is not in the ledger")
			return fmt.Errorf("the %s page %s names goal %s, which is not in the ledger; the push waits\n— open the goal or fix the page", record.Kind, filepath.Base(record.Path), id)
		}
	}
	words := strings.Fields(problem.Message)
	for i, word := range words {
		words[i] = filepath.Base(word)
	}
	return fmt.Errorf("the page %s: %s; the push waits\n— fix the page", filepath.Base(problem.Path), strings.Join(words, " "))
}

// candidateRoots reads a tree, which need not contain an installed engine.
// Only the candidate's committed template marker selects self-hosted homes.
func candidateRoots(workspace gittree.Workspace) (Roots, error) {
	checkout, err := workspace.TopLevel()
	if err != nil {
		return Roots{}, err
	}
	// Both paths canonical before they are compared: git reports /private/var on macOS where a scratch fallback hands out /var.
	checkout = canonical(checkout)
	installation := canonical(workspace.Dir)
	if installation == checkout && config.TemplateMode(filepath.Join(checkout, "metasystem")) {
		installation = filepath.Join(checkout, "metasystem")
	}
	state := checkout
	if filepath.Base(installation) == "metasystem" && config.TemplateMode(installation) {
		state = installation
	}
	return Roots{Checkout: checkout, Installation: stateroot.Installation(installation), StateRoot: stateroot.State(state)}, nil
}
