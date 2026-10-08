package launch

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/readsubject"
)

func TestUnitCollectedReadCountsDespiteDecisionEvidenceFailure(t *testing.T) {
	t.Parallel()
	for _, failure := range []string{"policy", "inherited evidence"} {
		t.Run(failure, func(t *testing.T) {
			t.Parallel()
			fixture := newUnitFixture(t, "diff", "branch", "round", "branch", "round")
			runner := fixture.runner
			result, err := runner.Advance(UnitRequest{Plan: fixture.plan})
			if err != nil {
				t.Fatal(err)
			}
			budget := 1
			record := result.Record
			record.CorrectionBudget = &budget
			if err := runner.save(record); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(t.TempDir(), "return.json")
			writeFile(t, path, structuredUnitReturn(2, "regression", "first.go"))
			if failure == "policy" {
				runner.ReviewPolicy = func() (string, error) { return "", errors.New("policy temporarily unavailable") }
			} else {
				runner.InheritedFindings = func(string, string) ([]readsubject.Finding, error) {
					return nil, errors.New("inherited evidence temporarily unavailable")
				}
			}
			if err := runner.ReviewSubject(record.ID, func(review UnitReview, retain func(UnitSubject) error) error {
				return retain(UnitSubject{Round: review.Round.Number, DiffDigest: review.DiffDigest,
					Examination: "first", ExaminationRound: 1, ExaminationReturnPath: path})
			}); err != nil {
				t.Fatal(err)
			}
			record, err = runner.Status(record.ID)
			if err != nil {
				t.Fatal(err)
			}
			first := record.Rounds[0]
			if first.Material != 2 || first.Stop.Decision != "stop" || first.Stop.Attempt != 1 {
				t.Fatalf("collected read lost its count during %s failure: %+v", failure, first)
			}
			runner.ReviewPolicy, runner.InheritedFindings = nil, nil
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			if err := runner.ReviewSubject(record.ID, func(review UnitReview, retain func(UnitSubject) error) error {
				return retain(*review.Subject)
			}); err != nil {
				t.Fatal(err)
			}
			record, err = runner.Status(record.ID)
			if err != nil {
				t.Fatal(err)
			}
			if repaired := record.Rounds[0]; repaired.Material != 2 || repaired.Stop.Attempt != 1 || repaired.Stop.Decision != "continue" {
				t.Fatalf("decision repair did not use retained reads: material=%d stop=%+v", repaired.Material, repaired.Stop)
			}
			if _, err := runner.Advance(UnitRequest{Resume: record.ID, FollowUp: writeFollowUp(t)}); err != nil {
				t.Fatal(err)
			}
			writeFile(t, path, structuredUnitReturn(1, "scope", "second.go"))
			if err := runner.ReviewSubject(record.ID, func(review UnitReview, retain func(UnitSubject) error) error {
				return retain(UnitSubject{Round: review.Round.Number, DiffDigest: review.DiffDigest,
					Examination: "second", ExaminationRound: 1, ExaminationReturnPath: path})
			}); err != nil {
				t.Fatal(err)
			}
			record, err = runner.Status(record.ID)
			if err != nil {
				t.Fatal(err)
			}
			second := record.Rounds[1]
			if second.Stop.Attempt != 2 || second.Stop.Decision != "stop" || second.Stop.Class != "correction allowance spent" {
				t.Fatalf("read failure granted another correction: %+v", second)
			}
		})
	}
}
