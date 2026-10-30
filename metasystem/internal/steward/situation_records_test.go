package steward

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/enginepin"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/hostload"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/registry"
)

// situationRegistry is a host registry naming each checkout armed, written
// through the registry's own writer with the owner rows its tests write.
func situationRegistry(t *testing.T, checkouts ...string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "armed-checkouts.jsonl")
	for i, checkout := range checkouts {
		tag := fmt.Sprintf("tag-%d", i)
		payload, err := json.Marshal(map[string]any{"schemaVersion": 1, "event": registry.EventRelaunched, "checkoutPath": checkout, "ownerTag": tag,
			"at": "2026-09-29T08:00:00Z", "generation": 1, "watcherTag": tag + "-w", "reaperTag": tag + "-r", "retiredThrough": 0})
		if err != nil {
			t.Fatal(err)
		}
		if err := registry.AppendFrame(path, payload); err != nil {
			t.Fatal(err)
		}
	}
	return path
}

// situationProber answers each pid's start and counts its probes.
type situationProber struct {
	starts map[int64]time.Time
	calls  map[int64]int
}

func (p *situationProber) Probe(pid int64) (identity.Exact, identity.Liveness, error) {
	p.calls[pid]++
	return identity.Exact{Pid: pid, StartedAt: p.starts[pid]}, identity.Alive, nil
}

// deadProber finds every process dead.
type deadProber struct{}

func (deadProber) Probe(int64) (identity.Exact, identity.Liveness, error) {
	return identity.Exact{}, identity.Dead, nil
}

// situationHostSources are injected host reads: no kernel, no host registry.
func situationHostSources(t *testing.T, registryPath string) SituationSources {
	t.Helper()
	return SituationSources{Now: func() time.Time { return situationNow }, LaneHome: func() (string, error) { return t.TempDir(), nil },
		Load: func(now time.Time) hostload.Sample {
			return hostload.Sample{At: now.Format(time.RFC3339), Load1m: 3, Available: true}
		},
		Processes: identity.FixedProcessTable{}, Argv: func(int64) ([]string, bool) { return nil, false },
		Cwd: func(int64) (string, bool) { return "", false }, Prober: deadProber{},
		Registry:   func() (string, error) { return registryPath, nil },
		Executions: func(string) map[string]enginepin.Execution { return map[string]enginepin.Execution{} }}
}

// withEmptyHost gives src an injected empty host: no processes, an absent
// registry and no engine roles.
func withEmptyHost(t *testing.T, src SituationSources) SituationSources {
	t.Helper()
	host := situationHostSources(t, filepath.Join(t.TempDir(), "absent-registry.jsonl"))
	src.Load, src.Processes, src.Argv, src.Cwd = host.Load, host.Processes, host.Argv, host.Cwd
	src.Prober, src.Registry, src.Executions = host.Prober, host.Registry, host.Executions
	return src
}

// readSituation is ReadSituation for a test. It fails the test when a host
// source is left nil, since the read would then fill it with the kernel's
// process table, argv, cwd and prober, or the host registry and engine pins.
func readSituation(t *testing.T, root string, src SituationSources) Situation {
	t.Helper()
	if nils := kernelHostSources(src); len(nils) > 0 {
		t.Fatalf("a test read reaches the host's own readers through %v; inject an empty host", nils)
	}
	return ReadSituation(root, src)
}

// kernelHostSources names the host sources left nil.
func kernelHostSources(src SituationSources) (nils []string) {
	for name, isNil := range map[string]bool{"Load": src.Load == nil, "Processes": src.Processes == nil, "Argv": src.Argv == nil,
		"Cwd": src.Cwd == nil, "Prober": src.Prober == nil, "Registry": src.Registry == nil, "Executions": src.Executions == nil} {
		if isNil {
			nils = append(nils, name)
		}
	}
	slices.Sort(nils)
	return nils
}

