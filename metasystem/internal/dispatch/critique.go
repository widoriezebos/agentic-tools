package dispatch

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	critiqueModel "github.com/widoriezebos/agentic-tools/metasystem/internal/critique"
)

const (
	CritiqueCapExhaustedExitCode = 10
	CritiqueCapExhaustedReason   = "cap-exhausted-human-raise"
)

const secondExhaustionRefused = "the review-round limit is exhausted with a severe or unproven finding open; only a person can go on"
const boundedExhaustionRefused = "the review-round limit is exhausted with bounded findings; close the critique register to defer them"

// DesignRoundLimit reads the admission's frozen allowance. Legacy roots use
// their retained review budget; severity never grants another examination.
func DesignRoundLimit(repoRoot, rootJob string, frozenLimit int64) int64 {
	if root, err := readObject(filepath.Join(repoRoot, "artifacts", "agents", "jobs", rootJob+".json")); err == nil {
		if value, present := root["designExaminationLimit"]; present {
			if limit, ok := numInt(value); ok && limit >= 0 && limit <= 4 {
				return limit
			}
			return 0
		}
		if limit, ok := numInt(root[reviewRoundLimitField]); ok {
			if asString(root["goalId"]) != "" || limit > 0 {
				return min(4, max(0, limit))
			}
		}
	}
	if frozenLimit <= 0 {
		return 4
	}
	return min(4, frozenLimit)
}

// critiqueState is the record table one critique decision reads: every
// parseable job record whose file name matches its own job identifier.
type critiqueState struct {
	agents  string
	records map[string]map[string]any
}

func loadCritiqueState(repoRoot string) critiqueState {
	return loadCritiqueStateAt(filepath.Join(repoRoot, "artifacts", "agents"))
}

func loadCritiqueStateAt(agents string) critiqueState {
	state, _ := readCritiqueStateAt(agents)
	return state
}

func readCritiqueStateAt(agents string) (critiqueState, error) {
	state := critiqueState{agents: agents, records: map[string]map[string]any{}}
	jobs := filepath.Join(agents, "jobs")
	entries, err := os.ReadDir(jobs)
	if os.IsNotExist(err) {
		return state, nil
	}
	if err != nil {
		return state, fmt.Errorf("cannot list job records in %s: %w", jobs, err)
	}
	var firstError error
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		path := filepath.Join(jobs, entry.Name())
		record, err := readObject(path)
		if err != nil {
			if firstError == nil {
				firstError = fmt.Errorf("cannot read job record %s: %w", path, err)
			}
			continue
		}
		stem := strings.TrimSuffix(filepath.Base(path), ".json")
		if asString(record["jobId"]) == stem {
			state.records[stem] = record
		}
	}
	return state, firstError
}

func (s critiqueState) chainRoot(job string) string {
	return lineageRoot(func(id string) (map[string]any, bool) {
		record, present := s.records[id]
		return record, present
	}, job)
}

