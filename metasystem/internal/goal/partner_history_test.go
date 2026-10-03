package goal

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/brain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
)

func TestHistoryLineThroughRoundTrips(t *testing.T) {
	t.Parallel()
	id := Opid("01ARZ3NDEKTSV4RRFFQ69G5FAA", "mac-a", "project-partner")
	line := "- 2026-10-03T19:00:00Z " + id + " edit actor=human:Wido"
	for _, suffix := range []string{"", " through=" + id} {
		original := line + suffix + " targets=goal-one reason=change the next step"
		parsed, err := ParseHistoryLine(original)
		if err != nil || RenderHistoryLine(parsed) != original {
			t.Fatalf("round trip: %v\n%s", err, RenderHistoryLine(parsed))
		}
	}
	for _, invalid := range []string{
		strings.Replace(line, "human:Wido", "mac-a+project-partner", 1) + " through=" + id,
		line + " through=",
		line + " through=invalid",
		line + " through=" + id + " through=" + id,
	} {
		if _, err := ParseHistoryLine(invalid); err == nil {
			t.Fatalf("invalid history parsed: %s", invalid)
		}
	}
}

func TestPartnerTurnVerdictDoesNotAskForAClaim(t *testing.T) {
	t.Parallel()
	fixture, _, endpoint := fakeServingFixture(t, "bed-m1", map[string]*GoalFile{"ready": budgetedQueuedGoal("ready", "2026-09-07T00:00:00Z")})
	fixture.Now = func() time.Time { return time.Date(2026, 9, 7, 1, 0, 0, 0, time.UTC) }
	record := brain.Record{Schema: brain.Schema, Role: brain.Partner, Ledger: existingLedgerIdentityFor(endpoint), Machine: "bed-m1", DeclaredBy: "Wido", DeclaredAt: "2026-09-07T00:00:00Z"}
	data, _ := json.Marshal(record)
	if err := os.MkdirAll(filepath.Dir(brain.Path(fixture.Root)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(brain.Path(fixture.Root), data, 0o644); err != nil {
		t.Fatal(err)
	}
	verdict, err := fixture.TurnVerdict(ScanResult{}, "partner-session", "", "main-partner")
	if err != nil || verdict.IdleRefusal || verdict.ShouldBlock || strings.Contains(verdict.Display, "IDLE WITH BACKLOG") {
		t.Fatalf("partner turn end asked for work: %+v %v", verdict, err)
	}
}

func TestBudgetActFromAnotherCheckoutKeepsTheHoldersClaimEpoch(t *testing.T) {
	t.Parallel()
	endpoint, person, proof := generalGrantBed(t)
	grant := GeneralGrant{Machine: "mac-a", Checkout: "/partner/checkout", Lineage: "project-partner", Until: person.Now.Add(time.Hour).Truncate(time.Minute)}
	if result, err := GrantGeneral(person, proof, grant); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("grant: %+v %v", result, err)
	}
	holder := attorneyReq(endpoint, 60, "mac-b")
	holder.ClaimEpoch = 6
	if result, err := openClaimForTest(t, holder, "partner-budget", "Keep working under a larger budget.", OriginMain, "Work.", testBudget()); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("holder claim: %+v %v", result, err)
	}
	partner := attorneyReq(endpoint, 61, "mac-a")
	partner.Actor.Human, partner.Actor.Lineage = "Wido", "project-partner"
	partner.CallerClass, partner.EpochAuthority, partner.ClaimEpoch = "MAIN", EpochAuthorityHolder, 19
	partner.Endpoint = endpoint.WithAttorneyEffect(person.opid(), func() (time.Time, error) { return partner.Now, nil })
	bigger := testBudget()
	bigger.AttemptLimit++
	admitted, err := humanauthority.HelmProof(endpoint.Root, humanauthority.HelmGrant{By: "Wido", Grant: person.opid()}, partner.Now)
	if err != nil {
		t.Fatal(err)
	}
	result, err := SetBudgetApproved(partner, "partner-budget", bigger, &admitted)
	if err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("partner budget: %+v %v", result, err)
	}
	tree, err := loadTreeFor(endpoint, result.Tip)
	if err != nil {
		t.Fatal(err)
	}
	file := tree.Live["partner-budget"]
	last := file.History[len(file.History)-1]
	if file.Claimed == nil || file.Claimed.Machine != "mac-b" || file.Claimed.Lineage != holder.Actor.Lineage || file.StopCapability == nil || file.StopCapability.ClaimEpoch != 6 || *file.Budget != bigger || last.Actor != "human:Wido" || last.Through != person.opid() {
		t.Fatalf("budget changed the holder's epoch or lost the grant: %+v history=%+v", file, last)
	}
}
