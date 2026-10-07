package goal

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

func TestPersonClaimProvenanceSurvivesHandoverAndResume(t *testing.T) {
	t.Parallel()
	for _, constraint := range []string{"pinned", "blocked"} {
		for _, handover := range []bool{false, true} {
			name := constraint + "/adopted"
			if handover {
				name = constraint + "/lane"
			}
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				endpoint, _ := fakeGoalEndpoint(t)
				request := verbReqFor(endpoint, "01J5X00000000000000000PH00", "mac-a")
				confirmed := func(step string, result PublishResult, err error) *GoalFile {
					t.Helper()
					if err != nil || result.Outcome != OutcomeConfirmed {
						t.Fatalf("%s: %+v %v", step, result, err)
					}
					tree, loadErr := loadTreeFor(endpoint, result.Tip)
					if loadErr != nil {
						t.Fatal(loadErr)
					}
					return tree.Live["reserved"]
				}
				result, err := Open(request, "reserved", "Keep the person's reservation through custody changes.", OriginMain, "Land the work.")
				confirmed("open", result, err)
				request.Ulid = "01J5X00000000000000000PH01"
				if constraint == "pinned" {
					pin := request
					pin.Actor.Human = "Wido"
					result, err = SetPin(pin, "reserved", "mac-b")
					confirmed("pin", result, err)
				} else {
					result, err = Open(request, "blocker", "Finish separately.", OriginMain, "Wait.")
					confirmed("open blocker", result, err)
					request.Ulid = "01J5X00000000000000000PH02"
					blocked := []string{"blocker"}
					result, err = Edit(request, "reserved", EditFields{Blocked: &blocked})
					confirmed("block", result, err)
				}
				person := personalRequest(t, endpoint, "01J5X00000000000000000PH10")
				result, err = Claim(person, "reserved")
				reserved := confirmed("reserve", result, err)
				if !reserved.PersonalReservation() || reserved.StopCapability.ClaimEpoch != 0 || len(reserved.Claimed.Warnings) == 0 {
					t.Fatalf("reservation lost its origin or constraint warning: %+v", reserved)
				}
				budget := testBudget()
				person.Ulid = "01J5X00000000000000000PH20"
				result, err = SetBudgetApproved(person, "reserved", budget, person.Authority)
				confirmed("approve", result, err)
				owner := request
				owner.Ulid, owner.ClaimEpoch = "01J5X00000000000000000PH30", 7
				owner.CallerClass, owner.EpochAuthority = "MAIN", EpochAuthorityHolder
				result, err = Handover(owner, "reserved", owner.Actor.Machine, owner.Actor.Lineage, 7, "batch-person", func() (identity.Liveness, error) { return identity.Alive, nil })
				adopted := confirmed("adopt", result, err)
				if !adopted.PersonalReservation() || adopted.StopCapability.ClaimEpoch != 7 {
					t.Fatalf("adoption lost personal provenance: %+v", adopted)
				}
				if handover {
					owner.Ulid = "01J5X00000000000000000PH40"
					result, err = Handover(owner, "reserved", "lane-host", LaneClaimLineage, 11, "batch-person", func() (identity.Liveness, error) { return identity.Alive, nil })
					adopted = confirmed("handover to lane", result, err)
					owner.Actor, owner.ClaimEpoch = Actor{Machine: "lane-host", Lineage: LaneClaimLineage}, 11
					owner.EpochAuthority = EpochAuthorityLane
				}
				capability := *adopted.StopCapability
				owner.Ulid, owner.Now = "01J5X00000000000000000PH50", owner.Now.Add(time.Minute)
				stop := CloseStopRequest{VerbRequest: owner, GoalID: "reserved", StopID: "stop-reserved", Reason: StopReasonElapsedLimit, Capability: capability}
				result, err = CloseStop(stop)
				stopped := confirmed("breach-stop", result, err)
				person.Ulid, person.Now = "01J5X00000000000000000PH60", owner.Now.Add(time.Minute)
				batch := StopBatch{
					StopID: stop.StopID, GoalID: "reserved", GoalRevision: stopped.Claimed.Revision,
					FenceEpoch: stopped.StopFence.Epoch, CapabilityGeneration: capability.Generation,
					Machine: capability.Machine, ClaimEpoch: capability.ClaimEpoch, Reason: stop.Reason,
					State: StopBatchComplete, OpenedAt: owner.stamp(), UpdatedAt: person.stamp(), CompletedAt: person.stamp(), Pass: 1,
				}
				if err := WriteStopBatch(endpoint.Root, batch); err != nil {
					t.Fatal(err)
				}
				result, err = Resume(ResumeRequest{VerbRequest: person, GoalID: "reserved", Budget: budget, Authority: person.Authority})
				resumed := confirmed("same-owner resume", result, err)
				if !resumed.PersonalReservation() || resumed.Claimed.By != reserved.Claimed.By || resumed.Claimed.Machine != owner.Actor.Machine || resumed.Claimed.Lineage != owner.Actor.Lineage || resumed.StopFence != nil || resumed.StopCapability.ClaimEpoch != owner.ClaimEpoch {
					t.Fatalf("resume lost provenance, moved ownership or kept the fence: %+v", resumed)
				}
				if resumed.Pinned != reserved.Pinned || !reflect.DeepEqual(resumed.Blocked, reserved.Blocked) || !reflect.DeepEqual(resumed.Approved, adopted.Approved) {
					t.Fatal("continuation changed the pin, blocker or approval")
				}
				// A reservation must remain outside the at-rest claim quota,
				// including after resume creates a fresh execution revision.
				tree, _ := loadTreeFor(endpoint, result.Tip)
				ordinary := vGoal("ordinary", StateClaimed)
				ordinary.Claimed.Machine = owner.Actor.Machine
				tree.Live[ordinary.Id] = ordinary
				if problems := ValidateTree(tree); len(problems) != 0 {
					t.Fatalf("continued reservation consumed the ordinary quota: %v", problems)
				}
			})
		}
	}
}

