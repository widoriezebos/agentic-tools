package launch

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/tickrequest"
)

func TestSeatTerminalTransitionRequestsItsOwningSteward(t *testing.T) {
	t.Parallel()
	for _, state := range []State{Completed, Failed, Cancelled} {
		t.Run(string(state), func(t *testing.T) {
			t.Parallel()
			root, other := t.TempDir(), t.TempDir()
			store := Store{Root: t.TempDir()}
			data, _ := json.Marshal(root)
			record := Record{ID: "seat-one", Kind: "seat", State: Starting, WorkingDirectory: other, AdapterData: map[string]json.RawMessage{"fenceRoot": data}}
			if err := store.Create(record); err != nil {
				t.Fatal(err)
			}
			if _, err := store.Update(record.ID, func(r *Record) error { r.State = Running; return nil }); err != nil {
				t.Fatal(err)
			}
			if _, err := tickrequest.Read(root); !os.IsNotExist(err) {
				t.Fatal("start requested a tick")
			}
			if _, err := store.Update(record.ID, func(r *Record) error { r.State = state; return nil }); err != nil {
				t.Fatal(err)
			}
			request, err := tickrequest.Read(root)
			if err != nil || request.Reason != "launch-ended:seat-one" {
				t.Fatalf("request=%+v err=%v", request, err)
			}
			if _, err := tickrequest.Read(other); !os.IsNotExist(err) {
				t.Fatal("working directory got request instead of owner")
			}
			if err := tickrequest.Acknowledge(root, request); err != nil {
				t.Fatal(err)
			}
			if _, err := store.Update(record.ID, func(r *Record) error { r.Measured = true; return nil }); err != nil {
				t.Fatal(err)
			}
			if _, err := tickrequest.Read(root); !os.IsNotExist(err) {
				t.Fatal("terminal metadata requested another tick")
			}
		})
	}
}

func TestUnitEndRequestsAdmittingCheckout(t *testing.T) {
	fixture := newUnitFixture(t, "")
	root := t.TempDir()
	fixture.runner.Installation = root
	result, err := fixture.runner.Advance(UnitRequest{Plan: fixture.plan})
	if err != nil {
		t.Fatal(err)
	}
	if result.Record.State != "awaiting-judgement" {
		t.Fatalf("unit did not end: %s", result.Record.State)
	}
	request, err := tickrequest.Read(root)
	if err != nil || request.Reason != "unit-ended:"+result.Record.ID {
		t.Fatalf("request=%+v err=%v", request, err)
	}
	if err := tickrequest.Acknowledge(root, request); err != nil {
		t.Fatal(err)
	}
	fixture.runner.Installation = t.TempDir()
	if _, err := fixture.runner.Advance(UnitRequest{Resume: result.Record.ID}); err != nil {
		t.Fatal(err)
	}
	if _, err := tickrequest.Read(root); !os.IsNotExist(err) {
		t.Fatal("re-reading ended unit requested a cycle")
	}
	if _, err := tickrequest.Read(fixture.runner.Installation); !os.IsNotExist(err) {
		t.Fatal("request was routed to another caller")
	}
}

func TestSeatCompletionSurvivesRequestWriteFailure(t *testing.T) {
	t.Parallel()
	store := Store{Root: t.TempDir()}
	root := t.TempDir() + "/unavailable"
	if err := os.WriteFile(root, []byte("blocked"), 0600); err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(root)
	if err := store.Create(Record{ID: "seat", Kind: "seat", State: Running, AdapterData: map[string]json.RawMessage{"fenceRoot": data}}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Update("seat", func(r *Record) error { r.State = Completed; r.FinishedAt = time.Now().Format(time.RFC3339); return nil }); err != nil {
		t.Fatal(err)
	}
	r, err := store.Read("seat")
	if err != nil || r.State != Completed {
		t.Fatalf("completion lost: %+v %v", r, err)
	}
}
