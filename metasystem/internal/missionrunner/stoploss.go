package missionrunner

// The mission stop-loss verdict (plans/stop-loss-core.md): a pure replay of
// (sealed contract, ledger). No cached counter and no state field — identical
// inputs give identical verdicts on every load, after any crash. Bests seed
// from the sealed baseline and fold forward over each measurement line; the
// only automatic reset besides a qualifying new best is the human's vocal
// `Stop-loss reset:` ledger line. Missions sealed before the replay semantics
// (no ledgerSemantics state field) verdict under the legacy rules the shipped
// shell check enforced, so a sealed budget's meaning never changes
// mid-mission. Non-mission callers keep scripts/assert-stop-loss.sh untouched.
//
// Replay invariant: the replay reads ONLY classification, best, and reset
// lines. Cycle-block annotations (`- Return: rejected:<reason>`,
// `- Outcome: capped`) are audit trail, never fuse input — a ledger with and
// without them yields the identical verdict.

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/contract"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/mission"
)

// Stop-loss park kinds. Stagnation is the only kind whose ask accepts the
// vocal `reset:` answer; a cycle-budget park is an exhausted sealed allowance
// and a legacy park keeps the amendment-only path those missions started with.
const (
	StopLossStagnation  = mission.StopLossKindStagnation
	StopLossCycleBudget = mission.StopLossKindCycleBudget
	StopLossLegacy      = "legacy"
)

// StopLossVerdict is one derived reading of the fuse: how far the mission is
// through both sealed ledger budgets and whether either tripped.
type StopLossVerdict struct {
	Semantics    int64
	Cycles       int
	CycleBudget  int
	NoGainBudget int
	Stagnant     int // replay semantics only; cycles since the last best=yes or reset line
	Tripped      bool
	Kind         string
	Detail       string
}

// askQuestion words the stop-loss ask for the park kind. Only the stagnation
// question names the reset answer, because only a stagnation park accepts it.
func (v *StopLossVerdict) askQuestion() string {
	switch v.Kind {
	case StopLossStagnation:
		return fmt.Sprintf("Stop-loss: %d stagnant cycles against the sealed no-gain budget of %d. Answer this ask with reset:<reason> to spend more of the still-sealed fences, or amend, price, reseal, and sign the mission budget.", v.Stagnant, v.NoGainBudget)
	case StopLossCycleBudget:
		return fmt.Sprintf("Stop-loss: %d cycles recorded against the sealed cycle budget of %d — an exhausted sealed allowance. Amend, price, reseal, and sign the mission budget; reset: cannot extend it.", v.Cycles, v.CycleBudget)
	default:
		return "Amend, price, reseal, and sign the mission budget before requesting stop-loss unpark."
	}
}

// stopLossGate is the sealed measurement authority the replay folds under:
// the declared metrics in declaration order, their thresholds and noise
// floors, the gate direction, and the sealed baseline every best starts from.
type stopLossGate struct {
	direction  string
	metrics    []string
	thresholds map[string]string
	noise      map[string]float64
	baseline   map[string]float64
}

// stopLossGateFromContract builds the gate spec from a sealed contract's
// text (for declaration order) and parsed blocks. A sealed mission contract
// always carries every field read here; a gap is a runner-stopping defect,
// never a silent default.
func stopLossGateFromContract(text string, authored, seal map[string]string) (*stopLossGate, error) {
	metrics, err := thresholdDeclarationOrder(text)
	if err != nil {
		return nil, err
	}
	gate := &stopLossGate{
		direction:  authored["gate.direction"],
		metrics:    metrics,
		thresholds: map[string]string{},
		noise:      map[string]float64{},
		baseline:   map[string]float64{},
	}
	if gate.direction != "max" && gate.direction != "min" {
		return nil, failf(3, "mission contract gate.direction is invalid")
	}
	for _, metric := range metrics {
		gate.thresholds[metric] = authored["gate.threshold."+metric]
		noise, err := strconv.ParseFloat(authored["gate.noise-floor."+metric], 64)
		if err != nil {
			return nil, failf(3, "mission contract gate.noise-floor.%s is not numeric", metric)
		}
		gate.noise[metric] = noise
		baseline, err := strconv.ParseFloat(seal["sealed.baseline."+metric], 64)
		if err != nil {
			return nil, failf(3, "mission contract sealed.baseline.%s is not numeric", metric)
		}
		gate.baseline[metric] = baseline
	}
	return gate, nil
}

// thresholdDeclarationOrder lists the declared gate metrics in the order the
// authored block declares them — the order the new-best tuple compares in.
func thresholdDeclarationOrder(text string) ([]string, error) {
	blocks := authoredBlockRe.FindAllStringSubmatch(text, -1)
	if len(blocks) != 1 {
		return nil, failf(3, "mission contract lacks one authored block")
	}
	var metrics []string
	for _, raw := range strings.Split(blocks[0][1], "\n") {
		key, _, found := strings.Cut(raw, "=")
		if found && strings.HasPrefix(key, "gate.threshold.") {
			metrics = append(metrics, strings.TrimPrefix(key, "gate.threshold."))
		}
	}
	if len(metrics) == 0 {
		return nil, failf(3, "mission contract declares no gate thresholds")
	}
	return metrics, nil
}

