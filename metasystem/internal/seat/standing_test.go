package seat

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func presence(machine string, at time.Time, tickSeconds int) Record {
	return Record{
		PresenceSchema: RecordSchema, Machine: machine, RepoIdentity: "repo-" + machine,
		Generation: 4, Engine: "3f9c1e2", ArmedLineage: NoLease, TickSeconds: tickSeconds,
		TickAt: FormatTime(at),
	}
}

func TestTheOneInequalityAtItsBoundary(t *testing.T) {
	t.Parallel()
	window := 30 * time.Minute
	// The threshold is the reader's window or three of the writer's own
	// ticks, whichever is longer: a machine that ticks every forty minutes is
	// not read dead by a reader that expected ten.
	slow := presence("m1c", fixtureClock.Add(-2*time.Hour), 2400)
	if got := Threshold(window, slow.TickSeconds); got != 2*time.Hour {
		t.Fatalf("threshold = %s; want three of the writer's ticks", got)
	}
	cases := []struct {
		name string
		age  time.Duration
		want Standing
	}{
		{"one second below the threshold", 2*time.Hour - time.Second, Reachable},
		{"at the threshold", 2 * time.Hour, Reachable},
		{"one second above the threshold", 2*time.Hour + time.Second, Unreachable},
	}
	for _, test := range cases {
		record := presence("m1c", fixtureClock.Add(-test.age), 2400)
		standing, reason := Judge(record, fixtureClock, window)
		if standing != test.want {
			t.Errorf("%s = %s (%s); want %s", test.name, standing, reason, test.want)
		}
	}
	// A brisk writer falls back on the reader's own window.
	brisk := presence("m1e", fixtureClock.Add(-31*time.Minute), 600)
	if standing, _ := Judge(brisk, fixtureClock, window); standing != Unreachable {
		t.Errorf("a brisk writer 31 minutes old = %s; want unreachable", standing)
	}
}

func TestClockAheadWithinAndBeyondTheThreshold(t *testing.T) {
	t.Parallel()
	window := 30 * time.Minute
	near := presence("m1c", fixtureClock.Add(10*time.Minute), 600)
	standing, reason := Judge(near, fixtureClock, window)
	if standing != Reachable || !strings.Contains(reason, "clock ahead by 10 min") {
		t.Fatalf("a record ten minutes ahead = %s (%s); want reachable naming the clock", standing, reason)
	}
	far := presence("m1c", fixtureClock.Add(90*time.Minute), 600)
	standing, reason = Judge(far, fixtureClock, window)
	if standing != Unknown || !strings.Contains(reason, "clock ahead by") {
		t.Fatalf("a record ninety minutes ahead = %s (%s); want unknown naming the clock", standing, reason)
	}
}

func TestTheMachineSetIsTheUnionOfRefsAndClaims(t *testing.T) {
	t.Parallel()
	copied := Copy{
		Records:   map[string]Record{"m1e": presence("m1e", fixtureClock.Add(-2*time.Minute), 600)},
		Malformed: map[string]string{"m1d": "SEAT_PRESENCE_MALFORMED: torn"},
	}
	standings := Fleet(FleetInput{
		This: "m1e", Copy: copied,
		Claims: map[string][]string{"m0b": {"fleet-channel-gateway"}, "m1e": {"goal-a"}},
		Now:    fixtureClock, Window: 30 * time.Minute,
	})
	seen := map[string]MachineStanding{}
	for _, line := range standings {
		seen[line.Machine] = line
	}
	for _, machine := range []string{"m1e", "m1d", "m0b"} {
		if _, ok := seen[machine]; !ok {
			t.Fatalf("machine %s is missing from %v", machine, seen)
		}
	}
	if standings[0].Machine != "m1e" || !standings[0].This {
		t.Fatalf("this machine is not first: %+v", standings[0])
	}
	if seen["m0b"].Standing != Unknown || seen["m0b"].Reason != "no presence" {
		t.Fatalf("a claiming machine that never published = %+v", seen["m0b"])
	}
	if seen["m1d"].Standing != Unknown || seen["m1d"].Reason != "malformed presence" {
		t.Fatalf("a malformed record = %+v", seen["m1d"])
	}
	if seen["m1e"].Standing != Reachable {
		t.Fatalf("this machine = %+v", seen["m1e"])
	}
}

func TestSilentHoldersAreFlaggedAndClaimsUnavailableRaisesNone(t *testing.T) {
	t.Parallel()
	copied := Copy{Records: map[string]Record{"m1c": presence("m1c", fixtureClock.Add(-6*time.Hour), 600)}}
	claims := map[string][]string{"m1c": {"tests-parallel-and-deterministic"}}
	standings := Fleet(FleetInput{This: "m1e", Copy: copied, Claims: claims, Now: fixtureClock, Window: 30 * time.Minute})
	var flagged string
	for _, line := range standings {
		if line.Machine == "m1c" {
			flagged = line.Flag
		}
	}
	if !strings.Contains(flagged, "held by a machine unreachable since") {
		t.Fatalf("flag = %q; want the silent-holder words", flagged)
	}
	blind := Fleet(FleetInput{This: "m1e", Copy: copied, Claims: claims,
		ClaimsUnavailable: "the accepted ledger tip is unreadable", Now: fixtureClock, Window: 30 * time.Minute})
	for _, line := range blind {
		if line.Flag != "" {
			t.Fatalf("an unreadable tip raised the flag anyway: %+v", line)
		}
	}
}

