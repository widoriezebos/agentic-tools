// Package mission holds the mission lifecycle decisions: the stop-loss
// ledger, the resource fences and cap authorization, mission state and
// end-state adjudication, and the ask/answer park protocol.
//
// This file is the atomic owner of the mission-wide stop-loss ledger: a markdown file with a cycle and no-gain budget and one
// contiguous "### Cycle N" block per adjudicated cycle. Every mutation takes
// the ledger's flock and writes atomically.
package mission

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"golang.org/x/sys/unix"
)

// Classifications are the verdicts a cycle may carry.
var Classifications = map[string]bool{
	"contract-improved":  true,
	"falsified-continue": true,
	"falsified-dead-end": true,
	"no-progress":        true,
	"unresolved":         true,
	"invalid-run":        true,
}

// Cycle is one adjudicated cycle: its number, the full classification line,
// and the annotation lines the block carries beside it.
type Cycle struct {
	Number      int
	Line        string
	Annotations []string
}

var (
	cycleBudgetRe    = regexp.MustCompile(`(?m)^- Cycle budget:[ \t]*([1-9][0-9]*)[ \t]*$`)
	noGainBudgetRe   = regexp.MustCompile(`(?m)^- No-gain budget:[ \t]*([1-9][0-9]*)[ \t]*$`)
	headingRe        = regexp.MustCompile(`(?m)^### Cycle ([1-9][0-9]*)[ \t]*$`)
	classificationRe = regexp.MustCompile(`(?m)^- Classification:[ \t]*([^\n]+)$`)
	classPrefixRe    = regexp.MustCompile(`^([a-z-]+)`)
	shaRe            = regexp.MustCompile(`^[0-9a-f]{40,64}$`)
	// measurementLineRe splits a runner-written classification line into its
	// verdict, candidate sha, and observed tokens.
	measurementLineRe = regexp.MustCompile(`^([a-z-]+); candidate-sha=([^;\n]+); observed=(.*)$`)
	// resetLineRe is the vocal stop-loss reset line the answer path appends
	// (docs/design/stop-loss-core.md): the only automatic stagnation reset besides a
	// new best, and it always names the human-answered ask it echoes.
	resetLineRe = regexp.MustCompile(`^Stop-loss reset: ask=([a-z0-9][a-z0-9-]*); reason=([^\n]*)$`)
	// Annotation lines inside a cycle block (plans/patience-turn-identity.md):
	// facts recorded beside — never inside — the classification line. They are
	// audit trail: parsers tolerate and expose them, the prompt's ledger tail
	// ignores them, and the stop-loss replay never reads them as fuse input.
	annotationLineRe = regexp.MustCompile(`(?m)^- ((?:Return|Outcome|Drain|Landed unconsumed|Patience|Patience overflow): [^\n]+?)[ \t]*$`)
	// annotationWriteRe is the strict grammar for the annotation kinds the
	// runner writes: the fault that rejected a turn's return, a fired cap,
	// the survivor count of a drain-stalled cycle healed on resume, and the
	// landed-unconsumed rows a completed mission's terminal delivery appends
	// (plans/patience-orphan-usage.md): chain root, round number or the
	// invalid/unreadable marker (the overflow row carries the remaining
	// count), and a whitespace-free return path or the literal none.
	annotationWriteRe = regexp.MustCompile(`^(Return: rejected:.+|Outcome: capped|Drain: stalled:(?:0|[1-9][0-9]*)` +
		`|Landed unconsumed: chain=[a-z0-9][a-z0-9-]* round=(?:[1-9][0-9]*|invalid|unreadable) path=[^ \t]+` +
		`|Patience: chain=[a-z0-9][a-z0-9-]* rounds=[1-9][0-9]* floor=[1-9][0-9]*` +
		`|Patience: orphan=[a-z0-9][a-z0-9-]* rounds=[1-9][0-9]*` +
		`|Patience: excluded=[1-9][0-9]*` +
		`|Patience overflow: chains=[1-9][0-9]*)$`)
)