// observedValues reads a measurement line's recorded metric values over a
// baseline copy: a metric the line omits or records unparseably folds as the
// sealed baseline, so a broken line can never manufacture a new best.
func (g *stopLossGate) observedValues(observed string) map[string]float64 {
	values := make(map[string]float64, len(g.baseline))
	for metric, value := range g.baseline {
		values[metric] = value
	}
	for _, pair := range strings.Split(observed, ",") {
		name, raw, found := strings.Cut(pair, "=")
		if !found {
			continue
		}
		if _, declared := values[name]; !declared {
			continue
		}
		if value, err := strconv.ParseFloat(raw, 64); err == nil {
			values[name] = value
		}
	}
	return values
}

// tuple builds the new-best comparison tuple: the count of declared
// thresholds met, then each metric's raw directed value in declaration order.
func (g *stopLossGate) tuple(values map[string]float64) []float64 {
	tuple := make([]float64, 0, len(g.metrics)+1)
	met := 0
	for _, metric := range g.metrics {
		if pass, err := contract.ThresholdPasses(g.thresholds[metric], values[metric]); err == nil && pass {
			met++
		}
	}
	tuple = append(tuple, float64(met))
	for _, metric := range g.metrics {
		value := values[metric]
		if g.direction == "min" {
			value = -value
		}
		tuple = append(tuple, value)
	}
	return tuple
}

// qualifies decides candidate-versus-current-best, the only comparison the
// fold ever makes: the candidate is a new best when it is lexicographically
// greater and its first differing component clears its gate — the integer
// thresholds-met count on any strict increase, a metric-value component only
// past that metric's sealed noise floor.
func (g *stopLossGate) qualifies(candidate, best []float64) bool {
	for i := range candidate {
		if candidate[i] == best[i] {
			continue
		}
		if candidate[i] < best[i] {
			return false
		}
		if i == 0 {
			return true
		}
		return candidate[i]-best[i] > g.noise[g.metrics[i-1]]
	}
	return false
}

// lineIsBest settles one measurement line's new-best status — the recorded
// marker wins over re-derivation; a marker-less legacy line derives — and
// returns the folded best tuple after the line.
func (g *stopLossGate) lineIsBest(event mission.LedgerEvent, best []float64) (bool, []float64) {
	candidate := g.tuple(g.observedValues(event.Observed))
	isBest := false
	switch event.Best {
	case "yes":
		isBest = true
	case "no":
		isBest = false
	default:
		isBest = g.qualifies(candidate, best)
	}
	if isBest {
		return true, candidate
	}
	return false, best
}

// replayStopLossVerdict is the replay-semantics fuse: stagnant counts every
// no-progress and unresolved cycle since the last best=yes or reset line —
// no decay rule exists — and the same verdict enforces the sealed cycle
// budget. An exhausted cycle budget dominates, because a vocal reset must
// never extend a spent sealed allowance.
func replayStopLossVerdict(gate *stopLossGate, cycleBudget, noGainBudget int, events []mission.LedgerEvent) StopLossVerdict {
	best := gate.tuple(gate.baseline)
	cycles, stagnant := 0, 0
	for _, event := range events {
		if event.Reset {
			stagnant = 0
			continue
		}
		cycles++
		var isBest bool
		isBest, best = gate.lineIsBest(event, best)
		if isBest {
			stagnant = 0
			continue
		}
		if event.Classification == "no-progress" || event.Classification == "unresolved" {
			stagnant++
		}
	}
	verdict := StopLossVerdict{
		Semantics: 2, Cycles: cycles, CycleBudget: cycleBudget,
		NoGainBudget: noGainBudget, Stagnant: stagnant,
	}
	switch {
	case cycles >= cycleBudget:
		verdict.Tripped = true
		verdict.Kind = StopLossCycleBudget
		verdict.Detail = fmt.Sprintf("%d cycles recorded against the sealed cycle budget of %d", cycles, cycleBudget)
	case stagnant >= noGainBudget:
		verdict.Tripped = true
		verdict.Kind = StopLossStagnation
		verdict.Detail = fmt.Sprintf("%d stagnant cycles against the sealed no-gain budget of %d", stagnant, noGainBudget)
	}
	return verdict
}

