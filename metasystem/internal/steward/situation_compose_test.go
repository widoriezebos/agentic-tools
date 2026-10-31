package steward

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/board"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/enginepin"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/hostload"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
)

// composePublish publishes one board message at the time given.
func composePublish(t *testing.T, home string, request board.Request, at time.Time) board.Message {
	t.Helper()
	published, err := board.Publish(home, request, at)
	if err != nil {
		t.Fatal(err)
	}
	return published.Message
}

// composeSources read root's ledger with the live lane reader and the board
// at home, on an injected empty host, at *now.
func composeSources(t *testing.T, root, home string, now *time.Time) SituationSources {
	t.Helper()
	return withEmptyHost(t, SituationSources{Now: func() time.Time { return *now }, LaneHome: func() (string, error) { return t.TempDir(), nil },
		ClaimLaneState: claimQuotaLane(t, root, nil), BoardHome: func() (string, error) { return home, nil }})
}

func TestTheFirstThingIsTheOldestOutstandingAsk(t *testing.T) {
	t.Parallel()
	root, home := situationQuotaBed(t), t.TempDir()
	at := func(d time.Duration) time.Time { return situationNow.Add(d) }
	if err := askOnce(askedReceipts(root), home, board.Sender{Machine: "m1x"}, board.Address{Goal: "held-h"}, "release held-h", "Release held-h?", at(-40*time.Minute)); err != nil {
		t.Fatal(err)
	}
	free := "steward-" + signalDigest("release held-h")[:24]
	early := composePublish(t, home, board.Request{Kind: board.KindAsk, From: board.Sender{Machine: "m1x"}, To: board.Address{Machine: "m1e"}, Text: "Is it built?", Deadline: 30 * time.Minute}, at(-20*time.Minute))
	late := composePublish(t, home, board.Request{Kind: board.KindAsk, From: board.Sender{Machine: "m1x"}, To: board.Address{Machine: "m1e"}, Text: "Is it proven?", Deadline: 30 * time.Minute}, at(-5*time.Minute))
	composePublish(t, home, board.Request{Kind: board.KindAsk, From: board.Sender{Machine: "m1e"}, To: board.Address{Machine: "m1x"}, Text: "Can you read it?"}, at(-time.Hour))

	// The claim quota's own writer asks from this machine to its held goal.
	ledger := newHealthLedger(root, at(-10*time.Minute))
	ledger.laneState = claimQuotaLane(t, root, nil)
	role := checkClaimQuotaWith(root, root, ledger)
	if err := askClaimQuotaHolders(HealthVerdict{Roles: []RoleVerdict{role}}, root, home, goal.ResolveMachine, at(-10*time.Minute)); err != nil || role.Status != HealthDead {
		t.Fatalf("the quota writer: %v (role %+v)", err, role)
	}
	quota := map[string]bool{}
	for _, ask := range claimQuotaAsks(t, home, "held-h") {
		if ask.From.Machine == "m1e" {
			quota[ask.ID] = true
		}
	}
	if len(quota) != 3 {
		t.Fatalf("the quota writer asked %v, want three asks", quota)
	}

	now := situationNow
	src := composeSources(t, root, home, &now)
	first := func() FirstThing {
		t.Helper()
		return Compose(readSituation(t, root, src)).First
	}
	reply := func(thread, to string) {
		t.Helper()
		composePublish(t, home, board.Request{Kind: board.KindReply, From: board.Sender{Machine: "m1e"}, To: board.Address{Machine: to}, Thread: thread, Text: "Done."}, now)
	}
	if got := first(); got.Kind != "ask" || got.Subject != free || got.Command != "metasystem agent inbox" || !strings.Contains(got.Why, "no deadline") || !strings.Contains(got.Why, "40m ago") {
		t.Fatalf("at the read: %+v, want the oldest incoming ask %s, deadline-free, not the older outgoing one", got, free)
	}
	now = at(11 * time.Minute)
	if got := first(); got.Kind != "ask" || got.Subject != early.ID || !strings.Contains(got.Why, "past its deadline") {
		t.Fatalf("at +11m: %+v, want the expired ask %s first", got, early.ID)
	}
	reply(early.ID, "m1x")
	if got := first(); got.Subject != free {
		t.Fatalf("the expired ask replied: %+v, want the deadline-free ask %s again", got, free)
	}
	reply(free, "m1x")
	reply(late.ID, "m1x")
	if got := first(); got.Kind != "ask" || !quota[got.Subject] {
		t.Fatalf("the quota writer's asks left: %+v, want one of %v, incoming work before the claim", got, quota)
	}
	for id := range quota {
		reply(id, "m1e")
	}
	if got := first(); got.Kind != "continue" || got.Subject != "held-h" || got.Command != "metasystem goal claim held-h" {
		t.Fatalf("every incoming ask replied: %+v, want to continue held-h", got)
	}

	red := goal.TrunkRedEntry{ID: "tr-fast", Identity: "tr-fast", Group: "fast", Status: "failed", Failures: []goal.TrunkRedFailure{},
		Sightings: []goal.TrunkRedSighting{{Attempt: "attempt-1", Batch: "batch-1", BaseCommit: "base-1", SeenAt: "2026-08-24T08:00:00Z", Opid: goal.Opid("01J5X0000000000000000000R0", "m1e", "lin-1")}},
		Owner:     goal.TrunkRedOwner{Machine: "m1e", Since: "2026-08-24T08:00:00Z", How: "joiner"}, Holds: []string{"batch-1"}, Opened: "2026-08-24T08:00:00Z"}
	reds := claimQuotaTickBed(t, map[string][]byte{"plans/goals/trunk-red.json": goal.RenderTrunkRed([]goal.TrunkRedEntry{red})}, claimQuotaHeld("held-h"))
	redSrc := composeSources(t, reds, t.TempDir(), &now)
	redSrc.RedLine = func(entry goal.TrunkRedEntry, at time.Time) string {
		return "the red " + entry.ID + " read at " + at.Format(time.RFC3339)
	}
	if got := Compose(readSituation(t, reds, redSrc)).First; got.Kind != "red" || got.Subject != "tr-fast" || got.Why != "the red tr-fast read at "+now.Format(time.RFC3339) || got.Command != "" {
		t.Fatalf("an owned red: %+v, want it before the claim, worded by goal next's line", got)
	}

	ready := claimQuotaTickBed(t, nil, claimQuotaReady("ready-r", 1, "m1e"))
	if got := Compose(readSituation(t, ready, composeSources(t, ready, t.TempDir(), &now))).First; got.Kind != "claim" || got.Subject != "ready-r" || got.Command != "metasystem goal claim ready-r" {
		t.Fatalf("no claim and one ready goal: %+v, want to claim ready-r", got)
	}
	empty := claimQuotaTickBed(t, nil)
	emptySrc := composeSources(t, empty, t.TempDir(), &now)
	if got := Compose(readSituation(t, empty, emptySrc)).First; got.Kind != "wait" || got.Why != "nothing; wait" || got.Command != "" {
		t.Fatalf("nothing to do: %+v, want wait", got)
	}
	emptySrc.BoardHome = func() (string, error) { return "", errors.New("no board home") }
	page := Compose(readSituation(t, empty, emptySrc))
	if got := page.First; got.Kind != "unknown" || !strings.HasPrefix(got.Why, "unknown: asks unavailable") {
		t.Fatalf("the asks unreadable: %+v, want unknown, never wait", got)
	}
	if line := page.Parts[0].Summary[0]; line != page.First.Why {
		t.Fatalf("the page's first line is %q, want the unknown said once: %q", line, page.First.Why)
	}
}

