package goal

import (
	"errors"
	"fmt"
	"testing"
)

// A coded refusal reads as its words alone; its code is data for --verbose,
// --json and records ("Messages a Person Reads").
func TestCodedRefusalKeepsItsCodeOutOfItsWords(t *testing.T) {
	t.Parallel()
	err := fmt.Errorf("goal approve: %w", coded("APPROVAL_REQUIRED", errors.New("goal g1 isn't approved yet")))
	if got := err.Error(); got != "goal approve: goal g1 isn't approved yet" {
		t.Fatalf("words = %q", got)
	}
	if got := RefusalCode(err); got != "APPROVAL_REQUIRED" {
		t.Fatalf("code = %q", got)
	}
	if got := RecordText(err); got != "APPROVAL_REQUIRED: goal approve: goal g1 isn't approved yet" {
		t.Fatalf("record text = %q", got)
	}
	if RefusalCode(errors.New("plain")) != "" || RecordText(errors.New("plain")) != "plain" || coded("X_Y", nil) != nil {
		t.Fatal("an uncoded error has no code and records its words")
	}
}

// A mutation's coded refusal reaches the publish result as words and a code,
// and the journal records both.
func TestPublishCarriesAMutationRefusalCodeAsData(t *testing.T) {
	t.Parallel()
	e, _ := fakeGoalEndpoint(t)
	res, err := Publish(e, PublishRequest{
		Opid: "op-coded", Machine: "mac-a", Lineage: "l1",
		Intent: testIntentFor("open"), Message: "goal open coded",
		Mutate: func(string) ([]Change, error) {
			return nil, coded("TRUNK_RED_CLOSED", errors.New("that red is already closed"))
		},
	})
	if err != nil || res.Outcome != OutcomeRejected {
		t.Fatalf("the publish is rejected: %+v %v", res, err)
	}
	if res.Detail != "that red is already closed" || res.Code != "TRUNK_RED_CLOSED" {
		t.Fatalf("detail %q code %q", res.Detail, res.Code)
	}
	entry, err := ReadEntry(e.Root, "op-coded")
	if err != nil || entry.Evidence != "TRUNK_RED_CLOSED: that red is already closed" {
		t.Fatalf("journal evidence %q %v", entry.Evidence, err)
	}
}
