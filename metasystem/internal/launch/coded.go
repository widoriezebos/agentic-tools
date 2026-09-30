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

// IsCode says whether err carries the refusal code.
func IsCode(err error, code string) bool { return ErrorCode(err) == code }
