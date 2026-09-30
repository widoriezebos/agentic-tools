package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/realpath"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/registry"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/seat"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stopfence"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stoptransition"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/textui"
)

// machineBedNow is the bed's clock: the fleet's reader and the lane's.
var machineBedNow = time.Date(2026, 9, 30, 10, 0, 0, 0, time.UTC)

// machineBed is one computer with three checkouts and a fleet: this
// checkout (m1e, running its helpers and a job), a stopped one the registry
// still records (m1x), and the landing lane's checkout (no nickname, its
// landing owner running); the fleet also names m2a, which runs on another
// computer. Every process is a fixture item of a stop-transition family:
// nothing is signalled and no Git runs.
type machineBed struct {
	t                    *testing.T
	home, registry       string
	this, other, landing string
	class                string
	nicknames            map[string]string
	families             map[string][]stoptransition.Family
	launchDir            string
	dead                 map[int64]bool
	fleetCalls           int
	laneAlive            bool
	registryUnreadable   bool
	stopped              map[string]*int
	// extra are further checkouts a test adds; fleetErr fails the fleet
	// read, and fleetEdit changes the report it answers.
	extra     []string
	fleetErr  error
	fleetEdit func(*seat.Report)
	// worktrees are the further worktrees Git registers for a checkout;
	// every checkout lists itself first, as git worktree list does.
	worktrees map[string][]string
	// worktreesErr fails git worktree list for every checkout.
	worktreesErr error
}

// machineItem is one fixture process: live until its family stops it.
type machineItem struct {
	item stoptransition.Item
	live bool
}

// machineFamily is a family of fixture processes. Stopping one ends it,
// except in the untracked family, which only reports.
type machineFamily struct {
	name  string
	items []*machineItem
	stops *int
}

func (f *machineFamily) Name() string { return f.name }
func (f *machineFamily) Inventory() ([]stoptransition.Item, error) {
	var items []stoptransition.Item
	for _, current := range f.items {
		if current.live {
			items = append(items, current.item)
		}
	}
	return items, nil
}
func (f *machineFamily) Stop(item stoptransition.Item) (stoptransition.Outcome, error) {
	for _, current := range f.items {
		if current.item.Key == item.Key {
			if f.name == "untracked" {
				return stoptransition.Outcome{Line: strings.TrimSuffix(item.StatusLine, ": running") + ": not the metasystem's, not touched", Complete: true}, nil
			}
			current.live = false
			*f.stops++
		}
	}
	return stoptransition.Outcome{Line: strings.TrimSuffix(item.StatusLine, ": running") + ": stopped (TERM)", Complete: true, Survivor: item.Survivor}, nil
}

func machineFixtureItem(family, component, id string, pid int64, line string) *machineItem {
	key := family + ":" + component + ":" + id + fmt.Sprint(pid)
	return &machineItem{live: true, item: stoptransition.Item{Key: key, StatusLine: line, ObserveOnly: family == "untracked",
		Survivor: stopfence.Survivor{Component: component, ID: id, Pid: pid, PidStartedAt: 1790000000 + pid}}}
}

