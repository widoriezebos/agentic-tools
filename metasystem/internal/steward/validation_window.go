package steward

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/run"
)

const directValidationWindowSize = 2

var retiredCatchClassSections = []string{
	"go-engine-gate",
	"static-contract-audits",
	"supervision-and-census-fixtures",
	"dispatcher-adapter-and-mission-runner-fixtures",
	"adoption-fixtures",
	"witness-gate-fixtures",
}

type validationWindowObservation struct {
	RunID       string   `json:"runId"`
	StageLedger string   `json:"stageLedger"`
	ObservedAt  string   `json:"observedAt"`
	Missing     []string `json:"missingSections"`
	NonGreen    []string `json:"nonGreenSections"`
}

type validationWindowState struct {
	Schema       int                           `json:"schema"`
	Custodian    string                        `json:"custodian"`
	Observer     string                        `json:"observer"`
	WindowSize   int                           `json:"windowSize"`
	CatchClasses []string                      `json:"catchClassSectionIds"`
	Observations []validationWindowObservation `json:"observations"`
}

func validationWindowPath(repoRoot string) string {
	return filepath.Join(repoRoot, "artifacts", "agents", "steward", "direct-validation-window.json")
}

func newValidationWindow() validationWindowState {
	return validationWindowState{Schema: 1, Custodian: "Wido", Observer: "steward",
		WindowSize: directValidationWindowSize, CatchClasses: append([]string(nil), retiredCatchClassSections...)}
}

func loadValidationWindow(repoRoot string) (validationWindowState, error) {
	data, err := os.ReadFile(validationWindowPath(repoRoot))
	if os.IsNotExist(err) {
		return newValidationWindow(), nil
	}
	if err != nil {
		return validationWindowState{}, err
	}
	state := validationWindowState{}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&state); err != nil {
		return validationWindowState{}, err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF || state.Schema != 1 || state.WindowSize != directValidationWindowSize ||
		state.Custodian != "Wido" || state.Observer != "steward" || strings.Join(state.CatchClasses, "\x00") != strings.Join(retiredCatchClassSections, "\x00") {
		return validationWindowState{}, fmt.Errorf("direct-validation observation window has an unknown schema or contract")
	}
	return state, nil
}

func saveValidationWindow(repoRoot string, state validationWindowState) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	durable, err := atomicfile.WriteText(validationWindowPath(repoRoot), string(data)+"\n", repoRoot)
	if err != nil {
		return err
	}
	if !durable {
		return fmt.Errorf("direct-validation observation window durability is unknown")
	}
	return nil
}

func stageLedgerFromLog(repoRoot, logPath string) (string, error) {
	if !filepath.IsAbs(logPath) {
		logPath = filepath.Join(repoRoot, logPath)
	}
	data, err := os.ReadFile(logPath)
	if err != nil {
		return "", err
	}
	ledger := ""
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "stage results: ") {
			ledger = strings.TrimSpace(strings.TrimPrefix(line, "stage results: "))
		}
	}
	if err := scanner.Err(); err != nil || ledger == "" {
		return "", fmt.Errorf("the direct validator log has no readable stage-results path")
	}
	if !filepath.IsAbs(ledger) {
		ledger = filepath.Join(repoRoot, ledger)
	}
	ledger, err = filepath.Abs(ledger)
	if err != nil {
		return "", err
	}
	wantRoot, err := filepath.Abs(filepath.Join(repoRoot, "artifacts", "agents", "validation-stage-results"))
	if err != nil || (ledger != wantRoot && !strings.HasPrefix(ledger, wantRoot+string(filepath.Separator))) {
		return "", fmt.Errorf("stage-results path is outside the validator ledger directory")
	}
	return ledger, nil
}

func compareStageLedger(path string) (missing, nonGreen []string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return append([]string(nil), retiredCatchClassSections...), []string{"ledger-unavailable"}
	}
	statuses := map[string]string{}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Split(line, "\t")
		if len(fields) >= 3 && fields[0] == "section" {
			statuses[fields[1]] = fields[2]
		}
	}
	for _, id := range retiredCatchClassSections {
		status, present := statuses[id]
		if !present {
			missing = append(missing, id)
		} else if status != "pass" {
			nonGreen = append(nonGreen, id+"="+status)
		}
	}
	return missing, nonGreen
}

func observeDirectValidationWindow(repoRoot string, now time.Time) error {
	state, err := loadValidationWindow(repoRoot)
	if err != nil || len(state.Observations) >= state.WindowSize {
		return err
	}
	seen := map[string]bool{}
	for _, observation := range state.Observations {
		seen[observation.RunID] = true
	}
	store := &run.Store{Root: repoRoot}
	var candidates []*run.Record
	for _, path := range run.RecordFiles(repoRoot) {
		id := strings.TrimSuffix(filepath.Base(path), ".json")
		record, readErr := store.Read(id)
		if readErr == nil && record != nil && (record.Status == run.StatusGreen || record.Status == run.StatusRed) && record.Kind == "suite" &&
			record.Display == "weight-triggered direct validation" && record.Governed != nil && !seen[id] {
			candidates = append(candidates, record)
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		return *candidates[i].TerminalSeq < *candidates[j].TerminalSeq
	})
	changed := false
	for _, record := range candidates {
		if len(state.Observations) >= state.WindowSize {
			break
		}
		ledger, ledgerErr := stageLedgerFromLog(repoRoot, record.Log)
		if ledgerErr != nil {
			continue
		}
		observation := validationWindowObservation{RunID: record.RunId, StageLedger: ledger, ObservedAt: now.UTC().Format(time.RFC3339)}
		observation.Missing, observation.NonGreen = compareStageLedger(ledger)
		if len(observation.NonGreen) == 1 && observation.NonGreen[0] == "ledger-unavailable" {
			continue
		}
		state.Observations = append(state.Observations, observation)
		changed = true
	}
	if !changed {
		return nil
	}
	return saveValidationWindow(repoRoot, state)
}

func directValidationWindowFailures(repoRoot string) []string {
	state, err := loadValidationWindow(repoRoot)
	if err != nil {
		return []string{"direct-validation observation window unavailable: " + err.Error()}
	}
	var failures []string
	for _, observation := range state.Observations {
		if len(observation.Missing) > 0 || len(observation.NonGreen) > 0 {
			failures = append(failures, fmt.Sprintf("direct validation %s catch-class diff missing=%v nonGreen=%v",
				observation.RunID, observation.Missing, observation.NonGreen))
		}
	}
	return failures
}
