package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/board"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/hostload"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/registry"
	metarun "github.com/widoriezebos/agentic-tools/metasystem/internal/run"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
)

// situationArmed is a host registry naming each checkout armed.
func situationArmed(t *testing.T, checkouts ...string) string {
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

// situationTree is every path under root with its size and change time,
// Git's own directory included: the read fetches nothing.
func situationTree(t *testing.T, root string) map[string]string {
	t.Helper()
	tree := map[string]string{}
	if err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err == nil {
			tree[path] = fmt.Sprintf("%d %v", info.Size(), info.ModTime())
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	return tree
}

func TestSystemSituationPrintsThePage(t *testing.T) {
	t.Parallel()
	bed := newWorkBed(t)
	brief := bed.brief("page.md", "Working Mode: implement\n\nBuild the unit.\n\n| Unit | Lines |\n| --- | --- |\n| first | 40 |\n")
	if code, result, _ := bed.work(append([]string{"work", "build", bed.id, "first", "--brief", brief}, workCheck...)...); code != 0 {
		t.Fatalf("build: %+v", result)
	}
	root := bed.root()
	install := situationCheckout(t, bed)
	pagePath := filepath.Join(install, "artifacts", "agents", "steward", "situation.json")
	boardHome := t.TempDir()
	ask, err := board.Publish(boardHome, board.Request{Kind: board.KindAsk, From: board.Sender{Machine: "m1x"}, To: board.Address{Machine: "mac-cli"}, Text: "Is it built?", Deadline: 30 * time.Minute}, situationNow.Add(-5*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	store := launch.Store{Root: filepath.Join(t.TempDir(), "launch")}
	if err := store.Create(launch.Record{ID: "build-1", Kind: "build", Goal: bed.id, State: launch.Running}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Update("build-1", func(r *launch.Record) error {
		r.State, r.Outputs = launch.Completed, []launch.Output{{Path: "report.md", Bytes: 10}}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	projections := 0
	ledger := bed.workOwners().dependencies.endpoint
	owners := bed.workOwners()
	// A verb that ignores the given sources reads the live launch store.
	owners.processes.launches = func() *launch.Manager {
		t.Error("the verb read the live launch manager, not the given sources")
		return &launch.Manager{Store: store}
	}
	clock := situationNow
	owners.situation = situationEmptyHost(t, steward.SituationSources{Now: func() time.Time { return clock },
		Project: func(_ goal.Endpoint, fetch bool, now time.Time) (goal.Projection, error) {
			projections++
			endpoint, err := ledger(root)
			if err != nil {
				return goal.Projection{}, err
			}
			return goal.Project(endpoint, fetch, now)
		},
		HostWork: func() (int, int, error) { return 0, 0, nil }, LaneHome: func() (string, error) { return situationLaneBed(t), nil },
		BoardHome: func() (string, error) { return boardHome, nil }, Launches: (&launch.Manager{Store: store}).EffectiveList,
		Holder: func(string) (steward.SeatHolder, error) { return steward.SeatHolder{}, nil }})
	owners.situation.Registry = func() (string, error) { return situationArmed(t, root), nil }
	owners.situation.Processes = identity.FixedProcessTable{{Pid: 4242, Group: 4242, Session: 4242, Parent: 1}}
	owners.situation.Argv = func(int64) ([]string, bool) { return []string{"go", "test", "./..."}, true }
	owners.situation.Cwd = func(int64) (string, bool) { return root, true }
	owners.situation.Load = func(time.Time) hostload.Sample { return hostload.Sample{Load1m: 3, Available: true} }

	command, _, _ := resolveIntentArgv([]string{"system", "situation"})
	run := func(args ...string) string {
		t.Helper()
		before := situationTree(t, root)
		var stdout, stderr bytes.Buffer
		if code := runIntentIn(command, args, &stdout, &stderr, root, owners); code != 0 {
			t.Fatalf("system situation %v = %d\n%s%s", args, code, stdout.String(), stderr.String())
		}
		if after := situationTree(t, root); !reflect.DeepEqual(before, after) {
			for path, was := range after {
				if before[path] != was && path != pagePath && path != filepath.Dir(pagePath) {
					t.Errorf("system situation %v wrote %s", args, path)
				}
			}
		}
		return stdout.String()
	}

	// The first call reads and saves the shared page; a call within two
	// minutes prints it and reads nothing, and one at three reads again.
	page := run()
	for _, want := range []string{", 0s ago", "First thing", "ask " + ask.Message.ID, "Program", "Host", "Engine", "Alerts", "more; metasystem system situation --verbose"} {
		if !strings.Contains(page, want) {
			t.Fatalf("the page lacks %q:\n%s", want, page)
		}
	}
	if strings.Contains(page, "pid 4242") || projections != 1 {
		t.Fatalf("the default page holds a detail line or read the ledger %d times:\n%s", projections, page)
	}
	clock = situationNow.Add(time.Minute)
	if verbose := run("--verbose"); !strings.Contains(verbose, "pid 4242 go test") || !strings.Contains(verbose, "1. "+bed.id) || !strings.Contains(verbose, ", 1m ago") || projections != 1 {
		t.Fatalf("--verbose lacks the process, the queue or the page's age (ledger read %d times):\n%s", projections, verbose)
	}

	var result struct {
		Data struct {
			Composition steward.Composition
			Situation   struct {
				Launches  []launch.Record
				Processes []steward.HostProcess
				Asks      []struct {
					Thread struct {
						ID   string
						Root struct{ DeadlineAt *time.Time }
					}
				}
			}
		}
	}
	if err := json.Unmarshal([]byte(run("--json")), &result); err != nil || projections != 1 {
		t.Fatalf("--json: %v (ledger read %d times)", err, projections)
	}
	data := result.Data
	if data.Composition.First.Subject != ask.Message.ID || len(data.Situation.Launches) != 1 || len(data.Situation.Launches[0].Outputs) != 1 || data.Situation.Launches[0].Outputs[0].Path != "report.md" {
		t.Fatalf("--json: first %+v, launches %+v, want the ask first and the build's report", data.Composition.First, data.Situation.Launches)
	}
	if len(data.Situation.Processes) != 1 || data.Situation.Processes[0].Pid != 4242 || data.Situation.Processes[0].Kind != "go test" {
		t.Fatalf("--json: processes %+v, want pid 4242, a go test", data.Situation.Processes)
	}
	if asks := data.Situation.Asks; len(asks) != 1 || asks[0].Thread.ID != ask.Message.ID || asks[0].Thread.Root.DeadlineAt == nil || !asks[0].Thread.Root.DeadlineAt.Equal(*ask.Message.DeadlineAt) {
		t.Fatalf("--json: asks %+v, want %s with its deadline %v", asks, ask.Message.ID, ask.Message.DeadlineAt)
	}
	clock = situationNow.Add(3 * time.Minute)
	if stale := run(); !strings.Contains(stale, ", 0s ago") || projections != 2 {
		t.Fatalf("a page three minutes old was kept (ledger read %d times):\n%s", projections, stale)
	}
}

// situationLayoutBed is the status bed's checkout read at the goldens' clock
// with no lane, an empty board and host, and one completed build.
func situationLayoutBed(t *testing.T) layoutBed {
	bed := statusLayoutBed(false)(t)
	for _, args := range [][]string{{"init", "-q"}, {"config", "metasystem.goal.machine", "m1e"}} {
		if out, err := exec.Command("git", append([]string{"-C", bed.cwd}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	store := launch.Store{Root: filepath.Join(t.TempDir(), "launch")}
	if err := store.Create(launch.Record{ID: "build-1", Kind: "build", Goal: "switch-on-trial", State: launch.Running}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Update("build-1", func(r *launch.Record) error {
		r.State, r.Outputs = launch.Completed, []launch.Output{{Path: "report.md", Bytes: 10}}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	bed.owners.situation = situationEmptyHost(t, steward.SituationSources{HostWork: func() (int, int, error) { return 0, 0, nil },
		LaneHome:  func() (string, error) { return "", fmt.Errorf("this host has no home for the lane") },
		BoardHome: func() (string, error) { return home, nil }, Launches: (&launch.Manager{Store: store}).EffectiveList,
		Holder: func(string) (steward.SeatHolder, error) { return steward.SeatHolder{}, nil }})
	return bed
}

// situationWaitBed is a checkout whose page is read over counted sources on
// a virtual wait clock given through work wait's clock seam; after each
// sleep, event runs at the clock's new time.
type situationWaitBed struct {
	t                    *testing.T
	root, install, board string
	owners               intentOwners
	clock                *virtualWaitClock
	projections          int
	event                func(at time.Time)
}

func newSituationWaitBed(t *testing.T) *situationWaitBed {
	t.Helper()
	bed := newWorkBed(t)
	b := &situationWaitBed{t: t, root: bed.root(), install: situationCheckout(t, bed), board: t.TempDir(), clock: newVirtualWaitClock(situationNow)}
	clock := situationNow
	b.owners = countedSituationOwners(t, bed, &clock, &b.projections)
	b.owners.situation.Now = b.clock.Now
	b.owners.situation.BoardHome = func() (string, error) { return b.board, nil }
	b.owners.work.waitClock = func(options *metarun.WaitOptions) {
		b.clock.apply(options)
		options.Sleep = func(ctx context.Context, d time.Duration) error {
			err := b.clock.Sleep(ctx, d)
			if b.event != nil {
				b.event(b.clock.Now())
			}
			return err
		}
	}
	return b
}

func (b *situationWaitBed) run(args ...string) (int, string) {
	b.t.Helper()
	command, _, _ := resolveIntentArgv([]string{"system", "situation"})
	var stdout, stderr bytes.Buffer
	code := runIntentIn(command, args, &stdout, &stderr, b.root, b.owners)
	return code, stdout.String() + stderr.String()
}

func (b *situationWaitBed) receipts() []string {
	b.t.Helper()
	data, err := os.ReadFile(filepath.Join(b.install, "artifacts", "agents", "steward", "situation-reads.jsonl"))
	if err != nil && !os.IsNotExist(err) {
		b.t.Fatal(err)
	}
	return strings.Fields(string(data))
}

func TestSystemSituationWaitIsBounded(t *testing.T) {
	t.Parallel()
	b := newSituationWaitBed(t)
	for _, bad := range []string{"61m", "0", "soon"} {
		code, out := b.run("--wait", bad)
		if _, err := os.Stat(filepath.Join(b.install, "artifacts", "agents", "steward", "situation.json")); code != 2 || !os.IsNotExist(err) || b.projections != 0 ||
			!strings.Contains(out, "--wait takes a duration up to 60m, such as 10m; nothing was done") || !strings.Contains(out, "--wait 60m") {
			t.Fatalf("--wait %s = %d (page %v, ledger read %d times), want refused with code 2 and nothing read:\n%s", bad, code, err, b.projections, out)
		}
	}
	code, out := b.run("--wait", "5m", "--json")
	var result struct{ Data struct{ Woke string } }
	if err := json.Unmarshal([]byte(out), &result); err != nil || code != 0 || result.Data.Woke != "nothing changed in 5m" || !b.clock.Now().Equal(situationNow.Add(5*time.Minute)) {
		t.Fatalf("--wait 5m with nothing changing = %d, woke %q (%v) at %v:\n%s", code, result.Data.Woke, err, b.clock.Now(), out)
	}
	var ask board.Message
	at := situationNow.Add(7 * time.Minute)
	b.event = func(now time.Time) {
		if ask.ID == "" && !now.Before(at) {
			published, err := board.Publish(b.board, board.Request{Kind: board.KindAsk, From: board.Sender{Machine: "m1x"}, To: board.Address{Machine: "mac-cli"}, Text: "Is it built?"}, now)
			if err != nil {
				t.Error(err)
			}
			ask = published.Message
		}
	}
	if code, out := b.run("--wait", "10m"); code != 0 || strings.SplitN(out, "\n", 2)[0] != "woke: changed: ask|"+ask.ID+"|open" || !strings.Contains(out, "First thing") {
		t.Fatalf("a new ask two minutes into the wait = %d, want the page led by the ask:\n%s", code, out)
	}
}

func TestASituationReadByTheMainLeavesAReceipt(t *testing.T) {
	t.Parallel()
	b := newSituationWaitBed(t)
	caller := lease.ClassifyResult{Class: lease.ClassMain, Holder: true, MainId: "main-1"}
	b.owners.dependencies.claimHolder.Caller = func(string, int64) (lease.ClassifyResult, error) { return caller, nil }
	b.owners.dependencies.claimHolder.Holder = func(string) (lease.CurrentHolderView, error) {
		return lease.CurrentHolderView{MainId: "main-1", SessionId: "s-1"}, nil
	}
	for want := 1; want <= 2; want++ {
		if code, out := b.run(); code != 0 || len(b.receipts()) != want {
			t.Fatalf("call %d by the main = %d with receipts %v, want %d:\n%s", want, code, b.receipts(), want, out)
		}
	}
	var line struct {
		Session string
		At      time.Time
		First   struct{ Kind, Subject string }
	}
	if err := json.Unmarshal([]byte(b.receipts()[0]), &line); err != nil || line.Session != "s-1" || !line.At.Equal(situationNow) || line.First.Kind == "" {
		t.Fatalf("the receipt %s (%v), want session s-1 at %v with the first thing", b.receipts()[0], err, situationNow)
	}
	caller = lease.ClassifyResult{Class: lease.ClassDelegate, Pid: 7}
	if code, _ := b.run(); code != 0 || len(b.receipts()) != 2 {
		t.Fatalf("a delegate's call = %d with receipts %v, want none added", code, b.receipts())
	}
	caller = lease.ClassifyResult{Class: lease.ClassMain, Holder: true, MainId: "main-1"}
	folder := filepath.Join(b.install, "artifacts", "agents", "steward")
	if err := os.Rename(folder, folder+"-aside"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(folder, []byte("not a folder"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, out := b.run("--verbose"); code != 0 || !strings.Contains(out, "First thing") || !strings.Contains(out, "the read receipt was not written: ") {
		t.Fatalf("an unwritable receipt = %d, want the page with the error as a detail:\n%s", code, out)
	}
}