// composeLane makes the registered lane's checkout a Git checkout and reads
// its status as landing status does, over the given host observation.
func composeLane(t *testing.T, laneInstall string, hostWork func() (int, int, error), owner lane.OwnerView) func(string, time.Time) plain.Status {
	t.Helper()
	if err := os.MkdirAll(laneInstall, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(laneInstall, "metasystem.conf"), []byte("metasystem.template=true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"init", "-q", "-b", "main"}, {"add", "metasystem/metasystem.conf"}, {"-c", "user.name=fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "lane"},
		{"update-ref", "refs/remotes/origin/main", "HEAD"}, {"config", "metasystem.goal.machine", "m1e"}} {
		seatGit(t, filepath.Dir(laneInstall), args...)
	}
	return func(home string, at time.Time) plain.Status {
		record, _, err := lane.Read(home)
		if err != nil {
			t.Fatal(err)
		}
		return plain.ReadStatus(home, record, lane.View{Root: &record.Root, Owner: owner}, plain.ProveSeams{Now: func() time.Time { return at }, HostWork: hostWork})
	}
}

func TestTheWakeFieldsChangeOnlyOnSemanticEvents(t *testing.T) {
	t.Parallel()
	root, laneHome, laneInstall := situationRecordsBed(t)
	home := t.TempDir()
	store := launch.Store{Root: filepath.Join(t.TempDir(), "launch")}
	for _, record := range []launch.Record{{ID: "host-build", Kind: "build", State: launch.Running}, {ID: "read-2", Kind: "read", Goal: "g1", State: launch.Running}} {
		if err := store.Create(record); err != nil {
			t.Fatal(err)
		}
	}
	manager := &launch.Manager{Store: store}
	laneStatus := composeLane(t, laneInstall, manager.HostWork, lane.OwnerView{State: "running"})
	held := func(at time.Time) {
		t.Helper()
		if err := plain.RecordIntervention(laneInstall, plain.Intervention{At: at.Format(time.RFC3339Nano), Row: "proof-beside-builds", Subject: "main", Outcome: "held: 1 builds and 0 unit checks run"}); err != nil {
			t.Fatal(err)
		}
	}
	held(situationNow.Add(-10 * time.Minute))
	// A pause an hour old fires a row whose minutes advance.
	if _, err := lane.SetPauseBecause(laneHome, "Wido", "a test pause", situationNow.Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := RecordSeatIdleIncident(root, SeatIdleIncident{SessionID: "s-wake", BacklogDigest: "ledger-unreadable", Refusal: 3, IntentID: "intent-wake"}, situationNow); err != nil {
		t.Fatal(err)
	}

	now, load, start := situationNow.Add(time.Minute), 3.0, situationNow.Add(-time.Hour)
	prober := &situationProber{starts: map[int64]time.Time{101: start}, calls: map[int64]int{}}
	src := situationHostSources(t, situationRegistry(t, root))
	src.Now = func() time.Time { return now }
	src.Load = func(at time.Time) hostload.Sample {
		return hostload.Sample{At: at.Format(time.RFC3339), Load1m: load, Available: true}
	}
	src.Processes = identity.FixedProcessTable{{Pid: 101, Group: 101, Session: 101, Parent: 1}}
	src.Argv = func(int64) ([]string, bool) { return []string{"go", "test", "./..."}, true }
	src.Cwd = func(int64) (string, bool) { return root, true }
	src.Prober = prober
	src.LaneHome = func() (string, error) { return laneHome, nil }
	src.HostWork = manager.HostWork
	src.LaneStatus = laneStatus
	src.ClaimLaneState = claimQuotaLane(t, root, nil)
	src.BoardHome = func() (string, error) { return home, nil }
	src.Launches = manager.EffectiveList
	read := func() (Situation, Composition) {
		t.Helper()
		s := readSituation(t, root, src)
		return s, Compose(s)
	}
	minutes := func(s Situation) string {
		if s.Lane.Health == nil {
			t.Fatalf("the lane has no health: %v %v", s.Lane.Problems, s.Unavailable)
		}
		for _, fired := range s.Lane.Health.Rows() {
			if fired.Row == "paused" {
				return fired.Minutes
			}
		}
		return ""
	}

	s1, c1 := read()
	now, load, prober.starts[101] = situationNow.Add(2*time.Minute), 9, start.Add(-time.Hour)
	s2, c2 := read()
	if m1, m2 := minutes(s1), minutes(s2); m1 == "" || m1 == m2 || s1.Load.Load1m == s2.Load.Load1m || len(s2.Processes) != 1 || s1.Processes[0].Started.Equal(s2.Processes[0].Started) {
		t.Fatalf("the two reads do not differ in presentation: minutes %q %q, load %v %v, processes %+v %+v", m1, m2, s1.Load, s2.Load, s1.Processes, s2.Processes)
	}
	if c1.Digest == "" || c1.Digest != c2.Digest || !slices.Equal(c1.Wake, c2.Wake) {
		t.Fatalf("a new load sample, process start and lane minutes woke:\n%v\n%v", c1.Wake, c2.Wake)
	}
	for _, want := range []string{"hold|main", "row|proof-beside-builds|main"} {
		if !slices.Contains(c2.Wake, want) {
			t.Fatalf("the wake lacks %q: %v", want, c2.Wake)
		}
	}
	seen := map[string]bool{c2.Digest: true}
	step := func(event string) (Situation, Composition) {
		t.Helper()
		s, c := read()
		if seen[c.Digest] {
			t.Fatalf("%s: the digest did not change or came back:\n%v", event, c.Wake)
		}
		seen[c.Digest] = true
		return s, c
	}

	if err := humanauthority.AppendAttorneyLog(root, "answered 26"); err != nil {
		t.Fatal(err)
	}
	if s, _ := step("an attorney line"); len(s.Answered) != 20 {
		t.Fatalf("the attorney log kept %d lines, want 20", len(s.Answered))
	}
	ask := composePublish(t, home, board.Request{Kind: board.KindAsk, From: board.Sender{Machine: "m1x"}, To: board.Address{Machine: "m1e"}, Text: "Is it built?", Deadline: 15 * time.Minute}, now)
	_, c := step("a new ask")
	if want := []time.Time{*ask.DeadlineAt, situationNow.Add(20 * time.Minute)}; !slices.EqualFunc(c.Deadlines, want, time.Time.Equal) {
		t.Fatalf("deadlines = %v, want the ask's and the hold's %v", c.Deadlines, want)
	}
	now = ask.DeadlineAt.Add(time.Minute)
	if _, c = step("the ask's deadline"); !slices.Contains(c.Wake, "ask|"+ask.ID+"|expired") {
		t.Fatalf("past the deadline with no file change: %v", c.Wake)
	}
	composePublish(t, home, board.Request{Kind: board.KindReply, From: board.Sender{Machine: "m1e"}, To: board.Address{Machine: "m1x"}, Thread: ask.ID, Text: "Yes."}, now)
	step("a reply")
	if _, err := store.Update("host-build", func(r *launch.Record) error { r.State = launch.Completed; return nil }); err != nil {
		t.Fatal(err)
	}
	if _, c = step("the build completed"); !slices.Contains(c.Wake, "hold|none") || slices.Contains(c.Wake, "row|proof-beside-builds|main") {
		t.Fatalf("the build ended: %v, want no hold and no row", c.Wake)
	}
	if err := store.Create(launch.Record{ID: "host-build-2", Kind: "build", State: launch.Running}); err != nil {
		t.Fatal(err)
	}
	held(now)
	if _, c = step("a held line"); !slices.Contains(c.Wake, "hold|main") || !slices.Contains(c.Wake, "row|proof-beside-builds|main") {
		t.Fatalf("a held line: %v, want the hold and its row", c.Wake)
	}
	if _, err := store.Update("read-2", func(r *launch.Record) error { r.State = launch.Completed; return nil }); err != nil {
		t.Fatal(err)
	}
	step("a launch completed")
	s, _ := read()
	for _, q := range s.Questions["here"] {
		if q.State == "open" {
			if err := channel.Close(root, q.ID, "answered", nil, channel.DestinationConfig{}); err != nil {
				t.Fatal(err)
			}
		}
	}
	step("a question answered")
	if err := ClearSeatIdleIncidentForIntent(root, "intent-wake", now); err != nil {
		t.Fatal(err)
	}
	step("an alert cleared")
}

func TestTheWakeExcludesPresentation(t *testing.T) {
	t.Parallel()
	t0 := situationNow
	base := func() Situation {
		return Situation{At: t0.Add(3 * time.Minute), Machine: "m1e", Load: hostload.Sample{At: t0.Format(time.RFC3339), Load1m: 3, Available: true},
			Processes:   []HostProcess{{Pid: 101, Kind: "go test", Checkout: "/seat", Started: t0}},
			Lane:        plain.Status{Health: &plain.Health{Drift: []plain.Fired{{Row: "proof-beside-builds", Subject: "main", Minutes: "3"}}}},
			Unavailable: map[string]string{"designs": "read at 09:03"}}
	}
	want := Compose(base())
	for name, change := range map[string]func(*Situation){
		"the read time": func(s *Situation) { s.At = t0.Add(5 * time.Minute) },
		"the load": func(s *Situation) {
			s.Load = hostload.Sample{At: t0.Add(time.Minute).Format(time.RFC3339), Load1m: 14, Available: true}
		},
		"a start":     func(s *Situation) { s.Processes[0].Started = t0.Add(-time.Hour) },
		"the minutes": func(s *Situation) { s.Lane.Health.Drift[0].Minutes = "45" },
		"a reason":    func(s *Situation) { s.Unavailable["designs"] = "read at 09:05" },
	} {
		s := base()
		change(&s)
		if got := Compose(s); got.Digest != want.Digest || !slices.Equal(got.Wake, want.Wake) {
			t.Errorf("%s woke: %v, want %v", name, got.Wake, want.Wake)
		}
	}
	if len(want.Wake) == 0 || !slices.Contains(want.Wake, "unavailable|designs") {
		t.Fatalf("the wake = %v, want the part that failed by name", want.Wake)
	}
	age := func(after time.Duration) []string {
		s := base()
		s.At = t0.Add(after)
		for _, part := range Compose(s).Parts {
			if part.Name == "host" {
				return part.Detail
			}
		}
		return nil
	}
	for after, word := range map[time.Duration]string{3 * time.Minute: "3m ago", 5 * time.Minute: "5m ago"} {
		if lines := age(after); !slices.ContainsFunc(lines, func(line string) bool { return strings.HasSuffix(line, "started "+word) }) {
			t.Errorf("at t0+%v the host's detail is %q, want a process started %s", after, lines, word)
		}
	}
}

func TestTheProgramPartSaysTheQuotaOnce(t *testing.T) {
	t.Parallel()
	program := func(root string, states map[string]string) []string {
		t.Helper()
		now := situationNow
		src := composeSources(t, root, t.TempDir(), &now)
		src.ClaimLaneState = claimQuotaLane(t, root, states)
		var lines []string
		for _, part := range Compose(readSituation(t, root, src)).Parts {
			if part.Name == "program" {
				for _, line := range part.Summary {
					if strings.Contains(line, "behind this seat's claim") {
						lines = append(lines, line)
					}
				}
			}
		}
		return lines
	}
	release := "metasystem goal release held-h --reason TEXT"
	root := situationQuotaBed(t)
	if lines, want := program(root, nil), "3 ready goals wait behind this seat's claim on held-h; next: "+release; !slices.Equal(lines, []string{want}) {
		t.Fatalf("three ready goals behind one claim: %q, want one line %q", lines, want)
	}
	one := claimQuotaTickBed(t, nil, claimQuotaHeld("held-h"), claimQuotaReady("ready-r", 1, "m1e"))
	if lines, want := program(one, nil), "1 ready goal waits behind this seat's claim on held-h; next: "+release; !slices.Equal(lines, []string{want}) {
		t.Fatalf("one ready goal: %q, want %q", lines, want)
	}
	if lines := program(root, map[string]string{"held-h": "waiting"}); len(lines) != 0 {
		t.Fatalf("a waiting hand-in frees the claim: %q, want no line", lines)
	}
}

func TestTheEnginePartKeepsTheExecutionsWithoutTheRegistry(t *testing.T) {
	t.Parallel()
	src := situationHostSources(t, "")
	src.Registry = func() (string, error) { return "", errors.New("no host home") }
	src.Executions = func(string) map[string]enginepin.Execution {
		return map[string]enginepin.Execution{"runner": {Generation: 7}}
	}
	for _, part := range Compose(readSituation(t, t.TempDir(), src)).Parts {
		if part.Name != "engine" {
			continue
		}
		if !slices.Contains(part.Summary, "runner generation 7") || !strings.HasPrefix(part.Unavailable, "seat holders: ") || !strings.Contains(part.Unavailable, "no host home") {
			t.Fatalf("the engine part = %+v, want the executions kept and only the seat holders unavailable", part)
		}
		return
	}
	t.Fatal("the page has no engine part")
}

func TestADeadlineMarksWhenAPauseRowFires(t *testing.T) {
	t.Parallel()
	laneHome := situationLaneHome(t)
	record, _, err := lane.Read(laneHome)
	if err != nil {
		t.Fatal(err)
	}
	src := withEmptyHost(t, SituationSources{Now: func() time.Time { return situationNow }, LaneHome: func() (string, error) { return laneHome, nil },
		HostWork: func() (int, int, error) { return 0, 0, nil }, BoardHome: func() (string, error) { return t.TempDir(), nil }})
	// The landing agent has run three minutes with no step: silent at 20, past healing at 80.
	since := situationNow.Add(-3 * time.Minute).Format(time.RFC3339)
	src.LaneStatus = composeLane(t, record.Install, src.HostWork, lane.OwnerView{State: lane.OwnerRunning, Since: &since})
	// A lane question open 12 minutes is overdue at 30.
	if _, err := channel.Ask(channel.AskRequest{RepoRoot: record.Install, About: "lane", Kind: "other", Machine: "m1e", Facts: []string{"Is the lane stopped?"}, Now: situationNow.Add(-12 * time.Minute)}); err != nil {
		t.Fatal(err)
	}
	if _, err := lane.SetPauseBecause(laneHome, "Wido", "a test pause", situationNow.Add(-20*time.Minute)); err != nil {
		t.Fatal(err)
	}
	s := readSituation(t, t.TempDir(), src)
	if s.Lane.Health == nil || s.Lane.Health.Paused == nil || slices.ContainsFunc(s.Lane.Health.Rows(), func(row plain.Fired) bool { return row.Row == "paused" }) {
		t.Fatalf("a pause 20 minutes in: health %+v, want it paused and its row not yet fired", s.Lane.Health)
	}
	for row, want := range map[string]time.Time{"paused": situationNow.Add(10*time.Minute + time.Second), "question-open": situationNow.Add(18*time.Minute + time.Second),
		"silent-agent": situationNow.Add(17*time.Minute + time.Second), "cannot-heal silent-agent": situationNow.Add(77*time.Minute + time.Second)} {
		if !slices.ContainsFunc(Compose(s).Deadlines, want.Equal) {
			t.Fatalf("deadlines = %v, want the %s row's firing at %v (health %+v)", Compose(s).Deadlines, row, want, s.Lane.Health)
		}
	}

	// A hand-in waiting 45 minutes turns overdue at 60 and cannot heal at 120.
	if _, _, err := plain.HandIn(record.Install, plain.Line{Goal: "g-late", SHA: strings.Repeat("ab", 20), At: situationNow.Add(-45 * time.Minute).Format(time.RFC3339)}); err != nil {
		t.Fatal(err)
	}
	s = readSituation(t, t.TempDir(), src)
	deadlines := Compose(s).Deadlines
	for _, want := range []time.Time{situationNow.Add(15*time.Minute + time.Second), situationNow.Add(75*time.Minute + time.Second)} {
		if slices.ContainsFunc(s.Lane.Health.Rows(), func(row plain.Fired) bool { return row.Row == "overdue-goal" }) || !slices.ContainsFunc(deadlines, want.Equal) {
			t.Fatalf("a hand-in 45 minutes waiting: goals %+v, deadlines %v, want %v and no overdue row yet", s.Lane.Health.Goals, deadlines, want)
		}
	}
}
