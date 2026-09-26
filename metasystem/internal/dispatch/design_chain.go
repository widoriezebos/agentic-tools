package dispatch

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"
)

// DesignCritiqueChain is one design-critic chain of one design document, as
// the job records hold it: its root, its newest round and whether the chain
// is closed.
type DesignCritiqueChain struct {
	Root        string
	Goal        string
	Closed      bool
	NewestJob   string
	NewestRound int64
	Newest      map[string]any
}

// DesignCritiqueChains selects the design-critic chains of one goal and one
// design document (its canonical path), oldest root first. The read is the
// same job-record state the critique owner locks and folds; it writes
// nothing.
func DesignCritiqueChains(repoRoot, goalID, designPath string) []DesignCritiqueChain {
	state := loadCritiqueState(repoRoot)
	canonical := designPath
	if resolved, err := filepath.EvalSymlinks(designPath); err == nil {
		canonical = resolved
	}
	var chains []DesignCritiqueChain
	for jobID, record := range state.records {
		if state.chainRoot(jobID) != jobID || asString(record["role"]) != "design-critic" {
			continue
		}
		recorded := asString(record["design"])
		if recorded != "" && !filepath.IsAbs(recorded) {
			// Dispatch records the design repository-relative, as the
			// critic's shared checkout names it.
			recorded = filepath.Join(checkoutTop(repoRoot), filepath.FromSlash(recorded))
		}
		if resolved, err := filepath.EvalSymlinks(recorded); err == nil {
			recorded = resolved
		}
		if recorded != canonical || goalID != "" && asString(record["goalId"]) != "" && asString(record["goalId"]) != goalID {
			continue
		}
		chain := DesignCritiqueChain{Root: jobID, Goal: asString(record["goalId"])}
		chain.Closed, _ = record["chainClosed"].(bool)
		for memberID, member := range state.records {
			if state.chainRoot(memberID) != jobID {
				continue
			}
			round, _ := strconv.ParseInt(asString(member["round"]), 10, 64)
			if number, ok := numInt(member["round"]); ok {
				round = number
			}
			if round >= chain.NewestRound {
				chain.NewestRound, chain.NewestJob, chain.Newest = round, memberID, member
			}
		}
		chains = append(chains, chain)
	}
	sort.Slice(chains, func(i, j int) bool { return chains[i].Root < chains[j].Root })
	return chains
}

// FindOperationRecord is the job record that carries an operation id, found
// by the same scan the launch claim uses; empty when none does.
func FindOperationRecord(repoRoot, operationID string) (string, map[string]any, error) {
	_, record, err := findOperationRecord(repoRoot, operationID, "")
	if record == nil || err != nil {
		return "", nil, err
	}
	return asString(record["jobId"]), record, nil
}

// checkoutTop is the Git checkout that holds dir: the nearest directory at
// or above it with a .git entry, or dir itself when there is none.
func checkoutTop(dir string) string {
	for probe := dir; ; probe = filepath.Dir(probe) {
		if _, err := os.Lstat(filepath.Join(probe, ".git")); err == nil {
			return probe
		}
		if probe == filepath.Dir(probe) {
			return dir
		}
	}
}
