package launch

import (
	"fmt"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/refusal"
)

// CodedError is a launch refusal: its Error is the plain reason a person
// reads; the code and facts are details (refusal.Coded).
type CodedError = refusal.Coded

// coded builds a launch refusal; the reason is a plain sentence.
func coded(code, facts string, reason error) error { return refusal.New(code, facts, reason) }

// ErrorCode is the refusal code carried by err (refusal.CodeOf).
func ErrorCode(err error) string { return refusal.CodeOf(err) }

// ErrorDetail is err's code-first detail line (refusal.DetailOf).
func ErrorDetail(err error) string { return refusal.DetailOf(err) }

// unitFacts are a unit refusal's key=value facts.
func unitFacts(unit, goal, more string) string {
	return strings.TrimSpace("unit=" + unit + " goal=" + goal + " " + more)
}

// roundLimit refuses a round past the goal's approved number of review
// rounds.
func roundLimit(record UnitRunRecord, rounds int) error {
	return coded("UNIT_ROUND_LIMIT", unitFacts(record.Unit, record.Goal, fmt.Sprintf("run=%s rounds=%d limit=%d", record.ID, rounds, record.MaxRounds)),
		fmt.Errorf("unit %s has used all %d review rounds the goal approved; another round needs a larger budget", record.Unit, record.MaxRounds))
}

// UnitRoundCapError carries the next command as arguments, keeping names intact.
type UnitRoundCapError struct {
	*CodedError
	Next []string
}

func (err *UnitRoundCapError) Unwrap() error { return err.CodedError }

func (runner *UnitRunner) countedCap(record UnitRunRecord) error {
	counted, machinery := countedRounds(record)
	capReached := counted >= record.CountedCap
	if record.CountedCap <= 0 || !capReached {
		return nil
	}
	var newest UnitRound
	for _, round := range record.Rounds {
		if round.Cause == "own" {
			newest = round
		}
	}
	material, err := runner.roundMaterial(record, newest)
	if err != nil {
		return err
	}
	committed := false
	for _, subject := range record.Subjects {
		committed = committed || subject.Commit != ""
	}
	next := fmt.Sprintf("metasystem work review %s --work %s", record.Goal, record.Unit)
	nextArgs := []string{"work", "review", record.Goal, "--work", record.Unit}
	outcome := fmt.Sprintf("Keep the remaining findings as follow-ups with metasystem goal notes %s --read %s --add TEXT.", record.Goal, record.ID)
	if material > 0 {
		next = fmt.Sprintf("metasystem work build %s --work NEW --brief FILE --check ...", record.Goal)
		nextArgs = []string{"work", "build", record.Goal, "--work", "NEW", "--brief", "FILE", "--check", "..."}
		outcome = "Split the unit; the new work builds from the worktree as it stands."
		if committed {
			outcome = fmt.Sprintf("Split on top; only a person's metasystem goal accept-risk %s --finding F naming NEW closes it.", record.Goal)
		}
	}
	return &UnitRoundCapError{CodedError: &CodedError{Code: "UNIT_ROUND_CAP", Facts: unitFacts(record.Unit, record.Goal, fmt.Sprintf("run=%s counted=%d cap=%d machinery=%d", record.ID, counted, record.CountedCap, machinery)),
		Reason: fmt.Errorf("unit %s has used its %d counted rounds; nothing was started. %s", record.Unit, record.CountedCap, outcome), Run: next}, Next: nextArgs}
}

func (runner *UnitRunner) roundDivergent(record UnitRunRecord) error {
	var reads []UnitRound
	for _, round := range record.Rounds {
		if len(round.Reads) > 0 {
			reads = append(reads, round)
		}
	}
	if len(reads) < 2 {
		return nil
	}
	newest, repeats, err := runner.roundFindings(record, reads[len(reads)-1])
	if err != nil || newest <= 0 {
		return err
	}
	previous, _, err := runner.roundFindings(record, reads[len(reads)-2])
	if err != nil || previous < 0 {
		return err
	}
	if newest >= previous || repeats > 0 {
		return &CodedError{Code: "UNIT_ROUND_DIVERGENT", Facts: unitFacts(record.Unit, record.Goal, fmt.Sprintf("run=%s previous=%d newest=%d repeats=%d", record.ID, previous, newest, repeats)),
			Reason:     fmt.Errorf("the last two reads of %s found %d, then %d material findings, %d of them repeats; nothing was started", record.Unit, previous, newest, repeats),
			Run:        fmt.Sprintf("metasystem work build %s --work NEW --brief FILE --check ...", record.Goal),
			Background: fmt.Sprintf("take-a-step-back; land with metasystem work review %s --work %s; or split with metasystem work build %s --work NEW --brief FILE --check ...", record.Goal, record.Unit, record.Goal)}
	}
	return nil
}

// IsCode says whether err carries the refusal code.
func IsCode(err error, code string) bool { return ErrorCode(err) == code }
