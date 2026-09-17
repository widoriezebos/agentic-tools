package branch

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

const LandTrunkRedCode = "GOAL_LAND_TRUNK_RED"

type DiagnosticRun struct {
	Goal, Tree string
	Groups     []string
	Purpose    string
	Mode       string
	NoReuse    bool
}

type DiagnosticResult struct {
	AttemptID string
	Green     bool
}

type RedRunner interface {
	Run(DiagnosticRun) (DiagnosticResult, error)
}

type TrunkRedEntry struct {
	Goal, Branch, Commit, Attempt string
	Groups                        []string
	Open                          bool
}

type TrunkRedRecorder interface {
	RecordTrunkRed(TrunkRedEntry) error
}

type LandingProgressRecorder interface {
	RecordLandingProgress(recordLine, nextStep string) error
}

type RedRequest struct {
	Goal, Endpoint, Branch, BranchTip, LastUnit string
	Proof                                       LandingProof
	FailingGroups                               []string
	ChangeSet                                   []string
	Contract                                    testpolicy.Contract
	AdmitDiagnostic                             func() error
	Runner                                      RedRunner
	TrunkRed                                    TrunkRedRecorder
	Progress                                    LandingProgressRecorder
}

type RedResult struct {
	Classification, EndpointAttempt string
	ProofNumber                     int
}

func LandingChangeSet(repo, endpoint, branchTip, goal, lastUnit string) ([]string, error) {
	status, err := InspectStatus(repo, endpoint, branchTip, goal)
	if err != nil {
		return nil, err
	}
	count := 0
	for index := 0; index < status.Prefix; index++ {
		if status.Units[index].Unit == lastUnit {
			count = index + 1
			break
		}
	}
	if count == 0 {
		return nil, fmt.Errorf("landing change set has no land-ready unit %s", lastUnit)
	}
	paths := map[string]bool{}
	for _, group := range landingUnits(status, count) {
		commits := append([]Commit(nil), group.folds...)
		commits = append(commits, Commit{ID: group.status.Commit})
		for _, commit := range commits {
			entries, err := RawEntries(repo, commit.ID)
			if err != nil {
				return nil, err
			}
			for _, entry := range entries {
				paths[entry.Path] = true
			}
		}
	}
	changed := make([]string, 0, len(paths))
	for path := range paths {
		changed = append(changed, path)
	}
	sort.Strings(changed)
	return changed, nil
}

func manifestNamesPath(pattern, candidate string) bool {
	pattern = filepath.ToSlash(strings.TrimPrefix(pattern, "./"))
	candidate = filepath.ToSlash(strings.TrimPrefix(candidate, "./"))
	if pattern == candidate {
		return true
	}
	if prefix, ok := strings.CutSuffix(pattern, "/**"); ok {
		return candidate == prefix || strings.HasPrefix(candidate, prefix+"/")
	}
	matched, _ := filepath.Match(pattern, candidate)
	return matched
}

func groupInputs(contract testpolicy.Contract, groupID string) ([]string, bool) {
	for _, group := range contract.Groups {
		if group.ID == groupID {
			return group.Inputs, true
		}
	}
	return nil, false
}

func failuresBelongToGoal(contract testpolicy.Contract, groups, changed []string) (bool, error) {
	for _, groupID := range groups {
		inputs, found := groupInputs(contract, groupID)
		if !found {
			return false, fmt.Errorf("testing contract has no failing group %s", groupID)
		}
		for _, input := range inputs {
			for _, path := range changed {
				if manifestNamesPath(input, path) {
					return true, nil
				}
			}
		}
	}
	return false, nil
}

