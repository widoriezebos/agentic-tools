package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/enginepin"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/hostload"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/registry"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
)

var situationNow = time.Date(2026, 10, 30, 9, 0, 0, 0, time.UTC)

// situationLaneBed registers a lane on a Git checkout the lane's status
// reads, with a proof-beside-builds hold written ten minutes ago.
func situationLaneBed(t *testing.T) string {
	t.Helper()
	home, checkout := t.TempDir(), t.TempDir()
	install := filepath.Join(checkout, "metasystem")
	if err := os.MkdirAll(install, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(install, "metasystem.conf"), []byte("metasystem.template=true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"init", "-q", "-b", "main"}, {"add", "-A"},
		{"-c", "user.name=fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "lane"}, {"update-ref", "refs/remotes/origin/main", "HEAD"}} {
		if out, err := exec.Command("git", append([]string{"-C", checkout}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	data, err := json.Marshal(lane.Record{Root: checkout, Install: install, CustodyEpoch: 1, RegisteredBy: "Wido"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(lane.HostDir(home), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(lane.RecordPath(home), data, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := plain.RecordIntervention(install, plain.Intervention{At: situationNow.Add(-10 * time.Minute).Format(time.RFC3339Nano),
		Row: "proof-beside-builds", Subject: "main", Outcome: "held: 1 builds and 0 unit checks run"}); err != nil {
		t.Fatal(err)
	}
	return home
}

// situationEmptyHost gives src an injected empty host: no processes, an
// absent registry and no engine roles.
func situationEmptyHost(t *testing.T, src steward.SituationSources) steward.SituationSources {
	t.Helper()
	absent := filepath.Join(t.TempDir(), "absent-registry.jsonl")
	src.Load = func(now time.Time) hostload.Sample { return hostload.Sample{Available: true} }
	src.Processes, src.Prober = identity.FixedProcessTable{}, situationDeadProber{}
	src.Argv = func(int64) ([]string, bool) { return nil, false }
	src.Cwd = func(int64) (string, bool) { return "", false }
	src.Registry = func() (string, error) { return absent, nil }
	src.Executions = func(string) map[string]enginepin.Execution { return map[string]enginepin.Execution{} }
	return src
}

// readTestSituation is steward.ReadSituation for a test. It fails the test
// when a host source is left nil, since the read would then fill it with the
// kernel's process table, argv, cwd and prober, or the host registry and
// engine pins.
func readTestSituation(t *testing.T, root string, src steward.SituationSources) steward.Situation {
	t.Helper()
	var nils []string
	for name, isNil := range map[string]bool{"Load": src.Load == nil, "Processes": src.Processes == nil, "Argv": src.Argv == nil,
		"Cwd": src.Cwd == nil, "Prober": src.Prober == nil, "Registry": src.Registry == nil, "Executions": src.Executions == nil} {
		if isNil {
			nils = append(nils, name)
		}
	}
	if len(nils) > 0 {
		slices.Sort(nils)
		t.Fatalf("a test read reaches the host's own readers through %v; inject an empty host", nils)
	}
	return steward.ReadSituation(root, src)
}

func TestTheSituationAndTheLaneSayOneHold(t *testing.T) {
	t.Parallel()
	home := situationLaneBed(t)
	parent := t.TempDir()
	store := launch.Store{Root: filepath.Join(parent, "launch")}
	if err := store.Create(launch.Record{ID: "host-build", Kind: "build", State: launch.Running}); err != nil {
		t.Fatal(err)
	}
	now := situationNow
	read := func() steward.Situation {
		t.Helper()
		src := situationSources(defaultIntentOwners(), situationEmptyHost(t, steward.SituationSources{Now: func() time.Time { return now },
			HostWork: (&launch.Manager{Store: store}).HostWork, LaneHome: func() (string, error) { return home, nil }}))
		s := readTestSituation(t, t.TempDir(), src)
		if s.Unavailable["lane"] != "" || s.Lane.Health == nil {
			t.Fatalf("the lane does not read: %q problems=%v", s.Unavailable["lane"], s.Lane.Problems)
		}
		if s.Unavailable["ledger"] == "" || s.Unavailable["quota"] == "" {
			t.Fatalf("a checkout with no ledger reads one: %v", s.Unavailable)
		}
		return s
	}
	row := func(s steward.Situation) bool {
		for _, fired := range s.Lane.Health.Drift {
			if fired.Row == "proof-beside-builds" {
				return true
			}
		}
		return false
	}

	s := read()
	if s.Hold == nil || s.Hold.Subject != "main" || !s.Hold.Until.Equal(situationNow.Add(20*time.Minute)) || !row(s) {
		t.Fatalf("while the build runs: hold=%+v row=%v", s.Hold, row(s))
	}
	if _, err := store.Update("host-build", func(r *launch.Record) error { r.State = launch.Completed; return nil }); err != nil {
		t.Fatal(err)
	}
	now = situationNow.Add(time.Minute)
	if s = read(); s.Hold != nil || row(s) || s.Unavailable["hold"] != "" {
		t.Fatalf("after the build completed: hold=%+v row=%v unavailable=%q", s.Hold, row(s), s.Unavailable["hold"])
	}
	if err := os.Rename(filepath.Join(parent, "launch"), filepath.Join(parent, "launch-gone")); err != nil {
		t.Fatal(err)
	}
	now = situationNow.Add(2 * time.Minute)
	if s = read(); s.Hold == nil || !strings.Contains(s.Unavailable["hold"], store.Root) {
		t.Fatalf("an unreadable store holds: hold=%+v unavailable=%q", s.Hold, s.Unavailable["hold"])
	}
}

// countedLedgerOwners are the work bed's owners with the endpoint reader and
// the command clock counted.
func countedLedgerOwners(bed *workBed, endpoints, clocks *int) intentOwners {
	owners := bed.workOwners()
	endpoint, commandNow := owners.dependencies.endpoint, owners.commandNow
	owners.dependencies.endpoint = func(root string) (goal.Endpoint, error) { *endpoints++; return endpoint(root) }
	owners.commandNow = func(root string) (time.Time, error) { *clocks++; return commandNow(root) }
	return owners
}

func TestTheSituationReadsAGivenProjection(t *testing.T) {
	t.Parallel()
	bed := newWorkBed(t)
	endpoints, clocks := 0, 0
	owners := countedLedgerOwners(bed, &endpoints, &clocks)
	read := &intentInvocation{cwd: bed.root(), owners: owners}
	if problem := read.selectRoot(); problem != nil {
		t.Fatal(problem.Summary)
	}
	projection, at, problem := read.projectionWithFetch(false)
	if problem != nil || projection.Tip == "" || endpoints != 1 || clocks != 1 {
		t.Fatalf("no given projection reads as today: tip=%q problem=%+v endpoints=%d clocks=%d", projection.Tip, problem, endpoints, clocks)
	}
	given := &intentInvocation{cwd: bed.root(), owners: owners, givenProjection: &projection, givenNow: at.Add(time.Hour)}
	if problem := given.selectRoot(); problem != nil {
		t.Fatal(problem.Summary)
	}
	for _, fetch := range []bool{false, true} {
		got, now, problem := given.projectionWithFetch(fetch)
		if problem != nil || got.Tip != projection.Tip || !now.Equal(at.Add(time.Hour)) || endpoints != 1 || clocks != 1 {
			t.Fatalf("a given projection (fetch %v): tip=%q now=%v problem=%+v endpoints=%d clocks=%d, want it and no read", fetch, got.Tip, now, problem, endpoints, clocks)
		}
	}
}

func TestTheSituationReadsTheLedgerOnce(t *testing.T) {
	t.Parallel()
	bed := newWorkBed(t)
	brief := bed.brief("once.md", "Working Mode: implement\n\nBuild the unit.\n\n| Unit | Lines |\n| --- | --- |\n| first | 40 |\n")
	if code, result, _ := bed.work(append([]string{"work", "build", bed.id, "first", "--brief", brief}, workCheck...)...); code != 0 {
		t.Fatalf("build: %+v", result)
	}
	// The steward's ledger asks Git whether the checkout has an accepted
	// ledger and names its machine; the projection itself is the bed's.
	root := bed.root()
	for _, args := range [][]string{{"init", "-q"}, {"config", "metasystem.goal.machine", "mac-cli"}, {"add", "plans/goals/backlog.md"},
		{"-c", "user.name=fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "ledger"}, {"update-ref", goal.AcceptedRef, "HEAD"}} {
		if out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	units, err := stewardUnitStages(bed.workOwners())(root, bed.id)
	if err != nil || len(units) != 1 {
		t.Fatalf("the seat's unit reader: units=%+v err=%v", units, err)
	}
	endpoints, clocks, projections := 0, 0, 0
	owners := countedLedgerOwners(bed, &endpoints, &clocks)
	ledger := bed.workOwners().dependencies.endpoint
	project := func(_ goal.Endpoint, fetch bool, now time.Time) (goal.Projection, error) {
		projections++
		endpoint, err := ledger(root)
		if err != nil {
			return goal.Projection{}, err
		}
		return goal.Project(endpoint, fetch, now)
	}
	src := situationSources(owners, situationEmptyHost(t, steward.SituationSources{Now: func() time.Time { return situationNow }, Project: project,
		HostWork: func() (int, int, error) { return 0, 0, nil }, LaneHome: func() (string, error) { return t.TempDir(), nil }}))
	s := readTestSituation(t, root, src)
	facts := s.GoalFacts[bed.id]
	if !strings.Contains(facts, strings.TrimSpace(units[0].Line)) || !strings.Contains(facts, units[0].Stage) {
		t.Fatalf("the claimed goal's facts lack its unit %+v:\n%s\nunavailable=%v", units[0], facts, s.Unavailable)
	}
	if projections != 1 || endpoints != 0 || clocks != 0 {
		t.Fatalf("the read projected %d times and read the endpoint %d and the clock %d times, want one projection and neither", projections, endpoints, clocks)
	}
}

// situationDeadProber finds every process dead.
type situationDeadProber struct{}

func (situationDeadProber) Probe(int64) (identity.Exact, identity.Liveness, error) {
	return identity.Exact{}, identity.Dead, nil
}

func TestTheSituationComposesHostAndRecords(t *testing.T) {
	t.Parallel()
	leased, free := t.TempDir(), t.TempDir()
	pid := int64(os.Getpid())
	started, ok := lease.StartedAt(pid, nil)
	if !ok {
		t.Fatal("this test's own start cannot be read")
	}
	if _, err := lease.Announce(leased, "situation-session", pid, started, "situation", "fake", "lin-situation"); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "armed-checkouts.jsonl")
	for i, checkout := range []string{leased, free} {
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
	given := steward.SituationSources{Now: func() time.Time { return situationNow }, HostWork: func() (int, int, error) { return 0, 0, nil },
		LaneHome:  func() (string, error) { return t.TempDir(), nil },
		Load:      func(now time.Time) hostload.Sample { return hostload.Sample{Available: true} },
		Processes: identity.FixedProcessTable{}, Argv: func(int64) ([]string, bool) { return nil, false }, Cwd: func(int64) (string, bool) { return "", false },
		Prober: situationDeadProber{}, Registry: func() (string, error) { return path, nil },
		Executions: func(string) map[string]enginepin.Execution {
			return map[string]enginepin.Execution{"runner": {Generation: 3}}
		}}
	s := readTestSituation(t, t.TempDir(), situationSources(defaultIntentOwners(), given))
	holders := map[string]steward.SeatHolder{}
	for _, holder := range s.Holders {
		holders[holder.Checkout] = holder
	}
	if holder := holders[leased]; holder.MainId == "" || holder.Lineage != "lin-situation" || holder.Pid != pid || holder.Unavailable != "" {
		t.Fatalf("the leased checkout's holder = %+v, want this test's main", holder)
	}
	if holder, ok := holders[free]; !ok || holder.MainId != "" || holder.Unavailable != "" || s.Unavailable["engine"] != "" || s.Executions["runner"].Generation != 3 {
		t.Fatalf("the free checkout's holder = %+v (listed %v), unavailable %q, want no holder and no error", holder, ok, s.Unavailable["engine"])
	}

	given.Holder = func(string) (steward.SeatHolder, error) { return steward.SeatHolder{MainId: "given"}, nil }
	if s = readTestSituation(t, t.TempDir(), situationSources(defaultIntentOwners(), given)); holdersMain(s.Holders) != "given given" {
		t.Fatalf("a given holder reader was replaced: holders %+v", s.Holders)
	}
}

func holdersMain(holders []steward.SeatHolder) string {
	var mains []string
	for _, holder := range holders {
		mains = append(mains, holder.MainId)
	}
	return strings.Join(mains, " ")
}
