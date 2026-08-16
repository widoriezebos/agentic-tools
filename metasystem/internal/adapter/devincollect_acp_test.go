package adapter

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func (f *collectFixture) acpOutcomeWith(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(f.roundDir, "acp-outcome.json")
	writeFile(t, path, body)
	return path
}

// The ACP channel is EXCLUSIVE: a delivered outcome's candidate rides
// the same qualification path as every legacy channel, and nothing
// about the walk consults stdout, the named file, or the transcript —
// evidence never crosses transports.
func TestCollectACPExclusiveDelivery(t *testing.T) {
	f := newCollectFixture(t)
	candidate, _ := json.Marshal(string(f.validReturn))
	f.params.ACPOutcomePath = f.acpOutcomeWith(t,
		`{"row":"delivered","sessionId":"sess-1","stopReason":"end_turn","candidate":`+string(candidate)+`}`)
	// A poisoned legacy channel proves the walk never looks at it.
	writeFile(t, f.params.StdoutPath, "junk that would reject")

	verdict, err := DevinCollect(f.params)
	if err != nil {
		t.Fatal(err)
	}
	if !verdict.Delivered || verdict.Channel != "acp" {
		t.Fatalf("%+v", verdict)
	}
	accepted, _ := os.ReadFile(verdict.Reply)
	if string(accepted) != string(f.validReturn) {
		t.Fatal("the accepted snapshot must be the candidate bytes verbatim")
	}
	for _, rejection := range verdict.Rejected {
		if strings.HasPrefix(rejection, "stdout") {
			t.Fatal("the legacy channels must never be consulted on the acp walk")
		}
	}
}

// No fallthrough: an undelivered or invalid ACP outcome fails
// honestly even when a perfectly valid legacy candidate sits right
// there.
func TestCollectACPNeverFallsThrough(t *testing.T) {
	f := newCollectFixture(t)
	writeFile(t, f.params.StdoutPath, string(f.validReturn))
	f.params.ACPOutcomePath = f.acpOutcomeWith(t,
		`{"row":"incomplete","sessionId":"sess-1","stopReason":"max_tokens"}`)

	verdict, err := DevinCollect(f.params)
	if err != nil {
		t.Fatal(err)
	}
	if verdict.Delivered || verdict.Channel != "none" {
		t.Fatalf("an incomplete acp turn must not deliver via legacy scraping: %+v", verdict)
	}
	if len(verdict.Rejected) == 0 || !strings.Contains(verdict.Rejected[0], "row=incomplete") {
		t.Fatalf("the rejection must name the acp row: %v", verdict.Rejected)
	}
}

// The wrong session is a named rejection (the outcome's correlation
// is part of the evidence chain), and presence-only reports the acp
// candidate bar without validating.
func TestCollectACPSessionAndPresence(t *testing.T) {
	f := newCollectFixture(t)
	candidate, _ := json.Marshal(string(f.validReturn))
	outcome := f.acpOutcomeWith(t,
		`{"row":"delivered","sessionId":"SOMEONE-ELSE","stopReason":"end_turn","candidate":`+string(candidate)+`}`)
	f.params.ACPOutcomePath = outcome

	verdict, err := DevinCollect(f.params)
	if err != nil {
		t.Fatal(err)
	}
	if verdict.Delivered || len(verdict.Rejected) == 0 || !strings.Contains(verdict.Rejected[0], "not this turn's session") {
		t.Fatalf("%+v", verdict)
	}

	f.params.PresenceOnly = true
	f.params.Session = ""
	verdict, err = DevinCollect(f.params)
	if err != nil || !verdict.CandidatesPresent {
		t.Fatalf("presence must see the delivered candidate: %+v err %v", verdict, err)
	}
}

// A thinned journal disqualifies delivery even with a perfect
// candidate: the raw evidence is admittedly incomplete (P3
// critique F8).
func TestCollectACPJournalThinningRejects(t *testing.T) {
	f := newCollectFixture(t)
	candidate, _ := json.Marshal(string(f.validReturn))
	f.params.ACPOutcomePath = f.acpOutcomeWith(t,
		`{"row":"delivered","sessionId":"sess-1","stopReason":"end_turn","journalError":"disk full","candidate":`+string(candidate)+`}`)
	verdict, err := DevinCollect(f.params)
	if err != nil {
		t.Fatal(err)
	}
	if verdict.Delivered || len(verdict.Rejected) == 0 || !strings.Contains(verdict.Rejected[0], "journal thinned") {
		t.Fatalf("%+v", verdict)
	}
}
