package goal

import (
	"strings"
	"testing"
)

func TestClaimApprovalEligibilityKeepsNormAtClaimAdmission(t *testing.T) {
	t.Parallel()
	endpoint, request := restampFixture(t, "standing-approval")
	tree, err := loadTreeFor(endpoint, acceptedTipForEndpoint(t, endpoint))
	if err != nil {
		t.Fatal(err)
	}
	file := tree.Live["standing-approval"]
	file.Tier = 1
	file.Risk = &RiskRecord{Severity: 1, Novelty: 1, Exposure: 1, Accumulation: 1, Basis: "Exercise standing approval independently of claim norms."}
	file.Budget.ReservedJobMinutesLimit, file.Budget.ReviewRoundLimit = 1000, 2
	file.NormApproval = nil
	file.Approved.Digest = ApprovalDigest(file.Intent, file.Tier, *file.Budget, file.Risk)
	if eligibility := ClaimApprovalEligibility(tree, file, request.Now); !eligibility.Ready {
		t.Fatalf("standing approval refused proof spending: %+v", eligibility)
	}
	if eligibility := claimExecutionEligibility(newClaimAdmissionContext(endpoint.Root), tree, file, request.Now); eligibility.Ready || !strings.Contains(eligibility.Wait, "over its tier") {
		t.Fatalf("revival ignored claim norm: %+v", eligibility)
	}
	if _, err := requireApprovedForClaim(endpoint.Root, tree, file, request.Now, "claim"); err == nil || !strings.Contains(err.Error(), "over its tier") {
		t.Fatalf("claim ignored its norm: %v", err)
	}

	for _, failure := range []string{"missing approval", "missing budget", "damaged approval", "expired approval", "missing capability", "fenced claim"} {
		t.Run(failure, func(t *testing.T) {
			t.Parallel()
			copy, problems := ParseFile(RenderFile(file))
			if len(problems) != 0 {
				t.Fatal(problems)
			}
			switch failure {
			case "missing approval":
				copy.Approved = nil
			case "missing budget":
				copy.Budget = nil
			case "damaged approval":
				copy.Approved.Digest = strings.Repeat("0", 64)
			case "expired approval":
				copy.Approved.Authority, copy.Approved.ReviewBy = ApprovalAuthorityRelayed, "2000-01-01"
				event := &copy.History[copy.Approved.Revision-1]
				event.AuthorityOutcome, event.AuthorityReviewBy = AuthorityOutcomeTemporaryHumanWord, copy.Approved.ReviewBy
			case "missing capability":
				copy.StopCapability = nil
			case "fenced claim":
				copy.StopFence = &StopFence{StopID: "stop-standing-approval"}
			}
			if eligibility := ClaimApprovalEligibility(tree, copy, request.Now); eligibility.Ready || eligibility.Wait == "" {
				t.Fatalf("%s admitted spending: %+v", failure, eligibility)
			}
		})
	}
}

func TestSetBudgetHolderRenewsOrdinaryClaimAndPreservesPersonalEpoch(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"ordinary", "reserved", "adopted reservation"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			endpoint, _ := fakeGoalEndpoint(t)
			request := verbReqFor(endpoint, "01J5X00000000000000000BH00", "mac-a")
			result, err := Open(request, "rebudgeted", "Keep the claim's epoch owner.", OriginMain, "Work it.")
			if err != nil || result.Outcome != OutcomeConfirmed {
				t.Fatalf("open: %+v %v", result, err)
			}
			budget := testBudget()
			approveGoalForTest(t, request, "rebudgeted", budget)
			request.Ulid = "01J5X00000000000000000BH10"
			if kind != "ordinary" {
				request = personalRequest(t, endpoint, request.Ulid)
				if kind == "adopted reservation" {
					request.ClaimEpoch = 5
				}
			}
			result, err = Claim(request, "rebudgeted")
			if err != nil || result.Outcome != OutcomeConfirmed {
				t.Fatalf("claim: %+v %v", result, err)
			}
			tree, err := loadTreeFor(endpoint, result.Tip)
			if err != nil {
				t.Fatal(err)
			}
			before := tree.Live["rebudgeted"]
			budget.AttemptLimit++
			request.Ulid, request.Actor.Human = "01J5X00000000000000000BH20", "Wido"
			request.CallerClass, request.EpochAuthority, request.ClaimEpoch = "MAIN", EpochAuthorityHolder, 9
			result, err = SetBudgetApproved(request, "rebudgeted", budget, testHumanAuthority(t, endpoint.Root, request.Now))
			if err != nil || result.Outcome != OutcomeConfirmed {
				t.Fatalf("budget: %+v %v", result, err)
			}
			tree, err = loadTreeFor(endpoint, result.Tip)
			if err != nil {
				t.Fatal(err)
			}
			after := tree.Live["rebudgeted"]
			wantEpoch := before.StopCapability.ClaimEpoch
			if kind == "ordinary" {
				wantEpoch = 9
			}
			if after.StopCapability == nil || after.StopCapability.ClaimEpoch != wantEpoch ||
				after.Claimed.By != before.Claimed.By || after.PersonalReservation() != before.PersonalReservation() ||
				after.Claimed.EpisodeRevision != before.Claimed.EpisodeRevision ||
				after.Claimed.AccountingRevision <= before.Claimed.AccountingRevision || *after.Budget != budget {
				t.Fatalf("budget moved the wrong epoch or lost claim accounting: before=%+v after=%+v want epoch=%d", before, after, wantEpoch)
			}
		})
	}
}