func (s critiqueState) latestMember(chain string) map[string]any {
	var ids []string
	for id := range s.records {
		if s.chainRoot(id) == chain {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	var best map[string]any
	bestRound := int64(0)
	for _, id := range ids {
		round, ok := numInt(s.records[id]["round"])
		if !ok {
			continue
		}
		if best == nil || round > bestRound {
			best, bestRound = s.records[id], round
		}
	}
	return best
}

func exhaustions(record map[string]any) ([]map[string]any, error) {
	value, present := record["critiqueExhaustions"]
	if !present {
		return nil, nil
	}
	list, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("critiqueExhaustions is malformed; only a person can go on")
	}
	if len(list) > 1 {
		return nil, errors.New(secondExhaustionRefused)
	}
	var entries []map[string]any
	for _, item := range list {
		entry, ok := item.(map[string]any)
		if !ok {
			return nil, errors.New(secondExhaustionRefused)
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

type exhaustionBoundary int

const (
	noExhaustion exhaustionBoundary = iota
	boundedTerminalExhaustion
	severeTerminalExhaustion
)

type critiqueCapState struct {
	round    int64
	openIDs  []string
	boundary exhaustionBoundary
}

func readCritiqueCapState(repoRoot string, records critiqueState, rootJob string, root map[string]any) (critiqueCapState, error) {
	registerValue, present := root[findingRegisterField]
	if !present {
		return critiqueCapState{}, fmt.Errorf("critic chain has no canonical finding register")
	}
	register, err := decodeFindingRegister(registerValue)
	if err != nil {
		return critiqueCapState{}, fmt.Errorf("critic chain has a malformed finding register: %v", err)
	}
	round, err := findingRegisterRound(root, len(register))
	if err != nil {
		return critiqueCapState{}, fmt.Errorf("critic chain has malformed register round state: %v", err)
	}
	state := critiqueCapState{round: round, openIDs: openRegisterFindingIDs(register)}
	if len(state.openIDs) == 0 {
		return state, nil
	}
	accounting, err := critiqueRoundAccounting(repoRoot, records, rootJob, root)
	if err != nil {
		return critiqueCapState{}, malformedRoundAccounting(rootJob, err)
	}
	if asString(root["role"]) == "design-critic" {
		accounting.limit = DesignRoundLimit(repoRoot, rootJob, accounting.limit)
	}
	if accounting.consumed < accounting.limit {
		return state, nil
	}
	for _, finding := range register {
		if finding.Status != "open" && finding.Status != "disputed" {
			continue
		}
		if finding.RigorClass == critiqueModel.Severe || finding.RigorClass == critiqueModel.Unproven {
			state.boundary = severeTerminalExhaustion
			return state, nil
		}
	}
	state.boundary = boundedTerminalExhaustion
	return state, nil
}

func requireRegisterCaughtUp(state critiqueState, root string, capState critiqueCapState) error {
	latest := state.latestMember(root)
	if latest == nil {
		return fmt.Errorf("critic chain %s has no readable round records", root)
	}
	latestRound, ok := numInt(latest["round"])
	if !ok || latestRound < 1 {
		return fmt.Errorf("critic chain %s has an invalid latest round", root)
	}
	if latestRound != capState.round {
		repoRoot := filepath.Dir(filepath.Dir(state.agents))
		return fmt.Errorf("the findings of critique %s are recorded up to round %d, but round %d exists\nfirst run job critique-register-advance --repo %s --root-job %s --round-job %s", root, capState.round, latestRound, repoRoot, root, asString(latest["jobId"]))
	}
	return nil
}

func terminalCapError(cap critiqueCapState) error {
	switch cap.boundary {
	case boundedTerminalExhaustion:
		return &OpError{Code: CritiqueCapExhaustedExitCode, Reason: CritiqueCapExhaustedReason,
			Message: fmt.Sprintf("%s at round %d with open finding identifiers: %s", boundedExhaustionRefused, cap.round, strings.Join(cap.openIDs, ", "))}
	case severeTerminalExhaustion:
		return &OpError{Code: CritiqueCapExhaustedExitCode, Reason: CritiqueCapExhaustedReason,
			Message: fmt.Sprintf("%s at terminal round %d with open finding identifiers: %s", secondExhaustionRefused, cap.round, strings.Join(cap.openIDs, ", "))}
	default:
		return nil
	}
}

// CritiqueExhaustionAdvance checks canonical critic registers before a
// successor reservation. A terminal bounded or severe exhaustion refuses;
// neither outcome authorizes another critique round.
func CritiqueExhaustionAdvance(repoRoot, rootJob, role, messagePath, successor string) (outcome string, err error) {
	_, err = os.ReadFile(messagePath)
	if err != nil {
		return "", fmt.Errorf("critique exhaustion successor message is unreadable: %v", err)
	}
	return withFindingRegisterLock(repoRoot, func() (string, error) {
		state := loadCritiqueState(repoRoot)

		inspect := func(criticRoot string) (critiqueCapState, error) {
			root, present := state.records[criticRoot]
			if !present {
				return critiqueCapState{}, fmt.Errorf("critique root record %s is unreadable", criticRoot)
			}
			capState, capErr := readCritiqueCapState(repoRoot, state, criticRoot, root)
			if capErr != nil {
				return critiqueCapState{}, capErr
			}
			if caughtUpErr := requireRegisterCaughtUp(state, criticRoot, capState); caughtUpErr != nil {
				return critiqueCapState{}, caughtUpErr
			}
			if _, exhaustionErr := exhaustions(root); exhaustionErr != nil {
				return critiqueCapState{}, exhaustionErr
			}
			return capState, nil
		}

		switch role {
		case "design-critic", "code-critic", "warden":
			if role == "design-critic" {
				if decision, ok := state.records[rootJob]["designDecision"].(map[string]any); ok && asString(decision["decision"]) != "continue" {
					return "", fmt.Errorf("the design examination cannot continue; run metasystem design review '%s' --dispositions FILE", strings.ReplaceAll(asString(decision["scope"]), "'", "'\\''"))
				}
			}
			capState, inspectErr := inspect(rootJob)
			if inspectErr != nil {
				return "", inspectErr
			}
			if terminalErr := terminalCapError(capState); terminalErr != nil {
				if role == "design-critic" {
					return "", designCapHumanRaise(asString(state.records[rootJob]["design"]), capState.round, capState.openIDs)
				}
				return "", terminalErr
			}
			return "none", nil

		case "implementer":
			implementationIDs := map[string]bool{}
			for id := range state.records {
				if state.chainRoot(id) == rootJob {
					implementationIDs[id] = true
				}
			}
			var criticIDs []string
			for id, record := range state.records {
				criticRole := asString(record["role"])
				if (criticRole == "code-critic" || criticRole == "warden") && record["parentJob"] == nil && implementationIDs[asString(record["reviews"])] {
					criticIDs = append(criticIDs, id)
				}
			}
			sort.Strings(criticIDs)
			for _, criticID := range criticIDs {
				capState, inspectErr := inspect(criticID)
				if inspectErr != nil {
					return "", inspectErr
				}
				if terminalErr := terminalCapError(capState); terminalErr != nil {
					return "", terminalErr
				}
			}
			return "none", nil

		default:
			return "", fmt.Errorf("critique exhaustion has no rule for role %s", role)
		}
	})
}