// Stop-loss park kinds an ask record carries in its stopLossKind field. Only
// a stagnation park accepts the vocal `reset:` answer; a cycle-budget park is
// an exhausted sealed allowance and takes the amendment path alone.
const (
	StopLossKindStagnation  = "stagnation"
	StopLossKindCycleBudget = "cycle-budget"
)

// resetReasonMaxLen caps a reset reason so the ledger line stays one bounded
// line; an over-long reason is refused, never truncated.
const resetReasonMaxLen = 500

// CappedAnnotation is the annotation naming a fired turn cap in the cycle
// block, separate from the classification line so every parser keeps reading.
const CappedAnnotation = "Outcome: capped"

// DrainStalledObserved is the observed token a healed drain-stalled cycle
// carries (plans/patience-mission-reap-drain.md): distinguishable from every
// other no-progress cause, so starvation is recorded exactly once and
// unambiguously.
const DrainStalledObserved = "unmeasurable:drain-stalled"

// DrainStalledAnnotation composes the annotation counting the unprovable
// survivors a drain-stalled park named. Audit trail beside the healed
// classification line, never fuse input.
func DrainStalledAnnotation(survivors int) string {
	if survivors < 0 {
		survivors = 0
	}
	return fmt.Sprintf("Drain: stalled:%d", survivors)
}

// LandedUnconsumedAnnotation composes the terminal-delivery annotation for one
// Landed Returns row (plans/patience-orphan-usage.md): the row's three fields
// as chain/round/path tokens, one bounded ledger line per row. Audit trail
// beside the final classification line, never a classification and never fuse
// input.
func LandedUnconsumedAnnotation(chainRoot, round, path string) string {
	return fmt.Sprintf("Landed unconsumed: chain=%s round=%s path=%s", chainRoot, round, path)
}

// PatienceChainAnnotation composes the vocal floor-breach annotation for one
// well-formed chain (plans/patience-satellite-4.md): audit trail beside the
// classification line, never fuse input.
func PatienceChainAnnotation(root string, rounds, floor int) string {
	return fmt.Sprintf("Patience: chain=%s rounds=%d floor=%d", root, rounds, floor)
}

// PatienceOrphanAnnotation composes the floor-independent damage report for a
// single-round orphan chain: its records are clean, only its ancestry is
// broken, so no floor field rides the line.
func PatienceOrphanAnnotation(id string, rounds int) string {
	return fmt.Sprintf("Patience: orphan=%s rounds=%d", id, rounds)
}

// PatienceExcludedAnnotation composes the aggregate voice for mission-owned
// readable records the participation boundary rejected — a count and a human
// handoff, no identities, no floors, no taxonomy.
func PatienceExcludedAnnotation(count int) string {
	return fmt.Sprintf("Patience: excluded=%d", count)
}

// PatienceOverflowAnnotation composes the single overflow line when breaches
// and orphan reports together exceed the detail bound; the count includes
// both kinds.
func PatienceOverflowAnnotation(count int) string {
	return fmt.Sprintf("Patience overflow: chains=%d", count)
}

// rejectedReasonMaxLen bounds the reason inside a Return: rejected annotation.
// Unlike a human-authored reset reason, this reason is runner-composed from a
// refusal message, so truncation is the right failure mode: the ledger line
// stays bounded and the full detail lives in the turn record.
const rejectedReasonMaxLen = 200

// ReturnRejectedAnnotation composes the annotation naming the fault that
// rejected a turn's return, flattened and bounded to one ledger-safe line.
func ReturnRejectedAnnotation(reason string) string {
	flat := strings.TrimSpace(strings.NewReplacer("\r", " ", "\n", " ", "\t", " ").Replace(reason))
	if flat == "" {
		flat = "unspecified"
	}
	if len(flat) > rejectedReasonMaxLen {
		cut := rejectedReasonMaxLen
		for cut > 0 && !utf8.RuneStart(flat[cut]) {
			cut--
		}
		flat = flat[:cut]
	}
	return "Return: rejected:" + flat
}

