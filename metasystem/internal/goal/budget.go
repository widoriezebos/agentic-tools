package goal

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goalbudget"
)

// ParseWorkingDuration accepts one or more positive integer day, hour, and
// minute segments. A working day is eight hours; seconds and fractions are not
// part of the budget grammar.
func ParseWorkingDuration(value string) (time.Duration, bool) {
	return goalbudget.ParseWorkingDuration(value)
}

// FormatWorkingDuration is the canonical elapsed-limit token. It preserves
// the working-day convention and never emits seconds.
func FormatWorkingDuration(value time.Duration) string {
	return goalbudget.FormatWorkingDuration(value)
}

// Budget is the complete limit tuple carried by one goal revision. Spending
// is projected from job and governed-run records; the goal never stores
// counters beside these limits.
type Budget = goalbudget.Budget

// NewBudget validates and canonicalizes the complete tuple accepted by the
// command surface. There is no partial form and no numeric default.
func NewBudget(elapsedLimit string, attemptLimit, reservedJobMinutesLimit, activeJobLimit, reviewRoundLimit int64) (Budget, error) {
	return goalbudget.New(elapsedLimit, attemptLimit, reservedJobMinutesLimit, activeJobLimit, reviewRoundLimit)
}

func parseBudgetRecord(value string) (Budget, bool, error) {
	record, err := parseKVRecord(value,
		[]string{"elapsedLimit", "attemptLimit", "reservedJobMinutesLimit", "activeJobLimit"}, []string{"reviewRoundLimit"}, "")
	if err != nil {
		return Budget{}, false, err
	}
	positive := func(key string) (uint64, error) {
		number, parseErr := strconv.ParseUint(record[key], 10, 64)
		if parseErr != nil || number == 0 {
			return 0, fmt.Errorf("%s=%q is not a positive integer", key, record[key])
		}
		return number, nil
	}
	attempts, err := positive("attemptLimit")
	if err != nil {
		return Budget{}, false, err
	}
	reserved, err := positive("reservedJobMinutesLimit")
	if err != nil {
		return Budget{}, false, err
	}
	active, err := positive("activeJobLimit")
	if err != nil {
		return Budget{}, false, err
	}
	legacyFour := record["reviewRoundLimit"] == ""
	reviewRounds := int64(0)
	if !legacyFour {
		reviewRounds, err = strconv.ParseInt(record["reviewRoundLimit"], 10, 64)
		if err != nil || reviewRounds < 0 {
			return Budget{}, false, fmt.Errorf("reviewRoundLimit=%q is not a non-negative integer", record["reviewRoundLimit"])
		}
	}
	budget := Budget{
		ElapsedLimit:            record["elapsedLimit"],
		AttemptLimit:            attempts,
		ReservedJobMinutesLimit: reserved,
		ActiveJobLimit:          active,
		ReviewRoundLimit:        reviewRounds,
	}
	if err := budget.Validate(); err != nil {
		return Budget{}, false, err
	}
	return budget, legacyFour, nil
}

func budgetIntentArgs(b Budget) map[string]string {
	return map[string]string{
		"elapsedLimit":            b.ElapsedLimit,
		"attemptLimit":            strconv.FormatUint(b.AttemptLimit, 10),
		"reservedJobMinutesLimit": strconv.FormatUint(b.ReservedJobMinutesLimit, 10),
		"activeJobLimit":          strconv.FormatUint(b.ActiveJobLimit, 10),
		"reviewRoundLimit":        strconv.FormatInt(b.ReviewRoundLimit, 10),
	}
}

