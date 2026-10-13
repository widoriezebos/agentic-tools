package plain

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/conflict"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/enginebuild"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lock"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"
)

type deployBed struct {
	t                                    *testing.T
	home, install, checkout, merge, head string
	record                               lane.Record
	seams                                ProveSeams
	opts                                 DeployOptions
	calls                                []string
	builds, restarts, waits              int
	alive                                bool
}

func newDeployBed(t *testing.T) *deployBed {
	t.Helper()
	root := t.TempDir()
	b := &deployBed{t: t, home: filepath.Join(root, "home"), checkout: filepath.Join(root, "lane"), head: "old"}
	b.install = filepath.Join(b.checkout, "metasystem")
	b.record = lane.Record{Root: b.checkout, Install: b.install, RegisteredBy: "Wido", CustodyEpoch: 1, At: bedNow.Format(time.RFC3339)}
	writeDeployJSON(t, filepath.Join(lane.HostDir(b.home), "landing-lane.json"), b.record)
	b.stamp("old")
	if err := os.MkdirAll(filepath.Join(Dir(b.install), "proofs"), 0755); err != nil {
		t.Fatal(err)
	}
	b.seams = ProveSeams{Now: func() time.Time { return bedNow }, Alive: func(Running) bool { return b.alive },
		Git: func(dir string, args ...string) (string, error) {
			if dir != b.checkout && dir != b.install {
				t.Fatalf("Git ran outside lane checkout: %s", dir)
			}
			call := strings.Join(args, " ")
			b.calls = append(b.calls, call)
			switch call {
			case "rev-parse --verify --quiet MERGE_HEAD^{commit}":
				if b.merge == "" {
					return "", exec.Command("false").Run()
				}
				return b.merge, nil
			case "rev-parse --verify HEAD^{commit}":
				return b.head, nil
			case "rev-parse --verify origin/main^{commit}":
				return "main", nil
			case "rev-parse --show-prefix":
				return "metasystem/", nil
			case "status --porcelain=v1 -z --no-renames --untracked-files=all":
				return "", nil
			case "fetch origin main":
				return "", nil
			case "diff --name-only --no-renames old main":
				return "", nil
			case "merge --abort":
				b.merge = ""
				return "", nil
			case "checkout --detach main":
				b.head = "main"
				return "", nil
			default:
				t.Fatalf("unexpected Git call: %s", call)
				return "", nil
			}
		},
		Command: func(command *exec.Cmd) error {
			if command.Dir != b.install || len(command.Args) != 6 || !slices.Equal(command.Args[:5], []string{"go", "run", "./cmd/devgate", "build", "--out"}) {
				t.Fatalf("built wrong install: %+v", command)
			}
			b.calls = append(b.calls, "build")
			b.builds++
			stamp, err := installedEngineStamp(b.install)
			if err != nil || stamp != "old" && stamp != "" {
				t.Fatalf("build replaced installed engine early: %s %v", stamp, err)
			}
			return os.WriteFile(command.Args[5], []byte(enginebuild.StampRecord("main")), 0600)
		},
		WaitPoll: func() { b.waits++; b.alive = false },
		EndProof: func(Running) error { t.Fatal("deploy killed the proof"); return nil },
		Launch:   func([]string, string, string) (int64, error) { t.Fatal("deploy started the lane"); return 0, nil },
	}
	b.opts = DeployOptions{RestartSupervisor: func(install string) error {
		if install != b.install {
			t.Fatalf("restarted wrong installation: %s", install)
		}
		b.restarts++
		return nil
	}, MergeSettled: func(*Fix) (bool, error) { return true, nil }}
	return b
}

func writeDeployJSON(t *testing.T, path string, value any) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
}

func (b *deployBed) stamp(stamp string) {
	b.t.Helper()
	path := filepath.Join(b.install, "bin", "metasystem")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		b.t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(enginebuild.StampRecord(stamp)), 0600); err != nil {
		b.t.Fatal(err)
	}
}

func (b *deployBed) deploy() (DeployRun, error) {
	return DeployEngine(b.home, ActProvenance{Kind: "deploy", Person: "Wido", Root: b.checkout, Destination: b.record, CheckedAt: bedNow}, bedNow, b.opts, b.seams)
}

