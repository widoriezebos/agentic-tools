package dispatch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

func TestOrdinaryClaimDispatchKeepsBudgetAdmission(t *testing.T) {
	t.Parallel()
	for _, state := range []string{"within budget", "missing budget", "missing capability", "fenced"} {
		t.Run(state, func(t *testing.T) {
			t.Parallel()
			bed := newGoalAdmissionBed(t, 2)
			path := filepath.Join(bed.root, "plans", "goals", "bounded.md")
			if state == "fenced" {
				bed.addFenced(t, "bounded", "stop-bounded", [3]string{
					"01ARZ3NDEKTSV4RRFFQ69G5FAV", "01ARZ3NDEKTSV4RRFFQ69G5FAW", "01ARZ3NDEKTSV4RRFFQ69G5FAX",
				})
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			file, problems := goal.ParseFile(data)
			if len(problems) != 0 {
				t.Fatal(problems)
			}
			file.Approved, file.NormApproval = nil, nil
			switch state {
			case "missing budget":
				file.Budget = nil
			case "missing capability":
				file.StopCapability = nil
			}
			if err := os.WriteFile(path, goal.RenderFile(file), 0o644); err != nil {
				t.Fatal(err)
			}
			bed.accept(t)
			before := string(bed.repository.files["plans/goals/bounded.md"])
			verdict, err := bed.revisionAdmission(file.Id, file.Claimed.Revision, 1, time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC))
			switch state {
			case "within budget":
				if err != nil || verdict.Refused() {
					t.Fatalf("ordinary budgeted claim acquired an approval requirement: %+v %v", verdict, err)
				}
			case "missing budget":
				if err != nil || verdict.Refusal == nil || verdict.Refusal.Unknown == nil || verdict.Refusal.Unknown.Code != BudgetUnknown {
					t.Fatalf("missing budget did not retain its typed refusal: %+v %v", verdict, err)
				}
			case "missing capability":
				if err == nil || !strings.Contains(err.Error(), "predates breach-stop authority") {
					t.Fatalf("missing capability entered spending admission: %+v %v", verdict, err)
				}
			case "fenced":
				if err != nil || verdict.Refusal == nil || verdict.Refusal.Unknown == nil || !strings.Contains(verdict.Refusal.Unknown.Reason, "stop-bounded") {
					t.Fatalf("fenced claim entered spending admission: %+v %v", verdict, err)
				}
			}
			if string(bed.repository.files["plans/goals/bounded.md"]) != before || verdict.Extension != nil {
				t.Fatal("admission changed the claim or offered an extension")
			}
		})
	}
}

func TestPersonClaimAdmissionRequiresAdoptionAndApproval(t *testing.T) {
	t.Parallel()
	for _, epoch := range []int64{0, 7} {
		t.Run(map[int64]string{0: "awaiting session", 7: "adopted awaiting approval"}[epoch], func(t *testing.T) {
			t.Parallel()
			bed := newGoalAdmissionBed(t, 2)
			path := filepath.Join(bed.root, "plans", "goals", "bounded.md")
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			file, problems := goal.ParseFile(data)
			if len(problems) > 0 {
				t.Fatal(problems)
			}
			damaged := *file
			damaged.Budget = nil
			if _, problems := goal.ParseFile(goal.RenderFile(&damaged)); len(problems) == 0 {
				t.Fatal("an approval without its required budget was accepted as valid storage")
			}
			file.Approved = nil
			file.Claimed.By = "human:Fixture"
			file.Claimed.EpisodeAt, file.Claimed.EpisodeRevision = file.Claimed.At, file.Claimed.Revision
			file.StopCapability.ClaimEpoch = epoch
			file.History[1].Actor = file.Claimed.By
			file.History[1].Opid = goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5FAW", file.Claimed.Machine, file.Claimed.Lineage)
			file.History[1].AuthorityOutcome, file.History[1].AuthorityGeneration = goal.AuthorityOutcomeHumanAuthorityProven, 1
			if err := os.WriteFile(path, goal.RenderFile(file), 0o644); err != nil {
				t.Fatal(err)
			}
			bed.accept(t)
			before := string(bed.repository.files["plans/goals/bounded.md"])
			verdict, err := bed.revisionAdmission("bounded", 2, 5, time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC))
			remedy := "metasystem goal approve bounded"
			if epoch == 0 {
				remedy = "metasystem session start"
			}
			if err == nil || !strings.Contains(err.Error(), remedy) || verdict.Extension != nil || string(bed.repository.files["plans/goals/bounded.md"]) != before {
				t.Fatalf("reservation entered spending admission: %+v %v", verdict, err)
			}
			if epoch == 0 {
				if _, err := bed.binding("bounded", time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC)); err == nil || !strings.Contains(err.Error(), remedy) {
					t.Fatalf("binding without a caller epoch admitted zero: %v", err)
				}
			}
		})
	}
}