// legacyStopLossVerdict reproduces the shipped shell check's rules for
// missions pinned to the old semantics, over the ledger's own budget lines
// exactly as the script read them: any falsified-dead-end, two lifetime
// no-progress cycles, the cycle budget, and the trailing cycles without a
// contract-improved against the no-gain budget. Reset lines cannot lawfully
// exist on these missions and are ignored exactly as the script ignores
// unknown lines.
func legacyStopLossVerdict(cycleBudget, noGainBudget int, events []mission.LedgerEvent) StopLossVerdict {
	total, deadEnds, noProgress, lastImproved := 0, 0, 0, 0
	for _, event := range events {
		if event.Reset {
			continue
		}
		total++
		switch event.Classification {
		case "falsified-dead-end":
			deadEnds++
		case "no-progress":
			noProgress++
		case "contract-improved":
			lastImproved = total
		}
	}
	trailing := total - lastImproved
	verdict := StopLossVerdict{
		Semantics: 1, Cycles: total, CycleBudget: cycleBudget, NoGainBudget: noGainBudget,
	}
	switch {
	case deadEnds > 0:
		verdict.Tripped = true
		verdict.Detail = "a cycle was classified falsified-dead-end"
	case noProgress >= 2:
		verdict.Tripped = true
		verdict.Detail = fmt.Sprintf("%d cycles classified no-progress", noProgress)
	case total >= cycleBudget:
		verdict.Tripped = true
		verdict.Detail = fmt.Sprintf("%d cycles recorded against a budget of %d", total, cycleBudget)
	case trailing >= noGainBudget:
		verdict.Tripped = true
		verdict.Detail = fmt.Sprintf("%d trailing cycles without a contract-improved against a no-gain budget of %d", trailing, noGainBudget)
	}
	if verdict.Tripped {
		verdict.Kind = StopLossLegacy
	}
	return verdict
}

// stopLossSemantics reads the semantics the mission was pinned to at init: a
// state without the field replays under the legacy rules.
func stopLossSemantics(state map[string]any) (int64, error) {
	raw, present := state["ledgerSemantics"]
	if !present || raw == nil {
		return 1, nil
	}
	semantics, ok := jsonInt(raw)
	if !ok || semantics < 1 {
		return 0, failf(3, "mission state ledgerSemantics is invalid")
	}
	return semantics, nil
}

// stopLossVerdict derives the mission stop-loss verdict, keyed by the
// pinned semantics. The replay semantics take both budgets from the sealed
// contract; the legacy semantics take them from the ledger's own budget
// lines, exactly as the shell check the mission started under did.
func (e *Engine) stopLossVerdict(state map[string]any, ledger string) (*StopLossVerdict, error) {
	semantics, err := stopLossSemantics(state)
	if err != nil {
		return nil, err
	}
	ledgerCycleBudget, ledgerNoGain, events, err := mission.ParseLedgerEvents(ledger)
	if err != nil {
		return nil, failf(3, "mission stop-loss replay refused: %v", err)
	}
	switch semantics {
	case 1:
		verdict := legacyStopLossVerdict(ledgerCycleBudget, ledgerNoGain, events)
		return &verdict, nil
	case 2:
		gate, cycleBudget, noGainBudget, err := e.sealedStopLossInputs()
		if err != nil {
			return nil, err
		}
		verdict := replayStopLossVerdict(gate, cycleBudget, noGainBudget, events)
		return &verdict, nil
	default:
		return nil, failf(3, "mission ledgerSemantics %d is newer than this runner", semantics)
	}
}

// sealedStopLossInputs reads the replay's contract-side inputs from the
// pinned approved snapshot: the gate spec and both sealed ledger budgets.
func (e *Engine) sealedStopLossInputs() (*stopLossGate, int, int, error) {
	text, authored, seal, err := e.parseContract(true)
	if err != nil {
		return nil, 0, 0, err
	}
	gate, err := stopLossGateFromContract(text, authored, seal)
	if err != nil {
		return nil, 0, 0, err
	}
	cycleBudget, err := intFromString(authored["ledger.cycle-budget"])
	if err != nil {
		return nil, 0, 0, failf(3, "mission contract ledger.cycle-budget is invalid: %v", err)
	}
	noGainBudget, err := intFromString(authored["ledger.no-gain-budget"])
	if err != nil {
		return nil, 0, 0, failf(3, "mission contract ledger.no-gain-budget is invalid: %v", err)
	}
	return gate, cycleBudget, noGainBudget, nil
}

// bestMarker computes the best=yes|no token for the measurement line about to
// be appended, by the same fold the verdict replays — the marker and the
// verdict can never disagree. Legacy-semantics missions get no marker: they
// finish under the rules they started with.
func (e *Engine) bestMarker(state map[string]any, ledger, observed string) (string, error) {
	semantics, err := stopLossSemantics(state)
	if err != nil {
		return "", err
	}
	if semantics < 2 {
		return "", nil
	}
	gate, _, _, err := e.sealedStopLossInputs()
	if err != nil {
		return "", err
	}
	_, _, events, err := mission.ParseLedgerEvents(ledger)
	if err != nil {
		return "", failf(3, "mission stop-loss replay refused: %v", err)
	}
	best := gate.tuple(gate.baseline)
	for _, event := range events {
		if event.Reset {
			continue
		}
		_, best = gate.lineIsBest(event, best)
	}
	if gate.qualifies(gate.tuple(gate.observedValues(observed)), best) {
		return "yes", nil
	}
	return "no", nil
}
