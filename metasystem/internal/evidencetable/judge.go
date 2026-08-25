package evidencetable

import (
	"fmt"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/covenant"
)

// Verdicts for a covenant-backed pair, one per pair by precedence:
// broken-dep > floating > missing-row > unjudged-external > bound.
// Every refused run names at least one failing pair verdict.
const (
	VerdictBound            = "bound"
	VerdictMissingRow       = "missing-row"
	VerdictFloating         = "floating"
	VerdictBrokenDep        = "broken-dep"
	VerdictUnjudgedExternal = "unjudged-external"
)

// AssessmentRecordedUnverified labels every bound or
// unjudged-external row's status for what it is in this slice: a
// claim on file. Neither "observed" nor "referenced-not-run" is
// re-verified here — that needs observation metadata a later slice
// records — and nothing in the machine output may read as
// verification.
const AssessmentRecordedUnverified = "recorded-unverified"

// Refusal is one named reason the gate refused.
type Refusal struct {
	Kind   string `json:"kind"`
	Detail string `json:"detail"`
}

// Pair is one covenant requirement joined to its evidence row.
type Pair struct {
	ID         string   `json:"id"`
	Ref        string   `json:"ref"`
	Proof      string   `json:"proof"`
	Status     string   `json:"status,omitempty"`
	Kind       string   `json:"kind,omitempty"`
	Deps       []string `json:"deps,omitempty"`
	Verdict    string   `json:"verdict"`
	Assessment string   `json:"assessment,omitempty"`
}

// Orphan is a table row no covenant requirement cites — lawful (the
// table may know more than the covenant; deferred criteria live
// here), reported never refused.
type Orphan struct {
	CriterionID string   `json:"criterionId"`
	Criterion   string   `json:"criterion"`
	Proof       string   `json:"proof"`
	Status      string   `json:"status"`
	Notes       []string `json:"notes,omitempty"`
}

// Counts holds the recorded bookkeeping line beside what the rows
// derive; a mismatch is drift the counselor nudges about, never a
// refusal.
type Counts struct {
	RecordedWired    int64 `json:"recordedWired"`
	RecordedFloating int64 `json:"recordedFloating"`
	DerivedWired     int64 `json:"derivedWired"`
	DerivedFloating  int64 `json:"derivedFloating"`
	Match            bool  `json:"match"`
}

// Report is the whole judgment, JSON-ready.
type Report struct {
	App      string    `json:"app"`
	Outcome  string    `json:"outcome"`
	Refusals []Refusal `json:"refusals,omitempty"`
	Pairs    []Pair    `json:"pairs"`
	Orphans  []Orphan  `json:"orphans,omitempty"`
	Counts   Counts    `json:"counts"`
	Notes    []string  `json:"notes,omitempty"`
}

// Judge joins the covenant's requirements to the table's rows and
// applies the gate. rootFD anchors every dependency walk.
func Judge(cov *covenant.Covenant, table *Table, rootFD int) *Report {
	report := &Report{App: cov.Identity.Name, Outcome: "traceable"}
	rows := map[string]*Row{}
	backed := map[string]bool{}
	for i := range table.Rows {
		rows[table.Rows[i].CriterionID] = &table.Rows[i]
	}
	for _, req := range cov.Requirements {
		pair := Pair{ID: req.ID, Ref: req.Ref, Proof: req.Proof}
		row, ok := rows[req.ID]
		switch {
		case !ok:
			pair.Verdict = VerdictMissingRow
			report.Refusals = append(report.Refusals, Refusal{
				Kind:   VerdictMissingRow,
				Detail: fmt.Sprintf("covenant requirement %s (proof %s) has no evidence row", req.ID, req.Proof),
			})
		case row.ProofID != req.Proof:
			backed[req.ID] = true
			pair.Verdict = VerdictMissingRow
			report.Refusals = append(report.Refusals, Refusal{
				Kind:   VerdictMissingRow,
				Detail: fmt.Sprintf("criterion %s is bound to proof %s in the covenant but records proof %s in the table", req.ID, req.Proof, row.ProofID),
			})
		default:
			backed[req.ID] = true
			pair.Status = row.Status
			pair.Kind = row.Kind
			pair.Deps = row.Deps
			pair.Verdict, pair.Assessment = judgeRow(row, rootFD, report)
		}
		report.Pairs = append(report.Pairs, pair)
	}
	for i := range table.Rows {
		row := &table.Rows[i]
		if backed[row.CriterionID] {
			continue
		}
		orphan := Orphan{
			CriterionID: row.CriterionID,
			Criterion:   row.Criterion,
			Proof:       row.ProofID,
			Status:      row.Status,
		}
		if row.Status == StatusPlannedFloating {
			orphan.Notes = append(orphan.Notes, "floating: no executable proof yet — a goal, not a guarantee")
		}
		for i, dep := range row.Deps {
			if err := CheckDep(rootFD, dep, i == 0); err != nil {
				orphan.Notes = append(orphan.Notes, fmt.Sprintf("declared dep %s is broken: %v", dep, err))
			}
		}
		report.Orphans = append(report.Orphans, orphan)
	}
	report.Counts = deriveCounts(table)
	if !report.Counts.Match {
		report.Notes = append(report.Notes, fmt.Sprintf(
			"the recorded count line (Wired: %d. Floating: %d.) disagrees with the rows (wired %d, floating %d) — bookkeeping drift for the next sitting",
			report.Counts.RecordedWired, report.Counts.RecordedFloating,
			report.Counts.DerivedWired, report.Counts.DerivedFloating))
	}
	if len(report.Refusals) > 0 {
		report.Outcome = "refused"
	}
	return report
}

// judgeRow applies the per-row gate to a matched covenant-backed row
// and returns its verdict and assessment, appending refusals.
func judgeRow(row *Row, rootFD int, report *Report) (string, string) {
	if row.Status == StatusPlannedFloating {
		report.Refusals = append(report.Refusals, Refusal{
			Kind:   VerdictFloating,
			Detail: fmt.Sprintf("criterion %s is covenant-backed but records planned-floating — intent guaranteed by nothing", row.CriterionID),
		})
		return VerdictFloating, ""
	}
	// The first declared dep is the entrypoint FILE for every kind —
	// an external row's local adapter is checked like any other.
	for i, dep := range row.Deps {
		if err := CheckDep(rootFD, dep, i == 0); err != nil {
			report.Refusals = append(report.Refusals, Refusal{
				Kind:   VerdictBrokenDep,
				Detail: fmt.Sprintf("criterion %s declares dep %s: %v", row.CriterionID, dep, err),
			})
			return VerdictBrokenDep, ""
		}
	}
	if row.Kind == KindExternal {
		return VerdictUnjudgedExternal, AssessmentRecordedUnverified
	}
	return VerdictBound, AssessmentRecordedUnverified
}

func deriveCounts(table *Table) Counts {
	counts := Counts{
		RecordedWired:    table.RecordedWired,
		RecordedFloating: table.RecordedFloating,
	}
	for _, row := range table.Rows {
		if row.Status == StatusPlannedFloating {
			counts.DerivedFloating++
		} else {
			counts.DerivedWired++
		}
	}
	counts.Match = counts.RecordedWired == counts.DerivedWired &&
		counts.RecordedFloating == counts.DerivedFloating
	return counts
}
