// Package spend measures machine-local token and monetary usage without
// controlling admission. It owns the daily ledger consumed by steward health.
package spend

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/mission"
)

// Tokens keeps the four independently priced token classes.
type Tokens struct {
	Input     float64 `json:"input"`
	Cached    float64 `json:"cached"`
	Output    float64 `json:"output"`
	Reasoning float64 `json:"reasoning"`
}

// Total is the ceiling quantity: every token class counts.
func (t Tokens) Total() float64 { return t.Input + t.Cached + t.Output + t.Reasoning }

// Row is one aggregate tuple from the design's goal, machine, day, runtime,
// and canonical-model axes.
type Row struct {
	Goal          string  `json:"goal"`
	Machine       string  `json:"machine"`
	Day           string  `json:"day"`
	Runtime       string  `json:"runtime"`
	Model         string  `json:"model"`
	Tokens        Tokens  `json:"tokens"`
	Money         float64 `json:"money"`
	PricedRecords int     `json:"pricedRecords"`
	Unpriced      int     `json:"unpriced"`
	Foreign       int     `json:"foreign"`
}

// UnmeasuredEntry makes every record or transcript request whose spend could
// not enter a total visible in the ledger.
type UnmeasuredEntry struct {
	ID         string `json:"id"`
	File       string `json:"file,omitempty"`
	Goal       string `json:"goal,omitempty"`
	Machine    string `json:"machine,omitempty"`
	Day        string `json:"day,omitempty"`
	Provenance string `json:"provenance"`
	Detail     string `json:"detail"`
}

// ScopeSummary is one health-line scope, with monetary uncertainty kept
// beside the floor rather than represented as zero certainty.
type ScopeSummary struct {
	ID         string  `json:"id"`
	Goal       string  `json:"goal,omitempty"`
	Machine    string  `json:"machine"`
	Day        string  `json:"day,omitempty"`
	Tokens     float64 `json:"tokens"`
	Money      float64 `json:"money"`
	Unpriced   int     `json:"unpriced"`
	Foreign    int     `json:"foreign"`
	Unmeasured int     `json:"unmeasured"`
	Unreadable int     `json:"unreadable"`
	Inflight   int     `json:"inflight,omitempty"`
}

// SeatSummary discloses transcript coverage and every known attribution gap.
type SeatSummary struct {
	DayTokens            float64 `json:"dayTokens"`
	LifetimeTokens       float64 `json:"lifetimeTokens"`
	Files                int     `json:"files"`
	AgedFiles            int     `json:"agedFiles"`
	UnmeasuredRequests   int     `json:"unmeasuredRequests"`
	UnattributedRequests int     `json:"unattributedRequests"`
	CodexUnmeasured      bool    `json:"codexUnmeasured"`
}

// Ledger is the complete daily machine observation written by Measure.
type Ledger struct {
	SchemaVersion int                     `json:"schemaVersion"`
	ObservedAt    time.Time               `json:"observedAt"`
	Day           string                  `json:"day"`
	Machine       string                  `json:"machine"`
	Currency      string                  `json:"currency"`
	Settings      config.SpendSettings    `json:"-"`
	Rows          []Row                   `json:"rows"`
	DayScope      ScopeSummary            `json:"dayScope"`
	GoalScopes    map[string]ScopeSummary `json:"goalScopes"`
	ClaimedGoals  []string                `json:"claimedGoals"`
	Seat          SeatSummary             `json:"seat"`
	Unmeasured    []UnmeasuredEntry       `json:"unmeasured"`
	Inflight      []string                `json:"inflight"`
}

type pricedMeasurement struct {
	goal, machine, day, runtime, model string
	tokens                             Tokens
	money                              float64
	priced, unpriced, foreign          int
	dayEligible                        bool
}

// Path returns the daily spend ledger location.
func Path(repoRoot string, now time.Time) string {
	return filepath.Join(repoRoot, "artifacts", "agents", "steward", "spend", now.UTC().Format("2006-01-02")+".json")
}

