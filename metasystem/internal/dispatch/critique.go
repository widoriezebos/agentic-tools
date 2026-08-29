package dispatch

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	critiqueModel "github.com/widoriezebos/agentic-tools/metasystem/internal/critique"
)

const (
	firstSevereExhaustionRound = int64(3)
	terminalExhaustionRound    = int64(6)
	boundedFurtherRounds       = int64(2)

	CritiqueCapExhaustedExitCode = 10
	CritiqueCapExhaustedReason   = "cap-exhausted-human-raise"
)

const secondExhaustionRefused = "a second critique exhaustion is refused outright; waiting on the human is the only remedy"
const boundedExhaustionRefused = "the bounded critique cap is exhausted; further critique is refused and waiting on the human is the only remedy"

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

func requireEnumeration(message string, openIDs []string) error {
	var missing []string
	for _, id := range openIDs {
		pattern := `(?:^|[^A-Za-z0-9_-])` + regexp.QuoteMeta(id) + `(?:$|[^A-Za-z0-9_-])`
		if !regexp.MustCompile(pattern).MatchString(message) {
			missing = append(missing, id)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("critique budget exhausted; the implementer or design successor follow-up "+
			"must enumerate every open finding identifier: %s", strings.Join(missing, ", "))
	}
	return nil
}

type exhaustionBoundary int

const (
	noExhaustion exhaustionBoundary = iota
	firstSevereExhaustion
	boundedTerminalExhaustion
	severeTerminalExhaustion
)

type critiqueCapState struct {
	round    int64
	openIDs  []string
	boundary exhaustionBoundary
}

func readCritiqueCapState(root map[string]any) (critiqueCapState, error) {
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
	for _, finding := range register {
		if finding.RigorClass == critiqueModel.Severe || finding.RigorClass == critiqueModel.Unproven {
			if round >= terminalExhaustionRound {
				state.boundary = severeTerminalExhaustion
			} else if round >= firstSevereExhaustionRound {
				state.boundary = firstSevereExhaustion
			}
			return state, nil
		}
	}
	start, _, err := boundedCritiqueStart(root)
	if err != nil {
		return critiqueCapState{}, err
	}
	if start == 0 {
		return critiqueCapState{}, fmt.Errorf("an all-bounded critic register has no recorded first all-bounded round")
	}
	if round >= start+boundedFurtherRounds {
		state.boundary = boundedTerminalExhaustion
	}
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

type firstExhaustionWrite struct {
	root    string
	round   int64
	openIDs []string
}

// CritiqueExhaustionAdvance consumes canonical critic registers and writes a
// first severe exhaustion directly on the affected chain root. Terminal
// bounded and second severe exhaustion always refuse; neither outcome
// authorizes another critique round.
func CritiqueExhaustionAdvance(repoRoot, rootJob, role, messagePath, successor string) (outcome string, err error) {
	messageBytes, err := os.ReadFile(messagePath)
	if err != nil {
		return "", fmt.Errorf("critique exhaustion successor message is unreadable: %v", err)
	}
	message := string(messageBytes)
	return withFindingRegisterLock(repoRoot, func() (string, error) {
		state := loadCritiqueState(repoRoot)
		var writes []firstExhaustionWrite

		inspect := func(criticRoot string) (critiqueCapState, []map[string]any, error) {
			root, present := state.records[criticRoot]
			if !present {
				return critiqueCapState{}, nil, fmt.Errorf("critique root record %s is unreadable", criticRoot)
			}
			capState, capErr := readCritiqueCapState(root)
			if capErr != nil {
				return critiqueCapState{}, nil, capErr
			}
			if caughtUpErr := requireRegisterCaughtUp(state, criticRoot, capState); caughtUpErr != nil {
				return critiqueCapState{}, nil, caughtUpErr
			}
			previous, previousErr := exhaustions(root)
			return capState, previous, previousErr
		}

		switch role {
		case "design-critic":
			capState, previous, inspectErr := inspect(rootJob)
			if inspectErr != nil {
				return "", inspectErr
			}
			if terminalErr := terminalCapError(capState); terminalErr != nil {
				return "", terminalErr
			}
			if capState.boundary != firstSevereExhaustion {
				return "none", nil
			}
			if len(previous) > 0 {
				if roundOf(previous[0]) == capState.round && asString(previous[0]["successorJobId"]) == successor {
					return "unchanged", nil
				}
				if roundOf(previous[0]) < capState.round {
					return "none", nil
				}
				return "", errors.New(secondExhaustionRefused)
			}
			if enumerationErr := requireEnumeration(message, capState.openIDs); enumerationErr != nil {
				return "", enumerationErr
			}
			writes = append(writes, firstExhaustionWrite{rootJob, capState.round, capState.openIDs})

		case "code-critic", "warden":
			capState, previous, inspectErr := inspect(rootJob)
			if inspectErr != nil {
				return "", inspectErr
			}
			if terminalErr := terminalCapError(capState); terminalErr != nil {
				return "", terminalErr
			}
			if capState.boundary != firstSevereExhaustion {
				return "none", nil
			}
			if len(previous) == 0 {
				return "", fmt.Errorf("%s critique budget exhausted; dispatch an implementer follow-up that enumerates every open finding identifier before continuing the critic chain: %s", role, strings.Join(capState.openIDs, ", "))
			}
			if roundOf(previous[0]) > capState.round {
				return "", errors.New(secondExhaustionRefused)
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
				capState, previous, inspectErr := inspect(criticID)
				if inspectErr != nil {
					return "", inspectErr
				}
				if terminalErr := terminalCapError(capState); terminalErr != nil {
					return "", terminalErr
				}
				if capState.boundary != firstSevereExhaustion {
					continue
				}
				if len(previous) > 0 {
					if roundOf(previous[0]) <= capState.round {
						continue
					}
					return "", errors.New(secondExhaustionRefused)
				}
				if enumerationErr := requireEnumeration(message, capState.openIDs); enumerationErr != nil {
					return "", enumerationErr
				}
				writes = append(writes, firstExhaustionWrite{criticID, capState.round, capState.openIDs})
			}

		default:
			return "", fmt.Errorf("critique exhaustion has no rule for role %s", role)
		}

		if len(writes) == 0 {
			return "none", nil
		}
		for _, write := range writes {
			writeErr := withRecordLock(repoRoot, write.root, func(recordPath string) error {
				record, readErr := readObject(recordPath)
				if readErr != nil {
					return readErr
				}
				previous, previousErr := exhaustions(record)
				if previousErr != nil {
					return previousErr
				}
				if len(previous) > 0 {
					if roundOf(previous[0]) == write.round && asString(previous[0]["successorJobId"]) == successor {
						return nil
					}
					return errors.New(secondExhaustionRefused)
				}
				ids := make([]any, len(write.openIDs))
				for index, id := range write.openIDs {
					ids[index] = id
				}
				record["critiqueExhaustions"] = []any{map[string]any{
					"round": write.round, "openFindingIds": ids, "successorJobId": successor,
				}}
				return writeRecord(recordPath, record)
			})
			if writeErr != nil {
				return "", writeErr
			}
		}
		return "recorded", nil
	})
}

func roundOf(entry map[string]any) int64 {
	if round, ok := numInt(entry["round"]); ok {
		return round
	}
	return -1
}