func newMachineBed(t *testing.T) *machineBed {
	t.Helper()
	base := t.TempDir()
	b := &machineBed{t: t, class: lease.ClassHuman, home: filepath.Join(base, "home"), launchDir: filepath.Join(base, "launches"),
		dead: map[int64]bool{}, laneAlive: true, stopped: map[string]*int{}, worktrees: map[string][]string{}}
	for _, dir := range []string{b.home, b.launchDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	b.home = realpath.Resolve(b.home)
	checkout := func(name string) string {
		dir := filepath.Join(base, name)
		for _, file := range []string{filepath.Join(dir, "metasystem.conf"), filepath.Join(dir, "bin", "metasystem")} {
			if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := testexec.WriteFile(file, []byte("fixture\n"), 0o755); err != nil {
				t.Fatal(err)
			}
		}
		return realpath.Resolve(dir)
	}
	b.this, b.other, b.landing = checkout("agentic-tools-m1e"), checkout("agentic-tools-m1x"), checkout("agentic-tools-landing")
	b.nicknames = map[string]string{b.this: "m1e", b.other: "m1x"}
	stops := func(checkout string) *int { count := 0; b.stopped[checkout] = &count; return &count }
	thisStops, landingStops := stops(b.this), stops(b.landing)
	b.families = map[string][]stoptransition.Family{
		b.this: {
			&machineFamily{name: "job", stops: thisStops, items: []*machineItem{machineFixtureItem("job", "job", "j-7", 104, "job j-7 running pid 104 implementer: running")}},
			&machineFamily{name: "steward", stops: thisStops, items: []*machineItem{machineFixtureItem("steward", "steward-runner", "", 101, "steward-runner pid 101 started 1790000101: running")}},
			&machineFamily{name: "supervision", stops: thisStops, items: []*machineItem{
				machineFixtureItem("supervision", "supervision-owner", "", 102, "supervision-owner pid 102 tag t generation 1: running"),
				machineFixtureItem("supervision", "watcher", "", 103, "watcher pid 103: running")}},
			&machineFamily{name: "untracked", stops: thisStops, items: []*machineItem{machineFixtureItem("untracked", "untracked", "", 900, "untracked pid 900 codex app-server: running")}},
		},
		b.other: {&machineFamily{name: "supervision", stops: stops(b.other)}},
		b.landing: {
			&machineFamily{name: "supervision", stops: landingStops, items: []*machineItem{machineFixtureItem("supervision", "landing-owner", "", 301, "landing-owner pid 301: running")}},
			&machineFamily{name: "untracked", stops: landingStops, items: []*machineItem{machineFixtureItem("untracked", "untracked", "", 900, "untracked pid 900 codex app-server: running")}},
		},
	}
	// The stopped checkout's last stop completed an hour ago.
	if err := stopfence.Write(b.other, stopfence.Record{State: stopfence.StateClosed, Phase: stopfence.PhaseStopped, Generation: 1,
		ChangedAt: "2026-09-30T09:00:00Z", Checkout: b.other, By: stopfence.Actor{Verb: "stop", Process: stopfence.Process{Pid: 7}}}); err != nil {
		t.Fatal(err)
	}
	// The host registry: this checkout armed, the other one stopped.
	b.registry = filepath.Join(b.home, ".metasystem", "armed-checkouts.jsonl")
	if err := os.MkdirAll(filepath.Dir(b.registry), 0o700); err != nil {
		t.Fatal(err)
	}
	for _, row := range []struct{ checkout, tag string }{{b.this, "tag-e"}, {b.other, "tag-x"}} {
		payload, _ := json.Marshal(map[string]any{"schemaVersion": 1, "event": registry.EventRelaunched, "checkoutPath": row.checkout, "ownerTag": row.tag,
			"at": "2026-09-30T08:00:00Z", "generation": 1, "watcherTag": row.tag + "-w", "reaperTag": row.tag + "-r", "retiredThrough": 0})
		if err := registry.AppendFrame(b.registry, payload); err != nil {
			t.Fatal(err)
		}
	}
	exited, _ := json.Marshal(map[string]any{"schemaVersion": 1, "event": registry.EventExited, "checkoutPath": b.other, "ownerTag": "tag-x",
		"at": "2026-09-30T09:00:00Z", "reason": "shutdown", "teardownComplete": true})
	if err := registry.AppendFrame(b.registry, exited); err != nil {
		t.Fatal(err)
	}
	if _, _, err := lane.Register(b.home, b.landing, "Wido", machineBedNow.Add(-24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	// One launch of this user works in this checkout.
	supervisor := identity.Ref{Pid: 555, StartedAtSec: 555}
	if err := (launch.Store{Root: b.launchDir}).Create(launch.Record{ID: "l-1", Kind: "build", Goal: "g-1", WorkingDirectory: filepath.Join(b.this, "work"),
		State: launch.Running, Supervisor: &supervisor, ProcessGroup: &supervisor, StartedAt: "2026-09-30T09:30:00Z"}); err != nil {
		t.Fatal(err)
	}
	return b
}

// top answers the repository top of each fixture checkout, as Git would.
func (b *machineBed) top(path string) (string, error) {
	for _, checkout := range append([]string{b.this, b.other, b.landing}, b.extra...) {
		if path == checkout || strings.HasPrefix(path, checkout+string(filepath.Separator)) {
			return checkout, nil
		}
	}
	return "", fmt.Errorf("fatal: not a git repository: %s", path)
}

// git answers git worktree list --porcelain for each fixture checkout: the
// checkout itself, then the worktrees the test registered; no Git runs.
func (b *machineBed) git(dir string, args ...string) ([]byte, error) {
	if strings.Join(args, " ") != "worktree list --porcelain" {
		return nil, fmt.Errorf("the machine bed runs no git %s", strings.Join(args, " "))
	}
	if b.worktreesErr != nil {
		return nil, b.worktreesErr
	}
	top, err := b.top(dir)
	if err != nil {
		return nil, err
	}
	blocks := []string{"worktree " + top + "\nHEAD 0123456789abcdef0123456789abcdef01234567\nbranch refs/heads/main\n"}
	for _, worktree := range b.worktrees[top] {
		blocks = append(blocks, "worktree "+worktree+"\nHEAD 0123456789abcdef0123456789abcdef01234567\nbranch refs/heads/goal/"+filepath.Base(worktree)+"\n")
	}
	return []byte(strings.Join(blocks, "\n")), nil
}

type machineProber struct{ dead map[int64]bool }

func (p machineProber) Probe(pid int64) (identity.Exact, identity.Liveness, error) {
	if p.dead[pid] {
		return identity.Exact{}, identity.Dead, nil
	}
	return identity.Exact{Pid: pid, StartedAt: time.Unix(pid, 0)}, identity.Alive, nil
}

type machineProcesses struct{ dead map[int64]bool }

func (machineProcesses) SelfRef() (identity.Ref, error) {
	return identity.Ref{Pid: 10, StartedAtSec: 10}, nil
}
func (machineProcesses) StartChild(launch.Command) (launch.Child, identity.Ref, error) {
	return nil, identity.Ref{}, errors.New("the fixture starts no child")
}
func (p machineProcesses) SignalGroup(pid int64, _ syscall.Signal) error {
	p.dead[pid] = true
	return nil
}
func (p machineProcesses) GroupAlive(pid int64) (bool, error) { return !p.dead[pid], nil }

func (b *machineBed) owners() intentOwners {
	self := identity.Ref{Pid: int64(os.Getpid()), StartedAtSec: 1}
	tickAt := machineBedNow.Add(-3 * time.Hour).Format(time.RFC3339)
	return intentOwners{
		resolver: stateroot.NewResolver(b.top, noExecutable),
		processes: processIntentOwners{
			process: processOwners{
				repositoryTop: b.top,
				classify: func(string, string, int64) (lease.Classification, error) {
					return lease.Classification{Class: b.class}, nil
				},
				transition: func(scope processScope, scale int) *stoptransition.Transition {
					return &stoptransition.Transition{Root: scope.Root, Checkout: scope.Checkout, ScaleMilli: scale, Families: b.families[scope.Checkout],
						Self: func() (identity.Ref, error) { return self, nil }}
				},
			},
			fleet: func(root string, fetch bool, now time.Time) (seat.Report, error) {
				b.fleetCalls++
				report := seat.Report{This: "m1e"}
				report.SetNow(machineBedNow)
				report.Machines = []seat.MachineStanding{
					{Machine: "m1e", Standing: seat.Reachable, This: true, Record: &seat.Record{Machine: "m1e", TickAt: machineBedNow.Format(time.RFC3339)}},
					{Machine: "m1x", Standing: seat.Reachable, Record: &seat.Record{Machine: "m1x", TickAt: machineBedNow.Add(-time.Hour).Format(time.RFC3339)}},
					{Machine: "m2a", Standing: seat.Reachable, Record: &seat.Record{Machine: "m2a", TickAt: tickAt}},
				}
				if b.fleetErr != nil {
					return seat.Report{}, b.fleetErr
				}
				if b.fleetEdit != nil {
					b.fleetEdit(&report)
				}
				return report, nil
			},
			launches: func() *launch.Manager {
				return &launch.Manager{Store: launch.Store{Root: b.launchDir}, Prober: machineProber{dead: b.dead}, Processes: machineProcesses{dead: b.dead},
					Now: func() time.Time { return machineBedNow }, Sleep: func(time.Duration) {}}
			},
			cancelDispatch: func(string, string) (map[string]any, int, error) {
				b.t.Error("machine stop reached the dispatch cancellation owner; system stop cancels dispatch jobs")
				return nil, 1, nil
			},
		},
		landing: laneVerbOwners{
			// The lane beds keep no goal ledger: validation is never due.
			validation: func(string, time.Time) (bool, error) { return false, nil },
			home:       func() (string, error) { return b.home, nil },
			probe: func(string) (lane.OwnerProbe, error) {
				if !b.laneAlive {
					return lane.OwnerProbe{}, nil
				}
				return lane.OwnerProbe{Alive: true, PID: 4242, Since: machineBedNow.Add(-2 * time.Hour)}, nil
			},
			records: func(string) ([]batch.Record, error) { return nil, nil },
			now:     func() time.Time { return machineBedNow },
		},
		work: intentWorkOwners{git: b.git},
		machines: machineOwners{
			registryPath: func() (string, error) {
				if b.registryUnreadable {
					return b.home, nil
				}
				return b.registry, nil
			},
			nickname: func(checkout string) (string, bool) {
				name, ok := b.nicknames[checkout]
				return name, ok
			},
		},
	}
}

func (b *machineBed) run(args ...string) (int, string, string) {
	b.t.Helper()
	command, rest, ok := resolveIntentArgv(args)
	if !ok {
		b.t.Fatalf("no public command %q", args)
	}
	var stdout, stderr bytes.Buffer
	code := runIntentIn(command, rest, &stdout, &stderr, b.this, b.owners())
	return code, stdout.String(), stderr.String()
}

func (b *machineBed) runJSON(args ...string) (int, intentResult, map[string]any) {
	b.t.Helper()
	code, stdout, stderr := b.run(append(args, "--json")...)
	var result intentResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		b.t.Fatalf("%v printed no JSON result: %v; stdout=%q stderr=%q", args, err, stdout, stderr)
	}
	var raw struct {
		Data map[string]any `json:"data"`
	}
	_ = json.Unmarshal([]byte(stdout), &raw)
	return code, result, raw.Data
}

func (b *machineBed) fence(checkout string) stopfence.Record {
	b.t.Helper()
	record, err := stopfence.Read(checkout)
	if err != nil {
		b.t.Fatal(err)
	}
	return record
}

// machineLocal is a helper's start as machine list --verbose tells it: the
// local time, as short as its distance from now allows (textui.Env.Time).
func machineLocal(epoch int64) string {
	return textui.Env{Now: time.Now(), Zone: time.Local}.Time(time.Unix(epoch, 0))
}

// TestMachineListSummarizesThisComputerFirst: the default is one summary
// line and the fleet's one line per machine, unchanged; --verbose adds each
// machine of this computer with its checkout, its helpers with pid and
// start in local time, its jobs, the landing lane's owner, the machines on
// other computers by their last report, and the processes that are not
// MetaSystem's, once each. Every reading is status's own.
func TestMachineListSummarizesThisComputerFirst(t *testing.T) {
	t.Parallel()
	b := newMachineBed(t)
	code, stdout, stderr := b.run("machine", "list")
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	if code != 0 || lines[0] != "3 machines on this computer · 2 running, 1 stopped · 2 jobs running · 1 elsewhere" {
		t.Fatalf("machine list = %d %q %q", code, stdout, stderr)
	}
	if len(lines) < 7 || !strings.HasPrefix(lines[4], "  ● m1e ") || !strings.HasPrefix(lines[5], "  ○ m1x ") || !strings.HasPrefix(lines[6], "  ● m2a ") {
		t.Fatalf("the fleet's rows do not follow the headline: %q", stdout)
	}
	if strings.Contains(stdout, "steward runner") || strings.Contains(stdout, "Not ours") {
		t.Fatalf("the default printed the verbose detail: %q", stdout)
	}

	code, verbose, stderr := b.run("machine", "list", "--verbose")
	if code != 0 {
		t.Fatalf("machine list --verbose = %d %q %q", code, verbose, stderr)
	}
	local := func(at time.Time) string { return textui.Env{Now: time.Now(), Zone: time.Local}.Time(at) }
	for _, want := range []string{
		"m1e " + b.this + " · this checkout · running · since " + machineLocal(1790000101),
		"pids steward runner 101 · supervision owner 102 · repo watcher 103",
		"job job j-7 running pid 104 implementer since " + machineLocal(1790000104),
		"launch j1:l-1: build launch, running, goal g-1, in " + filepath.Join(b.this, "work"),
		"m1x " + b.other + " · stopped",
		"stopped since " + local(time.Date(2026, 9, 30, 9, 0, 0, 0, time.UTC)),
		"agentic-tools-landing " + b.landing + " · landing lane · running · since " + machineLocal(1790000301),
		"pids landing batch owner 301",
		"lane owner running, pid 4242, since " + local(machineBedNow.Add(-2*time.Hour)),
		"On other computers m2a last reported " + local(machineBedNow.Add(-3*time.Hour)),
		"Not ours, left alone 900 codex app-server since " + machineLocal(1790000900),
	} {
		if !strings.Contains(oneSpaced(verbose), want) {
			t.Errorf("--verbose lacks %q:\n%s", want, verbose)
		}
	}
	if strings.Count(verbose, "app-server") != 1 {
		t.Errorf("a process that is not ours is listed more than once:\n%s", verbose)
	}

	code, result, data := b.runJSON("machine", "list")
	host, _ := data["thisComputer"].(map[string]any)
	machines, _ := host["machines"].([]any)
	if code != 0 || result.Outcome != intentConfirmed || len(machines) != 3 || data["machines"] == nil {
		t.Fatalf("machine list --json = %d %+v data %v", code, result, data)
	}
	var names []string
	for _, machine := range machines {
		entry := machine.(map[string]any)
		names = append(names, fmt.Sprint(entry["name"], ":", entry["state"]))
	}
	if !slices.Equal(names, []string{"m1e:running", "agentic-tools-landing:running", "m1x:stopped"}) {
		t.Fatalf("machines = %v", names)
	}
	if others, _ := data["otherComputers"].([]any); len(others) != 1 || others[0].(map[string]any)["machine"] != "m2a" {
		t.Fatalf("other computers = %v", data["otherComputers"])
	}
	for checkout, generation := range map[string]int64{b.this: 0, b.other: 1, b.landing: 0} {
		if b.fence(checkout).Generation != generation || *b.stopped[checkout] != 0 {
			t.Fatalf("machine list changed %s", checkout)
		}
	}
}

// TestMachineListFailsClosedOnAnUnreadableRegistry: a host registry that
// cannot be read is reported, never read as no machines; what the other
// sources name is still listed.
func TestMachineListFailsClosedOnAnUnreadableRegistry(t *testing.T) {
	t.Parallel()
	b := newMachineBed(t)
	b.registryUnreadable = true
	code, result, _ := b.runJSON("machine", "list")
	if code != 1 || result.Outcome != intentPartial || !strings.Contains(result.Summary, "at least 2 machines on this computer") ||
		!strings.Contains(result.Summary, "the host registry "+b.home+" cannot be read") {
		t.Fatalf("an unreadable registry = %d %+v", code, result)
	}
	// Without the registry a stopped checkout of this computer cannot be
	// told from a machine on another computer: neither is claimed.
	if strings.Contains(result.Summary, "other computer") || !strings.Contains(result.Summary, "2 not found on this computer") {
		t.Fatalf("an unreadable registry placed machines on other computers: %+v", result)
	}
	code, result, _ = b.runJSON("machine", "stop", "m1x")
	if code != 1 || result.Outcome != intentFailed || strings.Contains(result.Summary, codeMachineOnAnotherComputer) || !strings.Contains(result.Summary, "cannot be read") {
		t.Fatalf("machine stop m1x with an unreadable registry = %d %+v", code, result)
	}
}

// TestMachineStopAllStopsEveryCheckoutThroughSystemStop: each machine of
// this computer, the lane's checkout included, stops through system stop
// itself (its proof, its fence, its job step); this user's launch in a
// stopped checkout is cancelled as work stop cancels it; what is not ours is
// not touched; a repeat is success that writes nothing.
func TestMachineStopAllStopsEveryCheckoutThroughSystemStop(t *testing.T) {
	t.Parallel()
	b := newMachineBed(t)
	code, result, data := b.runJSON("machine", "stop", "--all")
	if code != 0 || result.Outcome != intentConfirmed ||
		result.Summary != "stopped MetaSystem on 3 machines of this computer (1 already stopped); 1 launch cancelled" {
		t.Fatalf("machine stop --all = %d %+v %v", code, result, data)
	}
	for _, checkout := range []string{b.this, b.landing} {
		if fence := b.fence(checkout); !stopfence.Completed(fence) {
			t.Fatalf("%s was not stopped: %+v", checkout, fence)
		}
	}
	if b.fence(b.other).Generation != 1 || *b.stopped[b.this] != 4 || *b.stopped[b.landing] != 1 {
		t.Fatalf("stops: other generation %d, this %d, landing %d", b.fence(b.other).Generation, *b.stopped[b.this], *b.stopped[b.landing])
	}
	if record, err := (launch.Store{Root: b.launchDir}).Read("l-1"); err != nil || record.State != launch.Cancelled {
		t.Fatalf("the launch in the stopped checkout = %+v %v", record, err)
	}
	for _, family := range b.families[b.this] {
		for _, item := range family.(*machineFamily).items {
			if item.live != (family.Name() == "untracked") {
				t.Fatalf("%s %s live=%v after the stop", family.Name(), item.item.Key, item.live)
			}
		}
	}

}

// witnessMachineStopRepeat: a second machine stop --all, with every machine
// already stopped and only processes that are not MetaSystem's running, is
// success that writes nothing and names nothing it stopped.
func witnessMachineStopRepeat(t *testing.T) {
	b := newMachineBed(t)
	if code, result, _ := b.runJSON("machine", "stop", "--all"); code != 0 || result.Outcome != intentConfirmed {
		t.Fatalf("first machine stop --all = %d %+v", code, result)
	}
	before := idemTreeDigest(t, filepath.Dir(b.this))
	code, result, _ := b.runJSON("machine", "stop", "--all")
	if code != 0 || result.Outcome != intentUnchanged || result.Summary != "MetaSystem is already stopped on all 3 machines of this computer; nothing of MetaSystem's is running" {
		t.Fatalf("repeated machine stop --all = %d %+v", code, result)
	}
	idemSameTree(t, "a repeated machine stop --all", before, idemTreeDigest(t, filepath.Dir(b.this)))
}

// TestMachineStopOneMachine stops exactly the named machine, by nickname,
// and leaves every other checkout as it was.
func TestMachineStopOneMachine(t *testing.T) {
	t.Parallel()
	b := newMachineBed(t)
	code, result, _ := b.runJSON("machine", "stop", "agentic-tools-landing")
	if code != 0 || result.Outcome != intentConfirmed || result.Summary != "stopped MetaSystem on agentic-tools-landing ("+b.landing+")" {
		t.Fatalf("machine stop agentic-tools-landing = %d %+v", code, result)
	}
	if !stopfence.Completed(b.fence(b.landing)) || b.fence(b.this).State != stopfence.StateOpen {
		t.Fatalf("stop reached the wrong checkout: landing %+v this %+v", b.fence(b.landing), b.fence(b.this))
	}
	if record, _ := (launch.Store{Root: b.launchDir}).Read("l-1"); record.State != launch.Running {
		t.Fatalf("a launch in another checkout was cancelled: %+v", record)
	}
	code, result, _ = b.runJSON("machine", "stop", "m1x")
	if code != 0 || result.Outcome != intentUnchanged || !strings.Contains(result.Summary, "MetaSystem is already stopped on m1x") {
		t.Fatalf("stop of a stopped machine = %d %+v", code, result)
	}
}

// TestMachineStopRefusals: a machine on another computer is stopped on that
// computer; a name nothing knows, or both a name and --all, changes nothing;
// an agent is refused by system stop's own proof before any machine stops.
func TestMachineStopRefusals(t *testing.T) {
	t.Parallel()
	b := newMachineBed(t)
	code, result, _ := b.runJSON("machine", "stop", "m2a")
	if code != 1 || result.Outcome != intentRefused || !strings.Contains(result.Summary, "m2a runs on another computer") ||
		!strings.Contains(strings.Join(result.Details, "\n"), codeMachineOnAnotherComputer) ||
		result.Next == nil || strings.Join(result.Next.Argv, " ") != "metasystem system stop --repo PATH" {
		t.Fatalf("machine stop m2a = %d %+v", code, result)
	}
	for _, args := range [][]string{{"machine", "stop", "m9z"}, {"machine", "stop"}, {"machine", "stop", "m1e", "--all"}} {
		if code, result, _ := b.runJSON(args...); code != 2 || result.Outcome != intentRefused || !strings.Contains(result.Summary, "nothing was done") {
			t.Fatalf("%v = %d %+v", args, code, result)
		}
	}
	b.class = lease.ClassDelegate
	code, result, _ = b.runJSON("machine", "stop", "--all")
	if code != 1 || result.Outcome != intentRefused || !strings.Contains(result.Summary, "an agent started this shell") {
		t.Fatalf("an agent's machine stop --all = %d %+v", code, result)
	}
	for _, checkout := range []string{b.this, b.landing} {
		if b.fence(checkout).State != stopfence.StateOpen {
			t.Fatalf("a refused stop changed %s", checkout)
		}
	}
	if record, _ := (launch.Store{Root: b.launchDir}).Read("l-1"); record.State != launch.Running {
		t.Fatalf("a refused stop cancelled a launch: %+v", record)
	}
}

// register adds one checkout to the bed's host registry, armed or stopped.
func (b *machineBed) register(checkout, tag string, armed bool) {
	b.t.Helper()
	payload, _ := json.Marshal(map[string]any{"schemaVersion": 1, "event": registry.EventRelaunched, "checkoutPath": checkout, "ownerTag": tag,
		"at": "2026-09-30T08:00:00Z", "generation": 1, "watcherTag": tag + "-w", "reaperTag": tag + "-r", "retiredThrough": 0})
	if err := registry.AppendFrame(b.registry, payload); err != nil {
		b.t.Fatal(err)
	}
	if armed {
		return
	}
	exited, _ := json.Marshal(map[string]any{"schemaVersion": 1, "event": registry.EventExited, "checkoutPath": checkout, "ownerTag": tag,
		"at": "2026-09-30T09:00:00Z", "reason": "shutdown", "teardownComplete": true})
	if err := registry.AppendFrame(b.registry, exited); err != nil {
		b.t.Fatal(err)
	}
}

// TestMachineListCountsOnlyTheFleetsMachines: a machine is a checkout with
// a nickname that is armed, in the fleet's presence or the landing lane's
// record. The host registry's other registrations (test beds, scratch and
// builder clones, gone fixtures: fifty stopped and nameless, one armed and
// nameless, one nicknamed but neither armed nor in the fleet) are not
// machines: machine list does not count them, --verbose names only their
// number, and machine stop --all never acts on them. A real machine that
// cannot be read stays listed, unknown.
func TestMachineListCountsOnlyTheFleetsMachines(t *testing.T) {
	t.Parallel()
	b := newMachineBed(t)
	scratch := t.TempDir()
	for index := range 50 {
		b.register(filepath.Join(scratch, fmt.Sprintf("bed-%02d", index)), fmt.Sprintf("tag-bed-%02d", index), false)
	}
	b.register(filepath.Join(scratch, "armed-nameless"), "tag-armed-nameless", true)
	stray := filepath.Join(scratch, "nicknamed-stray")
	b.register(stray, "tag-stray", false)
	b.nicknames[stray] = "m9q"

	code, stdout, stderr := b.run("machine", "list")
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	if code != 0 || lines[0] != "3 machines on this computer · 2 running, 1 stopped · 2 jobs running · 1 elsewhere" {
		t.Fatalf("machine list with 52 other registrations = %d %q %q", code, stdout, stderr)
	}
	if strings.Contains(stdout, "other registered") || strings.Contains(stdout, "bed-") {
		t.Fatalf("the default output names the registrations that are not machines: %q", stdout)
	}
	code, verbose, _ := b.run("machine", "list", "--verbose")
	if code != 0 || !strings.Contains(verbose, "52 other registered checkouts are not machines\n") ||
		!strings.Contains(verbose, "→ metasystem disk clean  forgets those whose directories are gone\n") ||
		strings.Contains(verbose, "bed-") || strings.Contains(verbose, "nicknamed-stray") || strings.Contains(verbose, "armed-nameless") {
		t.Fatalf("machine list --verbose = %d:\n%s", code, verbose)
	}

	code, result, _ := b.runJSON("machine", "stop", "--all")
	if code != 0 || result.Outcome != intentConfirmed ||
		result.Summary != "stopped MetaSystem on 3 machines of this computer (1 already stopped); 1 launch cancelled" || len(result.Targets) != 3 {
		t.Fatalf("machine stop --all with 52 other registrations = %d %+v", code, result)
	}

	// A nicknamed armed checkout that cannot be read is a machine, unknown.
	broken := filepath.Join(scratch, "armed-unreadable")
	b.register(broken, "tag-broken", true)
	b.nicknames[broken] = "m1z"
	code, result, _ = b.runJSON("machine", "list")
	if code != 0 || !strings.HasPrefix(result.Summary, "4 machines on this computer: ") || !strings.Contains(result.Summary, ", 1 unknown;") {
		t.Fatalf("an unreadable armed machine = %d %+v", code, result)
	}
}
