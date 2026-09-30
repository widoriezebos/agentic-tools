package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/realpath"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/registry"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/seat"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/lifecycle"
)

// uiSeatsExact is the fixture identity of pid, as lifecycle's own beds
// write it.
func uiSeatsExact(pid int64) identity.Exact {
	exact := identity.Exact{Pid: pid, StartedAt: time.Unix(1_700_000_000, 123_456_000)}
	if runtime.GOOS == "linux" {
		exact.StartTicks = 987654
		exact.BootID = "test-boot-id"
	}
	return exact
}

// uiSeatsProber answers each pid it knows with its state, every other pid
// dead.
type uiSeatsProber map[int64]identity.Liveness

func (p uiSeatsProber) Probe(pid int64) (identity.Exact, identity.Liveness, error) {
	state, ok := p[pid]
	if !ok {
		return identity.Exact{}, identity.Dead, nil
	}
	return uiSeatsExact(pid), state, nil
}

// uiSeatsSignal is one signal the spy received.
type uiSeatsSignal struct {
	pid    int
	signal syscall.Signal
}

// uiSeatsBed is seats as temp state roots, a prober, a signal spy and a
// spawn counter; nothing is signalled or launched.
type uiSeatsBed struct {
	t      *testing.T
	prober uiSeatsProber
	sent   []uiSeatsSignal
	spawns int
	after  func(time.Duration) <-chan time.Time
	// specs are the launches spawned, engines the installations whose
	// engine was asked for; listen and listenSet are the --listen typed.
	specs     []lifecycle.LaunchSpec
	engines   []string
	listen    string
	listenSet bool
	// ready, when set, is the address the fake child announces instead of
	// the one it was asked to listen at.
	ready string
}

func newUISeatsBed(t *testing.T) *uiSeatsBed {
	return &uiSeatsBed{t: t, prober: uiSeatsProber{}, after: uiSeatsNeverAfter}
}

// uiSeatsNeverAfter is a lock wait that never times out; uiSeatsFiredAfter
// one that already has.
func uiSeatsNeverAfter(time.Duration) <-chan time.Time { return make(chan time.Time) }
func uiSeatsFiredAfter(time.Duration) <-chan time.Time {
	fired := make(chan time.Time, 1)
	fired <- time.Time{}
	return fired
}

func (b *uiSeatsBed) seat(name string) uiSeat {
	root := realpath.Resolve(b.t.TempDir())
	// Every seat's installation carries its settings, empty by default.
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), nil, 0o644); err != nil {
		b.t.Fatal(err)
	}
	return uiSeat{Name: name, Checkout: root, Roots: lifecycle.Roots{Checkout: root, Installation: root, StateRoot: root}}
}

// live writes the seat's record of a server at pid and address, and its
// unheld lock, as the server writes them; the prober answers pid state.
func (b *uiSeatsBed) live(s uiSeat, pid int64, address string, state identity.Liveness) {
	b.t.Helper()
	uiSeatsWriteRecord(b.t, s.Roots.StateRoot, pid, address)
	b.prober[pid] = state
}

// engine installs an engine file under the seat's installation, and conf
// its metasystem.conf.
func (b *uiSeatsBed) engine(s uiSeat, content string) string {
	b.t.Helper()
	path := filepath.Join(s.Roots.Installation, "bin", "metasystem")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		b.t.Fatal(err)
	}
	if err := testexec.WriteFile(path, []byte(content), 0o755); err != nil {
		b.t.Fatal(err)
	}
	return path
}

func (b *uiSeatsBed) conf(s uiSeat, body string) {
	b.t.Helper()
	if err := os.WriteFile(filepath.Join(s.Roots.Installation, "metasystem.conf"), []byte(body), 0o644); err != nil {
		b.t.Fatal(err)
	}
}