func TestPersonClaimProvenanceEndsOnOrdinaryClaim(t *testing.T) {
	t.Parallel()
	endpoint, _ := fakeGoalEndpoint(t)
	request := verbReqFor(endpoint, "01J5X00000000000000000PE00", "mac-a")
	if result, err := Open(request, "reserved", "Keep personal provenance scoped to its ownership.", OriginMain, "Claim it."); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("open: %+v %v", result, err)
	}
	person := personalRequest(t, endpoint, "01J5X00000000000000000PE10")
	if result, err := Claim(person, "reserved"); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("reserve: %+v %v", result, err)
	}
	budget := testBudget()
	person.Ulid = "01J5X00000000000000000PE20"
	if result, err := SetBudgetApproved(person, "reserved", budget, person.Authority); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("approve: %+v %v", result, err)
	}
	request.Ulid = "01J5X00000000000000000PE30"
	if result, err := Release(request, "reserved"); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("release: %+v %v", result, err)
	}
	request.Ulid, request.Actor.Machine, request.Actor.Lineage = "01J5X00000000000000000PE40", "mac-b", "ordinary-lineage"
	result, err := Claim(request, "reserved")
	if err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("ordinary claim: %+v %v", result, err)
	}
	tree, err := loadTreeFor(endpoint, result.Tip)
	if err != nil {
		t.Fatal(err)
	}
	f := tree.Live["reserved"]
	if f.PersonalReservation() || f.Claimed.By != "" {
		t.Fatalf("ordinary pair inherited a person's exemption: %+v", f.Claimed)
	}
	f.Claimed.By = "human:Wido"
	if f.PersonalReservation() {
		t.Fatal("a carried name bypassed the ordinary claim in history")
	}
	if _, problems := ParseFile(RenderFile(f)); len(problems) == 0 || !strings.Contains(stringProblems(problems), "person-origin history") {
		t.Fatalf("ordinary claim with personal provenance loaded: %v", problems)
	}
}
