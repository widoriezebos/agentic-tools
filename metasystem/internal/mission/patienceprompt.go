package mission

import (
	"fmt"
	"path/filepath"
	"regexp"
)

// Patience prompt projection (plans/patience-satellite-4.md): the next
// turn's This Turn lines derive at assembly time from the FINAL cycle
// block's Patience annotations — the durable bytes carry the chain kind, so
// no second source can disagree with the booking-time classification.

var (
	patienceChainAnnotationRe    = regexp.MustCompile(`^Patience: chain=([a-z0-9][a-z0-9-]*) rounds=([1-9][0-9]*) floor=([1-9][0-9]*)$`)
	patienceOrphanAnnotationRe   = regexp.MustCompile(`^Patience: orphan=([a-z0-9][a-z0-9-]*) rounds=([1-9][0-9]*)$`)
	patienceExcludedAnnotationRe = regexp.MustCompile(`^Patience: excluded=([1-9][0-9]*)$`)
	patienceOverflowAnnotationRe = regexp.MustCompile(`^Patience overflow: chains=([1-9][0-9]*)$`)
)

// patiencePromptLines projects the final cycle block's Patience annotations
// into This Turn lines. Detail lines are FILTERED against current
// chain-closed flags: park-then-close cannot strand a stale warning, the
// ledger keeps its history, and the prompt stays a pure function of
// (ledger, chain-closed flags). The overflow and excluded lines name no
// chains and are exempt; their one-booking staleness is the design's
// stated crash contract. A missing or unparsable ledger
// projects nothing: patience is a non-acting audit surface and prompt
// assembly must not fail on its account.
func patiencePromptLines(ledgerPath, jobsDir string) []string {
	_, _, cycles, err := ParseLedger(ledgerPath)
	if err != nil || len(cycles) == 0 {
		return nil
	}
	var lines []string
	for _, annotation := range cycles[len(cycles)-1].Annotations {
		switch {
		case patienceChainAnnotationRe.MatchString(annotation):
			m := patienceChainAnnotationRe.FindStringSubmatch(annotation)
			if patienceChainClosed(jobsDir, m[1]) {
				continue
			}
			lines = append(lines, fmt.Sprintf(
				"Patience: chain %s has %s unwitnessed rounds (floor %s) — certify landed value or close the chain.",
				m[1], m[2], m[3]))
		case patienceOrphanAnnotationRe.MatchString(annotation):
			m := patienceOrphanAnnotationRe.FindStringSubmatch(annotation)
			if patienceChainClosed(jobsDir, m[1]) {
				continue
			}
			lines = append(lines, fmt.Sprintf(
				"Patience: orphan job %s has unwitnessed spend — certify landed value or flag it to the human.",
				m[1]))
		case patienceExcludedAnnotationRe.MatchString(annotation):
			m := patienceExcludedAnnotationRe.FindStringSubmatch(annotation)
			lines = append(lines, fmt.Sprintf(
				"Patience: %s record(s) excluded from patience — flag it to the human.", m[1]))
		case patienceOverflowAnnotationRe.MatchString(annotation):
			m := patienceOverflowAnnotationRe.FindStringSubmatch(annotation)
			lines = append(lines, fmt.Sprintf(
				"Patience: %s more chains need attention (see ledger).", m[1]))
		}
	}
	return lines
}

// patienceChainClosed reads one root record's chainClosed flag; an
// unreadable record keeps the warning — losing sight of a record never
// silences a drought that was already booked.
func patienceChainClosed(jobsDir, root string) bool {
	doc, err := readJSONObjectFile(filepath.Join(jobsDir, root+".json"))
	if err != nil {
		return false
	}
	closed, _ := doc["chainClosed"].(bool)
	return closed
}