// Measure reads every job record and eligible Claude transcript, prices each
// record independently, builds both machine-day and lifetime goal scopes, and
// publishes the daily ledger only when its content changed.
func Measure(repoRoot, machine string, now time.Time) (Ledger, error) {
	now = now.UTC()
	settings, err := config.ReadSpendSettings(filepath.Join(repoRoot, "metasystem.conf"))
	if err != nil {
		return Ledger{}, err
	}
	claimed, err := claimedGoals(repoRoot, machine)
	if err != nil {
		return Ledger{}, err
	}
	ledger := Ledger{
		SchemaVersion: 1, ObservedAt: now, Day: now.Format("2006-01-02"), Machine: machine,
		Currency: settings.Currency, Settings: settings, GoalScopes: map[string]ScopeSummary{},
		ClaimedGoals: claimed, Unmeasured: []UnmeasuredEntry{}, Inflight: []string{},
	}

	jobsDir := filepath.Join(repoRoot, "artifacts", "agents", "jobs")
	entries, err := os.ReadDir(jobsDir)
	if err != nil && !os.IsNotExist(err) {
		return Ledger{}, fmt.Errorf("cannot list jobs directory %s: %w", jobsDir, err)
	}
	delegateSessions := map[string]bool{}
	var measured []pricedMeasurement
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		recordPath := filepath.Join(jobsDir, entry.Name())
		measurement := mission.JobUsageAt(repoRoot, recordPath)
		if measurement.Record == nil {
			ledger.Unmeasured = append(ledger.Unmeasured, UnmeasuredEntry{
				ID: entry.Name(), File: relativePath(repoRoot, recordPath), Provenance: "unreadable", Detail: fmt.Sprint(measurement.Detail),
			})
			continue
		}
		record := measurement.Record
		for _, key := range []string{"sessionId", "resumedSessionId"} {
			if session, _ := record[key].(string); session != "" {
				delegateSessions[session] = true
			}
		}
		status, _ := record["status"].(string)
		id, _ := record["jobId"].(string)
		if id == "" {
			id = strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
		}
		goalID := textOr(record["goalId"], "none")
		recordMachine := textOr(record["machineId"], machine)
		started, stampErr := parseTime(record["startedAt"])
		if !terminalStatus(status) {
			ledger.Inflight = append(ledger.Inflight, id)
			continue
		}
		if stampErr != nil {
			ledger.Unmeasured = append(ledger.Unmeasured, UnmeasuredEntry{
				ID: id, Goal: goalID, Machine: recordMachine, Provenance: "unavailable", Detail: "no startedAt",
			})
			continue
		}
		day := started.UTC().Format("2006-01-02")
		if measurement.Provenance != "reported" && measurement.Provenance != "derived" {
			ledger.Unmeasured = append(ledger.Unmeasured, UnmeasuredEntry{
				ID: id, File: relativePath(repoRoot, recordPath), Goal: goalID, Machine: recordMachine,
				Day: day, Provenance: measurement.Provenance, Detail: fmt.Sprint(measurement.Detail),
			})
			continue
		}
		tokens := tokensFromMission(measurement.Tokens)
		runtime := textOr(record["runtime"], "unknown")
		model := textOr(record["canonicalModelKey"], "unknown")
		money, priced, unpriced, foreign := price(runtime, model, measurement.Tokens, measurement.Cost, measurement.ProviderUnit != nil, settings)
		measured = append(measured, pricedMeasurement{
			goal: goalID, machine: recordMachine, day: day, runtime: runtime, model: model,
			tokens: tokens, money: money, priced: priced, unpriced: unpriced, foreign: foreign, dayEligible: true,
		})
	}

	seatRows, seat, seatUnmeasured, err := readSeat(repoRoot, machine, now, delegateSessions, settings)
	if err != nil {
		return Ledger{}, err
	}
	measured = append(measured, seatRows...)
	ledger.Seat = seat
	ledger.Unmeasured = append(ledger.Unmeasured, seatUnmeasured...)
	ledger.Rows = aggregateRows(measured)
	ledger.DayScope = summarizeDay(ledger, measured)
	goals := map[string]bool{"seat": true}
	for _, row := range measured {
		goals[row.goal] = true
	}
	for _, item := range ledger.Unmeasured {
		if item.Goal != "" {
			goals[item.Goal] = true
		}
	}
	for goalID := range goals {
		ledger.GoalScopes[goalID] = summarizeGoal(ledger, measured, goalID)
	}
	sort.Strings(ledger.ClaimedGoals)
	sort.Strings(ledger.Inflight)
	sort.Slice(ledger.Unmeasured, func(i, j int) bool {
		if ledger.Unmeasured[i].ID != ledger.Unmeasured[j].ID {
			return ledger.Unmeasured[i].ID < ledger.Unmeasured[j].ID
		}
		return ledger.Unmeasured[i].Detail < ledger.Unmeasured[j].Detail
	})
	if err := writeLedger(repoRoot, &ledger); err != nil {
		return Ledger{}, err
	}
	return ledger, nil
}