// ParseLedger validates a mission ledger and returns its budgets and cycles.
// It enforces exactly one positive Cycle budget and No-gain budget, cycle
// headings contiguous from 1, and one known Classification line per cycle.
func ParseLedger(file string) (cycleBudget, noGainBudget int, cycles []Cycle, err error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return 0, 0, nil, fmt.Errorf("cannot read mission ledger: %w", err)
	}
	text := string(data)
	cb := cycleBudgetRe.FindAllStringSubmatch(text, -1)
	ngb := noGainBudgetRe.FindAllStringSubmatch(text, -1)
	if len(cb) != 1 || len(ngb) != 1 {
		return 0, 0, nil, fmt.Errorf("mission ledger must have exactly one positive Cycle budget and No-gain budget")
	}
	headings := headingRe.FindAllStringSubmatchIndex(text, -1)
	for i, loc := range headings {
		number, _ := strconv.Atoi(text[loc[2]:loc[3]])
		if number != i+1 {
			return 0, 0, nil, fmt.Errorf("mission ledger cycle headings must be contiguous from 1")
		}
		blockStart := loc[1]
		blockEnd := len(text)
		if i+1 < len(headings) {
			blockEnd = headings[i+1][0]
		}
		block := text[blockStart:blockEnd]
		matches := classificationRe.FindAllStringSubmatch(block, -1)
		if len(matches) != 1 {
			return 0, 0, nil, fmt.Errorf("Cycle %d must have exactly one Classification line", number)
		}
		prefix := classPrefixRe.FindStringSubmatch(matches[0][1])
		if prefix == nil || !Classifications[prefix[1]] {
			return 0, 0, nil, fmt.Errorf("Cycle %d has an unknown classification", number)
		}
		var annotations []string
		for _, annotation := range annotationLineRe.FindAllStringSubmatch(block, -1) {
			annotations = append(annotations, annotation[1])
		}
		cycles = append(cycles, Cycle{Number: number, Line: matches[0][1], Annotations: annotations})
	}
	cycleBudget, _ = strconv.Atoi(cb[0][1])
	noGainBudget, _ = strconv.Atoi(ngb[0][1])
	return cycleBudget, noGainBudget, cycles, nil
}

// InitLedger creates a new mission ledger with the given budgets; it refuses to
// overwrite an existing one.
func InitLedger(file string, cycleBudget, noGainBudget int) error {
	if cycleBudget < 1 || noGainBudget < 1 {
		return fmt.Errorf("mission ledger budgets must be positive integers")
	}
	lock, err := lockFile(file)
	if err != nil {
		return err
	}
	defer lock.release()
	if _, err := os.Stat(file); err == nil {
		return fmt.Errorf("mission ledger already exists")
	}
	text := fmt.Sprintf("# Mission Ledger\n\n- Cycle budget: %d\n- No-gain budget: %d\n", cycleBudget, noGainBudget)
	return atomicWriteText(file, text)
}