func TestDeployMerge(t *testing.T) {
	t.Parallel()
	for _, state := range []string{"abandoned", "rebuild", "resolving", "wrong-checkpoint", "wrong-tip", "writers-running", "writers-unreadable"} {
		t.Run(state, func(t *testing.T) {
			t.Parallel()
			b := newDeployBed(t)
			b.merge = "goal-tip"
			fix := &Fix{Attempt: "fix", Goal: "G", Commit: b.head, Tip: b.merge, State: "abandoned", Units: []string{"merge"}, Paths: []conflict.Path{{Path: "file.go"}}}
			switch state {
			case "resolving":
				fix.State = "resolving"
			case "rebuild":
				fix.State = "resolving"
				original := b.seams.Git
				b.seams.Git = func(dir string, args ...string) (string, error) {
					if args[0] == "diff" {
						return "metasystem/file.go", nil
					}
					return original(dir, args...)
				}
			case "wrong-checkpoint":
				fix.Commit = "older"
			case "wrong-tip":
				fix.Tip = "older"
			case "writers-running":
				b.opts.MergeSettled = func(*Fix) (bool, error) { return false, nil }
			case "writers-unreadable":
				b.opts.MergeSettled = func(*Fix) (bool, error) { return false, os.ErrPermission }
			}
			if err := WriteFix(b.install, fix); err != nil {
				t.Fatal(err)
			}
			run, err := b.deploy()
			if state == "abandoned" || state == "rebuild" {
				if err != nil || b.builds != 1 || b.restarts != 1 || b.merge != "" || slices.Index(b.calls, "merge --abort") > slices.Index(b.calls, "build") {
					t.Fatalf("deploy: %+v err=%v calls=%v", run, err, b.calls)
				}
			} else {
				if err == nil || b.builds != 0 || b.restarts != 0 || b.merge != "goal-tip" || slices.Contains(b.calls, "checkout --detach main") {
					t.Fatalf("unsafe deploy: %+v %v %v", run, err, b.calls)
				}
				var refusal *Refusal
				if state == "resolving" && (!strings.Contains(err.Error(), "the lane is resolving G; give it back or wait") || !errors.As(err, &refusal) || !strings.Contains(refusal.Next, "metasystem landing give-back G")) {
					t.Fatalf("missing remedy: %v", err)
				}
			}
		})
	}
}

func TestDeployPreservesLaneRunState(t *testing.T) {
	t.Parallel()
	for _, state := range []string{"paused", "stopped", "running"} {
		t.Run(state, func(t *testing.T) {
			t.Parallel()
			b := newDeployBed(t)
			if state != "running" {
				if _, err := lane.SetPause(b.home, "Wido", bedNow); err != nil {
					t.Fatal(err)
				}
			}
			keeper := lane.AgentState{Launch: "keeper", StartedAt: bedNow.Format(time.RFC3339)}
			if state == "stopped" {
				keeper.ReapedAt = bedNow.Add(time.Minute).Format(time.RFC3339)
			}
			writeDeployJSON(t, lane.AgentStatePath(b.home), keeper)
			before, err := os.ReadFile(lane.AgentStatePath(b.home))
			if err != nil {
				t.Fatal(err)
			}
			pause, paused := lane.ReadPause(b.home)
			if _, err := b.deploy(); err != nil {
				t.Fatal(err)
			}
			after, err := os.ReadFile(lane.AgentStatePath(b.home))
			if err != nil {
				t.Fatal(err)
			}
			got, isPaused := lane.ReadPause(b.home)
			if string(before) != string(after) || !reflect.DeepEqual(pause, got) || paused != isPaused {
				t.Fatalf("deploy changed lane state: %s %s %+v %+v", before, after, pause, got)
			}
		})
	}
}