func budgetFromIntentArgs(args map[string]string) (Budget, error) {
	if args["elapsedLimit"] == "" || args["attemptLimit"] == "" ||
		args["reservedJobMinutesLimit"] == "" || args["activeJobLimit"] == "" || args["reviewRoundLimit"] == "" {
		return Budget{}, fmt.Errorf("the stored budget is incomplete; all five fields are required")
	}
	attempts, err := strconv.ParseInt(args["attemptLimit"], 10, 64)
	if err != nil {
		return Budget{}, fmt.Errorf("the stored attemptLimit is invalid: %v", err)
	}
	reserved, err := strconv.ParseInt(args["reservedJobMinutesLimit"], 10, 64)
	if err != nil {
		return Budget{}, fmt.Errorf("the stored reservedJobMinutesLimit is invalid: %v", err)
	}
	active, err := strconv.ParseInt(args["activeJobLimit"], 10, 64)
	if err != nil {
		return Budget{}, fmt.Errorf("the stored activeJobLimit is invalid: %v", err)
	}
	reviewRounds, err := strconv.ParseInt(args["reviewRoundLimit"], 10, 64)
	if err != nil {
		return Budget{}, fmt.Errorf("the stored reviewRoundLimit is invalid: %v", err)
	}
	return NewBudget(args["elapsedLimit"], attempts, reserved, active, reviewRounds)
}

func parseBudgetExtensionArrow(value string) (uint64, uint64, error) {
	fromRaw, toRaw, found := strings.Cut(value, "->")
	if !found || strings.Contains(toRaw, "->") {
		return 0, 0, fmt.Errorf("%q is not <from>-><to>", value)
	}
	from, fromErr := strconv.ParseUint(fromRaw, 10, 64)
	to, toErr := strconv.ParseUint(toRaw, 10, 64)
	if fromErr != nil || toErr != nil || from == 0 || to <= from {
		return 0, 0, fmt.Errorf("%q is not a positive increasing pair", value)
	}
	return from, to, nil
}

// ValidateBudgetExtensionRecord proves the marker is complete and binds it
// to the history event that wrote it. The current tuple is intentionally not
// compared with To: a later person's set-budget may replace the tuple while
// the marker remains.
func (f *GoalFile) ValidateBudgetExtensionRecord() error {
	if f == nil || f.BudgetExtension == nil {
		return nil
	}
	x := f.BudgetExtension
	if !validStamp(x.At) || !validOpidShape(x.Opid) || !validStamp(x.EvidenceAt) ||
		x.AttemptLimitFrom == 0 || x.AttemptLimitTo <= x.AttemptLimitFrom ||
		x.ReservedJobMinutesFrom == 0 || x.ReservedJobMinutesTo <= x.ReservedJobMinutesFrom {
		return fmt.Errorf("record has incomplete or non-increasing coordinates")
	}
	if x.EvidenceKind != "review" && x.EvidenceKind != "landing" && x.EvidenceKind != "receipt" {
		return fmt.Errorf("evidence kind %q is not review|landing|receipt", x.EvidenceKind)
	}
	if x.EvidenceID == "" || strings.ContainsAny(x.EvidenceID, " \t\r\n:@") {
		return fmt.Errorf("evidence id %q is not one record token", x.EvidenceID)
	}
	for _, event := range f.History {
		if event.At == x.At && event.Opid == x.Opid && event.Verb == "extend-budget" &&
			event.Actor != "" && !strings.HasPrefix(event.Actor, "human:") && contains(event.Targets, f.Id) {
			return nil
		}
	}
	return fmt.Errorf("record does not bind its extend-budget History event")
}

// approvalPrecedesBudgetExtension distinguishes a later approval from the
// approval whose tuple earned the extension. History order resolves lawful
// same-second acts because ledger timestamps intentionally have no fraction.
func approvalPrecedesBudgetExtension(f *GoalFile) bool {
	if f == nil || f.Approved == nil || f.BudgetExtension == nil || f.Approved.Revision == 0 {
		return false
	}
	approvedAt, approvedErr := time.Parse(time.RFC3339, f.Approved.At)
	extendedAt, extendedErr := time.Parse(time.RFC3339, f.BudgetExtension.At)
	if approvedErr != nil || extendedErr != nil {
		return false
	}
	if approvedAt.Before(extendedAt) {
		return true
	}
	if !approvedAt.Equal(extendedAt) {
		return false
	}
	approvedIndex := int(f.Approved.Revision - 1)
	for index, event := range f.History {
		if event.Opid == f.BudgetExtension.Opid && event.Verb == "extend-budget" {
			return approvedIndex < index
		}
	}
	return false
}