// AppendCycle appends the next cycle's verdict. cycle must be exactly one past
// the last recorded cycle, the classification must be known, and the candidate
// sha must be a resolved git sha. best is the new-best marker replay honors
// over re-derivation (docs/design/stop-loss-core.md): "yes" or "no" appends the
// token, "" writes a marker-less legacy line. Annotations land as their own
// lines under the classification line, one atomic append with it, so a cycle
// block always carries both facts or neither.
func AppendCycle(file string, cycle int, classification, candidateSHA, observed, best string, annotations ...string) error {
	lock, err := lockFile(file)
	if err != nil {
		return err
	}
	defer lock.release()
	_, _, cycles, err := ParseLedger(file)
	if err != nil {
		return err
	}
	if expected := len(cycles) + 1; cycle != expected {
		return fmt.Errorf("next mission ledger cycle must be %d", expected)
	}
	if !Classifications[classification] {
		return fmt.Errorf("unknown mission classification")
	}
	sha, err := oneLine(candidateSHA, "candidate sha")
	if err != nil {
		return err
	}
	if !shaRe.MatchString(sha) {
		return fmt.Errorf("candidate sha must be a resolved git sha")
	}
	observedLine, err := oneLine(observed, "observed measurement")
	if err != nil {
		return err
	}
	if best != "" && best != "yes" && best != "no" {
		return fmt.Errorf("new-best marker must be yes, no, or absent")
	}
	for _, annotation := range annotations {
		if !annotationWriteRe.MatchString(annotation) {
			return fmt.Errorf("unknown mission ledger annotation kind: %q", annotation)
		}
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("cannot read mission ledger: %w", err)
	}
	existing := strings.TrimRightFunc(string(data), unicode.IsSpace)
	marker := ""
	if best != "" {
		marker = "; best=" + best
	}
	entry := fmt.Sprintf("\n\n### Cycle %d\n- Classification: %s; candidate-sha=%s; observed=%s%s\n",
		cycle, classification, sha, observedLine, marker)
	for _, annotation := range annotations {
		entry += "- " + annotation + "\n"
	}
	return atomicWriteText(file, existing+entry)
}

// AppendAnnotations appends annotation lines to an existing cycle's block —
// the terminal delivery for facts that exist only after that cycle's own line
// landed, such as the completion conclude's Landed Returns list
// (plans/patience-orphan-usage.md). Only the FINAL cycle accepts the append:
// any earlier block is closed history, and history is never rewritten. The
// annotations must match the strict write grammar, and the append is one
// atomic write under the ledger lock.
func AppendAnnotations(file string, cycle int, annotations ...string) error {
	if len(annotations) == 0 {
		return nil
	}
	lock, err := lockFile(file)
	if err != nil {
		return err
	}
	defer lock.release()
	_, _, cycles, err := ParseLedger(file)
	if err != nil {
		return err
	}
	if len(cycles) == 0 || cycle != len(cycles) {
		return fmt.Errorf("annotations append only to the final ledger cycle (%d)", len(cycles))
	}
	for _, annotation := range annotations {
		if !annotationWriteRe.MatchString(annotation) {
			return fmt.Errorf("unknown mission ledger annotation kind: %q", annotation)
		}
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("cannot read mission ledger: %w", err)
	}
	var appended strings.Builder
	appended.WriteString(strings.TrimRightFunc(string(data), unicode.IsSpace))
	for _, annotation := range annotations {
		appended.WriteString("\n- " + annotation)
	}
	appended.WriteString("\n")
	return atomicWriteText(file, appended.String())
}

// AppendReset appends the vocal stop-loss reset line naming the answered ask.
// The reason is refused — never mangled — when it is empty, carries a newline,
// or exceeds the length cap, so a ledger line can never be split or
// structurally injected; the append is one atomic write under the ledger lock.
func AppendReset(file, askID, reason string) error {
	if !idRe.MatchString(askID) {
		return fmt.Errorf("stop-loss reset must name a valid ask id")
	}
	if strings.ContainsAny(reason, "\n\r") {
		return fmt.Errorf("stop-loss reset reason must not contain newlines")
	}
	if strings.TrimSpace(reason) == "" {
		return fmt.Errorf("stop-loss reset requires a non-empty reason")
	}
	if len(reason) > resetReasonMaxLen {
		return fmt.Errorf("stop-loss reset reason exceeds %d characters", resetReasonMaxLen)
	}
	lock, err := lockFile(file)
	if err != nil {
		return err
	}
	defer lock.release()
	if _, _, _, err := ParseLedger(file); err != nil {
		return err
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("cannot read mission ledger: %w", err)
	}
	existing := strings.TrimRightFunc(string(data), unicode.IsSpace)
	entry := fmt.Sprintf("\n\nStop-loss reset: ask=%s; reason=%s\n", askID, reason)
	return atomicWriteText(file, existing+entry)
}