func landingNextStep(goal, unit string, proof LandingProof) string {
	groups := "none"
	if len(proof.Groups) != 0 {
		groups = strings.Join(proof.Groups, ",")
	}
	fix := proof.Fix
	if fix == "" {
		fix = "pending"
	}
	if proof.Verdict == "green" {
		return fmt.Sprintf("LANDED %s through %s after %d proofs", goal, unit, proof.Number)
	}
	return fmt.Sprintf("landing %s through %s: proof %d of %s red on %s, fix %s", goal, unit, proof.Number, proof.Candidate, groups, fix)
}

func recordLandingProgress(recorder LandingProgressRecorder, goal, unit string, proof LandingProof) error {
	if recorder == nil {
		return fmt.Errorf("landing progress recorder is unavailable")
	}
	return recorder.RecordLandingProgress(RenderLandingProof(proof), landingNextStep(goal, unit, proof))
}

func HandleLandingRed(req RedRequest) (RedResult, error) {
	if req.Goal == "" || !hex40(req.Endpoint) || !hex40(req.BranchTip) || req.Proof.Number < 1 ||
		!hex40(req.Proof.Candidate) || !hex40(req.Proof.Landing) || len(req.FailingGroups) == 0 {
		return RedResult{}, fmt.Errorf("landing red classification needs the goal, endpoint, branch, proof, and failing groups")
	}
	groups := append([]string(nil), req.FailingGroups...)
	sort.Strings(groups)
	belongs, err := failuresBelongToGoal(req.Contract, groups, req.ChangeSet)
	if err != nil {
		return RedResult{}, err
	}
	proof := req.Proof
	proof.Verdict = "red"
	proof.Groups = groups
	if belongs {
		if err := recordLandingProgress(req.Progress, req.Goal, req.LastUnit, proof); err != nil {
			return RedResult{}, err
		}
		return RedResult{Classification: "goal-red", ProofNumber: proof.Number}, nil
	}
	if req.AdmitDiagnostic == nil {
		return RedResult{}, fmt.Errorf("diagnostic admission is unavailable")
	}
	if err := req.AdmitDiagnostic(); err != nil {
		return RedResult{}, err
	}
	if req.Runner == nil {
		return RedResult{}, fmt.Errorf("diagnostic runner is unavailable")
	}
	diagnostic, err := req.Runner.Run(DiagnosticRun{Goal: req.Goal, Tree: req.Endpoint, Groups: groups,
		Purpose: "diagnostic", Mode: "canary", NoReuse: true})
	if err != nil {
		return RedResult{}, err
	}
	if diagnostic.AttemptID == "" {
		return RedResult{}, fmt.Errorf("diagnostic runner returned no attempt id")
	}
	if diagnostic.Green {
		if err := recordLandingProgress(req.Progress, req.Goal, req.LastUnit, proof); err != nil {
			return RedResult{}, err
		}
		return RedResult{Classification: "goal-red", EndpointAttempt: diagnostic.AttemptID, ProofNumber: proof.Number}, nil
	}
	if req.TrunkRed == nil {
		return RedResult{}, fmt.Errorf("trunk-red ledger owner is unavailable")
	}
	entry := TrunkRedEntry{Goal: req.Goal, Branch: req.Branch, Commit: req.BranchTip,
		Attempt: diagnostic.AttemptID, Groups: groups, Open: true}
	if err := req.TrunkRed.RecordTrunkRed(entry); err != nil {
		return RedResult{}, err
	}
	if err := recordLandingProgress(req.Progress, req.Goal, req.LastUnit, proof); err != nil {
		return RedResult{}, err
	}
	return RedResult{Classification: "trunk-red", EndpointAttempt: diagnostic.AttemptID, ProofNumber: proof.Number},
		operationRefusal(LandTrunkRedCode, "groups %s are red on endpoint %s; the landing remains on %s", strings.Join(groups, ","), req.Endpoint, req.Branch)
}

func RecordLandingGreen(recorder LandingProgressRecorder, goal, lastUnit string, proof LandingProof) error {
	proof.Verdict = "green"
	proof.Groups = nil
	return recordLandingProgress(recorder, goal, lastUnit, proof)
}
