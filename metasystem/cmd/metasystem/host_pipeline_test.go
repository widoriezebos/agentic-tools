package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/board"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batchowner"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/registry"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

type pipelineProber struct{}

func (pipelineProber) Probe(pid int64) (identity.Exact, identity.Liveness, error) {
	if pid == 999 {
		return identity.Exact{}, identity.Dead, nil
	}
	return identity.Exact{Pid: pid, StartedAt: time.Unix(1000, 0)}, identity.Alive, nil
}

// pipelineHost is a fixture host: a registry, checkout directories, their
// nicknames and a board, all under one temporary home.
type pipelineHost struct {
	root, registryPath, home string
	nicknames                map[string]string
}

func newPipelineHost(t *testing.T) *pipelineHost {
	t.Helper()
	root := t.TempDir()
	host := &pipelineHost{root: root, registryPath: filepath.Join(root, "armed-checkouts.jsonl"), home: filepath.Join(root, ".metasystem"), nicknames: map[string]string{}}
	return host
}

// checkout creates an armed (or, closed, disarmed) checkout named machine.
func (host *pipelineHost) checkout(t *testing.T, name, machine string, closed, gone bool) string {
	t.Helper()
	path := filepath.Join(host.root, name, "metasystem")
	if !gone {
		if err := os.MkdirAll(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	host.nicknames[path] = machine
	events := []string{registry.EventRelaunched}
	if closed {
		events = append(events, registry.EventExited)
	}
	for _, event := range events {
		payload, _ := json.Marshal(map[string]any{"schemaVersion": 1, "event": event, "checkoutPath": path, "ownerTag": "tag-" + name,
			"at": "2026-09-29T08:00:00Z", "generation": 1, "watcherTag": "w-" + name, "reaperTag": "r-" + name, "retiredThrough": 0,
			"reason": "shutdown", "teardownComplete": true})
		if err := registry.AppendFrame(host.registryPath, payload); err != nil {
			t.Fatal(err)
		}
	}
	return path
}

func (host *pipelineHost) card(t *testing.T, machine, installation, goalID string, stage board.Stage, at time.Time) {
	t.Helper()
	card := board.Card{Seat: board.Seat{Machine: machine, Installation: installation}, Goal: goalID, Stage: stage, Writer: board.Writer{At: at}}
	if stage == board.StageBuild {
		card.Owner = &board.Owner{Pid: 41, PidStartedAt: 1000}
	}
	if err := board.WriteAt(host.home, card); err != nil {
		t.Fatal(err)
	}
}

func (host *pipelineHost) source(claims map[string]string) batchowner.HostPipeline {
	source := batchowner.ProductionPipeline(20*time.Minute, func() (map[string]string, error) { return claims, nil })
	source.Registry = func() (string, error) { return host.registryPath, nil }
	source.Home = func() (string, error) { return host.home, nil }
	source.Prober = pipelineProber{}
	source.Machine = func(checkout string) (string, error) {
		if machine, ok := host.nicknames[checkout]; ok && machine != "" {
			return machine, nil
		}
		return "", errors.New("no nickname")
	}
	return source
}

// viewReasons are a board view's Unknown goals as machine/goal → reason,
// and a seat Unknown as a whole under machine/.
func viewReasons(view board.View) map[string]string {
	reasons := map[string]string{}
	for _, seat := range view.Seats {
		if seat.Unknown != "" {
			reasons[seat.Machine+"/"] = seat.Unknown
		}
		for _, goal := range seat.Goals {
			if goal.Unknown != "" {
				reasons[seat.Machine+"/"+goal.Goal] = goal.Unknown
			}
		}
	}
	return reasons
}

// viewBelieved are a board view's goals that are not Unknown, in seat order.
func viewBelieved(view board.View) []string {
	var believed []string
	for _, seat := range view.Seats {
		for _, goal := range seat.Goals {
			if goal.Unknown == "" {
				believed = append(believed, goal.Goal)
			}
		}
	}
	return believed
}

// TestBatchPipelineReadsOnlyTheArmedSeats (R24, U10b-1; the production
// source's half of TestBoardReadsOnlyTheSeatsItIsGiven): a fixture registry
// with two open seats, a closed one, one whose directory is gone, a board
// directory with no registration, a card of a foreign installation and two
// armed checkouts with one nickname: the source reads exactly the armed,
// present seats with their nicknames, the foreign card is Unknown with its
// reason, and the shared nickname makes the seat Unknown.
func TestBatchPipelineReadsOnlyTheArmedSeats(t *testing.T) {
	t.Parallel()
	host := newPipelineHost(t)
	now := time.Date(2026, 9, 29, 10, 0, 0, 0, time.UTC)
	m1b := host.checkout(t, "b", "m1b", false, false)
	m1c := host.checkout(t, "c", "m1c", false, false)
	closed := host.checkout(t, "closed", "m1x", true, false)
	host.checkout(t, "gone", "m1g", false, true)
	host.checkout(t, "e1", "m1e", false, false)
	host.checkout(t, "e2", "m1e", false, false)
	host.card(t, "m1b", m1b, "goal-b", board.StageBuild, now)
	host.card(t, "m1c", "/elsewhere/metasystem", "goal-c", board.StageBuild, now)
	host.card(t, "m1x", closed, "goal-x", board.StageBuild, now)
	host.card(t, "m1g", "/gone", "goal-g", board.StageBuild, now)
	host.card(t, "stranger", "/stranger", "goal-s", board.StageBuild, now)
	_ = m1c
	view := host.source(map[string]string{"goal-b": "m1b", "goal-c": "m1c", "goal-x": "m1x", "goal-g": "m1g", "goal-s": "stranger"}).View(now)
	reasons := viewReasons(view)
	if believed := viewBelieved(view); !view.Readable || len(believed) != 1 || believed[0] != "goal-b" {
		t.Fatalf("believed %v unknown %v (%v %s)", believed, reasons, view.Readable, view.Reason)
	}
	if !strings.Contains(reasons["m1c/goal-c"], "not the armed checkout") {
		t.Fatalf("the foreign installation: %v", reasons)
	}
	if !strings.Contains(reasons["m1e/"], "shared") {
		t.Fatalf("the shared nickname's seat is not Unknown: %v", reasons)
	}
	for _, seat := range view.Seats {
		if seat.Machine == "m1x" || seat.Machine == "m1g" || seat.Machine == "stranger" {
			t.Fatalf("a closed, gone or unregistered seat was read: %+v", seat)
		}
	}
}

// TestBatchPipelineKeepsTheRegistryError (R24, U10b-1; the source's half of
// TestArmedCheckoutsKeepTheirError): an unreadable registry is an
// unreadable view with the cause; an empty registry is a readable, empty
// one.
func TestBatchPipelineKeepsTheRegistryError(t *testing.T) {
	t.Parallel()
	host := newPipelineHost(t)
	if err := os.Mkdir(host.registryPath, 0o700); err != nil {
		t.Fatal(err)
	}
	if view := host.source(nil).View(time.Now()); view.Readable || !strings.HasPrefix(view.Reason, "registry: ") {
		t.Fatalf("an unreadable registry = %+v", view)
	}
	empty := newPipelineHost(t)
	if view := empty.source(nil).View(time.Now()); !view.Readable || len(view.Seats) != 0 {
		t.Fatalf("an empty registry = %+v", view)
	}
}

// TestBatchPipelineChecksTheLedger (R24, U10b-1; the source's half of
// TestUnknownIsNeverNear): a card under m1b for a goal the ledger's live
// claim gives to m1c is claim moved; a live card for a goal nobody claims is
// not claimed; a goal claimed by a seat of this host with no card is no
// card; the same card with a matching claim is believed.
func TestBatchPipelineChecksTheLedger(t *testing.T) {
	t.Parallel()
	host := newPipelineHost(t)
	now := time.Date(2026, 9, 29, 10, 0, 0, 0, time.UTC)
	m1b := host.checkout(t, "b", "m1b", false, false)
	host.checkout(t, "c", "m1c", false, false)
	host.card(t, "m1b", m1b, "goal-moved", board.StageBuild, now)
	host.card(t, "m1b", m1b, "goal-unclaimed", board.StageBuild, now)
	host.card(t, "m1b", m1b, "goal-kept", board.StageBuild, now)
	host.card(t, "m1b", m1b, "goal-done", board.StageReleased, now)
	view := host.source(map[string]string{"goal-moved": "m1c", "goal-kept": "m1b", "goal-silent": "m1c", "goal-elsewhere": "m1z"}).View(now)
	reasons := viewReasons(view)
	if reasons["m1b/goal-moved"] != "claim moved to m1c" || reasons["m1b/goal-unclaimed"] != "not claimed" || reasons["m1c/goal-silent"] != "no card" {
		t.Fatalf("ledger reasons %v", reasons)
	}
	if _, named := reasons["m1z/goal-elsewhere"]; named {
		t.Fatal("a claim of another host's machine is none of this board's business")
	}
	believed := viewBelieved(view)
	slices.Sort(believed)
	if strings.Join(believed, ",") != "goal-done,goal-kept" {
		t.Fatalf("believed %v", believed)
	}
}

func TestStewardRetiresTheCardsOfDoneGoals(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"template", "adopted-nested", "two-seats", "ledger-unreadable", "registry-unreadable", "home-unreadable", "no-ledger", "root-unresolvable"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			host := newPipelineHost(t)
			now := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
			b := host.checkout(t, "app/tools", "m1b", false, false)
			x := host.checkout(t, "closed", "m1x", true, false)
			n := host.checkout(t, "unnamed", "", false, false)
			conf := ""
			if name == "template" {
				conf = "metasystem.template=true\n"
			}
			if err := os.WriteFile(filepath.Join(b, "metasystem.conf"), []byte(conf), 0600); err != nil {
				t.Fatal(err)
			}
			for machine, install := range map[string]string{"m1b": b, "m1x": x, "n": n} {
				host.card(t, machine, install, "goal-done", board.StageLanded, now)
			}
			host.card(t, "m1b", b, "goal-live", board.StageClaimedIdle, now)
			host.card(t, "m1b", b, "goal-released", board.StageReleased, now)
			host.card(t, "m1b", b, "goal-both", board.StageClaimedIdle, now)
			cardPath := func(machine, id string) string { return filepath.Join(board.Dir(host.home), machine, id+".json") }
			bytesAt := func(path string) string {
				t.Helper()
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				return string(data)
			}
			before := map[string]string{}
			for _, pair := range [][2]string{{"m1b", "goal-done"}, {"m1x", "goal-done"}, {"n", "goal-done"}, {"m1b", "goal-live"}, {"m1b", "goal-released"}, {"m1b", "goal-both"}} {
				path := cardPath(pair[0], pair[1])
				before[path] = bytesAt(path)
			}
			tree := &goal.TreeGoals{Done: map[string]*goal.GoalFile{"goal-done": {State: goal.StateDone}, "goal-both": {State: goal.StateDone}}, Live: map[string]*goal.GoalFile{"goal-live": {State: goal.StateClaimed}, "goal-released": {State: goal.StateApproved}, "goal-both": {State: goal.StateApproved}}}
			roots := map[string]string{b: filepath.Join(host.root, "app")}
			trees := map[string]*goal.TreeGoals{roots[b]: tree, b: tree}
			if name == "two-seats" {
				c := host.checkout(t, "other", "m1c", false, false)
				if err := os.WriteFile(filepath.Join(c, "metasystem.conf"), nil, 0600); err != nil {
					t.Fatal(err)
				}
				roots[c] = filepath.Dir(c)
				trees[roots[c]] = &goal.TreeGoals{Live: map[string]*goal.GoalFile{"goal-done": {State: goal.StateApproved}}}
				host.card(t, "m1c", c, "goal-done", board.StageClaimedIdle, now)
				before[cardPath("m1c", "goal-done")] = bytesAt(cardPath("m1c", "goal-done"))
			}
			resolver := stateroot.NewResolver(func(install string) (string, error) {
				if name == "root-unresolvable" {
					return "", errors.New("state root unavailable")
				}
				return roots[install], nil
			}, func() (string, error) { t.Fatal("executable discovery is not needed"); return "", nil })
			var read []string
			ledger := func(root string) (*goal.TreeGoals, error) {
				read = append(read, root)
				if name == "ledger-unreadable" {
					return nil, errors.New("ledger unreadable")
				}
				if name == "no-ledger" {
					return nil, nil
				}
				tree, ok := trees[root]
				if !ok {
					t.Fatalf("ledger read at unexpected root %s", root)
				}
				return tree, nil
			}
			source := host.source(nil)
			if name == "registry-unreadable" {
				source.Registry = func() (string, error) { return "", errors.New("registry unreadable") }
			}
			if name == "home-unreadable" {
				source.Home = func() (string, error) { return "", errors.New("home unreadable") }
			}
			if err := source.RetireDone(resolver.RootForInstallation, ledger)("ignored", now); err != nil {
				t.Fatal(err)
			}
			moved := name == "template" || name == "adopted-nested" || name == "two-seats"
			for path, want := range before {
				if path == cardPath("m1b", "goal-done") && moved {
					if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
						t.Fatalf("done card remains: %v", err)
					}
					path = filepath.Join(board.Dir(host.home), board.RetiredNamespace, "m1b", "goal-done.20261021T120000Z.done.json")
				}
				if got := bytesAt(path); got != want {
					t.Fatalf("%s bytes changed", path)
				}
			}
			archived, err := filepath.Glob(filepath.Join(board.Dir(host.home), board.RetiredNamespace, "*", "*.json"))
			if err != nil || moved && len(archived) != 1 || !moved && len(archived) != 0 {
				t.Fatalf("archives %v: %v", archived, err)
			}
			view := host.source(map[string]string{"goal-live": "m1b", "goal-both": "m1b"}).View(now)
			switch name {
			case "template", "adopted-nested":
				wantRoot := roots[b]
				if name == "template" {
					wantRoot = b
				}
				if !slices.Equal(read, []string{wantRoot}) || !slices.Contains(viewBelieved(view), "goal-live") || slices.Contains(viewBelieved(view), "goal-done") {
					t.Fatalf("roots %v, view %+v", read, view)
				}
			case "two-seats":
				if len(read) != 2 {
					t.Fatalf("each seat needs its own ledger: %v", read)
				}
			case "ledger-unreadable", "root-unresolvable":
				reason := "ledger unreadable"
				if name == "root-unresolvable" {
					reason = "state root unavailable"
				}
				if !slices.Equal(view.Kept, []string{"card kept: m1b/*: " + reason}) {
					t.Fatalf("kept lines %v", view.Kept)
				}
			case "registry-unreadable", "home-unreadable":
				if len(read) != 0 {
					t.Fatalf("read ledger without host evidence: %v", read)
				}
				if _, err := os.Stat(filepath.Join(board.Dir(host.home), board.RetiredNamespace)); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("retired area written: %v", err)
				}
			}
			if name != "ledger-unreadable" && name != "root-unresolvable" && len(view.Kept) != 0 {
				t.Fatalf("unexpected kept lines: %v", view.Kept)
			}
		})
	}
}
