package lane

import (
	"encoding/json"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"
)

func viewSources(home string, alive bool) ViewSources {
	return ViewSources{Home: home, Now: laneNow,
		Owner: func(string) (OwnerProbe, error) {
			if !alive {
				return OwnerProbe{}, nil
			}
			return OwnerProbe{Alive: true, PID: 4242, Since: laneNow.Add(-time.Hour)}, nil
		}}
}

func keysOf(t *testing.T, raw json.RawMessage) []string {
	t.Helper()
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil {
		t.Fatal(err)
	}
	var keys []string
	for key := range object {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// The API's shape is fixed: every key is present, absent values are null.
func TestViewShapeWithoutALane(t *testing.T) {
	t.Parallel()
	home, _, _ := laneDirs(t)
	view := BuildView(viewSources(home, false))
	data, err := json.Marshal(view)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := keysOf(t, data), []string{"batch", "next", "owner", "registered_at", "registered_by", "root", "summary", "wake"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("lane keys = %v, want %v", got, want)
	}
	var object map[string]json.RawMessage
	_ = json.Unmarshal(data, &object)
	for _, key := range []string{"root", "registered_by", "registered_at", "batch", "next", "wake"} {
		if string(object[key]) != "null" {
			t.Errorf("%s = %s; want null", key, object[key])
		}
	}
	if got, want := keysOf(t, object["owner"]), []string{"last_exit", "pid", "retry_hint", "since", "state", "stopped_by"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("owner keys = %v, want %v", got, want)
	}
	if view.Owner.State != OwnerUnready || !strings.Contains(view.Summary, "no landing lane is registered") {
		t.Fatalf("view = %+v", view)
	}
}

// A paused lane says who stopped it and what a person runs; an agent whose
// state can't be read is said so, never taken as running.
func TestViewPausedAndUnknownAgent(t *testing.T) {
	t.Parallel()
	home, root, _ := laneDirs(t)
	register(t, home, root)
	if _, err := SetPause(home, "Wido", laneNow); err != nil {
		t.Fatal(err)
	}
	view := BuildView(viewSources(home, true))
	if view.Owner.State != OwnerStopped || view.Owner.StoppedBy == nil || *view.Owner.StoppedBy != "Wido" || view.Owner.RetryHint == nil ||
		!strings.Contains(*view.Owner.RetryHint, "metasystem landing start") || !strings.Contains(view.Summary, "stopped by Wido") {
		t.Fatalf("paused view = %+v %+v", view.Owner, view.Summary)
	}
	if _, err := ClearPause(home); err != nil {
		t.Fatal(err)
	}
	sources := viewSources(home, false)
	sources.Owner = func(string) (OwnerProbe, error) { return OwnerProbe{}, os.ErrPermission }
	view = BuildView(sources)
	if view.Owner.State != OwnerUnready || view.Owner.LastExit == nil || !strings.Contains(*view.Owner.LastExit, "whether the landing agent runs is unknown") ||
		!strings.Contains(view.Summary, "unknown") {
		t.Fatalf("unknown agent view = %+v %q", view.Owner, view.Summary)
	}
}

// F-4: a registered lane whose checkout is gone tells the page the command
// that fixes it.
func TestViewOfAGoneLaneNamesTheFix(t *testing.T) {
	t.Parallel()
	home, root, _ := laneDirs(t)
	register(t, home, root)
	if err := os.RemoveAll(root); err != nil {
		t.Fatal(err)
	}
	view := BuildView(viewSources(home, false))
	if view.Root == nil || view.Owner.RetryHint == nil || !strings.Contains(*view.Owner.RetryHint, "metasystem landing set PATH") || !strings.Contains(view.Summary, "no longer exists") {
		t.Fatalf("gone lane view = %+v %q", view.Owner, view.Summary)
	}
}

// A lane record that can't be read is said in the view's unreadable field as
// well as its summary, so a reader tells it from a lane that was never
// registered; a lane with no record carries none.
func TestViewSaysALaneRecordItCannotRead(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	if err := os.MkdirAll(HostDir(home), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(RecordPath(home), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	view := BuildView(viewSources(home, false))
	if view.Root != nil || view.Unreadable == "" || !strings.Contains(view.Summary, "can't be read") {
		t.Fatalf("view of an unreadable record = %+v; want no root and the read error said", view)
	}
	if none := BuildView(viewSources(t.TempDir(), false)); none.Unreadable != "" {
		t.Fatalf("a lane never registered reads as unreadable: %q", none.Unreadable)
	}
}

// Fix round 4, sweep: a probe of the landing agent, or a check of whether
// the lane can run, that fails for a reason other than a refusal is kept as
// what the view could not read, for the status's problems; a refusal is a
// read that answered.
func TestViewKeepsWhatItCouldNotCheck(t *testing.T) {
	t.Parallel()
	home, root, _ := laneDirs(t)
	register(t, home, root)
	sources := viewSources(home, false)
	sources.Owner = func(string) (OwnerProbe, error) { return OwnerProbe{}, os.ErrPermission }
	if view := BuildView(sources); view.Owner.Unread != "whether the landing agent runs is unknown: "+os.ErrPermission.Error() {
		t.Fatalf("unread %q; want the probe's failure", view.Owner.Unread)
	}
	sources = viewSources(home, false)
	sources.Ready = func(string) error { return os.ErrPermission }
	if view := BuildView(sources); view.Owner.Unread != "whether the lane can run is unknown: "+os.ErrPermission.Error() {
		t.Fatalf("unread %q; want the readiness check's failure", view.Owner.Unread)
	}
	sources.Ready = func(root string) error { return UnarmedRefusal(root) }
	if view := BuildView(sources); view.Owner.Unread != "" {
		t.Fatalf("unread %q; want none for a refusal", view.Owner.Unread)
	}
}
