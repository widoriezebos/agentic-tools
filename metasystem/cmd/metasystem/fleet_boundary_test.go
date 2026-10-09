package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testprovider"
)

type fleetBoundaryBed struct {
	*workBed
	owners  intentOwners
	run     launch.UnitRunRecord
	now     time.Time
	session string
}

func newFleetBoundaryBed(t *testing.T, driver string) *fleetBoundaryBed {
	t.Helper()
	bed := newWorkBed(t)
	testprovider.Register(t, bed.root())
	exact, state, err := (identity.KernelProber{}).Probe(int64(os.Getpid()))
	if err != nil || state != identity.Alive {
		t.Fatalf("process identity: %s %v", state, err)
	}
	now := exact.StartedAt.Add(time.Hour)
	bed.manager.Now = func() time.Time { return now }
	if err := os.WriteFile(filepath.Join(bed.root(), "metasystem.conf"), []byte("metasystem.runtimes=fake\nlaunch.build.model.fake=fixture\nlaunch.read.model.fake=fixture\nlaunch.critique.model.fake=fixture\nlaunch.design.model.fake=fixture\nlaunch.seat.window.tokens=200000\nseat.driver="+driver+"\nproof.cheap=true\nproof.audits=true\nproof.deadline=15\n"), 0600); err != nil {
		t.Fatal(err)
	}
	session := "fleet-" + driver
	ref := exact.Ref()
	if _, err := lease.AnnounceWithPair(bed.root(), session, ref.Pid, ref.StartedAtSec, ref.StartTicks, ref.BootID, "fleet-boundary", "fake", "coordinator"); err != nil {
		t.Fatal(err)
	}
	brief := bed.brief("boundary.md", "Build the unit.\n")
	bed.declaredCheap = "go test -timeout 30m -count=1 -run 'TestA|TestB' ./..."
	checks := []string{"--read-tool-calls", "12"}
	code, result, _ := bed.work(append([]string{"work", "build", bed.id, "finished", "--brief", brief, "--lines", "5"}, checks...)...)
	if code != 0 {
		t.Fatalf("public build: %d %+v", code, result)
	}
	id := resultData(t, result)["run"].(string)
	runner := &launch.UnitRunner{Manager: bed.manager, Git: workGit{bed}, Root: bed.unitRoot}
	// The production subject writer retains the collected read's subject;
	// the branch adapter below observes its published attestation.
	if err := runner.ReviewSubject(id, func(review launch.UnitReview, retain func(launch.UnitSubject) error) error {
		return retain(launch.UnitSubject{Round: review.Round.Number, Commit: strings.Repeat("a", 40), ResultDigest: launch.UnitResultDigest(review.Result), DiffDigest: review.DiffDigest})
	}); err != nil {
		t.Fatal(err)
	}
	run, err := runner.Status(id)
	if err != nil {
		t.Fatal(err)
	}
	owners := bed.workOwners()
	owners.work.inspectRead = func(string, string, string) (branch.BranchReadResult, error) {
		return branch.BranchReadResult{State: "collected", Published: true, AttestationCommit: strings.Repeat("b", 40)}, nil
	}
	return &fleetBoundaryBed{bed, owners, run, now, session}
}

func (bed *fleetBoundaryBed) observe(t *testing.T) (int, string) {
	t.Helper()
	var problem bytes.Buffer
	code := runStewardRunWithDependencies([]string{"--repo", bed.root()}, io.Discard, &problem,
		func(root string, _ steward.WorkerCensus, _ func() error, _ time.Duration, cfg steward.TickConfig) error {
			cfg.WorkStateRoot = filepath.Dir(bed.unitRoot)
			wireStewardSeat(&cfg, bed.owners)
			return cfg.CompletedBoundary(root, bed.now)
		}, nil, nil, func(string) int { return 1 }, func() (bool, error) { return false, nil })
	return code, problem.String()
}

func (bed *fleetBoundaryBed) saveRun(t *testing.T, run launch.UnitRunRecord) {
	t.Helper()
	data, err := json.Marshal(run)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(bed.unitRoot, run.ID, "run.json")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
}

func (bed *fleetBoundaryBed) boundaries(t *testing.T) []steward.UnitBoundary {
	t.Helper()
	events, err := steward.ReadUnitBoundaries(bed.root())
	if err != nil {
		t.Fatal(err)
	}
	return events
}

