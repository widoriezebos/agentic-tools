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

const secondExhaustionRefused = "the review-round limit is exhausted with a severe or unproven finding; waiting on the human is the only remedy"
const boundedExhaustionRefused = "the review-round limit is exhausted with bounded findings; close the critique register to defer them"

// critiqueState is the record table one critique decision reads: every
// parseable job record whose file name matches its own job identifier.
type critiqueState struct {
	agents  string
	records map[string]map[string]any
}

func loadCritiqueState(repoRoot string) critiqueState {
	agents := filepath.Join(repoRoot, "artifacts", "agents")
	state := critiqueState{agents: agents, records: map[string]map[string]any{}}
	paths, _ := filepath.Glob(filepath.Join(agents, "jobs", "*.json"))
	for _, path := range paths {
		record, err := readObject(path)
		if err != nil {
			continue
		}
		stem := strings.TrimSuffix(filepath.Base(path), ".json")
		if asString(record["jobId"]) == stem {
			state.records[stem] = record
		}
	}
	return state
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
		return nil, fmt.Errorf("critiqueExhaustions is malformed; waiting on the human is the only remedy")
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
		return fmt.Errorf("critic chain %s has folded through round %d but its latest record is round %d; advance the canonical register before reading exhaustion", root, capState.round, latestRound)
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
			capState, inspectErr := inspect(rootJob)
			if inspectErr != nil {
				return "", inspectErr
			}
			if terminalErr := terminalCapError(capState); terminalErr != nil {
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
