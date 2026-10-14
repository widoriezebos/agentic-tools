package dispatch

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/project"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/readsubject"
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
// design record through its path aliases, oldest root first. The read is the
// same job-record state the critique owner locks and folds; it writes
// nothing.
func DesignCritiqueChains(repoRoot, goalID, designPath string) []DesignCritiqueChain {
	state := loadCritiqueState(repoRoot)
	return designCritiqueChains(state, repoRoot, goalID, designPath, goalID == "")
}

// ReadDesignCritiqueChains returns the readable chains and reports the first job
// record that could not be read or parsed, naming the file so a gate can distinguish
// absent critique evidence from a broken check.
func ReadDesignCritiqueChains(repoRoot, goalID, designPath string) ([]DesignCritiqueChain, error) {
	state, err := readCritiqueStateAt(filepath.Join(repoRoot, "artifacts", "agents"))
	chains := designCritiqueChains(state, repoRoot, goalID, designPath, false)
	for _, chain := range chains {
		id, record := chain.Root, state.records[chain.Root]
		if err != nil || state.chainRoot(id) != id || asString(record["role"]) != "design-critic" || asString(record["goalId"]) != goalID || NeverLaunched(record) {
			continue
		}
		_, present, subjectErr := readsubject.ReadRoundSubject(state.agents, id, 1)
		if subjectErr != nil {
			err = fmt.Errorf("design root %s has unreadable canonical subject: %w", id, subjectErr)
		} else if !present {
			recorded := asString(record["design"])
			if !filepath.IsAbs(recorded) {
				recorded = filepath.Join(checkoutTop(repoRoot), filepath.FromSlash(recorded))
			}
			if _, problem := os.Stat(recorded); problem != nil {
				err = fmt.Errorf("design root %s has neither a frozen identity nor its original page: %w", id, problem)
			}
		}
	}
	return chains, err
}

func designCritiqueChains(state critiqueState, repoRoot, goalID, designPath string, anyGoal bool) []DesignCritiqueChain {
	var designID string
	if data, err := os.ReadFile(designPath); err == nil {
		if record, problems, present := project.ParseRecord(designPath, string(data)); present && len(problems) == 0 && record.Kind == project.KindDesign {
			designID = record.ID
		}
	}
	aliases := map[string]bool{}
	if designID != "" {
		path := filepath.Join(state.agents, "intent-review", "design-"+strings.ToLower(designID), "chain.json")
		var entry struct{ Goal, Design string }
		if data, err := os.ReadFile(path); err == nil && json.Unmarshal(data, &entry) == nil && (anyGoal || entry.Goal == goalID) {
			aliases[entry.Design] = true
		}
	}
	canonical := designPath
	if resolved, err := filepath.EvalSymlinks(designPath); err == nil {
		canonical = resolved
	}
	var chains []DesignCritiqueChain
	for jobID, record := range state.records {
		if state.chainRoot(jobID) != jobID || asString(record["role"]) != "design-critic" {
			continue
		}
		// A root refused at setup never ran (the setup-refusal-release
		// rule): it is no critique of the design, so a later review or close
		// passes it by.
		if NeverLaunched(record) {
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
		sameRecord := false
		if subject, present, err := readsubject.ReadRoundSubject(state.agents, jobID, 1); err == nil && present && subject.DesignPage != "" {
			frozen, problems, present := project.ParseRecord(subject.DesignPath, subject.DesignPage)
			sameRecord = present && len(problems) == 0 && frozen.ID == designID && designID != ""
		}
		if !sameRecord && designID != "" {
			retained, _ := record["read"].(map[string]any)
			identity, _ := retained["design"].(map[string]any)
			sameRecord = asString(identity["recordId"]) == designID && asString(identity["root"]) == jobID
		}
		if (!sameRecord && recorded != canonical && !aliases[recorded]) || !anyGoal && asString(record["goalId"]) != goalID {
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