func TestFleetBoundaryPublicCompletedUnit(t *testing.T) {
	t.Parallel()
	for _, driver := range []string{"auto", "person"} {
		t.Run(driver, func(t *testing.T) {
			t.Parallel()
			bed := newFleetBoundaryBed(t, driver)
			superseded := bed.run
			superseded.ID = "superseded"
			bed.saveRun(t, superseded)
			for range 2 {
				if code, problem := bed.observe(t); code != 0 {
					t.Fatalf("public steward: %d %s", code, problem)
				}
				events := bed.boundaries(t)
				if len(events) != 1 || events[0].Session != bed.session || events[0].Goal != bed.id || events[0].Unit != bed.run.ID+"/1" || events[0].Outcome != "green" || events[0].Seat != bed.root() {
					t.Fatalf("completed boundary: %+v", events)
				}
			}
			if intents, err := steward.LiveIntents(bed.root()); err != nil || len(intents) != 0 {
				t.Fatalf("boundary first part created a handoff: %+v %v", intents, err)
			}
			var output, problem bytes.Buffer
			code := dispatchOn([]string{"session", "handoff", "--status", "--root", bed.root(), "--json"}, &output, &problem)
			var status contextStatusOutput
			if code != 0 || json.Unmarshal(output.Bytes(), &status) != nil || len(status.Boundaries) != 1 {
				t.Fatalf("public status: %d %s %s", code, &output, &problem)
			}
			if _, state, err := (identity.KernelProber{}).Probe(int64(os.Getpid())); state != identity.Alive || err != nil {
				t.Fatalf("observer ended the session: %s %v", state, err)
			}
		})
	}
}

func TestFleetBoundaryPublicReadWaivedGoal(t *testing.T) {
	t.Parallel()
	for _, waiver := range []string{"tier-one", "zero-review-rounds"} {
		t.Run(waiver, func(t *testing.T) {
			t.Parallel()
			bed := newFleetBoundaryBed(t, "auto")
			waived := bed.goalFile(bed.id)
			waived.Id = "read-waived"
			if waiver == "tier-one" {
				waived.Tier, waived.Risk.Severity, waived.Risk.Novelty = 1, 1, 1
			} else {
				waived.Budget.ReviewRoundLimit = 0
			}
			waived.Approved.Digest = goal.ApprovalDigest(waived.Intent, waived.Tier, *waived.Budget, waived.Risk)
			bed.addGoal(waived)
			worktree := t.TempDir()
			commit, base := strings.Repeat("c", 40), strings.Repeat("d", 40)
			git := bed.owners.work.git
			bed.owners.work.git = func(dir string, args ...string) ([]byte, error) {
				joined := strings.Join(args, " ")
				if joined == "worktree list --porcelain" {
					out, err := git(dir, args...)
					return append(out, []byte("\nworktree "+worktree+"\nHEAD "+commit+"\nbranch refs/heads/goal/"+waived.Id+"\n")...), err
				}
				switch joined {
				case "rev-parse --verify HEAD^{commit}":
					if dir == worktree {
						return []byte(commit + "\n"), nil
					}
				case "merge-base " + base + " " + commit:
					return []byte(base + "\n"), nil
				case "rev-list --first-parent --reverse --parents " + base + ".." + commit, "rev-list --parents -n 1 " + commit:
					return []byte(commit + " " + base + "\n"), nil
				case "show -s --format=%(trailers:only,unfold=true) " + commit:
					return []byte("Goal-Unit: " + waived.Id + "/hand\n"), nil
				case "diff-tree -r -z --no-renames --full-index " + commit + "^ " + commit:
					return []byte(":000000 100644 " + strings.Repeat("0", 40) + " " + strings.Repeat("e", 40) + " A\x00hand.go\x00"), nil
				}
				return git(dir, args...)
			}
			bed.owners.connection.endpointTip = func(string, goal.Endpoint) (string, error) { return base, nil }
			inspect := bed.owners.work.inspectRead
			bed.owners.work.inspectRead = func(root, id, subject string) (branch.BranchReadResult, error) {
				if id == waived.Id {
					return branch.BranchReadResult{}, nil
				}
				return inspect(root, id, subject)
			}
			if code, status := bed.runJSON(bed.owners, "status", waived.Id, "--work", "hand"); code != 0 || !strings.Contains(status.Summary, "committed, ready to land without a read") {
				t.Fatalf("read-waived hand commit: %d %+v", code, status)
			}
			if code, problem := bed.observe(t); code != 0 {
				t.Fatalf("public steward: %d %s", code, problem)
			}
			if events := bed.boundaries(t); len(events) != 1 || events[0].Goal != bed.id || events[0].Unit != bed.run.ID+"/1" {
				t.Fatalf("read-waived goal hid the other goal's completed boundary: %+v", events)
			}
			// Retained preparation must stay visible without changing the stages
			// used to observe a later completed unit.
			events := bed.boundaries(t)
			act := &steward.BoundaryAct{Tip: commit, Summary: "prepared next work", Command: []string{"metasystem", "work", "land", waived.Id}}
			events[0].Next = act
			events = append(events, steward.UnitBoundary{Seat: bed.root(), Session: bed.session, Goal: waived.Id, Unit: "hand/1", Outcome: "green", Next: act})
			data, err := json.Marshal(events)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(bed.root(), "artifacts", "agents", "steward", "unit-boundaries.json"), data, 0600); err != nil {
				t.Fatal(err)
			}
			if code, status := bed.runJSON(bed.owners, "status", waived.Id, "--work", "hand"); code != 0 || !strings.Contains(status.Summary, "committed, ready to land without a read") {
				t.Fatalf("prepared read-waived stage: %d %+v", code, status)
			}
			var cfg steward.TickConfig
			wireStewardSeat(&cfg, bed.owners)
			for id, want := range map[string]string{
				waived.Id: "committed, ready to land without a read",
				bed.id:    "reviewed; its read is collected and published (attestation bbbbbbbbbbbb)",
			} {
				units, err := cfg.Units(bed.root(), id)
				if err != nil || len(units) != 1 || units[0].Stage != want {
					t.Fatalf("prepared unit stages for %s: %+v %v", id, units, err)
				}
			}
			code, status := bed.runJSON(bed.owners, "work", "status", bed.id, "--work", "finished")
			if code != 0 {
				t.Fatalf("prepared public status: %d %+v", code, status)
			}
			view := resultData(t, status)["work"].([]any)[0].(map[string]any)
			wantStage := "reviewed; its read is collected and published (attestation bbbbbbbbbbbb)"
			if view["stage"] != wantStage || !strings.HasSuffix(status.Summary, wantStage) || view["boundaryAct"] == nil {
				t.Fatalf("preparation changed the unit stage or disappeared: %+v", status)
			}
			round := bed.run.Rounds[0]
			round.Number = 2
			bed.run.Rounds = append(bed.run.Rounds, round)
			subject := bed.run.Subjects[0]
			subject.Round = 2
			bed.run.Subjects = append(bed.run.Subjects, subject)
			bed.saveRun(t, bed.run)
			if code, problem := bed.observe(t); code != 0 {
				t.Fatalf("next public boundary: %d %s", code, problem)
			}
			if events := bed.boundaries(t); len(events) != 3 || events[2].Goal != bed.id || events[2].Unit != bed.run.ID+"/2" {
				t.Fatalf("prepared read-waived goal hid the next boundary: %+v", events)
			}
		})
	}
}