func uiSeatsWriteRecord(t *testing.T, stateRoot string, pid int64, address string) {
	t.Helper()
	process, err := identity.EncodeRef(uiSeatsExact(pid).Ref())
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(lifecycle.Record{SchemaVersion: 1, Process: process, Address: address, Checkout: stateRoot, Installation: stateRoot,
		StartedAt: "2026-09-30T10:00:00Z", EngineBuild: "fixture"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(lifecycle.Dir(stateRoot), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(lifecycle.Dir(stateRoot), "server.json"), append(encoded, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	lock, err := os.OpenFile(filepath.Join(lifecycle.Dir(stateRoot), "server.flock"), os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if err := lock.Close(); err != nil {
		t.Fatal(err)
	}
}

func uiSeatsHasRecord(stateRoot string) bool {
	_, err := os.Stat(filepath.Join(lifecycle.Dir(stateRoot), "server.json"))
	return err == nil
}

// uiSeatsHoldLock holds the seat's lock as a running server does, until
// the returned release.
func uiSeatsHoldLock(t *testing.T, stateRoot string) func() {
	t.Helper()
	lock, err := os.OpenFile(filepath.Join(lifecycle.Dir(stateRoot), "server.flock"), os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		t.Fatal(err)
	}
	released := false
	release := func() {
		if !released {
			released = true
			_ = syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
			_ = lock.Close()
		}
	}
	t.Cleanup(release)
	return release
}

func (b *uiSeatsBed) effects(inventory func() uiSeatInventory) uiLifecycleEffects {
	return uiLifecycleEffects{
		prober: b.prober,
		engine: func(installation string) (string, error) {
			b.engines = append(b.engines, installation)
			return uiInstallationEngine(installation)
		},
		listenFor: func(target lifecycle.Roots) (string, error) {
			return uiListen("restart", target, b.listen, b.listenSet)
		},
		spawn: func(spec lifecycle.LaunchSpec) (lifecycle.Child, error) {
			b.spawns++
			b.specs = append(b.specs, spec)
			if b.ready != "" {
				return idemUIChild{address: b.ready}, nil
			}
			return idemUIChild{address: spec.Args[slices.Index(spec.Args, "--listen")+1]}, nil
		},
		send: func(pid int, signal syscall.Signal) error {
			b.sent = append(b.sent, uiSeatsSignal{pid, signal})
			return nil
		},
		after: func(wait time.Duration) <-chan time.Time { return b.after(wait) },
		seats: inventory,
	}
}

func uiSeatsOf(this string, seats ...uiSeat) func() uiSeatInventory {
	return func() uiSeatInventory { return uiSeatInventory{This: this, Seats: seats} }
}

func uiSeatsText(result uiLifecycleResult) string { return strings.Join(result.Result.Lines, "\n") }

// TestUIStopStopsTheOneOtherSeatsInterface (D-stop, one): with no
// interface on this seat and exactly one other machine running one, stop
// stops that one through its own record and says which.
func TestUIStopStopsTheOneOtherSeatsInterface(t *testing.T) {
	t.Parallel()
	b := newUISeatsBed(t)
	a, other := b.seat("m1e"), b.seat("ui")
	b.live(other, 5201, "127.0.0.1:7878", identity.Alive)
	before := idemTreeDigest(t, a.Roots.StateRoot)
	got := uiLifecycleRunWith("stop", a.Roots, "", 0, b.effects(uiSeatsOf("m1e", other)))
	if !slices.Equal(b.sent, []uiSeatsSignal{{5201, syscall.SIGTERM}}) {
		t.Fatalf("signals = %v, want one SIGTERM to 5201; result %+v", b.sent, got)
	}
	if uiSeatsHasRecord(other.Roots.StateRoot) {
		t.Fatalf("the stopped seat's record remains")
	}
	if got.Result.Code != 0 || got.Unchanged || got.Seat == nil || got.Seat.Name != "ui" || got.Seat.Checkout != other.Checkout ||
		!slices.Equal(got.Result.Lines, []string{"no interface for m1e; stopped the interface of machine ui (pid 5201, :7878)"}) {
		t.Fatalf("stop from another seat = %+v", got)
	}
	idemSameTree(t, "this seat's tree", before, idemTreeDigest(t, a.Roots.StateRoot))
}

// TestUIStopWithSeveralOtherSeatsStopsNoneAndListsEach (D-stop, several).
func TestUIStopWithSeveralOtherSeatsStopsNoneAndListsEach(t *testing.T) {
	t.Parallel()
	b := newUISeatsBed(t)
	a, first, second := b.seat("m1e"), b.seat("ui"), b.seat("landing")
	b.live(first, 5201, "127.0.0.1:7878", identity.Alive)
	b.live(second, 5202, "127.0.0.1:7879", identity.Alive)
	got := uiLifecycleRunWith("stop", a.Roots, "", 0, b.effects(uiSeatsOf("m1e", first, second)))
	text := uiSeatsText(got)
	if len(b.sent) != 0 || !uiSeatsHasRecord(first.Roots.StateRoot) || !uiSeatsHasRecord(second.Roots.StateRoot) {
		t.Fatalf("several running seats: signals %v, result %+v", b.sent, got)
	}
	if got.Result.Code != 1 || got.Unchanged || got.Seat != nil || len(got.Seats) != 2 ||
		got.Result.Lines[0] != "no interface for m1e; 2 machines of this computer run one; nothing was done" ||
		!strings.Contains(text, "machine ui: pid 5201 at 127.0.0.1:7878: metasystem ui stop --repo "+first.Checkout) ||
		!strings.Contains(text, "machine landing: pid 5202 at 127.0.0.1:7879: metasystem ui stop --repo "+second.Checkout) {
		t.Fatalf("several running seats = %+v", got)
	}
}

// TestUIStopOnAnotherSeatKeepsTheIdentityProof (D-proof): another seat's
// record is judged by the same exact-identity proof as this seat's.
func TestUIStopOnAnotherSeatKeepsTheIdentityProof(t *testing.T) {
	t.Parallel()
	b := newUISeatsBed(t)
	a, other := b.seat("m1e"), b.seat("ui")
	b.live(other, 5203, "127.0.0.1:7878", identity.Unknown)
	got := uiLifecycleRunWith("stop", a.Roots, "", 0, b.effects(uiSeatsOf("m1e", other)))
	if len(b.sent) != 0 || !uiSeatsHasRecord(other.Roots.StateRoot) || got.Result.Code != 1 || got.Unchanged ||
		!slices.Equal(got.Result.Lines, []string{"no interface for m1e; machine ui: cannot prove pid 5203 is the interface server; nothing was changed"}) {
		t.Fatalf("an uninspectable seat = signals %v, %+v", b.sent, got)
	}

	b.prober[5203] = identity.Dead
	got = uiLifecycleRunWith("stop", a.Roots, "", 0, b.effects(uiSeatsOf("m1e", other)))
	if len(b.sent) != 0 || uiSeatsHasRecord(other.Roots.StateRoot) || got.Result.Code != 0 || !got.Unchanged ||
		!slices.Equal(got.Result.Lines, []string{"interface not running"}) {
		t.Fatalf("a dead seat's record = signals %v, %+v", b.sent, got)
	}
}

// TestUIStatusPointsAtTheSeatThatRunsOne (D-status).
func TestUIStatusPointsAtTheSeatThatRunsOne(t *testing.T) {
	t.Parallel()
	b := newUISeatsBed(t)
	a, other := b.seat("m1e"), b.seat("ui")
	b.live(other, 5204, "127.0.0.1:7878", identity.Alive)
	pointer := "machine ui runs one (pid 5204 at 127.0.0.1:7878): metasystem ui status --repo " + other.Checkout
	got := uiLifecycleRunWith("status", a.Roots, "", 0, b.effects(uiSeatsOf("m1e", other)))
	if got.State != lifecycle.Stopped || got.Result.Code != 1 || !slices.Contains(got.Result.Lines, pointer) || len(got.Seats) != 1 {
		t.Fatalf("status of a stopped seat = %+v", got)
	}

	problem := "the fleet's presence cannot be read: boom"
	got = uiLifecycleRunWith("status", a.Roots, "", 0, b.effects(func() uiSeatInventory {
		return uiSeatInventory{This: "m1e", Seats: []uiSeat{other}, Problems: []string{problem}}
	}))
	if got.State != lifecycle.Stopped || got.Result.Code != 1 || !slices.Contains(got.Result.Lines, "the machines of this computer could not all be read: "+problem) ||
		!slices.Equal(got.SeatsProblems, []string{problem}) {
		t.Fatalf("status with an inventory problem = %+v", got)
	}

	b.live(a, 5205, "127.0.0.1:7879", identity.Alive)
	asked := 0
	got = uiLifecycleRunWith("status", a.Roots, "", 0, b.effects(func() uiSeatInventory { asked++; return uiSeatsOf("m1e", other)() }))
	if got.State != lifecycle.Running || strings.Contains(uiSeatsText(got), "machine ui") || asked != 0 || len(got.Seats) != 0 {
		t.Fatalf("status of a running seat = %+v, inventory read %d times", got, asked)
	}
}

// TestUIStartRefusesTheAddressAnotherSeatHolds (D-start).
func TestUIStartRefusesTheAddressAnotherSeatHolds(t *testing.T) {
	t.Parallel()
	b := newUISeatsBed(t)
	a, other := b.seat("m1e"), b.seat("ui")
	b.live(other, 5206, "127.0.0.1:8765", identity.Alive)
	b.engine(a, "engine of m1e")
	got := uiLifecycleRunWith("start", a.Roots, "127.0.0.1:8765", 0, b.effects(uiSeatsOf("m1e", other)))
	want := "no interface for m1e; machine ui runs one at 127.0.0.1:8765 (pid 5206); stop it with: metasystem ui stop --repo " + other.Checkout +
		", or start this seat's at another address with --listen"
	if b.spawns != 0 || got.Result.Code != 1 || got.Unchanged || !slices.Equal(got.Result.Lines, []string{want}) ||
		len(got.Seats) != 1 || got.Seats[0].Checkout != other.Checkout {
		t.Fatalf("start at the address another seat holds = spawns %d, %+v", b.spawns, got)
	}
	got = uiLifecycleRunWith("start", a.Roots, "127.0.0.1:9999", 0, b.effects(uiSeatsOf("m1e", other)))
	if b.spawns != 1 || got.Result.Code != 0 {
		t.Fatalf("start at another address = spawns %d, %+v", b.spawns, got)
	}
}

// TestUIStopRefusesAcrossSeatsWhenTheInventoryIsIncomplete (D-stop, US-01):
// uniqueness needs a complete inventory.
func TestUIStopRefusesAcrossSeatsWhenTheInventoryIsIncomplete(t *testing.T) {
	t.Parallel()
	b := newUISeatsBed(t)
	a, missing, lane := b.seat("m1e"), b.seat("m1x"), b.seat("landing")
	b.live(missing, 5208, "127.0.0.1:7878", identity.Alive)
	b.live(lane, 5209, "127.0.0.1:7879", identity.Alive)
	problem := "the fleet's presence cannot be read: boom"
	incomplete := func() uiSeatInventory {
		return uiSeatInventory{This: "m1e", Seats: []uiSeat{lane}, Problems: []string{problem}}
	}
	got := uiLifecycleRunWith("stop", a.Roots, "", 0, b.effects(incomplete))
	text := uiSeatsText(got)
	if len(b.sent) != 0 || !uiSeatsHasRecord(lane.Roots.StateRoot) || !uiSeatsHasRecord(missing.Roots.StateRoot) || got.Result.Code != 1 || got.Unchanged ||
		!strings.Contains(text, "the machines of this computer could not all be read: "+problem+"; nothing was stopped") ||
		!strings.Contains(text, "metasystem ui stop --repo "+lane.Checkout) || !slices.Equal(got.SeatsProblems, []string{problem}) {
		t.Fatalf("an incomplete inventory = signals %v, %+v", b.sent, got)
	}
	got = uiLifecycleRunWith("stop", a.Roots, "", 0, b.effects(uiSeatsOf("m1e", lane)))
	if !slices.Equal(b.sent, []uiSeatsSignal{{5209, syscall.SIGTERM}}) || got.Result.Code != 0 || got.Seat == nil || got.Seat.Name != "landing" {
		t.Fatalf("the same inventory complete = signals %v, %+v", b.sent, got)
	}
}

// TestUIStartKeepsTodaysPathWhenThisSeatRuns (D-start, US-02).
func TestUIStartKeepsTodaysPathWhenThisSeatRuns(t *testing.T) {
	t.Parallel()
	b := newUISeatsBed(t)
	a, other := b.seat("m1e"), b.seat("ui")
	b.live(a, 5210, "127.0.0.1:9999", identity.Alive)
	b.engine(a, "engine of m1e")
	b.live(other, 5211, "127.0.0.1:7878", identity.Alive)
	got := uiLifecycleRunWith("start", a.Roots, "127.0.0.1:7878", 0, b.effects(uiSeatsOf("m1e", other)))
	if b.spawns != 1 || got.Unchanged || strings.Contains(uiSeatsText(got), "machine ui") || len(got.Seats) != 0 {
		t.Fatalf("start while this seat runs elsewhere = spawns %d, %+v", b.spawns, got)
	}
}

// TestUIStopTimeoutWithHeldLock (D-proof, US-06): another seat's server
// that keeps its lock after SIGTERM is left running and reported.
func TestUIStopTimeoutWithHeldLock(t *testing.T) {
	t.Parallel()
	b := newUISeatsBed(t)
	b.after = uiSeatsFiredAfter
	a, other := b.seat("m1e"), b.seat("ui")
	b.live(other, 5212, "127.0.0.1:7878", identity.Alive)
	release := uiSeatsHoldLock(t, other.Roots.StateRoot)
	got := uiLifecycleRunWith("stop", a.Roots, "", 0, b.effects(uiSeatsOf("m1e", other)))
	if !slices.Equal(b.sent, []uiSeatsSignal{{5212, syscall.SIGTERM}}) || !uiSeatsHasRecord(other.Roots.StateRoot) || got.Result.Code != 1 || got.Unchanged ||
		!slices.Equal(got.Result.Lines, []string{"no interface for m1e; machine ui: interface (pid 5212) did not stop within 0s; it was sent SIGTERM and left running"}) {
		t.Fatalf("a seat that keeps its lock = signals %v, %+v", b.sent, got)
	}
	release()
}

// uiSeatsAddCheckout adds a checkout to the machine bed: a nickname when
// one is given, and a registry row when armed.
func uiSeatsAddCheckout(b *machineBed, name, nickname string, armed bool) string {
	b.t.Helper()
	dir := filepath.Join(filepath.Dir(b.this), name)
	for _, file := range []string{filepath.Join(dir, "metasystem.conf"), filepath.Join(dir, "bin", "metasystem")} {
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			b.t.Fatal(err)
		}
		if err := testexec.WriteFile(file, []byte("fixture\n"), 0o755); err != nil {
			b.t.Fatal(err)
		}
	}
	dir = realpath.Resolve(dir)
	b.extra = append(b.extra, dir)
	if nickname != "" {
		b.nicknames[dir] = nickname
	}
	if armed {
		payload, _ := json.Marshal(map[string]any{"schemaVersion": 1, "event": registry.EventRelaunched, "checkoutPath": dir, "ownerTag": "tag-" + name,
			"at": "2026-09-30T08:00:00Z", "generation": 1, "watcherTag": "tag-" + name + "-w", "reaperTag": "tag-" + name + "-r", "retiredThrough": 0})
		if err := registry.AppendFrame(b.registry, payload); err != nil {
			b.t.Fatal(err)
		}
	}
	return dir
}

// uiSeatsRunVerb runs a public ui verb on the machine bed with the
// interface seam answered by run.
func uiSeatsRunVerb(b *machineBed, run func(verb string, roots lifecycle.Roots, options uiIntentOptions) (uiLifecycleResult, error), args ...string) (int, intentResult, map[string]any) {
	b.t.Helper()
	owners := b.owners()
	owners.processes.ui = run
	command, rest, ok := resolveIntentArgv(append(args, "--json"))
	if !ok {
		b.t.Fatalf("no public command %q", args)
	}
	var stdout, stderr bytes.Buffer
	code := runIntentIn(command, rest, &stdout, &stderr, b.this, owners)
	var result intentResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		b.t.Fatalf("%v printed no JSON result: %v; %q %q", args, err, stdout.String(), stderr.String())
	}
	var raw struct {
		Data map[string]any `json:"data"`
	}
	_ = json.Unmarshal(stdout.Bytes(), &raw)
	return code, result, raw.Data
}

// uiSeatsInventoryOf reads the inventory a ui verb's options carry on the
// machine bed.
func uiSeatsInventoryOf(b *machineBed) uiSeatInventory {
	b.t.Helper()
	var inventory *uiSeatInventory
	uiSeatsRunVerb(b, func(verb string, _ lifecycle.Roots, options uiIntentOptions) (uiLifecycleResult, error) {
		if options.seats == nil {
			b.t.Fatalf("ui %s carries no seats reader", verb)
		}
		read := options.seats()
		inventory = &read
		return uiLifecycleResult{Result: lifecycle.Result{Lines: []string{"interface not running"}}, State: lifecycle.Stopped}, nil
	}, "ui", "status")
	if inventory == nil {
		b.t.Fatal("the seats reader was never reached")
	}
	return *inventory
}

func uiSeatsNames(inventory uiSeatInventory) []string {
	var names []string
	for _, s := range inventory.Seats {
		names = append(names, s.Name+"@"+s.Checkout)
	}
	return names
}

// TestUIOtherSeatsAreTheMachinesOfThisComputer (D-seats): the other seats
// are what machine list names, this checkout left out, and the inventory
// says when it is incomplete.
func TestUIOtherSeatsAreTheMachinesOfThisComputer(t *testing.T) {
	t.Parallel()
	b := newMachineBed(t)
	armed := uiSeatsAddCheckout(b, "agentic-tools-m1a", "m1a", true)
	uiSeatsAddCheckout(b, "scratch-clone", "", true)
	inventory := uiSeatsInventoryOf(b)
	want := []string{"agentic-tools-landing@" + b.landing, "m1a@" + armed, "m1x@" + b.other}
	if inventory.This != "m1e" || !slices.Equal(uiSeatsNames(inventory), want) || len(inventory.Problems) != 0 {
		t.Fatalf("inventory = %+v, want %v", inventory, want)
	}
	for _, s := range inventory.Seats {
		if s.Roots.Checkout != s.Checkout || s.Roots.Installation != s.Checkout || s.Roots.StateRoot != s.Checkout {
			t.Fatalf("seat %s roots = %+v", s.Name, s.Roots)
		}
	}

	b.fleetErr = errors.New("presence transport down")
	inventory = uiSeatsInventoryOf(b)
	if !slices.Equal(uiSeatsNames(inventory), []string{"agentic-tools-landing@" + b.landing, "m1a@" + armed}) ||
		len(inventory.Problems) != 1 || !strings.Contains(inventory.Problems[0], "presence transport down") {
		t.Fatalf("inventory with a failed fleet read = %+v", inventory)
	}

	b.fleetErr = nil
	b.registryUnreadable = true
	inventory = uiSeatsInventoryOf(b)
	if len(inventory.Problems) != 1 || !strings.Contains(inventory.Problems[0], "the host registry "+b.home+" cannot be read") {
		t.Fatalf("inventory with an unreadable registry = %+v", inventory)
	}
}

// TestUIStopRefusesReportProblemsWithNilError (D-seats, US-05): a fleet
// report that loses machines with a nil error is a problem, and stops
// nothing across seats.
func TestUIStopRefusesReportProblemsWithNilError(t *testing.T) {
	t.Parallel()
	for _, leg := range []struct {
		name    string
		edit    func(*seat.Report)
		problem string
	}{
		{"copy problem", func(report *seat.Report) { report.CopyProblem = "presence refs unreadable" }, "presence refs unreadable"},
		{"claims unavailable", func(report *seat.Report) { report.ClaimsUnavailable = "claims tip unreadable" }, "claims tip unreadable"},
	} {
		t.Run(leg.name, func(t *testing.T) {
			t.Parallel()
			m := newMachineBed(t)
			m.fleetEdit = leg.edit
			u := newUISeatsBed(t)
			u.live(uiSeat{Roots: lifecycle.Roots{StateRoot: m.landing}}, 5301, "127.0.0.1:7878", identity.Alive)
			code, result, data := uiSeatsRunVerb(m, func(verb string, roots lifecycle.Roots, options uiIntentOptions) (uiLifecycleResult, error) {
				return uiLifecycleRunWith(verb, roots, "", 0, u.effects(options.seats)), nil
			}, "ui", "stop")
			var lines []string
			for _, line := range data["lines"].([]any) {
				lines = append(lines, line.(string))
			}
			text := strings.Join(lines, "\n")
			problems, _ := data["seatsProblems"].([]any)
			if code != 1 || result.Outcome != intentRefused || len(u.sent) != 0 || !uiSeatsHasRecord(m.landing) ||
				!strings.Contains(text, leg.problem) || !strings.Contains(text, "nothing was stopped") ||
				len(problems) != 1 || !strings.Contains(problems[0].(string), leg.problem) {
				t.Fatalf("stop with a %s = %d signals %v %+v data %v", leg.name, code, u.sent, result, data)
			}
		})
	}
}

// TestUIStopRendersTheSeatItStopped (seams): the verb names the seat the
// lifecycle acted on.
func TestUIStopRendersTheSeatItStopped(t *testing.T) {
	t.Parallel()
	b := newProcessBed(t)
	owners := b.owners()
	other := uiSeat{Name: "ui", Checkout: "/work/agentic-tools-ui"}
	answer := uiLifecycleResult{Result: lifecycle.Result{Lines: []string{"no interface for m1e; stopped the interface of machine ui (pid 5201, :7878)"}}, Seat: &other}
	owners.processes.ui = func(string, lifecycle.Roots, uiIntentOptions) (uiLifecycleResult, error) { return answer, nil }
	code, result := b.runJSON(owners, "ui", "stop")
	if code != 0 || result.Outcome != intentConfirmed || len(result.Targets) != 1 || result.Targets[0] != (intentTarget{Kind: "ui", ID: other.Checkout}) ||
		result.Summary != answer.Result.Lines[0] {
		t.Fatalf("a stop of another seat = %d %+v", code, result)
	}
	if code, stdout, stderr := b.run(owners, "ui", "stop"); code != 0 || strings.Count(stdout+stderr, answer.Result.Lines[0]) != 1 {
		t.Fatalf("the line naming the seat is not printed once: %d %q %q", code, stdout, stderr)
	}
	answer = uiLifecycleResult{Result: lifecycle.Result{Lines: []string{"no interface for m1e; 2 machines of this computer run one; nothing was done"}, Code: 1},
		Seats: []uiSeatView{{Machine: "ui", Checkout: "/work/a"}, {Machine: "landing", Checkout: "/work/b"}}}
	code, result = b.runJSON(owners, "ui", "stop")
	if code != 1 || result.Outcome != intentRefused {
		t.Fatalf("a stop of several seats = %d %+v", code, result)
	}
}

// TestUIHelpNamesTheOtherSeats (D-words): the help of stop, status and
// start says what each does when another machine runs the interface.
func TestUIHelpNamesTheOtherSeats(t *testing.T) {
	t.Parallel()
	for verb, want := range map[string]string{
		"stop":   "When this seat runs no interface and exactly one other machine of this computer does, stop stops that one and says which; when several do, it lists each with its --repo command and stops none.",
		"status": "When this seat runs no interface, names the machine of this computer that does.",
		"start":  "When this seat runs no interface, names the machine of this computer that does.",
	} {
		_, page := readHelpJSON(t, "ui", verb, "--json")
		if page.Command == nil || !slices.Contains(page.Command.Details, want) {
			t.Errorf("ui %s help lacks %q: %+v", verb, want, page.Command)
		}
	}
	if _, page := readHelpJSON(t, "ui", "restart", "--json"); page.Command == nil || page.Command.Summary != "restart the browser interface with the checkout's own engine" {
		t.Errorf("ui restart help summary = %+v", page.Command)
	}
	if reason := idempotencyRows["ui restart"].why; strings.Contains(reason, "executable on disk") || !strings.Contains(reason, "the checkout's own engine") {
		t.Errorf("ui restart idempotency reason = %q", reason)
	}
}

// uiSeatsSpawned is the launch's engine, --repo, --metasystem-root and
// --listen.
func uiSeatsSpawned(spec lifecycle.LaunchSpec) []string {
	value := func(flag string) string { return spec.Args[slices.Index(spec.Args, flag)+1] }
	return []string{spec.Executable, value("--repo"), value("--metasystem-root"), value("--listen")}
}

// TestUIRestartRestartsTheOneOtherSeatsInterface (D-restart, one; AM-01,
// AM-02): with no interface on this seat and exactly one other machine
// running one, restart restarts that one with its own engine at its own
// address, and reports the pid and address the new server was ready with.
func TestUIRestartRestartsTheOneOtherSeatsInterface(t *testing.T) {
	t.Parallel()
	b := newUISeatsBed(t)
	a, other := b.seat("m1e"), b.seat("ui")
	b.live(other, 5501, "127.0.0.1:8765", identity.Alive)
	engine := b.engine(other, "engine of ui")
	b.conf(other, "ui.listen=127.0.0.1:8765\n")
	// The child announces another address than it was asked for: the
	// report carries what Launch returned (AM-02).
	b.ready = "127.0.0.1:8766"
	before := idemTreeDigest(t, a.Roots.StateRoot)
	got := uiLifecycleRunWith("restart", a.Roots, "127.0.0.1:7878", 0, b.effects(uiSeatsOf("m1e", other)))
	if !slices.Equal(b.sent, []uiSeatsSignal{{5501, syscall.SIGTERM}}) || len(b.specs) != 1 ||
		!slices.Equal(uiSeatsSpawned(b.specs[0]), []string{engine, other.Checkout, other.Roots.Installation, "127.0.0.1:8765"}) {
		t.Fatalf("restart of the other seat = signals %v launches %+v, %+v", b.sent, b.specs, got)
	}
	if got.Result.Code != 0 || got.Seat == nil || got.Seat.Name != "ui" || got.Restart == nil || got.Restart.Stop != lifecycle.StoppedNow ||
		!got.Restart.Started || got.Restart.Start.Code != 0 ||
		!slices.Equal(got.Result.Lines, []string{"no interface for m1e; restarted the interface of machine ui (pid 5501 -> 4343, :8766)"}) {
		t.Fatalf("restart of the other seat = %+v %+v", got, got.Restart)
	}
	idemSameTree(t, "this seat's tree", before, idemTreeDigest(t, a.Roots.StateRoot))

	// A --listen typed takes precedence over the target's own address.
	typed := newUISeatsBed(t)
	typed.listen, typed.listenSet = "127.0.0.1:9999", true
	a, other = typed.seat("m1e"), typed.seat("ui")
	typed.live(other, 5502, "127.0.0.1:8765", identity.Alive)
	typed.engine(other, "engine of ui")
	typed.conf(other, "ui.listen=127.0.0.1:8765\n")
	got = uiLifecycleRunWith("restart", a.Roots, "127.0.0.1:7878", 0, typed.effects(uiSeatsOf("m1e", other)))
	if len(typed.specs) != 1 || uiSeatsSpawned(typed.specs[0])[3] != "127.0.0.1:9999" || got.Result.Code != 0 {
		t.Fatalf("restart with --listen = launches %+v, %+v", typed.specs, got)
	}

	// The target's address that does not validate stops nothing.
	invalid := newUISeatsBed(t)
	a, other = invalid.seat("m1e"), invalid.seat("ui")
	invalid.live(other, 5503, "127.0.0.1:8765", identity.Alive)
	invalid.engine(other, "engine of ui")
	invalid.conf(other, "ui.listen=example.com:80\n")
	got = uiLifecycleRunWith("restart", a.Roots, "127.0.0.1:7878", 0, invalid.effects(uiSeatsOf("m1e", other)))
	if len(invalid.sent) != 0 || invalid.spawns != 0 || !uiSeatsHasRecord(other.Roots.StateRoot) || got.Result.Code != 1 ||
		len(got.Result.Lines) != 1 || !strings.Contains(got.Result.Lines[0], "the listen address must use a loopback IP literal") || !strings.Contains(got.Result.Lines[0], "nothing was done") {
		t.Fatalf("restart at an invalid address = signals %v spawns %d, %+v", invalid.sent, invalid.spawns, got)
	}
}

// TestUIStartLaunchesTheTargetInstallationsEngine (D-engine): start and
// restart launch the engine the target installation carries, never the
// process's own executable.
func TestUIStartLaunchesTheTargetInstallationsEngine(t *testing.T) {
	t.Parallel()
	b := newUISeatsBed(t)
	a, other := b.seat("m1e"), b.seat("ui")
	own := b.engine(a, "engine of m1e")
	got := uiLifecycleRunWith("start", a.Roots, "127.0.0.1:9999", 0, b.effects(uiSeatsOf("m1e", other)))
	if got.Result.Code != 0 || len(b.specs) != 1 || !slices.Equal(uiSeatsSpawned(b.specs[0]), []string{own, a.Checkout, a.Roots.Installation, "127.0.0.1:9999"}) ||
		!slices.Contains(b.engines, a.Roots.Installation) {
		t.Fatalf("start = launches %+v engines %v, %+v", b.specs, b.engines, got)
	}

	b.live(other, 5504, "127.0.0.1:8765", identity.Alive)
	theirs := b.engine(other, "engine of ui")
	third := b.seat("landing")
	got = uiLifecycleRunWith("restart", third.Roots, "127.0.0.1:7878", 0, b.effects(uiSeatsOf("landing", other)))
	if got.Result.Code != 0 || len(b.specs) != 2 || uiSeatsSpawned(b.specs[1])[0] != theirs || uiSeatsSpawned(b.specs[1])[1] != other.Checkout ||
		uiSeatsSpawned(b.specs[1])[2] != other.Roots.Installation || !slices.Contains(b.engines, other.Roots.Installation) {
		t.Fatalf("restart of another seat = launches %+v engines %v, %+v", b.specs, b.engines, got)
	}
}

// TestUIRestartRefusesWhenTheTargetEngineIsMissing (D-engine): no engine,
// nothing stopped.
func TestUIRestartRefusesWhenTheTargetEngineIsMissing(t *testing.T) {
	t.Parallel()
	b := newUISeatsBed(t)
	a, other := b.seat("m1e"), b.seat("ui")
	b.live(other, 5505, "127.0.0.1:8765", identity.Alive)
	got := uiLifecycleRunWith("restart", a.Roots, "127.0.0.1:7878", 0, b.effects(uiSeatsOf("m1e", other)))
	text := uiSeatsText(got)
	if len(b.sent) != 0 || b.spawns != 0 || !uiSeatsHasRecord(other.Roots.StateRoot) || got.Result.Code != 1 ||
		!strings.Contains(text, other.Roots.Installation) || !strings.Contains(text, "carries no engine at bin/metasystem") {
		t.Fatalf("restart of a seat without an engine = signals %v spawns %d, %+v", b.sent, b.spawns, got)
	}
	got = uiLifecycleRunWith("start", a.Roots, "127.0.0.1:9999", 0, b.effects(uiSeatsOf("m1e", other)))
	text = uiSeatsText(got)
	if b.spawns != 0 || got.Result.Code != 1 || !strings.Contains(text, a.Roots.Installation) || !strings.Contains(text, "carries no engine at bin/metasystem") {
		t.Fatalf("start without an engine = spawns %d, %+v", b.spawns, got)
	}
}

// TestUIRestartWithSeveralOrIncompleteRestartsNone (D-restart, several and
// incomplete).
func TestUIRestartWithSeveralOrIncompleteRestartsNone(t *testing.T) {
	t.Parallel()
	b := newUISeatsBed(t)
	a, first, second := b.seat("m1e"), b.seat("ui"), b.seat("landing")
	b.live(first, 5506, "127.0.0.1:7878", identity.Alive)
	b.live(second, 5507, "127.0.0.1:7879", identity.Alive)
	b.engine(first, "engine of ui")
	b.engine(second, "engine of landing")
	got := uiLifecycleRunWith("restart", a.Roots, "127.0.0.1:7878", 0, b.effects(uiSeatsOf("m1e", first, second)))
	text := uiSeatsText(got)
	if len(b.sent) != 0 || b.spawns != 0 || !uiSeatsHasRecord(first.Roots.StateRoot) || !uiSeatsHasRecord(second.Roots.StateRoot) || got.Result.Code != 1 ||
		!strings.Contains(text, "machine ui: pid 5506 at 127.0.0.1:7878: metasystem ui restart --repo "+first.Checkout) ||
		!strings.Contains(text, "machine landing: pid 5507 at 127.0.0.1:7879: metasystem ui restart --repo "+second.Checkout) {
		t.Fatalf("restart with several = signals %v spawns %d, %+v", b.sent, b.spawns, got)
	}

	m := newMachineBed(t)
	m.fleetEdit = func(report *seat.Report) { report.CopyProblem = "presence refs unreadable" }
	u := newUISeatsBed(t)
	u.live(uiSeat{Roots: lifecycle.Roots{StateRoot: m.landing}}, 5508, "127.0.0.1:7878", identity.Alive)
	code, result, data := uiSeatsRunVerb(m, func(verb string, roots lifecycle.Roots, options uiIntentOptions) (uiLifecycleResult, error) {
		return uiLifecycleRunWith(verb, roots, "127.0.0.1:7878", 0, u.effects(options.seats)), nil
	}, "ui", "restart")
	var lines []string
	for _, line := range data["lines"].([]any) {
		lines = append(lines, line.(string))
	}
	problems, _ := data["seatsProblems"].([]any)
	if code != 1 || result.Outcome != intentRefused || len(u.sent) != 0 || u.spawns != 0 || !uiSeatsHasRecord(m.landing) ||
		!strings.Contains(strings.Join(lines, "\n"), "presence refs unreadable") || len(problems) != 1 {
		t.Fatalf("restart with an incomplete inventory = %d signals %v spawns %d %+v data %v", code, u.sent, u.spawns, result, data)
	}
}

// TestUIRestartTimeoutWithHeldLock (D-restart, D-proof): the restart's stop
// leg on another seat that keeps its lock starts nothing.
func TestUIRestartTimeoutWithHeldLock(t *testing.T) {
	t.Parallel()
	b := newUISeatsBed(t)
	b.after = uiSeatsFiredAfter
	a, other := b.seat("m1e"), b.seat("ui")
	b.live(other, 5509, "127.0.0.1:7878", identity.Alive)
	b.engine(other, "engine of ui")
	release := uiSeatsHoldLock(t, other.Roots.StateRoot)
	got := uiLifecycleRunWith("restart", a.Roots, "127.0.0.1:7878", 0, b.effects(uiSeatsOf("m1e", other)))
	if !slices.Equal(b.sent, []uiSeatsSignal{{5509, syscall.SIGTERM}}) || b.spawns != 0 || !uiSeatsHasRecord(other.Roots.StateRoot) || got.Result.Code != 1 ||
		got.Restart == nil || got.Restart.Stop != lifecycle.Timeout || got.Seat == nil || got.Seat.Name != "ui" {
		t.Fatalf("restart of a seat that keeps its lock = signals %v spawns %d, %+v %+v", b.sent, b.spawns, got, got.Restart)
	}
	release()
}

// TestUIStatusComparesTheTargetInstallationsEngine (D-engine): status
// compares the running server with the engine restart would launch.
func TestUIStatusComparesTheTargetInstallationsEngine(t *testing.T) {
	t.Parallel()
	b := newUISeatsBed(t)
	a := b.seat("m1e")
	engine := b.engine(a, "engine of m1e")
	digest, err := uiFileDigest(engine)
	if err != nil {
		t.Fatal(err)
	}
	b.live(a, 5510, "127.0.0.1:7878", identity.Alive)
	uiSeatsSetDigest(t, a.Roots.StateRoot, digest)
	changed := "the executable on disk differs from the one the interface is running; to pick it up: metasystem ui restart"
	got := uiLifecycleRunWith("status", a.Roots, "", 0, b.effects(uiSeatsOf("m1e")))
	if got.State != lifecycle.Running || slices.Contains(got.Result.Lines, changed) {
		t.Fatalf("status with the installation's engine running = %+v", got)
	}
	b.engine(a, "engine of m1e, rebuilt")
	got = uiLifecycleRunWith("status", a.Roots, "", 0, b.effects(uiSeatsOf("m1e")))
	if !slices.Contains(got.Result.Lines, changed) {
		t.Fatalf("status after the installation's engine changed = %+v", got)
	}
}

func uiSeatsSetDigest(t *testing.T, stateRoot, digest string) {
	t.Helper()
	path := filepath.Join(lifecycle.Dir(stateRoot), "server.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var rec lifecycle.Record
	if err := json.Unmarshal(data, &rec); err != nil {
		t.Fatal(err)
	}
	rec.ExecutableDigest = digest
	encoded, _ := json.Marshal(rec)
	if err := os.WriteFile(path, encoded, 0o600); err != nil {
		t.Fatal(err)
	}
}

// TestUIRestartRendersTheSeatItRestarted (D-restart, seams): a restart of
// another machine's interface targets that seat and names it in its next
// steps; a refusal before anything stopped carries its decision; a start
// that fails where nothing ran does not say something stopped.
func TestUIRestartRendersTheSeatItRestarted(t *testing.T) {
	t.Parallel()
	b := newProcessBed(t)
	owners := b.owners()
	other := uiSeat{Name: "ui", Checkout: "/work/agentic-tools-ui"}
	var answer uiLifecycleResult
	owners.processes.ui = func(string, lifecycle.Roots, uiIntentOptions) (uiLifecycleResult, error) { return answer, nil }

	line := "no interface for m1e; restarted the interface of machine ui (pid 5501 -> 4343, :8765)"
	answer = uiLifecycleResult{Result: lifecycle.Result{Lines: []string{line}}, Seat: &other,
		Restart: &lifecycle.RestartReport{Stop: lifecycle.StoppedNow, Started: true}}
	code, result := b.runJSON(owners, "ui", "restart")
	if code != 0 || result.Outcome != intentConfirmed || result.Summary != line || len(result.Targets) != 1 || result.Targets[0].ID != other.Checkout {
		t.Fatalf("a restart of another seat = %d %+v", code, result)
	}
	if code, stdout, stderr := b.run(owners, "ui", "restart"); code != 0 || strings.Count(stdout+stderr, line) != 1 {
		t.Fatalf("the line naming the restarted seat is not printed once: %d %q %q", code, stdout, stderr)
	}

	answer = uiLifecycleResult{Result: lifecycle.Result{Lines: []string{"no interface for m1e; machine ui: interface (pid 5501) did not stop within 0s; it was sent SIGTERM and left running"}, Code: 1},
		Seat: &other, Restart: &lifecycle.RestartReport{Stop: lifecycle.Timeout}}
	code, result = b.runJSON(owners, "ui", "restart")
	if code != 1 || result.Outcome != intentPartial || result.Next == nil || !slices.Equal(result.Next.Argv, []string{"metasystem", "ui", "restart", "--repo", other.Checkout}) {
		t.Fatalf("a timed-out restart of another seat = %d %+v", code, result)
	}

	answer = uiLifecycleResult{Result: lifecycle.Result{Lines: []string{"the installation /work/a carries no engine at bin/metasystem; nothing was done"}, Code: 1},
		Decision: uiEngineDecision}
	code, result = b.runJSON(owners, "ui", "restart")
	if code != 1 || result.Outcome != intentRefused || result.Decision != uiEngineDecision {
		t.Fatalf("a restart without an engine = %d %+v", code, result)
	}

	answer = uiLifecycleResult{Result: lifecycle.Result{Lines: []string{"cannot launch the interface server: boom"}, Code: 1},
		Restart: &lifecycle.RestartReport{Stop: lifecycle.StopOutcome(lifecycle.Stopped), Started: true, Start: lifecycle.Result{Code: 1}}}
	code, result = b.runJSON(owners, "ui", "restart")
	if code != 1 || result.Outcome != intentPartial || strings.Contains(result.Summary, "stopped but") {
		t.Fatalf("a restart that started nothing where nothing ran = %d %+v", code, result)
	}
}

// TestUIRestartIsNotBlockedByTheCallersListen (SOL-US-02): the caller's own
// ui.listen is resolved for the caller's own restart only; restarting the
// one other seat uses that seat's address, through the verb's own seam.
func TestUIRestartIsNotBlockedByTheCallersListen(t *testing.T) {
	t.Parallel()
	b := newUISeatsBed(t)
	a, other := b.seat("m1e"), b.seat("ui")
	b.conf(a, "ui.listen=example.com:80\n")
	b.live(other, 5601, "127.0.0.1:8765", identity.Alive)
	b.engine(other, "engine of ui")
	b.conf(other, "ui.listen=127.0.0.1:8765\n")
	options := uiIntentOptions{seats: uiSeatsOf("m1e", other)}
	got, err := uiLifecycleForWith("restart", a.Roots, options, b.effects(nil))
	if err != nil || got.Result.Code != 0 || got.Seat == nil || got.Seat.Name != "ui" || len(b.specs) != 1 ||
		uiSeatsSpawned(b.specs[0])[3] != "127.0.0.1:8765" {
		t.Fatalf("restart of a healthy seat from one with an invalid ui.listen = %v %+v, launches %+v", err, got, b.specs)
	}

	// The caller's own restart still refuses on its own address, and
	// stops nothing.
	own := newUISeatsBed(t)
	a = own.seat("m1e")
	own.conf(a, "ui.listen=example.com:80\n")
	own.live(a, 5602, "127.0.0.1:7878", identity.Alive)
	own.engine(a, "engine of m1e")
	for _, seats := range []func() uiSeatInventory{uiSeatsOf("m1e"), nil} {
		got, err = uiLifecycleForWith("restart", a.Roots, uiIntentOptions{seats: seats}, own.effects(nil))
		refused := err != nil && strings.Contains(err.Error(), "loopback") ||
			err == nil && got.Result.Code == 1 && len(got.Result.Lines) == 1 && strings.Contains(got.Result.Lines[0], "loopback")
		if !refused || len(own.sent) != 0 || own.spawns != 0 || !uiSeatsHasRecord(a.Roots.StateRoot) {
			t.Fatalf("the caller's own restart with an invalid ui.listen = %v %+v, signals %v spawns %d", err, got, own.sent, own.spawns)
		}
	}
	a2 := own.seat("m1x")
	own.conf(a2, "ui.listen=example.com:80\n")
	got, err = uiLifecycleForWith("restart", a2.Roots, uiIntentOptions{seats: uiSeatsOf("m1x")}, own.effects(nil))
	if err == nil && got.Result.Code == 0 || own.spawns != 0 {
		t.Fatalf("restart with no other seat and an invalid ui.listen = %v %+v, spawns %d", err, got, own.spawns)
	}
}

// TestUICrossSeatFailuresShowTheirDiagnosticOnce (SOL-US-03): in text
// output a cross-seat failure's own line is printed exactly once, whether
// the summary is that line or a generic one.
func TestUICrossSeatFailuresShowTheirDiagnosticOnce(t *testing.T) {
	t.Parallel()
	b := newProcessBed(t)
	owners := b.owners()
	other := uiSeat{Name: "ui", Checkout: "/work/agentic-tools-ui"}
	var answer uiLifecycleResult
	owners.processes.ui = func(string, lifecycle.Roots, uiIntentOptions) (uiLifecycleResult, error) { return answer, nil }
	timeout := "no interface for m1e; machine ui: interface (pid 5201) did not stop within 0s; it was sent SIGTERM and left running"
	failedStart := "no interface for m1e; machine ui: cannot launch the interface server: exec format error"
	for _, leg := range []struct {
		name, verb, line string
		answer           uiLifecycleResult
	}{
		{"stop timeout", "stop", timeout, uiLifecycleResult{Result: lifecycle.Result{Lines: []string{timeout}, Code: 1}, Seat: &other}},
		{"restart timeout", "restart", timeout, uiLifecycleResult{Result: lifecycle.Result{Lines: []string{timeout}, Code: 1}, Seat: &other,
			Restart: &lifecycle.RestartReport{Stop: lifecycle.Timeout}}},
		{"restart whose start fails", "restart", failedStart, uiLifecycleResult{Result: lifecycle.Result{Lines: []string{failedStart}, Code: 1}, Seat: &other,
			Restart: &lifecycle.RestartReport{Stop: lifecycle.StoppedNow, Started: true, Start: lifecycle.Result{Code: 1}}}},
	} {
		answer = leg.answer
		code, stdout, stderr := b.run(owners, "ui", leg.verb)
		if code != 1 || strings.Count(stdout+stderr, leg.line) != 1 {
			t.Errorf("%s: the diagnostic is not printed exactly once: %d %q %q", leg.name, code, stdout, stderr)
		}
	}
}
