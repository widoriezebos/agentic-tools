package main

import (
	"bytes"
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
	"github.com/widoriezebos/agentic-tools/metasystem/internal/registry"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
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
// outside Git's own directory: the units reader reads the remote's main
// through a disposable ref, which Git records as its fetch.
func situationTree(t *testing.T, root string) map[string]string {
	t.Helper()
	tree := map[string]string{}
	if err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err == nil && info.IsDir() && info.Name() == ".git" {
			return filepath.SkipDir
		}
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
	for _, args := range [][]string{{"init", "-q"}, {"config", "metasystem.goal.machine", "mac-cli"}, {"add", "plans/goals/backlog.md"},
		{"-c", "user.name=fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "ledger"}, {"update-ref", goal.AcceptedRef, "HEAD"}} {
		if out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	layout, err := bed.workOwners().resolver.ResolveLayout(root)
	if err != nil {
		t.Fatal(err)
	}
	engine := filepath.Join(layout.InstallationRoot.Path(), "bin", "metasystem")
	if err := os.MkdirAll(filepath.Dir(engine), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := testexec.WriteFile(engine, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
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
	owners.situation = situationEmptyHost(t, steward.SituationSources{Now: func() time.Time { return situationNow },
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
				if before[path] != was {
					t.Errorf("system situation %v wrote %s", args, path)
				}
			}
		}
		return stdout.String()
	}

	page := run()
	for _, want := range []string{"First thing", "ask " + ask.Message.ID, "Program", "Host", "Engine", "Alerts", "more; metasystem system situation --verbose"} {
		if !strings.Contains(page, want) {
			t.Fatalf("the page lacks %q:\n%s", want, page)
		}
	}
	if strings.Contains(page, "pid 4242") || projections != 1 {
		t.Fatalf("the default page holds a detail line or read the ledger %d times:\n%s", projections, page)
	}
	if verbose := run("--verbose"); !strings.Contains(verbose, "pid 4242 go test") || !strings.Contains(verbose, "1. "+bed.id) || projections != 2 {
		t.Fatalf("--verbose lacks the process or the queue (ledger read %d times):\n%s", projections, verbose)
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
	if err := json.Unmarshal([]byte(run("--json")), &result); err != nil || projections != 3 {
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
