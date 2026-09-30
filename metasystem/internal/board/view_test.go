package board

import (
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// TestBoardViewIsOneLinePerSeatInLocalTime (R23, R24, U10d): the view every
// reader prints names each armed seat of this host on one line, with its
// underway goal's stage, round of limit or proof sections and local start
// time, its Unknown goals with their reason, a seat with nothing underway
// as such; the short form counts claimed-idle and finished cards and the
// verbose form lists each; the header says bridge live or absent; a goal's
// own line names its seat.
func TestBoardViewIsOneLinePerSeatInLocalTime(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	cest := time.FixedZone("CEST", 2*60*60)
	now := time.Date(2026, 9, 29, 8, 30, 0, 0, time.UTC)
	three := 3
	seats := []Seat{{Machine: "m1b", Installation: "/c/b/metasystem"}, {Machine: "m1c", Installation: "/c/c/metasystem"}, {Machine: "m1e", Installation: "/c/e/metasystem"}}
	owner := &Owner{Pid: 42, PidStartedAt: 1000}
	put(t, home, Card{Seat: seats[0], Writer: Writer{At: now}, Goal: "goal-x", Stage: StageReview, Round: &Round{N: 2, Max: &three}, Owner: owner, Job: &Job{ID: "j", Kind: "read"},
		Since: now.Add(-18 * time.Minute), LastProgressAt: now.Add(-2 * time.Minute)})
	put(t, home, Card{Seat: seats[0], Writer: Writer{At: now}, Goal: "goal-w", Stage: StageClaimedIdle, Since: now.Add(-3 * time.Hour), LastProgressAt: now.Add(-3 * time.Hour)})
	put(t, home, Card{Seat: seats[0], Writer: Writer{At: now}, Goal: "goal-v", Stage: StageLanded, Since: now.Add(-40 * time.Minute), LastProgressAt: now.Add(-40 * time.Minute)})
	put(t, home, Card{Seat: seats[1], Writer: Writer{At: now}, Goal: "goal-y", Stage: StageUnitProof, Proof: &Proof{Attempt: "a", Done: 120, Planned: 189}, Owner: owner,
		Since: now.Add(-25 * time.Minute), LastProgressAt: now.Add(-1 * time.Minute)})
	put(t, home, Card{Seat: seats[2], Writer: Writer{At: now}, Goal: "goal-z", Stage: StageBuild, Owner: &Owner{Pid: 77, PidStartedAt: 1000},
		Since: now.Add(-60 * time.Minute), LastProgressAt: now.Add(-50 * time.Minute)})
	picture, _ := Read(home, seats, fakeProber{42: 1000}, now, 20*time.Minute)
	view := NewView(seats, CheckClaims(picture, seats, map[string]string{"goal-x": "m1b", "goal-w": "m1b", "goal-y": "m1c", "goal-z": "m1e", "goal-u": "m1c"}))
	view.Readable, view.Bridge = true, BridgeAbsent

	short := view.Lines(now, cest, false)
	want := []string{
		"board: 3 seats on this host (bridge absent)",
		"  m1b: goal-x, review round 2 of 3 since 10:12 (1 claimed idle, 1 finished)",
		"  m1c: goal-y, unit proof 120 of 189 since 10:05; goal-u unknown: no card",
		"  m1e: goal-z unknown: writer dead (pid 77) since 09:40",
	}
	if !reflect.DeepEqual(short, want) {
		t.Fatalf("short view:\n%s\nwant:\n%s", strings.Join(short, "\n"), strings.Join(want, "\n"))
	}
	verbose := view.Lines(now, cest, true)
	for _, line := range []string{"    goal-w, claimed idle since 07:30", "    goal-v, landed since 09:50", "    goal-x, review round 2 of 3 since 10:12"} {
		if !contains(verbose, line) {
			t.Errorf("verbose view lacks %q:\n%s", line, strings.Join(verbose, "\n"))
		}
	}
	if line, ok := view.GoalLine("goal-y", now, cest); !ok || line != "board: goal-y on m1c, unit proof 120 of 189 since 10:05" {
		t.Errorf("goal line: %q %v", line, ok)
	}
	if line, ok := view.GoalLine("goal-u", now, cest); !ok || line != "board: goal-u on m1c unknown: no card" {
		t.Errorf("goal line of a card-less claim: %q %v", line, ok)
	}
	if _, ok := view.GoalLine("goal-none", now, cest); ok {
		t.Error("a goal on no seat has no board line")
	}

	empty := NewView(nil, Picture{})
	empty.Readable, empty.Bridge = true, BridgeLive
	if got := empty.Lines(now, cest, false); !reflect.DeepEqual(got, []string{"board: no armed seat on this host (bridge live)"}) {
		t.Errorf("empty view: %q", got)
	}
	unreadable := View{Reason: "registry: permission denied", Bridge: BridgeAbsent}
	if got := unreadable.Lines(now, cest, false); !reflect.DeepEqual(got, []string{"board unreadable (registry: permission denied) (bridge absent)"}) {
		t.Errorf("unreadable view: %q", got)
	}
}

// TestBridgeStateIsTheSocketsPresence (R25, U10d): a one-shot view says
// bridge live when a socket stands at the bridge's path and absent
// otherwise, without connecting; a regular file there is not a bridge.
func TestBridgeStateIsTheSocketsPresence(t *testing.T) {
	t.Parallel()
	home, err := os.MkdirTemp("/tmp", "bv")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(home) })
	if got := BridgeState(home); got != BridgeAbsent {
		t.Fatalf("no socket: %s", got)
	}
	if err := os.MkdirAll(filepath.Join(home, "host"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(SocketPath(home), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if got := BridgeState(home); got != BridgeAbsent {
		t.Fatalf("a regular file: %s", got)
	}
	if err := os.Remove(SocketPath(home)); err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("unix", SocketPath(home))
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	if got := BridgeState(home); got != BridgeLive {
		t.Fatalf("a socket: %s", got)
	}
}

func contains(lines []string, want string) bool {
	for _, line := range lines {
		if line == want {
			return true
		}
	}
	return false
}

// TestVerboseBoardNamesEachGoalOnce (F5): --verbose printed a seat's
// underway goal twice, on the seat line and again on its own line. The
// verbose form gives each goal its own line and the seat line counts them.
func TestVerboseBoardNamesEachGoalOnce(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 9, 30, 9, 0, 0, 0, time.UTC)
	view := View{Readable: true, Bridge: BridgeLive, Seats: []SeatView{
		{Machine: "landing", Goals: []GoalView{}},
		{Machine: "m1e", Goals: []GoalView{{Goal: "switch-on-trial", Unknown: "not claimed", Since: now.Add(-2 * time.Hour)}}},
		{Machine: "m1b", Goals: []GoalView{{Goal: "goal-x", Stage: StageBuild, Since: now.Add(-time.Hour)}, {Goal: "goal-v", Stage: StageLanded, Since: now.Add(-time.Hour)}}},
	}}
	lines := view.Lines(now, time.UTC, true)
	for _, goalID := range []string{"switch-on-trial", "goal-x", "goal-v"} {
		count := 0
		for _, line := range lines {
			count += strings.Count(line, goalID)
		}
		if count != 1 {
			t.Errorf("--verbose names %s %d times:\n%s", goalID, count, strings.Join(lines, "\n"))
		}
	}
	want := []string{
		"board: 3 seats on this host (bridge live)",
		"  landing: nothing underway",
		"  m1e: 1 goal",
		"    switch-on-trial unknown: not claimed",
		"  m1b: 2 goals",
		"    goal-x, build since 08:00",
		"    goal-v, landed since 08:00",
	}
	if !reflect.DeepEqual(lines, want) {
		t.Errorf("verbose view:\n%s\nwant:\n%s", strings.Join(lines, "\n"), strings.Join(want, "\n"))
	}
}