func TestFleetBoundaryPublicHoldsOpenWork(t *testing.T) {
	t.Parallel()
	for _, scenario := range []string{"running", "starting", "capacity-held", "proof-pending", "read-pending", "revision", "undecided-read", "ask", "old-completion", "unknown-completion"} {
		t.Run(scenario, func(t *testing.T) {
			t.Parallel()
			bed := newFleetBoundaryBed(t, "auto")
			run := bed.run
			round := &run.Rounds[len(run.Rounds)-1]
			switch scenario {
			case "running", "starting", "capacity-held":
				// An older completed record remains beside the current unfinished work.
				run.ID, run.Unit, run.State = "current", "current", "running"
				state := launch.StepRunning
				if scenario != "running" {
					state = launch.StepStarting
				}
				round.Steps = []launch.UnitStep{{Name: "build", State: state, LaunchID: "not-started"}}
			case "proof-pending", "read-pending":
				name := "proof:check"
				if scenario == "read-pending" {
					name = "read"
				}
				round.Steps = append(round.Steps, launch.UnitStep{Name: name, State: launch.StepPending})
			case "revision":
				round.Outcome = "proof-red"
			case "undecided-read":
				bed.owners.work.inspectRead = func(string, string, string) (branch.BranchReadResult, error) {
					return branch.BranchReadResult{State: "examining"}, nil
				}
			case "ask":
				path := filepath.Join(bed.root(), "artifacts", "agents", "channel", "questions", "question.json")
				data, err := json.Marshal(map[string]any{"id": "question", "goal": bed.id, "openedAt": bed.now, "state": "open"})
				if err != nil {
					t.Fatal(err)
				}
				if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, data, 0600); err != nil {
					t.Fatal(err)
				}
			case "old-completion":
				for i := range round.Steps {
					round.Steps[i].FinishedAt = "2000-01-01T00:00:00Z"
				}
			case "unknown-completion":
				round.Steps[len(round.Steps)-1].FinishedAt = "unknown"
			}
			bed.saveRun(t, run)
			code, problem := bed.observe(t)
			if scenario == "unknown-completion" {
				if code == 0 || !strings.Contains(problem, "completion time is unknown") {
					t.Fatalf("unknown completion: %d %s", code, problem)
				}
			} else if code != 0 {
				t.Fatalf("unfinished boundary: %d %s", code, problem)
			}
			if events := bed.boundaries(t); len(events) != 0 {
				t.Fatalf("unfinished work emitted: %+v", events)
			}
		})
	}
}

