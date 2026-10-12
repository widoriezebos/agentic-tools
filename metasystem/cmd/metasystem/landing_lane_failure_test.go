package main

import "testing"

// A lane refusal without an underlying error renders its summary alone; it used to
// dereference the nil error and crash the verb (finding 233, 2026-10-12).
func TestLandingLaneFailureWithoutAnErrorRendersTheSummary(t *testing.T) {
	t.Parallel()
	result := landingLaneFailure(nil, "only the lane's current session builds its merge", nil)
	if result.Outcome != intentFailed || result.Summary != "only the lane's current session builds its merge" || len(result.Details) != 0 {
		t.Fatalf("refusal without an error: %+v", result)
	}
	withErr := landingLaneFailure(nil, "the full check command could not run", errText("boom"))
	if len(withErr.Details) != 1 || withErr.Details[0] != "boom" {
		t.Fatalf("refusal with an error: %+v", withErr)
	}
}

type errText string

func (e errText) Error() string { return string(e) }
