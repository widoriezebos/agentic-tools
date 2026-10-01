package lane

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var laneNow = time.Date(2026, 9, 29, 18, 0, 0, 0, time.UTC)

func laneDirs(t *testing.T) (home, first, second string) {
	t.Helper()
	base := t.TempDir()
	home, first, second = filepath.Join(base, "home"), filepath.Join(base, "landing-a"), filepath.Join(base, "landing-b")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatal(err)
	}
	// Both landing checkouts are nested: the checkout root is not the
	// installation root, the shape the lane runs in.
	laneCheckout(t, first, true)
	laneCheckout(t, second, true)
	return home, first, second
}

// A seat's setting never registers a lane (design r10 §1): with no record
// every seat, whatever it names, has no lane; once a person registers one,
// a seat naming it or naming nothing uses it and nothing is rewritten.
func TestResolveNeverRegistersAndUsesThePersonsLane(t *testing.T) {
	t.Parallel()
	home, root, _ := laneDirs(t)
	for _, seat := range []string{root, ""} {
		none, err := Resolve(home, seat)
		if err != nil || none.Root != "" {
			t.Fatalf("seat %q with nothing registered = %+v, %v; want no lane", seat, none, err)
		}
	}
	if _, err := os.Stat(RecordPath(home)); err == nil {
		t.Fatalf("a seat's resolution wrote the lane record")
	}
	register(t, home, root)
	info, err := os.Stat(RecordPath(home))
	if err != nil {
		t.Fatal(err)
	}
	for _, seat := range []string{root + string(filepath.Separator), ""} {
		got, err := Resolve(home, seat)
		if err != nil || got.Root != resolved(root) || got.FromHost != (seat == "") {
			t.Fatalf("seat %q = %+v, %v; want the person's lane", seat, got, err)
		}
	}
	record, ok, err := Read(home)
	if err != nil || !ok || record.RegisteredBy != "m1e" || record.At != laneNow.Format(time.RFC3339) {
		t.Fatalf("record = %+v %v %v; want the registration kept", record, ok, err)
	}
	after, _ := os.Stat(RecordPath(home))
	if !after.ModTime().Equal(info.ModTime()) {
		t.Fatalf("a seat's resolution rewrote the record")
	}
}

// A seat whose setting names another checkout is refused in plain words
// naming both paths and both ways out; the record is unchanged.
func TestResolveRefusesAnotherSeatRoot(t *testing.T) {
	t.Parallel()
	home, first, second := laneDirs(t)
	register(t, home, first)
	_, err := Resolve(home, second)
	var refusal *Refusal
	if !errors.As(err, &refusal) || refusal.Code != CodeMismatch {
		t.Fatalf("err = %v; want %s", err, CodeMismatch)
	}
	text := err.Error() + " " + refusal.Fix
	for _, want := range []string{resolved(first), resolved(second), "registered by m1e", "metasystem settings set landing.batch-root " + resolved(first), "metasystem landing set " + resolved(second)} {
		if !strings.Contains(text, want) {
			t.Errorf("refusal %q lacks %q", text, want)
		}
	}
	record, _, _ := Read(home)
	if record.Root != resolved(first) {
		t.Fatalf("record moved to %s", record.Root)
	}
}

// A registered checkout that is gone is reported, never replaced by a seat's
// other setting.
func TestResolveReportsGoneRoot(t *testing.T) {
	t.Parallel()
	home, first, second := laneDirs(t)
	register(t, home, first)
	if err := os.RemoveAll(first); err != nil {
		t.Fatal(err)
	}
	for _, seat := range []string{"", first, second} {
		_, err := Resolve(home, seat)
		var refusal *Refusal
		if !errors.As(err, &refusal) || refusal.Code != CodeGone || !strings.Contains(err.Error(), first) || !strings.Contains(refusal.Fix, "metasystem landing set PATH") {
			t.Fatalf("seat %q: err = %v; want %s naming %s", seat, err, CodeGone, first)
		}
	}
	record, _, _ := Read(home)
	if record.Root != resolved(first) {
		t.Fatalf("a gone lane was replaced by %s", record.Root)
	}
}

// Register moves the lane on a person's word, is unchanged on a repeat, and
// refuses while an unset is under way.
func TestRegisterMovesUnchangedAndRefuses(t *testing.T) {
	t.Parallel()
	home, first, second := laneDirs(t)
	layoutOf := func(root string) Layout {
		layout, err := NewLayout(root)
		if err != nil {
			t.Fatal(err)
		}
		return layout
	}
	if _, changed, err := Register(home, layoutOf(first), "Wido", laneNow); err != nil || !changed {
		t.Fatalf("first register = %v, %v", changed, err)
	}
	if _, changed, err := Register(home, layoutOf(first), "Wido", laneNow.Add(time.Minute)); err != nil || changed {
		t.Fatalf("repeat = %v, %v; want unchanged", changed, err)
	}
	previous, changed, err := Register(home, layoutOf(second), "Wido", laneNow.Add(time.Hour))
	if err != nil || !changed || previous.Root != resolved(first) {
		t.Fatalf("move = %+v %v %v", previous, changed, err)
	}
	var refusal *Refusal
	if _, _, err := Register(home, Layout{}, "Wido", laneNow); !errors.As(err, &refusal) || refusal.Code != CodeRegisterInvalid {
		t.Fatalf("register of no layout = %v; want %s", err, CodeRegisterInvalid)
	}
	seams := emptyUnsetSeams()
	seams.Settle = func(Layout) (Settlement, error) { return Settlement{Live: []string{"a proof runs"}}, nil }
	if report, err := Unset(home, "Wido", laneNow, false, seams); err != nil || report.Stopped != StepSettled {
		t.Fatalf("unset = %+v %v", report, err)
	}
	if _, _, err := Register(home, layoutOf(first), "Wido", laneNow); !errors.As(err, &refusal) || refusal.Code != CodeUnsetting || !strings.Contains(refusal.Fix, "metasystem landing unset") {
		t.Fatalf("register during an unset = %v; want %s naming landing unset", err, CodeUnsetting)
	}
}
