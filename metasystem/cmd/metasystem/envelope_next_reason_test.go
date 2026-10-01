package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/verbresult"
)

// A result whose line 2 is a reason with no command ("nothing to do; why")
// carries that line under --json as next with an empty argv, so a reader
// of the envelope can show the same two lines the text shows.
func TestEnvelopeCarriesANothingToDoReasonAsNext(t *testing.T) {
	t.Parallel()
	inv, out := systemMessageInvocation(t, "app status", "--json")
	inv.render(intentResult{Outcome: intentUnchanged, Summary: "the application is not running", nextReason: "nothing to do; it is stopped"})
	var envelope struct {
		Next *struct {
			Argv   []string `json:"argv"`
			Reason string   `json:"reason"`
		} `json:"next"`
	}
	if err := json.Unmarshal(out.Bytes(), &envelope); err != nil {
		t.Fatalf("unreadable envelope %q: %v", out.String(), err)
	}
	if envelope.Next == nil || envelope.Next.Argv == nil || len(envelope.Next.Argv) != 0 || envelope.Next.Reason != "nothing to do; it is stopped" {
		t.Fatalf("envelope next = %+v in %q; want an empty argv and the reason", envelope.Next, out.String())
	}
	if !strings.Contains(out.String(), `"argv": []`) {
		t.Fatalf("the empty argv is not printed as []: %q", out.String())
	}
	// The shared reader reads it, and a reader's error carries no command.
	read, err := verbresult.Read(out.Bytes(), "app status", 0, "")
	if err != nil || read.Next == nil || read.Next.Reason != "nothing to do; it is stopped" {
		t.Fatalf("verbresult.Read = %+v, %v", read, err)
	}
	// Its text is unchanged: no empty command line is added.
	text, textOut := systemMessageInvocation(t, "app status")
	text.render(intentResult{Outcome: intentUnchanged, Summary: "the application is not running", nextReason: "nothing to do; it is stopped"})
	if strings.Contains(textOut.String(), "→") {
		t.Fatalf("text gained a line 2: %q", textOut.String())
	}
	// A result with neither a command nor a reason still has no next.
	bare, bareOut := systemMessageInvocation(t, "app status", "--json")
	bare.render(intentResult{Outcome: intentConfirmed, Summary: "done"})
	if strings.Contains(bareOut.String(), `"next"`) {
		t.Fatalf("a result without line 2 printed a next: %q", bareOut.String())
	}
}

// landing run --json on an empty lane carries the text's line 2.
func TestLandingRunJSONWithAnEmptyQueueCarriesTheReason(t *testing.T) {
	t.Parallel()
	bed, _ := landingRunBed(t)
	code, stdout, stderr := bed.run(t, "landing", "run", "--json")
	var result struct {
		Outcome string
		Next    *struct {
			Argv   []string
			Reason string
		}
	}
	if code != 0 || json.Unmarshal([]byte(stdout), &result) != nil || result.Outcome != string(intentUnchanged) {
		t.Fatalf("landing run --json of an empty lane = %d %q %q", code, stdout, stderr)
	}
	if result.Next == nil || len(result.Next.Argv) != 0 || result.Next.Reason != "nothing to do; the lane is empty" {
		t.Fatalf("landing run --json next = %+v; want the reason with no command", result.Next)
	}
}

// landing run --json while its agent runs carries the reason too.
func TestLandingRunJSONWhileRunningCarriesTheReason(t *testing.T) {
	t.Parallel()
	bed, _ := landingRunBed(t)
	bed.records = []batch.Record{{BatchID: "b-one", State: batch.StateOpen, Units: []batch.Unit{{GoalID: "g-one", State: batch.UnitJoined}}}}
	if code, stdout, stderr := bed.run(t, "landing", "run"); code != 0 {
		t.Fatalf("landing run = %d %q %q", code, stdout, stderr)
	}
	code, stdout, stderr := bed.run(t, "landing", "run", "--json")
	var result struct {
		Next *struct {
			Argv   []string
			Reason string
		}
	}
	if code != 0 || json.Unmarshal([]byte(stdout), &result) != nil || result.Next == nil || len(result.Next.Argv) != 0 || result.Next.Reason != "nothing to do; it is landing b-one" {
		t.Fatalf("landing run --json again = %d %q %q; want the reason naming its batch", code, stdout, stderr)
	}
}