// LedgerEvent is one replay-ordered ledger event: an adjudicated cycle's
// measurement line, or a stop-loss reset line between cycles.
type LedgerEvent struct {
	// Cycle events.
	Cycle          int
	Classification string
	Observed       string
	Best           string   // "yes"/"no" when the line carries the marker, "" otherwise
	Annotations    []string // annotation lines beside the classification; audit trail, never fuse input
	// Reset events.
	Reset  bool
	AskID  string
	Reason string
}

// ParseLedgerEvents validates a mission ledger and returns its budgets plus
// every cycle and reset line in file order — the replay input for the derived
// stop-loss verdict. A classification line an older writer produced without
// the candidate-sha/observed tokens degrades conservatively: the verdict word
// stands and the observed value is empty (folds as baseline). Derivation
// never writes anything.
func ParseLedgerEvents(file string) (cycleBudget, noGainBudget int, events []LedgerEvent, err error) {
	cycleBudget, noGainBudget, cycles, err := ParseLedger(file)
	if err != nil {
		return 0, 0, nil, err
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return 0, 0, nil, fmt.Errorf("cannot read mission ledger: %w", err)
	}
	text := string(data)
	headings := headingRe.FindAllStringSubmatchIndex(text, -1)

	type positioned struct {
		offset int
		event  LedgerEvent
	}
	var ordered []positioned
	for i, cycle := range cycles {
		event := LedgerEvent{Cycle: cycle.Number, Annotations: cycle.Annotations}
		if m := measurementLineRe.FindStringSubmatch(cycle.Line); m != nil {
			event.Classification = m[1]
			event.Observed = strings.TrimSpace(m[3])
			for _, marker := range []string{"yes", "no"} {
				if suffix := "; best=" + marker; strings.HasSuffix(event.Observed, suffix) {
					event.Observed = strings.TrimSuffix(event.Observed, suffix)
					event.Best = marker
					break
				}
			}
		} else {
			event.Classification = classPrefixRe.FindStringSubmatch(cycle.Line)[1]
		}
		ordered = append(ordered, positioned{offset: headings[i][0], event: event})
	}
	offset := 0
	for _, raw := range strings.Split(text, "\n") {
		if m := resetLineRe.FindStringSubmatch(raw); m != nil {
			ordered = append(ordered, positioned{offset: offset, event: LedgerEvent{
				Reset: true, AskID: m[1], Reason: m[2],
			}})
		}
		offset += len(raw) + 1
	}
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].offset < ordered[j].offset })
	events = make([]LedgerEvent, len(ordered))
	for i, item := range ordered {
		events[i] = item.event
	}
	return cycleBudget, noGainBudget, events, nil
}

func oneLine(value, label string) (string, error) {
	if value == "" || strings.ContainsAny(value, "\n\r") {
		return "", fmt.Errorf("%s must be one non-empty line", label)
	}
	return value, nil
}

type fileLock struct{ f *os.File }

func lockFile(file string) (*fileLock, error) {
	lockPath := file + ".lock"
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	if err := unix.Flock(int(f.Fd()), unix.LOCK_EX); err != nil {
		f.Close()
		return nil, err
	}
	return &fileLock{f: f}, nil
}

func (l *fileLock) release() {
	if l == nil || l.f == nil {
		return
	}
	_ = unix.Flock(int(l.f.Fd()), unix.LOCK_UN)
	_ = l.f.Close()
}

func atomicWriteText(path, text string) error {
	// The durable-anchor argument is empty until this writer is converted to
	// the two-outcome signature (go-production-grade B5): with no anchor the
	// owner syncs the target directory only, which is exactly the behavior
	// this caller had before. Converting it is the caller-migration step,
	// and until then no crash-durability may be claimed here.
	_, err := atomicfile.WriteText(path, text, "")
	return err
}