func TestDeployRunningProof(t *testing.T) {
	t.Parallel()
	for _, now := range []bool{false, true} {
		t.Run(map[bool]string{false: "wait", true: "now"}[now], func(t *testing.T) {
			t.Parallel()
			b := newDeployBed(t)
			b.alive, b.opts.Now = true, now
			running := Running{Attempt: "proof", BatchID: "batch", Tree: "tree", Since: bedNow.Format(time.RFC3339), ClassificationOf: "recorded-red", Person: &ActProvenance{Person: "Wido", Kind: "proof"}, BatchMembers: []GoalSHA{{Goal: "G", SHA: "tip"}}, Executions: []ExecutionAdmission{{Attempt: "execution", Command: "frozen check"}}}
			if err := writeRunning(b.install, running); err != nil {
				t.Fatal(err)
			}
			if err := writeBatch(b.install, &Batch{ID: "batch", Base: "old", Lane: b.record, State: BatchRunning}); err != nil {
				t.Fatal(err)
			}
			run, err := b.deploy()
			if err != nil || now && b.waits != 0 || !now && b.waits != 1 {
				t.Fatalf("proof wait: %+v %v waits=%d", run, err, b.waits)
			}
			batch, err := ReadBatch(b.install)
			if err != nil {
				t.Fatal(err)
			}
			if now {
				requests, err := readLines[DeployRun](filepath.Join(Dir(b.install), "deploys.jsonl"))
				if err != nil {
					t.Fatal(err)
				}
				if run.RestartAttempt != "proof" || len(requests) != 2 || requests[0].RestartAttempt != "proof" || !b.alive {
					t.Fatalf("restart not recorded, or proof killed: %+v %+v", run, batch)
				}
				ended := 0
				b.seams.EndProof = func(r Running) error {
					if r.Attempt != "proof" {
						t.Fatal("wrong proof ended")
					}
					ended++
					b.alive = false
					return nil
				}
				if err := RestartDeployedProof(b.install, b.checkout, b.seams); err != nil {
					t.Fatal(err)
				}
				batch, err = ReadBatch(b.install)
				if err != nil {
					t.Fatal(err)
				}
				if ended != 1 || batch.State != BatchPrepared || !batch.Open || len(batch.Restarts) != 1 {
					t.Fatalf("keeper did not restart: %+v ended=%d", batch, ended)
				}
				results, err := Results(b.install)
				if err != nil {
					t.Fatal(err)
				}
				if len(results) != 1 || results[0].ClassificationOf != running.ClassificationOf || results[0].BatchID != running.BatchID || !reflect.DeepEqual(results[0].Person, running.Person) || !reflect.DeepEqual(results[0].Executions, running.Executions) || !reflect.DeepEqual(results[0].BatchMembers, running.BatchMembers) {
					t.Fatalf("restart erased admitted history: %+v", results)
				}
				if err := RestartDeployedProof(b.install, b.checkout, b.seams); err != nil || ended != 1 {
					t.Fatalf("keeper repeated restart: %v ended=%d", err, ended)
				}
			} else if len(batch.Actions) != 0 || run.RestartAttempt != "" {
				t.Fatalf("finished proof restarted: %+v %+v", run, batch)
			}
		})
	}
}

