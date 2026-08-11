// Package mission holds the mission lifecycle decisions. This file is the
// atomic owner of the mission-wide
// stop-loss ledger: a markdown file with a cycle and no-gain budget and one
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

// Cycle is one adjudicated cycle: its number and the full classification line.
type Cycle struct {
	Number int
	Line   string
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
	// (plans/stop-loss-core.md): the only automatic stagnation reset besides a
	// new best, and it always names the human-answered ask it echoes.
	resetLineRe = regexp.MustCompile(`^Stop-loss reset: ask=([a-z0-9][a-z0-9-]*); reason=([^\n]*)$`)
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
		cycles = append(cycles, Cycle{Number: number, Line: matches[0][1]})
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
// over re-derivation (plans/stop-loss-core.md): "yes" or "no" appends the
// token, "" writes a marker-less legacy line.
func AppendCycle(file string, cycle int, classification, candidateSHA, observed, best string) error {
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
	return atomicWriteText(file, existing+entry)
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
	Best           string // "yes"/"no" when the line carries the marker, "" otherwise
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
		event := LedgerEvent{Cycle: cycle.Number}
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
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.WriteString(text); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return err
	}
	if dir, err := os.Open(filepath.Dir(path)); err == nil {
		_ = dir.Sync()
		_ = dir.Close()
	}
	return nil
}
