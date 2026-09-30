package lane

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
)

func refusedWith(err error, code string) bool {
	var refusal *Refusal
	return errors.As(err, &refusal) && refusal.Code == code
}

// The pause holds (design r10 K2): every gated operation reads the pause
// and the unset fence under the host flock immediately before it acts. A
// paused lane refuses every agent operation and admits a person's cleanup
// return; a seat still joins a stopped lane and waits (stop holds seats).
// An unset fences joins too. An unreadable pause or journal counts as
// present, and an operation the gate does not know is refused.
func TestGateHoldsWhilePausedAdmitsPersonCleanup(t *testing.T) {
	t.Parallel()
	home, root, _ := laneDirs(t)
	admitted := func(op Operation, authority Authority) error {
		started := 0
		err := Gate(home, op, authority, func(record Record) error {
			started++
			if record.Root != resolved(root) {
				t.Errorf("gate handed %+v to %s", record, op)
			}
			return nil
		})
		if (err == nil) != (started == 1) {
			t.Fatalf("%s by %s: err %v but started %d times", op, authority, err, started)
		}
		return err
	}
	if err := admitted(OpProve, AuthorityAgent); !refusedWith(err, CodeNotRegistered) {
		t.Fatalf("prove with no lane = %v", err)
	}
	register(t, home, root)
	agentOps := []Operation{OpBegin, OpProve, OpPublish, OpAdvance, OpValidate, OpReturn}
	for _, op := range append(agentOps, OpJoin) {
		if err := admitted(op, AuthorityAgent); err != nil {
			t.Fatalf("%s in a running lane = %v", op, err)
		}
	}
	if err := admitted("rebase", AuthorityAgent); err == nil {
		t.Fatalf("the gate admitted an operation it does not know")
	}
	pausedChecks := func(state string) {
		for _, op := range agentOps {
			if err := admitted(op, AuthorityAgent); !refusedWith(err, CodePaused) {
				t.Errorf("%s by the agent in a %s lane = %v; want %s", op, state, err, CodePaused)
			}
		}
		if err := admitted(OpJoin, AuthorityAgent); err != nil {
			t.Errorf("a join to a %s lane = %v; want it to join and wait", state, err)
		}
		if err := admitted(OpReturn, AuthorityPerson); err != nil {
			t.Errorf("a person's return in a %s lane = %v; want it admitted", state, err)
		}
		if err := admitted(OpPublish, AuthorityPerson); !refusedWith(err, CodePaused) {
			t.Errorf("a person's publish in a %s lane = %v; only cleanup returns pass a pause", state, err)
		}
	}
	if _, err := SetPause(home, "Wido", laneNow); err != nil {
		t.Fatal(err)
	}
	pausedChecks("paused")
	if err := os.WriteFile(pausePath(home), []byte("{torn"), 0o600); err != nil {
		t.Fatal(err)
	}
	pausedChecks("unreadably paused")
	if _, err := ClearPause(home); err != nil {
		t.Fatal(err)
	}
	if err := admitted(OpProve, AuthorityAgent); err != nil {
		t.Fatalf("prove after the pause cleared = %v", err)
	}
	if err := os.WriteFile(unsetPath(home), []byte("{torn"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, op := range append(agentOps, OpJoin) {
		if err := admitted(op, AuthorityAgent); !refusedWith(err, CodeUnsetting) {
			t.Errorf("%s during an unset = %v; want %s", op, err, CodeUnsetting)
		}
	}
	if err := admitted(OpReturn, AuthorityPerson); err != nil {
		t.Errorf("a person's return during an unset = %v", err)
	}
}

func emptyUnsetSeams() UnsetSeams {
	return UnsetSeams{
		Settle:    func(Layout) (Settlement, error) { return Settlement{}, nil },
		Records:   func(Layout) ([]batch.Record, error) { return nil, nil },
		Reconcile: func(Layout, batch.Record) ([]Unresolved, error) { return nil, nil },
		Return:    func(Layout, batch.Record, string) ([]Unresolved, error) { return nil, nil },
		Confirm:   func(Layout, []batch.Record) ([]Unresolved, error) { return nil, nil },
	}
}

// Settling (design r10 §1 step 2) waits for live work and for custody whose
// state is unknown; a person's --force overrides only the unknown. Until it
// settles nothing further runs, and the lane stays registered, fenced and
// paused. An unset with no lane changes nothing.
func TestUnsetSettleWaitsAndForceOverridesOnlyUnknown(t *testing.T) {
	t.Parallel()
	home, root, _ := laneDirs(t)
	if report, err := Unset(home, "Wido", laneNow, false, emptyUnsetSeams()); err != nil || !report.NoLane {
		t.Fatalf("unset with no lane = %+v %v", report, err)
	}
	if _, fenced, _ := ReadUnset(home); fenced {
		t.Fatalf("an unset with no lane fenced the host")
	}
	register(t, home, root)
	settlement, returned := Settlement{Live: []string{"batch b1 is publishing to main"}}, 0
	seams := emptyUnsetSeams()
	seams.Settle = func(Layout) (Settlement, error) { return settlement, nil }
	seams.Return = func(Layout, batch.Record, string) ([]Unresolved, error) { returned++; return nil, nil }
	for _, force := range []bool{false, true} {
		report, err := Unset(home, "Wido", laneNow, force, seams)
		if err != nil || report.Unregistered || report.Stopped != StepSettled || !strings.Contains(strings.Join(report.Settlement.Live, " "), "publishing") {
			t.Fatalf("unset (force %v) with live work = %+v %v; want it to wait", force, report, err)
		}
	}
	settlement = Settlement{Unknown: []string{"whether the owner runs is unknown"}}
	if report, err := Unset(home, "Wido", laneNow, false, seams); err != nil || report.Stopped != StepSettled {
		t.Fatalf("unset with unknown custody = %+v %v; want it to wait for --force", report, err)
	}
	if _, ok, _ := Read(home); !ok {
		t.Fatalf("an unsettled unset unregistered the lane")
	}
	if _, paused := ReadPause(home); !paused {
		t.Fatalf("the fence did not pause the lane")
	}
	if err := Gate(home, OpJoin, AuthorityAgent, nil); !refusedWith(err, CodeUnsetting) {
		t.Fatalf("join while unsetting = %v", err)
	}
	if returned != 0 {
		t.Fatalf("members were returned before custody settled")
	}
	report, err := Unset(home, "Wido", laneNow, true, seams)
	if err != nil || !report.Unregistered || !report.Resumed {
		t.Fatalf("unset --force over unknown custody = %+v %v", report, err)
	}
	if err := Gate(home, OpJoin, AuthorityAgent, nil); !refusedWith(err, CodeNotRegistered) {
		t.Fatalf("join after unset = %v; want no lane", err)
	}
}

// A lane an older engine registered (no installation, no epoch) can still
// be unset: its layout is resolved once at the fence, and a person's
// cleanup returns pass the gate that refuses every other use of the old
// record.
func TestUnsetEndsALaneAnOlderEngineRegistered(t *testing.T) {
	t.Parallel()
	home, root, _ := laneDirs(t)
	if err := os.MkdirAll(HostDir(home), 0o700); err != nil {
		t.Fatal(err)
	}
	old := `{"root":"` + resolved(root) + `","registeredBy":"m1e","at":"2026-09-29T18:00:00Z"}`
	if err := os.WriteFile(RecordPath(home), []byte(old), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Gate(home, OpProve, AuthorityAgent, nil); !refusedWith(err, CodeRecordIncomplete) {
		t.Fatalf("agent work on an old record = %v", err)
	}
	seams, returned := emptyUnsetSeams(), 0
	var gateErr error
	seams.Records = func(Layout) ([]batch.Record, error) { return []batch.Record{{BatchID: "b1"}}, nil }
	seams.Return = func(layout Layout, _ batch.Record, _ string) ([]Unresolved, error) {
		returned++
		if string(layout.Install) != resolved(root)+"/metasystem" {
			t.Errorf("returns ran on layout %+v", layout)
		}
		gateErr = Gate(home, OpReturn, AuthorityPerson, nil)
		return nil, gateErr
	}
	report, err := Unset(home, "Wido", laneNow, false, seams)
	if err != nil || !report.Unregistered || returned != 1 || gateErr != nil {
		t.Fatalf("unset of an old record = %+v %v (returned %d, gate %v); want the person's returns admitted and the lane unregistered", report, err, returned, gateErr)
	}
}

// An unset whose journal cannot be read is not stuck: with the record still
// there the journal is written again and the unset goes on, fenced all the
// while; with the record already gone (the unset ended between its two
// removals) the journal is removed and the host has no lane.
func TestUnsetRecoversAnUnreadableJournal(t *testing.T) {
	t.Parallel()
	home, root, _ := laneDirs(t)
	register(t, home, root)
	if err := os.WriteFile(unsetPath(home), []byte("{torn"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Gate(home, OpJoin, AuthorityAgent, nil); !refusedWith(err, CodeUnsetting) {
		t.Fatalf("join with an unreadable journal = %v; want the fence to hold", err)
	}
	if report, err := Unset(home, "Wido", laneNow, false, emptyUnsetSeams()); err != nil || !report.Unregistered {
		t.Fatalf("unset over an unreadable journal = %+v %v", report, err)
	}
	if err := os.WriteFile(unsetPath(home), []byte("{torn"), 0o600); err != nil {
		t.Fatal(err)
	}
	if report, err := Unset(home, "Wido", laneNow, false, emptyUnsetSeams()); err != nil || !report.NoLane {
		t.Fatalf("unset of a journal left without a record = %+v %v; want no lane", report, err)
	}
	if _, fenced, _ := ReadUnset(home); fenced {
		t.Fatalf("the ended unset's journal still fences the host")
	}
	register(t, home, root)
}