func TestTheSituationReadsTheHostByCheckout(t *testing.T) {
	t.Parallel()
	top := t.TempDir()
	a, b := filepath.Join(top, "A"), filepath.Join(top, "A", "nested", "B")
	src := situationHostSources(t, situationRegistry(t, a, b))
	argv := map[int64][]string{101: {"go", "test", "./..."}, 102: {"/var/folders/x/go-build123/b001/plain.test", "-test.run=X"},
		103: {"vim"}, 104: {"steward.test"}, 106: {"/usr/local/go/bin/go", "test", "./internal/steward"}}
	cwds := map[int64]string{101: a, 102: filepath.Join(a, "artifacts", "agents", "workspaces", "goal-x", "build", "metasystem"),
		103: a, 104: "/elsewhere", 105: a, 106: filepath.Join(b, "internal")}
	cwdCalls := map[int64]int{}
	prober := &situationProber{starts: map[int64]time.Time{}, calls: map[int64]int{}}
	table := identity.FixedProcessTable{}
	for pid := int64(101); pid <= 106; pid++ {
		table = append(table, identity.ProcessRow{Pid: pid, Group: pid, Session: pid, Parent: 1})
	}
	src.Processes = table
	src.Argv = func(pid int64) ([]string, bool) { words, ok := argv[pid]; return words, ok }
	src.Cwd = func(pid int64) (string, bool) { cwdCalls[pid]++; return cwds[pid], true }
	src.Prober = prober
	read := func(load float64, start time.Time) map[int64]HostProcess {
		t.Helper()
		for pid := int64(101); pid <= 106; pid++ {
			prober.starts[pid] = start.Add(time.Duration(pid) * time.Second)
		}
		src.Load = func(now time.Time) hostload.Sample {
			return hostload.Sample{At: now.Format(time.RFC3339), Load1m: load, Available: true}
		}
		s := readSituation(t, t.TempDir(), src)
		if s.Load.Load1m != load || s.Unavailable["host"] != "" {
			t.Fatalf("the load = %+v, unavailable %q", s.Load, s.Unavailable["host"])
		}
		byPid := map[int64]HostProcess{}
		for _, process := range s.Processes {
			byPid[process.Pid] = process
		}
		if !slices.IsSortedFunc(s.Processes, func(x, y HostProcess) int {
			return cmp.Or(strings.Compare(x.Checkout, y.Checkout), cmp.Compare(x.Pid, y.Pid))
		}) {
			t.Fatalf("processes are not sorted by checkout, then pid: %+v", s.Processes)
		}
		for pid, process := range byPid {
			if !process.Started.Equal(prober.starts[pid]) {
				t.Fatalf("pid %d started %v, want the prober's %v", pid, process.Started, prober.starts[pid])
			}
		}
		return byPid
	}
	facts := func(processes map[int64]HostProcess) map[int64][3]string {
		out := map[int64][3]string{}
		for pid, process := range processes {
			out[pid] = [3]string{process.Kind, process.Checkout, process.Workspace}
		}
		return out
	}

	first := read(3, situationNow.Add(-time.Hour))
	want := map[int64][3]string{101: {"go test", a, ""}, 102: {"test binary", a, "goal-x/build"}, 104: {"test binary", "elsewhere", ""}, 106: {"go test", b, ""}}
	if got := facts(first); !maps.Equal(got, want) {
		t.Fatalf("processes = %v, want %v", got, want)
	}
	for _, pid := range []int64{103, 105} {
		if cwdCalls[pid] != 0 || prober.calls[pid] != 0 {
			t.Fatalf("pid %d is no test process but its cwd was read %d and its start %d times", pid, cwdCalls[pid], prober.calls[pid])
		}
	}
	if second := read(9, situationNow.Add(-2*time.Hour)); !maps.Equal(facts(second), want) {
		t.Fatalf("another load and other starts changed the facts: %v", facts(second))
	}
}

