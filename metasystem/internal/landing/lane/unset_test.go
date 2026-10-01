package lane

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func emptyUnsetSeams() UnsetSeams {
	return UnsetSeams{Settle: func(Layout) (Settlement, error) { return Settlement{}, nil }}
}

// Settling (design r10 §1 step 2) waits for a live agent and for state that
// is unknown; a person's --force overrides only the unknown. Until it
// settles the lane stays registered, fenced and paused. An unset with no
// lane changes nothing.
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
	settlement := Settlement{Live: []string{"the landing agent landing-1 still runs"}}
	seams := emptyUnsetSeams()
	seams.Settle = func(Layout) (Settlement, error) { return settlement, nil }
	for _, force := range []bool{false, true} {
		report, err := Unset(home, "Wido", laneNow, force, seams)
		if err != nil || report.Unregistered || report.Stopped != StepSettled || !strings.Contains(strings.Join(report.Settlement.Live, " "), "still runs") {
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
	if _, fenced, _ := ReadUnset(home); !fenced {
		t.Fatalf("an unsettled unset is not fenced")
	}
	report, err := Unset(home, "Wido", laneNow, true, seams)
	if err != nil || !report.Unregistered || !report.Resumed {
		t.Fatalf("unset --force over unknown custody = %+v %v", report, err)
	}
	if _, ok, _ := Read(home); ok {
		t.Fatalf("the lane is still registered after the unset")
	}
}

// A lane an older engine registered (no installation, no epoch) can still
// be unset: its layout is resolved once at the fence.
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
	seams, settled := emptyUnsetSeams(), 0
	seams.Settle = func(layout Layout) (Settlement, error) {
		settled++
		if string(layout.Install) != resolved(root)+"/metasystem" {
			t.Errorf("settle ran on layout %+v", layout)
		}
		return Settlement{}, nil
	}
	report, err := Unset(home, "Wido", laneNow, false, seams)
	if err != nil || !report.Unregistered || settled != 1 {
		t.Fatalf("unset of an old record = %+v %v (settled %d); want the lane unregistered", report, err, settled)
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
	if _, fenced, _ := ReadUnset(home); !fenced {
		t.Fatalf("an unreadable journal does not fence the host")
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

// A lane whose checkout is gone holds nothing an unset could read or
// return: the unset unregisters it at once and says so, whether the record
// is an older engine's or the checkout vanished during the unset. No
// command loops on it.
func TestUnsetOfALaneWhoseCheckoutIsGone(t *testing.T) {
	t.Parallel()
	home, root, second := laneDirs(t)
	if err := os.MkdirAll(HostDir(home), 0o700); err != nil {
		t.Fatal(err)
	}
	old := `{"root":"` + resolved(root) + `","registeredBy":"m1e","at":"2026-09-29T18:00:00Z"}`
	if err := os.WriteFile(RecordPath(home), []byte(old), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(root); err != nil {
		t.Fatal(err)
	}
	report, err := Unset(home, "Wido", laneNow, false, emptyUnsetSeams())
	if err != nil || !report.Unregistered || !report.CheckoutGone {
		t.Fatalf("unset of an old lane whose checkout is gone = %+v %v", report, err)
	}
	register(t, home, second)
	seams := emptyUnsetSeams()
	seams.Settle = func(Layout) (Settlement, error) { return Settlement{Live: []string{"the landing agent runs"}}, nil }
	if report, err := Unset(home, "Wido", laneNow, false, seams); err != nil || report.Stopped != StepSettled {
		t.Fatalf("unset = %+v %v", report, err)
	}
	if err := os.RemoveAll(second); err != nil {
		t.Fatal(err)
	}
	report, err = Unset(home, "Wido", laneNow, false, seams)
	if err != nil || !report.Unregistered || !report.CheckoutGone {
		t.Fatalf("resumed unset after the checkout vanished = %+v %v", report, err)
	}
	if _, ok, _ := Read(home); ok {
		t.Fatalf("the lane is still registered")
	}
	if _, paused := ReadPause(home); paused {
		t.Fatalf("the unset left its pause behind")
	}
}

// An older engine's lane whose checkout landing set would refuse (two
// installations in it) is not sent round to landing set: unset says what is
// wrong with the checkout and to run landing unset once it is fixed.
func TestUnsetOfAnOlderLaneSetWouldRefuseSaysWhatToDo(t *testing.T) {
	t.Parallel()
	home, root, _ := laneDirs(t)
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(HostDir(home), 0o700); err != nil {
		t.Fatal(err)
	}
	old := `{"root":"` + resolved(root) + `","registeredBy":"m1e","at":"2026-09-29T18:00:00Z"}`
	if err := os.WriteFile(RecordPath(home), []byte(old), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := Unset(home, "Wido", laneNow, false, emptyUnsetSeams())
	var refusal *Refusal
	if !errors.As(err, &refusal) || strings.Join(refusal.Argv, " ") != "metasystem landing unset" || !strings.Contains(refusal.Message, "two MetaSystem installations") {
		t.Fatalf("unset of an old lane set would refuse = %v; want what to fix and landing unset", err)
	}
}