func TestFleetBoundaryPublicUnreadableState(t *testing.T) {
	t.Parallel()
	bed := newFleetBoundaryBed(t, "auto")
	path := filepath.Join(bed.root(), "artifacts", "agents", "steward", "unit-boundaries.json")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("broken"), 0600); err != nil {
		t.Fatal(err)
	}
	if code, problem := bed.observe(t); code == 0 || !strings.Contains(problem, "invalid character") {
		t.Fatalf("unreadable state: %d %s", code, problem)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "broken" {
		t.Fatalf("unreadable history was replaced: %q %v", data, err)
	}
}

func TestFleetBoundaryPublicLoopRevivesWithoutSession(t *testing.T) {
	t.Parallel()
	if os.Getenv("FLEET_BOUNDARY_LOOP_CHILD") == "" {
		tools := t.TempDir()
		gitStub := filepath.Join(tools, "git")
		if err := testexec.WriteFile(gitStub, []byte("#!/bin/sh\nexit 1\n"), 0700); err != nil {
			t.Fatal(err)
		}
		command := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestFleetBoundaryPublicLoopRevivesWithoutSession$", "-test.count=1", "-test.timeout=30m")
		for _, entry := range os.Environ() {
			name, _, _ := strings.Cut(entry, "=")
			if name != "METASYSTEM_SUPERVISION_REGISTRY_HOME" && name != "METASYSTEM_TESTENV_REGISTRY_NONCE" && name != "PATH" {
				command.Env = append(command.Env, entry)
			}
		}
		command.Env = append(command.Env, "FLEET_BOUNDARY_LOOP_CHILD="+gitStub, "PATH="+tools+string(os.PathListSeparator)+os.Getenv("PATH"))
		command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		testenv.ReapFixtureProcessGroups(t, []testenv.FixtureProcessGroup{{Verb: "isolated fleet boundary loop", Resolve: func() (int, bool, error) {
			if command.Process == nil {
				return 0, false, nil
			}
			return command.Process.Pid, true, nil
		}}})
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("isolated real loop: %v\n%s", err, output)
		}
		return
	}
	if path, err := exec.LookPath("git"); err != nil || path != os.Getenv("FLEET_BOUNDARY_LOOP_CHILD") {
		t.Fatalf("real Git must be unavailable: path=%s error=%v", path, err)
	}
	for _, scenario := range []string{"no-lease", "no-announcement"} {
		t.Run(scenario, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			if err := os.MkdirAll(filepath.Join(root, ".git"), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.runtimes=fake\n"), 0600); err != nil {
				t.Fatal(err)
			}
			if scenario == "no-announcement" {
				path := filepath.Join(root, "artifacts", "agents", "mains", "worktree-lease.json")
				if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(`{"holderMainId":"legacy","pid":41,"revision":1,"claimEpoch":1}`), 0600); err != nil {
					t.Fatal(err)
				}
			} else if _, err := lease.CurrentHolder(root); !errors.Is(err, lease.ErrLeaseAbsent) {
				t.Fatalf("fixture must have no lease: %v", err)
			}
			if err := steward.MintIntent(root, steward.Intent{Nonce: "prepared", Goal: "held", Runtime: "fake"}); err != nil {
				t.Fatal(err)
			}
			now := time.Date(2026, 10, 8, 8, 0, 0, 0, time.UTC)
			stop := func() {
				if err := os.WriteFile(filepath.Join(root, "artifacts", "agents", "steward", "stop"), nil, 0600); err != nil {
					t.Fatal(err)
				}
			}
			clock := &steward.HandoffClock{Now: func() time.Time { return now }, Sleep: func(d time.Duration) { now = now.Add(d); stop() }}
			revives, observations := 0, 0
			registered := families()
			for i := range registered {
				if registered[i].name != "steward" {
					continue
				}
				for j := range registered[i].verbs {
					if registered[i].verbs[j].name == "run" {
						registered[i].verbs[j].run = func(args []string, stdout, stderr io.Writer) int {
							return runStewardRunWithDependencies(args, stdout, stderr,
								func(root string, census steward.WorkerCensus, _ func() error, interval time.Duration, cfg steward.TickConfig) error {
									cfg.Now = now
									observer := cfg.CompletedBoundary
									cfg.CompletedBoundary = func(root string, at time.Time) error {
										observations++
										err := observer(root, at)
										if err != nil {
											t.Errorf("missing session must mean no event: %v", err)
										}
										return err
									}
									return steward.RunLoop(root, census, func() error { revives++; stop(); return nil }, interval, cfg)
								}, probeStewardRuntime, clock, func(string) int { return 1 }, func() (bool, error) { return false, nil })
						}
					}
				}
			}
			var output, problem bytes.Buffer
			code := dispatchWithFamiliesAndRepositoryTop([]string{"steward", "run", "--repo", root}, &output, &problem, registered, func(string) (string, error) { t.Fatal("runner called Git"); return "", nil })
			if code != 0 || revives != 1 || observations != 1 {
				t.Fatalf("real observer/loop: exit=%d revivals=%d observations=%d stderr=%s", code, revives, observations, &problem)
			}
			if events, err := steward.ReadUnitBoundaries(root); err != nil || len(events) != 0 {
				t.Fatalf("sessionless loop emitted an event: %+v %v", events, err)
			}
		})
	}
}

