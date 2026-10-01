package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/enginecause"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/verbresult"
)

// TestIntentLandJoinRefusalShowsItsCauseUnderVerbose (2026-10-01): a join
// refused at test planning reads as the planning child's plain words; its
// cause, the policy engine's own judgment, is in the details --verbose shows.
func TestIntentLandJoinRefusalShowsItsCauseUnderVerbose(t *testing.T) {
	t.Parallel()
	b := newDeliveryBed(t)
	owners := &landingOwners{configured: true, status: readBranch(2, "critic-root", "critic-root")}
	owners.install(b)
	cause := "rebuilt engine was built from 8b76c7cdaead312d0c8e79aae9fd9ba40219df8c, which is not landed on 16cdb9d8ae460c61db882d1ac4be11c994b96f83"
	child := enginecause.RefuseWith("engine-behind-tip", []enginecause.Fact{enginecause.Path("checkout", "/lane/metasystem")},
		"the pinned engine was not built from the landing branch this run tests against", cause)
	result := verbresult.FromError("test plan", 1, child, nil)
	owners.joinErr = fmt.Errorf("plan joined unit: %w", result.Err())

	code, landed := b.do("work", "land", "standing-validation")
	expectOutcome(t, "join refused at planning", code, landed, intentRefused)
	if !strings.HasPrefix(landed.Summary, "plan joined unit: the pinned engine was not built from the landing branch") || strings.Contains(landed.Summary, cause) {
		t.Fatalf("the plain line changed or carries the cause: %q", landed.Summary)
	}
	if !strings.Contains(strings.Join(landed.Details, "\n"), cause) {
		t.Fatalf("--verbose does not show the cause: %+v", landed.Details)
	}
}