func TestTheSituationReadsEngineAndSeatHolders(t *testing.T) {
	t.Parallel()
	top := t.TempDir()
	a, b, c := filepath.Join(top, "A"), filepath.Join(top, "B"), filepath.Join(top, "C")
	src := situationHostSources(t, situationRegistry(t, a, b, c))
	executionReads := 0
	src.Executions = func(string) map[string]enginepin.Execution {
		executionReads++
		return map[string]enginepin.Execution{"runner": {Generation: 7, Digest: "sha256:runner"}, "supervisor": {Absent: true, Cause: "no record"}}
	}
	src.Holder = func(checkout string) (SeatHolder, error) {
		switch checkout {
		case a:
			return SeatHolder{MainId: "m-1", SessionId: "s-1", Lineage: "lin-a", Pid: 4242}, nil
		case b:
			return SeatHolder{}, nil
		}
		return SeatHolder{}, errors.New("the lease cannot be read")
	}
	s := readSituation(t, t.TempDir(), src)
	if s.Executions["runner"].Generation != 7 || !s.Executions["supervisor"].Absent || s.Unavailable["engine"] != "" {
		t.Fatalf("executions = %+v, unavailable %q", s.Executions, s.Unavailable["engine"])
	}
	holders := map[string]SeatHolder{}
	for _, holder := range s.Holders {
		holders[holder.Checkout] = holder
	}
	if holder := holders[a]; holder.MainId != "m-1" || holder.Lineage != "lin-a" || holder.Unavailable != "" {
		t.Fatalf("A's holder = %+v, want m-1 of lin-a", holder)
	}
	if holder, ok := holders[b]; !ok || holder.MainId != "" || holder.Unavailable != "" {
		t.Fatalf("B's holder = %+v (listed %v), want no holder and no error", holder, ok)
	}
	if holder := holders[c]; holder.Unavailable != "the lease cannot be read" || len(s.Holders) != 3 {
		t.Fatalf("C's holder = %+v of %d seats, want its error on its row", holder, len(s.Holders))
	}

	src.Holder = nil
	if s = readSituation(t, t.TempDir(), src); s.Unavailable["engine"] != "no reader" || s.Executions["runner"].Generation != 7 || executionReads != 2 {
		t.Fatalf("no holder reader: unavailable %q, executions %+v (%d reads)", s.Unavailable["engine"], s.Executions, executionReads)
	}
}