func terminalStatus(status string) bool {
	switch status {
	case "completed", "failed", "timeout", "cancelled":
		return true
	}
	return false
}

func parseTime(raw any) (time.Time, error) {
	text, ok := raw.(string)
	if !ok || text == "" {
		return time.Time{}, fmt.Errorf("missing timestamp")
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04:05"} {
		if parsed, err := time.Parse(layout, text); err == nil {
			return parsed.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("unparsable timestamp %q", text)
}

func textOr(raw any, fallback string) string {
	if value, _ := raw.(string); value != "" {
		return value
	}
	return fallback
}

func tokensFromMission(classes map[string]float64) Tokens {
	return Tokens{
		Input: classes["inputTokens"], Cached: classes["cachedInputTokens"],
		Output: classes["outputTokens"], Reasoning: classes["reasoningTokens"],
	}
}

func price(runtime, model string, classes map[string]float64, native *mission.UsageCost, providerOnly bool, settings config.SpendSettings) (money float64, priced, unpriced, foreign int) {
	if native != nil && native.Currency == settings.Currency {
		return native.Amount, 1, 0, 0
	}
	if native != nil && native.Currency != settings.Currency {
		foreign = 1
	}
	classNames := map[string]string{
		"inputTokens": "input", "cachedInputTokens": "cached",
		"outputTokens": "output", "reasoningTokens": "reasoning",
	}
	if len(classes) == 0 {
		return 0, 0, 1, foreign
	}
	for field, value := range classes {
		class := classNames[field]
		rate, ok := settings.Prices[config.SpendPriceKey{Runtime: runtime, Model: model, Class: class}]
		if !ok {
			return 0, 0, 1, foreign
		}
		money += value * rate / 1_000_000
	}
	if providerOnly && len(classes) == 0 {
		return 0, 0, 1, foreign
	}
	return money, 1, 0, foreign
}

func aggregateRows(measurements []pricedMeasurement) []Row {
	rows := map[string]*Row{}
	for _, item := range measurements {
		key := strings.Join([]string{item.goal, item.machine, item.day, item.runtime, item.model}, "\x00")
		row := rows[key]
		if row == nil {
			row = &Row{Goal: item.goal, Machine: item.machine, Day: item.day, Runtime: item.runtime, Model: item.model}
			rows[key] = row
		}
		row.Tokens.Input += item.tokens.Input
		row.Tokens.Cached += item.tokens.Cached
		row.Tokens.Output += item.tokens.Output
		row.Tokens.Reasoning += item.tokens.Reasoning
		row.Money += item.money
		row.PricedRecords += item.priced
		row.Unpriced += item.unpriced
		row.Foreign += item.foreign
	}
	out := make([]Row, 0, len(rows))
	for _, row := range rows {
		out = append(out, *row)
	}
	sort.Slice(out, func(i, j int) bool {
		left := []string{out[i].Goal, out[i].Machine, out[i].Day, out[i].Runtime, out[i].Model}
		right := []string{out[j].Goal, out[j].Machine, out[j].Day, out[j].Runtime, out[j].Model}
		return strings.Join(left, "\x00") < strings.Join(right, "\x00")
	})
	return out
}

func summarizeDay(ledger Ledger, measurements []pricedMeasurement) ScopeSummary {
	scope := ScopeSummary{ID: "day-" + ledger.Day, Machine: ledger.Machine, Day: ledger.Day}
	for _, item := range measurements {
		if item.machine == ledger.Machine && item.day == ledger.Day && item.dayEligible {
			addScope(&scope, item)
		}
	}
	for _, item := range ledger.Unmeasured {
		if item.Provenance == "unreadable" || (item.Machine == ledger.Machine && item.Day == ledger.Day) {
			scope.Unmeasured++
			if item.Provenance == "unreadable" {
				scope.Unreadable++
			}
		}
	}
	for range ledger.Inflight {
		scope.Inflight++
	}
	return scope
}

func summarizeGoal(ledger Ledger, measurements []pricedMeasurement, goalID string) ScopeSummary {
	scope := ScopeSummary{ID: "goal-" + goalID, Goal: goalID, Machine: ledger.Machine}
	for _, item := range measurements {
		if item.machine == ledger.Machine && item.goal == goalID {
			addScope(&scope, item)
		}
	}
	for _, item := range ledger.Unmeasured {
		if item.Machine == ledger.Machine && item.Goal == goalID {
			scope.Unmeasured++
			if item.Provenance == "unreadable" {
				scope.Unreadable++
			}
		}
	}
	return scope
}

func addScope(scope *ScopeSummary, item pricedMeasurement) {
	scope.Tokens += item.tokens.Total()
	scope.Money += item.money
	scope.Unpriced += item.unpriced
	scope.Foreign += item.foreign
}

func claimedGoals(repoRoot, machine string) ([]string, error) {
	dir := filepath.Join(repoRoot, "plans", "goals")
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("cannot read goal ledger %s: %w", dir, err)
	}
	var claimed []string
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".md" || entry.Name() == "backlog.md" {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("cannot read goal ledger %s: %w", path, err)
		}
		file, problems := goal.ParseFile(data)
		if len(problems) != 0 {
			return nil, fmt.Errorf("cannot read goal ledger %s: %s", path, problems[0])
		}
		if file.State == goal.StateClaimed && file.Claimed != nil && file.Claimed.Machine == machine {
			claimed = append(claimed, file.Id)
		}
	}
	return claimed, nil
}

func relativePath(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return path
	}
	return filepath.ToSlash(rel)
}

func writeLedger(repoRoot string, ledger *Ledger) error {
	path := Path(repoRoot, ledger.ObservedAt)
	if existing, err := os.ReadFile(path); err == nil {
		var prior Ledger
		if json.Unmarshal(existing, &prior) == nil {
			priorObservedAt := prior.ObservedAt
			prior.ObservedAt = time.Time{}
			copy := *ledger
			copy.ObservedAt = time.Time{}
			before, _ := json.Marshal(prior)
			after, _ := json.Marshal(copy)
			if bytes.Equal(before, after) {
				ledger.ObservedAt = priorObservedAt
				return nil
			}
		}
	}
	rendered, err := json.MarshalIndent(ledger, "", "  ")
	if err != nil {
		return err
	}
	rendered = append(rendered, '\n')
	_, err = atomicfile.WriteText(path, string(rendered), repoRoot)
	return err
}

func formatTokens(value float64) string {
	return fmt.Sprintf("%.0f", math.Floor(value))
}