func TestFleetBoundaryPublicStatusRetainsReading(t *testing.T) {
	t.Parallel()
	for _, scenario := range []string{"incomplete", "foreign", "malformed"} {
		t.Run(scenario, func(t *testing.T) {
			t.Parallel()
			bed := newFleetBoundaryBed(t, "auto")
			if code, problem := bed.observe(t); code != 0 {
				t.Fatalf("seed boundary: %d %s", code, problem)
			}
			events := bed.boundaries(t)
			other := events[0]
			if scenario == "foreign" {
				other.Seat = filepath.Join(t.TempDir(), "other-seat")
			} else {
				other.Unit = ""
			}
			events = append([]steward.UnitBoundary{other}, events...)
			data, err := json.Marshal(events)
			if err != nil {
				t.Fatal(err)
			}
			if scenario == "malformed" {
				data = []byte("broken")
			}
			path := filepath.Join(bed.root(), "artifacts", "agents", "steward", "unit-boundaries.json")
			if err := os.WriteFile(path, data, 0600); err != nil {
				t.Fatal(err)
			}
			alias := filepath.Join(t.TempDir(), "seat")
			if err := os.Symlink(bed.root(), alias); err != nil {
				t.Fatal(err)
			}
			for _, format := range []string{"json", "text"} {
				var output, problem bytes.Buffer
				args := []string{"session", "handoff", "--status", "--root", alias}
				if format == "json" {
					args = append(args, "--json")
				}
				code := dispatchOn(args, &output, &problem)
				if code != 0 {
					t.Fatalf("%s %s status hid reading: exit=%d stdout=%s stderr=%s", scenario, format, code, &output, &problem)
				}
				if format == "json" {
					var status contextStatusOutput
					if err := json.Unmarshal(output.Bytes(), &status); err != nil || status.Reading.Capability == "" {
						t.Fatalf("missing current reading: %s %v", &output, err)
					}
					want := 1
					if scenario == "malformed" {
						want = 0
					}
					if len(status.Boundaries) != want || (scenario != "foreign" && !strings.Contains(output.String(), `"unitBoundaryError"`)) {
						t.Fatalf("boundary diagnostic or valid entry lost: %s", &output)
					}
				} else if !strings.Contains(output.String(), "The context budget is") || (scenario != "foreign" && !strings.Contains(output.String(), "Unit boundary history:")) {
					t.Fatalf("reading or boundary diagnostic lost: %s", &output)
				}
			}
			if after, err := os.ReadFile(path); err != nil || !bytes.Equal(after, data) {
				t.Fatalf("status rewrote history: %s %v", after, err)
			}
			if scenario == "foreign" {
				foreignOnly, err := json.Marshal([]steward.UnitBoundary{other})
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, foreignOnly, 0600); err != nil {
					t.Fatal(err)
				}
				if code, problem := bed.observe(t); code != 0 {
					t.Fatalf("foreign history prevented the next boundary: %d %s", code, problem)
				}
				stored, err := os.ReadFile(path)
				var retained []steward.UnitBoundary
				if err != nil || json.Unmarshal(stored, &retained) != nil || len(retained) != 2 || retained[0] != other || retained[1].Session != bed.session {
					t.Fatalf("next boundary discarded history: %s %v", stored, err)
				}
			}
		})
	}
}