// situationRecordsBed is the claim-quota ledger made an installation, with
// one live and one expired grant, three design pages (one malformed), a
// review of the first, two questions (one closed), 25 attorney log lines, 12
// rulings and an open alert; and a lane whose installation holds one alert.
func situationRecordsBed(t *testing.T) (root, laneHome, laneInstall string) {
	t.Helper()
	root = situationQuotaBed(t)
	write := func(relative, text string) {
		t.Helper()
		path := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("metasystem.conf", "metasystem.template=true\n")
	grant := func(ulid, since, expires string) goal.PowerOfAttorneyEntry {
		return goal.PowerOfAttorneyEntry{ID: goal.Opid(ulid, "m1e", "lin-1"), By: "human:Wido", Tiers: []uint8{1}, Verbs: []string{"approve"}, Since: since, Expires: expires}
	}
	write("plans/goals/backlog.md", string(goal.RenderRoot(&goal.RootRecord{Identity: "01ARZ3NDEKTSV4RRFFQ69G5FAV", FormatVersion: "1", SyncMode: goal.SyncLocal, Revision: 2,
		PowerOfAttorney: []goal.PowerOfAttorneyEntry{grant("01J5X00000000000000000PB00", "2026-08-24T08:00:00Z", "2026-08-26"), grant("01J5X00000000000000000PB01", "2026-08-10T09:00:00Z", "2026-08-12")}})))
	seatGit(t, root, "add", "plans/goals/backlog.md")
	seatGit(t, root, "commit", "-qm", "grants")
	seatGit(t, root, "update-ref", goal.AcceptedRef, "HEAD")

	write("plans/designs/first.md", "# First design\n\n- Kind: design\n- Id: 01J5X00000000000000000DS01\n- Status: accepted\n- Goals: g1\n- Critique: Astra rounds 1-3 folded\n- Revision: 4\n\nThe page.\n")
	write("plans/designs/second.md", "# Second design\n\n- Kind: design\n- Id: 01J5X00000000000000000DS02\n- Status: proposed\n- Critique: none yet\n\nThe page.\n")
	// The head declares no Status: project.Read refuses it.
	write("plans/designs/bad.md", "# Bad design\n\n- Kind: design\n- Id: 01J5X00000000000000000DS03\n\nThe page.\n")
	canonical, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	job, err := json.Marshal(map[string]any{"jobId": "design-critic-first", "role": "design-critic", "status": "completed",
		"design": filepath.Join(canonical, "plans", "designs", "first.md"), "goalId": "g1", "round": 2})
	if err != nil {
		t.Fatal(err)
	}
	write("artifacts/agents/jobs/design-critic-first.json", string(job))

	for i, fact := range []string{"Is the lane stopped?", "Was the batch proven?"} {
		q, err := channel.Ask(channel.AskRequest{RepoRoot: root, About: "lane", Kind: "other", Machine: "m1e", Facts: []string{fact}, Now: situationNow.Add(time.Duration(i-10) * time.Minute)})
		if err != nil {
			t.Fatal(err)
		}
		if i == 1 {
			if err := channel.Close(root, q.ID, "answered", nil, channel.DestinationConfig{}); err != nil {
				t.Fatal(err)
			}
		}
	}
	for i := 1; i <= 25; i++ {
		if err := humanauthority.AppendAttorneyLog(root, fmt.Sprintf("answered %02d", i)); err != nil {
			t.Fatal(err)
		}
	}
	var register strings.Builder
	for i := 1; i <= 12; i++ {
		fmt.Fprintf(&register, "| R-%d | 2026-08-%02d | Ruling %d | given | Wido |  |\n", i, i, i)
	}
	write("memory/rulings.md", register.String())

	laneHome = situationLaneHome(t)
	record, _, err := lane.Read(laneHome)
	if err != nil {
		t.Fatal(err)
	}
	laneInstall = record.Install
	for _, store := range []string{root, laneInstall} {
		if _, err := RecordSeatIdleIncident(store, SeatIdleIncident{SessionID: "s-" + filepath.Base(store), BacklogDigest: "ledger-unreadable", Refusal: 3}, situationNow); err != nil {
			t.Fatal(err)
		}
	}
	return root, laneHome, laneInstall
}

// situationLaunches is a launch store with a build completed through the
// store's update, as the supervisor writes it, and a read still recorded
// running whose supervisor and child the manager's prober finds dead.
func situationLaunches(t *testing.T) *launch.Manager {
	t.Helper()
	store := launch.Store{Root: filepath.Join(t.TempDir(), "launch")}
	if err := store.Create(launch.Record{ID: "build-1", Kind: "build", Goal: "g1", State: launch.Running}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Update("build-1", func(r *launch.Record) error {
		r.State, r.Outputs = launch.Completed, []launch.Output{{Path: "report.md", Bytes: 10}, {Path: "return.json", Bytes: 2}}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.Create(launch.Record{ID: "read-1", Kind: "read", Goal: "g1", State: launch.Running,
		Supervisor: &identity.Ref{Pid: 10, StartedAtSec: 1}, Child: &identity.Ref{Pid: 20, StartedAtSec: 1}}); err != nil {
		t.Fatal(err)
	}
	return &launch.Manager{Store: store, Prober: deadProber{}}
}

func TestTheSituationReadsTheRecords(t *testing.T) {
	t.Parallel()
	root, laneHome, laneInstall := situationRecordsBed(t)
	src := situationHostSources(t, filepath.Join(t.TempDir(), "absent.jsonl"))
	src.LaneHome = func() (string, error) { return laneHome, nil }
	src.ClaimLaneState = claimQuotaLane(t, root, nil)
	src.Launches = situationLaunches(t).EffectiveList
	s := readSituation(t, root, src)

	launches := map[string]launch.Record{}
	for _, record := range s.Launches {
		launches[record.ID] = record
	}
	if build := launches["build-1"]; build.State != launch.Completed || len(build.Outputs) != 2 || build.Outputs[0].Path != "report.md" || build.Outputs[1].Path != "return.json" {
		t.Fatalf("the build = %+v, want completed with report.md and return.json", build)
	}
	if read := launches["read-1"]; read.State != launch.Failed || read.Reason != "supervisor-lost" {
		t.Fatalf("the read whose supervisor died = %+v, want failed", read)
	}

	designs := map[string]SituationDesign{}
	for _, design := range s.Designs {
		designs[filepath.Base(design.Record.Path)] = design
	}
	first, second := designs["first.md"], designs["second.md"]
	if first.Record.Status != "accepted" || first.Revision != "4" || first.Critique != "Astra rounds 1-3 folded" || len(first.Reviews) != 1 || first.Reviews[0].NewestRound != 2 {
		t.Fatalf("the first design = status %q revision %q critique %q reviews %+v", first.Record.Status, first.Revision, first.Critique, first.Reviews)
	}
	if second.Record.ID == "" || second.Revision != "" || second.Critique != "none yet" {
		t.Fatalf("the second design = %+v, want it with no revision", second)
	}
	if len(s.DesignProblems) == 0 || s.DesignProblems[0].Path != "plans/designs/bad.md" ||
		!strings.HasPrefix(s.Unavailable["designs"], "incomplete: ") || !strings.Contains(s.Unavailable["designs"], "plans/designs/bad.md") {
		t.Fatalf("the bad page: problems %+v, unavailable %q", s.DesignProblems, s.Unavailable["designs"])
	}
	for _, problem := range s.DesignProblems {
		if !strings.HasPrefix(problem.Path, "plans/designs/") {
			t.Fatalf("a problem outside the design homes: %+v", problem)
		}
	}

	states := map[string]string{}
	for _, q := range s.Questions["here"] {
		states[q.Facts[0]] = q.State
	}
	if len(states) != 2 || states["Is the lane stopped?"] != "open" || states["Was the batch proven?"] != "closed" || s.Unavailable["questions"] != "" {
		t.Fatalf("questions = %v, unavailable %q, want one open and one closed", states, s.Unavailable["questions"])
	}
	if len(s.Grants) != 1 || !strings.Contains(s.Grants[0].ID, "01J5X00000000000000000PB00") {
		t.Fatalf("grants = %+v, want the live one", s.Grants)
	}
	if len(s.Answered) != 20 || s.Answered[0] != "answered 06" || s.Answered[19] != "answered 25" {
		t.Fatalf("answered = %q, want the last 20, newest last", s.Answered)
	}
	if len(s.Rulings) != 10 || s.Rulings[0].ID != "R-3" || s.Rulings[9].ID != "R-12" || s.Unavailable["authority"] != "" {
		t.Fatalf("rulings = %+v, unavailable %q, want the last 10", s.Rulings, s.Unavailable["authority"])
	}
	if len(s.Alerts["here"]) != 1 || len(s.Alerts["lane"]) != 1 || s.LaneInstall != laneInstall || s.Unavailable["alerts"] != "" {
		t.Fatalf("alerts = %+v (lane %q), unavailable %q, want one here and one in the lane", s.Alerts, s.LaneInstall, s.Unavailable["alerts"])
	}

	third := t.TempDir()
	if s = readSituation(t, third, src); len(s.Alerts["here"]) != 0 || len(s.Alerts["lane"]) != 1 {
		t.Fatalf("a root with no alerts: %+v", s.Alerts)
	}
	if _, err := os.Stat(filepath.Dir(alertLockPath(third))); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the read made the alert lock folder in a root with no alerts: %v", err)
	}

	// Self-hosted, the historical home metasystem/plans holds the design and
	// review homes too: a bad design is one problem, and a bad review none.
	hosted := situationSelfHostedBed(t)
	if s = readSituation(t, hosted, src); len(s.DesignProblems) != 1 || s.DesignProblems[0].Path != "metasystem/plans/designs/bad.md" ||
		!strings.HasPrefix(s.Unavailable["designs"], "incomplete: 1 problem(s); ") {
		t.Fatalf("the self-hosted bad pages: problems %+v, unavailable %q, want the bad design once", s.DesignProblems, s.Unavailable["designs"])
	}
}

// situationSelfHostedBed is a template installation at metasystem/ under a
// Git checkout, with one malformed design and one malformed review.
func situationSelfHostedBed(t *testing.T) string {
	t.Helper()
	checkout, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	seatGit(t, checkout, "init", "-q", "-b", "main")
	for relative, text := range map[string]string{
		"metasystem/metasystem.conf":             "metasystem.template=true\n",
		"development/metasystem-design.md":       "# The metasystem's design\n",
		"metasystem/plans/designs/bad.md":        "# Bad design\n\n- Kind: design\n- Id: 01J5X00000000000000000DS04\n\nThe page.\n",
		"metasystem/plans/reviews/bad-review.md": "# Bad review\n\n- Kind: review\n- Id: 01J5X00000000000000000RV01\n\nThe page.\n",
	} {
		path := filepath.Join(checkout, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(checkout, "metasystem", "scripts", "agents"), 0o755); err != nil {
		t.Fatal(err)
	}
	return filepath.Join(checkout, "metasystem")
}

func TestTheSituationPartsFailAlone(t *testing.T) {
	t.Parallel()
	root, laneHome, _ := situationRecordsBed(t)
	src := situationHostSources(t, "")
	src.LaneHome = func() (string, error) { return laneHome, nil }
	src.Registry = func() (string, error) { return "", errors.New("no host home") }
	src.Launches = situationLaunches(t).EffectiveList
	broken := filepath.Join(root, "artifacts", "agents", "channel", "questions", "broken.json")
	if err := os.WriteFile(broken, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	src.Executions = func(string) map[string]enginepin.Execution {
		return map[string]enginepin.Execution{"runner": {Generation: 7}}
	}
	src.Holder = func(string) (SeatHolder, error) {
		t.Error("a holder was read without the registry")
		return SeatHolder{}, nil
	}
	s := readSituation(t, root, src)
	if !strings.Contains(s.Unavailable["host"], "no host home") || !strings.Contains(s.Unavailable["engine"], "no host home") ||
		!s.Load.Available || s.Executions["runner"].Generation != 7 || len(s.Processes) != 0 || len(s.Holders) != 0 {
		t.Fatalf("a failed registry read: unavailable %v, load %+v, executions %+v, %d processes, %d holders",
			s.Unavailable, s.Load, s.Executions, len(s.Processes), len(s.Holders))
	}
	if len(s.Launches) != 2 || len(s.Designs) < 2 {
		t.Fatalf("a failed registry read: %d launches, %d designs, want both still read", len(s.Launches), len(s.Designs))
	}
	if !strings.Contains(s.Unavailable["questions"], broken) || len(s.Questions["here"]) != 2 {
		t.Fatalf("an unreadable question: unavailable %q, %d questions, want it named and the others kept", s.Unavailable["questions"], len(s.Questions["here"]))
	}
	src.Launches = nil
	if s = readSituation(t, root, src); s.Unavailable["launches"] != "no reader" || len(s.Launches) != 0 || len(s.Designs) < 2 {
		t.Fatalf("no launch reader: unavailable %q, %d launches, %d designs", s.Unavailable["launches"], len(s.Launches), len(s.Designs))
	}

	seat := filepath.Join(t.TempDir(), "A")
	src.Registry = func() (string, error) { return situationRegistry(t, seat), nil }
	src.Processes = identity.ScriptedProcessTable{PidsErr: errors.New("the process table cannot be read")}
	src.Holder = func(string) (SeatHolder, error) { return SeatHolder{MainId: "m-1"}, nil }
	s = readSituation(t, t.TempDir(), src)
	if !strings.Contains(s.Unavailable["host"], "the process table cannot be read") || len(s.Holders) != 1 || s.Holders[0].MainId != "m-1" || s.Unavailable["engine"] != "" {
		t.Fatalf("a failed process table: unavailable %v, holders %+v, want the seats still read", s.Unavailable, s.Holders)
	}
}