func TestDeployStampedEngineAndRepeat(t *testing.T) {
	t.Parallel()
	b := newDeployBed(t)
	path := filepath.Join(b.install, "bin", "metasystem")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	command := testenv.Go("build", "-buildvcs=false", "-ldflags", enginebuild.StampLinkerFlags("old"), "-o", path, "../../enginebuild/testdata/stampprobe")
	if out, err := command.CombinedOutput(); err != nil {
		t.Fatalf("stamp fixture: %v %s", err, out)
	}
	build := b.seams.Command
	b.seams.Command = func(command *exec.Cmd) error {
		staged := command.Args[5]
		if _, err := os.Stat(staged); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("build output already exists: %v", err)
		}
		if err := build(command); err != nil {
			return err
		}
		if err := os.Remove(staged); err != nil {
			t.Fatal(err)
		}
		fixture := testenv.Go("build", "-buildvcs=false", "-ldflags", enginebuild.StampLinkerFlags("main"), "-o", staged, "../../enginebuild/testdata/stampprobe")
		out, err := fixture.CombinedOutput()
		if err != nil {
			t.Fatalf("staged stamp fixture: %v %s", err, out)
		}
		return nil
	}
	run, err := b.deploy()
	if err != nil || run.Before != "old" || run.After != "main" {
		t.Fatalf("identity: %+v %v", run, err)
	}
	if out, err := exec.Command(path).CombinedOutput(); err != nil || strings.TrimSpace(string(out)) != "main" {
		t.Fatalf("activated fixture binary: %v %s", err, out)
	}
	if leftovers, err := filepath.Glob(filepath.Join(b.install, "bin", ".metasystem.deploy-*")); err != nil || len(leftovers) != 0 {
		t.Fatalf("staging remains: %v %v", leftovers, err)
	}
	before, err := os.ReadFile(filepath.Join(Dir(b.install), "deploys.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	run, err = b.deploy()
	after, readErr := os.ReadFile(filepath.Join(Dir(b.install), "deploys.jsonl"))
	if err != nil || readErr != nil || !run.Unchanged || !slices.Contains(run.Steps, "nothing to deploy") || b.builds != 1 || b.restarts != 1 || string(before) != string(after) {
		t.Fatalf("repeat: %+v %v calls=%v", run, err, b.calls)
	}
}

func TestDeployFailuresStopBeforeRestart(t *testing.T) {
	t.Parallel()
	for _, failure := range []string{"fetch", "checkout", "build", "wrong-install", "restart"} {
		t.Run(failure, func(t *testing.T) {
			t.Parallel()
			b := newDeployBed(t)
			original := b.seams.Git
			b.seams.Git = func(dir string, args ...string) (string, error) {
				if args[0] == failure {
					return "", os.ErrPermission
				}
				return original(dir, args...)
			}
			if failure == "build" {
				b.seams.Command = func(*exec.Cmd) error { return os.ErrPermission }
			}
			if failure == "wrong-install" {
				b.seams.Command = func(*exec.Cmd) error { return nil }
			}
			if failure == "restart" {
				b.opts.RestartSupervisor = func(string) error { return os.ErrPermission }
			}
			_, err := b.deploy()
			if err == nil || b.restarts != 0 {
				t.Fatalf("failure ignored: %v restarts=%d", err, b.restarts)
			}
			if failure != "wrong-install" && !errors.Is(err, os.ErrPermission) {
				t.Fatalf("failure lost: %v", err)
			}
		})
	}
}

func TestDeployCompletesAFailedSupervisorRestartWithoutBuildingAgain(t *testing.T) {
	t.Parallel()
	b := newDeployBed(t)
	b.opts.RestartSupervisor = func(string) error { return os.ErrPermission }
	if _, err := b.deploy(); !errors.Is(err, os.ErrPermission) {
		t.Fatalf("restart failure: %v", err)
	}
	b.opts.RestartSupervisor = func(string) error { b.restarts++; return nil }
	b.merge = "goal-tip"
	if err := WriteFix(b.install, &Fix{Attempt: "fix", Goal: "G", Commit: b.head, Tip: b.merge, State: "abandoned", Units: []string{"merge"}, Paths: []conflict.Path{{Path: "file.go"}}}); err != nil {
		t.Fatal(err)
	}
	run, err := b.deploy()
	if err != nil || run.Unchanged || !run.SupervisorRestarted || b.builds != 1 || b.restarts != 1 || b.merge != "goal-tip" || slices.Contains(b.calls, "merge --abort") {
		t.Fatalf("restart retry: %+v %v builds=%d restarts=%d", run, err, b.builds, b.restarts)
	}
	run, err = b.deploy()
	if err != nil || !run.Unchanged || b.restarts != 1 {
		t.Fatalf("completed restart repeated: %+v %v", run, err)
	}
}

func TestDeployWaitObservesTheProofUnderTheLaneLock(t *testing.T) {
	t.Parallel()
	b := newDeployBed(t)
	if err := writeRunning(b.install, Running{Attempt: "proof"}); err != nil {
		t.Fatal(err)
	}
	checks := 0
	b.seams.Alive = func(Running) bool {
		checks++
		// A proof observation belongs inside the same lock that protects the build.
		held, err := lock.File(lockPath(b.install), 0600, lock.TryExclusive)
		if err == nil {
			held.Release()
			t.Fatal("proof was observed outside the lane lock")
		}
		if !lock.Busy(err) {
			t.Fatal(err)
		}
		return checks == 1
	}
	_, err := b.deploy()
	if err != nil || b.waits != 1 || b.builds != 1 {
		t.Fatalf("proof admission race: %v waits=%d builds=%d", err, b.waits, b.builds)
	}
}

func TestDeployNowRestartsATrunkProofThroughTheKeeper(t *testing.T) {
	t.Parallel()
	b := newDeployBed(t)
	b.alive, b.opts.Now = true, true
	if err := writeRunning(b.install, Running{Attempt: "proof", Trunk: true, Commit: "old", Tree: "old-tree", Since: bedNow.Format(time.RFC3339)}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.deploy(); err != nil {
		t.Fatal(err)
	}
	b.seams.EndProof = func(Running) error { b.alive = false; return nil }
	original := b.seams.Git
	b.seams.Git = func(dir string, args ...string) (string, error) {
		if strings.Join(args, " ") == "rev-parse --verify main^{tree}" {
			return "new-tree", nil
		}
		return original(dir, args...)
	}
	b.seams.Incidents = func(string, string, string) ([]goal.TrunkRedEntry, error) { return nil, nil }
	b.seams.CommandForCommit = func(commit string) (string, error) {
		if commit != "main" {
			t.Fatalf("command from wrong commit: %s", commit)
		}
		return "fixture check", nil
	}
	b.seams.Executable = func() (string, error) { return "/fixture/engine", nil }
	b.seams.NewID = func() string { return "replacement" }
	launches := 0
	b.seams.Launch = func(argv []string, dir, log string) (int64, error) {
		if !slices.Contains(argv, "--trunk") || !slices.Contains(argv, "--fresh") || dir != b.checkout {
			t.Fatalf("wrong replacement check: %v %s", argv, dir)
		}
		launches++
		return int64(os.Getpid()), nil
	}
	if err := RestartDeployedProof(b.install, b.checkout, b.seams); err != nil {
		t.Fatal(err)
	}
	running, recorded, _, err := ReadRunning(b.install, b.seams)
	if err != nil || !recorded || running.Attempt != "replacement" || !running.Trunk || running.Commit != "main" || launches != 1 {
		t.Fatalf("trunk restart: %+v recorded=%t launches=%d err=%v", running, recorded, launches, err)
	}
}

func TestDeployCurrentEnginePreservesPendingMerge(t *testing.T) {
	t.Parallel()
	for _, state := range []string{"resolving", "abandoned"} {
		t.Run(state, func(t *testing.T) {
			t.Parallel()
			b := newDeployBed(t)
			b.stamp("main")
			b.merge = "goal-tip"
			fix := &Fix{Attempt: "fix", Goal: "G", Commit: b.head, Tip: b.merge, State: state, Units: []string{"merge"}, Paths: []conflict.Path{{Path: "file.go"}}}
			if err := WriteFix(b.install, fix); err != nil {
				t.Fatal(err)
			}
			before := idemDeployFix(t, b.install)
			run, err := b.deploy()
			if err != nil || !run.Unchanged || b.merge != "goal-tip" || b.builds != 0 || b.restarts != 0 || slices.Contains(b.calls, "merge --abort") || !reflect.DeepEqual(before, idemDeployFix(t, b.install)) {
				t.Fatalf("current engine changed merge: %+v %v calls=%v", run, err, b.calls)
			}
		})
	}
}

func idemDeployFix(t *testing.T, install string) *Fix {
	t.Helper()
	fix, err := readFix(install, true)
	if err != nil {
		t.Fatal(err)
	}
	return fix
}

func TestDeployRejectsDirtyEngine(t *testing.T) {
	t.Parallel()
	for _, status := range []string{" M metasystem/internal/edited.go\x00", "?? metasystem/cmd/new file.go\x00", "D  metasystem/go.sum\x00"} {
		t.Run(status, func(t *testing.T) {
			t.Parallel()
			b := newDeployBed(t)
			original := b.seams.Git
			b.seams.Git = func(dir string, args ...string) (string, error) {
				if args[0] == "status" {
					return status, nil
				}
				if slices.Contains(args, "--show-prefix") {
					return "metasystem/", nil
				}
				return original(dir, args...)
			}
			before, err := os.ReadFile(filepath.Join(b.install, "bin", "metasystem"))
			if err != nil {
				t.Fatal(err)
			}
			_, err = b.deploy()
			after, readErr := os.ReadFile(filepath.Join(b.install, "bin", "metasystem"))
			if err == nil || !strings.Contains(err.Error(), "dirty") || readErr != nil || string(before) != string(after) || b.builds != 0 || b.restarts != 0 {
				t.Fatalf("dirty engine activated: err=%v calls=%v before=%q after=%q", err, b.calls, before, after)
			}
		})
	}
}

func TestDeployWrongStampPreservesInstalledEngine(t *testing.T) {
	t.Parallel()
	b := newDeployBed(t)
	original := b.seams.Command
	b.seams.Command = func(command *exec.Cmd) error {
		if err := original(command); err != nil {
			return err
		}
		path := filepath.Join(b.install, "bin", "metasystem")
		if i := slices.Index(command.Args, "--out"); i >= 0 {
			path = command.Args[i+1]
		}
		return os.WriteFile(path, []byte(enginebuild.StampRecord("wrong")), 0600)
	}
	_, err := b.deploy()
	stamp, readErr := installedEngineStamp(b.install)
	if leftovers, err := filepath.Glob(filepath.Join(b.install, "bin", ".metasystem.deploy-*")); err != nil || len(leftovers) != 0 {
		t.Fatalf("staging remains: %v %v", leftovers, err)
	}
	if err == nil || readErr != nil || stamp != "old" || b.restarts != 0 {
		t.Fatalf("unverified engine activated: err=%v stamp=%s read=%v", err, stamp, readErr)
	}
}

func TestDeployStampOverrideRefusesBeforeBuild(t *testing.T) {
	t.Parallel()
	b := newDeployBed(t)
	_, err := deployEngineBuild(b.install, "main", b.seams, "main")
	stamp, readErr := installedEngineStamp(b.install)
	if err == nil || !strings.Contains(err.Error(), "stamp override") || b.builds != 0 || stamp != "old" || readErr != nil {
		t.Fatalf("override built engine: %v stamp=%s read=%v builds=%d", err, stamp, readErr, b.builds)
	}
}

func TestDeployNonEngineDirtDoesNotBlock(t *testing.T) {
	t.Parallel()
	b := newDeployBed(t)
	original := b.seams.Git
	b.seams.Git = func(dir string, args ...string) (string, error) {
		if args[0] == "status" {
			return "?? metasystem/plans/note.md\x00?? metasystem/internal/ui/web/_app/node_modules/file.js\x00", nil
		}
		return original(dir, args...)
	}
	run, err := b.deploy()
	if err != nil || run.After != "main" || b.builds != 1 || b.restarts != 1 {
		t.Fatalf("benign dirt blocked: %+v %v", run, err)
	}
}

func TestDeployStatusPreservesPorcelainColumnsAndPaths(t *testing.T) {
	t.Parallel()
	install := t.TempDir()
	want := " M metasystem/internal/a.go\x00?? metasystem/cmd/line\nbreak.go\x00"
	status, err := deployStatus(install, ProveSeams{Command: func(command *exec.Cmd) error {
		if !slices.Equal(command.Args, []string{"git", "-C", install, "status", "--porcelain=v1", "-z", "--no-renames", "--untracked-files=all"}) {
			t.Fatalf("wrong status command: %v", command.Args)
		}
		_, err := io.WriteString(command.Stdout, want)
		return err
	}})
	if err != nil || status != want {
		t.Fatalf("status bytes changed: %q %v", status, err)
	}
}

func TestDeployMergeUsesResumeRecordScope(t *testing.T) {
	t.Parallel()
	b := newDeployBed(t)
	b.merge = "goal-tip"
	fix := &Fix{Attempt: "merge", Goal: "G", Commit: b.head, Tip: b.merge, State: "abandoned", Units: []string{"merge"}, Paths: []conflict.Path{{Path: "file.go"}}}
	if err := WriteFix(b.install, fix); err != nil {
		t.Fatal(err)
	}
	if err := WriteFix(b.install, &Fix{Attempt: "proof", Goal: "other", Commit: b.head, State: "abandoned"}); err != nil {
		t.Fatal(err)
	}
	// The proof record is newer, but it does not describe the pending merge.
	for i, attempt := range []string{"merge", "proof"} {
		at := bedNow.Add(time.Duration(i) * time.Minute)
		if err := os.Chtimes(filepath.Join(Dir(b.install), "fixes", attempt+".json"), at, at); err != nil {
			t.Fatal(err)
		}
	}
	run, err := b.deploy()
	if err != nil || b.merge != "" || b.builds != 1 || b.restarts != 1 || !slices.Contains(run.Steps, "aborted the abandoned merge of G") {
		t.Fatalf("merge=%q builds=%d restarts=%d run=%+v err=%v", b.merge, b.builds, b.restarts, run, err)
	}
}
