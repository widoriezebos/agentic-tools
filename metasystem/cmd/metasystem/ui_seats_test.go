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
	return uiSeat{Name: name, Checkout: root, Roots: lifecycle.Roots{Checkout: root, Installation: root, StateRoot: root}}
}

// live writes the seat's record of a server at pid and address, and its
// unheld lock, as the server writes them; the prober answers pid state.
func (b *uiSeatsBed) live(s uiSeat, pid int64, address string, state identity.Liveness) {
	b.t.Helper()
	uiSeatsWriteRecord(b.t, s.Roots.StateRoot, pid, address)
	b.prober[pid] = state
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
		prober:     b.prober,
		executable: func() (string, error) { return "/fake/metasystem", nil },
		spawn: func(spec lifecycle.LaunchSpec) (lifecycle.Child, error) {
			b.spawns++
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

// TestUIRestartInAnotherSeatPointsWithoutStoppingIt (D-start, restart).
func TestUIRestartInAnotherSeatPointsWithoutStoppingIt(t *testing.T) {
	t.Parallel()
	b := newUISeatsBed(t)
	a, other := b.seat("m1e"), b.seat("ui")
	b.live(other, 5207, "127.0.0.1:7878", identity.Alive)
	got := uiLifecycleRunWith("restart", a.Roots, "127.0.0.1:7878", 0, b.effects(uiSeatsOf("m1e", other)))
	if got.Restart == nil || got.Restart.Stop != lifecycle.StopOutcome(lifecycle.Stopped) || !got.Restart.Started || got.Restart.Start.Code != 1 ||
		len(b.sent) != 0 || b.spawns != 0 || !uiSeatsHasRecord(other.Roots.StateRoot) || !strings.Contains(uiSeatsText(got), "metasystem ui stop --repo "+other.Checkout) {
		t.Fatalf("restart beside another seat = signals %v spawns %d, %+v %+v", b.sent, b.spawns, got, got.Restart)
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
}