func TestSinceIsFrozenAtTheFirstObservationOfAStanding(t *testing.T) {
	t.Parallel()
	copied := Copy{Records: map[string]Record{"m1c": presence("m1c", fixtureClock.Add(-6*time.Hour), 600)}}
	first := Fleet(FleetInput{This: "m1e", Copy: copied, Now: fixtureClock, Window: 30 * time.Minute})
	frozen := Observations(first)
	later := Fleet(FleetInput{This: "m1e", Copy: copied, Previous: frozen,
		Now: fixtureClock.Add(time.Hour), Window: 30 * time.Minute})
	for _, line := range later {
		if line.Machine != "m1c" {
			continue
		}
		if line.Since != frozen["m1c"].Since {
			t.Fatalf("since moved from %q to %q", frozen["m1c"].Since, line.Since)
		}
	}
	// A change of standing takes the new observation's own time.
	back := Copy{Records: map[string]Record{"m1c": presence("m1c", fixtureClock.Add(time.Hour-time.Minute), 600)}}
	recovered := Fleet(FleetInput{This: "m1e", Copy: back, Previous: frozen,
		Now: fixtureClock.Add(time.Hour), Window: 30 * time.Minute})
	for _, line := range recovered {
		if line.Machine == "m1c" && line.Since == frozen["m1c"].Since {
			t.Fatalf("a changed standing kept the old since %q", line.Since)
		}
	}
}

func TestTheReaderJoinsBothNamespacesNewestTickWinning(t *testing.T) {
	t.Parallel()
	stale := Copy{Records: map[string]Record{"m1c": presence("m1c", fixtureClock.Add(-6*time.Hour), 600)}}
	live := Copy{Records: map[string]Record{"m1c": presence("m1c", fixtureClock.Add(-2*time.Minute), 600)}}
	joined := Join(stale, live)
	if joined.Records["m1c"].TickAt != live.Records["m1c"].TickAt {
		t.Fatalf("join kept the stale rung: %+v", joined.Records["m1c"])
	}
	joined = Join(live, stale)
	if joined.Records["m1c"].TickAt != live.Records["m1c"].TickAt {
		t.Fatalf("join order changed the winner: %+v", joined.Records["m1c"])
	}
	// A readable record on one rung beats a malformed one on the other.
	torn := Copy{Malformed: map[string]string{"m1c": "SEAT_PRESENCE_MALFORMED: torn"}}
	joined = Join(torn, live)
	if _, refused := joined.Malformed["m1c"]; refused {
		t.Fatalf("a readable rung was shadowed by a malformed one: %+v", joined)
	}
	// With nothing readable anywhere the refusal stands.
	joined = Join(torn, Copy{})
	if joined.Malformed["m1c"] == "" {
		t.Fatalf("a malformed record disappeared: %+v", joined)
	}
}

func TestFleetTextAndJSONComeFromTheInjectedClock(t *testing.T) {
	t.Parallel()
	running := "2026-09-24T09:30:05Z"
	mine := presence("m1e", fixtureClock.Add(-2*time.Minute), 600)
	mine.Chain = &Chain{Root: "job-a", Job: "job-b", Role: "implementer", Round: 2, Goal: "finish-test-repairs-and-integrate", StartedAt: &running}
	copied := Copy{Records: map[string]Record{
		"m1e": mine,
		"m1c": presence("m1c", fixtureClock.Add(-6*time.Hour), 600),
	}}
	report := Report{}
	report.SetNow(fixtureClock)
	report.CopySource, report.CopyReadAt = "the tick", FormatTime(fixtureClock.Add(-2*time.Minute))
	report.This = "m1e"
	report.Machines = Fleet(FleetInput{
		This: "m1e", Copy: copied,
		Claims: map[string][]string{"m1c": {"tests-parallel-and-deterministic"}, "m0b": {"fleet-channel-gateway"}},
		Now:    fixtureClock, Window: 30 * time.Minute,
	})
	state := PublicationState{Schema: 1, LastOutcome: OutcomePublished, LastSuccessAt: FormatTime(fixtureClock.Add(-2 * time.Minute)), Rung: 1}
	report.Publication = &state
	text := report.Text()
	for _, want := range []string{
		"m1e  reachable  2 min ago  generation 4  engine 3f9c1e2  running implementer round 2 on finish-test-repairs-and-integrate",
		"m1c  unreachable  6 h ago",
		"held by a machine unreachable since",
		"m0b  unknown  no presence; named by the claim on fleet-channel-gateway",
		"presence of this machine: published 2 min ago on rung 1, a metasystem ref",
		"presence copy fetched 2 min ago by the tick",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("fleet text is missing %q:\n%s", want, text)
		}
	}
	encoded, err := report.JSON()
	if err != nil {
		t.Fatal(err)
	}
	var back struct {
		Machines []struct {
			Machine    string   `json:"machine"`
			Standing   string   `json:"standing"`
			AgeSeconds *int64   `json:"ageSeconds"`
			Holds      []string `json:"holds"`
			Flag       string   `json:"flag"`
			Record     *Record  `json:"record"`
		} `json:"machines"`
		Now string `json:"now"`
	}
	if err := json.Unmarshal(encoded, &back); err != nil {
		t.Fatal(err)
	}
	if back.Now != "2026-09-24T09:40:00Z" {
		t.Fatalf("json now = %q", back.Now)
	}
	if len(back.Machines) != 3 || back.Machines[0].Machine != "m1e" {
		t.Fatalf("json machines = %+v", back.Machines)
	}
	if back.Machines[0].AgeSeconds == nil || *back.Machines[0].AgeSeconds != 120 {
		t.Fatalf("json age = %v", back.Machines[0].AgeSeconds)
	}
	if back.Machines[0].Record == nil || back.Machines[0].Record.Chain == nil {
		t.Fatalf("json record = %+v", back.Machines[0].Record)
	}
}
